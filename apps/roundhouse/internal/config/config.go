package config

import (
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	// Server
	APIPort  string `mapstructure:"API_PORT"`
	WorkerID string `mapstructure:"WORKER_ID"`

	// Database
	DatabaseURL string `mapstructure:"DATABASE_URL"`

	// Redis
	RedisURL string `mapstructure:"REDIS_URL"`

	// Build settings
	BuildWorkDir     string        `mapstructure:"BUILD_WORK_DIR"`
	BuildTimeout     time.Duration `mapstructure:"BUILD_TIMEOUT"`
	Registry         string        `mapstructure:"REGISTRY"`
	RegistryUser     string        `mapstructure:"REGISTRY_USER"`
	RegistryPassword string        `mapstructure:"REGISTRY_PASSWORD"`

	// Security
	GenerateSBOM bool   `mapstructure:"GENERATE_SBOM"`
	SignImages   bool   `mapstructure:"SIGN_IMAGES"`
	CosignKey    string `mapstructure:"COSIGN_KEY"`

	// Webhooks
	GitHubWebhookSecret    string `mapstructure:"GITHUB_WEBHOOK_SECRET"`
	GitLabWebhookSecret    string `mapstructure:"GITLAB_WEBHOOK_SECRET"`
	BitbucketWebhookSecret string `mapstructure:"BITBUCKET_WEBHOOK_SECRET"`

	// Callbacks
	SwitchyardInternalURL string `mapstructure:"SWITCHYARD_INTERNAL_URL"`
	SwitchyardAPIKey      string `mapstructure:"SWITCHYARD_API_KEY"`

	// Worker settings
	MaxConcurrentBuilds int           `mapstructure:"MAX_CONCURRENT_BUILDS"`
	PollInterval        time.Duration `mapstructure:"POLL_INTERVAL"`
}

func Load() (*Config, error) {
	viper.SetDefault("API_PORT", "8081")
	viper.SetDefault("BUILD_WORK_DIR", "/tmp/roundhouse-builds")
	viper.SetDefault("BUILD_TIMEOUT", 30*time.Minute)
	viper.SetDefault("GENERATE_SBOM", true)
	viper.SetDefault("SIGN_IMAGES", true)
	viper.SetDefault("MAX_CONCURRENT_BUILDS", 3)
	viper.SetDefault("POLL_INTERVAL", 5*time.Second)
	viper.SetDefault("REGISTRY", "ghcr.io")

	viper.AutomaticEnv()

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
