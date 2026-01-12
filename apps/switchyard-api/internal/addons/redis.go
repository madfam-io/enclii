package addons

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/madfam-org/enclii/apps/switchyard-api/internal/k8s"
	"github.com/madfam-org/enclii/packages/sdk-go/pkg/types"
)

// RedisProvisioner implements AddonProvisioner for Redis
type RedisProvisioner struct {
	k8sClient *k8s.Client
	logger    *logrus.Logger
}

// NewRedisProvisioner creates a new Redis provisioner
func NewRedisProvisioner(k8sClient *k8s.Client, logger *logrus.Logger) *RedisProvisioner {
	return &RedisProvisioner{
		k8sClient: k8sClient,
		logger:    logger,
	}
}

// Provision creates a new Redis instance using StatefulSet
func (p *RedisProvisioner) Provision(ctx context.Context, req *ProvisionRequest) (*ProvisionResult, error) {
	addon := req.Addon
	namespace := req.Namespace

	// Generate resource name
	resourceName := fmt.Sprintf("redis-%s", addon.ID.String()[:8])

	logger := p.logger.WithFields(logrus.Fields{
		"addon_id":  addon.ID,
		"namespace": namespace,
		"resource":  resourceName,
	})

	logger.Info("Provisioning Redis StatefulSet")

	// Parse config
	memory := addon.Config.Memory
	if memory == "" {
		memory = DefaultMemory
	}

	replicas := int32(addon.Config.Replicas)
	if replicas == 0 {
		replicas = 1
	}

	// Create headless service for StatefulSet
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourceName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":             resourceName,
				"enclii.dev/type": "addon",
				"enclii.dev/kind": "redis",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "redis",
					Port:       6379,
					TargetPort: intstr.FromInt(6379),
				},
			},
			ClusterIP: "None", // Headless service for StatefulSet
			Selector: map[string]string{
				"app": resourceName,
			},
		},
	}

	_, err := p.k8sClient.Clientset.CoreV1().Services(namespace).Create(ctx, service, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return nil, fmt.Errorf("failed to create Redis service: %w", err)
	}

	// Create StatefulSet
	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourceName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":             resourceName,
				"enclii.dev/type": "addon",
				"enclii.dev/kind": "redis",
			},
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: resourceName,
			Replicas:    &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": resourceName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":             resourceName,
						"enclii.dev/type": "addon",
						"enclii.dev/kind": "redis",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "redis",
							Image: "redis:7-alpine",
							Ports: []corev1.ContainerPort{
								{
									Name:          "redis",
									ContainerPort: 6379,
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse(memory),
									corev1.ResourceCPU:    resource.MustParse("50m"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse(memory),
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
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									Exec: &corev1.ExecAction{
										Command: []string{"redis-cli", "ping"},
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       10,
							},
						},
					},
				},
			},
		},
	}

	_, err = p.k8sClient.Clientset.AppsV1().StatefulSets(namespace).Create(ctx, statefulSet, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return nil, fmt.Errorf("failed to create Redis StatefulSet: %w", err)
	}

	logger.Info("Redis StatefulSet created successfully")

	return &ProvisionResult{
		K8sResourceName:  resourceName,
		ConnectionSecret: "", // Redis doesn't require a secret by default
	}, nil
}

// Deprovision removes a Redis instance
func (p *RedisProvisioner) Deprovision(ctx context.Context, addon *types.DatabaseAddon) error {
	if addon.K8sNamespace == "" || addon.K8sResourceName == "" {
		return nil // Nothing to deprovision
	}

	logger := p.logger.WithFields(logrus.Fields{
		"addon_id":  addon.ID,
		"namespace": addon.K8sNamespace,
		"resource":  addon.K8sResourceName,
	})

	logger.Info("Deprovisioning Redis StatefulSet")

	// Delete StatefulSet
	err := p.k8sClient.Clientset.AppsV1().StatefulSets(addon.K8sNamespace).Delete(
		ctx,
		addon.K8sResourceName,
		metav1.DeleteOptions{},
	)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete Redis StatefulSet: %w", err)
	}

	// Delete Service
	err = p.k8sClient.Clientset.CoreV1().Services(addon.K8sNamespace).Delete(
		ctx,
		addon.K8sResourceName,
		metav1.DeleteOptions{},
	)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete Redis service: %w", err)
	}

	logger.Info("Redis StatefulSet deprovisioned successfully")
	return nil
}

// GetStatus returns the current status of a Redis instance
func (p *RedisProvisioner) GetStatus(ctx context.Context, addon *types.DatabaseAddon) (*StatusResult, error) {
	if addon.K8sNamespace == "" || addon.K8sResourceName == "" {
		return &StatusResult{
			Status:        types.DatabaseAddonStatusPending,
			StatusMessage: "Waiting for K8s resource creation",
		}, nil
	}

	statefulSet, err := p.k8sClient.Clientset.AppsV1().StatefulSets(addon.K8sNamespace).Get(
		ctx,
		addon.K8sResourceName,
		metav1.GetOptions{},
	)
	if err != nil {
		if errors.IsNotFound(err) {
			return &StatusResult{
				Status:        types.DatabaseAddonStatusDeleted,
				StatusMessage: "Redis StatefulSet not found",
			}, nil
		}
		return nil, fmt.Errorf("failed to get Redis StatefulSet: %w", err)
	}

	result := &StatusResult{
		Host:         fmt.Sprintf("%s.%s.svc.cluster.local", addon.K8sResourceName, addon.K8sNamespace),
		Port:         6379,
		DatabaseName: "0", // Redis default DB
		Username:     "",
	}

	if statefulSet.Status.ReadyReplicas == *statefulSet.Spec.Replicas {
		result.Status = types.DatabaseAddonStatusReady
		result.StatusMessage = fmt.Sprintf("Redis ready with %d replicas", statefulSet.Status.ReadyReplicas)
		result.Ready = true
	} else {
		result.Status = types.DatabaseAddonStatusProvisioning
		result.StatusMessage = fmt.Sprintf("Redis provisioning: %d/%d replicas ready",
			statefulSet.Status.ReadyReplicas, *statefulSet.Spec.Replicas)
	}

	return result, nil
}

// GetCredentials returns connection credentials for a Redis instance
func (p *RedisProvisioner) GetCredentials(ctx context.Context, addon *types.DatabaseAddon) (*types.DatabaseAddonCredentials, error) {
	if addon.Status != types.DatabaseAddonStatusReady {
		return nil, fmt.Errorf("addon is not ready")
	}

	host := fmt.Sprintf("%s.%s.svc.cluster.local", addon.K8sResourceName, addon.K8sNamespace)

	return &types.DatabaseAddonCredentials{
		Host:         host,
		Port:         6379,
		DatabaseName: "0",
		Username:     "",
		Password:     "", // Redis without auth by default (can be enhanced)
		ConnectionURI: fmt.Sprintf("redis://%s:6379/0", host),
	}, nil
}

// GetConnectionURI returns the connection URI for a Redis instance
func (p *RedisProvisioner) GetConnectionURI(ctx context.Context, addon *types.DatabaseAddon) (string, error) {
	creds, err := p.GetCredentials(ctx, addon)
	if err != nil {
		return "", err
	}
	return creds.ConnectionURI, nil
}
