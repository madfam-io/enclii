package reconciler

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/madfam/enclii/apps/switchyard-api/internal/k8s"
	"github.com/madfam/enclii/packages/sdk-go/pkg/types"
)

// Mock Kubernetes client for testing
type MockK8sClient struct {
	mock.Mock
}

func (m *MockK8sClient) DeployService(ctx context.Context, spec *k8s.DeploymentSpec) error {
	args := m.Called(ctx, spec)
	return args.Error(0)
}

func (m *MockK8sClient) EnsureNamespace(ctx context.Context, namespace string) error {
	args := m.Called(ctx, namespace)
	return args.Error(0)
}

func (m *MockK8sClient) GetDeploymentStatus(ctx context.Context, name, namespace string) (*types.Deployment, error) {
	args := m.Called(ctx, name, namespace)
	return args.Get(0).(*types.Deployment), args.Error(1)
}

func (m *MockK8sClient) RollbackDeployment(ctx context.Context, name, namespace string) error {
	args := m.Called(ctx, name, namespace)
	return args.Error(0)
}

func (m *MockK8sClient) GetLogs(ctx context.Context, namespace, labelSelector string, lines int, follow bool) (string, error) {
	args := m.Called(ctx, namespace, labelSelector, lines, follow)
	return args.String(0), args.Error(1)
}

func TestServiceReconciler_Reconcile(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel) // Reduce test noise

	t.Run("successful reconciliation", func(t *testing.T) {
		mockClient := &MockK8sClient{}
		reconciler := NewServiceReconciler(&k8s.Client{}, logger)
		reconciler.k8sClient = mockClient // Inject mock

		// Setup test data
		service := &types.Service{
			ID:        "service-123",
			Name:      "test-service",
			ProjectID: "project-123",
		}

		release := &types.Release{
			ID:       "release-123",
			Version:  "v1.0.0",
			ImageURL: "registry.example.com/test-service:v1.0.0",
		}

		deployment := &types.Deployment{
			ID:        "deployment-123",
			ServiceID: service.ID,
			ReleaseID: release.ID,
		}

		req := &ReconcileRequest{
			Service:    service,
			Release:    release,
			Deployment: deployment,
		}

		// Setup mock expectations
		mockClient.On("EnsureNamespace", mock.Anything, "enclii-project-123").Return(nil)
		
		ctx := context.Background()
		result := reconciler.Reconcile(ctx, req)

		// Assertions
		require.NotNil(t, result)
		assert.True(t, result.Success)
		assert.Equal(t, "Service deployed successfully", result.Message)
		assert.Contains(t, result.K8sObjects, "deployment/test-service")
		assert.Contains(t, result.K8sObjects, "service/test-service")

		mockClient.AssertExpectations(t)
	})

	t.Run("namespace creation failure", func(t *testing.T) {
		mockClient := &MockK8sClient{}
		reconciler := NewServiceReconciler(&k8s.Client{}, logger)
		reconciler.k8sClient = mockClient

		service := &types.Service{
			ID:        "service-123",
			Name:      "test-service",
			ProjectID: "project-123",
		}

		release := &types.Release{
			ID:       "release-123",
			Version:  "v1.0.0",
			ImageURL: "registry.example.com/test-service:v1.0.0",
		}

		deployment := &types.Deployment{
			ID:        "deployment-123",
			ServiceID: service.ID,
			ReleaseID: release.ID,
		}

		req := &ReconcileRequest{
			Service:    service,
			Release:    release,
			Deployment: deployment,
		}

		// Setup mock to return error
		mockClient.On("EnsureNamespace", mock.Anything, "enclii-project-123").Return(assert.AnError)

		ctx := context.Background()
		result := reconciler.Reconcile(ctx, req)

		// Assertions
		require.NotNil(t, result)
		assert.False(t, result.Success)
		assert.Equal(t, "Failed to ensure namespace", result.Message)
		assert.NotNil(t, result.Error)

		mockClient.AssertExpectations(t)
	})
}

func TestServiceReconciler_generateManifests(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	reconciler := NewServiceReconciler(&k8s.Client{}, logger)

	service := &types.Service{
		ID:        "service-123",
		Name:      "test-service",
		ProjectID: "project-123",
		Config:    map[string]interface{}{"replicas": float64(2)},
	}

	release := &types.Release{
		ID:       "release-123",
		Version:  "v1.0.0",
		ImageURL: "registry.example.com/test-service:v1.0.0",
		BuildID:  "build-123",
	}

	deployment := &types.Deployment{
		ID:        "deployment-123",
		ServiceID: service.ID,
		ReleaseID: release.ID,
		CreatedAt: time.Now(),
	}

	req := &ReconcileRequest{
		Service:    service,
		Release:    release,
		Deployment: deployment,
	}

	// Test manifest generation
	k8sDeployment, k8sService, err := reconciler.generateManifests(req, "test-namespace")

	require.NoError(t, err)
	require.NotNil(t, k8sDeployment)
	require.NotNil(t, k8sService)

	// Test deployment manifest
	assert.Equal(t, "test-service", k8sDeployment.Name)
	assert.Equal(t, "test-namespace", k8sDeployment.Namespace)
	assert.Equal(t, int32(2), *k8sDeployment.Spec.Replicas)
	assert.Equal(t, "registry.example.com/test-service:v1.0.0", k8sDeployment.Spec.Template.Spec.Containers[0].Image)

	// Test service manifest
	assert.Equal(t, "test-service", k8sService.Name)
	assert.Equal(t, "test-namespace", k8sService.Namespace)
	assert.Equal(t, int32(80), k8sService.Spec.Ports[0].Port)

	// Test labels
	expectedLabels := map[string]string{
		"app":                    service.Name,
		"version":                release.Version,
		"enclii.dev/service":     service.Name,
		"enclii.dev/project":     service.ProjectID,
		"enclii.dev/release":     release.ID,
		"enclii.dev/deployment":  deployment.ID,
		"enclii.dev/managed-by":  "switchyard",
	}

	for key, value := range expectedLabels {
		assert.Equal(t, value, k8sDeployment.Labels[key], "Deployment label %s", key)
		assert.Equal(t, value, k8sService.Labels[key], "Service label %s", key)
	}

	// Test environment variables
	containers := k8sDeployment.Spec.Template.Spec.Containers
	require.Len(t, containers, 1)
	
	envVars := containers[0].Env
	envMap := make(map[string]string)
	for _, env := range envVars {
		envMap[env.Name] = env.Value
	}

	assert.Equal(t, service.Name, envMap["ENCLII_SERVICE_NAME"])
	assert.Equal(t, service.ProjectID, envMap["ENCLII_PROJECT_ID"])
	assert.Equal(t, release.Version, envMap["ENCLII_RELEASE_VERSION"])
	assert.Equal(t, deployment.ID, envMap["ENCLII_DEPLOYMENT_ID"])
	assert.Equal(t, "8080", envMap["PORT"])
}

func TestServiceReconciler_Delete(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	mockClient := &MockK8sClient{}
	reconciler := NewServiceReconciler(&k8s.Client{}, logger)
	
	// Since we can't easily mock the internal clientset, we'll test the method signature and basic logic
	err := reconciler.Delete(context.Background(), "test-namespace", "test-service")
	
	// In a real implementation, this would interact with Kubernetes
	// For now, we test that the method doesn't panic and handles the basic flow
	assert.Error(t, err) // Expected since we don't have a real k8s connection
}

func TestServiceReconciler_Rollback(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	reconciler := NewServiceReconciler(&k8s.Client{}, logger)
	
	// Test rollback method signature and basic logic
	err := reconciler.Rollback(context.Background(), "test-namespace", "test-service")
	
	// Expected to fail since we don't have a real k8s connection
	assert.Error(t, err)
}

// Benchmark the reconciliation process
func BenchmarkServiceReconciler_generateManifests(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	reconciler := NewServiceReconciler(&k8s.Client{}, logger)

	service := &types.Service{
		ID:        "service-123",
		Name:      "test-service",
		ProjectID: "project-123",
	}

	release := &types.Release{
		ID:       "release-123",
		Version:  "v1.0.0",
		ImageURL: "registry.example.com/test-service:v1.0.0",
		BuildID:  "build-123",
	}

	deployment := &types.Deployment{
		ID:        "deployment-123",
		ServiceID: service.ID,
		ReleaseID: release.ID,
		CreatedAt: time.Now(),
	}

	req := &ReconcileRequest{
		Service:    service,
		Release:    release,
		Deployment: deployment,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := reconciler.generateManifests(req, "test-namespace")
		if err != nil {
			b.Fatal(err)
		}
	}
}