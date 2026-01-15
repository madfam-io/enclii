package k8s

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/madfam-org/enclii/packages/sdk-go/pkg/types"
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

// Config returns the Kubernetes REST config for creating additional clients
func (c *Client) Config() *rest.Config {
	return c.config
}

type DeploymentSpec struct {
	Name        string
	Namespace   string
	ImageURI    string
	Port        int32
	Replicas    int32
	HealthPath  string
	Environment map[string]string
	Labels      map[string]string
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

	// Find the previous image from ReplicaSet history
	previousImage, err := c.getPreviousImage(ctx, name, namespace, deployment)
	if err != nil {
		return fmt.Errorf("failed to find previous image: %w", err)
	}

	if previousImage == "" {
		return fmt.Errorf("no previous revision found to rollback to")
	}

	// Update the deployment with the previous image
	if len(deployment.Spec.Template.Spec.Containers) == 0 {
		return fmt.Errorf("deployment has no containers")
	}

	currentImage := deployment.Spec.Template.Spec.Containers[0].Image
	if currentImage == previousImage {
		return fmt.Errorf("already at previous revision (image: %s)", currentImage)
	}

	deployment.Spec.Template.Spec.Containers[0].Image = previousImage

	// Add rollback annotation for audit trail
	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = make(map[string]string)
	}
	deployment.Spec.Template.Annotations["enclii.dev/rollback-from"] = currentImage
	deployment.Spec.Template.Annotations["enclii.dev/rollback-at"] = metav1.Now().Format(time.RFC3339)

	_, err = c.Clientset.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to rollback deployment: %w", err)
	}

	return nil
}

// getPreviousImage finds the image from the previous ReplicaSet revision
func (c *Client) getPreviousImage(ctx context.Context, deploymentName, namespace string, deployment *appsv1.Deployment) (string, error) {
	// List all ReplicaSets owned by this deployment
	rsList, err := c.Clientset.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", deploymentName),
	})
	if err != nil {
		return "", fmt.Errorf("failed to list replica sets: %w", err)
	}

	if len(rsList.Items) < 2 {
		return "", fmt.Errorf("no previous revision available (only %d replica set(s) found)", len(rsList.Items))
	}

	// Find the current revision number
	currentRevision := deployment.Annotations["deployment.kubernetes.io/revision"]

	// Find the previous revision's ReplicaSet
	var previousRS *appsv1.ReplicaSet
	var previousRevision int64

	for i := range rsList.Items {
		rs := &rsList.Items[i]

		// Skip ReplicaSets not owned by this deployment
		isOwned := false
		for _, ownerRef := range rs.OwnerReferences {
			if ownerRef.UID == deployment.UID {
				isOwned = true
				break
			}
		}
		if !isOwned {
			continue
		}

		rsRevision := rs.Annotations["deployment.kubernetes.io/revision"]
		if rsRevision == currentRevision {
			continue // Skip current revision
		}

		// Parse revision number
		var rev int64
		fmt.Sscanf(rsRevision, "%d", &rev)

		// Keep track of the highest revision that's not current
		if rev > previousRevision {
			previousRevision = rev
			previousRS = rs
		}
	}

	if previousRS == nil {
		return "", fmt.Errorf("could not find previous replica set")
	}

	// Get the image from the previous ReplicaSet
	if len(previousRS.Spec.Template.Spec.Containers) == 0 {
		return "", fmt.Errorf("previous replica set has no containers")
	}

	return previousRS.Spec.Template.Spec.Containers[0].Image, nil
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

// DeploymentStatusInfo contains detailed deployment status information
type DeploymentStatusInfo struct {
	Replicas            int32
	UpdatedReplicas     int32
	ReadyReplicas       int32
	AvailableReplicas   int32
	UnavailableReplicas int32
	Generation          int64
	ObservedGeneration  int64
	ImageTag            string // Image tag from first container (for version display)
}

// GetDeploymentStatusInfo returns detailed status information about a deployment
func (c *Client) GetDeploymentStatusInfo(ctx context.Context, namespace, name string) (*DeploymentStatusInfo, error) {
	deployment, err := c.Clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment: %w", err)
	}

	// Extract image tag from first container for version display
	imageTag := ""
	if len(deployment.Spec.Template.Spec.Containers) > 0 {
		image := deployment.Spec.Template.Spec.Containers[0].Image
		// Extract tag after the last ":"
		if idx := strings.LastIndex(image, ":"); idx != -1 {
			imageTag = image[idx+1:]
		}
	}

	status := &DeploymentStatusInfo{
		Replicas:            deployment.Status.Replicas,
		UpdatedReplicas:     deployment.Status.UpdatedReplicas,
		ReadyReplicas:       deployment.Status.ReadyReplicas,
		AvailableReplicas:   deployment.Status.AvailableReplicas,
		UnavailableReplicas: deployment.Status.UnavailableReplicas,
		Generation:          deployment.Generation,
		ObservedGeneration:  deployment.Status.ObservedGeneration,
		ImageTag:            imageTag,
	}

	return status, nil
}

// LogStreamOptions configures log streaming behavior
type LogStreamOptions struct {
	Namespace     string
	LabelSelector string
	TailLines     int64
	Follow        bool
	Timestamps    bool
}

// LogLine represents a single log line with metadata
type LogLine struct {
	Pod       string    `json:"pod"`
	Container string    `json:"container"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
}

// StreamLogs streams logs from pods matching the label selector to a channel
func (c *Client) StreamLogs(ctx context.Context, opts LogStreamOptions, logChan chan<- LogLine, errChan chan<- error) {
	defer close(logChan)
	defer close(errChan)

	// Get pods matching the label selector
	pods, err := c.ListPods(ctx, opts.Namespace, opts.LabelSelector)
	if err != nil {
		errChan <- fmt.Errorf("failed to list pods: %w", err)
		return
	}

	if len(pods.Items) == 0 {
		errChan <- fmt.Errorf("no pods found matching selector: %s", opts.LabelSelector)
		return
	}

	// Create a wait group to track all goroutines
	var wg sync.WaitGroup

	// Stream logs from each pod
	for _, pod := range pods.Items {
		for _, container := range pod.Spec.Containers {
			wg.Add(1)
			go func(podName, containerName string) {
				defer wg.Done()
				c.streamPodLogs(ctx, opts, podName, containerName, logChan, errChan)
			}(pod.Name, container.Name)
		}
	}

	wg.Wait()
}

// streamPodLogs streams logs from a specific pod/container
func (c *Client) streamPodLogs(ctx context.Context, opts LogStreamOptions, podName, containerName string, logChan chan<- LogLine, errChan chan<- error) {
	podLogOpts := &corev1.PodLogOptions{
		Container:  containerName,
		Follow:     opts.Follow,
		Timestamps: opts.Timestamps,
	}

	if opts.TailLines > 0 {
		podLogOpts.TailLines = &opts.TailLines
	}

	req := c.Clientset.CoreV1().Pods(opts.Namespace).GetLogs(podName, podLogOpts)
	stream, err := req.Stream(ctx)
	if err != nil {
		errChan <- fmt.Errorf("failed to get log stream for pod %s: %w", podName, err)
		return
	}
	defer stream.Close()

	scanner := bufio.NewScanner(stream)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
			line := scanner.Text()
			logLine := LogLine{
				Pod:       podName,
				Container: containerName,
				Timestamp: time.Now(),
				Message:   line,
			}

			// Parse timestamp if present (format: 2006-01-02T15:04:05.999999999Z message)
			if opts.Timestamps && len(line) > 30 {
				if ts, err := time.Parse(time.RFC3339Nano, line[:30]); err == nil {
					logLine.Timestamp = ts
					logLine.Message = strings.TrimPrefix(line[30:], " ")
				}
			}

			select {
			case logChan <- logLine:
			case <-ctx.Done():
				return
			}
		}
	}

	if err := scanner.Err(); err != nil {
		errChan <- fmt.Errorf("error reading logs for pod %s: %w", podName, err)
	}
}

// ListDeployments returns all deployments in a namespace
func (c *Client) ListDeployments(ctx context.Context, namespace string) ([]appsv1.Deployment, error) {
	list, err := c.Clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list deployments in namespace %s: %w", namespace, err)
	}
	return list.Items, nil
}

// ScaleDeployment scales a deployment to the specified number of replicas
func (c *Client) ScaleDeployment(ctx context.Context, namespace, name string, replicas int32) error {
	deployment, err := c.Clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get deployment: %w", err)
	}

	deployment.Spec.Replicas = &replicas

	_, err = c.Clientset.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to scale deployment: %w", err)
	}

	return nil
}

// DeleteDeploymentAndService deletes a deployment and its associated service
func (c *Client) DeleteDeploymentAndService(ctx context.Context, namespace, name string) error {
	// Delete deployment
	err := c.Clientset.AppsV1().Deployments(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		// Ignore not found errors
		if !strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("failed to delete deployment: %w", err)
		}
	}

	// Delete service
	err = c.Clientset.CoreV1().Services(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		// Ignore not found errors
		if !strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("failed to delete service: %w", err)
		}
	}

	return nil
}

// DeploymentExists checks if a deployment exists
func (c *Client) DeploymentExists(ctx context.Context, namespace, name string) (bool, error) {
	_, err := c.Clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return false, nil
		}
		return false, fmt.Errorf("failed to check deployment: %w", err)
	}
	return true, nil
}

// RollingRestart triggers a rolling restart of a deployment by updating the restart annotation
func (c *Client) RollingRestart(ctx context.Context, namespace, name string) error {
	// Get the deployment
	deployment, err := c.Clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get deployment: %w", err)
	}

	// Add/update restart annotation to trigger rolling restart
	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = make(map[string]string)
	}

	// Update restart annotation with current timestamp to trigger rollout
	deployment.Spec.Template.Annotations["enclii.dev/restartedAt"] = metav1.Now().Format(time.RFC3339)
	deployment.Spec.Template.Annotations["enclii.dev/restartReason"] = "secret-rotation"

	// Update the deployment
	_, err = c.Clientset.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update deployment for rolling restart: %w", err)
	}

	return nil
}

// =============================================================================
// Metrics Collection (Real K8s Metrics Server Data)
// =============================================================================

// PodMetrics represents CPU and memory metrics for a pod
type PodMetrics struct {
	PodName     string
	Namespace   string
	Containers  []ContainerMetrics
	Timestamp   time.Time
	TotalCPU    int64 // millicores
	TotalMemory int64 // bytes
}

// ContainerMetrics represents metrics for a single container
type ContainerMetrics struct {
	Name   string
	CPU    int64 // millicores
	Memory int64 // bytes
}

// NamespaceMetrics represents aggregated metrics for a namespace
type NamespaceMetrics struct {
	Namespace   string
	PodCount    int
	TotalCPU    int64 // millicores
	TotalMemory int64 // bytes
	Pods        []PodMetrics
}

// ClusterMetrics represents aggregated metrics for the cluster
type ClusterMetrics struct {
	TotalCPU       int64 // millicores
	TotalMemory    int64 // bytes
	TotalPods      int
	Namespaces     map[string]*NamespaceMetrics
	CollectedAt    time.Time
	MetricsEnabled bool // whether metrics-server is available
}

// metricsAPIResponse represents the raw response from metrics.k8s.io API
type metricsAPIResponse struct {
	Kind       string `json:"kind"`
	APIVersion string `json:"apiVersion"`
	Items      []struct {
		Metadata struct {
			Name      string    `json:"name"`
			Namespace string    `json:"namespace"`
			CreatedAt time.Time `json:"creationTimestamp"`
		} `json:"metadata"`
		Timestamp  time.Time `json:"timestamp"`
		Window     string    `json:"window"`
		Containers []struct {
			Name  string `json:"name"`
			Usage struct {
				CPU    string `json:"cpu"`
				Memory string `json:"memory"`
			} `json:"usage"`
		} `json:"containers"`
	} `json:"items"`
}

// GetPodMetrics retrieves metrics for pods in a namespace
func (c *Client) GetPodMetrics(ctx context.Context, namespace string) ([]PodMetrics, error) {
	// Use the REST client to query metrics.k8s.io/v1beta1
	path := fmt.Sprintf("/apis/metrics.k8s.io/v1beta1/namespaces/%s/pods", namespace)
	result, err := c.Clientset.RESTClient().Get().AbsPath(path).DoRaw(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get pod metrics: %w", err)
	}

	var response metricsAPIResponse
	if err := json.Unmarshal(result, &response); err != nil {
		return nil, fmt.Errorf("failed to parse metrics response: %w", err)
	}

	metrics := make([]PodMetrics, 0, len(response.Items))
	for _, item := range response.Items {
		pm := PodMetrics{
			PodName:    item.Metadata.Name,
			Namespace:  item.Metadata.Namespace,
			Timestamp:  item.Timestamp,
			Containers: make([]ContainerMetrics, 0, len(item.Containers)),
		}

		for _, container := range item.Containers {
			cpu := parseResourceQuantity(container.Usage.CPU)
			memory := parseResourceQuantity(container.Usage.Memory)
			pm.Containers = append(pm.Containers, ContainerMetrics{
				Name:   container.Name,
				CPU:    cpu,
				Memory: memory,
			})
			pm.TotalCPU += cpu
			pm.TotalMemory += memory
		}
		metrics = append(metrics, pm)
	}

	return metrics, nil
}

// GetNamespaceMetrics retrieves aggregated metrics for a namespace
func (c *Client) GetNamespaceMetrics(ctx context.Context, namespace string) (*NamespaceMetrics, error) {
	pods, err := c.GetPodMetrics(ctx, namespace)
	if err != nil {
		return nil, err
	}

	nm := &NamespaceMetrics{
		Namespace: namespace,
		PodCount:  len(pods),
		Pods:      pods,
	}

	for _, pod := range pods {
		nm.TotalCPU += pod.TotalCPU
		nm.TotalMemory += pod.TotalMemory
	}

	return nm, nil
}

// GetClusterMetrics retrieves aggregated metrics for the entire cluster
func (c *Client) GetClusterMetrics(ctx context.Context) (*ClusterMetrics, error) {
	cm := &ClusterMetrics{
		Namespaces:     make(map[string]*NamespaceMetrics),
		CollectedAt:    time.Now(),
		MetricsEnabled: true,
	}

	// Get all pods metrics across all namespaces
	path := "/apis/metrics.k8s.io/v1beta1/pods"
	result, err := c.Clientset.RESTClient().Get().AbsPath(path).DoRaw(ctx)
	if err != nil {
		// Metrics server might not be available
		cm.MetricsEnabled = false
		return cm, nil
	}

	var response metricsAPIResponse
	if err := json.Unmarshal(result, &response); err != nil {
		return nil, fmt.Errorf("failed to parse metrics response: %w", err)
	}

	for _, item := range response.Items {
		ns := item.Metadata.Namespace

		// Initialize namespace if not exists
		if _, ok := cm.Namespaces[ns]; !ok {
			cm.Namespaces[ns] = &NamespaceMetrics{
				Namespace: ns,
				Pods:      make([]PodMetrics, 0),
			}
		}

		pm := PodMetrics{
			PodName:    item.Metadata.Name,
			Namespace:  ns,
			Timestamp:  item.Timestamp,
			Containers: make([]ContainerMetrics, 0, len(item.Containers)),
		}

		for _, container := range item.Containers {
			cpu := parseResourceQuantity(container.Usage.CPU)
			memory := parseResourceQuantity(container.Usage.Memory)
			pm.Containers = append(pm.Containers, ContainerMetrics{
				Name:   container.Name,
				CPU:    cpu,
				Memory: memory,
			})
			pm.TotalCPU += cpu
			pm.TotalMemory += memory
		}

		cm.Namespaces[ns].Pods = append(cm.Namespaces[ns].Pods, pm)
		cm.Namespaces[ns].PodCount++
		cm.Namespaces[ns].TotalCPU += pm.TotalCPU
		cm.Namespaces[ns].TotalMemory += pm.TotalMemory

		cm.TotalCPU += pm.TotalCPU
		cm.TotalMemory += pm.TotalMemory
		cm.TotalPods++
	}

	return cm, nil
}

// GetServiceMetrics retrieves metrics for pods matching a service's label selector
func (c *Client) GetServiceMetrics(ctx context.Context, namespace, serviceName string) (*NamespaceMetrics, error) {
	// Get pods for this service
	pods, err := c.ListPods(ctx, namespace, fmt.Sprintf("app=%s", serviceName))
	if err != nil {
		return nil, fmt.Errorf("failed to list pods for service: %w", err)
	}

	// Get all pod metrics for the namespace
	allMetrics, err := c.GetPodMetrics(ctx, namespace)
	if err != nil {
		return nil, err
	}

	// Create a set of pod names for this service
	podNames := make(map[string]bool)
	for _, pod := range pods.Items {
		podNames[pod.Name] = true
	}

	// Filter metrics to only include pods for this service
	nm := &NamespaceMetrics{
		Namespace: namespace,
		Pods:      make([]PodMetrics, 0),
	}

	for _, pm := range allMetrics {
		if podNames[pm.PodName] {
			nm.Pods = append(nm.Pods, pm)
			nm.TotalCPU += pm.TotalCPU
			nm.TotalMemory += pm.TotalMemory
			nm.PodCount++
		}
	}

	return nm, nil
}

// MetricsServerAvailable checks if the metrics-server is available in the cluster
func (c *Client) MetricsServerAvailable(ctx context.Context) bool {
	path := "/apis/metrics.k8s.io/v1beta1"
	_, err := c.Clientset.RESTClient().Get().AbsPath(path).DoRaw(ctx)
	return err == nil
}

// parseResourceQuantity parses a Kubernetes resource quantity string to int64
// For CPU: returns millicores (e.g., "500m" -> 500, "1" -> 1000)
// For Memory: returns bytes (e.g., "100Mi" -> 104857600)
func parseResourceQuantity(value string) int64 {
	if value == "" {
		return 0
	}

	quantity, err := resource.ParseQuantity(value)
	if err != nil {
		return 0
	}

	// For CPU, convert to millicores
	if strings.HasSuffix(value, "n") || strings.HasSuffix(value, "u") || strings.HasSuffix(value, "m") || !strings.ContainsAny(value, "KMGTPEkmgtpe") {
		return quantity.MilliValue()
	}

	// For Memory, return bytes
	return quantity.Value()
}
