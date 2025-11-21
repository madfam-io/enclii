package validation

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestNewValidator(t *testing.T) {
	validator := NewValidator()

	if validator == nil {
		t.Fatal("NewValidator() returned nil")
	}

	if validator.validate == nil {
		t.Error("NewValidator() validate field is nil")
	}
}

func TestValidator_ValidateStruct(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name      string
		input     interface{}
		wantError bool
		errorCount int
	}{
		{
			name: "valid create project request",
			input: &CreateProjectRequest{
				Name: "Test Project",
				Slug: "test-project",
			},
			wantError: false,
		},
		{
			name: "create project - missing name",
			input: &CreateProjectRequest{
				Name: "",
				Slug: "test-project",
			},
			wantError: true,
		},
		{
			name: "create project - invalid slug",
			input: &CreateProjectRequest{
				Name: "Test",
				Slug: "AB", // too short
			},
			wantError: true,
		},
		{
			name: "valid create service request",
			input: &CreateServiceRequest{
				Name:    "test-service",
				GitRepo: "https://github.com/user/repo.git",
				BuildConfig: BuildConfig{
					Type: "auto",
				},
			},
			wantError: false,
		},
		{
			name: "create service - invalid git repo",
			input: &CreateServiceRequest{
				Name:    "test-service",
				GitRepo: "not-a-git-url",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validator.ValidateStruct(tt.input)

			if tt.wantError {
				if len(errors) == 0 {
					t.Error("ValidateStruct() expected validation errors, got none")
				}
				return
			}

			if len(errors) > 0 {
				t.Errorf("ValidateStruct() unexpected validation errors: %v", errors)
			}
		})
	}
}

func TestValidateDNSName(t *testing.T) {
	validator := NewValidator()

	type testStruct struct {
		Name string `validate:"dnsname"`
	}

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid dns name", "test-service", false},
		{"valid with numbers", "service123", false},
		{"valid single char", "a", false},
		{"invalid uppercase", "TestService", true},
		{"invalid underscore", "test_service", true},
		{"invalid start with hyphen", "-test", true},
		{"invalid end with hyphen", "test-", true},
		{"invalid empty", "", true},
		{"invalid too long", strings.Repeat("a", 64), true},
		{"valid max length", strings.Repeat("a", 63), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &testStruct{Name: tt.value}
			errors := validator.ValidateStruct(s)

			hasError := len(errors) > 0
			if hasError != tt.wantErr {
				t.Errorf("validateDNSName(%q) hasError = %v, want %v", tt.value, hasError, tt.wantErr)
			}
		})
	}
}

func TestValidateEnvVarName(t *testing.T) {
	validator := NewValidator()

	type testStruct struct {
		Var string `validate:"envvar"`
	}

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid env var", "DATABASE_URL", false},
		{"valid with numbers", "API_KEY_123", false},
		{"valid single char", "A", false},
		{"valid starts with underscore", "_PRIVATE", false},
		{"invalid lowercase", "database_url", true},
		{"invalid starts with number", "123_VAR", true},
		{"invalid hyphen", "DATABASE-URL", true},
		{"invalid space", "DATABASE URL", true},
		{"invalid empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &testStruct{Var: tt.value}
			errors := validator.ValidateStruct(s)

			hasError := len(errors) > 0
			if hasError != tt.wantErr {
				t.Errorf("validateEnvVarName(%q) hasError = %v, want %v", tt.value, hasError, tt.wantErr)
			}
		})
	}
}

func TestValidateGitRepo(t *testing.T) {
	validator := NewValidator()

	type testStruct struct {
		Repo string `validate:"gitrepo"`
	}

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid https", "https://github.com/user/repo.git", false},
		{"valid http", "http://github.com/user/repo.git", false},
		{"invalid git@ (SSH not supported)", "git@github.com:user/repo.git", true}, // Implementation only supports HTTP(S)
		{"invalid no .git", "https://github.com/user/repo", true},
		{"invalid no protocol", "github.com/user/repo.git", true},
		{"invalid empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &testStruct{Repo: tt.value}
			errors := validator.ValidateStruct(s)

			hasError := len(errors) > 0
			if hasError != tt.wantErr {
				t.Errorf("validateGitRepo(%q) hasError = %v, want %v", tt.value, hasError, tt.wantErr)
			}
		})
	}
}

func TestValidateK8sName(t *testing.T) {
	validator := NewValidator()

	type testStruct struct {
		Name string `validate:"k8sname"`
	}

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid k8s name", "my-app", false},
		{"valid with numbers", "app123", false},
		{"valid single char", "a", false},
		{"invalid uppercase", "MyApp", true},
		{"invalid start with hyphen", "-app", true},
		{"invalid end with hyphen", "app-", true},
		{"invalid underscore", "my_app", true},
		{"invalid empty", "", true},
		{"invalid too long", string(make([]byte, 64)), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &testStruct{Name: tt.value}
			errors := validator.ValidateStruct(s)

			hasError := len(errors) > 0
			if hasError != tt.wantErr {
				t.Errorf("validateK8sName(%q) hasError = %v, want %v", tt.value, hasError, tt.wantErr)
			}
		})
	}
}

func TestValidateProjectSlug(t *testing.T) {
	validator := NewValidator()

	type testStruct struct {
		Slug string `validate:"project_slug"`
	}

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid slug", "my-project", false},
		{"valid min length", "abc", false},
		{"invalid too short", "ab", true},
		{"invalid too long", string(make([]byte, 64)), true},
		{"invalid uppercase", "MyProject", true},
		{"invalid underscore", "my_project", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &testStruct{Slug: tt.value}
			errors := validator.ValidateStruct(s)

			hasError := len(errors) > 0
			if hasError != tt.wantErr {
				t.Errorf("validateProjectSlug(%q) hasError = %v, want %v", tt.value, hasError, tt.wantErr)
			}
		})
	}
}

func TestValidateServiceName(t *testing.T) {
	validator := NewValidator()

	type testStruct struct {
		Name string `validate:"service_name"`
	}

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid service name", "my-service", false},
		{"valid min length", "a", false},
		{"invalid empty", "", true},
		{"invalid too long", string(make([]byte, 64)), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &testStruct{Name: tt.value}
			errors := validator.ValidateStruct(s)

			hasError := len(errors) > 0
			if hasError != tt.wantErr {
				t.Errorf("validateServiceName(%q) hasError = %v, want %v", tt.value, hasError, tt.wantErr)
			}
		})
	}
}

func TestValidateSafeString(t *testing.T) {
	validator := NewValidator()

	type testStruct struct {
		Value string `validate:"safe_string"`
	}

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid string", "Hello World", false},
		{"valid with numbers", "Test123", false},
		{"valid with special chars", "test-project_name", false},
		{"invalid with <", "test<script>", true},
		{"invalid with >", "test>", true},
		{"invalid with quote", "test\"value", true},
		{"invalid with apostrophe", "test'value", true},
		{"invalid with ampersand", "test&value", true},
		{"invalid control char", "test\nvalue", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &testStruct{Value: tt.value}
			errors := validator.ValidateStruct(s)

			hasError := len(errors) > 0
			if hasError != tt.wantErr {
				t.Errorf("validateSafeString(%q) hasError = %v, want %v", tt.value, hasError, tt.wantErr)
			}
		})
	}
}

func TestValidatePortNumber(t *testing.T) {
	validator := NewValidator()

	type testStruct struct {
		Port int `validate:"port_number"`
	}

	tests := []struct {
		name    string
		value   int
		wantErr bool
	}{
		{"valid port 80", 80, false},
		{"valid port 8080", 8080, false},
		{"valid min port", 1, false},
		{"valid max port", 65535, false},
		{"invalid port 0", 0, true},
		{"invalid negative", -1, true},
		{"invalid too high", 65536, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &testStruct{Port: tt.value}
			errors := validator.ValidateStruct(s)

			hasError := len(errors) > 0
			if hasError != tt.wantErr {
				t.Errorf("validatePortNumber(%d) hasError = %v, want %v", tt.value, hasError, tt.wantErr)
			}
		})
	}
}

func TestSanitizeString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"normal string", "hello world", "hello world"},
		{"with spaces", "  hello world  ", "hello world"},
		{"remove <", "test<script>", "testscript"},
		{"remove >", "test>value", "testvalue"},
		{"remove quotes", "test\"value\"", "testvalue"},
		{"remove ampersand", "test&value", "testvalue"},
		{"remove control chars", "test\nvalue", "testvalue"},
		{"unicode", "тест", "тест"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeString(tt.input)
			if got != tt.expected {
				t.Errorf("SanitizeString(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSanitizeDNSName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"normal name", "my-project", "my-project"},
		{"uppercase", "MyProject", "myproject"},
		{"spaces", "My Project", "myproject"},
		{"special chars", "my_project!", "myproject"}, // Invalid chars removed, not replaced
		{"leading hyphen", "-test", "test"},
		{"trailing hyphen", "test-", "test"},
		{"empty", "", "default"},
		{"too long", strings.Repeat("a", 70), strings.Repeat("a", 63)}, // Use valid chars, truncated to 63

	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeDNSName(tt.input)
			// For too long test, just check length
			if tt.name == "too long" {
				if len(got) != 63 {
					t.Errorf("SanitizeDNSName() length = %d, want 63", len(got))
				}
				return
			}
			if got != tt.expected {
				t.Errorf("SanitizeDNSName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSanitizeProjectSlug(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		minLen   int
	}{
		{"normal slug", "my-project", 3},
		{"short slug", "ab", 3}, // Will be extended
		{"uppercase", "MyProject", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeProjectSlug(tt.input)
			if len(got) < tt.minLen {
				t.Errorf("SanitizeProjectSlug(%q) length = %d, want >= %d", tt.input, len(got), tt.minLen)
			}
		})
	}
}

func TestValidateUUID(t *testing.T) {
	validUUID := uuid.New()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid UUID", validUUID.String(), false},
		{"invalid UUID", "not-a-uuid", true},
		{"empty string", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ValidateUUID(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("ValidateUUID() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ValidateUUID() unexpected error: %v", err)
				return
			}

			if result.String() != tt.input {
				t.Errorf("ValidateUUID() = %v, want %v", result, tt.input)
			}
		})
	}
}

func TestValidationErrors_Error(t *testing.T) {
	errors := ValidationErrors{
		{Field: "name", Message: "Name is required"},
		{Field: "email", Message: "Invalid email format"},
	}

	errorStr := errors.Error()

	if errorStr == "" {
		t.Error("ValidationErrors.Error() returned empty string")
	}

	// Should contain both field names and messages
	if !contains(errorStr, "name") || !contains(errorStr, "Name is required") {
		t.Errorf("ValidationErrors.Error() missing name error: %s", errorStr)
	}

	if !contains(errorStr, "email") || !contains(errorStr, "Invalid email format") {
		t.Errorf("ValidationErrors.Error() missing email error: %s", errorStr)
	}
}

func TestGetErrorMessage(t *testing.T) {
	// This is tested indirectly through ValidateStruct tests
	// Just verify it doesn't panic with unknown tags
	validator := NewValidator()

	type testStruct struct {
		Field string `validate:"required"`
	}

	s := &testStruct{Field: ""}
	errors := validator.ValidateStruct(s)

	if len(errors) == 0 {
		t.Fatal("Expected validation error")
	}

	if errors[0].Message == "" {
		t.Error("Error message is empty")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
