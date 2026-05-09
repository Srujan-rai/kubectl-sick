package checker

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"kubectl-sick/pkg/types"
)

func CheckHPAs(ctx context.Context, client kubernetes.Interface, namespace string) ([]types.Issue, error) {
	hpas, err := client.AutoscalingV2().HorizontalPodAutoscalers(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		// Fall back to v1 if v2 not available
		return checkHPAsV1(ctx, client, namespace)
	}

	var issues []types.Issue
	now := time.Now()

	for _, hpa := range hpas.Items {
		if hpa.Status.CurrentReplicas >= hpa.Spec.MaxReplicas {
			// Check conditions to find when it hit max
			var atMaxSince time.Time
			for _, cond := range hpa.Status.Conditions {
				if cond.Type == "ScalingLimited" && cond.Status == "True" {
					atMaxSince = cond.LastTransitionTime.Time
					break
				}
			}

			if atMaxSince.IsZero() {
				atMaxSince = now
			}

			duration := now.Sub(atMaxSince)
			if duration > 30*time.Minute {
				issues = append(issues, types.Issue{
					Severity:  types.Info,
					Kind:      "HPA",
					Namespace: hpa.Namespace,
					Name:      hpa.Name,
					Reason:    fmt.Sprintf("At max replicas %s", formatDuration(duration)),
					Since:     atMaxSince,
				})
			}
		}
	}

	return issues, nil
}

func checkHPAsV1(ctx context.Context, client kubernetes.Interface, namespace string) ([]types.Issue, error) {
	hpas, err := client.AutoscalingV1().HorizontalPodAutoscalers(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing HPAs: %w", err)
	}

	var issues []types.Issue

	for _, hpa := range hpas.Items {
		if hpa.Status.CurrentReplicas >= hpa.Spec.MaxReplicas {
			issues = append(issues, types.Issue{
				Severity:  types.Info,
				Kind:      "HPA",
				Namespace: hpa.Namespace,
				Name:      hpa.Name,
				Reason:    "At max replicas",
			})
		}
	}

	return issues, nil
}
