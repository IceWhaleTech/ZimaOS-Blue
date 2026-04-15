package main

import (
	"strings"

	"github.com/spf13/cobra"
)

var a11yCmd = &cobra.Command{
	Use:   "a11y [action] [options]",
	Short: "Run native desktop accessibility actions through the local Blue service",
	Long: `Run native desktop accessibility actions through the local Blue service.

Examples:
  blue a11y --action focus --app-name "Feishu,飞书,Lark"
  blue a11y message --app-name "Feishu,飞书,Lark" --conversation "Orca" --value "你好，Orca"
  blue a11y act --ref @5 --act-type click
`,
	Args:               cobra.ArbitraryArgs,
	DisableFlagParsing: true,
	Run: func(_ *cobra.Command, args []string) {
		runA11yCLIArgs(args)
	},
}

func init() {
	rootCmd.AddCommand(a11yCmd)
}

func dispatchA11yFastPath(args []string) bool {
	runA11yCLIArgs(args)
	return true
}

func runA11yCLIArgs(args []string) {
	params, positional := parseIPCArgs(args)
	params, positional = normalizeA11yCLIArgs(params, positional)
	if len(positional) > 0 && strings.TrimSpace(params["value"]) == "" {
		params["value"] = strings.Join(positional, " ")
	}
	ipcDispatchPreparedCommand("a11y", prepareIPCParams(params), true)
}

func normalizeA11yCLIArgs(params map[string]string, positional []string) (map[string]string, []string) {
	if params == nil {
		params = make(map[string]string)
	}

	if strings.TrimSpace(params["action"]) == "" && len(positional) > 0 {
		action := strings.TrimSpace(positional[0])
		if action != "" && !strings.HasPrefix(action, "-") && !strings.Contains(action, "=") {
			params["action"] = action
			positional = append([]string(nil), positional[1:]...)
		}
	}

	for key, value := range cloneStringMap(params) {
		canonical := canonicalA11yCLIParamKey(key)
		if canonical == "" || canonical == key {
			continue
		}
		if strings.TrimSpace(params[canonical]) == "" {
			params[canonical] = value
		}
	}

	return params, positional
}

func canonicalA11yCLIParamKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	replacer := strings.NewReplacer("-", "_")
	return replacer.Replace(key)
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(src))
	for key, value := range src {
		out[key] = value
	}
	return out
}
