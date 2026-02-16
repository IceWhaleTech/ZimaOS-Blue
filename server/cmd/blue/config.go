package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
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

func configFilePath() string {
	configDir := getConfigDir()
	return filepath.Join(configDir, "config.yaml")
}

func loadConfigMap() (map[string]interface{}, error) {
	path := configFilePath()

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]interface{}), nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var m map[string]interface{}
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	if m == nil {
		m = make(map[string]interface{})
	}
	return m, nil
}

func saveConfigMap(m map[string]interface{}) error {
	data, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return os.WriteFile(configFilePath(), data, 0600)
}

// getNestedKey retrieves a value from a nested map using dot-separated key.
func getNestedKey(m map[string]interface{}, key string) (interface{}, bool) {
	parts := strings.Split(key, ".")
	var current interface{} = m
	for _, p := range parts {
		cm, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current, ok = cm[p]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

// setNestedKey sets a value in a nested map using dot-separated key.
func setNestedKey(m map[string]interface{}, key string, value interface{}) {
	parts := strings.Split(key, ".")
	current := m
	for i := 0; i < len(parts)-1; i++ {
		next, ok := current[parts[i]].(map[string]interface{})
		if !ok {
			next = make(map[string]interface{})
			current[parts[i]] = next
		}
		current = next
	}
	current[parts[len(parts)-1]] = value
}

func runConfigGet(cmd *cobra.Command, args []string) {
	key := args[0]

	m, err := loadConfigMap()
	if err != nil {
		printConfigError("Failed to load config", err)
		return
	}

	value, found := getNestedKey(m, key)
	if !found {
		if jsonOutput {
			printJSON(map[string]interface{}{"key": key, "value": nil, "found": false})
		} else {
			fmt.Printf("Key '%s' not found\n", key)
		}
		return
	}

	if jsonOutput {
		printJSON(map[string]interface{}{"key": key, "value": value, "found": true})
	} else {
		fmt.Printf("%s = %v\n", key, value)
	}
}

func runConfigSet(cmd *cobra.Command, args []string) {
	key := args[0]
	value := args[1]

	m, err := loadConfigMap()
	if err != nil {
		printConfigError("Failed to load config", err)
		return
	}

	// Try to parse value as JSON for complex types
	var parsedValue interface{}
	if err := json.Unmarshal([]byte(value), &parsedValue); err != nil {
		parsedValue = value
	}

	setNestedKey(m, key, parsedValue)

	if err := saveConfigMap(m); err != nil {
		printConfigError("Failed to save config", err)
		return
	}

	if jsonOutput {
		printJSON(map[string]interface{}{"key": key, "value": parsedValue, "success": true})
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

	m, err := loadConfigMap()
	if err != nil {
		printConfigError("Failed to load config", err)
		return
	}

	if _, found := getNestedKey(m, key); !found {
		if jsonOutput {
			printJSON(map[string]interface{}{"key": key, "success": false, "error": "key not found"})
		} else {
			fmt.Printf("Key '%s' not found\n", key)
		}
		return
	}

	deleteNestedKey(m, strings.Split(key, "."))

	if err := saveConfigMap(m); err != nil {
		printConfigError("Failed to save config", err)
		return
	}

	if jsonOutput {
		printJSON(map[string]interface{}{"key": key, "success": true})
	} else {
		if noColor {
			fmt.Printf("Unset %s\n", key)
		} else {
			fmt.Printf("\033[32m✓\033[0m Unset %s\n", key)
		}
	}
}

func runConfigList(cmd *cobra.Command, args []string) {
	m, err := loadConfigMap()
	if err != nil {
		printConfigError("Failed to load config", err)
		return
	}

	if jsonOutput {
		printJSON(m)
	} else {
		if len(m) == 0 {
			fmt.Println("No configuration values set")
			return
		}
		fmt.Println("Configuration:")
		printSettings("", m)
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
		if len(nested) == 0 {
			delete(m, keys[0])
		}
	}
}

func printConfigError(msg string, err error) {
	if jsonOutput {
		printJSON(map[string]interface{}{"success": false, "error": msg, "details": err.Error()})
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
