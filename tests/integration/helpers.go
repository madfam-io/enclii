package integration

import (
	"bytes"
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/utils/ptr"
)

// TestHelper provides helper functions for integration tests
type TestHelper struct {
	clientset *kubernetes.Clientset
	config    *rest.Config
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
		config:    config,
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

// ExecInPod executes a command in a pod and returns stdout
func (h *TestHelper) ExecInPod(ctx context.Context, podName, containerName string, command []string) (string, error) {
	req := h.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(h.namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   command,
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(h.config, "POST", req.URL())
	if err != nil {
		return "", fmt.Errorf("failed to create executor: %w", err)
	}

	var stdout, stderr bytes.Buffer
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		return "", fmt.Errorf("exec failed: %w (stderr: %s)", err, stderr.String())
	}

	return stdout.String(), nil
}

// ExecInPodWithStdin executes a command in a pod with stdin input
func (h *TestHelper) ExecInPodWithStdin(ctx context.Context, podName, containerName string, command []string, stdin string) (string, error) {
	req := h.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(h.namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   command,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(h.config, "POST", req.URL())
	if err != nil {
		return "", fmt.Errorf("failed to create executor: %w", err)
	}

	var stdout, stderr bytes.Buffer
	stdinReader := bytes.NewBufferString(stdin)

	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  stdinReader,
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		return "", fmt.Errorf("exec failed: %w (stderr: %s)", err, stderr.String())
	}

	return stdout.String(), nil
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

	// Delete all services (Services don't support DeleteCollection, must delete individually)
	if err := h.deleteAllServices(ctx); err != nil {
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

	// Delete all ingresses (Ingresses don't support DeleteCollection, must delete individually)
	if err := h.deleteAllIngresses(ctx); err != nil {
		return err
	}

	// Wait for pods to terminate
	time.Sleep(5 * time.Second)

	return nil
}

// deleteAllServices deletes all services in the namespace one by one
// Note: Services API doesn't support DeleteCollection
func (h *TestHelper) deleteAllServices(ctx context.Context) error {
	services, err := h.clientset.CoreV1().Services(h.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	for _, svc := range services.Items {
		if err := h.clientset.CoreV1().Services(h.namespace).Delete(ctx, svc.Name, metav1.DeleteOptions{}); err != nil {
			if !errors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}

// deleteAllIngresses deletes all ingresses in the namespace one by one
// Note: Ingresses API doesn't support DeleteCollection in all k8s versions
func (h *TestHelper) deleteAllIngresses(ctx context.Context) error {
	ingresses, err := h.clientset.NetworkingV1().Ingresses(h.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	for _, ing := range ingresses.Items {
		if err := h.clientset.NetworkingV1().Ingresses(h.namespace).Delete(ctx, ing.Name, metav1.DeleteOptions{}); err != nil {
			if !errors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}

// CreateSecret creates a secret in the test namespace
func (h *TestHelper) CreateSecret(ctx context.Context, name string, data map[string]string) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: h.namespace,
		},
		StringData: data,
	}

	_, err := h.clientset.CoreV1().Secrets(h.namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create secret %s: %w", name, err)
	}

	return nil
}

// DeployPostgres deploys PostgreSQL with PVC into the test namespace
func (h *TestHelper) DeployPostgres(ctx context.Context) error {
	storageClass := "standard"

	// Create PVC
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "postgres-pvc",
			Namespace: h.namespace,
			Labels: map[string]string{
				"app": "postgres",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: &storageClass,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
		},
	}

	_, err := h.clientset.CoreV1().PersistentVolumeClaims(h.namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create postgres PVC: %w", err)
	}

	// Create Deployment
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "postgres",
			Namespace: h.namespace,
			Labels: map[string]string{
				"app": "postgres",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "postgres",
				},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "postgres",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "postgres",
							Image: "postgres:15-alpine",
							Ports: []corev1.ContainerPort{
								{ContainerPort: 5432},
							},
							Env: []corev1.EnvVar{
								{
									Name: "POSTGRES_USER",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: "postgres-credentials"},
											Key:                  "username",
										},
									},
								},
								{
									Name: "POSTGRES_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: "postgres-credentials"},
											Key:                  "password",
										},
									},
								},
								{
									Name:  "PGDATA",
									Value: "/var/lib/postgresql/data/pgdata",
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "postgres-data",
									MountPath: "/var/lib/postgresql/data",
								},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									Exec: &corev1.ExecAction{
										Command: []string{"pg_isready", "-U", "postgres"},
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       5,
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "postgres-data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "postgres-pvc",
								},
							},
						},
					},
				},
			},
		},
	}

	_, err = h.clientset.AppsV1().Deployments(h.namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create postgres deployment: %w", err)
	}

	// Create Service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "postgres",
			Namespace: h.namespace,
			Labels: map[string]string{
				"app": "postgres",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": "postgres",
			},
			Ports: []corev1.ServicePort{
				{
					Port:     5432,
					Protocol: corev1.ProtocolTCP,
				},
			},
		},
	}

	_, err = h.clientset.CoreV1().Services(h.namespace).Create(ctx, service, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create postgres service: %w", err)
	}

	return nil
}

// DeployRedis deploys Redis with PVC into the test namespace
func (h *TestHelper) DeployRedis(ctx context.Context) error {
	storageClass := "standard"

	// Create PVC
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis-pvc",
			Namespace: h.namespace,
			Labels: map[string]string{
				"app": "redis",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: &storageClass,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("5Gi"),
				},
			},
		},
	}

	_, err := h.clientset.CoreV1().PersistentVolumeClaims(h.namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create redis PVC: %w", err)
	}

	// Create Deployment
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis",
			Namespace: h.namespace,
			Labels: map[string]string{
				"app": "redis",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "redis",
				},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "redis",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "redis",
							Image: "redis:7-alpine",
							Args:  []string{"--appendonly", "yes", "--dir", "/data"},
							Ports: []corev1.ContainerPort{
								{ContainerPort: 6379},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "redis-data",
									MountPath: "/data",
								},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									Exec: &corev1.ExecAction{
										Command: []string{"redis-cli", "ping"},
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       5,
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "redis-data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "redis-pvc",
								},
							},
						},
					},
				},
			},
		},
	}

	_, err = h.clientset.AppsV1().Deployments(h.namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create redis deployment: %w", err)
	}

	// Create Service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis",
			Namespace: h.namespace,
			Labels: map[string]string{
				"app": "redis",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": "redis",
			},
			Ports: []corev1.ServicePort{
				{
					Port:     6379,
					Protocol: corev1.ProtocolTCP,
				},
			},
		},
	}

	_, err = h.clientset.CoreV1().Services(h.namespace).Create(ctx, service, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create redis service: %w", err)
	}

	return nil
}

// DeployTestService deploys a simple test service with a volume for testing
func (h *TestHelper) DeployTestService(ctx context.Context, serviceName string, volumes map[string]string) error {
	storageClass := "standard"

	// Create PVCs for each volume
	for volumeName := range volumes {
		pvcName := serviceName + "-" + volumeName
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: h.namespace,
				Labels: map[string]string{
					"app": serviceName,
				},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				StorageClassName: &storageClass,
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			},
		}

		_, err := h.clientset.CoreV1().PersistentVolumeClaims(h.namespace).Create(ctx, pvc, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create PVC %s: %w", pvcName, err)
		}
	}

	// Build volume mounts and volumes
	var volumeMounts []corev1.VolumeMount
	var podVolumes []corev1.Volume

	for volumeName, mountPath := range volumes {
		pvcName := serviceName + "-" + volumeName
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      volumeName,
			MountPath: mountPath,
		})
		podVolumes = append(podVolumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		})
	}

	// Create Deployment
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: h.namespace,
			Labels: map[string]string{
				"app": serviceName,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": serviceName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": serviceName,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:         serviceName,
							Image:        "busybox:1.36",
							Command:      []string{"sh", "-c", "while true; do sleep 3600; done"},
							VolumeMounts: volumeMounts,
						},
					},
					Volumes: podVolumes,
				},
			},
		},
	}

	_, err := h.clientset.AppsV1().Deployments(h.namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create deployment %s: %w", serviceName, err)
	}

	// Create Service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: h.namespace,
			Labels: map[string]string{
				"app": serviceName,
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": serviceName,
			},
			Ports: []corev1.ServicePort{
				{
					Port:     80,
					Protocol: corev1.ProtocolTCP,
				},
			},
		},
	}

	_, err = h.clientset.CoreV1().Services(h.namespace).Create(ctx, service, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create service %s: %w", serviceName, err)
	}

	return nil
}
