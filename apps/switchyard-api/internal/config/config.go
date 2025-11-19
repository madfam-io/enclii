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
	Registry string

	// OIDC Configuration
	OIDCIssuer       string
	OIDCClientID     string
	OIDCClientSecret string

	// Kubernetes
	KubeConfig  string
	KubeContext string

	// Build Configuration
	BuildkitAddr    string
	BuildTimeout    int
	BuildWorkDir    string // Directory for cloning repositories during builds
	BuildCacheDir   string // Directory for buildpack layer cache
}

func Load() (*Config, error) {
	viper.AutomaticEnv()
	viper.SetEnvPrefix("ENCLII")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	// Set defaults
	viper.SetDefault("environment", "development")
	viper.SetDefault("port", "8080")
	viper.SetDefault("database-url", "postgres://postgres:password@localhost:5432/enclii_dev?sslmode=disable")
	viper.SetDefault("log-level", "info")
	viper.SetDefault("registry", "ghcr.io/madfam")
	viper.SetDefault("oidc-issuer", "http://localhost:5556")
	viper.SetDefault("oidc-client-id", "enclii")
	viper.SetDefault("oidc-client-secret", "enclii-secret")
	viper.SetDefault("kube-config", os.Getenv("HOME")+"/.kube/config")
	viper.SetDefault("kube-context", "kind-enclii")
	viper.SetDefault("buildkit-addr", "docker://")
	viper.SetDefault("build-timeout", 1800) // 30 minutes
	viper.SetDefault("build-work-dir", "/tmp/enclii-builds")
	viper.SetDefault("build-cache-dir", "/var/cache/enclii-buildpacks")

	// Parse log level
	logLevelStr := viper.GetString("log-level")
	logLevel, err := logrus.ParseLevel(logLevelStr)
	if err != nil {
		return nil, err
	}

	config := &Config{
		Environment:      viper.GetString("environment"),
		Port:            viper.GetString("port"),
		DatabaseURL:     viper.GetString("database-url"),
		LogLevel:        logLevel,
		Registry:        viper.GetString("registry"),
		OIDCIssuer:      viper.GetString("oidc-issuer"),
		OIDCClientID:    viper.GetString("oidc-client-id"),
		OIDCClientSecret: viper.GetString("oidc-client-secret"),
		KubeConfig:      viper.GetString("kube-config"),
		KubeContext:     viper.GetString("kube-context"),
		BuildkitAddr:    viper.GetString("buildkit-addr"),
		BuildTimeout:    viper.GetInt("build-timeout"),
		BuildWorkDir:    viper.GetString("build-work-dir"),
		BuildCacheDir:   viper.GetString("build-cache-dir"),
	}

	return config, nil
}