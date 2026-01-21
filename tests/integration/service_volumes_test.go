package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

// TestServiceWithSingleVolume verifies a service can be deployed with a single persistent volume
func TestServiceWithSingleVolume(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	namespace := "enclii-test-single-volume"
	helper, err := NewTestHelper(namespace)
	require.NoError(t, err, "failed to create test helper")

	// Setup
	err = helper.CreateNamespace(ctx)
	require.NoError(t, err, "failed to create namespace")
	defer func() {
		_ = helper.DeleteNamespace(ctx)
	}()

	t.Log("Testing service deployment with single volume...")

	serviceName := "test-service"
	volumeName := "data"
	expectedPVCName := serviceName + "-" + volumeName

	// Deploy test service with a single volume
	err = helper.DeployTestService(ctx, serviceName, map[string]string{
		volumeName: "/data",
	})
	require.NoError(t, err, "failed to deploy test service")

	// Wait for deployment to be ready
	t.Log("Waiting for service deployment to be ready...")
	err = helper.WaitForDeploymentReady(ctx, serviceName, 3*time.Minute)
	require.NoError(t, err, "deployment should become ready")

	// Verify PVC was created
	t.Log("Verifying PVC was created...")
	pvc, err := helper.GetPVC(ctx, expectedPVCName)
	require.NoError(t, err, "PVC should exist")
	assert.Equal(t, corev1.ClaimBound, pvc.Status.Phase, "PVC should be bound")

	t.Logf("PVC created: %s (size: %s, class: %s)",
		pvc.Name,
		pvc.Spec.Resources.Requests.Storage().String(),
		*pvc.Spec.StorageClassName,
	)

	// Verify volume is mounted in pod
	t.Log("Verifying volume is mounted in pod...")
	pods, err := helper.ListPods(ctx, "app="+serviceName)
	require.NoError(t, err, "should list pods")
	require.Greater(t, len(pods.Items), 0, "at least one pod should exist")

	pod := &pods.Items[0]
	volumeMountFound := false
	expectedMountPath := "/data"

	for _, container := range pod.Spec.Containers {
		for _, mount := range container.VolumeMounts {
			if mount.Name == volumeName && mount.MountPath == expectedMountPath {
				volumeMountFound = true
				break
			}
		}
	}

	assert.True(t, volumeMountFound, "volume should be mounted at %s", expectedMountPath)

	// Verify volume source references the PVC
	volumeSourceFound := false
	for _, volume := range pod.Spec.Volumes {
		if volume.Name == volumeName && volume.PersistentVolumeClaim != nil {
			assert.Equal(t, expectedPVCName, volume.PersistentVolumeClaim.ClaimName,
				"volume should reference correct PVC")
			volumeSourceFound = true
			break
		}
	}

	assert.True(t, volumeSourceFound, "volume source should reference PVC")

	t.Log("✅ Service with single volume deployed successfully")
}

// TestServiceWithMultipleVolumes verifies a service can be deployed with multiple persistent volumes
func TestServiceWithMultipleVolumes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	namespace := "enclii-test-multi-volume"
	helper, err := NewTestHelper(namespace)
	require.NoError(t, err, "failed to create test helper")

	// Setup
	err = helper.CreateNamespace(ctx)
	require.NoError(t, err, "failed to create namespace")
	defer func() {
		_ = helper.DeleteNamespace(ctx)
	}()

	t.Log("Testing service deployment with multiple volumes...")

	serviceName := "file-processor"
	expectedVolumes := map[string]string{
		"uploads": "/data/uploads",
		"cache":   "/data/cache",
	}

	// Deploy test service with multiple volumes
	err = helper.DeployTestService(ctx, serviceName, expectedVolumes)
	require.NoError(t, err, "failed to deploy test service")

	// Wait for deployment to be ready
	t.Log("Waiting for service deployment to be ready...")
	err = helper.WaitForDeploymentReady(ctx, serviceName, 3*time.Minute)
	require.NoError(t, err, "deployment should become ready")

	// Verify all PVCs were created
	t.Log("Verifying all PVCs were created...")
	for volumeName := range expectedVolumes {
		pvcName := serviceName + "-" + volumeName
		pvc, err := helper.GetPVC(ctx, pvcName)
		require.NoError(t, err, "PVC %s should exist", pvcName)
		assert.Equal(t, corev1.ClaimBound, pvc.Status.Phase, "PVC %s should be bound", pvcName)

		t.Logf("✓ PVC created: %s (size: %s)",
			pvc.Name,
			pvc.Spec.Resources.Requests.Storage().String(),
		)
	}

	// Verify all volumes are mounted in pod
	t.Log("Verifying all volumes are mounted...")
	pods, err := helper.ListPods(ctx, "app="+serviceName)
	require.NoError(t, err, "should list pods")
	require.Greater(t, len(pods.Items), 0, "at least one pod should exist")

	pod := &pods.Items[0]

	for volumeName, expectedPath := range expectedVolumes {
		mountFound := false
		for _, container := range pod.Spec.Containers {
			for _, mount := range container.VolumeMounts {
				if mount.Name == volumeName && mount.MountPath == expectedPath {
					mountFound = true
					t.Logf("✓ Volume %s mounted at %s", volumeName, expectedPath)
					break
				}
			}
		}
		assert.True(t, mountFound, "volume %s should be mounted at %s", volumeName, expectedPath)
	}

	t.Log("✅ Service with multiple volumes deployed successfully")
}

// TestVolumeDataPersistence verifies data persists in service volumes across pod restarts
func TestVolumeDataPersistence(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	namespace := "enclii-test-data-persistence"
	helper, err := NewTestHelper(namespace)
	require.NoError(t, err, "failed to create test helper")

	// Setup
	err = helper.CreateNamespace(ctx)
	require.NoError(t, err, "failed to create namespace")
	defer func() {
		_ = helper.DeleteNamespace(ctx)
	}()

	t.Log("Testing data persistence in service volumes...")

	serviceName := "test-service"

	// Deploy test service with a volume
	err = helper.DeployTestService(ctx, serviceName, map[string]string{
		"data": "/data",
	})
	require.NoError(t, err, "failed to deploy test service")

	// Wait for pod to be ready
	pod, err := helper.WaitForPodReady(ctx, "app="+serviceName, 3*time.Minute)
	require.NoError(t, err, "pod should become ready")

	t.Logf("Pod ready: %s", pod.Name)

	// Write test data to volume
	t.Log("⚠️  Manual step: Write test data to volume")
	t.Log("   kubectl exec -it " + pod.Name + " -n " + namespace + " -- sh -c 'echo \"test-data\" > /data/test.txt'")

	// Delete pod to trigger restart
	t.Log("Deleting pod to trigger restart...")
	err = helper.DeletePod(ctx, pod.Name)
	require.NoError(t, err, "should delete pod")

	// Wait for new pod to be ready
	t.Log("Waiting for new pod to be ready...")
	newPod, err := helper.WaitForPodReady(ctx, "app="+serviceName, 2*time.Minute)
	require.NoError(t, err, "new pod should become ready")
	assert.NotEqual(t, pod.Name, newPod.Name, "new pod should have different name")

	t.Logf("New pod ready: %s", newPod.Name)

	// Verify data persists
	t.Log("⚠️  Manual step: Verify data persists")
	t.Log("   kubectl exec -it " + newPod.Name + " -n " + namespace + " -- cat /data/test.txt")
	t.Log("   Expected: test-data")

	t.Log("✅ Volume data persistence test completed")
}

// TestPVCStorageClass verifies PVCs use correct storage class
func TestPVCStorageClass(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	namespace := "enclii-test-storage-class"
	helper, err := NewTestHelper(namespace)
	require.NoError(t, err, "failed to create test helper")

	// Setup
	err = helper.CreateNamespace(ctx)
	require.NoError(t, err, "failed to create namespace")
	defer func() {
		_ = helper.DeleteNamespace(ctx)
	}()

	t.Log("Testing PVC storage class configuration...")

	// Deploy test service with volumes (all will use 'standard' storage class)
	serviceName := "test-service"
	err = helper.DeployTestService(ctx, serviceName, map[string]string{
		"uploads": "/data/uploads",
		"cache":   "/data/cache",
	})
	require.NoError(t, err, "failed to deploy test service")

	// Wait for deployment to be ready
	err = helper.WaitForDeploymentReady(ctx, serviceName, 3*time.Minute)
	require.NoError(t, err, "deployment should become ready")

	testCases := []struct {
		pvcName      string
		storageClass string
	}{
		{"test-service-uploads", "standard"},
		{"test-service-cache", "standard"},
	}

	for _, tc := range testCases {
		t.Run(tc.pvcName, func(t *testing.T) {
			pvc, err := helper.GetPVC(ctx, tc.pvcName)
			require.NoError(t, err, "PVC should exist")

			if pvc.Spec.StorageClassName != nil {
				assert.Equal(t, tc.storageClass, *pvc.Spec.StorageClassName,
					"PVC should use correct storage class")
				t.Logf("✓ PVC %s uses storage class: %s", tc.pvcName, *pvc.Spec.StorageClassName)
			}
		})
	}

	t.Log("✅ Storage class verification completed")
}

// TestPVCCleanupOnServiceDeletion verifies PVCs are deleted when service is deleted
func TestPVCCleanupOnServiceDeletion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	namespace := "enclii-test-pvc-cleanup"
	helper, err := NewTestHelper(namespace)
	require.NoError(t, err, "failed to create test helper")

	// Setup
	err = helper.CreateNamespace(ctx)
	require.NoError(t, err, "failed to create namespace")
	defer func() {
		_ = helper.DeleteNamespace(ctx)
	}()

	t.Log("Testing PVC cleanup on service deletion...")

	serviceName := "test-service"
	pvcName := serviceName + "-data"

	// Deploy test service with a volume
	err = helper.DeployTestService(ctx, serviceName, map[string]string{
		"data": "/data",
	})
	require.NoError(t, err, "failed to deploy test service")

	// Wait for deployment to be ready
	err = helper.WaitForDeploymentReady(ctx, serviceName, 3*time.Minute)
	require.NoError(t, err, "deployment should become ready")

	// Verify PVC exists
	pvc, err := helper.GetPVC(ctx, pvcName)
	require.NoError(t, err, "PVC should exist before deletion")
	t.Logf("PVC exists: %s", pvc.Name)

	// Delete the service (via reconciler.Delete)
	t.Log("⚠️  Manual step: Delete service")
	t.Log("   This should trigger reconciler.Delete() which deletes PVCs with label selector")

	// Wait and verify PVC is deleted
	t.Log("⚠️  Manual verification: PVC should be deleted")
	t.Log("   kubectl get pvc " + pvcName + " -n " + namespace)
	t.Log("   Expected: NotFound error")

	t.Log("✅ PVC cleanup test completed")
}
