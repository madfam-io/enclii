package types

import (
	"testing"

	"github.com/google/uuid"
)

func TestService_IDString(t *testing.T) {
	id := uuid.New()
	service := &Service{ID: id}

	if got := service.IDString(); got != id.String() {
		t.Errorf("Service.IDString() = %v, want %v", got, id.String())
	}
}

func TestService_ProjectIDString(t *testing.T) {
	projectID := uuid.New()
	service := &Service{ProjectID: projectID}

	if got := service.ProjectIDString(); got != projectID.String() {
		t.Errorf("Service.ProjectIDString() = %v, want %v", got, projectID.String())
	}
}

func TestProject_IDString(t *testing.T) {
	id := uuid.New()
	project := &Project{ID: id}

	if got := project.IDString(); got != id.String() {
		t.Errorf("Project.IDString() = %v, want %v", got, id.String())
	}
}

func TestEnvironment_IDString(t *testing.T) {
	id := uuid.New()
	env := &Environment{ID: id}

	if got := env.IDString(); got != id.String() {
		t.Errorf("Environment.IDString() = %v, want %v", got, id.String())
	}
}

func TestEnvironment_ProjectIDString(t *testing.T) {
	projectID := uuid.New()
	env := &Environment{ProjectID: projectID}

	if got := env.ProjectIDString(); got != projectID.String() {
		t.Errorf("Environment.ProjectIDString() = %v, want %v", got, projectID.String())
	}
}

func TestRelease_IDString(t *testing.T) {
	id := uuid.New()
	release := &Release{ID: id}

	if got := release.IDString(); got != id.String() {
		t.Errorf("Release.IDString() = %v, want %v", got, id.String())
	}
}

func TestRelease_ServiceIDString(t *testing.T) {
	serviceID := uuid.New()
	release := &Release{ServiceID: serviceID}

	if got := release.ServiceIDString(); got != serviceID.String() {
		t.Errorf("Release.ServiceIDString() = %v, want %v", got, serviceID.String())
	}
}

func TestDeployment_IDString(t *testing.T) {
	id := uuid.New()
	deployment := &Deployment{ID: id}

	if got := deployment.IDString(); got != id.String() {
		t.Errorf("Deployment.IDString() = %v, want %v", got, id.String())
	}
}

func TestDeployment_ReleaseIDString(t *testing.T) {
	releaseID := uuid.New()
	deployment := &Deployment{ReleaseID: releaseID}

	if got := deployment.ReleaseIDString(); got != releaseID.String() {
		t.Errorf("Deployment.ReleaseIDString() = %v, want %v", got, releaseID.String())
	}
}

func TestDeployment_EnvironmentIDString(t *testing.T) {
	envID := uuid.New()
	deployment := &Deployment{EnvironmentID: envID}

	if got := deployment.EnvironmentIDString(); got != envID.String() {
		t.Errorf("Deployment.EnvironmentIDString() = %v, want %v", got, envID.String())
	}
}

func TestUser_IDString(t *testing.T) {
	id := uuid.New()
	user := &User{ID: id}

	if got := user.IDString(); got != id.String() {
		t.Errorf("User.IDString() = %v, want %v", got, id.String())
	}
}

func TestParseUUID(t *testing.T) {
	validUUID := uuid.New()
	validStr := validUUID.String()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid UUID",
			input:   validStr,
			wantErr: false,
		},
		{
			name:    "invalid UUID",
			input:   "not-a-uuid",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseUUID(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("ParseUUID() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseUUID() unexpected error: %v", err)
				return
			}

			if got.String() != tt.input {
				t.Errorf("ParseUUID() = %v, want %v", got, tt.input)
			}
		})
	}
}

func TestMustParseUUID(t *testing.T) {
	validUUID := uuid.New()
	validStr := validUUID.String()

	// Test valid UUID
	got := MustParseUUID(validStr)
	if got.String() != validStr {
		t.Errorf("MustParseUUID() = %v, want %v", got, validStr)
	}

	// Test panic on invalid UUID
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustParseUUID() did not panic on invalid UUID")
		}
	}()
	MustParseUUID("invalid-uuid")
}

func TestNewUUID(t *testing.T) {
	id1 := NewUUID()
	id2 := NewUUID()

	if id1 == id2 {
		t.Error("NewUUID() generated duplicate UUIDs")
	}

	if id1 == uuid.Nil {
		t.Error("NewUUID() generated nil UUID")
	}
}

func TestIsValidUUID(t *testing.T) {
	validUUID := uuid.New().String()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "valid UUID",
			input: validUUID,
			want:  true,
		},
		{
			name:  "invalid UUID",
			input: "not-a-uuid",
			want:  false,
		},
		{
			name:  "empty string",
			input: "",
			want:  false,
		},
		{
			name:  "almost valid UUID - wrong format",
			input: "12345678-1234-1234-1234-12345678901",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidUUID(tt.input); got != tt.want {
				t.Errorf("IsValidUUID(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
