package addons

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strconv"

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

// MySQL constants
const (
	DefaultMySQLVersion = "8.0"
	DefaultMySQLPort    = 3306
	MySQLDefaultUser    = "app"
	MySQLDefaultDB      = "app"
)

// MySQLProvisioner implements AddonProvisioner for MySQL
type MySQLProvisioner struct {
	k8sClient *k8s.Client
	logger    *logrus.Logger
}

// NewMySQLProvisioner creates a new MySQL provisioner
func NewMySQLProvisioner(k8sClient *k8s.Client, logger *logrus.Logger) *MySQLProvisioner {
	return &MySQLProvisioner{
		k8sClient: k8sClient,
		logger:    logger,
	}
}

// Provision creates a new MySQL instance using StatefulSet with PVC
func (p *MySQLProvisioner) Provision(ctx context.Context, req *ProvisionRequest) (*ProvisionResult, error) {
	addon := req.Addon
	namespace := req.Namespace

	// Generate resource name
	resourceName := fmt.Sprintf("mysql-%s", addon.ID.String()[:8])
	secretName := fmt.Sprintf("%s-secret", resourceName)

	logger := p.logger.WithFields(logrus.Fields{
		"addon_id":  addon.ID,
		"namespace": namespace,
		"resource":  resourceName,
	})

	logger.Info("Provisioning MySQL StatefulSet")

	// Ensure namespace exists
	if err := p.k8sClient.EnsureNamespace(ctx, namespace); err != nil {
		return nil, fmt.Errorf("failed to ensure namespace: %w", err)
	}

	// Parse config
	memory := addon.Config.Memory
	if memory == "" {
		memory = "512Mi"
	}

	cpu := addon.Config.CPU
	if cpu == "" {
		cpu = "250m"
	}

	storageSize := fmt.Sprintf("%dGi", addon.Config.StorageGB)
	if addon.Config.StorageGB == 0 {
		storageSize = DefaultStorageSize
	}

	version := addon.Config.Version
	if version == "" {
		version = DefaultMySQLVersion
	}

	// Generate root password and app user password
	rootPassword, err := generateSecurePassword(24)
	if err != nil {
		return nil, fmt.Errorf("failed to generate root password: %w", err)
	}

	appPassword, err := generateSecurePassword(24)
	if err != nil {
		return nil, fmt.Errorf("failed to generate app password: %w", err)
	}

	// Create secret with credentials
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":             resourceName,
				LabelManagedBy:    LabelManagedValue,
				LabelAddonID:      addon.ID.String(),
				LabelProjectID:    req.ProjectID.String(),
				LabelAddonType:    string(types.DatabaseAddonTypeMySQL),
				"enclii.dev/type": "addon",
				"enclii.dev/kind": "mysql",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"root-password": []byte(rootPassword),
			"username":      []byte(MySQLDefaultUser),
			"password":      []byte(appPassword),
			"database":      []byte(MySQLDefaultDB),
			"host":          []byte(fmt.Sprintf("%s.%s.svc.cluster.local", resourceName, namespace)),
			"port":          []byte(strconv.Itoa(DefaultMySQLPort)),
		},
	}

	_, err = p.k8sClient.Clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return nil, fmt.Errorf("failed to create MySQL secret: %w", err)
	}

	// Create headless service for StatefulSet
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourceName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":             resourceName,
				LabelManagedBy:    LabelManagedValue,
				LabelAddonID:      addon.ID.String(),
				"enclii.dev/type": "addon",
				"enclii.dev/kind": "mysql",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "mysql",
					Port:       DefaultMySQLPort,
					TargetPort: intstr.FromInt(DefaultMySQLPort),
				},
			},
			ClusterIP: "None", // Headless service for StatefulSet
			Selector: map[string]string{
				"app": resourceName,
			},
		},
	}

	_, err = p.k8sClient.Clientset.CoreV1().Services(namespace).Create(ctx, service, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return nil, fmt.Errorf("failed to create MySQL service: %w", err)
	}

	// Create StatefulSet
	replicas := int32(1) // MySQL standalone for now
	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourceName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":             resourceName,
				LabelManagedBy:    LabelManagedValue,
				LabelAddonID:      addon.ID.String(),
				LabelProjectID:    req.ProjectID.String(),
				"enclii.dev/type": "addon",
				"enclii.dev/kind": "mysql",
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
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "data",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse(storageSize),
							},
						},
					},
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":             resourceName,
						LabelManagedBy:    LabelManagedValue,
						LabelAddonID:      addon.ID.String(),
						"enclii.dev/type": "addon",
						"enclii.dev/kind": "mysql",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "mysql",
							Image: fmt.Sprintf("mysql:%s", version),
							Ports: []corev1.ContainerPort{
								{
									Name:          "mysql",
									ContainerPort: DefaultMySQLPort,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name: "MYSQL_ROOT_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: secretName,
											},
											Key: "root-password",
										},
									},
								},
								{
									Name: "MYSQL_USER",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: secretName,
											},
											Key: "username",
										},
									},
								},
								{
									Name: "MYSQL_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: secretName,
											},
											Key: "password",
										},
									},
								},
								{
									Name: "MYSQL_DATABASE",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: secretName,
											},
											Key: "database",
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "data",
									MountPath: "/var/lib/mysql",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse(memory),
									corev1.ResourceCPU:    resource.MustParse(cpu),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse(memory),
								},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									Exec: &corev1.ExecAction{
										Command: []string{
											"mysqladmin",
											"ping",
											"-h", "localhost",
											"-u", "root",
											"-p$(MYSQL_ROOT_PASSWORD)",
										},
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       10,
								TimeoutSeconds:      5,
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									Exec: &corev1.ExecAction{
										Command: []string{
											"mysqladmin",
											"ping",
											"-h", "localhost",
											"-u", "root",
											"-p$(MYSQL_ROOT_PASSWORD)",
										},
									},
								},
								InitialDelaySeconds: 60,
								PeriodSeconds:       15,
								TimeoutSeconds:      5,
							},
						},
					},
				},
			},
		},
	}

	_, err = p.k8sClient.Clientset.AppsV1().StatefulSets(namespace).Create(ctx, statefulSet, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return nil, fmt.Errorf("failed to create MySQL StatefulSet: %w", err)
	}

	logger.Info("MySQL StatefulSet created successfully")

	return &ProvisionResult{
		K8sResourceName:  resourceName,
		ConnectionSecret: secretName,
		Message:          "MySQL instance creation initiated",
	}, nil
}

// Deprovision removes a MySQL instance
func (p *MySQLProvisioner) Deprovision(ctx context.Context, addon *types.DatabaseAddon) error {
	if addon.K8sNamespace == "" || addon.K8sResourceName == "" {
		return nil // Nothing to deprovision
	}

	logger := p.logger.WithFields(logrus.Fields{
		"addon_id":  addon.ID,
		"namespace": addon.K8sNamespace,
		"resource":  addon.K8sResourceName,
	})

	logger.Info("Deprovisioning MySQL StatefulSet")

	// Delete StatefulSet
	err := p.k8sClient.Clientset.AppsV1().StatefulSets(addon.K8sNamespace).Delete(
		ctx,
		addon.K8sResourceName,
		metav1.DeleteOptions{},
	)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete MySQL StatefulSet: %w", err)
	}

	// Delete Service
	err = p.k8sClient.Clientset.CoreV1().Services(addon.K8sNamespace).Delete(
		ctx,
		addon.K8sResourceName,
		metav1.DeleteOptions{},
	)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete MySQL service: %w", err)
	}

	// Delete Secret
	secretName := fmt.Sprintf("%s-secret", addon.K8sResourceName)
	err = p.k8sClient.Clientset.CoreV1().Secrets(addon.K8sNamespace).Delete(
		ctx,
		secretName,
		metav1.DeleteOptions{},
	)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete MySQL secret: %w", err)
	}

	// Delete PVCs (StatefulSet doesn't delete them automatically)
	pvcName := fmt.Sprintf("data-%s-0", addon.K8sResourceName)
	err = p.k8sClient.Clientset.CoreV1().PersistentVolumeClaims(addon.K8sNamespace).Delete(
		ctx,
		pvcName,
		metav1.DeleteOptions{},
	)
	if err != nil && !errors.IsNotFound(err) {
		logger.WithError(err).Warn("Failed to delete MySQL PVC")
		// Don't fail the deprovisioning for PVC deletion
	}

	logger.Info("MySQL StatefulSet deprovisioned successfully")
	return nil
}

// GetStatus returns the current status of a MySQL instance
func (p *MySQLProvisioner) GetStatus(ctx context.Context, addon *types.DatabaseAddon) (*StatusResult, error) {
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
				StatusMessage: "MySQL StatefulSet not found",
			}, nil
		}
		return nil, fmt.Errorf("failed to get MySQL StatefulSet: %w", err)
	}

	result := &StatusResult{
		Host:         fmt.Sprintf("%s.%s.svc.cluster.local", addon.K8sResourceName, addon.K8sNamespace),
		Port:         DefaultMySQLPort,
		DatabaseName: MySQLDefaultDB,
		Username:     MySQLDefaultUser,
	}

	if statefulSet.Status.ReadyReplicas == *statefulSet.Spec.Replicas && statefulSet.Status.ReadyReplicas > 0 {
		result.Status = types.DatabaseAddonStatusReady
		result.StatusMessage = fmt.Sprintf("MySQL ready with %d replicas", statefulSet.Status.ReadyReplicas)
		result.Ready = true
	} else {
		result.Status = types.DatabaseAddonStatusProvisioning
		result.StatusMessage = fmt.Sprintf("MySQL provisioning: %d/%d replicas ready",
			statefulSet.Status.ReadyReplicas, *statefulSet.Spec.Replicas)
	}

	return result, nil
}

// GetCredentials returns connection credentials for a MySQL instance
func (p *MySQLProvisioner) GetCredentials(ctx context.Context, addon *types.DatabaseAddon) (*types.DatabaseAddonCredentials, error) {
	if addon.Status != types.DatabaseAddonStatusReady {
		return nil, fmt.Errorf("addon is not ready")
	}

	if addon.ConnectionSecret == "" || addon.K8sNamespace == "" {
		return nil, fmt.Errorf("addon does not have connection secret configured")
	}

	// Get the secret
	secret, err := p.k8sClient.Clientset.CoreV1().Secrets(addon.K8sNamespace).Get(
		ctx,
		addon.ConnectionSecret,
		metav1.GetOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection secret: %w", err)
	}

	host := string(secret.Data["host"])
	port := DefaultMySQLPort
	if portStr := string(secret.Data["port"]); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		}
	}

	username := string(secret.Data["username"])
	password := string(secret.Data["password"])
	database := string(secret.Data["database"])

	return &types.DatabaseAddonCredentials{
		Host:         host,
		Port:         port,
		DatabaseName: database,
		Username:     username,
		Password:     password,
		ConnectionURI: fmt.Sprintf(
			"mysql://%s:%s@%s:%d/%s",
			username,
			password,
			host,
			port,
			database,
		),
	}, nil
}

// GetConnectionURI returns the connection URI for a MySQL instance
func (p *MySQLProvisioner) GetConnectionURI(ctx context.Context, addon *types.DatabaseAddon) (string, error) {
	creds, err := p.GetCredentials(ctx, addon)
	if err != nil {
		return "", err
	}
	return creds.ConnectionURI, nil
}

// generateSecurePassword generates a cryptographically secure random password
func generateSecurePassword(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	// Use base64 URL encoding to get a string without special characters
	return base64.URLEncoding.EncodeToString(bytes)[:length], nil
}
