package checker

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"kubectl-sick/pkg/types"
)

func CheckJobs(ctx context.Context, client kubernetes.Interface, namespace string) ([]types.Issue, error) {
	jobs, err := client.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing jobs: %w", err)
	}

	var issues []types.Issue

	for _, job := range jobs.Items {
		// Skip completed jobs
		isComplete := false
		for _, cond := range job.Status.Conditions {
			if cond.Type == "Complete" && cond.Status == "True" {
				isComplete = true
				break
			}
		}
		if isComplete {
			continue
		}

		if job.Status.Failed > 0 && job.Status.Active == 0 {
			issues = append(issues, types.Issue{
				Severity:  types.Warning,
				Kind:      "Job",
				Namespace: job.Namespace,
				Name:      job.Name,
				Reason:    "Failed (backoffLimit reached)",
			})
		} else if job.Status.Active > 0 && job.Status.StartTime != nil {
			running := time.Since(job.Status.StartTime.Time)
			if running > time.Hour {
				issues = append(issues, types.Issue{
					Severity:  types.Info,
					Kind:      "Job",
					Namespace: job.Namespace,
					Name:      job.Name,
					Reason:    fmt.Sprintf("Running %s (check if stuck)", formatDuration(running)),
					Since:     job.Status.StartTime.Time,
				})
			}
		}
	}

	return issues, nil
}
