package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func dispatchPDFNativeFastPath(args []string) bool {
	parsed := parsePDFCLIArgs(args)
	if len(parsed) == 0 {
		return false
	}
	action := strings.ToLower(strings.TrimSpace(fmt.Sprint(parsed["action"])))
	if action == "" {
		action = "read"
		parsed["action"] = action
	}
	if action != "create" {
		return false
	}

	result, err := tools.NewPDFTool(nil).Execute(context.Background(), parsed)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		cliDispatchExit(1)
		return true
	}
	if jsonOutput {
		if raw, ok := result.(string); ok && json.Valid([]byte(raw)) {
			fmt.Println(raw)
			return true
		}
		printJSON(result)
	} else {
		fmt.Println(result)
	}
	return true
}

func parsePDFCLIArgs(args []string) map[string]interface{} {
	parsed := make(map[string]interface{}, len(args))
	for _, arg := range args {
		arg = strings.TrimSpace(arg)
		if arg == "" {
			continue
		}
		key, value, ok := strings.Cut(arg, "=")
		if !ok {
			if _, hasAction := parsed["action"]; !hasAction && !strings.HasPrefix(arg, "-") {
				parsed["action"] = arg
			}
			continue
		}
		key = strings.TrimLeft(strings.TrimSpace(key), "-")
		key = strings.ReplaceAll(key, "-", "_")
		if key == "" {
			continue
		}
		parsed[key] = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	return parsed
}
