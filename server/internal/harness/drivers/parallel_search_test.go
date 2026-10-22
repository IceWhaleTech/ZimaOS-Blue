package drivers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workflow"
)

type parallelWorkflowGateway struct{ gateway *tools.ToolGateway }

func (a parallelWorkflowGateway) Execute(ctx context.Context, req workflow.ToolExecutionRequest) (*workflow.ToolExecutionResult, error) {
	raw, err := json.Marshal(req.Arguments)
	if err != nil {
		return nil, err
	}
	result, err := a.gateway.Execute(ctx, tools.ToolGatewayRequest{ToolName: req.ToolName, Arguments: string(raw), RouteKind: tools.ToolRouteKindWorkflow, UserID: req.UserID, SessionID: req.SessionID})
	if err != nil {
		return nil, err
	}
	return &workflow.ToolExecutionResult{ExecutionResult: result.ExecutionResult, CompactPayload: result.CompactLLMPayload, AuditPayload: result.AuditPayload}, nil
}

func TestParallelSearchHarnessWorkflow(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.UserAgent(), "ZimaOS-Blue") {
			t.Error("missing project User-Agent")
		}
		if r.Header.Get("Authorization") != "" {
			t.Error("anonymous request carried credentials")
		}
		var req struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		result := interface{}(nil)
		switch req.Method {
		case "initialize":
			result = map[string]interface{}{"protocolVersion": "2025-03-26", "capabilities": map[string]interface{}{"tools": map[string]interface{}{}}}
		case "notifications/initialized":
			w.WriteHeader(202)
			return
		case "tools/list":
			result = map[string]interface{}{"tools": []interface{}{map[string]interface{}{"name": "web_search"}}}
		case "tools/call":
			result = map[string]interface{}{"structuredContent": map[string]interface{}{"results": []interface{}{map[string]interface{}{"title": "Blue docs", "url": "https://docs.example.com/blue", "excerpts": []string{"Focused source excerpt"}}}}}
		default:
			t.Errorf("unexpected method %q", req.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	defer server.Close()
	registry := tools.NewRegistry()
	search := tools.NewWebSearchTool(tools.WebSearchConfig{Provider: "parallel", ProviderSettings: map[string]tools.WebSearchProviderSetting{"parallel": {BaseURL: server.URL}}, CacheTTL: -1})
	registry.Register(tools.NewWebQueryTool(search, nil, nil, nil, nil))
	gateway := tools.NewToolGateway(registry, tools.NewExecutor(registry))
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "workflow.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository, err := workflow.NewRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	service, err := workflow.NewService(nil, repository)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	service.SetToolGateway(parallelWorkflowGateway{gateway: gateway})
	_, err = service.CreateWorkflow(ctx, &workflow.Workflow{ID: "parallel-search-fixture", Name: "Parallel search fixture", Status: workflow.WorkflowStatusActive, Nodes: []workflow.Node{
		{ID: "start", Name: "Start", Type: workflow.NodeTypeTrigger, Config: map[string]interface{}{"type": "manual"}},
		{ID: "search", Name: "Discover sources", Type: workflow.NodeTypeAction, Config: map[string]interface{}{"type": "skill", "skill_id": "web_query", "parameters": map[string]interface{}{"action": "search", "query": "Blue docs", "provider": "parallel", "format": "json"}}},
	}, Connections: []workflow.Connection{{ID: "start-search", SourceNode: "start", TargetNode: "search"}}})
	if err != nil {
		t.Fatal(err)
	}
	controller := newDriverTestController(t)
	controller.RegisterDriver(NewWorkflowDriver(service, controller))
	raw, err := os.ReadFile("../../../../harness/datasets/parallel-search/versions/v1/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]interface{}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	dataset, err := controller.CreateDataset(ctx, harness.DatasetSpec{Name: "parallel-search", Subject: "web-search-provider", OwnerUserID: "provider-test", DefaultRunKind: harness.RunKindWorkflow})
	if err != nil {
		t.Fatal(err)
	}
	version, err := controller.CreateDatasetVersion(ctx, dataset.ID, harness.DatasetVersionSpec{Version: "v1", SourceType: "local", Manifest: manifest})
	if err != nil {
		t.Fatal(err)
	}
	spec, err := controller.CreateEvalSpec(ctx, harness.EvalSpecSpec{Name: "Parallel provider", OwnerUserID: "provider-test", RunKind: harness.RunKindWorkflow, DatasetID: dataset.ID, DatasetVersionID: version.ID, ScoringConfig: harness.GroupScoringConfig{Mode: harness.ScoringModeRule, PassThreshold: 1}})
	if err != nil {
		t.Fatal(err)
	}
	run, err := controller.SubmitEvalRun(ctx, harness.EvalRunSpec{EvalSpecID: spec.ID, OwnerUserID: "provider-test", Metadata: map[string]interface{}{"candidate_id": "parallel-search-provider"}})
	if err != nil {
		t.Fatal(err)
	}
	dispatcher := harness.NewGroupDispatcher(controller)
	dispatcher.SetRunPollInterval(10 * time.Millisecond)
	if err := dispatcher.DispatchOnce(ctx); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		report, err := controller.GetEvalRunReport(ctx, run.ID)
		if err != nil {
			t.Fatal(err)
		}
		if report.GroupReport != nil && len(report.GroupReport.Scorecards) > 0 {
			if report.GroupReport.PassRate != 1 {
				b, _ := json.MarshalIndent(report, "", "  ")
				t.Fatalf("Harness score failed: %s", b)
			}
			if len(report.GroupReport.LinkedRuns) != 1 {
				t.Fatal("missing workflow evidence")
			}
			executionID, _ := report.GroupReport.LinkedRuns[0].Metadata["workflow_execution_id"].(string)
			execution, err := service.GetExecution(ctx, executionID)
			if err != nil {
				t.Fatal(err)
			}
			output, err := json.Marshal(execution.NodeResults["search"].Output)
			if err != nil {
				t.Fatal(err)
			}
			items := manifest["items"].([]interface{})
			metadata := items[0].(map[string]interface{})["metadata"].(map[string]interface{})
			for _, value := range metadata["required_output"].([]interface{}) {
				expected := value.(string)
				if !strings.Contains(string(output), expected) {
					t.Fatalf("workflow did not consume expected source %q: %s", expected, output)
				}
			}
			t.Logf("Harness dataset=%s version=%s candidate=parallel-search-provider score=%v", dataset.Name, version.Version, report.GroupReport.PassRate)
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("Harness eval did not finish")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
