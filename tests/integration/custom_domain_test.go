package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCustomDomainCreation verifies custom domain creates Ingress resource with correct configuration
func TestCustomDomainCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	namespace := "enclii-test-custom-domain"
	helper, err := NewTestHelper(namespace)
	require.NoError(t, err, "failed to create test helper")

	// Setup
	err = helper.CreateNamespace(ctx)
	require.NoError(t, err, "failed to create namespace")
	defer func() {
		_ = helper.DeleteNamespace(ctx)
	}()

	t.Log("Testing custom domain Ingress creation...")

	serviceName := "api-gateway"
	customDomain := "api.example.com"
	tlsIssuer := "letsencrypt-staging"

	// Create backend service first (Ingress needs a valid backend)
	err = helper.CreateBackendService(ctx, serviceName, 80)
	require.NoError(t, err, "failed to create backend service")

	// Create Ingress with custom domain configuration
	t.Log("Creating Ingress with TLS and cert-manager configuration...")
	ingress, err := helper.CreateIngress(ctx, serviceName, customDomain, serviceName, 80, tlsIssuer)
	require.NoError(t, err, "Ingress should be created")

	t.Logf("Ingress created: %s", ingress.Name)

	// Verify Ingress has correct host
	assert.Len(t, ingress.Spec.Rules, 1, "Ingress should have one rule")
	if len(ingress.Spec.Rules) > 0 {
		assert.Equal(t, customDomain, ingress.Spec.Rules[0].Host, "Ingress should have correct host")
		t.Logf("✓ Ingress host: %s", ingress.Spec.Rules[0].Host)
	}

	// Verify cert-manager annotations
	annotations := ingress.Annotations
	assert.Contains(t, annotations, "cert-manager.io/cluster-issuer", "should have cert-manager annotation")
	assert.Equal(t, tlsIssuer, annotations["cert-manager.io/cluster-issuer"], "should use correct issuer")
	t.Logf("✓ cert-manager issuer: %s", annotations["cert-manager.io/cluster-issuer"])

	// Verify SSL redirect
	assert.Equal(t, "true", annotations["nginx.ingress.kubernetes.io/ssl-redirect"], "should have SSL redirect enabled")

	// Verify TLS configuration
	assert.Len(t, ingress.Spec.TLS, 1, "Ingress should have TLS configuration")
	if len(ingress.Spec.TLS) > 0 {
		tls := ingress.Spec.TLS[0]
		assert.Contains(t, tls.Hosts, customDomain, "TLS should include custom domain")
		assert.True(t, strings.HasPrefix(tls.SecretName, serviceName), "TLS secret should have service name prefix")
		t.Logf("✓ TLS secret: %s", tls.SecretName)
	}

	t.Log("✅ Custom domain Ingress created successfully")
}

// TestTLSCertificateIssuance verifies TLS configuration is correct for cert-manager
func TestTLSCertificateIssuance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	namespace := "enclii-test-tls"
	helper, err := NewTestHelper(namespace)
	require.NoError(t, err, "failed to create test helper")

	// Setup
	err = helper.CreateNamespace(ctx)
	require.NoError(t, err, "failed to create namespace")
	defer func() {
		_ = helper.DeleteNamespace(ctx)
	}()

	t.Log("Testing TLS certificate configuration...")

	serviceName := "api-gateway"
	customDomain := "secure.example.com"
	tlsIssuer := "letsencrypt-staging"

	// Create backend service and Ingress
	err = helper.CreateBackendService(ctx, serviceName, 80)
	require.NoError(t, err, "failed to create backend service")

	ingress, err := helper.CreateIngress(ctx, serviceName, customDomain, serviceName, 80, tlsIssuer)
	require.NoError(t, err, "Ingress should be created")

	// Verify TLS configuration
	require.Len(t, ingress.Spec.TLS, 1, "Ingress should have TLS config")
	tlsSecretName := ingress.Spec.TLS[0].SecretName

	t.Logf("TLS secret name: %s", tlsSecretName)

	// Verify cert-manager annotation is set correctly
	issuer := ingress.Annotations["cert-manager.io/cluster-issuer"]
	assert.Equal(t, tlsIssuer, issuer, "cert-manager issuer should be configured")
	t.Logf("✓ cert-manager issuer configured: %s", issuer)

	// Verify TLS hosts match domain
	assert.Contains(t, ingress.Spec.TLS[0].Hosts, customDomain, "TLS hosts should include custom domain")
	t.Logf("✓ TLS hosts: %v", ingress.Spec.TLS[0].Hosts)

	t.Log("✅ TLS certificate configuration test completed")
	t.Log("   Note: cert-manager will issue certificate when Ingress is applied to a real cluster")
}

// TestDomainVerification documents the domain ownership verification flow
func TestDomainVerification(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Log("Testing domain verification flow...")

	// This test documents the verification flow
	// Actual DNS verification requires external DNS and is tested manually

	t.Log("Domain verification flow:")
	t.Log("  1. Add custom domain via API → returns verification_value")
	t.Log("  2. User adds DNS TXT record: _enclii-verify.domain.com TXT {verification_value}")
	t.Log("  3. User calls verify endpoint → system checks DNS")
	t.Log("  4. If verified, domain.verified=true and Ingress is created")

	t.Log("✅ Domain verification flow documented")
}

// TestIngressUpdate verifies Ingress can be updated when configuration changes
func TestIngressUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	namespace := "enclii-test-ingress-update"
	helper, err := NewTestHelper(namespace)
	require.NoError(t, err, "failed to create test helper")

	// Setup
	err = helper.CreateNamespace(ctx)
	require.NoError(t, err, "failed to create namespace")
	defer func() {
		_ = helper.DeleteNamespace(ctx)
	}()

	t.Log("Testing Ingress update on configuration changes...")

	serviceName := "api-gateway"
	customDomain := "update.example.com"

	// Create backend service and Ingress with staging issuer
	err = helper.CreateBackendService(ctx, serviceName, 80)
	require.NoError(t, err, "failed to create backend service")

	_, err = helper.CreateIngress(ctx, serviceName, customDomain, serviceName, 80, "letsencrypt-staging")
	require.NoError(t, err, "Ingress should be created")

	// Get initial Ingress
	initialIngress, err := helper.GetIngress(ctx, serviceName)
	require.NoError(t, err, "Ingress should exist")
	initialIssuer := initialIngress.Annotations["cert-manager.io/cluster-issuer"]

	t.Logf("Initial cert-manager issuer: %s", initialIssuer)
	assert.Equal(t, "letsencrypt-staging", initialIssuer, "should start with staging issuer")

	// Update to production issuer
	t.Log("Updating cert-manager issuer to production...")
	updatedIngress, err := helper.UpdateIngressAnnotation(ctx, serviceName, "cert-manager.io/cluster-issuer", "letsencrypt-prod")
	require.NoError(t, err, "Ingress update should succeed")

	updatedIssuer := updatedIngress.Annotations["cert-manager.io/cluster-issuer"]
	t.Logf("Updated cert-manager issuer: %s", updatedIssuer)

	// Verify issuer changed
	assert.NotEqual(t, initialIssuer, updatedIssuer, "issuer should have changed")
	assert.Equal(t, "letsencrypt-prod", updatedIssuer, "issuer should be letsencrypt-prod")

	t.Log("✅ Ingress update test completed")
}

// TestIngressDeletion verifies Ingress can be deleted
func TestIngressDeletion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	namespace := "enclii-test-ingress-deletion"
	helper, err := NewTestHelper(namespace)
	require.NoError(t, err, "failed to create test helper")

	// Setup
	err = helper.CreateNamespace(ctx)
	require.NoError(t, err, "failed to create namespace")
	defer func() {
		_ = helper.DeleteNamespace(ctx)
	}()

	t.Log("Testing Ingress deletion...")

	serviceName := "api-gateway"
	customDomain := "delete.example.com"

	// Create backend service and Ingress
	err = helper.CreateBackendService(ctx, serviceName, 80)
	require.NoError(t, err, "failed to create backend service")

	_, err = helper.CreateIngress(ctx, serviceName, customDomain, serviceName, 80, "letsencrypt-staging")
	require.NoError(t, err, "Ingress should be created")

	// Verify Ingress exists
	ingress, err := helper.GetIngress(ctx, serviceName)
	require.NoError(t, err, "Ingress should exist")
	t.Logf("Ingress exists: %s", ingress.Name)

	// Delete Ingress
	t.Log("Deleting Ingress...")
	err = helper.DeleteIngress(ctx, serviceName)
	require.NoError(t, err, "Ingress deletion should succeed")

	// Wait for Ingress to be deleted
	err = helper.WaitForIngressDeleted(ctx, serviceName, 30*time.Second)
	require.NoError(t, err, "Ingress should be deleted")

	// Verify Ingress no longer exists
	_, err = helper.GetIngress(ctx, serviceName)
	assert.Error(t, err, "Ingress should not exist after deletion")
	t.Log("✓ Ingress deleted successfully")

	t.Log("✅ Ingress deletion test completed")
}

// TestHTTPSRedirect verifies HTTPS redirect annotations are configured correctly
func TestHTTPSRedirect(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	namespace := "enclii-test-https-redirect"
	helper, err := NewTestHelper(namespace)
	require.NoError(t, err, "failed to create test helper")

	// Setup
	err = helper.CreateNamespace(ctx)
	require.NoError(t, err, "failed to create namespace")
	defer func() {
		_ = helper.DeleteNamespace(ctx)
	}()

	t.Log("Testing HTTPS redirect configuration...")

	serviceName := "api-gateway"
	customDomain := "redirect.example.com"

	// Create backend service and Ingress
	err = helper.CreateBackendService(ctx, serviceName, 80)
	require.NoError(t, err, "failed to create backend service")

	_, err = helper.CreateIngress(ctx, serviceName, customDomain, serviceName, 80, "letsencrypt-staging")
	require.NoError(t, err, "Ingress should be created")

	// Get Ingress
	ingress, err := helper.GetIngress(ctx, serviceName)
	require.NoError(t, err, "Ingress should exist")

	// Verify SSL redirect annotation
	annotations := ingress.Annotations
	sslRedirect, ok := annotations["nginx.ingress.kubernetes.io/ssl-redirect"]
	assert.True(t, ok, "should have ssl-redirect annotation")
	assert.Equal(t, "true", sslRedirect, "ssl-redirect should be enabled")
	t.Log("✓ ssl-redirect annotation: true")

	forceSslRedirect, ok := annotations["nginx.ingress.kubernetes.io/force-ssl-redirect"]
	assert.True(t, ok, "should have force-ssl-redirect annotation")
	assert.Equal(t, "true", forceSslRedirect, "force-ssl-redirect should be enabled")
	t.Log("✓ force-ssl-redirect annotation: true")

	t.Log("✅ HTTPS redirect configuration test completed")
}
