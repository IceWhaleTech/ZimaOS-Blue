package drivers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
)

type stubSelectorDryRunSource struct {
	response map[string]interface{}
	err      error
	queries  []string
	models   []string
}

func (s *stubSelectorDryRunSource) PreviewSelectorDryRun(_ context.Context, query string, model string) (map[string]interface{}, error) {
	s.queries = append(s.queries, query)
	s.models = append(s.models, model)
	if s.err != nil {
		return nil, s.err
	}
	raw, _ := json.Marshal(s.response)
	var cloned map[string]interface{}
	_ = json.Unmarshal(raw, &cloned)
	return cloned, nil
}

type stubHarnessDriver struct {
	kind       harness.RunKind
	validated  int
	started    int
	cancelled  int
	lastGoal   string
	lastDriver string
}

func (d *stubHarnessDriver) Kind() harness.RunKind { return d.kind }

func (d *stubHarnessDriver) Validate(spec harness.RunSpec) error {
	d.validated++
	d.lastGoal = spec.Goal
	d.lastDriver = fmt.Sprint(spec.Metadata["driver"])
	return nil
}

func (d *stubHarnessDriver) Start(_ context.Context, run *harness.Run, _ harness.RunEnv) error {
	d.started++
	d.lastGoal = run.Goal
	d.lastDriver = fmt.Sprint(run.Metadata["driver"])
	return nil
}

func (d *stubHarnessDriver) Cancel(_ context.Context, run *harness.Run) error {
	d.cancelled++
	if run != nil {
		d.lastGoal = run.Goal
		d.lastDriver = fmt.Sprint(run.Metadata["driver"])
	}
	return nil
}

func TestSelectorDryRunDriver_SubmitUsesGroupInputQueryAndCompletesRun(t *testing.T) {
	controller := newDriverTestController(t)
	source := &stubSelectorDryRunSource{
		response: map[string]interface{}{
			"selected_tools":        []interface{}{"web_query"},
			"skill_decision":        map[string]interface{}{"selected_skill": "web_query"},
			"skill_prompt_hint":     "Use web_query for latest documentation.",
			"canonical_skill_id":    "web_query",
			"skill_need_clarify":    false,
			"skill_route_outcome":   "selected",
			"decision_reason":       "ir_ranked",
			"decision_stage":        "rerank",
			"smart_skill_selection": true,
		},
	}
	controller.RegisterDriver(NewSelectorDryRunDriver(source))

	run, err := controller.Submit(context.Background(), harness.RunSpec{
		Kind: harness.RunKindAgentTask,
		Goal: "Route this query to web_query without clarification.",
		Metadata: map[string]interface{}{
			"driver": "selector_dry_run",
			"group_input": map[string]interface{}{
				"query": "Search the latest OpenAI Responses API documentation.",
				"model": "auto",
			},
			"required_fields": []string{
				"selected_tools",
				"skill_decision",
				"skill_prompt_hint",
				"canonical_skill_id",
				"skill_need_clarify",
				"skill_route_outcome",
			},
		},
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	if len(source.queries) != 1 || source.queries[0] != "Search the latest OpenAI Responses API documentation." {
		t.Fatalf("queries = %#v, want group_input.query", source.queries)
	}
	if len(source.models) != 1 || source.models[0] != "auto" {
		t.Fatalf("models = %#v, want [auto]", source.models)
	}
	if run.Status != harness.RunStatusCompleted {
		t.Fatalf("run status = %q, want completed", run.Status)
	}

	result := decodeRunResult(t, run.Result)
	if result["canonical_skill_id"] != "web_query" {
		t.Fatalf("canonical_skill_id = %#v, want web_query", result["canonical_skill_id"])
	}
	if result["skill_route_outcome"] != "selected" {
		t.Fatalf("skill_route_outcome = %#v, want selected", result["skill_route_outcome"])
	}
}

func TestSelectorDryRunDriver_RejectsMissingRequiredField(t *testing.T) {
	controller := newDriverTestController(t)
	controller.RegisterDriver(NewSelectorDryRunDriver(&stubSelectorDryRunSource{
		response: map[string]interface{}{
			"selected_tools":      []interface{}{"web_query"},
			"canonical_skill_id":  "web_query",
			"skill_need_clarify":  false,
			"skill_route_outcome": "selected",
		},
	}))

	_, err := controller.Submit(context.Background(), harness.RunSpec{
		Kind: harness.RunKindAgentTask,
		Goal: "Route this query to web_query without clarification.",
		Metadata: map[string]interface{}{
			"driver": "selector_dry_run",
			"group_input": map[string]interface{}{
				"query": "Search the latest OpenAI Responses API documentation.",
			},
			"required_fields": []string{"skill_prompt_hint"},
		},
	})
	if err == nil {
		t.Fatal("expected missing required field error")
	}
	if got := err.Error(); got != `selector dry-run response missing required field "skill_prompt_hint"` {
		t.Fatalf("error = %q, want missing required field", got)
	}
}

func TestDriverMux_RoutesByMetadataDriverName(t *testing.T) {
	defaultDriver := &stubHarnessDriver{kind: harness.RunKindAgentTask}
	overrideDriver := &stubHarnessDriver{kind: harness.RunKindAgentTask}
	mux := NewDriverMux(harness.RunKindAgentTask, defaultDriver)
	mux.Register("selector_dry_run", overrideDriver)

	defaultSpec := harness.RunSpec{
		Kind: harness.RunKindAgentTask,
		Goal: "regular task",
	}
	if err := mux.Validate(defaultSpec); err != nil {
		t.Fatalf("default Validate failed: %v", err)
	}
	if defaultDriver.validated != 1 || overrideDriver.validated != 0 {
		t.Fatalf("unexpected validate counters: default=%d override=%d", defaultDriver.validated, overrideDriver.validated)
	}

	overrideRun := &harness.Run{
		ID:     "run-1",
		Kind:   harness.RunKindAgentTask,
		Goal:   "selector task",
		Status: harness.RunStatusPending,
		Metadata: map[string]interface{}{
			"driver": "selector_dry_run",
		},
	}
	if err := mux.Start(context.Background(), overrideRun, harness.RunEnv{}); err != nil {
		t.Fatalf("override Start failed: %v", err)
	}
	if overrideDriver.started != 1 || defaultDriver.started != 0 {
		t.Fatalf("unexpected start counters: default=%d override=%d", defaultDriver.started, overrideDriver.started)
	}
}

func newDriverTestController(t *testing.T) *harness.Controller {
	t.Helper()

	tmpDir := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tmpDir, "selector-driver.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	store, err := harness.NewSQLiteStore(db)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	harnessCfg := *config.DefaultHarnessConfig()
	harnessCfg.StorePath = filepath.Join(tmpDir, "blue.db")
	harnessCfg.ArtifactRoot = filepath.Join(tmpDir, "artifacts")
	return harness.NewController(store, harness.NewPolicyResolver(harnessCfg, nil))
}

func decodeRunResult(t *testing.T, raw string) map[string]interface{} {
	t.Helper()

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("decode run result: %v raw=%s", err, raw)
	}
	return out
}
