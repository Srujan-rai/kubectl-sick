package cmd

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"kubectl-sick/pkg/checker"
	"kubectl-sick/pkg/output"
	"kubectl-sick/pkg/types"
)

var (
	namespace   string
	minSeverity string
	jsonOutput  bool
	yamlOutput  bool
	exitCode    bool
	explain     string
	watch       bool
	interval    int
	noColor     bool
	kubeContext string
	kubeconfig  string
)

var rootCmd = &cobra.Command{
	Use:   "kubectl-sick",
	Short: "Lists every broken thing in your cluster, sorted worst-first",
	RunE:  run,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Scope scan to a specific namespace (default: all namespaces)")
	rootCmd.Flags().StringVar(&minSeverity, "min-severity", "info", "Minimum severity to show: info, warning, critical")
	rootCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	rootCmd.Flags().BoolVar(&yamlOutput, "yaml", false, "Output as YAML")
	rootCmd.Flags().BoolVar(&exitCode, "exit-code", false, "Exit with code 1 if any critical issues found")
	rootCmd.Flags().StringVar(&explain, "explain", "", "Show logs + events for a resource (namespace/name or kind/namespace/name)")
	rootCmd.Flags().BoolVar(&watch, "watch", false, "Re-run scan every N seconds")
	rootCmd.Flags().IntVar(&interval, "interval", 30, "Watch interval in seconds (used with --watch)")
	rootCmd.Flags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	rootCmd.Flags().StringVar(&kubeContext, "context", "", "Kubernetes context to use")
	rootCmd.Flags().StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file")
}

func buildClients() (kubernetes.Interface, dynamic.Interface, discovery.DiscoveryInterface, string, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != "" {
		loadingRules.ExplicitPath = kubeconfig
	}

	configOverrides := &clientcmd.ConfigOverrides{}
	if kubeContext != "" {
		configOverrides.CurrentContext = kubeContext
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, nil, nil, "", fmt.Errorf("building kubeconfig: %w", err)
	}

	rawConfig, err := clientConfig.RawConfig()
	currentContext := ""
	if err == nil {
		currentContext = rawConfig.CurrentContext
		if kubeContext != "" {
			currentContext = kubeContext
		}
	}

	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, nil, nil, "", fmt.Errorf("creating kubernetes client: %w", err)
	}

	dynClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, nil, nil, "", fmt.Errorf("creating dynamic client: %w", err)
	}

	discoClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return nil, nil, nil, "", fmt.Errorf("creating discovery client: %w", err)
	}

	return client, dynClient, discoClient, currentContext, nil
}

func runScan(ctx context.Context, client kubernetes.Interface, dynClient dynamic.Interface, discoClient discovery.DiscoveryInterface) []types.Issue {
	var wg sync.WaitGroup
	results := make(chan []types.Issue, 8)

	checkerFns := []func() ([]types.Issue, error){
		func() ([]types.Issue, error) { return checker.CheckPods(ctx, client, namespace) },
		func() ([]types.Issue, error) { return checker.CheckNodes(ctx, client) },
		func() ([]types.Issue, error) { return checker.CheckPVCs(ctx, client, namespace) },
		func() ([]types.Issue, error) { return checker.CheckDeployments(ctx, client, namespace) },
		func() ([]types.Issue, error) { return checker.CheckJobs(ctx, client, namespace) },
		func() ([]types.Issue, error) {
			return checker.CheckCerts(ctx, dynClient, discoClient, namespace)
		},
		func() ([]types.Issue, error) { return checker.CheckHPAs(ctx, client, namespace) },
	}

	for _, fn := range checkerFns {
		wg.Add(1)
		go func(f func() ([]types.Issue, error)) {
			defer wg.Done()
			issues, err := f()
			if err != nil {
				fmt.Fprintf(os.Stderr, "warn: %v\n", err)
			}
			results <- issues
		}(fn)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var allIssues []types.Issue
	for batch := range results {
		allIssues = append(allIssues, batch...)
	}

	types.SortIssues(allIssues)
	return allIssues
}

func filterBySeverity(issues []types.Issue) []types.Issue {
	var minSev types.Severity
	switch minSeverity {
	case "warning":
		minSev = types.Warning
	case "critical":
		minSev = types.Critical
	default:
		minSev = types.Info
	}

	var filtered []types.Issue
	for _, issue := range issues {
		if issue.Severity >= minSev {
			filtered = append(filtered, issue)
		}
	}
	return filtered
}

func run(cmd *cobra.Command, args []string) error {
	client, dynClient, discoClient, clusterContext, err := buildClients()
	if err != nil {
		return err
	}

	ctx := context.Background()

	if explain != "" {
		return runExplain(ctx, client, explain)
	}

	if watch {
		return runWatch(ctx, client, dynClient, discoClient, clusterContext)
	}

	issues := runScan(ctx, client, dynClient, discoClient)
	issues = filterBySeverity(issues)

	if jsonOutput {
		return output.PrintJSON(issues, clusterContext)
	}

	if yamlOutput {
		return output.PrintYAML(issues, clusterContext)
	}

	output.PrintTable(issues, noColor)

	if exitCode {
		for _, issue := range issues {
			if issue.Severity == types.Critical {
				os.Exit(1)
			}
		}
	}

	return nil
}

func runWatch(ctx context.Context, client kubernetes.Interface, dynClient dynamic.Interface, discoClient discovery.DiscoveryInterface, clusterContext string) error {
	var prevIssues []types.Issue
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	sigCh := make(chan os.Signal, 1)
	// signal.Notify(sigCh, os.Interrupt) -- omit for minimal deps

	doScan := func() {
		// Clear terminal
		fmt.Print("\033[H\033[2J")
		now := time.Now()
		fmt.Printf("Watching cluster — last scan: %s — next in %ds\n\n",
			now.Format("15:04:05"), interval)

		issues := runScan(ctx, client, dynClient, discoClient)
		issues = filterBySeverity(issues)

		// Diff — save current before mutation so prevIssues is never polluted with markers
		prevMap := issueMap(prevIssues)
		currMap := issueMap(issues)
		currentIssues := make([]types.Issue, len(issues))
		copy(currentIssues, issues)

		for key := range prevMap {
			if _, exists := currMap[key]; !exists {
				resolved := prevMap[key]
				resolved.Reason = "[RESOLVED] " + resolved.Reason
				issues = append(issues, resolved)
			}
		}

		var displayIssues []types.Issue
		for _, issue := range issues {
			key := issueKey(issue)
			if _, existed := prevMap[key]; !existed {
				issue.Reason = "[NEW] " + issue.Reason
			}
			displayIssues = append(displayIssues, issue)
		}

		types.SortIssues(displayIssues)
		output.PrintTable(displayIssues, noColor)
		prevIssues = currentIssues
		_ = sigCh
		_ = clusterContext
	}

	doScan()
	for range ticker.C {
		doScan()
	}

	return nil
}

func issueKey(issue types.Issue) string {
	return fmt.Sprintf("%s/%s/%s", issue.Kind, issue.Namespace, issue.Name)
}

func issueMap(issues []types.Issue) map[string]types.Issue {
	m := make(map[string]types.Issue)
	for _, issue := range issues {
		m[issueKey(issue)] = issue
	}
	return m
}

func runExplain(ctx context.Context, client kubernetes.Interface, target string) error {
	// Parse namespace/name or kind/namespace/name
	var ns, name string

	parts := splitExplain(target)
	switch len(parts) {
	case 2:
		ns = parts[0]
		name = parts[1]
	case 3:
		ns = parts[1]
		name = parts[2]
	default:
		return fmt.Errorf("--explain format: namespace/name or kind/namespace/name")
	}

	fmt.Printf("\n── Pod: %s/%s ─────────────────────────────────────\n\n", ns, name)

	// Fetch pod logs
	pod, err := client.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("LOGS: could not fetch pod: %v\n", err)
	} else {
		fmt.Println("LOGS (last 20 lines):")
		tailLines := int64(20)
		for _, container := range pod.Spec.Containers {
			fmt.Printf("  [container: %s]\n", container.Name)
			req := client.CoreV1().Pods(ns).GetLogs(name, &corev1.PodLogOptions{
				Container: container.Name,
				TailLines: &tailLines,
			})
			logs, logErr := req.DoRaw(ctx)
			if logErr != nil {
				fmt.Printf("  (error fetching logs: %v)\n", logErr)
			} else {
				fmt.Printf("  %s\n", string(logs))
			}
		}
	}

	// Fetch events
	fmt.Println("\nEVENTS (last 5 warnings):")
	events, err := client.CoreV1().Events(ns).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s", name),
	})
	if err == nil {
		count := 0
		for i := len(events.Items) - 1; i >= 0 && count < 5; i-- {
			ev := events.Items[i]
			if ev.Type == "Warning" {
				age := time.Since(ev.LastTimestamp.Time)
				fmt.Printf("  %s\t%s\t%s\t%s/%s\t%s\n",
					formatDuration(age),
					ev.Type,
					ev.Reason,
					ev.InvolvedObject.Kind,
					ev.InvolvedObject.Name,
					ev.Message,
				)
				count++
			}
		}
	}

	return nil
}

func splitExplain(s string) []string {
	var parts []string
	current := ""
	for _, c := range s {
		if c == '/' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func formatDuration(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}
