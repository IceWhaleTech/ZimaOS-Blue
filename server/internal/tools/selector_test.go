package tools

import (
	"testing"
)

// mockToolDefs returns a realistic set of tool definitions for testing.
func mockToolDefs() []ToolDefinition {
	return []ToolDefinition{
		{Name: "calculator", Description: "Performs basic arithmetic operations. Supports +, -, *, /, and parentheses."},
		{Name: "weather", Description: "Get current weather information for a location including temperature, humidity, and conditions."},
		{Name: "datetime", Description: "Get current date, time, timezone information, and format dates."},
		{Name: "notes", Description: "Create, read, update, search, and delete notes with tags."},
		{Name: "tasks", Description: "Create, manage, and track tasks with priorities and status."},
		{Name: "translate", Description: "Translate text between languages."},
		{Name: "search", Description: "Search through text content using contains, regex, or fuzzy matching."},
		{Name: "memory", Description: "Search, retrieve, and inspect the memory system. Use search to find relevant memories by query."},
		{Name: "files", Description: "File system operations including read, write, list, and delete files."},
		{Name: "docker", Description: "Manage Docker containers, images, and volumes."},
		{Name: "network", Description: "Network diagnostics and connectivity information."},
		{Name: "sandbox", Description: "Execute commands in a sandboxed environment with resource limits."},
		{Name: "scheduler", Description: "Create, list, delete, and trigger scheduled tasks using cron expressions."},
		{Name: "system_info", Description: "Returns comprehensive system information including OS, hardware, network, and GPU details."},
		{Name: "notifications", Description: "Send, list, and manage system notifications."},
		{Name: "unit_converter", Description: "Convert between units of measurement including length, weight, temperature."},
		{Name: "workflows", Description: "Create, manage, and execute n8n-style workflow automations."},
	}
}

func containsToolName(defs []ToolDefinition, name string) bool {
	for _, d := range defs {
		if d.Name == name {
			return true
		}
	}
	return false
}

func TestToolSelector_WeatherQuery(t *testing.T) {
	ts := DefaultToolSelector()
	defs := mockToolDefs()

	selected := ts.Select("What's the weather like in Tokyo?", defs)

	if !containsToolName(selected, "weather") {
		t.Errorf("expected weather tool for weather query, got: %v", toolNames(selected))
	}
	t.Logf("Weather query selected %d tools: %v", len(selected), toolNames(selected))
}

func TestToolSelector_MathQuery(t *testing.T) {
	ts := DefaultToolSelector()
	defs := mockToolDefs()

	selected := ts.Select("Calculate 15% tip on $85.50", defs)

	if !containsToolName(selected, "calculator") {
		t.Errorf("expected calculator for math query, got: %v", toolNames(selected))
	}
	t.Logf("Math query selected %d tools: %v", len(selected), toolNames(selected))
}

func TestToolSelector_DockerQuery(t *testing.T) {
	ts := DefaultToolSelector()
	defs := mockToolDefs()

	selected := ts.Select("List all running Docker containers", defs)

	if !containsToolName(selected, "docker") {
		t.Errorf("expected docker for container query, got: %v", toolNames(selected))
	}
	t.Logf("Docker query selected %d tools: %v", len(selected), toolNames(selected))
}

func TestToolSelector_TranslateQuery(t *testing.T) {
	ts := DefaultToolSelector()
	defs := mockToolDefs()

	selected := ts.Select("Translate 'hello world' to Japanese", defs)

	if !containsToolName(selected, "translate") {
		t.Errorf("expected translate for translation query, got: %v", toolNames(selected))
	}
	t.Logf("Translate query selected %d tools: %v", len(selected), toolNames(selected))
}

func TestToolSelector_FileQuery(t *testing.T) {
	ts := DefaultToolSelector()
	defs := mockToolDefs()

	selected := ts.Select("Read the contents of /etc/hosts", defs)

	if !containsToolName(selected, "files") {
		t.Errorf("expected files for file read query, got: %v", toolNames(selected))
	}
	t.Logf("File query selected %d tools: %v", len(selected), toolNames(selected))
}

func TestToolSelector_SystemInfoQuery(t *testing.T) {
	ts := DefaultToolSelector()
	defs := mockToolDefs()

	selected := ts.Select("Show me system hardware information and CPU details", defs)

	if !containsToolName(selected, "system_info") {
		t.Errorf("expected system_info for hardware query, got: %v", toolNames(selected))
	}
	t.Logf("System info query selected %d tools: %v", len(selected), toolNames(selected))
}

func TestToolSelector_EmptyQuery(t *testing.T) {
	ts := DefaultToolSelector()
	defs := mockToolDefs()

	selected := ts.Select("", defs)
	if len(selected) != len(defs) {
		t.Errorf("empty query should return all tools, got %d/%d", len(selected), len(defs))
	}
}

func TestToolSelector_FewTools(t *testing.T) {
	ts := DefaultToolSelector()
	// With only 3 tools, should return all regardless of query
	defs := mockToolDefs()[:3]

	selected := ts.Select("random query", defs)
	if len(selected) != 3 {
		t.Errorf("with few tools should return all, got %d", len(selected))
	}
}

func TestToolSelector_AlwaysInclude(t *testing.T) {
	ts := DefaultToolSelector()
	ts.AlwaysInclude = []string{"memory", "ask"}
	defs := mockToolDefs()
	defs = append(defs, ToolDefinition{Name: "ask", Description: "Ask user preference questions."})

	selected := ts.Select("Calculate 2+2", defs)

	if !containsToolName(selected, "memory") {
		t.Errorf("memory should always be included, got: %v", toolNames(selected))
	}
	if !containsToolName(selected, "ask") {
		t.Errorf("ask should always be included, got: %v", toolNames(selected))
	}
}

func TestDefaultToolSelector_AlwaysIncludesAsk(t *testing.T) {
	ts := DefaultToolSelector()
	defs := append(mockToolDefs(), ToolDefinition{Name: "ask", Description: "Ask user preference questions."})

	selected := ts.Select("Calculate 2+2", defs)
	if !containsToolName(selected, "ask") {
		t.Errorf("default selector should keep ask, got: %v", toolNames(selected))
	}
}

func TestToolSelector_MaxToolsLimit(t *testing.T) {
	ts := DefaultToolSelector()
	ts.MaxTools = 5
	defs := mockToolDefs()

	selected := ts.Select("I need help with everything", defs)
	if len(selected) > 5 {
		t.Errorf("should respect MaxTools=5, got %d tools", len(selected))
	}
}

func TestToolSelector_ChineseNetworkDiag(t *testing.T) {
	ts := DefaultToolSelector()
	defs := mockToolDefs()

	selected := ts.Select("检查网络连接状态和DNS解析", defs)

	if !containsToolName(selected, "network") {
		t.Errorf("expected network for network diagnostics query, got: %v", toolNames(selected))
	}
	t.Logf("Network query selected %d tools: %v", len(selected), toolNames(selected))
}

func TestToolSelector_WorkflowQuery(t *testing.T) {
	ts := DefaultToolSelector()
	defs := mockToolDefs()

	selected := ts.Select("Create an automated workflow that checks server health every hour", defs)

	if !containsToolName(selected, "workflows") {
		t.Errorf("expected workflows for automation query, got: %v", toolNames(selected))
	}
	if !containsToolName(selected, "scheduler") {
		t.Errorf("expected scheduler for cron-related query, got: %v", toolNames(selected))
	}
	t.Logf("Workflow query selected %d tools: %v", len(selected), toolNames(selected))
}

// toolNames extracts tool names from definitions for logging.
func toolNames(defs []ToolDefinition) []string {
	names := make([]string, len(defs))
	for i, d := range defs {
		names[i] = d.Name
	}
	return names
}

func TestToolSelector_Stats(t *testing.T) {
	ts := DefaultToolSelector()
	defs := mockToolDefs()

	// Run a few selections
	ts.Select("What's the weather?", defs)
	ts.Select("Calculate 2+2", defs)
	ts.Select("提醒我开会", defs)

	stats := ts.Stats()
	if stats.Requests != 3 {
		t.Errorf("expected 3 requests, got %d", stats.Requests)
	}
	if stats.ToolsSkipped == 0 {
		t.Error("expected some tools to be skipped")
	}
	if stats.TokensSaved == 0 {
		t.Error("expected some tokens saved")
	}
	if stats.ToolsSent+stats.ToolsSkipped != stats.ToolsTotal {
		t.Errorf("sent(%d) + skipped(%d) != total(%d)", stats.ToolsSent, stats.ToolsSkipped, stats.ToolsTotal)
	}
	t.Logf("Stats after 3 queries: requests=%d, total=%d, sent=%d, skipped=%d, tokens_saved=%d",
		stats.Requests, stats.ToolsTotal, stats.ToolsSent, stats.ToolsSkipped, stats.TokensSaved)
}
