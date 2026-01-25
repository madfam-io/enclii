package reconciler

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/madfam-org/enclii/packages/sdk-go/pkg/types"
)

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
