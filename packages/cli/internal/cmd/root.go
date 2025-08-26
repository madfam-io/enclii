package cmd

import (
	"github.com/spf13/cobra"

	"github.com/madfam/enclii/packages/cli/internal/config"
)

func NewRootCommand(cfg *config.Config) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "enclii",
		Short: "🚂 Enclii CLI - Control & orchestration for your cloud",
		Long: `Enclii is a Railway-style platform that lets teams build, deploy, 
scale, and operate containerized services with guardrails.

Learn more at https://enclii.dev`,
	}

	// Add global flags
	rootCmd.PersistentFlags().String("api-endpoint", cfg.APIEndpoint, "API endpoint URL")
	rootCmd.PersistentFlags().String("log-level", "info", "Log level (debug, info, warn, error)")

	// Add subcommands
	rootCmd.AddCommand(NewInitCommand(cfg))
	rootCmd.AddCommand(NewDeployCommand(cfg))
	rootCmd.AddCommand(NewLogsCommand(cfg))
	rootCmd.AddCommand(NewPsCommand(cfg))
	rootCmd.AddCommand(NewRollbackCommand(cfg))
	rootCmd.AddCommand(NewVersionCommand())

	return rootCmd
}

func NewVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Println("enclii version 1.0.0-alpha")
			cmd.Println("Build: development")
		},
	}
}