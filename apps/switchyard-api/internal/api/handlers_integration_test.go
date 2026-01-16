//go:build integration

package api

import (
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/madfam-org/enclii/apps/switchyard-api/internal/config"
)

func setupTestHandler() *Handler {
	gin.SetMode(gin.TestMode)

	// For integration tests, use nil for all dependencies
	// Full integration tests should use proper test database setup
	handler := NewHandler(
		nil, // repos
		&config.Config{Registry: "test-registry"},
		nil, // auth manager
		nil, // cache
		nil, // builder
		nil, // k8s client
		nil, // controller
		nil, // reconciler
		nil, // metrics
		nil, // logger
		nil, // validator
		nil, // provenance checker
		nil, // compliance exporter
		nil, // topology builder
		nil, // auth service
		nil, // project service
		nil, // deployment service
		nil, // deployment group service
		nil, // roundhouse client
	)

	return handler
}

func TestCreateProject(t *testing.T) {
	_ = setupTestHandler()
	t.Skip("TODO: Requires full handler dependencies - see tests/integration/")
}

func TestListProjects(t *testing.T) {
	_ = setupTestHandler()
	t.Skip("TODO: Requires full handler dependencies - see tests/integration/")
}

func TestGetProject(t *testing.T) {
	_ = setupTestHandler()
	t.Skip("TODO: Requires full handler dependencies - see tests/integration/")
}

// Benchmark tests also need proper setup
func BenchmarkCreateProject(b *testing.B) {
	b.Skip("TODO: Requires full handler dependencies for benchmarking")
}
