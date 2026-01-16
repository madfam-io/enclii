//go:build integration

package services

import (
	"testing"
)

// Integration tests requiring database setup
// Run with: go test -tags=integration ./...

func TestProjectService_CreateProject(t *testing.T) {
	t.Skip("TODO: Requires database setup - see tests/integration/")
}

func TestProjectService_CreateProject_DuplicateSlug(t *testing.T) {
	t.Skip("TODO: Requires database setup - see tests/integration/")
}

func TestProjectService_GetProject(t *testing.T) {
	t.Skip("TODO: Requires database setup - see tests/integration/")
}

func TestProjectService_ListProjects(t *testing.T) {
	t.Skip("TODO: Requires database setup - see tests/integration/")
}

func TestProjectService_CreateService(t *testing.T) {
	t.Skip("TODO: Requires database setup - see tests/integration/")
}

func TestProjectService_GetService(t *testing.T) {
	t.Skip("TODO: Requires database setup - see tests/integration/")
}

func TestProjectService_ListServices(t *testing.T) {
	t.Skip("TODO: Requires database setup - see tests/integration/")
}
