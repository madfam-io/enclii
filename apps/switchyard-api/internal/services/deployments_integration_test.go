//go:build integration

package services

import (
	"testing"
)

// Integration tests requiring database setup
// Run with: go test -tags=integration ./...

func TestDeploymentService_Create(t *testing.T) {
	t.Skip("TODO: Requires database setup - see tests/integration/")
}

func TestDeploymentService_Get(t *testing.T) {
	t.Skip("TODO: Requires database setup - see tests/integration/")
}

func TestDeploymentService_List(t *testing.T) {
	t.Skip("TODO: Requires database setup - see tests/integration/")
}

func TestDeploymentService_UpdateStatus(t *testing.T) {
	t.Skip("TODO: Requires database setup - see tests/integration/")
}

func TestDeploymentService_GetLatest(t *testing.T) {
	t.Skip("TODO: Requires database setup - see tests/integration/")
}

func TestDeploymentService_Rollback(t *testing.T) {
	t.Skip("TODO: Requires database setup - see tests/integration/")
}
