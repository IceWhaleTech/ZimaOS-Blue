package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management",
	Long: `Manage ZimaOS-Blue configuration.

Subcommands:
  get <key>           Get a configuration value
  set <key> <value>   Set a configuration value
  unset <key>         Remove a configuration value
  list                List all configuration values`,
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value",
	Args:  cobra.ExactArgs(1),
	Run:   runConfigGet,
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	Run:   runConfigSet,
}

var configUnsetCmd = &cobra.Command{
	Use:   "unset <key>",
	Short: "Remove a configuration value",
	Args:  cobra.ExactArgs(1),
	Run:   runConfigUnset,
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configuration values",
	Run:   runConfigList,
}

func init() {
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configUnsetCmd)
	configCmd.AddCommand(configListCmd)
}

func loadConfig() (*viper.Viper, error) {
	v := viper.New()
	v.SetConfigType("yaml")

	configDir := getConfigDir()
	configPath := filepath.Join(configDir, "config.yaml")

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	// Create config file if it doesn't exist
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := os.WriteFile(configPath, []byte("# ZimaOS-Blue Configuration\n"), 0600); err != nil {
			return nil, fmt.Errorf("failed to create config file: %w", err)
		}
	}

	v.SetConfigFile(configPath)
	if err := v.ReadInConfig(); err != nil {
		// Ignore file not found errors
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
	}

	return v, nil
}

func saveConfig(v *viper.Viper) error {
	return v.WriteConfig()
}

func runConfigGet(cmd *cobra.Command, args []string) {
	key := args[0]

	v, err := loadConfig()
	if err != nil {
		printConfigError("Failed to load config", err)
		return
	}

	value := v.Get(key)
	if value == nil {
		if jsonOutput {
			printJSON(map[string]interface{}{
				"key":   key,
				"value": nil,
				"found": false,
			})
		} else {
			fmt.Printf("Key '%s' not found\n", key)
		}
		return
	}

	if jsonOutput {
		printJSON(map[string]interface{}{
			"key":   key,
			"value": value,
			"found": true,
		})
	} else {
		fmt.Printf("%s = %v\n", key, value)
	}
}

func runConfigSet(cmd *cobra.Command, args []string) {
	key := args[0]
	value := args[1]

	v, err := loadConfig()
	if err != nil {
		printConfigError("Failed to load config", err)
		return
	}

	// Try to parse value as JSON for complex types
	var parsedValue interface{}
	if err := json.Unmarshal([]byte(value), &parsedValue); err != nil {
		// Not JSON, use as string
		parsedValue = value
	}

	v.Set(key, parsedValue)

	if err := saveConfig(v); err != nil {
		printConfigError("Failed to save config", err)
		return
	}

	if jsonOutput {
		printJSON(map[string]interface{}{
			"key":     key,
			"value":   parsedValue,
			"success": true,
		})
	} else {
		if noColor {
			fmt.Printf("Set %s = %v\n", key, parsedValue)
		} else {
			fmt.Printf("\033[32m✓\033[0m Set %s = %v\n", key, parsedValue)
		}
	}
}

func runConfigUnset(cmd *cobra.Command, args []string) {
	key := args[0]

	v, err := loadConfig()
	if err != nil {
		printConfigError("Failed to load config", err)
		return
	}

	// Check if key exists
	if !v.IsSet(key) {
		if jsonOutput {
			printJSON(map[string]interface{}{
				"key":     key,
				"success": false,
				"error":   "key not found",
			})
		} else {
			fmt.Printf("Key '%s' not found\n", key)
		}
		return
	}

	// Viper doesn't have a direct unset, so we need to work around it
	// by getting all settings, removing the key, and rewriting
	allSettings := v.AllSettings()
	deleteNestedKey(allSettings, strings.Split(key, "."))

	// Create new viper with updated settings
	newV := viper.New()
	newV.SetConfigType("yaml")
	newV.SetConfigFile(v.ConfigFileUsed())
	for k, val := range allSettings {
		newV.Set(k, val)
	}

	if err := newV.WriteConfig(); err != nil {
		printConfigError("Failed to save config", err)
		return
	}

	if jsonOutput {
		printJSON(map[string]interface{}{
			"key":     key,
			"success": true,
		})
	} else {
		if noColor {
			fmt.Printf("Unset %s\n", key)
		} else {
			fmt.Printf("\033[32m✓\033[0m Unset %s\n", key)
		}
	}
}

func runConfigList(cmd *cobra.Command, args []string) {
	v, err := loadConfig()
	if err != nil {
		printConfigError("Failed to load config", err)
		return
	}

	settings := v.AllSettings()

	if jsonOutput {
		printJSON(settings)
	} else {
		if len(settings) == 0 {
			fmt.Println("No configuration values set")
			return
		}

		fmt.Println("Configuration:")
		printSettings("", settings)
	}
}

func printSettings(prefix string, settings map[string]interface{}) {
	for key, value := range settings {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}

		switch v := value.(type) {
		case map[string]interface{}:
			printSettings(fullKey, v)
		default:
			if noColor {
				fmt.Printf("  %s = %v\n", fullKey, value)
			} else {
				fmt.Printf("  \033[36m%s\033[0m = %v\n", fullKey, value)
			}
		}
	}
}

func deleteNestedKey(m map[string]interface{}, keys []string) {
	if len(keys) == 0 {
		return
	}

	if len(keys) == 1 {
		delete(m, keys[0])
		return
	}

	if nested, ok := m[keys[0]].(map[string]interface{}); ok {
		deleteNestedKey(nested, keys[1:])
		// Remove empty maps
		if len(nested) == 0 {
			delete(m, keys[0])
		}
	}
}

func printConfigError(msg string, err error) {
	if jsonOutput {
		printJSON(map[string]interface{}{
			"success": false,
			"error":   msg,
			"details": err.Error(),
		})
	} else {
		if noColor {
			fmt.Printf("Error: %s\n", msg)
		} else {
			fmt.Printf("\033[31mError:\033[0m %s\n", msg)
		}
		if verbose && err != nil {
			fmt.Printf("Details: %v\n", err)
		}
	}
	os.Exit(1)
}
