package reconciler

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/madfam/enclii/apps/switchyard-api/internal/k8s"
	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
)

// ServiceReconciler manages the lifecycle of services in Kubernetes
type ServiceReconciler struct {
	k8sClient *k8s.Client
	logger    *logrus.Logger
}

type ReconcileRequest struct {
	Service    *types.Service
	Release    *types.Release
	Deployment *types.Deployment
}

type ReconcileResult struct {
	Success     bool
	Message     string
	K8sObjects  []string
	NextCheck   *time.Time
	Error       error
}

func NewServiceReconciler(k8sClient *k8s.Client, logger *logrus.Logger) *ServiceReconciler {
	return &ServiceReconciler{
		k8sClient: k8sClient,
		logger:    logger,
	}
}

// Reconcile ensures the desired state matches the actual state in Kubernetes
func (r *ServiceReconciler) Reconcile(ctx context.Context, req *ReconcileRequest) *ReconcileResult {
	logger := r.logger.WithFields(logrus.Fields{
		"service":    req.Service.Name,
		"release":    req.Release.Version,
		"deployment": req.Deployment.ID,
	})

	logger.Info("Starting service reconciliation")

	// Create namespace if it doesn't exist
	namespace := fmt.Sprintf("enclii-%s", req.Service.ProjectID)
	if err := r.ensureNamespace(ctx, namespace); err != nil {
		return &ReconcileResult{
			Success: false,
			Message: "Failed to ensure namespace",
			Error:   err,
		}
	}

	// Generate Kubernetes manifests
	deployment, service, err := r.generateManifests(req, namespace)
	if err != nil {
		return &ReconcileResult{
			Success: false,
			Message: "Failed to generate manifests",
			Error:   err,
		}
	}

	// Apply deployment
	if err := r.applyDeployment(ctx, deployment); err != nil {
		return &ReconcileResult{
			Success: false,
			Message: "Failed to apply deployment",
			Error:   err,
		}
	}

	// Apply service
	if err := r.applyService(ctx, service); err != nil {
		return &ReconcileResult{
			Success: false,
			Message: "Failed to apply service",
			Error:   err,
		}
	}

	// Wait for deployment to be ready
	ready, err := r.waitForDeploymentReady(ctx, deployment.Namespace, deployment.Name, 5*time.Minute)
	if err != nil {
		return &ReconcileResult{
			Success: false,
			Message: "Failed to wait for deployment readiness",
			Error:   err,
		}
	}

	if !ready {
		nextCheck := time.Now().Add(30 * time.Second)
		return &ReconcileResult{
			Success:   false,
			Message:   "Deployment not ready, will retry",
			NextCheck: &nextCheck,
		}
	}

	logger.Info("Service reconciliation completed successfully")

	return &ReconcileResult{
		Success: true,
		Message: "Service deployed successfully",
		K8sObjects: []string{
			fmt.Sprintf("deployment/%s", deployment.Name),
			fmt.Sprintf("service/%s", service.Name),
		},
	}
}

func (r *ServiceReconciler) ensureNamespace(ctx context.Context, namespace string) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
			Labels: map[string]string{
				"managed-by": "enclii",
				"platform":   "enclii",
			},
		},
	}

	_, err := r.k8sClient.Clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create namespace %s: %w", namespace, err)
	}

	return nil
}

func (r *ServiceReconciler) generateManifests(req *ReconcileRequest, namespace string) (*appsv1.Deployment, *corev1.Service, error) {
	labels := map[string]string{
		"app":                    req.Service.Name,
		"version":                req.Release.Version,
		"enclii.dev/service":     req.Service.Name,
		"enclii.dev/project":     req.Service.ProjectID,
		"enclii.dev/release":     req.Release.ID,
		"enclii.dev/deployment":  req.Deployment.ID,
		"enclii.dev/managed-by":  "switchyard",
	}

	// Parse service configuration
	config := req.Service.Config
	replicas := int32(1)
	if config != nil {
		if r, ok := config["replicas"].(float64); ok {
			replicas = int32(r)
		}
	}

	// Build environment variables
	var envVars []corev1.EnvVar
	for key, value := range req.Release.Environment {
		envVars = append(envVars, corev1.EnvVar{
			Name:  key,
			Value: fmt.Sprintf("%v", value),
		})
	}

	// Add standard environment variables
	envVars = append(envVars, []corev1.EnvVar{
		{Name: "ENCLII_SERVICE_NAME", Value: req.Service.Name},
		{Name: "ENCLII_PROJECT_ID", Value: req.Service.ProjectID},
		{Name: "ENCLII_RELEASE_VERSION", Value: req.Release.Version},
		{Name: "ENCLII_DEPLOYMENT_ID", Value: req.Deployment.ID},
		{Name: "PORT", Value: "8080"}, // Default port
	}...)

	// Create deployment manifest
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Service.Name,
			Namespace: namespace,
			Labels:    labels,
			Annotations: map[string]string{
				"enclii.dev/build-id":        req.Release.BuildID,
				"enclii.dev/deployment-time": req.Deployment.CreatedAt.Format(time.RFC3339),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":                req.Service.Name,
					"enclii.dev/service": req.Service.Name,
				},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxUnavailable: &intstr.IntOrString{Type: intstr.String, StrVal: "25%"},
					MaxSurge:       &intstr.IntOrString{Type: intstr.String, StrVal: "25%"},
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
					Annotations: map[string]string{
						"enclii.dev/build-id": req.Release.BuildID,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  req.Service.Name,
							Image: req.Release.ImageURL,
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 8080,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Env: envVars,
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    mustParseQuantity("100m"),
									corev1.ResourceMemory: mustParseQuantity("128Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    mustParseQuantity("500m"),
									corev1.ResourceMemory: mustParseQuantity("512Mi"),
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/health",
										Port: intstr.FromInt(8080),
									},
								},
								InitialDelaySeconds: 30,
								TimeoutSeconds:      5,
								PeriodSeconds:       10,
								FailureThreshold:    3,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/health/ready",
										Port: intstr.FromInt(8080),
									},
								},
								InitialDelaySeconds: 5,
								TimeoutSeconds:      3,
								PeriodSeconds:       5,
								FailureThreshold:    2,
							},
						},
					},
					RestartPolicy:                 corev1.RestartPolicyAlways,
					TerminationGracePeriodSeconds: &[]int64{30}[0],
				},
			},
		},
	}

	// Create service manifest
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Service.Name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app":                req.Service.Name,
				"enclii.dev/service": req.Service.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.FromInt(8080),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}

	return deployment, service, nil
}

func (r *ServiceReconciler) applyDeployment(ctx context.Context, deployment *appsv1.Deployment) error {
	deploymentClient := r.k8sClient.Clientset.AppsV1().Deployments(deployment.Namespace)

	// Try to get existing deployment
	existing, err := deploymentClient.Get(ctx, deployment.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// Create new deployment
			_, err = deploymentClient.Create(ctx, deployment, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create deployment: %w", err)
			}
			r.logger.WithField("deployment", deployment.Name).Info("Created new deployment")
			return nil
		}
		return fmt.Errorf("failed to get existing deployment: %w", err)
	}

	// Update existing deployment
	deployment.ResourceVersion = existing.ResourceVersion
	_, err = deploymentClient.Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update deployment: %w", err)
	}
	r.logger.WithField("deployment", deployment.Name).Info("Updated existing deployment")
	return nil
}

func (r *ServiceReconciler) applyService(ctx context.Context, service *corev1.Service) error {
	serviceClient := r.k8sClient.Clientset.CoreV1().Services(service.Namespace)

	// Try to get existing service
	existing, err := serviceClient.Get(ctx, service.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// Create new service
			_, err = serviceClient.Create(ctx, service, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create service: %w", err)
			}
			r.logger.WithField("service", service.Name).Info("Created new service")
			return nil
		}
		return fmt.Errorf("failed to get existing service: %w", err)
	}

	// Update existing service (preserve cluster IP)
	service.ResourceVersion = existing.ResourceVersion
	service.Spec.ClusterIP = existing.Spec.ClusterIP
	_, err = serviceClient.Update(ctx, service, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update service: %w", err)
	}
	r.logger.WithField("service", service.Name).Info("Updated existing service")
	return nil
}

func (r *ServiceReconciler) waitForDeploymentReady(ctx context.Context, namespace, name string, timeout time.Duration) (bool, error) {
	deploymentClient := r.k8sClient.Clientset.AppsV1().Deployments(namespace)
	
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
			deployment, err := deploymentClient.Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}

			// Check if deployment is ready
			if deployment.Status.ReadyReplicas == *deployment.Spec.Replicas &&
				deployment.Status.UpdatedReplicas == *deployment.Spec.Replicas {
				return true, nil
			}

			time.Sleep(5 * time.Second)
		}
	}
}

// Rollback rolls back a deployment to the previous version
func (r *ServiceReconciler) Rollback(ctx context.Context, namespace, serviceName string) error {
	deploymentClient := r.k8sClient.Clientset.AppsV1().Deployments(namespace)
	
	// Get the deployment
	deployment, err := deploymentClient.Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get deployment: %w", err)
	}

	// Trigger rollback by updating the rollback annotation
	if deployment.Annotations == nil {
		deployment.Annotations = make(map[string]string)
	}
	deployment.Annotations["deployment.kubernetes.io/revision"] = "0"

	_, err = deploymentClient.Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to trigger rollback: %w", err)
	}

	r.logger.WithFields(logrus.Fields{
		"namespace":  namespace,
		"deployment": serviceName,
	}).Info("Triggered deployment rollback")

	return nil
}

// Delete removes all Kubernetes resources for a service
func (r *ServiceReconciler) Delete(ctx context.Context, namespace, serviceName string) error {
	// Delete deployment
	deploymentClient := r.k8sClient.Clientset.AppsV1().Deployments(namespace)
	err := deploymentClient.Delete(ctx, serviceName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete deployment: %w", err)
	}

	// Delete service
	serviceClient := r.k8sClient.Clientset.CoreV1().Services(namespace)
	err = serviceClient.Delete(ctx, serviceName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete service: %w", err)
	}

	r.logger.WithFields(logrus.Fields{
		"namespace": namespace,
		"service":   serviceName,
	}).Info("Deleted service resources")

	return nil
}

// Helper function to parse Kubernetes resource quantities
func mustParseQuantity(s string) resource.Quantity {
	return resource.MustParse(s)
}