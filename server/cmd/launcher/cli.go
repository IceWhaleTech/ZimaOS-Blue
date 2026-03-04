package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	skillEmbed "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill/embedded"
)

// parseCLIFlags extracts global flags from args, returns positional args and flag state.
type cliFlags struct {
	devMode bool
	noColor bool
	json    bool
	verbose bool
}

func parseCLIFlags(args []string) (positional []string, flags cliFlags) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dev":
			flags.devMode = true
		case "--no-color":
			flags.noColor = true
		case "--json":
			flags.json = true
		case "-v", "--verbose":
			flags.verbose = true
		case "--config", "--profile":
			if i+1 < len(args) {
				i++ // skip value
			}
		case "--":
			positional = append(positional, args[i+1:]...)
			return
		default:
			positional = append(positional, args[i])
		}
	}
	return
}

// tryFastCmd handles commands that can run entirely in the launcher process
// without exec-ing .bluecli. Returns true if handled.
func tryFastCmd(args []string) bool {
	positional, flags := parseCLIFlags(args)
	if len(positional) == 0 {
		return false
	}

	cmd := positional[0]
	switch cmd {
	case "help":
		runLauncherHelp(positional[1:])
		return true
	case "status":
		runLauncherStatus(flags)
		return true
	case "health":
		runLauncherHealth(flags)
		return true
	default:
		return false
	}
}

func printHelp() {
	fmt.Print(`ZimaOS-Blue - A Local-first Agent Runtime for Builders with Bolder Mind

Usage:
  blue [command]

Available Commands:
  (no command)  Start the server
  status        Show service health and recent activity
  health        Fetch health from running service
  version       Show version information
  doctor        Diagnose common issues
  config        Manage configuration
  models        Manage LLM models
  plugins       Manage plugins
  skills        Manage skills
  sessions      Manage chat sessions
  cron          Manage scheduled tasks
  logs          View service logs
  media         Media generation commands
  gateway       Manage API gateway
  remind        Manage reminders
  help          Show this help message

Flags:
      --config string   config file
      --dev             isolate state under ~/.zimaos-blue-dev
      --profile string  isolate state under ~/.zimaos-blue-<name>
      --no-color        disable ANSI colors
      --json            output in JSON format
  -v, --verbose         verbose output
  -h, --help            help for blue

Use "blue help <skill>" to show full SKILL.md manuals.
Use "blue [command] --help" for command flags.
	`)
}

func runLauncherHelp(args []string) {
	if len(args) == 0 {
		printHelp()
		return
	}

	topic := strings.TrimSpace(args[0])
	if topic == "" {
		printHelp()
		return
	}

	if manual, source, ok := findEmbeddedSkillManual(topic); ok {
		fmt.Printf("Source: %s\n\n", source)
		fmt.Print(manual)
		if !strings.HasSuffix(manual, "\n") {
			fmt.Println()
		}
		return
	}

	if printLauncherCommandHelp(topic) {
		return
	}

	fmt.Fprintf(os.Stderr, "Error: unknown help topic %q\n", strings.Join(args, " "))
	os.Exit(1)
}

func findEmbeddedSkillManual(topic string) (manual string, source string, ok bool) {
	for _, skillID := range candidateSkillIDs(topic) {
		embedPath := filepath.ToSlash(filepath.Join("skills", skillID, "SKILL.md"))
		data, err := skillEmbed.SkillsFS.ReadFile(embedPath)
		if err == nil {
			return string(data), "embedded://" + embedPath, true
		}
	}
	return "", "", false
}

func candidateSkillIDs(topic string) []string {
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return nil
	}

	ids := []string{topic}
	if dot := strings.IndexByte(topic, '.'); dot > 0 {
		ids = append(ids, topic[:dot])
	}

	out := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func printLauncherCommandHelp(topic string) bool {
	commandHelp := map[string]string{
		"help":     "Show help for a command or skill.\nUsage: blue help [topic]",
		"status":   "Show service health and recent activity.\nUsage: blue status [--dev] [--json]",
		"health":   "Fetch health from running service.\nUsage: blue health [--dev] [--json]",
		"version":  "Show version information.\nUsage: blue version",
		"doctor":   "Diagnose common issues.\nUsage: blue doctor [--fix]",
		"config":   "Manage configuration.\nUsage: blue config [subcommand]",
		"models":   "Manage LLM models.\nUsage: blue models [subcommand]",
		"plugins":  "Manage plugins.\nUsage: blue plugins [subcommand]",
		"skills":   "Manage skills.\nUsage: blue skills [subcommand]",
		"sessions": "Manage chat sessions.\nUsage: blue sessions [subcommand]",
		"cron":     "Manage scheduled tasks.\nUsage: blue cron [subcommand]",
		"logs":     "View service logs.\nUsage: blue logs [flags]",
		"media":    "Media generation commands.\nUsage: blue media [subcommand]",
		"gateway":  "Manage API gateway.\nUsage: blue gateway [subcommand]",
		"remind":   "Manage reminders.\nUsage: blue remind [subcommand]",
	}
	doc, ok := commandHelp[topic]
	if !ok {
		return false
	}

	fmt.Printf("%s\n", doc)
	return true
}

func servicePort(flags cliFlags) int {
	if flags.devMode {
		return 8081
	}
	return 8080
}

func runLauncherStatus(flags cliFlags) {
	port := servicePort(flags)
	baseURL := fmt.Sprintf("http://localhost:%d", port)
	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		if flags.json {
			printJSONObj(map[string]any{"status": "error", "errors": []string{"Service not running or unreachable"}})
		} else {
			printColorLine(flags.noColor, "31", "Status: ERROR")
			fmt.Fprintf(os.Stderr, "Error: Service not running or unreachable\n")
			if flags.verbose {
				fmt.Fprintf(os.Stderr, "Details: %v\n", err)
			}
		}
		os.Exit(1)
		return
	}
	defer resp.Body.Close()

	var health map[string]any
	json.NewDecoder(resp.Body).Decode(&health)

	status := "healthy"
	if resp.StatusCode != http.StatusOK {
		status = "unhealthy"
	}
	if s, ok := health["status"].(string); ok {
		status = s
	}
	ver, _ := health["version"].(string)

	if flags.json {
		health["status"] = status
		printJSONObj(health)
	} else {
		color := "32"
		if status != "healthy" && status != "ok" {
			color = "31"
		}
		printColorLine(flags.noColor, color, "Status: "+status)
		if ver != "" {
			fmt.Printf("Version: %s\n", ver)
		}
	}

	if status != "healthy" && status != "ok" {
		os.Exit(1)
	}
}

func runLauncherHealth(flags cliFlags) {
	port := servicePort(flags)
	baseURL := fmt.Sprintf("http://localhost:%d", port)
	client := &http.Client{Timeout: 5 * time.Second}

	start := time.Now()
	resp, err := client.Get(baseURL + "/health")
	latency := time.Since(start)

	if err != nil {
		if flags.json {
			printJSONObj(map[string]any{
				"status":    "error",
				"timestamp": time.Now().Format(time.RFC3339),
				"components": map[string]any{
					"api": map[string]any{"status": "error", "message": "Service not running or unreachable"},
				},
			})
		} else {
			printColorLine(flags.noColor, "31", "Health: ERROR")
			fmt.Fprintf(os.Stderr, "Error: Service not running or unreachable\n")
			if flags.verbose {
				fmt.Fprintf(os.Stderr, "Details: %v\n", err)
			}
		}
		os.Exit(1)
		return
	}
	defer resp.Body.Close()

	var health map[string]any
	json.NewDecoder(resp.Body).Decode(&health)

	status := "healthy"
	if s, ok := health["status"].(string); ok {
		status = s
	}
	if resp.StatusCode != http.StatusOK && status == "healthy" {
		status = "unhealthy"
	}
	ver, _ := health["version"].(string)

	if flags.json {
		health["status"] = status
		printJSONObj(health)
	} else {
		color := "32"
		if status != "healthy" && status != "ok" {
			color = "31"
		}
		printColorLine(flags.noColor, color, "Health: "+status)
		if ver != "" {
			fmt.Printf("Version: %s\n", ver)
		}
		fmt.Printf("Latency: %s\n", latency.Round(time.Millisecond))
	}

	if status != "healthy" && status != "ok" {
		os.Exit(1)
	}
}

func printColorLine(noColor bool, colorCode, text string) {
	if noColor || !strings.Contains(os.Getenv("TERM"), "color") && os.Getenv("TERM") == "" {
		fmt.Println(text)
	} else {
		fmt.Printf("\033[%sm%s\033[0m\n", colorCode, text)
	}
}

func printJSONObj(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}
