package checker

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"kubectl-sick/pkg/types"
)

var certGVR = schema.GroupVersionResource{
	Group:    "cert-manager.io",
	Version:  "v1",
	Resource: "certificates",
}

func CheckCerts(ctx context.Context, dynClient dynamic.Interface, disco discovery.DiscoveryInterface, namespace string) ([]types.Issue, error) {
	_, err := disco.ServerResourcesForGroupVersion("cert-manager.io/v1")
	if err != nil {
		return nil, nil
	}

	list, err := dynClient.Resource(certGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing certificates: %w", err)
	}

	var issues []types.Issue
	now := time.Now()

	for _, item := range list.Items {
		name := item.GetName()
		ns := item.GetNamespace()

		status, ok := item.Object["status"].(map[string]interface{})
		if !ok {
			continue
		}

		// Check Ready condition
		if conditions, ok := status["conditions"].([]interface{}); ok {
			for _, c := range conditions {
				cond, ok := c.(map[string]interface{})
				if !ok {
					continue
				}
				if cond["type"] == "Ready" && cond["status"] == "False" {
					issues = append(issues, types.Issue{
						Severity:  types.Warning,
						Kind:      "Certificate",
						Namespace: ns,
						Name:      name,
						Reason:    "Not ready",
					})
				}
			}
		}

		notAfterStr, _ := status["notAfter"].(string)
		if notAfterStr != "" {
			notAfter, err := time.Parse(time.RFC3339, notAfterStr)
			if err == nil {
				daysLeft := notAfter.Sub(now).Hours() / 24
				if daysLeft < 0 {
					issues = append(issues, types.Issue{
						Severity:  types.Critical,
						Kind:      "Certificate",
						Namespace: ns,
						Name:      name,
						Reason:    "Expired",
					})
				} else if daysLeft < 7 {
					issues = append(issues, types.Issue{
						Severity:  types.Warning,
						Kind:      "Certificate",
						Namespace: ns,
						Name:      name,
						Reason:    fmt.Sprintf("Expiring in %dd", int(daysLeft)),
					})
				}
			}
		}
	}

	return issues, nil
}
