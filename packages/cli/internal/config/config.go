package config

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type Config struct {
	Environment string
	LogLevel    logrus.Level

	// API Configuration
	APIEndpoint string
	APIToken    string

	// Local Configuration
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
	viper.SetDefault("api-endpoint", "http://localhost:8080")
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
		ProjectDir:  viper.GetString("project-dir"),
		ConfigFile:  viper.GetString("config-file"),
	}

	return config, nil
}