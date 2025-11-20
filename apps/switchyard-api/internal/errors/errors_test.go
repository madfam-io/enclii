package errors

import (
	"errors"
	"net/http"
	"testing"
)

func TestAppError_Error(t *testing.T) {
	tests := []struct {
		name     string
		appErr   *AppError
		expected string
	}{
		{
			name: "error without underlying error",
			appErr: &AppError{
				Code:    "TEST_ERROR",
				Message: "test message",
			},
			expected: "test message",
		},
		{
			name: "error with underlying error",
			appErr: &AppError{
				Code:    "TEST_ERROR",
				Message: "test message",
				Err:     errors.New("underlying error"),
			},
			expected: "test message: underlying error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.appErr.Error(); got != tt.expected {
				t.Errorf("Error() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAppError_WithDetails(t *testing.T) {
	original := ErrValidation
	details := map[string]string{"field": "email", "reason": "invalid format"}

	withDetails := original.WithDetails(details)

	if withDetails.Code != original.Code {
		t.Errorf("Code = %v, want %v", withDetails.Code, original.Code)
	}
	if withDetails.Details == nil {
		t.Error("Details should not be nil")
	}
}

func TestAppError_WithError(t *testing.T) {
	original := ErrDatabaseError
	underlyingErr := errors.New("connection failed")

	withErr := original.WithError(underlyingErr)

	if withErr.Err != underlyingErr {
		t.Errorf("Err = %v, want %v", withErr.Err, underlyingErr)
	}
	if !errors.Is(withErr, underlyingErr) {
		t.Error("Should be able to unwrap to underlying error")
	}
}

func TestAppError_Unwrap(t *testing.T) {
	underlyingErr := errors.New("underlying")
	appErr := ErrInternal.WithError(underlyingErr)

	unwrapped := errors.Unwrap(appErr)
	if unwrapped != underlyingErr {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, underlyingErr)
	}
}

func TestWrap(t *testing.T) {
	underlyingErr := errors.New("database connection failed")
	wrapped := Wrap(underlyingErr, ErrDatabaseError)

	if wrapped.Err != underlyingErr {
		t.Errorf("Wrapped error should contain underlying error")
	}
	if !errors.Is(wrapped, underlyingErr) {
		t.Error("Should be able to check underlying error with errors.Is")
	}
}

func TestIs(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		target   *AppError
		expected bool
	}{
		{
			name:     "matching app error",
			err:      ErrNotFound,
			target:   ErrNotFound,
			expected: true,
		},
		{
			name:     "wrapped matching app error",
			err:      ErrNotFound.WithError(errors.New("not found")),
			target:   ErrNotFound,
			expected: true,
		},
		{
			name:     "non-matching app error",
			err:      ErrNotFound,
			target:   ErrUnauthorized,
			expected: false,
		},
		{
			name:     "non-app error",
			err:      errors.New("generic error"),
			target:   ErrNotFound,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Is(tt.err, tt.target); got != tt.expected {
				t.Errorf("Is() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetHTTPStatus(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{
			name:     "not found error",
			err:      ErrNotFound,
			expected: http.StatusNotFound,
		},
		{
			name:     "unauthorized error",
			err:      ErrUnauthorized,
			expected: http.StatusUnauthorized,
		},
		{
			name:     "validation error",
			err:      ErrValidation,
			expected: http.StatusBadRequest,
		},
		{
			name:     "generic error defaults to 500",
			err:      errors.New("generic"),
			expected: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetHTTPStatus(tt.err); got != tt.expected {
				t.Errorf("GetHTTPStatus() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetErrorResponse(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		expectedCode string
	}{
		{
			name:         "app error",
			err:          ErrNotFound,
			expectedCode: "NOT_FOUND",
		},
		{
			name:         "app error with details",
			err:          ErrValidation.WithDetails(map[string]string{"field": "email"}),
			expectedCode: "VALIDATION_ERROR",
		},
		{
			name:         "generic error",
			err:          errors.New("generic"),
			expectedCode: "INTERNAL_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := GetErrorResponse(tt.err)

			errorMap, ok := response["error"].(map[string]any)
			if !ok {
				t.Fatal("Response should have 'error' map")
			}

			code, ok := errorMap["code"].(string)
			if !ok {
				t.Fatal("Error should have 'code' field")
			}

			if code != tt.expectedCode {
				t.Errorf("Error code = %v, want %v", code, tt.expectedCode)
			}
		})
	}
}

func TestNew(t *testing.T) {
	err := New("CUSTOM_ERROR", "custom message", http.StatusTeapot)

	if err.Code != "CUSTOM_ERROR" {
		t.Errorf("Code = %v, want CUSTOM_ERROR", err.Code)
	}
	if err.Message != "custom message" {
		t.Errorf("Message = %v, want 'custom message'", err.Message)
	}
	if err.HTTPStatus != http.StatusTeapot {
		t.Errorf("HTTPStatus = %v, want %v", err.HTTPStatus, http.StatusTeapot)
	}
}

func TestPredefinedErrors(t *testing.T) {
	// Test that common errors are properly defined
	errors := []*AppError{
		ErrNotFound,
		ErrUnauthorized,
		ErrValidation,
		ErrBuildFailed,
		ErrDeploymentFailed,
		ErrInternal,
	}

	for _, err := range errors {
		t.Run(err.Code, func(t *testing.T) {
			if err.Code == "" {
				t.Error("Code should not be empty")
			}
			if err.Message == "" {
				t.Error("Message should not be empty")
			}
			if err.HTTPStatus == 0 {
				t.Error("HTTPStatus should not be zero")
			}
		})
	}
}
