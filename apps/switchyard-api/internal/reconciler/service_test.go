package reconciler

import (
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// TestParseContainerPort tests the parseContainerPort function
func TestParseContainerPort(t *testing.T) {
	tests := []struct {
		name        string
		envVars     map[string]string
		wantPort    int32
		wantErr     bool
		description string
	}{
		{
			name:        "ENCLII_PORT set",
			envVars:     map[string]string{"ENCLII_PORT": "4200"},
			wantPort:    4200,
			wantErr:     false,
			description: "Should use ENCLII_PORT when set",
		},
		{
			name:        "PORT fallback",
			envVars:     map[string]string{"PORT": "4104"},
			wantPort:    4104,
			wantErr:     false,
			description: "Should fallback to PORT when ENCLII_PORT not set",
		},
		{
			name:        "ENCLII_PORT takes precedence",
			envVars:     map[string]string{"ENCLII_PORT": "4200", "PORT": "8080"},
			wantPort:    4200,
			wantErr:     false,
			description: "ENCLII_PORT should take precedence over PORT",
		},
		{
			name:        "Default when neither set",
			envVars:     map[string]string{},
			wantPort:    4200,
			wantErr:     false,
			description: "Should return default port (4200) when neither env var set",
		},
		{
			name:        "Default with other env vars",
			envVars:     map[string]string{"DATABASE_URL": "postgres://...", "API_KEY": "secret"},
			wantPort:    4200,
			wantErr:     false,
			description: "Should return default port when only unrelated env vars set",
		},
		{
			name:        "Invalid ENCLII_PORT",
			envVars:     map[string]string{"ENCLII_PORT": "not-a-number"},
			wantPort:    4200,
			wantErr:     true,
			description: "Should error and return default for invalid ENCLII_PORT",
		},
		{
			name:        "Invalid PORT",
			envVars:     map[string]string{"PORT": "abc"},
			wantPort:    4200,
			wantErr:     true,
			description: "Should error and return default for invalid PORT",
		},
		{
			name:        "ENCLII_PORT out of range (too high)",
			envVars:     map[string]string{"ENCLII_PORT": "70000"},
			wantPort:    4200,
			wantErr:     true,
			description: "Should error for port > 65535",
		},
		{
			name:        "ENCLII_PORT out of range (zero)",
			envVars:     map[string]string{"ENCLII_PORT": "0"},
			wantPort:    4200,
			wantErr:     true,
			description: "Should error for port 0",
		},
		{
			name:        "PORT out of range (negative)",
			envVars:     map[string]string{"PORT": "-1"},
			wantPort:    4200,
			wantErr:     true,
			description: "Should error for negative port",
		},
		{
			name:        "Empty ENCLII_PORT falls back to PORT",
			envVars:     map[string]string{"ENCLII_PORT": "", "PORT": "3000"},
			wantPort:    3000,
			wantErr:     false,
			description: "Empty ENCLII_PORT should fallback to PORT",
		},
		{
			name:        "Janua-style PORT",
			envVars:     map[string]string{"PORT": "4101"},
			wantPort:    4101,
			wantErr:     false,
			description: "Should handle Janua dashboard port (4101)",
		},
		{
			name:        "Minimum valid port",
			envVars:     map[string]string{"PORT": "1"},
			wantPort:    1,
			wantErr:     false,
			description: "Should accept minimum valid port (1)",
		},
		{
			name:        "Maximum valid port",
			envVars:     map[string]string{"PORT": "65535"},
			wantPort:    65535,
			wantErr:     false,
			description: "Should accept maximum valid port (65535)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPort, err := parseContainerPort(tt.envVars)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseContainerPort() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if gotPort != tt.wantPort {
				t.Errorf("parseContainerPort() = %v, want %v (%s)", gotPort, tt.wantPort, tt.description)
			}
		})
	}
}

// TestParseContainerPortWithSource tests source tracking
func TestParseContainerPortWithSource(t *testing.T) {
	tests := []struct {
		name       string
		envVars    map[string]string
		wantPort   int32
		wantSource PortSource
		wantErr    bool
	}{
		{
			name:       "Source is ENCLII_PORT",
			envVars:    map[string]string{"ENCLII_PORT": "4200"},
			wantPort:   4200,
			wantSource: PortSourceEncliiPort,
			wantErr:    false,
		},
		{
			name:       "Source is PORT",
			envVars:    map[string]string{"PORT": "4104"},
			wantPort:   4104,
			wantSource: PortSourcePort,
			wantErr:    false,
		},
		{
			name:       "Source is default",
			envVars:    map[string]string{},
			wantPort:   4200,
			wantSource: PortSourceDefault,
			wantErr:    false,
		},
		{
			name:       "ENCLII_PORT precedence with source",
			envVars:    map[string]string{"ENCLII_PORT": "4200", "PORT": "8080"},
			wantPort:   4200,
			wantSource: PortSourceEncliiPort,
			wantErr:    false,
		},
		{
			name:       "Invalid falls back to default source",
			envVars:    map[string]string{"ENCLII_PORT": "invalid"},
			wantPort:   4200,
			wantSource: PortSourceDefault,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPort, gotSource, err := parseContainerPortWithSource(tt.envVars)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseContainerPortWithSource() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if gotPort != tt.wantPort {
				t.Errorf("parseContainerPortWithSource() port = %v, want %v", gotPort, tt.wantPort)
			}

			if gotSource != tt.wantSource {
				t.Errorf("parseContainerPortWithSource() source = %v, want %v", gotSource, tt.wantSource)
			}
		})
	}
}

// TestExtractNetworkPolicyPort tests port extraction from NetworkPolicy
func TestExtractNetworkPolicyPort(t *testing.T) {
	tests := []struct {
		name     string
		np       *networkingv1.NetworkPolicy
		wantPort int32
	}{
		{
			name:     "Nil NetworkPolicy",
			np:       nil,
			wantPort: 0,
		},
		{
			name: "No ingress rules",
			np: &networkingv1.NetworkPolicy{
				Spec: networkingv1.NetworkPolicySpec{
					Ingress: []networkingv1.NetworkPolicyIngressRule{},
				},
			},
			wantPort: 0,
		},
		{
			name: "No ports in ingress rule",
			np: &networkingv1.NetworkPolicy{
				Spec: networkingv1.NetworkPolicySpec{
					Ingress: []networkingv1.NetworkPolicyIngressRule{
						{Ports: []networkingv1.NetworkPolicyPort{}},
					},
				},
			},
			wantPort: 0,
		},
		{
			name: "Valid port",
			np: &networkingv1.NetworkPolicy{
				Spec: networkingv1.NetworkPolicySpec{
					Ingress: []networkingv1.NetworkPolicyIngressRule{
						{
							Ports: []networkingv1.NetworkPolicyPort{
								{Port: &intstr.IntOrString{Type: intstr.Int, IntVal: 4200}},
							},
						},
					},
				},
			},
			wantPort: 4200,
		},
		{
			name: "Multiple ports returns first",
			np: &networkingv1.NetworkPolicy{
				Spec: networkingv1.NetworkPolicySpec{
					Ingress: []networkingv1.NetworkPolicyIngressRule{
						{
							Ports: []networkingv1.NetworkPolicyPort{
								{Port: &intstr.IntOrString{Type: intstr.Int, IntVal: 8080}},
								{Port: &intstr.IntOrString{Type: intstr.Int, IntVal: 443}},
							},
						},
					},
				},
			},
			wantPort: 8080,
		},
		{
			name: "Janua port",
			np: &networkingv1.NetworkPolicy{
				Spec: networkingv1.NetworkPolicySpec{
					Ingress: []networkingv1.NetworkPolicyIngressRule{
						{
							Ports: []networkingv1.NetworkPolicyPort{
								{Port: &intstr.IntOrString{Type: intstr.Int, IntVal: 4104}},
							},
						},
					},
				},
			},
			wantPort: 4104,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPort := extractNetworkPolicyPort(tt.np)

			if gotPort != tt.wantPort {
				t.Errorf("extractNetworkPolicyPort() = %v, want %v", gotPort, tt.wantPort)
			}
		})
	}
}

// TestPortSourceConstants ensures constants are correct
func TestPortSourceConstants(t *testing.T) {
	if PortSourceEncliiPort != "ENCLII_PORT" {
		t.Errorf("PortSourceEncliiPort = %v, want ENCLII_PORT", PortSourceEncliiPort)
	}
	if PortSourcePort != "PORT" {
		t.Errorf("PortSourcePort = %v, want PORT", PortSourcePort)
	}
	if PortSourceDefault != "default" {
		t.Errorf("PortSourceDefault = %v, want default", PortSourceDefault)
	}
}
