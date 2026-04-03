package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type CheckItem struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // ok | missing | warning
	Details string `json:"details,omitempty"`
}

type CategoryReport struct {
	Category string      `json:"category"`
	Items    []CheckItem `json:"items"`
}

type GapReport struct {
	GeneratedAt string           `json:"generated_at"`
	RepoRoot    string           `json:"repo_root"`
	Summary     Summary          `json:"summary"`
	Categories  []CategoryReport `json:"categories"`
}

type Summary struct {
	TotalChecks int `json:"total_checks"`
	OK          int `json:"ok"`
	Missing     int `json:"missing"`
	Warning     int `json:"warning"`
}

type checker struct {
	repoRoot string
}

func main() {
	var (
		repoRoot      string
		jsonOutput    string
		mdOutput      string
		failOnMissing bool
	)
	flag.StringVar(&repoRoot, "repo-root", "..", "repository root path")
	flag.StringVar(&jsonOutput, "json-output", "../docs/reports/chat-chain-gap.json", "JSON report output path")
	flag.StringVar(&mdOutput, "md-output", "../docs/reports/chat-chain-gap.md", "Markdown report output path")
	flag.BoolVar(&failOnMissing, "fail-on-missing", true, "exit non-zero if missing checks are found")
	flag.Parse()

	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		fatalf("resolve repo root: %v", err)
	}
	c := &checker{repoRoot: absRoot}

	report, err := c.run()
	if err != nil {
		fatalf("run gap checks: %v", err)
	}

	if err := writeJSON(jsonOutput, report); err != nil {
		fatalf("write JSON report: %v", err)
	}
	if err := writeMarkdown(mdOutput, report); err != nil {
		fatalf("write Markdown report: %v", err)
	}

	fmt.Printf("gap report generated:\n  json: %s\n  md:   %s\n", jsonOutput, mdOutput)
	fmt.Printf("summary: total=%d ok=%d missing=%d warning=%d\n",
		report.Summary.TotalChecks, report.Summary.OK, report.Summary.Missing, report.Summary.Warning)

	if failOnMissing && report.Summary.Missing > 0 {
		os.Exit(2)
	}
}

func fatalf(format string, args ...interface{}) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func (c *checker) run() (*GapReport, error) {
	routeChecks, err := c.checkRoutes()
	if err != nil {
		return nil, err
	}
	gatewayChecks, err := c.checkGatewayMethods()
	if err != nil {
		return nil, err
	}
	settingsChecks, err := c.checkSettings()
	if err != nil {
		return nil, err
	}
	hookChecks, err := c.checkHookLifecycle()
	if err != nil {
		return nil, err
	}

	categories := []CategoryReport{
		{Category: "routes_missing", Items: routeChecks},
		{Category: "gateway_methods_missing", Items: gatewayChecks},
		{Category: "settings_config_missing", Items: settingsChecks},
		{Category: "hook_lifecycle_missing", Items: hookChecks},
	}

	var summary Summary
	for _, cat := range categories {
		for _, item := range cat.Items {
			summary.TotalChecks++
			switch item.Status {
			case "ok":
				summary.OK++
			case "missing":
				summary.Missing++
			default:
				summary.Warning++
			}
		}
	}

	return &GapReport{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		RepoRoot:    c.repoRoot,
		Summary:     summary,
		Categories:  categories,
	}, nil
}

func (c *checker) checkRoutes() ([]CheckItem, error) {
	content, err := c.readFile("server/internal/bootstrap/routes.go")
	if err != nil {
		return nil, err
	}

	checks := []struct {
		name    string
		marker  string
		details string
	}{
		{
			name:    "chat_routes_registered",
			marker:  "deps.ChatHandler.RegisterRoutes(v1)",
			details: "expected chat API routes to be registered on /api/v1",
		},
		{
			name:    "gateway_routes_registered",
			marker:  "deps.GatewayHandler.RegisterRoutes(",
			details: "expected gateway HTTP/WS routes to be wired into bootstrap routes",
		},
	}

	items := make([]CheckItem, 0, len(checks))
	for _, chk := range checks {
		status := "ok"
		detail := ""
		if !strings.Contains(content, chk.marker) {
			status = "missing"
			detail = chk.details
		}
		items = append(items, CheckItem{Name: chk.name, Status: status, Details: detail})
	}
	return items, nil
}

func (c *checker) checkGatewayMethods() ([]CheckItem, error) {
	frontendContent, err := c.readFile("web/src/api/gateway.ts")
	if err != nil {
		return nil, err
	}
	frontendMethods := parseGatewayMethodsFromFrontend(frontendContent)

	backendMethods, err := c.parseGatewayMethodsFromBackend()
	if err != nil {
		return nil, err
	}

	items := make([]CheckItem, 0, len(frontendMethods))
	for _, m := range frontendMethods {
		_, ok := backendMethods[m]
		item := CheckItem{
			Name:   m,
			Status: "ok",
		}
		if !ok {
			item.Status = "missing"
			item.Details = "frontend gateway method is not registered by backend gateway runtime"
		}
		items = append(items, item)
	}
	return items, nil
}

func (c *checker) checkSettings() ([]CheckItem, error) {
	backend, err := c.readFile("server/internal/server/settings_handler.go")
	if err != nil {
		return nil, err
	}
	webAPI, err := c.readFile("web/src/api/settings.ts")
	if err != nil {
		return nil, err
	}
	webStore, err := c.readFile("web/src/stores/settings.ts")
	if err != nil {
		return nil, err
	}

	keys := []string{
		"skill_selector_mode",
		"skill_selector_confidence_threshold",
		"prompt_policy_version",
		"prompt_policy_profile",
		"agent_loop_policy_max_tool_rounds",
		"agent_loop_policy_max_auto_continue",
		"agent_loop_policy_pseudo_tool_call_budget",
		"agent_loop_policy_action_pledge_budget",
		"agent_loop_policy_missing_todo_budget",
		"agent_loop_policy_pending_todo_budget",
	}

	items := make([]CheckItem, 0, len(keys))
	for _, key := range keys {
		missing := make([]string, 0, 3)
		if !strings.Contains(backend, key) {
			missing = append(missing, "backend")
		}
		if !strings.Contains(webAPI, key) {
			missing = append(missing, "web-api")
		}
		if !strings.Contains(webStore, key) {
			missing = append(missing, "web-store")
		}
		item := CheckItem{Name: key, Status: "ok"}
		if len(missing) > 0 {
			item.Status = "missing"
			item.Details = "missing in: " + strings.Join(missing, ", ")
		}
		items = append(items, item)
	}

	return items, nil
}

func (c *checker) checkHookLifecycle() ([]CheckItem, error) {
	allGoFiles, err := listFiles(filepath.Join(c.repoRoot, "server/internal"), ".go")
	if err != nil {
		return nil, err
	}

	registerCount := 0
	triggerCallSites := make([]string, 0, 8)
	for _, file := range allGoFiles {
		content, readErr := os.ReadFile(file)
		if readErr != nil {
			return nil, readErr
		}
		text := string(content)
		if strings.Contains(text, "RegisterHook(") {
			registerCount++
		}
		if strings.Contains(text, "TriggerHook(") {
			rel, _ := filepath.Rel(c.repoRoot, file)
			triggerCallSites = append(triggerCallSites, filepath.ToSlash(rel))
		}
	}

	nonRegistryTriggers := make([]string, 0, len(triggerCallSites))
	for _, site := range triggerCallSites {
		if strings.HasSuffix(site, "/plugin/registry.go") {
			continue
		}
		if strings.HasSuffix(site, "_test.go") {
			continue
		}
		nonRegistryTriggers = append(nonRegistryTriggers, site)
	}
	sort.Strings(nonRegistryTriggers)

	items := []CheckItem{
		{
			Name:   "register_hook_available",
			Status: "ok",
		},
		{
			Name:   "hook_trigger_lifecycle_wired",
			Status: "ok",
		},
	}
	if registerCount == 0 {
		items[0].Status = "missing"
		items[0].Details = "no RegisterHook usage found under server/internal"
	}
	if len(nonRegistryTriggers) == 0 {
		items[1].Status = "missing"
		items[1].Details = "no TriggerHook call sites found in runtime flow (outside plugin registry/tests)"
	} else {
		items[1].Details = "trigger call sites: " + strings.Join(nonRegistryTriggers, ", ")
	}
	return items, nil
}

func parseGatewayMethodsFromFrontend(content string) []string {
	re := regexp.MustCompile(`this\.request\(['"]([^'"]+)['"]`)
	matches := re.FindAllStringSubmatch(content, -1)
	uniq := map[string]struct{}{}
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		uniq[m[1]] = struct{}{}
	}
	out := make([]string, 0, len(uniq))
	for k := range uniq {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (c *checker) parseGatewayMethodsFromBackend() (map[string]struct{}, error) {
	allGoFiles, err := listFiles(filepath.Join(c.repoRoot, "server"), ".go")
	if err != nil {
		return nil, err
	}
	re := regexp.MustCompile(`RegisterHandler\(\s*"([^"]+)"`)
	methods := map[string]struct{}{}

	for _, file := range allGoFiles {
		content, readErr := os.ReadFile(file)
		if readErr != nil {
			return nil, readErr
		}
		matches := re.FindAllStringSubmatch(string(content), -1)
		for _, m := range matches {
			if len(m) < 2 {
				continue
			}
			methods[m[1]] = struct{}{}
		}
	}
	return methods, nil
}

func writeJSON(outputPath string, report *GapReport) error {
	if report == nil {
		return errors.New("nil report")
	}
	absPath, err := filepath.Abs(outputPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0o750); err != nil {
		return err
	}
	b, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(absPath, append(b, '\n'), 0o640)
}

func writeMarkdown(outputPath string, report *GapReport) error {
	if report == nil {
		return errors.New("nil report")
	}
	absPath, err := filepath.Abs(outputPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0o750); err != nil {
		return err
	}

	var sb strings.Builder
	sb.WriteString("# Chat Chain Gap Report\n\n")
	sb.WriteString("- Generated at: `" + report.GeneratedAt + "`\n")
	sb.WriteString("- Repo root: `" + report.RepoRoot + "`\n")
	sb.WriteString(fmt.Sprintf("- Summary: total=%d, ok=%d, missing=%d, warning=%d\n\n",
		report.Summary.TotalChecks, report.Summary.OK, report.Summary.Missing, report.Summary.Warning))

	for _, cat := range report.Categories {
		sb.WriteString("## " + cat.Category + "\n\n")
		sb.WriteString("| check | status | details |\n")
		sb.WriteString("|---|---|---|\n")
		for _, item := range cat.Items {
			detail := strings.ReplaceAll(item.Details, "|", "\\|")
			if detail == "" {
				detail = "-"
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", item.Name, item.Status, detail))
		}
		sb.WriteString("\n")
	}

	return os.WriteFile(absPath, []byte(sb.String()), 0o640)
}

func listFiles(root string, ext string) ([]string, error) {
	out := make([]string, 0, 128)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if ext == "" || strings.HasSuffix(path, ext) {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

func (c *checker) readFile(rel string) (string, error) {
	path := filepath.Join(c.repoRoot, rel)
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", rel, err)
	}
	return string(b), nil
}
