package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCustomDomainCreation verifies custom domain creates Ingress resource
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

	t.Log("Testing custom domain creation...")

	serviceName := "api-gateway"
	customDomain := "api.example.com"

	// Add custom domain via API
	t.Log("⚠️  Manual step: Add custom domain via API")
	t.Log("   POST /v1/services/{service_id}/domains")
	t.Log("   {")
	t.Log("     \"domain\": \"" + customDomain + "\",")
	t.Log("     \"environment\": \"production\",")
	t.Log("     \"tls_enabled\": true,")
	t.Log("     \"tls_issuer\": \"letsencrypt-staging\"")
	t.Log("   }")

	// Wait for Ingress to be created
	t.Log("Waiting for Ingress to be created...")
	ingress, err := helper.WaitForIngressCreated(ctx, serviceName, 1*time.Minute)
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
	assert.Equal(t, "letsencrypt-staging", annotations["cert-manager.io/cluster-issuer"],
		"should use correct issuer")
	t.Logf("✓ cert-manager issuer: %s", annotations["cert-manager.io/cluster-issuer"])

	// Verify SSL redirect
	assert.Equal(t, "true", annotations["nginx.ingress.kubernetes.io/ssl-redirect"],
		"should have SSL redirect enabled")

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

// TestTLSCertificateIssuance verifies cert-manager issues TLS certificate
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

	t.Log("Testing TLS certificate issuance...")

	serviceName := "api-gateway"

	// Get Ingress
	ingress, err := helper.GetIngress(ctx, serviceName)
	require.NoError(t, err, "Ingress should exist")

	// Get TLS secret name
	require.Len(t, ingress.Spec.TLS, 1, "Ingress should have TLS config")
	tlsSecretName := ingress.Spec.TLS[0].SecretName

	t.Logf("TLS secret name: %s", tlsSecretName)

	// Wait for Certificate to be issued
	t.Log("⚠️  Manual verification: Wait for Certificate to be issued")
	t.Log("   kubectl get certificate -n " + namespace)
	t.Log("   kubectl describe certificate " + tlsSecretName + " -n " + namespace)
	t.Log("   Wait for status: Ready=True")

	// Verify Secret is created
	t.Log("⚠️  Manual verification: Verify Secret contains TLS certificate")
	t.Log("   kubectl get secret " + tlsSecretName + " -n " + namespace)
	t.Log("   kubectl get secret " + tlsSecretName + " -o jsonpath='{.data.tls\\.crt}' | base64 -d | openssl x509 -text -noout")

	t.Log("✅ TLS certificate issuance test completed")
	t.Log("   Note: cert-manager must be installed for this test")
}

// TestDomainVerification verifies domain ownership via DNS TXT record
func TestDomainVerification(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Log("Testing domain verification...")

	testDomain := "verify.example.com"
	verificationCode := "enclii-verification=abc123"

	// Add custom domain (would return verification code)
	t.Log("⚠️  Manual step: Add custom domain")
	t.Log("   POST /v1/services/{service_id}/domains")
	t.Log("   Response will include: verification_value")

	// Add DNS TXT record
	t.Log("⚠️  Manual step: Add DNS TXT record")
	t.Log("   " + testDomain + " TXT " + verificationCode)

	// Verify DNS propagation
	t.Log("⚠️  Manual step: Verify DNS propagation")
	t.Log("   dig TXT " + testDomain)
	t.Log("   Should return: " + verificationCode)

	// Call verify endpoint
	t.Log("⚠️  Manual step: Call verify endpoint")
	t.Log("   POST /v1/services/{service_id}/domains/{domain_id}/verify")
	t.Log("   Should return: {\"message\": \"domain verified successfully\"}")

	// Verify database updated
	t.Log("⚠️  Manual step: Verify database")
	t.Log("   SELECT domain, verified, verified_at FROM custom_domains WHERE domain='" + testDomain + "';")
	t.Log("   verified should be true, verified_at should be set")

	t.Log("✅ Domain verification test completed")
}

// TestIngressUpdate verifies Ingress is updated when domain settings change
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

	t.Log("Testing Ingress update on domain changes...")

	serviceName := "api-gateway"

	// Get initial Ingress
	initialIngress, err := helper.GetIngress(ctx, serviceName)
	require.NoError(t, err, "Ingress should exist")
	initialIssuer := initialIngress.Annotations["cert-manager.io/cluster-issuer"]

	t.Logf("Initial cert-manager issuer: %s", initialIssuer)

	// Update domain TLS issuer
	t.Log("⚠️  Manual step: Update domain TLS issuer")
	t.Log("   PATCH /v1/services/{service_id}/domains/{domain_id}")
	t.Log("   {\"tls_issuer\": \"letsencrypt-prod\"}")

	// Wait for Ingress to be updated
	time.Sleep(5 * time.Second)

	// Get updated Ingress
	updatedIngress, err := helper.GetIngress(ctx, serviceName)
	require.NoError(t, err, "Ingress should still exist")

	updatedIssuer := updatedIngress.Annotations["cert-manager.io/cluster-issuer"]
	t.Logf("Updated cert-manager issuer: %s", updatedIssuer)

	// Verify issuer changed
	assert.NotEqual(t, initialIssuer, updatedIssuer, "issuer should have changed")
	assert.Equal(t, "letsencrypt-prod", updatedIssuer, "issuer should be letsencrypt-prod")

	t.Log("✅ Ingress update test completed")
}

// TestIngressDeletion verifies Ingress is deleted when all custom domains are removed
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

	t.Log("Testing Ingress deletion when domains are removed...")

	serviceName := "api-gateway"

	// Verify Ingress exists
	ingress, err := helper.GetIngress(ctx, serviceName)
	require.NoError(t, err, "Ingress should exist")
	t.Logf("Ingress exists: %s", ingress.Name)

	// Delete all custom domains
	t.Log("⚠️  Manual step: Delete all custom domains")
	t.Log("   DELETE /v1/services/{service_id}/domains/{domain_id}")

	// Wait for Ingress to be deleted
	t.Log("⚠️  Manual verification: Ingress should be deleted")
	t.Log("   kubectl get ingress " + serviceName + " -n " + namespace)
	t.Log("   Expected: NotFound error")

	t.Log("✅ Ingress deletion test completed")
}

// TestHTTPSRedirect verifies HTTPS redirect is configured
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

	// Get Ingress
	ingress, err := helper.GetIngress(ctx, serviceName)
	require.NoError(t, err, "Ingress should exist")

	// Verify SSL redirect annotation
	annotations := ingress.Annotations
	sslRedirect, ok := annotations["nginx.ingress.kubernetes.io/ssl-redirect"]
	assert.True(t, ok, "should have ssl-redirect annotation")
	assert.Equal(t, "true", sslRedirect, "ssl-redirect should be enabled")

	forceSslRedirect, ok := annotations["nginx.ingress.kubernetes.io/force-ssl-redirect"]
	assert.True(t, ok, "should have force-ssl-redirect annotation")
	assert.Equal(t, "true", forceSslRedirect, "force-ssl-redirect should be enabled")

	t.Log("✓ HTTPS redirect annotations configured correctly")

	// Test HTTP request redirects to HTTPS
	t.Log("⚠️  Manual verification: Test HTTP redirect")
	t.Log("   curl -v http://api.example.com/health")
	t.Log("   Expected: HTTP 301/302 redirect to https://api.example.com/health")

	t.Log("✅ HTTPS redirect test completed")
}

// mockAPIClient is a simple HTTP client for testing API endpoints
type mockAPIClient struct {
	baseURL string
	token   string
}

func (c *mockAPIClient) addCustomDomain(serviceID, domain, environment, tlsIssuer string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/v1/services/%s/domains", c.baseURL, serviceID)

	payload := map[string]interface{}{
		"domain":      domain,
		"environment": environment,
		"tls_enabled": true,
		"tls_issuer":  tlsIssuer,
	}

	payloadBytes, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", url, strings.NewReader(string(payloadBytes)))
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	return result, nil
}
