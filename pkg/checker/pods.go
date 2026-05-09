package checker

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"kubectl-sick/pkg/types"
)

func CheckPods(ctx context.Context, client kubernetes.Interface, namespace string) ([]types.Issue, error) {
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing pods: %w", err)
	}

	var issues []types.Issue

	for _, pod := range pods.Items {
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Waiting != nil {
				reason := cs.State.Waiting.Reason
				switch reason {
				case "CrashLoopBackOff":
					issues = append(issues, types.Issue{
						Severity:  types.Critical,
						Kind:      "Pod",
						Namespace: pod.Namespace,
						Name:      pod.Name,
						Reason:    fmt.Sprintf("CrashLoopBackOff (%dx)", cs.RestartCount),
					})
				case "ImagePullBackOff", "ErrImagePull":
					issues = append(issues, types.Issue{
						Severity:  types.Warning,
						Kind:      "Pod",
						Namespace: pod.Namespace,
						Name:      pod.Name,
						Reason:    reason,
					})
				case "Error":
					issues = append(issues, types.Issue{
						Severity:  types.Critical,
						Kind:      "Pod",
						Namespace: pod.Namespace,
						Name:      pod.Name,
						Reason:    "Error",
					})
				}
			}

			// Catch CrashLoop in the terminated phase of its backoff cycle:
			// pod shows "Running/Terminated" not "Waiting:CrashLoopBackOff" when kubectl-sick polls.
			if cs.State.Waiting == nil && cs.RestartCount >= 3 &&
				cs.LastTerminationState.Terminated != nil &&
				cs.LastTerminationState.Terminated.Reason == "Error" {
				issues = append(issues, types.Issue{
					Severity:  types.Critical,
					Kind:      "Pod",
					Namespace: pod.Namespace,
					Name:      pod.Name,
					Reason:    fmt.Sprintf("CrashLoopBackOff (%dx)", cs.RestartCount),
				})
			}

			if cs.LastTerminationState.Terminated != nil &&
				cs.LastTerminationState.Terminated.Reason == "OOMKilled" {
				issues = append(issues, types.Issue{
					Severity:  types.Warning,
					Kind:      "Pod",
					Namespace: pod.Namespace,
					Name:      pod.Name,
					Reason:    "OOMKilled",
				})
			}
		}

		if pod.Status.Phase == "Pending" {
			pendingDuration := time.Since(pod.CreationTimestamp.Time)
			if pendingDuration > 5*time.Minute {
				issues = append(issues, types.Issue{
					Severity:  types.Warning,
					Kind:      "Pod",
					Namespace: pod.Namespace,
					Name:      pod.Name,
					Reason:    fmt.Sprintf("Pending %s", formatDuration(pendingDuration)),
					Since:     pod.CreationTimestamp.Time,
				})
			}
		}

		if pod.DeletionTimestamp != nil {
			terminatingDuration := time.Since(pod.DeletionTimestamp.Time)
			if terminatingDuration > 10*time.Minute {
				issues = append(issues, types.Issue{
					Severity:  types.Warning,
					Kind:      "Pod",
					Namespace: pod.Namespace,
					Name:      pod.Name,
					Reason:    fmt.Sprintf("Stuck terminating %s", formatDuration(terminatingDuration)),
				})
			}
		}
	}

	return issues, nil
}

func formatDuration(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}
