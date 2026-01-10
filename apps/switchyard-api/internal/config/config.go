package config

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type Config struct {
	Environment string
	Port        string
	DatabaseURL string
	LogLevel    logrus.Level

	// Container Registry
	Registry         string
	RegistryUsername string
	RegistryPassword string

	// Authentication Mode
	AuthMode string // "local" (default) or "oidc"

	// OIDC Configuration
	OIDCIssuer           string
	OIDCClientID         string
	OIDCClientSecret     string
	OIDCRedirectURL      string
	PostLoginRedirectURL string // URL to redirect to after successful OIDC login (e.g., UI callback)

	// External Token Validation (for CLI/API direct access)
	ExternalJWKSURL      string // JWKS URL for validating external tokens (e.g., Janua)
	ExternalIssuer       string // Expected issuer for external tokens
	ExternalJWKSCacheTTL int    // Cache TTL in seconds for external JWKS

	// Token Expiration Settings
	AccessTokenExpireMinutes int // Access token lifetime in minutes (default: 15, set to 480 for 8 hours)
	RefreshTokenExpireDays   int // Refresh token lifetime in days (default: 7)

	// Janua Integration (for OAuth token retrieval)
	JanuaAPIURL string // Base URL for Janua API (e.g., https://api.janua.dev)

	// Kubernetes
	KubeConfig  string
	KubeContext string

	// Build Configuration
	BuildkitAddr  string
	BuildTimeout  int
	BuildWorkDir  string // Directory for cloning repositories during builds
	BuildCacheDir string // Directory for buildpack layer cache

	// Build Mode (in-process vs roundhouse worker)
	BuildMode        string // "in-process" (default) or "roundhouse"
	RoundhouseURL    string // URL of roundhouse worker (e.g., http://roundhouse:8080)
	RoundhouseAPIKey string // API key for authenticating with roundhouse
	SelfURL          string // This service's URL for callbacks (e.g., http://switchyard-api:4200)

	// Provenance / PR Approval
	GitHubToken         string // GitHub API token for PR verification
	GitHubWebhookSecret string // Secret for verifying GitHub webhook signatures

	// Compliance Webhooks
	ComplianceWebhooksEnabled bool
	VantaWebhookURL           string
	DrataWebhookURL           string

	// Secret Rotation (Vault)
	SecretRotationEnabled bool
	VaultAddress          string
	VaultToken            string
	VaultNamespace        string
	VaultPollInterval     int // Seconds

	// Redis Cache (for session revocation)
	RedisHost     string
	RedisPort     int
	RedisPassword string
}

func Load() (*Config, error) {
	viper.AutomaticEnv()
	viper.SetEnvPrefix("ENCLII")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	// Set defaults for development ONLY
	// SECURITY WARNING: These defaults are for local development only.
	// Production deployments MUST override these via environment variables.
	// Port 4200 per PORT_ALLOCATION.md in solarpunk-foundry (Enclii block: 4200-4299)
	viper.SetDefault("environment", "development")
	viper.SetDefault("port", "4200")
	viper.SetDefault("database-url", "postgres://janua:janua_dev@localhost:5432/enclii_dev?sslmode=disable")
	viper.SetDefault("log-level", "info")
	viper.SetDefault("registry", "ghcr.io/madfam-org")
	viper.SetDefault("registry-username", "")
	viper.SetDefault("registry-password", "")
	viper.SetDefault("auth-mode", "local") // Default to local bootstrap mode
	viper.SetDefault("oidc-issuer", "http://localhost:5556")
	viper.SetDefault("oidc-client-id", "enclii")
	viper.SetDefault("oidc-client-secret", "")
	viper.SetDefault("oidc-redirect-url", "http://localhost:4200/v1/auth/callback")
	viper.SetDefault("post-login-redirect-url", "")            // Empty = return JSON (for API clients)
	viper.SetDefault("external-jwks-url", "")                  // Empty = disabled
	viper.SetDefault("external-issuer", "")                    // Expected issuer for external tokens
	viper.SetDefault("external-jwks-cache-ttl", 300)           // 5 minutes default
	viper.SetDefault("access-token-expire-minutes", 15)        // 15 minutes default (set to 480 for 8 hours)
	viper.SetDefault("refresh-token-expire-days", 7)           // 7 days default
	viper.SetDefault("janua-api-url", "https://api.janua.dev") // Janua API for OAuth tokens
	viper.SetDefault("kube-config", os.Getenv("HOME")+"/.kube/config")
	viper.SetDefault("kube-context", "kind-enclii")
	viper.SetDefault("buildkit-addr", "docker://")
	viper.SetDefault("build-timeout", 1800) // 30 minutes
	viper.SetDefault("build-work-dir", "/tmp/enclii-builds")
	viper.SetDefault("build-cache-dir", "/var/cache/enclii-buildpacks")
	viper.SetDefault("build-mode", "in-process")                              // "in-process" or "roundhouse"
	viper.SetDefault("roundhouse-url", "http://roundhouse:8080")              // Roundhouse worker URL
	viper.SetDefault("roundhouse-api-key", "")                                // API key for roundhouse
	viper.SetDefault("self-url", "http://switchyard-api:4200")                // This service's URL for callbacks
	viper.SetDefault("github-webhook-secret", "")                             // Webhook disabled until secret configured
	viper.SetDefault("compliance-webhooks-enabled", false)
	viper.SetDefault("secret-rotation-enabled", false)
	viper.SetDefault("vault-poll-interval", 60) // Poll every 60 seconds
	viper.SetDefault("redis-host", "localhost")
	viper.SetDefault("redis-port", 6379)
	viper.SetDefault("redis-password", "")

	// Parse log level
	logLevelStr := viper.GetString("log-level")
	logLevel, err := logrus.ParseLevel(logLevelStr)
	if err != nil {
		return nil, err
	}

	config := &Config{
		Environment:               viper.GetString("environment"),
		Port:                      viper.GetString("port"),
		DatabaseURL:               viper.GetString("database-url"),
		LogLevel:                  logLevel,
		Registry:                  viper.GetString("registry"),
		RegistryUsername:          viper.GetString("registry-username"),
		RegistryPassword:          viper.GetString("registry-password"),
		AuthMode:                  viper.GetString("auth-mode"),
		OIDCIssuer:                viper.GetString("oidc-issuer"),
		OIDCClientID:              viper.GetString("oidc-client-id"),
		OIDCClientSecret:          viper.GetString("oidc-client-secret"),
		OIDCRedirectURL:           viper.GetString("oidc-redirect-url"),
		PostLoginRedirectURL:      viper.GetString("post-login-redirect-url"),
		ExternalJWKSURL:           viper.GetString("external-jwks-url"),
		ExternalIssuer:            viper.GetString("external-issuer"),
		ExternalJWKSCacheTTL:      viper.GetInt("external-jwks-cache-ttl"),
		AccessTokenExpireMinutes:  viper.GetInt("access-token-expire-minutes"),
		RefreshTokenExpireDays:    viper.GetInt("refresh-token-expire-days"),
		JanuaAPIURL:               viper.GetString("janua-api-url"),
		KubeConfig:                viper.GetString("kube-config"),
		KubeContext:               viper.GetString("kube-context"),
		BuildkitAddr:              viper.GetString("buildkit-addr"),
		BuildTimeout:              viper.GetInt("build-timeout"),
		BuildWorkDir:              viper.GetString("build-work-dir"),
		BuildCacheDir:             viper.GetString("build-cache-dir"),
		BuildMode:                 viper.GetString("build-mode"),
		RoundhouseURL:             viper.GetString("roundhouse-url"),
		RoundhouseAPIKey:          viper.GetString("roundhouse-api-key"),
		SelfURL:                   viper.GetString("self-url"),
		GitHubToken:               viper.GetString("github-token"),
		GitHubWebhookSecret:       viper.GetString("github-webhook-secret"),
		ComplianceWebhooksEnabled: viper.GetBool("compliance-webhooks-enabled"),
		VantaWebhookURL:           viper.GetString("vanta-webhook-url"),
		DrataWebhookURL:           viper.GetString("drata-webhook-url"),
		SecretRotationEnabled:     viper.GetBool("secret-rotation-enabled"),
		VaultAddress:              viper.GetString("vault-address"),
		VaultToken:                viper.GetString("vault-token"),
		VaultNamespace:            viper.GetString("vault-namespace"),
		VaultPollInterval:         viper.GetInt("vault-poll-interval"),
		RedisHost:                 viper.GetString("redis-host"),
		RedisPort:                 viper.GetInt("redis-port"),
		RedisPassword:             viper.GetString("redis-password"),
	}

	return config, nil
}
