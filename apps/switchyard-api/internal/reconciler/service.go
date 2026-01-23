package reconciler

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/madfam-org/enclii/apps/switchyard-api/internal/k8s"
	"github.com/madfam-org/enclii/packages/sdk-go/pkg/types"
)

// ServiceReconciler manages the lifecycle of services in Kubernetes
type ServiceReconciler struct {
	k8sClient *k8s.Client
	logger    *logrus.Logger
}

// EnvVarWithMeta represents an environment variable with metadata for K8s secret creation
type EnvVarWithMeta struct {
	Key      string
	Value    string
	IsSecret bool
}

type ReconcileRequest struct {
	Service         *types.Service
	Release         *types.Release
	Deployment      *types.Deployment
	Environment     *types.Environment // The target environment with kube_namespace
	CustomDomains   []types.CustomDomain
	Routes          []types.Route
	EnvVars         map[string]string // User-defined environment variables (decrypted) - DEPRECATED: use EnvVarsWithMeta
	EnvVarsWithMeta []EnvVarWithMeta  // Environment variables with IsSecret metadata for proper K8s secret creation
	AddonBindings   []AddonBinding    // Database addon bindings for env var injection
}

// AddonBinding represents a database addon bound to this service
type AddonBinding struct {
	EnvVarName       string                  // e.g., "DATABASE_URL", "REDIS_URL"
	AddonType        types.DatabaseAddonType // postgres, redis, mysql
	K8sNamespace     string                  // Namespace where addon resources exist
	K8sResourceName  string                  // Name of the addon K8s resource
	ConnectionSecret string                  // K8s secret name with credentials (for postgres)
}

type ReconcileResult struct {
	Success    bool
	Message    string
	K8sObjects []string
	NextCheck  *time.Time
	Error      error
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

	// Determine the Kubernetes namespace from the environment
	// The environment MUST have kube_namespace set - this is a data integrity requirement
	namespace := req.Environment.KubeNamespace
	if namespace == "" {
		// This is a data integrity issue - all environments should have kube_namespace set
		// during creation (via CreateEnvironment or auto-deploy)
		logger.Error("Environment has no kube_namespace set - this is a data integrity issue")
		return &ReconcileResult{
			Success: false,
			Message: "Environment has no kubernetes namespace configured",
			Error:   fmt.Errorf("missing kube_namespace for environment %s (ID: %s)", req.Environment.Name, req.Environment.ID),
		}
	}
	logger.WithField("namespace", namespace).Info("Using Kubernetes namespace for deployment")

	// Create namespace if it doesn't exist
	if err := r.ensureNamespace(ctx, namespace); err != nil {
		return &ReconcileResult{
			Success: false,
			Message: "Failed to ensure namespace",
			Error:   err,
		}
	}

	// Create PVCs if volumes are specified
	if len(req.Service.Volumes) > 0 {
		pvcs, err := r.generatePVCs(req, namespace)
		if err != nil {
			return &ReconcileResult{
				Success: false,
				Message: "Failed to generate PVCs",
				Error:   err,
			}
		}

		for _, pvc := range pvcs {
			if err := r.applyPVC(ctx, pvc); err != nil {
				return &ReconcileResult{
					Success: false,
					Message: fmt.Sprintf("Failed to apply PVC %s", pvc.Name),
					Error:   err,
				}
			}
		}
	}

	// Create K8s Secret for secret env vars (values not exposed in pod spec)
	secretName := fmt.Sprintf("%s-secrets", req.Service.Name)
	if err := r.ensureEnvSecret(ctx, req, namespace, secretName); err != nil {
		return &ReconcileResult{
			Success: false,
			Message: "Failed to create environment secrets",
			Error:   err,
		}
	}

	// Generate Kubernetes manifests
	deployment, service, err := r.generateManifests(req, namespace, secretName)
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

	// Apply Ingress if custom domains are configured
	k8sObjects := []string{
		fmt.Sprintf("deployment/%s", deployment.Name),
		fmt.Sprintf("service/%s", service.Name),
	}

	if len(req.CustomDomains) > 0 {
		ingress, err := r.generateIngress(req, namespace)
		if err != nil {
			return &ReconcileResult{
				Success: false,
				Message: "Failed to generate ingress",
				Error:   err,
			}
		}

		if err := r.applyIngress(ctx, ingress); err != nil {
			return &ReconcileResult{
				Success: false,
				Message: "Failed to apply ingress",
				Error:   err,
			}
		}

		k8sObjects = append(k8sObjects, fmt.Sprintf("ingress/%s", ingress.Name))
	}

	// Generate and apply NetworkPolicies for service isolation
	networkPolicies, err := r.generateNetworkPolicies(req, namespace)
	if err != nil {
		return &ReconcileResult{
			Success: false,
			Message: "Failed to generate network policies",
			Error:   err,
		}
	}

	for _, np := range networkPolicies {
		if err := r.applyNetworkPolicy(ctx, np); err != nil {
			return &ReconcileResult{
				Success: false,
				Message: fmt.Sprintf("Failed to apply network policy %s", np.Name),
				Error:   err,
			}
		}
		k8sObjects = append(k8sObjects, fmt.Sprintf("networkpolicy/%s", np.Name))
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
		Success:    true,
		Message:    "Service deployed successfully",
		K8sObjects: k8sObjects,
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

	created := false
	_, err := r.k8sClient.Clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create namespace %s: %w", namespace, err)
		}
	} else {
		created = true
		r.logger.WithField("namespace", namespace).Info("Created new namespace")
	}

	// GUARDRAIL: Copy registry credentials to new namespaces
	// This ensures pods can pull images from private registries (GHCR)
	if err := r.ensureRegistryCredentials(ctx, namespace); err != nil {
		// Log but don't fail - the credential check in triggerAutoDeploy is the primary guardrail
		r.logger.WithFields(logrus.Fields{
			"namespace": namespace,
			"created":   created,
		}).WithError(err).Warn("Failed to ensure registry credentials in namespace")
	}

	return nil
}

// ensureRegistryCredentials copies the registry credentials secret to the target namespace if missing
func (r *ServiceReconciler) ensureRegistryCredentials(ctx context.Context, targetNamespace string) error {
	const secretName = "enclii-registry-credentials"
	const sourceNamespace = "enclii"

	secretClient := r.k8sClient.Clientset.CoreV1().Secrets(targetNamespace)

	// Check if secret already exists
	_, err := secretClient.Get(ctx, secretName, metav1.GetOptions{})
	if err == nil {
		return nil // Already exists
	}
	if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check for registry credentials: %w", err)
	}

	// Get source secret
	sourceClient := r.k8sClient.Clientset.CoreV1().Secrets(sourceNamespace)
	sourceSecret, err := sourceClient.Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			r.logger.WithField("source_namespace", sourceNamespace).Warn("Source registry credentials not found - skipping copy")
			return nil // Source doesn't exist, nothing to copy
		}
		return fmt.Errorf("failed to get source registry credentials: %w", err)
	}

	// Create copy in target namespace
	newSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: targetNamespace,
			Labels: map[string]string{
				"enclii.dev/managed-by":  "switchyard-reconciler",
				"enclii.dev/copied-from": sourceNamespace,
			},
		},
		Type: sourceSecret.Type,
		Data: sourceSecret.Data,
	}

	_, err = secretClient.Create(ctx, newSecret, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			return nil // Race condition - another process created it
		}
		return fmt.Errorf("failed to create registry credentials: %w", err)
	}

	r.logger.WithFields(logrus.Fields{
		"namespace": targetNamespace,
		"secret":    secretName,
	}).Info("Copied registry credentials to namespace")

	return nil
}

func (r *ServiceReconciler) generateManifests(req *ReconcileRequest, namespace, secretName string) (*appsv1.Deployment, *corev1.Service, error) {
	labels := map[string]string{
		"app":                   req.Service.Name,
		"version":               req.Release.Version,
		"enclii.dev/service":    req.Service.Name,
		"enclii.dev/project":    req.Service.ProjectID.String(),
		"enclii.dev/release":    req.Release.ID.String(),
		"enclii.dev/deployment": req.Deployment.ID.String(),
		"enclii.dev/managed-by": "switchyard",
	}

	// Default configuration
	replicas := int32(1)

	// Determine the port to use (from ENCLII_PORT env var or default to 8080)
	containerPort, portErr := parseContainerPort(req.EnvVars)
	if portErr != nil {
		// Log the error but continue with default - this is a configuration issue
		logrus.WithFields(logrus.Fields{
			"service":      req.Service.Name,
			"enclii_port":  req.EnvVars["ENCLII_PORT"],
			"error":        portErr.Error(),
			"default_port": 8080,
		}).Warn("Invalid ENCLII_PORT value, using default port 4200")
	} else if _, ok := req.EnvVars["ENCLII_PORT"]; ok {
		logrus.WithFields(logrus.Fields{
			"service": req.Service.Name,
			"port":    containerPort,
		}).Info("Using ENCLII_PORT from environment variables")
	} else {
		logrus.WithFields(logrus.Fields{
			"service": req.Service.Name,
			"port":    containerPort,
		}).Debug("No ENCLII_PORT set, using default port 4200")
	}

	// Build environment variables
	var envVars []corev1.EnvVar

	// Add standard environment variables
	envVars = append(envVars, []corev1.EnvVar{
		{Name: "ENCLII_SERVICE_NAME", Value: req.Service.Name},
		{Name: "ENCLII_PROJECT_ID", Value: req.Service.ProjectID.String()},
		{Name: "ENCLII_RELEASE_VERSION", Value: req.Release.Version},
		{Name: "ENCLII_DEPLOYMENT_ID", Value: req.Deployment.ID.String()},
		{Name: "PORT", Value: strconv.Itoa(int(containerPort))}, // Use configured port
	}...)

	// Add user-defined environment variables (from database)
	// Secrets are referenced via K8s Secret, non-secrets are inline values
	hasSecrets := false
	if len(req.EnvVarsWithMeta) > 0 {
		// New path: use metadata-aware env vars
		for _, ev := range req.EnvVarsWithMeta {
			if ev.IsSecret {
				// Secret values are stored in K8s Secret, reference via secretKeyRef
				envVars = append(envVars, corev1.EnvVar{
					Name: ev.Key,
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: secretName,
							},
							Key: ev.Key,
						},
					},
				})
				hasSecrets = true
			} else {
				// Non-secret values are inline
				envVars = append(envVars, corev1.EnvVar{
					Name:  ev.Key,
					Value: ev.Value,
				})
			}
		}
	} else {
		// Legacy path: all values inline (backwards compatibility)
		for key, value := range req.EnvVars {
			envVars = append(envVars, corev1.EnvVar{
				Name:  key,
				Value: value,
			})
		}
	}

	// Log secret injection status
	if hasSecrets {
		logrus.WithFields(logrus.Fields{
			"service":     req.Service.Name,
			"secret_name": secretName,
		}).Info("Injecting secrets via K8s Secret reference")
	}

	// Add database addon environment variables (injected from bindings)
	addonEnvVars := buildAddonEnvVars(req.AddonBindings)
	envVars = append(envVars, addonEnvVars...)

	// Create deployment manifest
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Service.Name,
			Namespace: namespace,
			Labels:    labels,
			Annotations: map[string]string{
				"enclii.dev/git-sha":         req.Release.GitSHA,
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
						"enclii.dev/git-sha": req.Release.GitSHA,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  req.Service.Name,
							Image: req.Release.ImageURI,
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: containerPort,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Env:            envVars,
							Resources:      buildResourceRequirements(req.Service.Resources),
							LivenessProbe:  buildLivenessProbe(req.Service.HealthCheck, containerPort),
							ReadinessProbe: buildReadinessProbe(req.Service.HealthCheck, containerPort),
							VolumeMounts:   buildVolumeMountsWithKubeconfig(req.Service.Volumes, req.EnvVars),
						},
					},
					// ImagePullSecrets for private registries (GHCR, etc.)
					// This ensures pods can pull images that require authentication
					ImagePullSecrets: []corev1.LocalObjectReference{
						{Name: "enclii-registry-credentials"},
					},
					Volumes:                       buildVolumesWithKubeconfig(req.Service.Volumes, req.Service.Name, req.EnvVars),
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
					TargetPort: intstr.FromInt32(containerPort),
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

	// Update existing deployment - preserve the immutable selector
	// Kubernetes doesn't allow changing spec.selector on existing deployments
	deployment.ResourceVersion = existing.ResourceVersion
	deployment.Spec.Selector = existing.Spec.Selector

	// Also ensure pod template labels match the selector (required by k8s)
	// Preserve selector labels in pod template while adding our metadata labels
	for key, value := range existing.Spec.Selector.MatchLabels {
		deployment.Spec.Template.Labels[key] = value
	}

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

	// Update existing service (preserve cluster IP and selector)
	// Service selectors should generally match what the deployment is using
	service.ResourceVersion = existing.ResourceVersion
	service.Spec.ClusterIP = existing.Spec.ClusterIP

	// Preserve the existing selector to match the deployment's pods
	// Only use our new selector for new services
	if len(existing.Spec.Selector) > 0 {
		service.Spec.Selector = existing.Spec.Selector
	}

	_, err = serviceClient.Update(ctx, service, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update service: %w", err)
	}
	r.logger.WithField("service", service.Name).Info("Updated existing service")
	return nil
}

// ensureEnvSecret creates or updates a K8s Secret containing secret env vars
// This ensures sensitive values are not exposed in Pod specs and are stored encrypted in etcd
func (r *ServiceReconciler) ensureEnvSecret(ctx context.Context, req *ReconcileRequest, namespace, secretName string) error {
	// Collect secret values
	secretData := make(map[string][]byte)

	if len(req.EnvVarsWithMeta) > 0 {
		for _, ev := range req.EnvVarsWithMeta {
			if ev.IsSecret {
				secretData[ev.Key] = []byte(ev.Value)
			}
		}
	}

	// If no secrets, skip creating the secret
	if len(secretData) == 0 {
		r.logger.WithField("service", req.Service.Name).Debug("No secrets to inject, skipping K8s Secret creation")
		return nil
	}

	// Create K8s Secret resource
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":                   req.Service.Name,
				"enclii.dev/service":    req.Service.Name,
				"enclii.dev/project":    req.Service.ProjectID.String(),
				"enclii.dev/managed-by": "switchyard",
			},
			Annotations: map[string]string{
				"enclii.dev/deployment-id": req.Deployment.ID.String(),
				"enclii.dev/updated":       time.Now().Format(time.RFC3339),
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: secretData,
	}

	// Apply the secret (create or update)
	secretClient := r.k8sClient.Clientset.CoreV1().Secrets(namespace)

	existing, err := secretClient.Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// Create new secret
			_, err = secretClient.Create(ctx, secret, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create secret: %w", err)
			}
			r.logger.WithFields(logrus.Fields{
				"service":    req.Service.Name,
				"secret":     secretName,
				"keys_count": len(secretData),
			}).Info("Created K8s Secret for env vars")
			return nil
		}
		return fmt.Errorf("failed to get existing secret: %w", err)
	}

	// Update existing secret
	secret.ResourceVersion = existing.ResourceVersion
	_, err = secretClient.Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update secret: %w", err)
	}
	r.logger.WithFields(logrus.Fields{
		"service":    req.Service.Name,
		"secret":     secretName,
		"keys_count": len(secretData),
	}).Info("Updated K8s Secret for env vars")

	return nil
}

func (r *ServiceReconciler) waitForDeploymentReady(ctx context.Context, namespace, name string, timeout time.Duration) (bool, error) {
	deploymentClient := r.k8sClient.Clientset.AppsV1().Deployments(namespace)
	podClient := r.k8sClient.Clientset.CoreV1().Pods(namespace)

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

			// GUARDRAIL: Check for fatal pod conditions that won't self-heal
			// This provides early failure detection for issues like missing credentials
			pods, err := podClient.List(ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("app=%s", name),
			})
			if err == nil && len(pods.Items) > 0 {
				for _, pod := range pods.Items {
					if fatalErr := r.checkPodForFatalErrors(&pod); fatalErr != nil {
						r.logger.WithFields(logrus.Fields{
							"namespace":  namespace,
							"deployment": name,
							"pod":        pod.Name,
							"error":      fatalErr.Error(),
						}).Error("Pod has fatal error that won't self-heal")
						return false, fatalErr
					}
				}
			}

			time.Sleep(5 * time.Second)
		}
	}
}

// checkPodForFatalErrors examines a pod for conditions that indicate permanent failure
// Returns an error describing the fatal condition, or nil if the pod might still recover
func (r *ServiceReconciler) checkPodForFatalErrors(pod *corev1.Pod) error {
	// Check container statuses for fatal errors
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil {
			reason := cs.State.Waiting.Reason
			message := cs.State.Waiting.Message

			switch reason {
			case "ImagePullBackOff", "ErrImagePull":
				// Image pull failures are typically due to missing credentials or non-existent images
				// Check if it's a credentials issue (401/403)
				if strings.Contains(message, "401") || strings.Contains(message, "unauthorized") ||
					strings.Contains(message, "403") || strings.Contains(message, "forbidden") {
					return fmt.Errorf("image pull failed due to missing registry credentials: %s - ensure enclii-registry-credentials secret exists in namespace", message)
				}
				if strings.Contains(message, "not found") || strings.Contains(message, "manifest unknown") {
					return fmt.Errorf("image not found: %s - verify the image exists and tag is correct", message)
				}
				// After multiple backoffs, treat as fatal
				if cs.RestartCount > 0 || strings.Contains(reason, "BackOff") {
					return fmt.Errorf("image pull failed: %s - check registry credentials and image availability", message)
				}

			case "InvalidImageName":
				return fmt.Errorf("invalid image name: %s", message)

			case "CreateContainerConfigError":
				// Usually indicates missing secrets or configmaps
				if strings.Contains(message, "secret") {
					return fmt.Errorf("container config error - missing secret: %s", message)
				}
				return fmt.Errorf("container config error: %s", message)
			}
		}

		// Check for crash loops that indicate code/config issues
		if cs.State.Waiting != nil && cs.State.Waiting.Reason == "CrashLoopBackOff" {
			if cs.RestartCount >= 5 {
				return fmt.Errorf("container in CrashLoopBackOff after %d restarts - check application logs", cs.RestartCount)
			}
		}
	}

	return nil
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

	// Delete PVCs associated with this service
	pvcClient := r.k8sClient.Clientset.CoreV1().PersistentVolumeClaims(namespace)
	listOptions := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("enclii.dev/service=%s", serviceName),
	}
	pvcList, err := pvcClient.List(ctx, listOptions)
	if err != nil && !errors.IsNotFound(err) {
		r.logger.WithError(err).Warn("Failed to list PVCs for deletion")
	} else if pvcList != nil {
		for _, pvc := range pvcList.Items {
			err = pvcClient.Delete(ctx, pvc.Name, metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				r.logger.WithFields(logrus.Fields{
					"pvc": pvc.Name,
				}).WithError(err).Warn("Failed to delete PVC")
			}
		}
	}

	// Delete NetworkPolicies associated with this service
	npClient := r.k8sClient.Clientset.NetworkingV1().NetworkPolicies(namespace)
	npList, err := npClient.List(ctx, listOptions)
	if err != nil && !errors.IsNotFound(err) {
		r.logger.WithError(err).Warn("Failed to list NetworkPolicies for deletion")
	} else if npList != nil {
		for _, np := range npList.Items {
			err = npClient.Delete(ctx, np.Name, metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				r.logger.WithFields(logrus.Fields{
					"networkpolicy": np.Name,
				}).WithError(err).Warn("Failed to delete NetworkPolicy")
			}
		}
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

// parseContainerPort extracts and validates the container port from environment variables.
// Returns the port number (defaulting to 4200 per Enclii port allocation) and any validation error.
func parseContainerPort(envVars map[string]string) (int32, error) {
	const defaultPort int32 = 4200
	const minPort = 1
	const maxPort = 65535

	portStr, ok := envVars["ENCLII_PORT"]
	if !ok || portStr == "" {
		return defaultPort, nil
	}

	port, err := strconv.ParseInt(portStr, 10, 32)
	if err != nil {
		return defaultPort, fmt.Errorf("invalid ENCLII_PORT value '%s': %w", portStr, err)
	}

	if port < minPort || port > maxPort {
		return defaultPort, fmt.Errorf("ENCLII_PORT %d out of valid range (%d-%d)", port, minPort, maxPort)
	}

	return int32(port), nil
}

// buildResourceRequirements creates container resource requirements from config or defaults
func buildResourceRequirements(cfg *types.ResourceConfig) corev1.ResourceRequirements {
	// Default values
	cpuRequest := "100m"
	cpuLimit := "500m"
	memRequest := "128Mi"
	memLimit := "512Mi"

	if cfg != nil {
		if cfg.CPURequest != "" {
			cpuRequest = cfg.CPURequest
		}
		if cfg.CPULimit != "" {
			cpuLimit = cfg.CPULimit
		}
		if cfg.MemoryRequest != "" {
			memRequest = cfg.MemoryRequest
		}
		if cfg.MemoryLimit != "" {
			memLimit = cfg.MemoryLimit
		}
	}

	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    mustParseQuantity(cpuRequest),
			corev1.ResourceMemory: mustParseQuantity(memRequest),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    mustParseQuantity(cpuLimit),
			corev1.ResourceMemory: mustParseQuantity(memLimit),
		},
	}
}

// buildLivenessProbe creates a liveness probe from config or defaults
func buildLivenessProbe(cfg *types.HealthCheckConfig, containerPort int32) *corev1.Probe {
	// Check if probes are disabled
	if cfg != nil && cfg.Disabled {
		return nil
	}

	// Default values
	path := "/health"
	port := containerPort
	initialDelay := int32(30)
	timeout := int32(5)
	period := int32(10)
	failureThreshold := int32(3)

	if cfg != nil {
		if cfg.LivenessPath != "" {
			path = cfg.LivenessPath
		} else if cfg.Path != "" {
			path = cfg.Path
		}
		if cfg.Port > 0 {
			port = int32(cfg.Port)
		}
		if cfg.InitialDelaySeconds > 0 {
			initialDelay = int32(cfg.InitialDelaySeconds)
		}
		if cfg.TimeoutSeconds > 0 {
			timeout = int32(cfg.TimeoutSeconds)
		}
		if cfg.PeriodSeconds > 0 {
			period = int32(cfg.PeriodSeconds)
		}
		if cfg.FailureThreshold > 0 {
			failureThreshold = int32(cfg.FailureThreshold)
		}
	}

	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: path,
				Port: intstr.FromInt32(port),
			},
		},
		InitialDelaySeconds: initialDelay,
		TimeoutSeconds:      timeout,
		PeriodSeconds:       period,
		FailureThreshold:    failureThreshold,
	}
}

// buildReadinessProbe creates a readiness probe from config or defaults
func buildReadinessProbe(cfg *types.HealthCheckConfig, containerPort int32) *corev1.Probe {
	// Check if probes are disabled
	if cfg != nil && cfg.Disabled {
		return nil
	}

	// Default values
	path := "/health"
	port := containerPort
	initialDelay := int32(5)
	timeout := int32(3)
	period := int32(5)
	failureThreshold := int32(2)

	if cfg != nil {
		if cfg.ReadinessPath != "" {
			path = cfg.ReadinessPath
		} else if cfg.Path != "" {
			path = cfg.Path
		}
		if cfg.Port > 0 {
			port = int32(cfg.Port)
		}
		if cfg.InitialDelaySeconds > 0 {
			// For readiness, use a shorter initial delay if not explicitly set
			initialDelay = int32(cfg.InitialDelaySeconds)
		}
		if cfg.TimeoutSeconds > 0 {
			timeout = int32(cfg.TimeoutSeconds)
		}
		if cfg.PeriodSeconds > 0 {
			period = int32(cfg.PeriodSeconds)
		}
		if cfg.FailureThreshold > 0 {
			failureThreshold = int32(cfg.FailureThreshold)
		}
	}

	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: path,
				Port: intstr.FromInt32(port),
			},
		},
		InitialDelaySeconds: initialDelay,
		TimeoutSeconds:      timeout,
		PeriodSeconds:       period,
		FailureThreshold:    failureThreshold,
	}
}

// generatePVCs creates PersistentVolumeClaim manifests for service volumes
func (r *ServiceReconciler) generatePVCs(req *ReconcileRequest, namespace string) ([]*corev1.PersistentVolumeClaim, error) {
	var pvcs []*corev1.PersistentVolumeClaim

	labels := map[string]string{
		"app":                   req.Service.Name,
		"enclii.dev/service":    req.Service.Name,
		"enclii.dev/project":    req.Service.ProjectID.String(),
		"enclii.dev/managed-by": "switchyard",
	}

	for _, vol := range req.Service.Volumes {
		// Default values
		storageClassName := vol.StorageClassName
		if storageClassName == "" {
			storageClassName = "standard"
		}

		accessMode := corev1.PersistentVolumeAccessMode(vol.AccessMode)
		if accessMode == "" {
			accessMode = corev1.ReadWriteOnce
		}

		// Parse storage size
		storageSize, err := resource.ParseQuantity(vol.Size)
		if err != nil {
			return nil, fmt.Errorf("invalid storage size %s for volume %s: %w", vol.Size, vol.Name, err)
		}

		pvcName := fmt.Sprintf("%s-%s", req.Service.Name, vol.Name)

		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: namespace,
				Labels:    labels,
				Annotations: map[string]string{
					"enclii.dev/volume-name": vol.Name,
					"enclii.dev/mount-path":  vol.MountPath,
				},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{accessMode},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: storageSize,
					},
				},
				StorageClassName: &storageClassName,
			},
		}

		pvcs = append(pvcs, pvc)
	}

	return pvcs, nil
}

// applyPVC creates or updates a PersistentVolumeClaim
func (r *ServiceReconciler) applyPVC(ctx context.Context, pvc *corev1.PersistentVolumeClaim) error {
	pvcClient := r.k8sClient.Clientset.CoreV1().PersistentVolumeClaims(pvc.Namespace)

	// Try to get existing PVC
	existing, err := pvcClient.Get(ctx, pvc.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// Create new PVC
			_, err = pvcClient.Create(ctx, pvc, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create PVC: %w", err)
			}
			r.logger.WithField("pvc", pvc.Name).Info("Created new PVC")
			return nil
		}
		return fmt.Errorf("failed to get PVC: %w", err)
	}

	// PVC exists - PVCs are mostly immutable, only labels/annotations can be updated
	existing.Labels = pvc.Labels
	existing.Annotations = pvc.Annotations

	_, err = pvcClient.Update(ctx, existing, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update PVC: %w", err)
	}

	r.logger.WithField("pvc", pvc.Name).Info("Updated existing PVC")
	return nil
}

// buildVolumeMountsWithKubeconfig creates volume mounts including kubeconfig if needed
func buildVolumeMountsWithKubeconfig(volumes []types.Volume, envVars map[string]string) []corev1.VolumeMount {
	var volumeMounts []corev1.VolumeMount

	// Add PVC volume mounts
	for _, vol := range volumes {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      vol.Name,
			MountPath: vol.MountPath,
		})
	}

	// Add kubeconfig volume mount if ENCLII_KUBE_CONFIG is set
	if kubeconfigPath, ok := envVars["ENCLII_KUBE_CONFIG"]; ok && kubeconfigPath != "" {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "kubeconfig-cm",
			MountPath: "/etc/kubeconfig",
			ReadOnly:  true,
		})
	}

	return volumeMounts
}

// buildVolumesWithKubeconfig creates volumes including kubeconfig ConfigMap if needed
func buildVolumesWithKubeconfig(volumes []types.Volume, serviceName string, envVars map[string]string) []corev1.Volume {
	var podVolumes []corev1.Volume

	// Add PVC volumes
	for _, vol := range volumes {
		pvcName := fmt.Sprintf("%s-%s", serviceName, vol.Name)
		podVolumes = append(podVolumes, corev1.Volume{
			Name: vol.Name,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		})
	}

	// Add kubeconfig ConfigMap volume if ENCLII_KUBE_CONFIG is set
	if _, ok := envVars["ENCLII_KUBE_CONFIG"]; ok {
		podVolumes = append(podVolumes, corev1.Volume{
			Name: "kubeconfig-cm",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "switchyard-kubeconfig",
					},
				},
			},
		})
	}

	return podVolumes
}

// generateIngress creates an Ingress manifest for custom domains
func (r *ServiceReconciler) generateIngress(req *ReconcileRequest, namespace string) (*networkingv1.Ingress, error) {
	labels := map[string]string{
		"app":                   req.Service.Name,
		"enclii.dev/service":    req.Service.Name,
		"enclii.dev/project":    req.Service.ProjectID.String(),
		"enclii.dev/managed-by": "switchyard",
	}

	// Build ingress rules
	var rules []networkingv1.IngressRule
	var tlsConfigs []networkingv1.IngressTLS

	pathType := networkingv1.PathTypePrefix

	for _, domain := range req.CustomDomains {
		// Default path if no routes specified
		paths := []networkingv1.HTTPIngressPath{
			{
				Path:     "/",
				PathType: &pathType,
				Backend: networkingv1.IngressBackend{
					Service: &networkingv1.IngressServiceBackend{
						Name: req.Service.Name,
						Port: networkingv1.ServiceBackendPort{
							Number: 80,
						},
					},
				},
			},
		}

		// Override with custom routes if specified
		if len(req.Routes) > 0 {
			paths = []networkingv1.HTTPIngressPath{}
			for _, route := range req.Routes {
				routePathType := networkingv1.PathTypePrefix
				if route.PathType == "Exact" {
					routePathType = networkingv1.PathTypeExact
				} else if route.PathType == "ImplementationSpecific" {
					routePathType = networkingv1.PathTypeImplementationSpecific
				}

				paths = append(paths, networkingv1.HTTPIngressPath{
					Path:     route.Path,
					PathType: &routePathType,
					Backend: networkingv1.IngressBackend{
						Service: &networkingv1.IngressServiceBackend{
							Name: req.Service.Name,
							Port: networkingv1.ServiceBackendPort{
								Number: int32(route.Port),
							},
						},
					},
				})
			}
		}

		rules = append(rules, networkingv1.IngressRule{
			Host: domain.Domain,
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: paths,
				},
			},
		})

		// Add TLS configuration if enabled
		if domain.TLSEnabled {
			tlsIssuer := domain.TLSIssuer
			if tlsIssuer == "" {
				tlsIssuer = "letsencrypt-prod"
			}

			tlsConfigs = append(tlsConfigs, networkingv1.IngressTLS{
				Hosts:      []string{domain.Domain},
				SecretName: fmt.Sprintf("%s-%s-tls", req.Service.Name, sanitizeDomainForSecret(domain.Domain)),
			})
		}
	}

	// Determine cert-manager issuer
	tlsIssuer := "letsencrypt-prod"
	if len(req.CustomDomains) > 0 && req.CustomDomains[0].TLSIssuer != "" {
		tlsIssuer = req.CustomDomains[0].TLSIssuer
	}

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Service.Name,
			Namespace: namespace,
			Labels:    labels,
			Annotations: map[string]string{
				"kubernetes.io/ingress.class":                    "nginx",
				"cert-manager.io/cluster-issuer":                 tlsIssuer,
				"nginx.ingress.kubernetes.io/ssl-redirect":       "true",
				"nginx.ingress.kubernetes.io/force-ssl-redirect": "true",
			},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: stringPtr("nginx"),
			TLS:              tlsConfigs,
			Rules:            rules,
		},
	}

	return ingress, nil
}

// applyIngress creates or updates an Ingress
func (r *ServiceReconciler) applyIngress(ctx context.Context, ingress *networkingv1.Ingress) error {
	ingressClient := r.k8sClient.Clientset.NetworkingV1().Ingresses(ingress.Namespace)

	// Try to get existing ingress
	existing, err := ingressClient.Get(ctx, ingress.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// Create new ingress
			_, err = ingressClient.Create(ctx, ingress, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create ingress: %w", err)
			}
			r.logger.WithField("ingress", ingress.Name).Info("Created new ingress")
			return nil
		}
		return fmt.Errorf("failed to get ingress: %w", err)
	}

	// Update existing ingress
	existing.Labels = ingress.Labels
	existing.Annotations = ingress.Annotations
	existing.Spec = ingress.Spec

	_, err = ingressClient.Update(ctx, existing, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update ingress: %w", err)
	}

	r.logger.WithField("ingress", ingress.Name).Info("Updated existing ingress")
	return nil
}

// generateNetworkPolicies creates ingress and egress NetworkPolicy manifests for service isolation
func (r *ServiceReconciler) generateNetworkPolicies(req *ReconcileRequest, namespace string) ([]*networkingv1.NetworkPolicy, error) {
	labels := map[string]string{
		"app":                   req.Service.Name,
		"enclii.dev/service":    req.Service.Name,
		"enclii.dev/project":    req.Service.ProjectID.String(),
		"enclii.dev/managed-by": "switchyard",
	}

	podSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app":                req.Service.Name,
			"enclii.dev/service": req.Service.Name,
		},
	}

	// Determine the container port
	containerPort, _ := parseContainerPort(req.EnvVars)

	var policies []*networkingv1.NetworkPolicy

	// 1. Ingress Policy: Allow traffic only from ingress-nginx (Cloudflare Tunnel entry point)
	ingressPolicy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-ingress", req.Service.Name),
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: podSelector,
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					// Allow from ingress-nginx namespace (where cloudflared routes traffic)
					From: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"kubernetes.io/metadata.name": "ingress-nginx",
								},
							},
						},
						{
							// Also allow from cloudflare-tunnel namespace
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"kubernetes.io/metadata.name": "cloudflare-tunnel",
								},
							},
						},
						{
							// Allow traffic from same namespace (inter-service communication)
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"kubernetes.io/metadata.name": namespace,
								},
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: protocolPtr(corev1.ProtocolTCP),
							Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: containerPort},
						},
					},
				},
			},
		},
	}
	policies = append(policies, ingressPolicy)

	// 2. Egress Policy: Allow DNS, addon namespaces, and Kubernetes API
	egressRules := []networkingv1.NetworkPolicyEgressRule{
		// DNS egress (kube-dns in kube-system)
		{
			To: []networkingv1.NetworkPolicyPeer{
				{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"kubernetes.io/metadata.name": "kube-system",
						},
					},
					PodSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"k8s-app": "kube-dns",
						},
					},
				},
			},
			Ports: []networkingv1.NetworkPolicyPort{
				{Protocol: protocolPtr(corev1.ProtocolUDP), Port: &intstr.IntOrString{Type: intstr.Int, IntVal: 53}},
				{Protocol: protocolPtr(corev1.ProtocolTCP), Port: &intstr.IntOrString{Type: intstr.Int, IntVal: 53}},
			},
		},
		// Kubernetes API server (for services that need K8s access)
		// Internal cluster IPs (10.x.x.x)
		{
			To: []networkingv1.NetworkPolicyPeer{
				{
					IPBlock: &networkingv1.IPBlock{
						CIDR: "10.0.0.0/8",
					},
				},
			},
			Ports: []networkingv1.NetworkPolicyPort{
				{Protocol: protocolPtr(corev1.ProtocolTCP), Port: &intstr.IntOrString{Type: intstr.Int, IntVal: 443}},
				{Protocol: protocolPtr(corev1.ProtocolTCP), Port: &intstr.IntOrString{Type: intstr.Int, IntVal: 6443}},
			},
		},
		// External K8s API server (k3s single-node uses node's external IP)
		// Port 6443 is K8s API specific, safe to allow to any destination
		{
			Ports: []networkingv1.NetworkPolicyPort{
				{Protocol: protocolPtr(corev1.ProtocolTCP), Port: &intstr.IntOrString{Type: intstr.Int, IntVal: 6443}},
			},
		},
	}

	// Add egress rules for each addon binding (database access)
	for _, binding := range req.AddonBindings {
		addonPort := getAddonPort(binding.AddonType)
		egressRules = append(egressRules, networkingv1.NetworkPolicyEgressRule{
			To: []networkingv1.NetworkPolicyPeer{
				{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"kubernetes.io/metadata.name": binding.K8sNamespace,
						},
					},
				},
			},
			Ports: []networkingv1.NetworkPolicyPort{
				{Protocol: protocolPtr(corev1.ProtocolTCP), Port: &intstr.IntOrString{Type: intstr.Int, IntVal: addonPort}},
			},
		})
	}

	// Allow egress to data namespace (postgres, redis in shared data tier)
	egressRules = append(egressRules, networkingv1.NetworkPolicyEgressRule{
		To: []networkingv1.NetworkPolicyPeer{
			{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"kubernetes.io/metadata.name": "data",
					},
				},
			},
		},
		Ports: []networkingv1.NetworkPolicyPort{
			{Protocol: protocolPtr(corev1.ProtocolTCP), Port: &intstr.IntOrString{Type: intstr.Int, IntVal: 5432}}, // PostgreSQL
			{Protocol: protocolPtr(corev1.ProtocolTCP), Port: &intstr.IntOrString{Type: intstr.Int, IntVal: 6379}}, // Redis
		},
	})

	// Allow egress to same namespace (inter-service communication)
	egressRules = append(egressRules, networkingv1.NetworkPolicyEgressRule{
		To: []networkingv1.NetworkPolicyPeer{
			{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"kubernetes.io/metadata.name": namespace,
					},
				},
			},
		},
	})

	egressPolicy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-egress", req.Service.Name),
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: podSelector,
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
			Egress:      egressRules,
		},
	}
	policies = append(policies, egressPolicy)

	return policies, nil
}

// applyNetworkPolicy creates or updates a NetworkPolicy
func (r *ServiceReconciler) applyNetworkPolicy(ctx context.Context, np *networkingv1.NetworkPolicy) error {
	npClient := r.k8sClient.Clientset.NetworkingV1().NetworkPolicies(np.Namespace)

	// Try to get existing NetworkPolicy
	existing, err := npClient.Get(ctx, np.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// Create new NetworkPolicy
			_, err = npClient.Create(ctx, np, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create network policy: %w", err)
			}
			r.logger.WithField("networkpolicy", np.Name).Info("Created new network policy")
			return nil
		}
		return fmt.Errorf("failed to get network policy: %w", err)
	}

	// Update existing NetworkPolicy
	np.ResourceVersion = existing.ResourceVersion
	_, err = npClient.Update(ctx, np, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update network policy: %w", err)
	}

	r.logger.WithField("networkpolicy", np.Name).Info("Updated existing network policy")
	return nil
}

// protocolPtr returns a pointer to the given Protocol
func protocolPtr(p corev1.Protocol) *corev1.Protocol {
	return &p
}

// getAddonPort returns the default port for a database addon type
func getAddonPort(addonType types.DatabaseAddonType) int32 {
	switch addonType {
	case types.DatabaseAddonTypePostgres:
		return 5432
	case types.DatabaseAddonTypeRedis:
		return 6379
	case types.DatabaseAddonTypeMySQL:
		return 3306
	default:
		return 5432 // Default to PostgreSQL port
	}
}

// sanitizeDomainForSecret converts a domain name to a valid Kubernetes secret name
func sanitizeDomainForSecret(domain string) string {
	// Replace dots with dashes for valid secret name
	result := ""
	for _, char := range domain {
		if char == '.' {
			result += "-"
		} else {
			result += string(char)
		}
	}
	return result
}

// stringPtr returns a pointer to the given string
func stringPtr(s string) *string {
	return &s
}

// GetPodEnvVars retrieves environment variables from a running pod
func (r *ServiceReconciler) GetPodEnvVars(ctx context.Context, namespace, podName string) (map[string]string, error) {
	podClient := r.k8sClient.Clientset.CoreV1().Pods(namespace)

	pod, err := podClient.Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get pod: %w", err)
	}

	envVars := make(map[string]string)

	// Extract env vars from all containers
	for _, container := range pod.Spec.Containers {
		for _, env := range container.Env {
			// Skip vars that reference secrets or configmaps (can't read the actual value)
			if env.ValueFrom != nil {
				continue
			}
			envVars[env.Name] = env.Value
		}
	}

	return envVars, nil
}

// buildAddonEnvVars creates environment variables for database addon bindings
// For PostgreSQL: References the CloudNativePG-generated secret
// For Redis: Uses direct connection URL (no authentication by default)
func buildAddonEnvVars(bindings []AddonBinding) []corev1.EnvVar {
	var envVars []corev1.EnvVar

	for _, binding := range bindings {
		switch binding.AddonType {
		case types.DatabaseAddonTypePostgres:
			// CloudNativePG creates a secret named "<cluster>-app" with the connection URI
			secretName := binding.ConnectionSecret
			if secretName == "" {
				// Default CloudNativePG naming convention
				secretName = fmt.Sprintf("%s-app", binding.K8sResourceName)
			}

			envVars = append(envVars, corev1.EnvVar{
				Name: binding.EnvVarName,
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretName,
						},
						Key: "uri",
					},
				},
			})

		case types.DatabaseAddonTypeRedis:
			// Redis uses direct connection URL (no secret needed for basic setup)
			redisURL := fmt.Sprintf("redis://%s.%s.svc.cluster.local:6379/0",
				binding.K8sResourceName, binding.K8sNamespace)

			envVars = append(envVars, corev1.EnvVar{
				Name:  binding.EnvVarName,
				Value: redisURL,
			})

		case types.DatabaseAddonTypeMySQL:
			// MySQL secret reference (similar to PostgreSQL)
			secretName := binding.ConnectionSecret
			if secretName == "" {
				secretName = fmt.Sprintf("%s-credentials", binding.K8sResourceName)
			}

			envVars = append(envVars, corev1.EnvVar{
				Name: binding.EnvVarName,
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretName,
						},
						Key: "uri",
					},
				},
			})
		}
	}

	return envVars
}
