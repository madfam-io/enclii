//go:build integration

package services

import (
	"testing"
)

// Integration tests requiring database setup
// Run with: go test -tags=integration ./...

func TestAuthService_Register(t *testing.T) {
	t.Skip("TODO: Requires database setup - see tests/integration/")
}

func TestAuthService_Register_DuplicateEmail(t *testing.T) {
	t.Skip("TODO: Requires database setup - see tests/integration/")
}

func TestAuthService_Login(t *testing.T) {
	t.Skip("TODO: Requires database setup - see tests/integration/")
}

func TestAuthService_RefreshToken(t *testing.T) {
	t.Skip("TODO: Requires database setup - see tests/integration/")
}

func TestAuthService_Logout(t *testing.T) {
	t.Skip("TODO: Requires database setup - see tests/integration/")
}

func TestAuthService_CheckAccess(t *testing.T) {
	t.Skip("TODO: Requires database setup - see tests/integration/")
}
