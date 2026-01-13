package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetConfigMap retrieves a ConfigMap by name and namespace
func (c *Client) GetConfigMap(ctx context.Context, namespace, name string) (*corev1.ConfigMap, error) {
	cm, err := c.Clientset.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get configmap %s/%s: %w", namespace, name, err)
	}
	return cm, nil
}

// UpdateConfigMap updates an existing ConfigMap
func (c *Client) UpdateConfigMap(ctx context.Context, cm *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	updated, err := c.Clientset.CoreV1().ConfigMaps(cm.Namespace).Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to update configmap %s/%s: %w", cm.Namespace, cm.Name, err)
	}
	return updated, nil
}

// CreateConfigMap creates a new ConfigMap
func (c *Client) CreateConfigMap(ctx context.Context, cm *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	created, err := c.Clientset.CoreV1().ConfigMaps(cm.Namespace).Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create configmap %s/%s: %w", cm.Namespace, cm.Name, err)
	}
	return created, nil
}

// DeleteConfigMap deletes a ConfigMap
func (c *Client) DeleteConfigMap(ctx context.Context, namespace, name string) error {
	err := c.Clientset.CoreV1().ConfigMaps(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete configmap %s/%s: %w", namespace, name, err)
	}
	return nil
}

// ConfigMapExists checks if a ConfigMap exists
func (c *Client) ConfigMapExists(ctx context.Context, namespace, name string) (bool, error) {
	_, err := c.Clientset.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		// Check if it's a not found error
		if isNotFoundError(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check configmap %s/%s: %w", namespace, name, err)
	}
	return true, nil
}

// isNotFoundError checks if an error is a Kubernetes NotFound error
func isNotFoundError(err error) bool {
	return err != nil && (err.Error() == "not found" ||
		contains(err.Error(), "not found") ||
		contains(err.Error(), "NotFound"))
}

// contains is a helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
