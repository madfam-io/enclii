package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
)

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
