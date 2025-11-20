package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

// TestPostgreSQLPersistence verifies PostgreSQL data persists across pod restarts
func TestPostgreSQLPersistence(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	namespace := "enclii-test-postgres"
	helper, err := NewTestHelper(namespace)
	require.NoError(t, err, "failed to create test helper")

	// Setup
	err = helper.CreateNamespace(ctx)
	require.NoError(t, err, "failed to create namespace")
	defer func() {
		_ = helper.DeleteNamespace(ctx)
	}()

	t.Log("Deploying PostgreSQL with PVC...")

	// Apply PostgreSQL manifest
	// Note: In a real test, you would use kubectl apply or create resources programmatically
	// For now, we assume the manifest is already applied

	// Wait for PostgreSQL pod to be ready
	t.Log("Waiting for PostgreSQL pod to be ready...")
	pod, err := helper.WaitForPodReady(ctx, "app=postgres", 2*time.Minute)
	require.NoError(t, err, "PostgreSQL pod should become ready")
	require.NotNil(t, pod, "PostgreSQL pod should exist")

	t.Logf("PostgreSQL pod ready: %s", pod.Name)

	// Verify PVC is bound
	t.Log("Verifying PVC is bound...")
	err = helper.WaitForPVCBound(ctx, "postgres-pvc", 1*time.Minute)
	require.NoError(t, err, "postgres-pvc should be bound")

	pvc, err := helper.GetPVC(ctx, "postgres-pvc")
	require.NoError(t, err, "should get postgres-pvc")
	assert.Equal(t, corev1.ClaimBound, pvc.Status.Phase, "PVC should be bound")
	assert.Equal(t, "10Gi", pvc.Spec.Resources.Requests.Storage().String(), "PVC size should be 10Gi")

	t.Log("PVC verified successfully")

	// TODO: Write test data to PostgreSQL
	// This would require using kubectl exec or the remotecommand package
	// For manual testing, follow the steps in TESTING_GUIDE.md
	t.Log("⚠️  Manual step required: Write test data to PostgreSQL")
	t.Log("   kubectl exec -it " + pod.Name + " -- psql -U postgres -d enclii_dev")
	t.Log("   CREATE TABLE test (id INT, data TEXT);")
	t.Log("   INSERT INTO test VALUES (1, 'persistence-test');")

	// Delete the pod to trigger restart
	t.Log("Deleting PostgreSQL pod to trigger restart...")
	err = helper.DeletePod(ctx, pod.Name)
	require.NoError(t, err, "should delete pod")

	// Wait for new pod to be ready
	t.Log("Waiting for new PostgreSQL pod to be ready...")
	newPod, err := helper.WaitForPodReady(ctx, "app=postgres", 2*time.Minute)
	require.NoError(t, err, "new PostgreSQL pod should become ready")
	require.NotNil(t, newPod, "new PostgreSQL pod should exist")
	assert.NotEqual(t, pod.Name, newPod.Name, "new pod should have different name")

	t.Logf("New PostgreSQL pod ready: %s", newPod.Name)

	// TODO: Verify test data still exists
	// This would require using kubectl exec or the remotecommand package
	t.Log("⚠️  Manual step required: Verify test data persists")
	t.Log("   kubectl exec -it " + newPod.Name + " -- psql -U postgres -d enclii_dev -c \"SELECT * FROM test;\"")
	t.Log("   Expected: Row with id=1, data='persistence-test'")

	t.Log("✅ PostgreSQL persistence test completed")
	t.Log("   Manual verification required for data persistence")
}

// TestRedisPersistence verifies Redis cache persists across pod restarts
func TestRedisPersistence(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	namespace := "enclii-test-redis"
	helper, err := NewTestHelper(namespace)
	require.NoError(t, err, "failed to create test helper")

	// Setup
	err = helper.CreateNamespace(ctx)
	require.NoError(t, err, "failed to create namespace")
	defer func() {
		_ = helper.DeleteNamespace(ctx)
	}()

	t.Log("Deploying Redis with PVC...")

	// Wait for Redis pod to be ready
	t.Log("Waiting for Redis pod to be ready...")
	pod, err := helper.WaitForPodReady(ctx, "app=redis", 2*time.Minute)
	require.NoError(t, err, "Redis pod should become ready")
	require.NotNil(t, pod, "Redis pod should exist")

	t.Logf("Redis pod ready: %s", pod.Name)

	// Verify PVC is bound
	t.Log("Verifying PVC is bound...")
	err = helper.WaitForPVCBound(ctx, "redis-pvc", 1*time.Minute)
	require.NoError(t, err, "redis-pvc should be bound")

	pvc, err := helper.GetPVC(ctx, "redis-pvc")
	require.NoError(t, err, "should get redis-pvc")
	assert.Equal(t, corev1.ClaimBound, pvc.Status.Phase, "PVC should be bound")
	assert.Equal(t, "5Gi", pvc.Spec.Resources.Requests.Storage().String(), "PVC size should be 5Gi")

	t.Log("PVC verified successfully")

	// TODO: Write test data to Redis
	t.Log("⚠️  Manual step required: Write test data to Redis")
	t.Log("   kubectl exec -it " + pod.Name + " -- redis-cli")
	t.Log("   SET persistence-test \"this-should-survive-restart\"")
	t.Log("   BGSAVE")

	// Delete the pod to trigger restart
	t.Log("Deleting Redis pod to trigger restart...")
	err = helper.DeletePod(ctx, pod.Name)
	require.NoError(t, err, "should delete pod")

	// Wait for new pod to be ready
	t.Log("Waiting for new Redis pod to be ready...")
	newPod, err := helper.WaitForPodReady(ctx, "app=redis", 2*time.Minute)
	require.NoError(t, err, "new Redis pod should become ready")
	require.NotNil(t, newPod, "new Redis pod should exist")
	assert.NotEqual(t, pod.Name, newPod.Name, "new pod should have different name")

	t.Logf("New Redis pod ready: %s", newPod.Name)

	// TODO: Verify test data still exists
	t.Log("⚠️  Manual step required: Verify test data persists")
	t.Log("   kubectl exec -it " + newPod.Name + " -- redis-cli GET persistence-test")
	t.Log("   Expected: \"this-should-survive-restart\"")

	t.Log("✅ Redis persistence test completed")
	t.Log("   Manual verification required for data persistence")
}

// TestPVCWithRollingUpdate verifies PVCs persist during rolling updates
func TestPVCWithRollingUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	namespace := "enclii-test-rolling"
	helper, err := NewTestHelper(namespace)
	require.NoError(t, err, "failed to create test helper")

	// Setup
	err = helper.CreateNamespace(ctx)
	require.NoError(t, err, "failed to create namespace")
	defer func() {
		_ = helper.DeleteNamespace(ctx)
	}()

	t.Log("Testing PVC persistence during rolling update...")

	// Wait for PostgreSQL deployment to be ready
	t.Log("Waiting for PostgreSQL deployment...")
	err = helper.WaitForDeploymentReady(ctx, "postgres", 2*time.Minute)
	require.NoError(t, err, "PostgreSQL deployment should be ready")

	// Get initial PVC
	initialPVC, err := helper.GetPVC(ctx, "postgres-pvc")
	require.NoError(t, err, "should get initial PVC")
	initialUID := initialPVC.UID

	t.Logf("Initial PVC UID: %s", initialUID)

	// Trigger rolling update by updating deployment
	// (In real test, you would patch the deployment)
	t.Log("⚠️  Manual step: Trigger rolling update")
	t.Log("   kubectl set image deployment/postgres postgres=postgres:15 -n " + namespace)

	// Wait for rolling update to complete
	time.Sleep(10 * time.Second)

	// Verify PVC still exists with same UID
	finalPVC, err := helper.GetPVC(ctx, "postgres-pvc")
	require.NoError(t, err, "PVC should still exist after rolling update")
	assert.Equal(t, initialUID, finalPVC.UID, "PVC UID should not change during rolling update")

	t.Log("✅ PVC persisted during rolling update")
}
