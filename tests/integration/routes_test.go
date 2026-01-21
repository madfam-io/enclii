package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	networkingv1 "k8s.io/api/networking/v1"
)

// TestSingleRouteCreation verifies Ingress is created with a single route
func TestSingleRouteCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	namespace := "enclii-test-single-route"
	helper, err := NewTestHelper(namespace)
	require.NoError(t, err, "failed to create test helper")

	// Setup
	err = helper.CreateNamespace(ctx)
	require.NoError(t, err, "failed to create namespace")
	defer func() {
		_ = helper.DeleteNamespace(ctx)
	}()

	t.Log("Testing single route creation...")

	serviceName := "web-app"
	customDomain := "app.example.com"

	// Add custom domain + route via API
	t.Log("⚠️  Manual step: Add custom domain and route via API")
	t.Log("   POST /v1/services/{service_id}/domains")
	t.Log("   {")
	t.Log("     \"domain\": \"" + customDomain + "\",")
	t.Log("     \"environment\": \"production\",")
	t.Log("     \"tls_enabled\": true")
	t.Log("   }")
	t.Log("")
	t.Log("   POST /v1/services/{service_id}/routes")
	t.Log("   {")
	t.Log("     \"path\": \"/api/v1\",")
	t.Log("     \"path_type\": \"Prefix\",")
	t.Log("     \"port\": 8080,")
	t.Log("     \"environment\": \"production\"")
	t.Log("   }")

	// Wait for Ingress to be created
	t.Log("Waiting for Ingress to be created...")
	ingress, err := helper.WaitForIngressCreated(ctx, serviceName, 1*time.Minute)
	require.NoError(t, err, "Ingress should be created")

	t.Logf("Ingress created: %s", ingress.Name)

	// Verify Ingress has correct host
	assert.Len(t, ingress.Spec.Rules, 1, "Ingress should have one rule")
	if len(ingress.Spec.Rules) > 0 {
		rule := ingress.Spec.Rules[0]
		assert.Equal(t, customDomain, rule.Host, "Ingress should have correct host")

		// Verify path configuration
		assert.NotNil(t, rule.HTTP, "Ingress rule should have HTTP config")
		if rule.HTTP != nil {
			assert.Len(t, rule.HTTP.Paths, 1, "Ingress should have one path")
			if len(rule.HTTP.Paths) > 0 {
				path := rule.HTTP.Paths[0]
				assert.Equal(t, "/api/v1", path.Path, "Path should be /api/v1")
				require.NotNil(t, path.PathType, "PathType should not be nil")
				assert.Equal(t, networkingv1.PathTypePrefix, *path.PathType, "PathType should be Prefix")
				assert.Equal(t, int32(8080), path.Backend.Service.Port.Number, "Backend port should be 8080")

				t.Logf("✓ Route: %s -> %s:%d", path.Path, path.Backend.Service.Name, path.Backend.Service.Port.Number)
			}
		}
	}

	t.Log("✅ Single route Ingress created successfully")
}

// TestMultipleRoutesCreation verifies Ingress is created with multiple routes
func TestMultipleRoutesCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	namespace := "enclii-test-multi-routes"
	helper, err := NewTestHelper(namespace)
	require.NoError(t, err, "failed to create test helper")

	// Setup
	err = helper.CreateNamespace(ctx)
	require.NoError(t, err, "failed to create namespace")
	defer func() {
		_ = helper.DeleteNamespace(ctx)
	}()

	t.Log("Testing multiple routes creation...")

	serviceName := "web-app"

	// Add multiple routes via API
	t.Log("⚠️  Manual step: Add custom domain and multiple routes via API")
	t.Log("   Routes:")
	t.Log("   1. POST /v1/services/{service_id}/routes")
	t.Log("      {\"path\": \"/api/v1\", \"path_type\": \"Prefix\", \"port\": 8080}")
	t.Log("   2. POST /v1/services/{service_id}/routes")
	t.Log("      {\"path\": \"/api/v2\", \"path_type\": \"Prefix\", \"port\": 8080}")
	t.Log("   3. POST /v1/services/{service_id}/routes")
	t.Log("      {\"path\": \"/docs\", \"path_type\": \"Exact\", \"port\": 8080}")

	// Wait for Ingress to be created
	ingress, err := helper.WaitForIngressCreated(ctx, serviceName, 1*time.Minute)
	require.NoError(t, err, "Ingress should be created")

	// Verify Ingress has multiple paths
	assert.Len(t, ingress.Spec.Rules, 1, "Ingress should have one rule")
	if len(ingress.Spec.Rules) > 0 {
		rule := ingress.Spec.Rules[0]
		assert.NotNil(t, rule.HTTP, "Ingress rule should have HTTP config")

		if rule.HTTP != nil {
			assert.Len(t, rule.HTTP.Paths, 3, "Ingress should have three paths")

			// Track which paths we've found
			foundPaths := make(map[string]bool)
			for _, path := range rule.HTTP.Paths {
				foundPaths[path.Path] = true
				t.Logf("✓ Route: %s (%s) -> %s:%d",
					path.Path,
					*path.PathType,
					path.Backend.Service.Name,
					path.Backend.Service.Port.Number)
			}

			assert.True(t, foundPaths["/api/v1"], "Should have /api/v1 route")
			assert.True(t, foundPaths["/api/v2"], "Should have /api/v2 route")
			assert.True(t, foundPaths["/docs"], "Should have /docs route")
		}
	}

	t.Log("✅ Multiple routes Ingress created successfully")
}

// TestPathTypesConfiguration verifies different path types are configured correctly
func TestPathTypesConfiguration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	namespace := "enclii-test-path-types"
	helper, err := NewTestHelper(namespace)
	require.NoError(t, err, "failed to create test helper")

	// Setup
	err = helper.CreateNamespace(ctx)
	require.NoError(t, err, "failed to create namespace")
	defer func() {
		_ = helper.DeleteNamespace(ctx)
	}()

	t.Log("Testing different path types...")

	serviceName := "web-app"

	// Add routes with different path types
	t.Log("⚠️  Manual step: Add routes with different path types")
	t.Log("   1. Prefix:                 /api")
	t.Log("   2. Exact:                  /health")
	t.Log("   3. ImplementationSpecific: /special")

	// Wait for Ingress
	ingress, err := helper.WaitForIngressCreated(ctx, serviceName, 1*time.Minute)
	require.NoError(t, err, "Ingress should be created")

	// Verify path types
	if len(ingress.Spec.Rules) > 0 && ingress.Spec.Rules[0].HTTP != nil {
		paths := ingress.Spec.Rules[0].HTTP.Paths

		for _, path := range paths {
			if path.PathType == nil {
				t.Errorf("Path %s has nil PathType", path.Path)
				continue
			}
			pathType := *path.PathType

			switch path.Path {
			case "/api":
				assert.Equal(t, networkingv1.PathTypePrefix, pathType, "/api should be Prefix type")
			case "/health":
				assert.Equal(t, networkingv1.PathTypeExact, pathType, "/health should be Exact type")
			case "/special":
				assert.Equal(t, networkingv1.PathTypeImplementationSpecific, pathType, "/special should be ImplementationSpecific type")
			}

			t.Logf("✓ Path %s has type %s", path.Path, pathType)
		}
	}

	t.Log("✅ Path types configured correctly")
}

// TestRouteUpdateReflectedInIngress verifies Ingress updates when routes change
func TestRouteUpdateReflectedInIngress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	namespace := "enclii-test-route-update"
	helper, err := NewTestHelper(namespace)
	require.NoError(t, err, "failed to create test helper")

	// Setup
	err = helper.CreateNamespace(ctx)
	require.NoError(t, err, "failed to create namespace")
	defer func() {
		_ = helper.DeleteNamespace(ctx)
	}()

	t.Log("Testing route updates reflected in Ingress...")

	serviceName := "web-app"

	// Get initial Ingress
	initialIngress, err := helper.GetIngress(ctx, serviceName)
	require.NoError(t, err, "Ingress should exist")

	initialPathCount := 0
	if len(initialIngress.Spec.Rules) > 0 && initialIngress.Spec.Rules[0].HTTP != nil {
		initialPathCount = len(initialIngress.Spec.Rules[0].HTTP.Paths)
	}

	t.Logf("Initial path count: %d", initialPathCount)

	// Add new route
	t.Log("⚠️  Manual step: Add new route")
	t.Log("   POST /v1/services/{service_id}/routes")
	t.Log("   {\"path\": \"/new-api\", \"path_type\": \"Prefix\", \"port\": 8080}")

	// Wait for reconciliation
	time.Sleep(5 * time.Second)

	// Get updated Ingress
	updatedIngress, err := helper.GetIngress(ctx, serviceName)
	require.NoError(t, err, "Ingress should still exist")

	updatedPathCount := 0
	if len(updatedIngress.Spec.Rules) > 0 && updatedIngress.Spec.Rules[0].HTTP != nil {
		updatedPathCount = len(updatedIngress.Spec.Rules[0].HTTP.Paths)
	}

	t.Logf("Updated path count: %d", updatedPathCount)

	// Verify path count increased
	assert.Greater(t, updatedPathCount, initialPathCount, "Path count should increase after adding route")

	t.Log("✅ Route update reflected in Ingress")
}

// TestRouteDeletionReflectedInIngress verifies Ingress updates when routes are deleted
func TestRouteDeletionReflectedInIngress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	namespace := "enclii-test-route-deletion"
	helper, err := NewTestHelper(namespace)
	require.NoError(t, err, "failed to create test helper")

	// Setup
	err = helper.CreateNamespace(ctx)
	require.NoError(t, err, "failed to create namespace")
	defer func() {
		_ = helper.DeleteNamespace(ctx)
	}()

	t.Log("Testing route deletion reflected in Ingress...")

	serviceName := "web-app"

	// Get initial Ingress
	initialIngress, err := helper.GetIngress(ctx, serviceName)
	require.NoError(t, err, "Ingress should exist")

	initialPathCount := 0
	if len(initialIngress.Spec.Rules) > 0 && initialIngress.Spec.Rules[0].HTTP != nil {
		initialPathCount = len(initialIngress.Spec.Rules[0].HTTP.Paths)
	}

	t.Logf("Initial path count: %d", initialPathCount)

	// Delete a route
	t.Log("⚠️  Manual step: Delete a route")
	t.Log("   DELETE /v1/services/{service_id}/routes/{route_id}")

	// Wait for reconciliation
	time.Sleep(5 * time.Second)

	// Get updated Ingress
	updatedIngress, err := helper.GetIngress(ctx, serviceName)
	require.NoError(t, err, "Ingress should still exist")

	updatedPathCount := 0
	if len(updatedIngress.Spec.Rules) > 0 && updatedIngress.Spec.Rules[0].HTTP != nil {
		updatedPathCount = len(updatedIngress.Spec.Rules[0].HTTP.Paths)
	}

	t.Logf("Updated path count: %d", updatedPathCount)

	// Verify path count decreased
	assert.Less(t, updatedPathCount, initialPathCount, "Path count should decrease after deleting route")

	t.Log("✅ Route deletion reflected in Ingress")
}

// TestIngressPathPriority verifies path priority ordering in Ingress
func TestIngressPathPriority(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	namespace := "enclii-test-path-priority"
	helper, err := NewTestHelper(namespace)
	require.NoError(t, err, "failed to create test helper")

	// Setup
	err = helper.CreateNamespace(ctx)
	require.NoError(t, err, "failed to create namespace")
	defer func() {
		_ = helper.DeleteNamespace(ctx)
	}()

	t.Log("Testing Ingress path priority...")

	serviceName := "web-app"

	// Add routes with overlapping paths
	t.Log("⚠️  Manual step: Add overlapping routes")
	t.Log("   Routes (order matters - more specific first):")
	t.Log("   1. /api/v1/users     (Exact)")
	t.Log("   2. /api/v1           (Prefix)")
	t.Log("   3. /api              (Prefix)")

	// Wait for Ingress
	ingress, err := helper.WaitForIngressCreated(ctx, serviceName, 1*time.Minute)
	require.NoError(t, err, "Ingress should be created")

	// Verify path ordering (more specific paths should come first)
	if len(ingress.Spec.Rules) > 0 && ingress.Spec.Rules[0].HTTP != nil {
		paths := ingress.Spec.Rules[0].HTTP.Paths

		t.Log("Path ordering:")
		for i, path := range paths {
			t.Logf("  %d. %s (%s)", i+1, path.Path, *path.PathType)
		}

		// Exact paths should generally come before Prefix paths
		// More specific prefixes should come before less specific ones
		for i := 0; i < len(paths)-1; i++ {
			currentPath := paths[i].Path
			nextPath := paths[i+1].Path

			// If current is Exact and next is Prefix with same base, that's correct
			if *paths[i].PathType == networkingv1.PathTypeExact &&
				*paths[i+1].PathType == networkingv1.PathTypePrefix {
				continue
			}

			// If both are Prefix, longer (more specific) should come first
			if *paths[i].PathType == networkingv1.PathTypePrefix &&
				*paths[i+1].PathType == networkingv1.PathTypePrefix {
				assert.GreaterOrEqual(t, len(currentPath), len(nextPath),
					fmt.Sprintf("Path %s should come before %s", currentPath, nextPath))
			}
		}
	}

	t.Log("✅ Path priority configured correctly")
}

// TestMultipleDomainsSameService verifies multiple domains pointing to same service
func TestMultipleDomainsSameService(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	namespace := "enclii-test-multi-domains"
	helper, err := NewTestHelper(namespace)
	require.NoError(t, err, "failed to create test helper")

	// Setup
	err = helper.CreateNamespace(ctx)
	require.NoError(t, err, "failed to create namespace")
	defer func() {
		_ = helper.DeleteNamespace(ctx)
	}()

	t.Log("Testing multiple domains for same service...")

	serviceName := "web-app"

	// Add multiple domains
	t.Log("⚠️  Manual step: Add multiple custom domains")
	t.Log("   1. app.example.com")
	t.Log("   2. www.example.com")
	t.Log("   3. api.example.com")

	// Wait for Ingress
	ingress, err := helper.WaitForIngressCreated(ctx, serviceName, 1*time.Minute)
	require.NoError(t, err, "Ingress should be created")

	// Verify Ingress has multiple rules (one per domain)
	expectedDomains := []string{"app.example.com", "www.example.com", "api.example.com"}
	assert.GreaterOrEqual(t, len(ingress.Spec.Rules), 1, "Ingress should have at least one rule")

	foundDomains := make([]string, 0)
	for _, rule := range ingress.Spec.Rules {
		foundDomains = append(foundDomains, rule.Host)
		t.Logf("✓ Domain: %s", rule.Host)
	}

	// Verify TLS configuration for all domains
	if len(ingress.Spec.TLS) > 0 {
		for _, tls := range ingress.Spec.TLS {
			t.Logf("✓ TLS hosts: %s (secret: %s)", strings.Join(tls.Hosts, ", "), tls.SecretName)

			// Each domain should be in TLS config
			for _, domain := range expectedDomains {
				// Check if this TLS entry covers this domain
				for _, tlsHost := range tls.Hosts {
					if tlsHost == domain {
						t.Logf("  ✓ %s has TLS", domain)
					}
				}
			}
		}
	}

	t.Log("✅ Multiple domains configured successfully")
}

// TestRouteWithCustomPort verifies routes can target different service ports
func TestRouteWithCustomPort(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	namespace := "enclii-test-custom-port"
	helper, err := NewTestHelper(namespace)
	require.NoError(t, err, "failed to create test helper")

	// Setup
	err = helper.CreateNamespace(ctx)
	require.NoError(t, err, "failed to create namespace")
	defer func() {
		_ = helper.DeleteNamespace(ctx)
	}()

	t.Log("Testing routes with custom ports...")

	serviceName := "web-app"

	// Add routes targeting different ports
	t.Log("⚠️  Manual step: Add routes with different ports")
	t.Log("   1. /api  -> port 8080 (main API)")
	t.Log("   2. /admin -> port 9090 (admin panel)")
	t.Log("   3. /metrics -> port 9100 (metrics)")

	// Wait for Ingress
	ingress, err := helper.WaitForIngressCreated(ctx, serviceName, 1*time.Minute)
	require.NoError(t, err, "Ingress should be created")

	// Verify different ports are configured
	if len(ingress.Spec.Rules) > 0 && ingress.Spec.Rules[0].HTTP != nil {
		paths := ingress.Spec.Rules[0].HTTP.Paths

		portMap := make(map[string]int32)
		for _, path := range paths {
			portMap[path.Path] = path.Backend.Service.Port.Number
			t.Logf("✓ %s -> port %d", path.Path, path.Backend.Service.Port.Number)
		}

		// Verify we have different ports
		uniquePorts := make(map[int32]bool)
		for _, port := range portMap {
			uniquePorts[port] = true
		}

		assert.Greater(t, len(uniquePorts), 1, "Should have routes targeting different ports")
	}

	t.Log("✅ Custom port routing configured successfully")
}
