package k8s

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
)

type Client struct {
	Clientset *kubernetes.Clientset
	config    *rest.Config
}

func NewClient(kubeconfig string, kubecontext string) (*Client, error) {
	var config *rest.Config
	var err error

	if kubeconfig != "" {
		// Load from kubeconfig file
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
		}
	} else {
		// Try in-cluster config
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to load in-cluster config: %w", err)
		}
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return &Client{
		Clientset: clientset,
		config:    config,
	}, nil
}

type DeploymentSpec struct {
	Name         string
	Namespace    string
	ImageURI     string
	Port         int32
	Replicas     int32
	HealthPath   string
	Environment  map[string]string
	Labels       map[string]string
}

func (c *Client) DeployService(ctx context.Context, spec *DeploymentSpec) error {
	// Ensure namespace exists
	if err := c.EnsureNamespace(ctx, spec.Namespace); err != nil {
		return fmt.Errorf("failed to ensure namespace: %w", err)
	}

	// Create or update deployment
	if err := c.createOrUpdateDeployment(ctx, spec); err != nil {
		return fmt.Errorf("failed to create/update deployment: %w", err)
	}

	// Create or update service
	if err := c.createOrUpdateService(ctx, spec); err != nil {
		return fmt.Errorf("failed to create/update service: %w", err)
	}

	return nil
}

func (c *Client) EnsureNamespace(ctx context.Context, namespace string) error {
	_, err := c.Clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		// Namespace doesn't exist, create it
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
				Labels: map[string]string{
					"managed-by": "enclii",
				},
			},
		}
		_, err = c.Clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create namespace %s: %w", namespace, err)
		}
	}
	return nil
}

func (c *Client) createOrUpdateDeployment(ctx context.Context, spec *DeploymentSpec) error {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.Name,
			Namespace: spec.Namespace,
			Labels:    spec.Labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": spec.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": spec.Name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  spec.Name,
							Image: spec.ImageURI,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: spec.Port,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Env: c.buildEnvVars(spec.Environment),
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: spec.HealthPath,
										Port: intstr.FromInt(int(spec.Port)),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       10,
								TimeoutSeconds:      5,
								FailureThreshold:    3,
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: spec.HealthPath,
										Port: intstr.FromInt(int(spec.Port)),
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       30,
								TimeoutSeconds:      5,
								FailureThreshold:    3,
							},
						},
					},
				},
			},
		},
	}

	// Try to update first, if not found, create
	_, err := c.Clientset.AppsV1().Deployments(spec.Namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		_, err = c.Clientset.AppsV1().Deployments(spec.Namespace).Create(ctx, deployment, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create deployment: %w", err)
		}
	}

	return nil
}

func (c *Client) createOrUpdateService(ctx context.Context, spec *DeploymentSpec) error {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.Name,
			Namespace: spec.Namespace,
			Labels:    spec.Labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": spec.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Port:       spec.Port,
					TargetPort: intstr.FromInt(int(spec.Port)),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}

	// Try to update first, if not found, create
	_, err := c.Clientset.CoreV1().Services(spec.Namespace).Update(ctx, service, metav1.UpdateOptions{})
	if err != nil {
		_, err = c.Clientset.CoreV1().Services(spec.Namespace).Create(ctx, service, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create service: %w", err)
		}
	}

	return nil
}

func (c *Client) buildEnvVars(env map[string]string) []corev1.EnvVar {
	var envVars []corev1.EnvVar
	for key, value := range env {
		envVars = append(envVars, corev1.EnvVar{
			Name:  key,
			Value: value,
		})
	}
	return envVars
}

func (c *Client) GetDeploymentStatus(ctx context.Context, name, namespace string) (*types.Deployment, error) {
	deployment, err := c.Clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment: %w", err)
	}

	status := types.DeploymentStatusPending
	health := types.HealthStatusUnknown

	if deployment.Status.ReadyReplicas == *deployment.Spec.Replicas {
		status = types.DeploymentStatusRunning
		health = types.HealthStatusHealthy
	} else if deployment.Status.ReadyReplicas > 0 {
		status = types.DeploymentStatusRunning
		health = types.HealthStatusUnhealthy
	}

	return &types.Deployment{
		Status:   status,
		Health:   health,
		Replicas: int(deployment.Status.ReadyReplicas),
	}, nil
}

func (c *Client) RollbackDeployment(ctx context.Context, name, namespace string) error {
	// Get deployment
	deployment, err := c.Clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get deployment: %w", err)
	}

	// Rollback to previous revision
	rollbackConfig := &appsv1.DeploymentRollback{
		Name: name,
	}

	// Note: DeploymentRollback API was deprecated, using kubectl rollout undo approach
	// For MVP, we'll implement a simple approach by updating the deployment
	deployment.Spec.Template.Spec.Containers[0].Image = "previous-image" // TODO: Track previous images

	_, err = c.Clientset.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to rollback deployment: %w", err)
	}

	// In production, we'd use: kubectl rollout undo deployment/name -n namespace
	return nil
}

func (c *Client) GetPodLogs(ctx context.Context, podName, namespace string) (string, error) {
	req := c.Clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Follow:    false,
		TailLines: int64Ptr(100),
	})

	logs, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get log stream: %w", err)
	}
	defer logs.Close()

	buf := make([]byte, 1024)
	n, err := logs.Read(buf)
	if err != nil {
		return "", fmt.Errorf("failed to read logs: %w", err)
	}

	return string(buf[:n]), nil
}

func (c *Client) ListPods(ctx context.Context, namespace, labelSelector string) (*corev1.PodList, error) {
	return c.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
}

// GetLogs retrieves logs from pods matching the label selector
func (c *Client) GetLogs(ctx context.Context, namespace, labelSelector string, lines int, follow bool) (string, error) {
	// Get pods matching the label selector
	pods, err := c.ListPods(ctx, namespace, labelSelector)
	if err != nil {
		return "", fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return "No pods found", nil
	}

	var allLogs strings.Builder
	
	// Get logs from all pods
	for i, pod := range pods.Items {
		if i > 0 {
			allLogs.WriteString("\n--- Pod: " + pod.Name + " ---\n")
		}
		
		req := c.Clientset.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
			Follow:    follow,
			TailLines: int64Ptr(int64(lines)),
		})

		logs, err := req.Stream(ctx)
		if err != nil {
			allLogs.WriteString(fmt.Sprintf("Error getting logs for pod %s: %v\n", pod.Name, err))
			continue
		}

		// Read logs
		scanner := bufio.NewScanner(logs)
		for scanner.Scan() {
			allLogs.WriteString(scanner.Text())
			allLogs.WriteString("\n")
		}
		logs.Close()

		if err := scanner.Err(); err != nil {
			allLogs.WriteString(fmt.Sprintf("Error reading logs for pod %s: %v\n", pod.Name, err))
		}
	}

	return allLogs.String(), nil
}

func int64Ptr(i int64) *int64 {
	return &i
}