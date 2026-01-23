package reconciler

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsEncliiManagedDeployment(t *testing.T) {
	tests := []struct {
		name     string
		labels   map[string]string
		expected bool
	}{
		{
			name:     "no labels",
			labels:   nil,
			expected: false,
		},
		{
			name:     "empty labels",
			labels:   map[string]string{},
			expected: false,
		},
		{
			name:     "managed by switchyard",
			labels:   map[string]string{"enclii.dev/managed-by": "switchyard"},
			expected: true,
		},
		{
			name:     "managed by manual",
			labels:   map[string]string{"enclii.dev/managed-by": "manual"},
			expected: false,
		},
		{
			name:     "other label only",
			labels:   map[string]string{"app": "test"},
			expected: false,
		},
		{
			name:     "managed by switchyard with other labels",
			labels:   map[string]string{"enclii.dev/managed-by": "switchyard", "app": "myapp", "version": "v1"},
			expected: true,
		},
		{
			name:     "wrong managed-by value",
			labels:   map[string]string{"enclii.dev/managed-by": "argocd"},
			expected: false,
		},
		{
			name:     "empty managed-by value",
			labels:   map[string]string{"enclii.dev/managed-by": ""},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dep := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Labels: tt.labels,
				},
			}
			if got := isEncliiManagedDeployment(dep); got != tt.expected {
				t.Errorf("isEncliiManagedDeployment() = %v, expected %v", got, tt.expected)
			}
		})
	}
}
