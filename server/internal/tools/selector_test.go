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
	defs := mockToolDefs()[:3]

	selected := ts.Select("random query", defs)
	if len(selected) != 0 {
		t.Errorf("zero-signal query should not expose unrelated tools, got %v", toolNames(selected))
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

func TestToolSelector_PlainReplyQueryPrefersNoTools(t *testing.T) {
	ts := DefaultToolSelector()
	defs := append(mockToolDefs(),
		ToolDefinition{Name: "exec", Description: "Run terminal commands."},
		ToolDefinition{Name: "ask", Description: "Ask user preference questions."},
		ToolDefinition{Name: "gateway", Description: "Inspect gateway state."},
	)

	selected := ts.Select(`Say "Hello, I'm ready!" to confirm you can respond.`, defs)
	if len(selected) != 0 {
		t.Fatalf("plain reply query should not expose tools, got: %v", toolNames(selected))
	}
}

func TestToolSelector_NoZeroScoreFallbackNoise(t *testing.T) {
	ts := DefaultToolSelector()
	ts.MaxTools = 3
	defs := []ToolDefinition{
		{Name: "exec", Description: "Run terminal commands."},
		{Name: "ask", Description: "Ask user preference questions."},
		{Name: "gateway", Description: "Inspect gateway routes and connections."},
		{Name: "ppt", Description: "Generate presentation slide assets."},
		{Name: "calendar", Description: "Create and review calendar events."},
		{Name: "email", Description: "Search and triage inbox messages."},
	}

	selected := ts.Select("obscure neutral phrase without tool overlap", defs)
	if len(selected) > 2 {
		t.Fatalf("expected selector to avoid arbitrary zero-score fallback, got: %v", toolNames(selected))
	}
	if containsToolName(selected, "gateway") || containsToolName(selected, "ppt") {
		t.Fatalf("selector should not surface unrelated tools on zero-signal query, got: %v", toolNames(selected))
	}
}

func TestToolSelector_WorkspaceFileTaskPrefersFileWorkflow(t *testing.T) {
	ts := DefaultToolSelector()
	ts.MaxTools = 8
	defs := []ToolDefinition{
		{Name: "exec", Description: "Run terminal commands."},
		{Name: "ask", Description: "Ask user preference questions."},
		{Name: "file_read", Description: "Read files from the workspace."},
		{Name: "ls", Description: "List files in directories."},
		{Name: "find", Description: "Find text in files."},
		{Name: "file_write", Description: "Write files to the workspace."},
		{Name: "file_delete", Description: "Delete files from the workspace."},
		{Name: "convert", Description: "Convert and parse CSV/XLSX files."},
		{Name: "calendar", Description: "Create and review calendar events."},
		{Name: "email", Description: "Search and triage inbox messages."},
		{Name: "deep_research", Description: "Run deep research or check an existing research job status."},
		{Name: "reminder", Description: "Create reminders and notifications."},
	}

	selected := ts.Select("Review all files in the research/ folder and write a daily summary to daily_briefing.md.", defs)
	names := toolNames(selected)

	if !containsToolName(selected, "file_read") || !containsToolName(selected, "file_write") || !containsToolName(selected, "file_delete") {
		t.Fatalf("expected file workflow tools for workspace file task, got=%v", names)
	}
	if containsToolName(selected, "calendar") || containsToolName(selected, "email") {
		t.Fatalf("expected local file task to suppress calendar/email tools, got=%v", names)
	}
	if containsToolName(selected, "deep_research") || containsToolName(selected, "reminder") {
		t.Fatalf("expected local file task to suppress remote productivity tools, got=%v", names)
	}
}

func TestToolSelector_StructuredWorkspaceArtifactTaskUsesGenericFileWorkflow(t *testing.T) {
	ts := DefaultToolSelector()
	ts.MaxTools = 10
	defs := []ToolDefinition{
		{Name: "ask", Description: "Ask user preference questions."},
		{Name: "file_read", Description: "Read files from the workspace."},
		{Name: "file_write", Description: "Write files to the workspace."},
		{Name: "file_delete", Description: "Delete files from the workspace."},
		{Name: "edit", Description: "Edit files in place."},
		{Name: "ls", Description: "List files in directories."},
		{Name: "find", Description: "Find files and text in the workspace."},
		{Name: "grep", Description: "Search matching lines in files."},
		{Name: "convert", Description: "Convert, normalize, and extract content from local files in multiple formats."},
		{Name: "pdf", Description: "Read and extract text from PDF files."},
		{Name: "image", Description: "Inspect images from the workspace."},
		{Name: "calendar", Description: "Create and review calendar events."},
	}

	selected := ts.Select("I have a report in openclaw_report.pdf in my workspace. Extract the answers and write them one answer per line to answer.txt.", defs)
	names := toolNames(selected)

	for _, required := range []string{"file_read", "file_write", "ls", "find", "convert", "pdf"} {
		if !containsToolName(selected, required) {
			t.Fatalf("expected generic local artifact workflow to keep %s, got=%v", required, names)
		}
	}
	if !containsToolName(selected, "edit") || !containsToolName(selected, "file_delete") || !containsToolName(selected, "grep") {
		t.Fatalf("expected generic local artifact workflow to keep broad file-manipulation tools, got=%v", names)
	}
	for _, excluded := range []string{"image", "calendar"} {
		if containsToolName(selected, excluded) {
			t.Fatalf("expected generic local artifact workflow to suppress unrelated tool %s, got=%v", excluded, names)
		}
	}
}

func TestToolSelector_StructuredWorkspaceArtifactTaskKeepsBroadWorkflowForNonPDFInputs(t *testing.T) {
	ts := DefaultToolSelector()
	ts.MaxTools = 10
	defs := []ToolDefinition{
		{Name: "ask", Description: "Ask user preference questions."},
		{Name: "file_read", Description: "Read files from the workspace."},
		{Name: "file_write", Description: "Write files to the workspace."},
		{Name: "file_delete", Description: "Delete files from the workspace."},
		{Name: "edit", Description: "Edit files in place."},
		{Name: "ls", Description: "List files in directories."},
		{Name: "find", Description: "Find files and text in the workspace."},
		{Name: "grep", Description: "Search matching lines in files."},
		{Name: "convert", Description: "Convert, normalize, and extract content from local files in multiple formats."},
		{Name: "pdf", Description: "Read and extract text from PDF files."},
		{Name: "calendar", Description: "Create and review calendar events."},
	}

	selected := ts.Select("Read summary_source.txt from my workspace and write a concise three-paragraph summary to summary_output.txt.", defs)
	names := toolNames(selected)

	for _, required := range []string{"file_read", "file_write", "ls", "find", "convert", "edit", "file_delete", "grep"} {
		if !containsToolName(selected, required) {
			t.Fatalf("expected generic local artifact workflow to keep %s, got=%v", required, names)
		}
	}
	if !containsToolName(selected, "pdf") {
		t.Fatalf("expected broad local artifact workflow to remain format-agnostic, got=%v", names)
	}
	if containsToolName(selected, "calendar") {
		t.Fatalf("expected non-PDF local artifact task to suppress unrelated calendar tools, got=%v", names)
	}
}

func TestToolSelector_LiveWebQueryAvoidsLocalFileTools(t *testing.T) {
	ts := DefaultToolSelector()
	defs := []ToolDefinition{
		{Name: "web_query", Description: "Search the web for latest sources and references."},
		{Name: "file_read", Description: "Read workspace files."},
		{Name: "file_write", Description: "Write workspace files."},
		{Name: "convert", Description: "Convert CSV and XLSX files."},
		{Name: "exec", Description: "Run terminal commands."},
	}

	selected := ts.Select("搜索最新新闻并给我来源和引用", defs)
	names := toolNames(selected)
	if !containsToolName(selected, "web_query") {
		t.Fatalf("expected web_query for live web query, got=%v", names)
	}
	if containsToolName(selected, "file_read") || containsToolName(selected, "file_write") || containsToolName(selected, "convert") {
		t.Fatalf("expected live web query to suppress local file tools, got=%v", names)
	}
}

func TestToolSelector_UIReviewerNeedsUIEvidence(t *testing.T) {
	ts := DefaultToolSelector()
	defs := []ToolDefinition{
		{Name: "ui_reviewer", Description: "Review screenshots and UI layouts."},
		{Name: "analyze", Description: "Analyze files and reports."},
		{Name: "file_read", Description: "Read workspace files."},
	}

	selected := ts.Select("Review the files in docs/ and summarize the report.", defs)
	if containsToolName(selected, "ui_reviewer") {
		t.Fatalf("ui_reviewer should stay hidden without UI evidence, got=%v", toolNames(selected))
	}

	selected = ts.Select("Review this screenshot and audit the UI layout.", defs)
	if !containsToolName(selected, "ui_reviewer") {
		t.Fatalf("expected ui_reviewer when screenshot/UI evidence exists, got=%v", toolNames(selected))
	}
}

func TestToolSelector_URLAnalysisKeepsAnalyzeVisible(t *testing.T) {
	ts := DefaultToolSelector()
	defs := []ToolDefinition{
		{Name: "analyze", Description: "Analyze URLs, files, and reports."},
		{Name: "browser", Description: "Open web pages."},
		{Name: "web_query", Description: "Search the web."},
	}

	selected := ts.Select("Analyze https://example.com/blog and summarize the key findings into a report.", defs)
	if !containsToolName(selected, "analyze") {
		t.Fatalf("expected analyze for URL analysis query, got=%v", toolNames(selected))
	}
}

func TestToolSelector_DefinitionMetaQuerySuppressesTools(t *testing.T) {
	ts := DefaultToolSelector()
	defs := []ToolDefinition{
		{Name: "web_query", Description: "Search the web."},
		{Name: "browser", Description: "Open web pages."},
		{Name: "exec", Description: "Run commands."},
	}

	selected := ts.Select("How to use the web_query tool?", defs)
	if len(selected) != 0 {
		t.Fatalf("definition/meta query should suppress tool exposure, got=%v", toolNames(selected))
	}
}

func TestLooksLikeWorkspaceFileTask(t *testing.T) {
	positive := []string{
		"Review all files in the research/ folder and write a daily summary to daily_briefing.md.",
		"The emails have been provided to you in the inbox/ folder in your workspace. Read all 13 emails and create a triage report.",
		"I have two data files in my workspace: quarterly_sales.csv and company_expenses.xlsx. Read and analyze both files.",
	}
	for _, query := range positive {
		if !LooksLikeWorkspaceFileTask(query) {
			t.Fatalf("expected workspace file task detection for %q", query)
		}
	}

	negative := []string{
		"Create a competitive landscape analysis for the enterprise observability market and save it to market_research.md.",
		"Archive unread emails from Alice.",
		"Show me today's calendar.",
	}
	for _, query := range negative {
		if LooksLikeWorkspaceFileTask(query) {
			t.Fatalf("did not expect workspace file task detection for %q", query)
		}
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
