package reconciler

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/madfam-org/enclii/packages/sdk-go/pkg/types"
)

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

// protocolPtr returns a pointer to the given Protocol
func protocolPtr(p corev1.Protocol) *corev1.Protocol {
	return &p
}
