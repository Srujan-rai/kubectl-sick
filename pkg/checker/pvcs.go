package checker

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"kubectl-sick/pkg/types"
)

func CheckPVCs(ctx context.Context, client kubernetes.Interface, namespace string) ([]types.Issue, error) {
	pvcs, err := client.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing pvcs: %w", err)
	}

	var issues []types.Issue

	for _, pvc := range pvcs.Items {
		switch pvc.Status.Phase {
		case corev1.ClaimPending:
			d := time.Since(pvc.CreationTimestamp.Time)
			issues = append(issues, types.Issue{
				Severity:  types.Critical,
				Kind:      "PVC",
				Namespace: pvc.Namespace,
				Name:      pvc.Name,
				Reason:    fmt.Sprintf("Pending (unbound) %s", formatDuration(d)),
				Since:     pvc.CreationTimestamp.Time,
			})
		case corev1.ClaimLost:
			issues = append(issues, types.Issue{
				Severity:  types.Critical,
				Kind:      "PVC",
				Namespace: pvc.Namespace,
				Name:      pvc.Name,
				Reason:    "Lost",
			})
		}
	}

	return issues, nil
}
