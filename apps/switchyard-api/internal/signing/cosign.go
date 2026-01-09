package signing

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Signer handles container image signing with Cosign
type Signer struct {
	keyless bool // Use keyless signing (OIDC-based)
	timeout time.Duration
}

// NewSigner creates a new image signer
func NewSigner(keyless bool, timeout time.Duration) *Signer {
	if timeout == 0 {
		timeout = 2 * time.Minute
	}

	return &Signer{
		keyless: keyless,
		timeout: timeout,
	}
}

// SignResult represents the result of a signing operation
type SignResult struct {
	Success       bool      `json:"success"`
	Signature     string    `json:"signature"` // The signature digest
	SignedAt      time.Time `json:"signed_at"`
	SigningMethod string    `json:"signing_method"` // "keyless" or "key-based"
	Error         error     `json:"error,omitempty"`
}

// SignImage signs a container image using Cosign
// imageURI: Full image URI (e.g., "ghcr.io/madfam/my-service:v1.2.3")
// Returns signature information or error
func (s *Signer) SignImage(ctx context.Context, imageURI string) (*SignResult, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	result := &SignResult{
		SignedAt:      time.Now().UTC(),
		SigningMethod: s.getSigningMethod(),
	}

	if s.keyless {
		// Keyless signing using OIDC (GitHub Actions, GitLab CI, etc.)
		// This is the recommended approach for CI/CD environments
		// cosign sign --yes IMAGE_URI
		return s.signKeyless(timeoutCtx, imageURI, result)
	}

	// Key-based signing (requires COSIGN_KEY environment variable)
	// cosign sign --key env://COSIGN_KEY IMAGE_URI
	return s.signWithKey(timeoutCtx, imageURI, result)
}

// signKeyless performs keyless signing using OIDC
func (s *Signer) signKeyless(ctx context.Context, imageURI string, result *SignResult) (*SignResult, error) {
	// Keyless signing requires OIDC authentication
	// In production, this would be automatically handled by GitHub Actions, GitLab CI, etc.
	// For local development, use: export COSIGN_EXPERIMENTAL=1
	cmd := exec.CommandContext(ctx, "cosign", "sign", "--yes", imageURI)

	output, err := cmd.CombinedOutput()
	if err != nil {
		result.Error = fmt.Errorf("cosign keyless signing failed: %w (output: %s)", err, string(output))
		return result, result.Error
	}

	result.Success = true
	result.Signature = s.extractSignature(string(output))

	return result, nil
}

// signWithKey performs signing with a private key
func (s *Signer) signWithKey(ctx context.Context, imageURI string, result *SignResult) (*SignResult, error) {
	// Sign with key from environment variable
	// Requires COSIGN_KEY and COSIGN_PASSWORD environment variables
	cmd := exec.CommandContext(ctx, "cosign", "sign", "--key", "env://COSIGN_KEY", imageURI)

	output, err := cmd.CombinedOutput()
	if err != nil {
		result.Error = fmt.Errorf("cosign key-based signing failed: %w (output: %s)", err, string(output))
		return result, result.Error
	}

	result.Success = true
	result.Signature = s.extractSignature(string(output))

	return result, nil
}

// VerifySignature verifies the signature of a container image
func (s *Signer) VerifySignature(ctx context.Context, imageURI string) (bool, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	if s.keyless {
		// Verify keyless signature
		// cosign verify IMAGE_URI
		cmd := exec.CommandContext(timeoutCtx, "cosign", "verify", imageURI)
		_, err := cmd.Output()
		if err != nil {
			return false, fmt.Errorf("signature verification failed: %w", err)
		}
		return true, nil
	}

	// Verify with public key
	// cosign verify --key env://COSIGN_PUBLIC_KEY IMAGE_URI
	cmd := exec.CommandContext(timeoutCtx, "cosign", "verify", "--key", "env://COSIGN_PUBLIC_KEY", imageURI)
	_, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("signature verification failed: %w", err)
	}

	return true, nil
}

// AttestSBOM attaches an SBOM attestation to the signed image
// This creates a verifiable link between the image and its SBOM
func (s *Signer) AttestSBOM(ctx context.Context, imageURI, sbomPath string) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	// Attach SBOM as attestation
	// cosign attest --predicate SBOM_FILE --type cyclonedx IMAGE_URI
	var cmd *exec.Cmd
	if s.keyless {
		cmd = exec.CommandContext(timeoutCtx, "cosign", "attest", "--yes", "--predicate", sbomPath, "--type", "cyclonedx", imageURI)
	} else {
		cmd = exec.CommandContext(timeoutCtx, "cosign", "attest", "--key", "env://COSIGN_KEY", "--predicate", sbomPath, "--type", "cyclonedx", imageURI)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("SBOM attestation failed: %w (output: %s)", err, string(output))
	}

	return nil
}

// ValidateCosignInstalled checks if Cosign is installed and available
func (s *Signer) ValidateCosignInstalled() error {
	cmd := exec.Command("cosign", "version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("cosign is not installed or not in PATH. Install from: https://github.com/sigstore/cosign")
	}

	// Cosign is installed
	_ = output // version output not currently used
	return nil
}

// extractSignature extracts the signature digest from cosign output
func (s *Signer) extractSignature(output string) string {
	// Cosign output typically contains the signature digest
	// Example: "Pushing signature to: ghcr.io/madfam/my-service:sha256-abc123.sig"
	// We extract the digest portion

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Pushing signature to:") || strings.Contains(line, "sha256") {
			// Extract the signature reference
			parts := strings.Fields(line)
			if len(parts) > 0 {
				lastPart := parts[len(parts)-1]
				if strings.Contains(lastPart, "sha256") {
					return lastPart
				}
			}
		}
	}

	// If we can't extract a specific signature, return a truncated output
	if len(output) > 200 {
		return output[:200] + "..."
	}
	return output
}

// getSigningMethod returns a string describing the signing method
func (s *Signer) getSigningMethod() string {
	if s.keyless {
		return "keyless"
	}
	return "key-based"
}

// GenerateKeyPair generates a new Cosign key pair
// This is a helper function for initial setup
// WARNING: Store keys securely! Never commit to git.
func GenerateKeyPair(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "cosign", "generate-key-pair")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("key generation failed: %w (output: %s)", err, string(output))
	}

	fmt.Println("Key pair generated successfully!")
	fmt.Println("⚠️  IMPORTANT: Store cosign.key and cosign.pub securely")
	fmt.Println("⚠️  Add cosign.key to .gitignore to prevent accidental commits")
	fmt.Println("")
	fmt.Println("To use the keys:")
	fmt.Println("  export COSIGN_KEY=$(cat cosign.key)")
	fmt.Println("  export COSIGN_PUBLIC_KEY=$(cat cosign.pub)")
	fmt.Println("  export COSIGN_PASSWORD=<your-password>")

	return nil
}

// SignatureInfo represents stored signature metadata
type SignatureInfo struct {
	ImageURI      string    `json:"image_uri"`
	Signature     string    `json:"signature"`
	SignedAt      time.Time `json:"signed_at"`
	SigningMethod string    `json:"signing_method"`
	Verified      bool      `json:"verified"`
}

// GetSignatureInfo retrieves signature information for an image
// This queries the OCI registry for signature metadata
func (s *Signer) GetSignatureInfo(ctx context.Context, imageURI string) (*SignatureInfo, error) {
	// Verify the signature (this also retrieves signature info)
	verified, err := s.VerifySignature(ctx, imageURI)

	info := &SignatureInfo{
		ImageURI:      imageURI,
		Verified:      verified,
		SigningMethod: s.getSigningMethod(),
	}

	if err != nil {
		return info, err
	}

	return info, nil
}
