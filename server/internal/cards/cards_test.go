package cards

import (
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

func TestToCard(t *testing.T) {
	tests := []struct {
		name     string
		tool     string
		content  string
		wantType string
		wantNil  bool
	}{
		{"calculator_success", "calculator", `{"expression":"1+1","result":2}`, "result", false},
		{"calculator_error", "calculator", `{"error":"bad expr"}`, "result", false},
		{"calculator_invalid", "calculator", `not json`, "", true},
		{"web_search", "web_search", `{"query":"go","results":[{"title":"Go"}],"total_count":1}`, "search", false},
		{"web_search_empty", "web_search", `{"query":"go","results":[],"total_count":0}`, "search", false},
		{"web_query", "web_query", `{"status":"ok","mode":"search_read","title":"Example","content":"Hello world","target_url":"https://example.com","final_url":"https://example.com","sources":[{"title":"Example","url":"https://example.com","selected":true}],"warnings":[],"next_action":"none","diagnostics":{"route":"search_read","attempts":[],"candidate_count":1,"selected_source":1,"degraded":false}}`, "result", false},
		{"web_query_video", "web_query", `{"status":"ok","mode":"video_read","title":"Demo video","content":"Transcript preview","target_url":"https://www.youtube.com/watch?v=demo","final_url":"https://www.youtube.com/watch?v=demo","media":{"platform":"youtube","language":"en"},"page":{"content":"Page summary"},"transcript":{"source":"subtitle_manual","language":"en","text":"Transcript preview"},"sources":[{"title":"Transcript","url":"https://www.youtube.com/api/timedtext","selected":true}],"warnings":[],"next_action":"none","diagnostics":{"route":"video_url","attempts":[],"candidate_count":1,"selected_source":1,"degraded":false}}`, "result", false},
		{"web_fetch_warning", "web_fetch", `{"url":"https://www.reddit.com/r/test","title":"Sign in","content":"Log in to continue","content_type":"text/html","extract_mode":"text","extractor":"html","warning":"page appears to be a login wall; use browser or pass browser_target_id","warning_code":"login_wall"}`, "web-fetch", false},
		{"deep_research", "deep_research", `{"query":"go","mode":"standard","answer":"summary","confidence":0.9,"evidence_count":2,"citations":[{"title":"A","url":"https://example.com"}]}`, "deep-research", false},
		{"current_time", "current_time", `{"datetime":"2025-01-01","timezone":"UTC","unix":1735689600}`, "result", false},
		{"read", "read", `{"path":"/tmp/x","content":"hello"}`, "collapsible-code", false},
		{"read_empty", "read", `{"path":"/tmp/x","content":""}`, "", true},
		{"read_error", "read", `{"error":"not found"}`, "result", false},
		{"write", "write", `{"path":"/tmp/x","message":"ok"}`, "result", false},
		{"legacy_file_read", "file_read", `{"path":"/tmp/x","content":"hello"}`, "collapsible-code", false},
		{"legacy_file_write", "file_write", `{"path":"/tmp/x","message":"ok"}`, "result", false},
		{"system_info", "system_info", `{"os":"linux","arch":"amd64"}`, "result", false},
		{"memory_search", "memory_search", `{"results":[{"text":"hi"}]}`, "result", false},
		{"memory_search_error", "memory_search", `{"error":"no index"}`, "result", false},
		{"ui_reviewer", "ui_reviewer", `{"url":"http://x.com","overall":80,"pass":true}`, "ui-review", false},
		{"ui_reviewer_error", "ui_reviewer", `{"error":"fail"}`, "ui-review", false},
		{"ui_reviewer_invalid", "ui_reviewer", `not json`, "", true},
		{"image", "image", `{"task_id":"img-1","status":"succeeded","image_urls":["/api/v1/media/files/a.png"]}`, "media-generate", false},
		{"image_generate", "image_generate", `{"status":"processing","task_id":"img-2","message":"still working"}`, "media-generate", false},
		{"ppt", "ppt", `{"task_id":"slide-1","image_urls":["/api/media/generated/images/a.png"],"thumbnail_urls":["/api/media/generated/thumbnails/a.png"],"review_summary":"score 92/100"}`, "media-generate", false},
		{"generic", "unknown_tool", `{"foo":"bar"}`, "result", false},
		{"generic_error", "unknown_tool", `{"error":"oops"}`, "result", false},
		{"generic_long", "unknown_tool", strings.Repeat("x", 600), "result", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			card := ToCard(tt.tool, tt.content)
			if tt.wantNil {
				if card != nil {
					t.Errorf("expected nil card, got %v", card)
				}
				return
			}
			if card == nil {
				t.Fatal("expected non-nil card")
			}
			if got := card["type"].(string); got != tt.wantType {
				t.Errorf("expected type %q, got %q", tt.wantType, got)
			}
		})
	}
}

func TestImageCard_MapsTaskEnvelopeToMediaGenerate(t *testing.T) {
	card := ToCard("image", `{"task":{"id":"task-9","status":"succeeded","outputs":[{"url":"https://example.com/a.png","thumbnail_url":"https://example.com/a-thumb.png","revised_prompt":"sunset city"}]}}`)
	if card == nil {
		t.Fatal("expected non-nil card")
	}
	if got := card["type"]; got != "media-generate" {
		t.Fatalf("type=%v, want media-generate", got)
	}
	if got := card["status"]; got != "success" {
		t.Fatalf("status=%v, want success", got)
	}
	if got := card["task_id"]; got != "task-9" {
		t.Fatalf("task_id=%v, want task-9", got)
	}

	images, ok := card["images"].([]map[string]interface{})
	if !ok || len(images) != 1 {
		t.Fatalf("images=%T %v, want one image item", card["images"], card["images"])
	}
	if got := images[0]["src"]; got != "https://example.com/a.png" {
		t.Fatalf("image src=%v, want https://example.com/a.png", got)
	}
	if got := images[0]["thumbnail"]; got != "https://example.com/a-thumb.png" {
		t.Fatalf("image thumbnail=%v, want https://example.com/a-thumb.png", got)
	}
	if got := images[0]["caption"]; got != "sunset city" {
		t.Fatalf("image caption=%v, want sunset city", got)
	}
}

func TestWebQueryCardIncludesMediaSummaryAndPreview(t *testing.T) {
	card := ToCard("web_query", `{"status":"ok","title":"Launch Post","content":"Readable article body","target_url":"https://example.com/launch","final_url":"https://example.com/launch","media":{"summary":"Hero image shows an analytics dashboard.","items":[{"url":"https://example.com/images/hero.png","alt":"Launch dashboard hero","analysis":"Analytics dashboard with KPI cards and a chart."}]}}`)
	if card == nil {
		t.Fatal("expected non-nil card")
	}
	if got := card["image"]; got != "https://example.com/images/hero.png" {
		t.Fatalf("image=%v, want hero preview url", got)
	}
	images, ok := card["images"].([]map[string]interface{})
	if !ok || len(images) != 1 {
		t.Fatalf("images=%T %v, want one preview item", card["images"], card["images"])
	}
	if got := images[0]["caption"]; got != "Analytics dashboard with KPI cards and a chart." {
		t.Fatalf("caption=%v, want media analysis caption", got)
	}
	details, ok := card["details"].([]map[string]interface{})
	if !ok {
		t.Fatalf("details=%T, want []map[string]interface{}", card["details"])
	}
	foundSummary := false
	for _, detail := range details {
		if detail["label"] == "media_summary" && strings.Contains(detail["value"].(string), "analytics dashboard") {
			foundSummary = true
			break
		}
	}
	if !foundSummary {
		t.Fatalf("details=%v, want media_summary entry", details)
	}
}

func TestWebQueryCard_SearchOnlyEnvelopeBecomesSearchCard(t *testing.T) {
	card := ToCard("web_query", `{"status":"ok","mode":"search","query":"OpenAI latest updates","sources":[{"title":"OpenAI blog","url":"https://openai.com/blog","snippet":"Latest announcements and product updates.","selected":true},{"title":"OpenAI docs","url":"https://platform.openai.com/docs","snippet":"API and platform documentation"}],"next_action":"none"}`)
	if card == nil {
		t.Fatal("expected non-nil card")
	}
	if got := card["type"]; got != "search" {
		t.Fatalf("type=%v, want search", got)
	}
	if got := card["query"]; got != "OpenAI latest updates" {
		t.Fatalf("query=%v, want OpenAI latest updates", got)
	}
	rawResults, ok := card["results"].([]interface{})
	if !ok || len(rawResults) != 2 {
		t.Fatalf("results=%T %#v, want 2 mapped search results", card["results"], card["results"])
	}
	first, ok := rawResults[0].(map[string]interface{})
	if !ok {
		t.Fatalf("first result=%T, want map[string]interface{}", rawResults[0])
	}
	second, ok := rawResults[1].(map[string]interface{})
	if !ok {
		t.Fatalf("second result=%T, want map[string]interface{}", rawResults[1])
	}
	if first["title"] != "OpenAI blog" {
		t.Fatalf("first result title=%v, want OpenAI blog", first["title"])
	}
	if first["description"] != "Latest announcements and product updates." {
		t.Fatalf("first result description=%v, want snippet", first["description"])
	}
	if second["url"] != "https://platform.openai.com/docs" {
		t.Fatalf("second result url=%v, want docs url", second["url"])
	}
}

func TestWebSearchCard_EmptyResultsBecomeExplicitEmptySearchCard(t *testing.T) {
	card := ToCard("web_search", `{"query":"go","results":[],"total_count":0,"provider":"duckduckgo"}`)
	if card == nil {
		t.Fatal("expected non-nil card")
	}
	if got := card["type"]; got != "search" {
		t.Fatalf("type=%v, want search", got)
	}
	if got := card["status"]; got != "empty" {
		t.Fatalf("status=%v, want empty", got)
	}
	if got := card["provider"]; got != "duckduckgo" {
		t.Fatalf("provider=%v, want duckduckgo", got)
	}
	if got := card["query"]; got != "go" {
		t.Fatalf("query=%v, want go", got)
	}
	if message := strings.TrimSpace(card["message"].(string)); !strings.Contains(message, "No matching sources") {
		t.Fatalf("message=%q, want explicit empty-state text", message)
	}
	results, ok := card["results"].([]interface{})
	if !ok {
		t.Fatalf("results=%T, want []interface{}", card["results"])
	}
	if len(results) != 0 {
		t.Fatalf("results len=%d, want 0", len(results))
	}
}

func TestWebQueryCard_NoResultsEnvelopeBecomesEmptySearchCard(t *testing.T) {
	card := ToCard("web_query", `{"status":"partial","mode":"search","query":"OpenAI latest updates","next_action":"refine_query","warnings":[{"code":"no_results","message":"no matching sources were found"}],"sources":[]}`)
	if card == nil {
		t.Fatal("expected non-nil card")
	}
	if got := card["type"]; got != "search" {
		t.Fatalf("type=%v, want search", got)
	}
	if got := card["status"]; got != "empty" {
		t.Fatalf("status=%v, want empty", got)
	}
	if message := strings.TrimSpace(card["message"].(string)); !strings.Contains(message, "no matching sources") {
		t.Fatalf("message=%q, want no-results fallback", message)
	}
}

func TestWebQueryCard_SearchCardEmittedSkipsDuplicateSearchOnlyCard(t *testing.T) {
	card := ToCard("web_query", `{"status":"partial","mode":"search","query":"OpenAI latest updates","search_card_emitted":true,"next_action":"refine_query","sources":[{"title":"OpenAI blog","url":"https://openai.com/blog","snippet":"Latest announcements and product updates.","selected":true}]}`)
	if card != nil {
		t.Fatalf("expected nil card when search card already emitted, got %#v", card)
	}
}

func TestWebQueryCard_SearchCardEmittedStillReturnsSecondaryDetailCard(t *testing.T) {
	card := ToCard("web_query", `{"status":"ok","mode":"search_read","query":"OpenAI latest updates","search_card_emitted":true,"title":"OpenAI blog","content":"Readable article body","target_url":"https://openai.com/blog","final_url":"https://openai.com/blog","sources":[{"title":"OpenAI blog","url":"https://openai.com/blog","snippet":"Latest announcements and product updates.","selected":true}],"next_action":"none"}`)
	if card == nil {
		t.Fatal("expected non-nil secondary detail card")
	}
	if got := card["type"]; got != "result" {
		t.Fatalf("type=%v, want result", got)
	}
	if got := card["title"]; got != "OpenAI blog" {
		t.Fatalf("title=%v, want OpenAI blog", got)
	}
	if got := card["message"]; got != "Readable article body" {
		t.Fatalf("message=%v, want article body", got)
	}
}

func TestToolSearchCard_IsHiddenFromTypelessCards(t *testing.T) {
	card := ToCard("tool_search", `{"matches":[{"id":"read","name":"read","kind":"tool"},{"id":"write","name":"write","kind":"tool"}],"activated":{"tools":["read"]}}`)
	if card != nil {
		t.Fatalf("card=%v, want nil so tool_search only shows in the process/details surface", card)
	}
}

func TestFormatTypeless_SkipsToolSearchCards(t *testing.T) {
	calls := []llm.ToolCall{
		{ID: "1", Name: "tool_search", Arguments: `{"query":"LLM agent memory long-term memory RAG OpenClaw"}`},
	}
	results := []llm.Message{
		{Role: llm.RoleTool, Content: `{"matches":[],"activated":{}}`, ToolCallID: "1"},
	}

	out := FormatTypeless(calls, results)
	if strings.Contains(out, "```typeless") {
		t.Fatalf("out=%q, want no typeless block for tool_search", out)
	}
}

func TestImageCard_MapsProcessingToGenerating(t *testing.T) {
	card := ToCard("image_generate", `{"status":"processing","task_id":"task-42","message":"still generating"}`)
	if card == nil {
		t.Fatal("expected non-nil card")
	}
	if got := card["type"]; got != "media-generate" {
		t.Fatalf("type=%v, want media-generate", got)
	}
	if got := card["status"]; got != "generating" {
		t.Fatalf("status=%v, want generating", got)
	}
	if got := card["task_id"]; got != "task-42" {
		t.Fatalf("task_id=%v, want task-42", got)
	}
	if got := card["message"]; got != "still generating" {
		t.Fatalf("message=%v, want still generating", got)
	}
}

func TestFormatTypeless(t *testing.T) {
	calls := []llm.ToolCall{
		{ID: "1", Name: "calculator", Arguments: `{"expression":"1+1"}`},
		{ID: "2", Name: "current_time", Arguments: `{}`},
	}
	results := []llm.Message{
		{Role: llm.RoleTool, Content: `{"expression":"1+1","result":2}`, ToolCallID: "1"},
		{Role: llm.RoleTool, Content: `{"datetime":"now"}`, ToolCallID: "2"},
	}

	out := FormatTypeless(calls, results)
	if !strings.Contains(out, "```typeless") {
		t.Error("expected typeless fence blocks")
	}
	if strings.Count(out, "```typeless") != 2 {
		t.Errorf("expected 2 typeless blocks, got %d", strings.Count(out, "```typeless"))
	}
}

func TestFormatTypeless_MoreCallsThanResults(t *testing.T) {
	calls := []llm.ToolCall{
		{ID: "1", Name: "calculator", Arguments: `{}`},
		{ID: "2", Name: "calculator", Arguments: `{}`},
	}
	results := []llm.Message{
		{Role: llm.RoleTool, Content: `{"result":42}`, ToolCallID: "1"},
	}

	out := FormatTypeless(calls, results)
	if strings.Count(out, "```typeless") != 1 {
		t.Errorf("expected 1 typeless block, got %d", strings.Count(out, "```typeless"))
	}
}

func TestFileWriteCardPrefersOriginalPathAndCarriesRevealPath(t *testing.T) {
	card := ToCard("file_write", `{"path":"reports/final.md","original_path":"@docs/final.md","absolute_path":"/tmp/workspace/reports/final.md","success":true}`)
	if card == nil {
		t.Fatal("expected non-nil card")
	}

	details, ok := card["details"].([]map[string]interface{})
	if !ok || len(details) != 1 {
		t.Fatalf("details=%T %v, want one detail", card["details"], card["details"])
	}
	if got := details[0]["value"]; got != "@docs/final.md" {
		t.Fatalf("detail value=%v, want original path", got)
	}
	if got := details[0]["reveal_path"]; got != "/tmp/workspace/reports/final.md" {
		t.Fatalf("detail reveal_path=%v, want absolute path", got)
	}

	artifacts, ok := card["artifacts"].([]map[string]interface{})
	if !ok || len(artifacts) != 1 {
		t.Fatalf("artifacts=%T %v, want one artifact", card["artifacts"], card["artifacts"])
	}
	if got := artifacts[0]["path"]; got != "reports/final.md" {
		t.Fatalf("artifact path=%v, want resolved path", got)
	}
	if got := artifacts[0]["local_path"]; got != "/tmp/workspace/reports/final.md" {
		t.Fatalf("artifact local_path=%v, want absolute path", got)
	}
}

func TestDeepResearchCard_FieldsPreserved(t *testing.T) {
	content := `{"job_id":"job-1","query":"ZimaOS","mode":"deep","answer":"summary","confidence":0.87,"evidence_count":3,"citations":[{"title":"Doc","url":"https://example.com"}],"open_questions":["q1"],"support_count":2,"conflict_count":1,"has_conflict":true,"citation_coverage":0.92,"entity_disambiguation":{"enabled":true,"threshold":0.75},"stage_errors":["warn"],"timeline_sections":[{"label":"Recent","highlights":["h1"]}],"search_cards":[{"type":"search","query":"ZimaOS overview","totalCount":1,"results":[{"title":"Official docs","url":"https://example.com/docs","description":"docs"}]}],"time_windows":["30d"],"report_style":"timeline","strict_entity":true,"status":"completed"}`
	card := ToCard("deep_research", content)
	if card == nil {
		t.Fatal("expected non-nil deep-research card")
	}
	if got := card["type"]; got != "deep-research" {
		t.Fatalf("type=%v, want deep-research", got)
	}
	if got := card["query"]; got != "ZimaOS" {
		t.Fatalf("query=%v, want ZimaOS", got)
	}
	if got := card["evidence_count"]; got != float64(3) {
		t.Fatalf("evidence_count=%v, want 3", got)
	}
	if got := card["support_count"]; got != float64(2) {
		t.Fatalf("support_count=%v, want 2", got)
	}
	if got := card["conflict_count"]; got != float64(1) {
		t.Fatalf("conflict_count=%v, want 1", got)
	}
	if got := card["has_conflict"]; got != true {
		t.Fatalf("has_conflict=%v, want true", got)
	}
	if got := card["citation_coverage"]; got != 0.92 {
		t.Fatalf("citation_coverage=%v, want 0.92", got)
	}
	if got := card["report_style"]; got != "timeline" {
		t.Fatalf("report_style=%v, want timeline", got)
	}
	if got := card["strict_entity"]; got != true {
		t.Fatalf("strict_entity=%v, want true", got)
	}
	if card["timeline_sections"] == nil {
		t.Fatalf("expected timeline_sections")
	}
	if card["search_cards"] == nil {
		t.Fatalf("expected search_cards")
	}
	if card["entity_disambiguation"] == nil {
		t.Fatalf("expected entity_disambiguation")
	}
	if got := card["id"]; got != "deep-research-job-1" {
		t.Fatalf("id=%v, want deep-research-job-1", got)
	}
}

func TestDeepResearchCard_PrefersJobIDForStableID(t *testing.T) {
	content := `{"job_id":"job-42","query":"ZimaOS","answer":"summary"}`
	card := ToCard("deep_research", content)
	if card == nil {
		t.Fatal("expected non-nil deep-research card")
	}
	if got := card["id"]; got != "deep-research-job-42" {
		t.Fatalf("id=%v, want deep-research-job-42", got)
	}
}

func TestAnalyzeCard_StableID(t *testing.T) {
	content := `{"topic":"Market analysis","report_url":"/reports/r1.html"}`
	card := ToCard("analyze", content)
	if card == nil {
		t.Fatal("expected non-nil analyze card")
	}
	if got := card["id"]; got != "analyze-%2Freports%2Fr1.html" {
		t.Fatalf("id=%v, want analyze-%%2Freports%%2Fr1.html", got)
	}
}

func TestGenericCard_ShowsDetails(t *testing.T) {
	card := GenericCard("test", `{"foo":"bar","token":"secret"}`)
	if card["type"] != "result" {
		t.Fatalf("expected result type, got %v", card["type"])
	}
	if card["status"] != "success" {
		t.Fatalf("expected success status, got %v", card["status"])
	}
	if _, hasMessage := card["message"]; hasMessage {
		t.Fatalf("expected no fallback message when details exist, got %v", card["message"])
	}

	details, ok := card["details"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected details list, got %T", card["details"])
	}
	if len(details) != 2 {
		t.Fatalf("expected 2 detail items, got %d", len(details))
	}
	if details[0]["label"] != "foo" || details[0]["value"] != "bar" {
		t.Fatalf("unexpected first detail: %+v", details[0])
	}
	if details[1]["label"] != "token" || details[1]["value"] != "secret" {
		t.Fatalf("unexpected second detail: %+v", details[1])
	}
}

func TestGenericCard_ExtractsScreenshotPreview(t *testing.T) {
	base64PNG := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO7+5VQAAAAASUVORK5CYII="
	card := GenericCard("Browser", `{"message":"Screenshot captured","screenshot":"`+base64PNG+`","foo":"bar"}`)
	if card["image"] != "data:image/png;base64,"+base64PNG {
		t.Fatalf("expected screenshot preview image, got %v", card["image"])
	}
	details, ok := card["details"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected details list, got %T", card["details"])
	}
	if len(details) != 1 || details[0]["label"] != "foo" {
		t.Fatalf("expected screenshot detail to be omitted, got %+v", details)
	}
}

func TestGenericCard_ExtractsScreenshotGallery(t *testing.T) {
	pngA := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO7+5VQAAAAASUVORK5CYII="
	pngB := "iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAQAAADZc7J/AAAADUlEQVR42mNk+M/wHwAFAgJ/l8V6NwAAAABJRU5ErkJggg=="
	card := GenericCard("Browser", `{"screenshots":["`+pngA+`","`+pngB+`"],"count":2}`)
	images, ok := card["images"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected gallery images, got %T", card["images"])
	}
	if len(images) != 2 {
		t.Fatalf("expected 2 gallery images, got %d", len(images))
	}
	if images[0]["src"] != "data:image/png;base64,"+pngA {
		t.Fatalf("unexpected first image: %+v", images[0])
	}
	if images[1]["src"] != "data:image/png;base64,"+pngB {
		t.Fatalf("unexpected second image: %+v", images[1])
	}
	details, ok := card["details"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected details list, got %T", card["details"])
	}
	if len(details) != 1 || details[0]["label"] != "count" {
		t.Fatalf("expected only non-image details, got %+v", details)
	}
}

func TestGenericCard_ErrorShowsSanitizedMessage(t *testing.T) {
	card := GenericCard("test", `{"error":"secret stack trace?token=top-secret"}`)
	if card["status"] != "error" {
		t.Fatalf("expected error status, got %v", card["status"])
	}
	if card["message"] != "secret stack trace?token=[REDACTED]" {
		t.Fatalf("expected sanitized error message, got %v", card["message"])
	}
	if _, redacted := card["error_redacted"]; redacted {
		t.Fatalf("expected generic error to be visible, got redacted=%v", card["error_redacted"])
	}
}

func TestGenericCard_ObjectErrorShowsSanitizedMessage(t *testing.T) {
	card := GenericCard("test", `{"error":{"message":"permission denied","token":"top-secret"}}`)
	if card["status"] != "error" {
		t.Fatalf("expected error status, got %v", card["status"])
	}
	message, ok := card["message"].(string)
	if !ok || strings.TrimSpace(message) == "" {
		t.Fatalf("expected non-empty error message, got %v", card["message"])
	}
	if !strings.Contains(message, `"message":"permission denied"`) {
		t.Fatalf("expected error message details, got %v", message)
	}
	if strings.Contains(message, "top-secret") {
		t.Fatalf("expected object error secrets to be masked, got %v", message)
	}
	if _, redacted := card["error_redacted"]; redacted {
		t.Fatalf("expected object error to be visible, got redacted=%v", card["error_redacted"])
	}
}

func TestBuildErrorCard_UsesNoDetailsFallback(t *testing.T) {
	card := buildErrorCard("result", "test", map[string]interface{}{})
	if card["status"] != "error" {
		t.Fatalf("expected error status, got %v", card["status"])
	}
	if card["message"] != noErrorDetailsText {
		t.Fatalf("expected no-details fallback message, got %v", card["message"])
	}
	if _, redacted := card["error_redacted"]; redacted {
		t.Fatalf("expected fallback message to be visible, got redacted=%v", card["error_redacted"])
	}
}

func TestGenericCard_BrowserErrorShowsSanitizedMessage(t *testing.T) {
	card := GenericCard("browser", `{"error":"navigation failed: URL not allowed","success":false}`)
	if card["status"] != "error" {
		t.Fatalf("expected error status, got %v", card["status"])
	}
	if card["message"] != "navigation failed: URL not allowed" {
		t.Fatalf("expected browser error message, got %v", card["message"])
	}
	if _, redacted := card["error_redacted"]; redacted {
		t.Fatalf("expected browser error to be visible, got redacted=%v", card["error_redacted"])
	}
}

func TestGenericCard_EditErrorShowsSanitizedMessage(t *testing.T) {
	card := GenericCard("edit", `{"error":"target text not found in config.json?token=secret-value"}`)
	if card["status"] != "error" {
		t.Fatalf("expected error status, got %v", card["status"])
	}
	if card["message"] != "target text not found in config.json?token=[REDACTED]" {
		t.Fatalf("expected edit error message, got %v", card["message"])
	}
	if _, redacted := card["error_redacted"]; redacted {
		t.Fatalf("expected edit error to be visible, got redacted=%v", card["error_redacted"])
	}
	if strings.Contains(card["message"].(string), "secret-value") {
		t.Fatalf("expected edit error secrets to be masked, got %v", card["message"])
	}
}

func TestFileWriteCard_ErrorShowsSanitizedMessage(t *testing.T) {
	card := ToCard("write", `{"error":"permission denied for settings.json?token=secret-value","path":"settings.json"}`)
	if card == nil {
		t.Fatal("expected file write card")
	}
	if card["status"] != "error" {
		t.Fatalf("expected error status, got %v", card["status"])
	}
	if card["message"] != "permission denied for settings.json?token=[REDACTED]" {
		t.Fatalf("expected write error message, got %v", card["message"])
	}
	if strings.Contains(card["message"].(string), "secret-value") {
		t.Fatalf("expected write error secrets to be masked, got %v", card["message"])
	}
}

func TestFileReadCard_ErrorShowsSanitizedMessage(t *testing.T) {
	card := ToCard("read", `{"error":"permission denied for secrets.txt?token=secret-value","path":"secrets.txt"}`)
	if card == nil {
		t.Fatal("expected file read card")
	}
	if card["status"] != "error" {
		t.Fatalf("expected error status, got %v", card["status"])
	}
	if card["message"] != "permission denied for secrets.txt?token=[REDACTED]" {
		t.Fatalf("expected read error message, got %v", card["message"])
	}
	if strings.Contains(card["message"].(string), "secret-value") {
		t.Fatalf("expected read error secrets to be masked, got %v", card["message"])
	}
}

func TestLsCard_ShowsEntriesPreview(t *testing.T) {
	card := ToCard("ls", `{"base_path":".","count":3,"truncated":false,"max_depth":1,"max_entries":200,"include_hidden":false,"entries":[{"path":"sub","type":"dir"},{"path":"README.md","type":"file","size":123},{"path":"server","type":"dir"}]}`)
	if card == nil {
		t.Fatal("expected ls card")
	}
	if card["type"] != "result" {
		t.Fatalf("expected result type, got %v", card["type"])
	}
	if card["title"] != "ls" {
		t.Fatalf("expected title ls, got %v", card["title"])
	}
	if card["message"] != "3 entries in ." {
		t.Fatalf("unexpected message: %v", card["message"])
	}
	details, ok := card["details"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected details slice, got %#v", card["details"])
	}
	foundEntries := false
	for _, detail := range details {
		if detail["label"] != "entries" {
			continue
		}
		value, _ := detail["value"].(string)
		if !strings.Contains(value, "[dir] sub") || !strings.Contains(value, "[file] README.md (123 B)") {
			t.Fatalf("unexpected entries preview: %q", value)
		}
		if multiline, _ := detail["multiline"].(bool); !multiline {
			t.Fatalf("expected entries detail to be multiline")
		}
		foundEntries = true
	}
	if !foundEntries {
		t.Fatalf("expected entries preview detail: %#v", details)
	}
	if _, ok := card["warning"]; ok {
		t.Fatalf("did not expect warning for non-truncated card: %#v", card)
	}
}

func TestLsCard_TruncatedAddsWarning(t *testing.T) {
	card := ToCard("ls", `{"base_path":".","count":200,"truncated":true,"max_depth":1,"max_entries":200,"include_hidden":false,"entries":[{"path":"a","type":"file","size":1}]}`)
	if card == nil {
		t.Fatal("expected ls card")
	}
	if card["warning_code"] != "output_truncated" {
		t.Fatalf("expected output_truncated warning code, got %v", card["warning_code"])
	}
	warning, _ := card["warning"].(string)
	if !strings.Contains(warning, "truncated") {
		t.Fatalf("expected truncation warning text, got %q", warning)
	}
	message, _ := card["message"].(string)
	if !strings.Contains(message, "Showing first 200 entries") {
		t.Fatalf("expected truncated message, got %q", message)
	}
}

func TestExecCard_ShowsStructuredOutputWithMaskedSensitiveData(t *testing.T) {
	card := ToCard("exec", `{"command":"curl -H 'Authorization: Bearer secret-token-1234567890' https://example.com?token=abc123","stdout":"AWS_SECRET_ACCESS_KEY=abc123\ncontact=user@example.com\ncookie=session=abcdef","stderr":"password=abc123","exit_code":1,"warnings":["set --token super-secret"],"session_id":"abc","host":"sandbox","risk_level":"high","duration_ms":8}`)
	if card == nil {
		t.Fatal("expected exec card")
	}
	if card["type"] != "exec" {
		t.Fatalf("expected exec type, got %v", card["type"])
	}
	if card["status"] != "error" {
		t.Fatalf("expected error status, got %v", card["status"])
	}
	command, ok := card["command"].(string)
	if !ok || command == "" {
		t.Fatalf("expected command to be present, got %+v", card)
	}
	if strings.Contains(command, "secret-token-1234567890") || strings.Contains(command, "token=abc123") {
		t.Fatalf("expected command to be masked, got %q", command)
	}
	stdout, ok := card["stdout"].(string)
	if !ok || stdout == "" {
		t.Fatalf("expected stdout to be present, got %+v", card)
	}
	if strings.Contains(stdout, "AWS_SECRET_ACCESS_KEY=abc123") || strings.Contains(stdout, "user@example.com") || strings.Contains(stdout, "session=abcdef") {
		t.Fatalf("expected stdout to be masked, got %q", stdout)
	}
	stderr, ok := card["stderr"].(string)
	if !ok || stderr == "" {
		t.Fatalf("expected stderr to be present, got %+v", card)
	}
	if strings.Contains(stderr, "password=abc123") {
		t.Fatalf("expected stderr to be masked, got %q", stderr)
	}
	if card["warning_count"] != 1 {
		t.Fatalf("expected warning_count=1, got %+v", card["warning_count"])
	}
	warnings, ok := card["warnings"].([]string)
	if !ok || len(warnings) != 1 {
		t.Fatalf("expected one warning entry, got %+v", card["warnings"])
	}
	if strings.Contains(warnings[0], "super-secret") {
		t.Fatalf("expected warning text to be masked, got %q", warnings[0])
	}
	if card["host"] != "sandbox" {
		t.Fatalf("expected host to be preserved, got %+v", card["host"])
	}
	if _, ok := card["session_id"]; ok {
		t.Fatalf("session_id should remain omitted: %+v", card)
	}
}

func TestWebFetchCard_PreservesWarningCode(t *testing.T) {
	card := ToCard("web_fetch", `{"url":"https://www.reddit.com/r/test","title":"Sign in","content":"Log in to continue","content_type":"text/html","extract_mode":"text","extractor":"html","warning":"page appears to be a login wall; use browser or pass browser_target_id","warning_code":"login_wall"}`)
	if card == nil {
		t.Fatal("expected web_fetch card")
	}
	if card["type"] != "web-fetch" {
		t.Fatalf("expected web-fetch type, got %v", card["type"])
	}
	if card["title"] != "Sign in" {
		t.Fatalf("expected title to use page title, got %v", card["title"])
	}
	if card["status"] != "warning" {
		t.Fatalf("expected warning status, got %v", card["status"])
	}
	if card["warning_code"] != "login_wall" {
		t.Fatalf("expected warning_code=login_wall, got %v", card["warning_code"])
	}
	warning, ok := card["warning"].(string)
	if !ok || !strings.Contains(warning, "login wall") {
		t.Fatalf("expected warning text, got %v", card["warning"])
	}
	if card["url"] != "https://www.reddit.com/r/test" {
		t.Fatalf("expected url to be preserved, got %v", card["url"])
	}
	if card["id"] != "web-fetch-https%3A%2F%2Fwww.reddit.com%2Fr%2Ftest" {
		t.Fatalf("expected encoded stable id, got %v", card["id"])
	}
	actions, ok := card["actions"].([]map[string]interface{})
	if !ok || len(actions) != 1 {
		t.Fatalf("expected one web-fetch action, got %#v", card["actions"])
	}
	if actions[0]["id"] != "use_browser" {
		t.Fatalf("expected use_browser action, got %#v", actions[0])
	}
	formData, ok := actions[0]["form_data"].(map[string]interface{})
	if !ok || formData["url"] != "https://www.reddit.com/r/test" {
		t.Fatalf("expected web-fetch action form_data url, got %#v", actions[0]["form_data"])
	}
	if card["content"] != "Log in to continue" {
		t.Fatalf("expected content to be preserved, got %v", card["content"])
	}
	if card["extractor"] != "html" {
		t.Fatalf("expected extractor=html, got %v", card["extractor"])
	}
}

func TestWebFetchCard_PreservesChallengeWarningCode(t *testing.T) {
	card := ToCard("web_fetch", `{"url":"https://www.reddit.com/r/test","title":"Verification required","content":"Complete the verification to continue","content_type":"text/html","extract_mode":"text","extractor":"html","warning":"page appears to require a verification challenge; switch to browser or reuse browser_target_id","warning_code":"challenge"}`)
	if card == nil {
		t.Fatal("expected web_fetch card")
	}
	if card["warning_code"] != "challenge" {
		t.Fatalf("expected warning_code=challenge, got %v", card["warning_code"])
	}
	warning, ok := card["warning"].(string)
	if !ok || !strings.Contains(warning, "verification challenge") {
		t.Fatalf("expected challenge warning text, got %v", card["warning"])
	}
	actions, ok := card["actions"].([]map[string]interface{})
	if !ok || len(actions) != 1 || actions[0]["id"] != "use_browser" {
		t.Fatalf("expected use_browser action for challenge warning, got %#v", card["actions"])
	}
}

func TestWebFetchCard_PreservesBrowserRequiredWarningCode(t *testing.T) {
	card := ToCard("web_fetch", `{"url":"https://www.reddit.com/r/test","title":"Protected page","content":"Use a browser session to read this page.","content_type":"text/html","extract_mode":"text","extractor":"html","warning":"page requires a browser session for readable extraction; switch to browser or reuse browser_target_id","warning_code":"browser_required"}`)
	if card == nil {
		t.Fatal("expected web_fetch card")
	}
	if card["warning_code"] != "browser_required" {
		t.Fatalf("expected warning_code=browser_required, got %v", card["warning_code"])
	}
	warning, ok := card["warning"].(string)
	if !ok || !strings.Contains(warning, "browser session") {
		t.Fatalf("expected browser_required warning text, got %v", card["warning"])
	}
	actions, ok := card["actions"].([]map[string]interface{})
	if !ok || len(actions) != 1 || actions[0]["id"] != "use_browser" {
		t.Fatalf("expected use_browser action for browser_required warning, got %#v", card["actions"])
	}
}

func TestBrowserCard_IncludesExtractWithWebFetchAction(t *testing.T) {
	card := ToCard("browser", `{"url":"https://www.reddit.com/r/test","title":"r/test","target_id":"tab-42","strategy":"interactive","tree":"@1 link \"Log in\""}`)
	if card == nil {
		t.Fatal("expected browser card")
	}
	if card["type"] != "result" {
		t.Fatalf("expected result type, got %v", card["type"])
	}
	if card["title"] != "r/test" {
		t.Fatalf("expected title to use page title, got %v", card["title"])
	}
	actions, ok := card["actions"].([]map[string]interface{})
	if !ok || len(actions) != 1 {
		t.Fatalf("expected one browser action, got %#v", card["actions"])
	}
	if actions[0]["id"] != "extract_with_web_fetch" {
		t.Fatalf("expected extract_with_web_fetch action, got %#v", actions[0])
	}
	formData, ok := actions[0]["form_data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected browser action form_data, got %#v", actions[0]["form_data"])
	}
	if formData["url"] != "https://www.reddit.com/r/test" || formData["browser_target_id"] != "tab-42" {
		t.Fatalf("unexpected browser action form_data: %#v", formData)
	}
	details, ok := card["details"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected details slice, got %#v", card["details"])
	}
	for _, detail := range details {
		if detail["label"] == "tree" {
			t.Fatalf("expected browser card to hide tree detail, got %#v", details)
		}
	}
}

func TestBrowserCard_EmbedsScreenshotImage(t *testing.T) {
	base64PNG := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO7+5VQAAAAASUVORK5CYII="
	card := ToCard("browser", `{"message":"Screenshot captured for https://example.com","screenshot":"`+base64PNG+`"}`)
	if card == nil {
		t.Fatal("expected browser card")
	}
	if card["type"] != "result" {
		t.Fatalf("expected result type, got %v", card["type"])
	}
	if card["message"] != "Screenshot captured for https://example.com" {
		t.Fatalf("unexpected message: %v", card["message"])
	}
	wantImage := "data:image/png;base64," + base64PNG
	if card["image"] != wantImage {
		t.Fatalf("expected image %q, got %v", wantImage, card["image"])
	}

	cardWithDataURL := ToCard("browser", `{"screenshot":"data:image/png;base64,`+base64PNG+`"}`)
	if cardWithDataURL == nil {
		t.Fatal("expected browser card with data URL")
	}
	if cardWithDataURL["image"] != "data:image/png;base64,"+base64PNG {
		t.Fatalf("expected original data URL image, got %v", cardWithDataURL["image"])
	}
}

func TestBrowserCard_PreservesLocalScreenshotPath(t *testing.T) {
	card := ToCard("browser", `{"message":"Screenshot captured","screenshot":"C:\\Users\\orca\\AppData\\Local\\ZimaOS\\browser\\shot.png"}`)
	if card == nil {
		t.Fatal("expected browser card")
	}
	if card["image"] != `C:\Users\orca\AppData\Local\ZimaOS\browser\shot.png` {
		t.Fatalf("expected local screenshot path, got %v", card["image"])
	}
}

func TestBrowserCard_PreservesAPIScreenshotPath(t *testing.T) {
	card := ToCard("browser", `{"message":"Screenshot captured","screenshot":"/api/v1/media/browser/screenshot-1724277463.png"}`)
	if card == nil {
		t.Fatal("expected browser card")
	}
	if card["image"] != "/api/v1/media/browser/screenshot-1724277463.png" {
		t.Fatalf("expected api screenshot path, got %v", card["image"])
	}
}

func TestBrowserCard_NormalizesToolNameVariants(t *testing.T) {
	base64PNG := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO7+5VQAAAAASUVORK5CYII="

	tests := []string{"Browser", "browser.screenshot"}
	for _, toolName := range tests {
		t.Run(toolName, func(t *testing.T) {
			card := ToCard(toolName, `{"message":"Screenshot captured","screenshot":"`+base64PNG+`"}`)
			if card == nil {
				t.Fatal("expected browser screenshot card")
			}
			if card["type"] != "result" {
				t.Fatalf("expected result type, got %v", card["type"])
			}
			if card["image"] != "data:image/png;base64,"+base64PNG {
				t.Fatalf("expected normalized screenshot image, got %v", card["image"])
			}
		})
	}
}

func TestGenericCard_PreservesAPIImagePath(t *testing.T) {
	card := ToCard("unknown_tool", `{"screenshot":"/api/v1/media/browser/screenshot-1724277463.png"}`)
	if card == nil {
		t.Fatal("expected generic card")
	}
	if card["image"] != "/api/v1/media/browser/screenshot-1724277463.png" {
		t.Fatalf("expected api image path, got %v", card["image"])
	}
}

func TestFormatTypeless_ExecInjectsSanitizedCommand(t *testing.T) {
	calls := []llm.ToolCall{
		{ID: "1", Name: "exec", Arguments: `{"command":"curl https://example.com?token=raw-secret"}`},
	}
	results := []llm.Message{
		{Role: llm.RoleTool, Content: `{"stdout":"ok","exit_code":0}`, ToolCallID: "1"},
	}

	out := FormatTypeless(calls, results)
	if !strings.Contains(out, `"command":"curl https://example.com?token=[REDACTED]"`) {
		t.Fatalf("expected sanitized command in typeless block, got %q", out)
	}
	if strings.Contains(out, `"command_redacted":true`) || strings.Contains(out, `"hide_command":true`) {
		t.Fatalf("expected no forced command hiding markers, got %q", out)
	}
	if strings.Contains(out, "raw-secret") {
		t.Fatalf("expected sensitive token to be masked, got %q", out)
	}
}

func TestRegister_OverridesBuiltin(t *testing.T) {
	Register("calculator", func(content string) map[string]interface{} {
		return map[string]interface{}{"type": "custom", "raw": content}
	})
	defer func() {
		registryMu.Lock()
		delete(registry, "calculator")
		registryMu.Unlock()
	}()

	card := ToCard("calculator", `{"expression":"1+1","result":2}`)
	if card == nil {
		t.Fatal("expected non-nil card")
	}
	if card["type"] != "custom" {
		t.Errorf("expected custom type, got %v", card["type"])
	}
}

func TestRegister_NewTool(t *testing.T) {
	Register("my_tool", func(content string) map[string]interface{} {
		return map[string]interface{}{"type": "my_card"}
	})
	defer func() {
		registryMu.Lock()
		delete(registry, "my_tool")
		registryMu.Unlock()
	}()

	card := ToCard("my_tool", `{}`)
	if card == nil || card["type"] != "my_card" {
		t.Errorf("expected my_card type, got %v", card)
	}
}

func TestUIReviewCard(t *testing.T) {
	content := `{"url":"http://example.com","overall":82.1,"pass":true,"threshold":75,"visual":{"score":82},"functional":{"score":90},"accessibility":{"score":75},"issues":[{"severity":"major","category":"accessibility","description":"Missing alt"}],"steps":[{"id":"navigate","name":"Page Load","status":"success"}],"device":"desktop","channel":"web","media_url":"/api/v1/media/ui-review/a.png","thumbnail_url":"/api/v1/media/ui-review/thumb.png","screenshots":["/api/v1/media/ui-review/a.png"]}`
	card := ToCard("ui_reviewer", content)
	if card == nil {
		t.Fatal("expected non-nil card")
	}
	if card["type"] != "ui-review" {
		t.Errorf("expected type ui-review, got %v", card["type"])
	}
	if card["url"] != "http://example.com" {
		t.Errorf("expected url, got %v", card["url"])
	}
	if card["overall"] != 82.1 {
		t.Errorf("expected overall 82.1, got %v", card["overall"])
	}
	if card["pass"] != true {
		t.Errorf("expected pass true, got %v", card["pass"])
	}
	// Steps should be present
	if card["steps"] == nil {
		t.Error("expected steps")
	}
	// Issues should be present
	if card["issues"] == nil {
		t.Error("expected issues")
	}
	if card["device"] != "desktop" {
		t.Errorf("expected device desktop, got %v", card["device"])
	}
	if card["thumbnail_url"] != "/api/v1/media/ui-review/thumb.png" {
		t.Errorf("expected thumbnail_url, got %v", card["thumbnail_url"])
	}
	if card["screenshots"] == nil {
		t.Error("expected screenshots")
	}
	// Actions should be present
	actions, ok := card["actions"].([]map[string]interface{})
	if !ok || len(actions) != 3 {
		t.Errorf("expected 3 actions, got %v", card["actions"])
	} else if actions[0]["id"] != "recheck" {
		t.Errorf("expected first action id 'recheck', got %v", actions[0]["id"])
	}
	// Stable card ID from URL
	if card["id"] != "ui-review-http%3A%2F%2Fexample.com" {
		t.Errorf("expected card id, got %v", card["id"])
	}
}

func TestUIReviewCard_Error(t *testing.T) {
	content := `{"error":"browser not available?token=secret-value"}`
	card := ToCard("ui_reviewer", content)
	if card == nil {
		t.Fatal("expected non-nil card")
	}
	if card["type"] != "ui-review" {
		t.Errorf("expected type ui-review, got %v", card["type"])
	}
	if card["status"] != "error" {
		t.Errorf("expected status error, got %v", card["status"])
	}
	if card["message"] != "browser not available?token=[REDACTED]" {
		t.Errorf("expected sanitized ui-review error, got %v", card["message"])
	}
	if _, redacted := card["error_redacted"]; redacted {
		t.Errorf("expected ui-review error to be visible, got %v", card["error_redacted"])
	}
	// Error card should have retry action
	actions, ok := card["actions"].([]map[string]interface{})
	if !ok || len(actions) != 1 {
		t.Errorf("expected 1 action on error card, got %v", card["actions"])
	}
}

func TestUIReviewCard_ObjectError(t *testing.T) {
	content := `{"error":{"message":"browser not available","token":"secret-value"}}`
	card := ToCard("ui_reviewer", content)
	if card == nil {
		t.Fatal("expected non-nil card")
	}
	if card["type"] != "ui-review" {
		t.Errorf("expected type ui-review, got %v", card["type"])
	}
	if card["status"] != "error" {
		t.Errorf("expected status error, got %v", card["status"])
	}
	message, ok := card["message"].(string)
	if !ok || strings.TrimSpace(message) == "" {
		t.Errorf("expected non-empty ui-review error message, got %v", card["message"])
	}
	if strings.Contains(message, "secret-value") {
		t.Errorf("expected ui-review object error to be masked, got %v", message)
	}
	if _, redacted := card["error_redacted"]; redacted {
		t.Errorf("expected ui-review object error to be visible, got %v", card["error_redacted"])
	}
}

func TestUIReviewCard_InvalidJSON(t *testing.T) {
	card := ToCard("ui_reviewer", "not json")
	if card != nil {
		t.Errorf("expected nil card for invalid JSON, got %v", card)
	}
}

func TestSandboxCardDispatch(t *testing.T) {
	// Simulate sandbox tool result containing IPC response with _card hint
	content := `{"result":{"stdout":"{\"status\":\"ok\",\"data\":{\"_card\":\"ui_reviewer\",\"result\":\"{\\\"url\\\":\\\"http://x.com\\\",\\\"overall\\\":80,\\\"pass\\\":true}\"}}","stderr":"","exit_code":0},"message":"done"}`
	card := ToCard("sandbox", content)
	if card == nil {
		t.Fatal("expected non-nil card from sandbox dispatch")
	}
	if card["type"] != "ui-review" {
		t.Errorf("expected type ui-review, got %v", card["type"])
	}
}

func TestSandboxCardDispatch_NoHint(t *testing.T) {
	// Sandbox result without _card hint → falls through to GenericCard
	content := `{"result":{"stdout":"hello world","stderr":"","exit_code":0},"message":"done"}`
	card := ToCard("sandbox", content)
	if card == nil {
		t.Fatal("expected non-nil card")
	}
	if card["type"] != "result" {
		t.Errorf("expected generic result type, got %v", card["type"])
	}
}

func TestConvertTaskCard(t *testing.T) {
	card := ToCard("convert", `{"task_id":"task-1","status":"processing","action":"tts","sources":["/tmp/input.md"],"source_summary":"hello","target_format":"wav","progress":35,"message":"Processing","outputs":[{"output_id":"out-1","name":"speech.wav","path":"/tmp/exports/speech.wav"}]}`)
	if card == nil {
		t.Fatal("expected non-nil convert card")
	}
	if got := card["type"]; got != "convert-task" {
		t.Fatalf("type=%v, want convert-task", got)
	}
	if got := card["task_id"]; got != "task-1" {
		t.Fatalf("task_id=%v, want task-1", got)
	}
	if got := card["action"]; got != "tts" {
		t.Fatalf("action=%v, want tts", got)
	}
	sources, ok := card["sources"].([]string)
	if !ok {
		t.Fatalf("sources=%T, want []string", card["sources"])
	}
	if len(sources) != 1 || sources[0] != "/tmp/input.md" {
		t.Fatalf("sources=%v, want /tmp/input.md", sources)
	}
	outputs, ok := card["outputs"].([]interface{})
	if !ok || len(outputs) != 1 {
		t.Fatalf("outputs=%T %v, want 1 output", card["outputs"], card["outputs"])
	}
	first, ok := outputs[0].(map[string]interface{})
	if !ok {
		t.Fatalf("outputs[0]=%T, want map[string]interface{}", outputs[0])
	}
	if got := first["path"]; got != "/tmp/exports/speech.wav" {
		t.Fatalf("outputs[0].path=%v, want /tmp/exports/speech.wav", got)
	}
}
