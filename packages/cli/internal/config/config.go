package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Credentials stores OAuth tokens from login
type Credentials struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type"`
	ExpiresAt    time.Time `json:"expires_at"`
	Issuer       string    `json:"issuer"`
}

type Config struct {
	Environment string
	LogLevel    logrus.Level

	// API Configuration
	APIEndpoint string
	APIToken    string

	// OAuth Credentials (loaded from ~/.enclii/credentials.json)
	Credentials *Credentials

	// Project Configuration
	Project    string
	ProjectDir string
	ConfigFile string
}

func Load() (*Config, error) {
	viper.AutomaticEnv()
	viper.SetEnvPrefix("ENCLII")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	// Set defaults
	viper.SetDefault("environment", "development")
	viper.SetDefault("log-level", "info")
	viper.SetDefault("api-endpoint", "https://api.enclii.dev")
	viper.SetDefault("project", "default")
	viper.SetDefault("project-dir", ".")
	viper.SetDefault("config-file", os.Getenv("HOME")+"/.enclii/config.yml")

	// Parse log level
	logLevelStr := viper.GetString("log-level")
	logLevel, err := logrus.ParseLevel(logLevelStr)
	if err != nil {
		return nil, err
	}

	config := &Config{
		Environment: viper.GetString("environment"),
		LogLevel:    logLevel,
		APIEndpoint: viper.GetString("api-endpoint"),
		APIToken:    viper.GetString("api-token"),
		Project:     viper.GetString("project"),
		ProjectDir:  viper.GetString("project-dir"),
		ConfigFile:  viper.GetString("config-file"),
	}

	// Load OAuth credentials if available
	creds, err := loadCredentials()
	if err == nil && creds != nil {
		config.Credentials = creds
		// Use OAuth token if no API token is explicitly set
		if config.APIToken == "" && creds.AccessToken != "" {
			// Check if token is still valid
			if time.Now().Before(creds.ExpiresAt) {
				config.APIToken = creds.AccessToken
			}
		}
	}

	return config, nil
}

// loadCredentials loads saved OAuth credentials from disk
func loadCredentials() (*Credentials, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	credsPath := filepath.Join(home, ".enclii", "credentials.json")
	data, err := os.ReadFile(credsPath)
	if err != nil {
		return nil, err
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}

	return &creds, nil
}

// GetCredentialsPath returns the path to the credentials file
func GetCredentialsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".enclii", "credentials.json")
}
