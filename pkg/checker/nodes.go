package checker

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"kubectl-sick/pkg/types"
)

func CheckNodes(ctx context.Context, client kubernetes.Interface) ([]types.Issue, error) {
	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing nodes: %w", err)
	}

	var issues []types.Issue

	for _, node := range nodes.Items {
		for _, condition := range node.Status.Conditions {
			switch condition.Type {
			case corev1.NodeReady:
				if condition.Status == corev1.ConditionFalse || condition.Status == corev1.ConditionUnknown {
					issues = append(issues, types.Issue{
						Severity:  types.Critical,
						Kind:      "Node",
						Namespace: "",
						Name:      node.Name,
						Reason:    "NotReady",
					})
				}
			case corev1.NodeMemoryPressure:
				if condition.Status == corev1.ConditionTrue {
					issues = append(issues, types.Issue{
						Severity:  types.Critical,
						Kind:      "Node",
						Namespace: "",
						Name:      node.Name,
						Reason:    "MemoryPressure",
					})
				}
			case corev1.NodeDiskPressure:
				if condition.Status == corev1.ConditionTrue {
					issues = append(issues, types.Issue{
						Severity:  types.Critical,
						Kind:      "Node",
						Namespace: "",
						Name:      node.Name,
						Reason:    "DiskPressure",
					})
				}
			case corev1.NodePIDPressure:
				if condition.Status == corev1.ConditionTrue {
					issues = append(issues, types.Issue{
						Severity:  types.Warning,
						Kind:      "Node",
						Namespace: "",
						Name:      node.Name,
						Reason:    "PIDPressure",
					})
				}
			}
		}
	}

	return issues, nil
}
