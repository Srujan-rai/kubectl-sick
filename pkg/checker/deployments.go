package checker

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"kubectl-sick/pkg/types"
)

func CheckDeployments(ctx context.Context, client kubernetes.Interface, namespace string) ([]types.Issue, error) {
	deployments, err := client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing deployments: %w", err)
	}

	var issues []types.Issue

	for _, d := range deployments.Items {
		desired := int32(1)
		if d.Spec.Replicas != nil {
			desired = *d.Spec.Replicas
		}
		available := d.Status.AvailableReplicas

		if available < desired {
			sev := types.Warning
			if available == 0 {
				sev = types.Critical
			}
			issues = append(issues, types.Issue{
				Severity:  sev,
				Kind:      "Deployment",
				Namespace: d.Namespace,
				Name:      d.Name,
				Reason:    fmt.Sprintf("%d/%d replicas available", available, desired),
			})
		}
	}

	return issues, nil
}
