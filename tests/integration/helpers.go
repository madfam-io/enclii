package integration

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// TestHelper provides helper functions for integration tests
type TestHelper struct {
	clientset *kubernetes.Clientset
	namespace string
}

// NewTestHelper creates a new test helper
func NewTestHelper(namespace string) (*TestHelper, error) {
	config, err := getKubeConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return &TestHelper{
		clientset: clientset,
		namespace: namespace,
	}, nil
}

// getKubeConfig returns Kubernetes configuration
func getKubeConfig() (*rest.Config, error) {
	// Try in-cluster config first
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	// Fall back to kubeconfig file
	kubeconfig := clientcmd.NewDefaultClientConfigLoadingRules().GetDefaultFilename()
	config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	return config, nil
}

// CreateNamespace creates a test namespace
func (h *TestHelper) CreateNamespace(ctx context.Context) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: h.namespace,
			Labels: map[string]string{
				"test": "integration",
			},
		},
	}

	_, err := h.clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

// DeleteNamespace deletes the test namespace
func (h *TestHelper) DeleteNamespace(ctx context.Context) error {
	return h.clientset.CoreV1().Namespaces().Delete(ctx, h.namespace, metav1.DeleteOptions{})
}

// WaitForPodReady waits for a pod to be ready
func (h *TestHelper) WaitForPodReady(ctx context.Context, labelSelector string, timeout time.Duration) (*corev1.Pod, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		pods, err := h.clientset.CoreV1().Pods(h.namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			return nil, err
		}

		if len(pods.Items) == 0 {
			time.Sleep(2 * time.Second)
			continue
		}

		pod := &pods.Items[0]
		if pod.Status.Phase == corev1.PodRunning {
			// Check all containers are ready
			allReady := true
			for _, condition := range pod.Status.Conditions {
				if condition.Type == corev1.PodReady && condition.Status != corev1.ConditionTrue {
					allReady = false
					break
				}
			}

			if allReady {
				return pod, nil
			}
		}

		time.Sleep(2 * time.Second)
	}

	return nil, fmt.Errorf("timeout waiting for pod with selector %s", labelSelector)
}

// WaitForDeploymentReady waits for a deployment to be ready
func (h *TestHelper) WaitForDeploymentReady(ctx context.Context, name string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		deployment, err := h.clientset.AppsV1().Deployments(h.namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				time.Sleep(2 * time.Second)
				continue
			}
			return err
		}

		if deployment.Status.ReadyReplicas == *deployment.Spec.Replicas &&
			deployment.Status.UpdatedReplicas == *deployment.Spec.Replicas {
			return nil
		}

		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("timeout waiting for deployment %s", name)
}

// GetPVC gets a PersistentVolumeClaim
func (h *TestHelper) GetPVC(ctx context.Context, name string) (*corev1.PersistentVolumeClaim, error) {
	return h.clientset.CoreV1().PersistentVolumeClaims(h.namespace).Get(ctx, name, metav1.GetOptions{})
}

// WaitForPVCBound waits for a PVC to be bound
func (h *TestHelper) WaitForPVCBound(ctx context.Context, name string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		pvc, err := h.GetPVC(ctx, name)
		if err != nil {
			if errors.IsNotFound(err) {
				time.Sleep(2 * time.Second)
				continue
			}
			return err
		}

		if pvc.Status.Phase == corev1.ClaimBound {
			return nil
		}

		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("timeout waiting for PVC %s to be bound", name)
}

// ExecInPod executes a command in a pod
func (h *TestHelper) ExecInPod(ctx context.Context, podName, containerName string, command []string) (string, error) {
	// Note: This is a simplified version. Full implementation would use client-go's remotecommand package
	// For actual integration tests, you would implement this using:
	// - client-go/tools/remotecommand
	// - client-go/rest
	// - Create a SPDY executor and run the command

	// Placeholder for now
	return "", fmt.Errorf("exec not implemented - use kubectl exec for manual testing")
}

// DeletePod deletes a pod
func (h *TestHelper) DeletePod(ctx context.Context, name string) error {
	return h.clientset.CoreV1().Pods(h.namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// GetIngress gets an Ingress resource
func (h *TestHelper) GetIngress(ctx context.Context, name string) (*networkingv1.Ingress, error) {
	return h.clientset.NetworkingV1().Ingresses(h.namespace).Get(ctx, name, metav1.GetOptions{})
}

// WaitForIngressCreated waits for an Ingress to be created
func (h *TestHelper) WaitForIngressCreated(ctx context.Context, name string, timeout time.Duration) (*networkingv1.Ingress, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		ingress, err := h.GetIngress(ctx, name)
		if err != nil {
			if errors.IsNotFound(err) {
				time.Sleep(2 * time.Second)
				continue
			}
			return nil, err
		}

		return ingress, nil
	}

	return nil, fmt.Errorf("timeout waiting for Ingress %s", name)
}

// GetDeployment gets a Deployment
func (h *TestHelper) GetDeployment(ctx context.Context, name string) (*appsv1.Deployment, error) {
	return h.clientset.AppsV1().Deployments(h.namespace).Get(ctx, name, metav1.GetOptions{})
}

// GetService gets a Service
func (h *TestHelper) GetService(ctx context.Context, name string) (*corev1.Service, error) {
	return h.clientset.CoreV1().Services(h.namespace).Get(ctx, name, metav1.GetOptions{})
}

// GetPod gets a Pod by name
func (h *TestHelper) GetPod(ctx context.Context, name string) (*corev1.Pod, error) {
	return h.clientset.CoreV1().Pods(h.namespace).Get(ctx, name, metav1.GetOptions{})
}

// ListPods lists pods by label selector
func (h *TestHelper) ListPods(ctx context.Context, labelSelector string) (*corev1.PodList, error) {
	return h.clientset.CoreV1().Pods(h.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
}

// Cleanup removes all resources in the test namespace
func (h *TestHelper) Cleanup(ctx context.Context) error {
	// Delete all deployments
	if err := h.clientset.AppsV1().Deployments(h.namespace).DeleteCollection(
		ctx,
		metav1.DeleteOptions{},
		metav1.ListOptions{},
	); err != nil && !errors.IsNotFound(err) {
		return err
	}

	// Delete all services
	if err := h.clientset.CoreV1().Services(h.namespace).DeleteCollection(
		ctx,
		metav1.DeleteOptions{},
		metav1.ListOptions{},
	); err != nil && !errors.IsNotFound(err) {
		return err
	}

	// Delete all PVCs
	if err := h.clientset.CoreV1().PersistentVolumeClaims(h.namespace).DeleteCollection(
		ctx,
		metav1.DeleteOptions{},
		metav1.ListOptions{},
	); err != nil && !errors.IsNotFound(err) {
		return err
	}

	// Delete all ingresses
	if err := h.clientset.NetworkingV1().Ingresses(h.namespace).DeleteCollection(
		ctx,
		metav1.DeleteOptions{},
		metav1.ListOptions{},
	); err != nil && !errors.IsNotFound(err) {
		return err
	}

	// Wait for pods to terminate
	time.Sleep(5 * time.Second)

	return nil
}
