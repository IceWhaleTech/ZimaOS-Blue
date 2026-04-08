package main

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	// Global flags
	cfgFile           string
	devMode           bool
	profile           string
	noColor           bool
	jsonOutput        bool
	verbose           bool
	disableIntercepts bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "blue",
	Short: "ZimaOS-Blue - A Local-first Agent Runtime for Builders with Bolder Mind",
	Long: `ZimaOS-Blue is a A Local-first Agent Runtime for Builders with Bolder Mind that provides
AI assistant capabilities with local-first architecture.

It supports multiple LLM providers, skills, and
integrates with various automation and productivity services.`,
	Version: version,
	// Run the server by default if no subcommand is provided
	RunE: func(cmd *cobra.Command, args []string) error {
		// If no subcommand, run the server in the normal foreground UX path.
		return runForegroundServer()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ./config.yaml, ./config/, /etc/zimaos-blue/, ~/.zimaos-blue/)")
	rootCmd.PersistentFlags().BoolVar(&devMode, "dev", false, "isolate state under ~/.zimaos-blue-dev with shifted default ports")
	rootCmd.PersistentFlags().StringVar(&profile, "profile", "", "isolate state under ~/.zimaos-blue-<name>")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable ANSI colors")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output in JSON format")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVar(&disableIntercepts, "no-intercept", false, "disable chat prompt interception for controlled runs such as pinchbench")

	// Add subcommands
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(healthCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(gatewayCmd)
	rootCmd.AddCommand(modelsCmd)
	rootCmd.AddCommand(versionCmd)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag
		return
	}

	// If --dev or --profile is set, use the specific config directory
	if devMode || profile != "" {
		configDir := getConfigDir()
		cfgFile = filepath.Join(configDir, "config.yaml")
		return
	}

	// Otherwise, let config.Load() search in order:
	// 1. Current directory (./config.yaml)
	// 2. ./config/config.yaml
	// 3. /etc/zimaos-blue/config.yaml
	// 4. $HOME/.zimaos-blue/config.yaml (fallback)
	// cfgFile remains empty, config.Load will handle the search
}

// getConfigDir returns the configuration directory based on flags
func getConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}

	if devMode {
		return filepath.Join(home, ".zimaos-blue-dev")
	}

	if profile != "" {
		return filepath.Join(home, ".zimaos-blue-"+profile)
	}

	return filepath.Join(home, ".zimaos-blue")
}

// getDataDir returns the data directory based on flags
func getDataDir() string {
	return filepath.Join(getConfigDir(), "data")
}

// getLogsDir returns the logs directory based on flags
func getLogsDir() string {
	return filepath.Join(getConfigDir(), "logs")
}
