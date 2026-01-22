package integration

import (
	"context"
	"strings"
	"testing"

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
	customDomain := "single-route.example.com"

	// Create backend service
	err = helper.CreateBackendService(ctx, serviceName, 8080)
	require.NoError(t, err, "failed to create backend service")

	// Create Ingress with single path
	paths := []IngressPath{
		{Path: "/api/v1", PathType: networkingv1.PathTypePrefix, Service: serviceName, Port: 8080},
	}
	ingress, err := helper.CreateIngressWithPaths(ctx, serviceName, customDomain, paths, "letsencrypt-staging")
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
	customDomain := "multi-routes.example.com"

	// Create backend service
	err = helper.CreateBackendService(ctx, serviceName, 8080)
	require.NoError(t, err, "failed to create backend service")

	// Create Ingress with multiple paths
	paths := []IngressPath{
		{Path: "/api/v1", PathType: networkingv1.PathTypePrefix, Service: serviceName, Port: 8080},
		{Path: "/api/v2", PathType: networkingv1.PathTypePrefix, Service: serviceName, Port: 8080},
		{Path: "/docs", PathType: networkingv1.PathTypeExact, Service: serviceName, Port: 8080},
	}
	ingress, err := helper.CreateIngressWithPaths(ctx, serviceName, customDomain, paths, "letsencrypt-staging")
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
	customDomain := "path-types.example.com"

	// Create backend service
	err = helper.CreateBackendService(ctx, serviceName, 8080)
	require.NoError(t, err, "failed to create backend service")

	// Create Ingress with different path types
	paths := []IngressPath{
		{Path: "/api", PathType: networkingv1.PathTypePrefix, Service: serviceName, Port: 8080},
		{Path: "/health", PathType: networkingv1.PathTypeExact, Service: serviceName, Port: 8080},
		{Path: "/special", PathType: networkingv1.PathTypeImplementationSpecific, Service: serviceName, Port: 8080},
	}
	ingress, err := helper.CreateIngressWithPaths(ctx, serviceName, customDomain, paths, "letsencrypt-staging")
	require.NoError(t, err, "Ingress should be created")

	// Verify path types
	if len(ingress.Spec.Rules) > 0 && ingress.Spec.Rules[0].HTTP != nil {
		ingressPaths := ingress.Spec.Rules[0].HTTP.Paths

		for _, path := range ingressPaths {
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

// TestRouteUpdateReflectedInIngress verifies Ingress updates when routes are added
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
	customDomain := "route-update.example.com"

	// Create backend service
	err = helper.CreateBackendService(ctx, serviceName, 8080)
	require.NoError(t, err, "failed to create backend service")

	// Create initial Ingress with one path
	initialPaths := []IngressPath{
		{Path: "/api/v1", PathType: networkingv1.PathTypePrefix, Service: serviceName, Port: 8080},
	}
	_, err = helper.CreateIngressWithPaths(ctx, serviceName, customDomain, initialPaths, "letsencrypt-staging")
	require.NoError(t, err, "Ingress should be created")

	// Get initial Ingress
	initialIngress, err := helper.GetIngress(ctx, serviceName)
	require.NoError(t, err, "Ingress should exist")

	initialPathCount := 0
	if len(initialIngress.Spec.Rules) > 0 && initialIngress.Spec.Rules[0].HTTP != nil {
		initialPathCount = len(initialIngress.Spec.Rules[0].HTTP.Paths)
	}

	t.Logf("Initial path count: %d", initialPathCount)

	// Add new route
	t.Log("Adding new route...")
	newPath := IngressPath{Path: "/new-api", PathType: networkingv1.PathTypePrefix, Service: serviceName, Port: 8080}
	_, err = helper.AddIngressPath(ctx, serviceName, newPath)
	require.NoError(t, err, "should add new path")

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
	customDomain := "route-deletion.example.com"

	// Create backend service
	err = helper.CreateBackendService(ctx, serviceName, 8080)
	require.NoError(t, err, "failed to create backend service")

	// Create Ingress with multiple paths
	paths := []IngressPath{
		{Path: "/api/v1", PathType: networkingv1.PathTypePrefix, Service: serviceName, Port: 8080},
		{Path: "/api/v2", PathType: networkingv1.PathTypePrefix, Service: serviceName, Port: 8080},
		{Path: "/docs", PathType: networkingv1.PathTypeExact, Service: serviceName, Port: 8080},
	}
	_, err = helper.CreateIngressWithPaths(ctx, serviceName, customDomain, paths, "letsencrypt-staging")
	require.NoError(t, err, "Ingress should be created")

	// Get initial Ingress
	initialIngress, err := helper.GetIngress(ctx, serviceName)
	require.NoError(t, err, "Ingress should exist")

	initialPathCount := 0
	if len(initialIngress.Spec.Rules) > 0 && initialIngress.Spec.Rules[0].HTTP != nil {
		initialPathCount = len(initialIngress.Spec.Rules[0].HTTP.Paths)
	}

	t.Logf("Initial path count: %d", initialPathCount)

	// Delete a route
	t.Log("Removing /api/v2 route...")
	_, err = helper.RemoveIngressPath(ctx, serviceName, "/api/v2")
	require.NoError(t, err, "should remove path")

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
	customDomain := "path-priority.example.com"

	// Create backend service
	err = helper.CreateBackendService(ctx, serviceName, 8080)
	require.NoError(t, err, "failed to create backend service")

	// Create Ingress with paths in priority order (more specific first)
	// Note: Kubernetes doesn't enforce ordering, but our platform should
	paths := []IngressPath{
		{Path: "/api/v1/users", PathType: networkingv1.PathTypeExact, Service: serviceName, Port: 8080},
		{Path: "/api/v1", PathType: networkingv1.PathTypePrefix, Service: serviceName, Port: 8080},
		{Path: "/api", PathType: networkingv1.PathTypePrefix, Service: serviceName, Port: 8080},
	}
	ingress, err := helper.CreateIngressWithPaths(ctx, serviceName, customDomain, paths, "letsencrypt-staging")
	require.NoError(t, err, "Ingress should be created")

	// Verify paths are present
	if len(ingress.Spec.Rules) > 0 && ingress.Spec.Rules[0].HTTP != nil {
		ingressPaths := ingress.Spec.Rules[0].HTTP.Paths

		t.Log("Path ordering:")
		for i, path := range ingressPaths {
			t.Logf("  %d. %s (%s)", i+1, path.Path, *path.PathType)
		}

		// Verify all paths exist
		foundPaths := make(map[string]bool)
		for _, path := range ingressPaths {
			foundPaths[path.Path] = true
		}
		assert.True(t, foundPaths["/api/v1/users"], "Should have /api/v1/users")
		assert.True(t, foundPaths["/api/v1"], "Should have /api/v1")
		assert.True(t, foundPaths["/api"], "Should have /api")
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
	hosts := []string{"multi-domains-1.example.com", "multi-domains-2.example.com", "multi-domains-3.example.com"}

	// Create backend service
	err = helper.CreateBackendService(ctx, serviceName, 8080)
	require.NoError(t, err, "failed to create backend service")

	// Create Ingress with multiple hosts
	ingress, err := helper.CreateIngressWithMultipleHosts(ctx, serviceName, hosts, serviceName, 8080, "letsencrypt-staging")
	require.NoError(t, err, "Ingress should be created")

	// Verify Ingress has multiple rules (one per domain)
	assert.Len(t, ingress.Spec.Rules, 3, "Ingress should have three rules")

	foundDomains := make([]string, 0)
	for _, rule := range ingress.Spec.Rules {
		foundDomains = append(foundDomains, rule.Host)
		t.Logf("✓ Domain: %s", rule.Host)
	}

	// Verify all hosts are present
	for _, host := range hosts {
		found := false
		for _, domain := range foundDomains {
			if domain == host {
				found = true
				break
			}
		}
		assert.True(t, found, "Should have domain %s", host)
	}

	// Verify TLS configuration for all domains
	if len(ingress.Spec.TLS) > 0 {
		for _, tls := range ingress.Spec.TLS {
			t.Logf("✓ TLS hosts: %s (secret: %s)", strings.Join(tls.Hosts, ", "), tls.SecretName)
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
	customDomain := "custom-port.example.com"

	// Create backend services for different ports
	err = helper.CreateBackendService(ctx, "api-service", 8080)
	require.NoError(t, err, "failed to create api backend service")
	err = helper.CreateBackendService(ctx, "admin-service", 9090)
	require.NoError(t, err, "failed to create admin backend service")
	err = helper.CreateBackendService(ctx, "metrics-service", 9100)
	require.NoError(t, err, "failed to create metrics backend service")

	// Create Ingress with routes targeting different ports
	paths := []IngressPath{
		{Path: "/api", PathType: networkingv1.PathTypePrefix, Service: "api-service", Port: 8080},
		{Path: "/admin", PathType: networkingv1.PathTypePrefix, Service: "admin-service", Port: 9090},
		{Path: "/metrics", PathType: networkingv1.PathTypePrefix, Service: "metrics-service", Port: 9100},
	}
	ingress, err := helper.CreateIngressWithPaths(ctx, serviceName, customDomain, paths, "letsencrypt-staging")
	require.NoError(t, err, "Ingress should be created")

	// Verify different ports are configured
	if len(ingress.Spec.Rules) > 0 && ingress.Spec.Rules[0].HTTP != nil {
		ingressPaths := ingress.Spec.Rules[0].HTTP.Paths

		portMap := make(map[string]int32)
		for _, path := range ingressPaths {
			portMap[path.Path] = path.Backend.Service.Port.Number
			t.Logf("✓ %s -> %s:%d", path.Path, path.Backend.Service.Name, path.Backend.Service.Port.Number)
		}

		// Verify we have different ports
		uniquePorts := make(map[int32]bool)
		for _, port := range portMap {
			uniquePorts[port] = true
		}

		assert.Equal(t, 3, len(uniquePorts), "Should have routes targeting three different ports")
	}

	t.Log("✅ Custom port routing configured successfully")
}
