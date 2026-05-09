package output

import (
	"encoding/json"
	"fmt"
	"time"

	sigs_yaml "sigs.k8s.io/yaml"
	"kubectl-sick/pkg/types"
)

type JSONOutput struct {
	ScannedAt      time.Time   `json:"scanned_at"`
	ClusterContext string      `json:"cluster_context"`
	Summary        JSONSummary `json:"summary"`
	Issues         []JSONIssue `json:"issues"`
}

type JSONSummary struct {
	Critical int `json:"critical"`
	Warning  int `json:"warning"`
	Info     int `json:"info"`
}

type JSONIssue struct {
	Severity  string     `json:"severity"`
	Kind      string     `json:"kind"`
	Namespace string     `json:"namespace"`
	Name      string     `json:"name"`
	Reason    string     `json:"reason"`
	Since     *time.Time `json:"since,omitempty"`
}

func PrintJSON(issues []types.Issue, clusterContext string) error {
	out := buildOutput(issues, clusterContext)
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func PrintYAML(issues []types.Issue, clusterContext string) error {
	out := buildOutput(issues, clusterContext)
	data, err := sigs_yaml.Marshal(out)
	if err != nil {
		return fmt.Errorf("marshaling YAML: %w", err)
	}
	fmt.Print(string(data))
	return nil
}

func buildOutput(issues []types.Issue, clusterContext string) JSONOutput {
	var critical, warning, info int
	jsonIssues := make([]JSONIssue, 0, len(issues))

	for _, issue := range issues {
		switch issue.Severity {
		case types.Critical:
			critical++
		case types.Warning:
			warning++
		case types.Info:
			info++
		}

		ns := issue.Namespace
		ji := JSONIssue{
			Severity:  issue.Severity.String(),
			Kind:      issue.Kind,
			Namespace: ns,
			Name:      issue.Name,
			Reason:    issue.Reason,
		}
		if !issue.Since.IsZero() {
			t := issue.Since
			ji.Since = &t
		}
		jsonIssues = append(jsonIssues, ji)
	}

	return JSONOutput{
		ScannedAt:      time.Now().UTC(),
		ClusterContext: clusterContext,
		Summary: JSONSummary{
			Critical: critical,
			Warning:  warning,
			Info:     info,
		},
		Issues: jsonIssues,
	}
}
