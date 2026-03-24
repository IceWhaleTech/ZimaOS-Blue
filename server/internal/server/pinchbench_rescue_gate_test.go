package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	_ "github.com/mattn/go-sqlite3"
)

type pinchBenchCategory struct {
	name    string
	weight  float64
	target  float64
	passed  int
	total   int
	details []string
}

func (c *pinchBenchCategory) check(ok bool, detail string) {
	c.total++
	if ok {
		c.passed++
		return
	}
	c.details = append(c.details, detail)
}

func (c pinchBenchCategory) score() float64 {
	if c.total == 0 {
		return 0
	}
	return float64(c.passed) * 100 / float64(c.total)
}

type pinchBenchLLMBridge struct {
	responses []string
	idx       int
}

func (m *pinchBenchLLMBridge) Chat(_ context.Context, _ string, _ int) (string, error) {
	if m.idx < len(m.responses) {
		resp := m.responses[m.idx]
		m.idx++
		return resp, nil
	}
	return "{}", nil
}

type pinchBenchReminderService struct {
	list []tools.PushResult
}

func (m *pinchBenchReminderService) Add(_ context.Context, _, _ string, _ time.Time, _ string, _ string, _ *time.Time) (tools.PushResult, error) {
	return tools.PushResult{}, nil
}

func (m *pinchBenchReminderService) List(_ context.Context, _ string) ([]tools.PushResult, error) {
	return append([]tools.PushResult(nil), m.list...), nil
}

func (m *pinchBenchReminderService) Delete(_ context.Context, _, _ string) error { return nil }

func (m *pinchBenchReminderService) Clear(_ context.Context, _ string) (int64, error) { return 0, nil }

type pinchBenchCronService struct {
	jobs []tools.CronJobInfo
}

func (m *pinchBenchCronService) Config(_ context.Context) tools.CronConfigInfo {
	return tools.CronConfigInfo{}
}

func (m *pinchBenchCronService) Handlers(_ context.Context) []string { return nil }

func (m *pinchBenchCronService) ListJobs(_ context.Context) ([]tools.CronJobInfo, error) {
	return append([]tools.CronJobInfo(nil), m.jobs...), nil
}

func (m *pinchBenchCronService) GetJob(_ context.Context, _ string) (*tools.CronJobInfo, error) {
	return nil, nil
}

func (m *pinchBenchCronService) CreateJob(_ context.Context, _, _, _, _ string, _ map[string]interface{}) (*tools.CronJobInfo, error) {
	return nil, nil
}

func (m *pinchBenchCronService) UpdateJob(_ context.Context, _, _, _, _ string, _ map[string]interface{}) error {
	return nil
}

func (m *pinchBenchCronService) DeleteJob(_ context.Context, _ string) error  { return nil }
func (m *pinchBenchCronService) EnableJob(_ context.Context, _ string) error  { return nil }
func (m *pinchBenchCronService) DisableJob(_ context.Context, _ string) error { return nil }
func (m *pinchBenchCronService) TriggerJob(_ context.Context, _ string) error { return nil }

func (m *pinchBenchCronService) GetExecutions(_ context.Context, _ string, _ int) ([]tools.CronExecutionInfo, error) {
	return nil, nil
}

func openPinchBenchGateDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "pinchbench_gate.db"))
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func loadPinchBenchEmailFixtures(t *testing.T) []tools.EmailMessage {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "tools", "testdata", "pinchbench_email_fixture.json"))
	if err != nil {
		t.Fatalf("read email fixture: %v", err)
	}
	var fixtures []tools.EmailMessage
	if err := json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatalf("decode email fixture: %v", err)
	}
	return fixtures
}

func loadPinchBenchCalendarFixtures(t *testing.T) []tools.CalendarEvent {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "tools", "testdata", "pinchbench_calendar_fixture.json"))
	if err != nil {
		t.Fatalf("read calendar fixture: %v", err)
	}
	var fixtures []tools.CalendarEvent
	if err := json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatalf("decode calendar fixture: %v", err)
	}
	return fixtures
}

func toolDefNames(defs []tools.ToolDefinition) []string {
	names := make([]string, 0, len(defs))
	for _, def := range defs {
		names = append(names, def.Name)
	}
	return names
}

func timePtr(value time.Time) *time.Time {
	return &value
}

func TestPinchBenchFirstTierRescueGate(t *testing.T) {
	now := time.Date(2026, time.March, 20, 9, 0, 0, 0, time.UTC)
	ctx := tools.WithLang(tools.WithUserID(context.Background(), "user-1"), "en-US")
	db := openPinchBenchGateDB(t)

	emailSvc, err := tools.NewLocalEmailService(db)
	if err != nil {
		t.Fatalf("new email service: %v", err)
	}
	emailSvc.SetNowFunc(func() time.Time { return now })
	if err := emailSvc.SeedFixtures(context.Background(), "user-1", loadPinchBenchEmailFixtures(t)); err != nil {
		t.Fatalf("seed email fixtures: %v", err)
	}

	calendarSvc, err := tools.NewLocalCalendarService(db)
	if err != nil {
		t.Fatalf("new calendar service: %v", err)
	}
	calendarSvc.SetNowFunc(func() time.Time { return now })
	if err := calendarSvc.SeedFixtures(context.Background(), "user-1", loadPinchBenchCalendarFixtures(t)); err != nil {
		t.Fatalf("seed calendar fixtures: %v", err)
	}

	emailTool := tools.NewEmailTool(emailSvc)
	calendarTool := tools.NewCalendarTool(calendarSvc)
	calendarTool.SetNowFunc(func() time.Time { return now })
	calendarTool.SetEmailService(emailSvc)
	calendarTool.SetReminderService(&pinchBenchReminderService{
		list: []tools.PushResult{
			{
				ID:      "reminder-1",
				Message: "Call the vendor",
				FireAt:  now.Add(4 * time.Hour),
				Status:  "scheduled",
			},
		},
	})
	calendarTool.SetCronService(&pinchBenchCronService{
		jobs: []tools.CronJobInfo{
			{
				ID:        "job-1",
				Name:      "Daily export",
				Schedule:  "0 18 * * *",
				Handler:   "export.daily",
				Enabled:   true,
				NextRunAt: timePtr(now.Add(9 * time.Hour)),
				Payload: map[string]interface{}{
					"user_id": "user-1",
				},
			},
		},
	})

	categories := []pinchBenchCategory{
		{name: "analysis", weight: 26, target: 90},
		{name: "writing", weight: 14, target: 90},
		{name: "email", weight: 22, target: 90},
		{name: "research", weight: 20, target: 90},
		{name: "productivity", weight: 18, target: 90},
	}

	analysisJSON := `{"summary":"Blue inline summary","stats":[{"label":"coverage","value":"5 first-tier classes"}],"insights":[{"title":"routing","summary":"Inline mode avoids HTML detours"}],"recommendations":["Prefer inline answers for normal analysis prompts"]}`
	reportHTML := `<div class="report"><h1>Blue analysis report</h1></div>`

	inlineAnalyze := tools.NewAnalyzeTool()
	inlineAnalyze.SetLLMBridge(&pinchBenchLLMBridge{responses: []string{analysisJSON}})
	inlineResultAny, err := inlineAnalyze.Execute(ctx, map[string]interface{}{
		"topic": "PinchBench inline analysis",
		"text":  "Blue should answer inline by default.",
		"lang":  "en-US",
	})
	if err != nil {
		t.Fatalf("inline analyze: %v", err)
	}
	inlineResultStr := inlineResultAny.(string)
	var inlineResult map[string]interface{}
	if err := json.Unmarshal([]byte(inlineResultStr), &inlineResult); err != nil {
		t.Fatalf("decode inline analyze result: %v", err)
	}
	categories[0].check(inlineResult["output_mode"] == "inline", "analysis should default to inline mode")
	categories[0].check(inlineResult["report_url"] == nil, "analysis inline mode should not emit report_url")
	answer, _ := inlineResult["answer"].(string)
	categories[0].check(strings.Contains(answer, "Blue inline summary"), "analysis inline answer should include synthesized summary")

	reportAnalyze := tools.NewAnalyzeTool()
	reportAnalyze.SetLLMBridge(&pinchBenchLLMBridge{responses: []string{analysisJSON, reportHTML}})
	reportAnalyze.SetMediaDir(t.TempDir())
	reportResultAny, err := reportAnalyze.Execute(ctx, map[string]interface{}{
		"topic":       "PinchBench report analysis",
		"text":        "Blue should still support explicit report mode.",
		"lang":        "en-US",
		"output_mode": "report",
	})
	if err != nil {
		t.Fatalf("report analyze: %v", err)
	}
	reportResultStr := reportResultAny.(string)
	var reportResult map[string]interface{}
	if err := json.Unmarshal([]byte(reportResultStr), &reportResult); err != nil {
		t.Fatalf("decode report analyze result: %v", err)
	}
	categories[0].check(reportResult["report_url"] != nil, "analysis report mode should emit report_url")

	baseDefs := []tools.ToolDefinition{
		{Name: "web_search"},
		{Name: "research_run"},
		{Name: "analyze"},
		{Name: "email"},
		{Name: "calendar"},
	}
	categories[1].check(len(applyWritingToolPreference(baseDefs, "Rewrite this update in a more concise and formal tone")) == 0, "writing prompts should suppress unrelated tool detours")
	categories[1].check(len(applyWritingToolPreference(baseDefs, "Please research the latest AI model launches with sources")) == len(baseDefs), "research prompts should not be treated as pure writing")
	categories[1].check(len(applyWritingToolPreference(baseDefs, "Archive unread emails from Alice")) == len(baseDefs), "email prompts should not be treated as pure writing")

	searchAny, err := emailTool.Execute(ctx, map[string]interface{}{
		"action": "search",
		"from":   "Alice Chen",
	})
	if err != nil {
		t.Fatalf("email search: %v", err)
	}
	searchResult := searchAny.(map[string]interface{})
	categories[2].check(searchResult["count"].(int) == 1, "email search should find sender-specific inbox results")

	archiveAny, err := emailTool.Execute(ctx, map[string]interface{}{
		"action": "archive",
		"id":     "mail-budget",
	})
	if err != nil {
		t.Fatalf("email archive: %v", err)
	}
	archiveResult := archiveAny.(map[string]interface{})
	archiveEmail := archiveResult["email"].(map[string]interface{})
	categories[2].check(archiveEmail["archived"].(bool), "email archive should mark the message archived")

	labelAny, err := emailTool.Execute(ctx, map[string]interface{}{
		"action": "label",
		"id":     "mail-budget",
		"labels": []interface{}{"follow-up"},
	})
	if err != nil {
		t.Fatalf("email label: %v", err)
	}
	labelResult := labelAny.(map[string]interface{})
	labelNames := labelResult["email"].(map[string]interface{})["labels"].([]string)
	foundFollowUp := false
	for _, label := range labelNames {
		if label == "follow-up" {
			foundFollowUp = true
			break
		}
	}
	categories[2].check(foundFollowUp, "email label should retain newly added labels")

	summaryAny, err := emailTool.Execute(ctx, map[string]interface{}{"action": "summarize"})
	if err != nil {
		t.Fatalf("email summarize: %v", err)
	}
	summaryResult := summaryAny.(map[string]interface{})
	categories[2].check(summaryResult["actionable_count"].(int) >= 1 && strings.TrimSpace(summaryResult["summary"].(string)) != "", "email summarize should return a human-readable actionable summary")

	researchDefs := []tools.ToolDefinition{
		{Name: "web_search"},
		{Name: "research_run"},
		{Name: "research_status"},
		{Name: "browser"},
		{Name: "calendar"},
	}
	researchFiltered := applyResearchToolPreference(researchDefs, "请调研最近一周 AI agent 的进展，并附来源引用")
	categories[3].check(strings.Join(toolDefNames(researchFiltered), ",") == "web_search,research_run,research_status,browser", fmt.Sprintf("research prompts should preserve the current research-capable toolset, got=%v", toolDefNames(researchFiltered)))
	categories[3].check(hasSearchCapabilityInToolDefs(researchFiltered), "research filtered toolset should still advertise search capability")
	categories[3].check(len(applyResearchToolPreference(researchDefs, "帮我润色这段话")) == len(researchDefs), "non-research prompts should not be force-routed into research tools")
	disabled := false
	deepDisabled := applyDeepResearchPreference(researchDefs, &disabled)
	categories[3].check(!strings.Contains(strings.Join(toolDefNames(deepDisabled), ","), "research_run"), "deep research disable flag should remove research_run from tool exposure")

	createAny, err := calendarTool.Execute(ctx, map[string]interface{}{
		"action": "create",
		"title":  "Natural language planning review",
		"time":   "tomorrow 3pm",
		"end":    "4pm",
	})
	if err != nil {
		t.Fatalf("calendar create: %v", err)
	}
	createResult := createAny.(map[string]interface{})
	createEvent := createResult["event"].(map[string]interface{})
	categories[4].check(createEvent["start_at"].(string) == "2026-03-21T15:00:00Z", "calendar create should support natural language start times")

	todayAny, err := calendarTool.Execute(ctx, map[string]interface{}{
		"action": "today",
		"date":   "2026-03-20T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("calendar today: %v", err)
	}
	todayResult := todayAny.(map[string]interface{})
	categories[4].check(todayResult["count"].(int) == 2, "calendar today should return seeded agenda items")

	dailyAny, err := calendarTool.Execute(ctx, map[string]interface{}{
		"action": "daily_summary",
		"date":   "2026-03-20T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("calendar daily summary: %v", err)
	}
	dailyResult := dailyAny.(map[string]interface{})
	categories[4].check(dailyResult["focus_item_count"].(int) == 5, "daily_summary should aggregate events, reminders, jobs, and priority email")
	categories[4].check(strings.Contains(dailyResult["summary"].(string), "Project sync"), "daily_summary should produce a readable summary, not only raw payloads")

	var weightedNumerator float64
	var totalWeight float64
	failures := make([]string, 0)
	scoreLines := make([]string, 0, len(categories))
	for _, category := range categories {
		score := category.score()
		weightedNumerator += score * category.weight
		totalWeight += category.weight
		scoreLines = append(scoreLines, fmt.Sprintf("%s=%.1f/100", category.name, score))
		if score < category.target {
			failures = append(failures, fmt.Sprintf("%s %.1f < %.1f (%s)", category.name, score, category.target, strings.Join(category.details, "; ")))
		}
	}
	weightedScore := weightedNumerator / totalWeight
	if weightedScore < 95 {
		failures = append(failures, fmt.Sprintf("weighted score %.1f < 95.0", weightedScore))
	}
	if len(failures) > 0 {
		t.Fatalf("PinchBench first-tier rescue gate failed.\nScores: %s\nFailures: %s", strings.Join(scoreLines, ", "), strings.Join(failures, " | "))
	}
}
