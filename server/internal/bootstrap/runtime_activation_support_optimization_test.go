package bootstrap

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/optimization"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
)

func TestBindHarnessRuntimeOptimizationWiresControllerTriggerer(t *testing.T) {
	manager, err := optimization.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("optimization.NewManager: %v", err)
	}
	settings := serverpkg.NewSettingsHandler(kvstore.NewMemoryStore())
	settings.SetAgentcoreRunnerManager(manager)
	bundle := &HarnessRuntimeBundle{Controller: &harness.Controller{}}

	bindHarnessRuntimeOptimization(settings, bundle)

	optimizationField := reflect.ValueOf(bundle.Controller).Elem().FieldByName("optimization")
	if !optimizationField.IsValid() || optimizationField.IsNil() {
		t.Fatal("expected controller optimization triggerer to be wired")
	}
}

func TestHarnessOptimizationTriggererExecutesPreparedRunnerAndPersistsTranscript(t *testing.T) {
	bin := buildOptimizationTestRunnerBinary(t)
	root := filepath.Join(t.TempDir(), "agentcore-runner")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("MkdirAll root: %v", err)
	}
	sum := sha256FileForOptimizationTest(t, bin)
	status := optimization.Status{
		BinaryReady:    true,
		BinaryPath:     bin,
		BinarySHA256:   sum,
		ResolvedRef:    "main",
		ResolvedCommit: strings.Repeat("a", 40),
	}
	statusData, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		t.Fatalf("marshal status: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "status.json"), statusData, 0o644); err != nil {
		t.Fatalf("write status.json: %v", err)
	}
	manager, err := optimization.NewManager(root)
	if err != nil {
		t.Fatalf("optimization.NewManager: %v", err)
	}

	settings := serverpkg.NewSettingsHandler(kvstore.NewMemoryStore())
	settings.SetAgentcoreRunnerManager(manager)
	patchAgentcoreRunnerSettingsForOptimizationTest(t, settings, `{
		"experimental_agentcore_runner_enabled": true,
		"experimental_agentcore_runner_repo_url": "https://github.com/IceWhaleTech/ZimaOS-Blue",
		"experimental_agentcore_runner_ref": "main"
	}`)

	triggerer := &harnessOptimizationTriggerer{
		manager:  manager,
		settings: settings,
	}
	event := harness.OptimizationTrigger{
		Reason:              harness.OptimizationReasonExecutionGateFailed,
		CandidateID:         "candidate-123",
		EvalRunID:           "eval-456",
		BaseEvalRunID:       "base-789",
		OptimizationSurface: harness.OptimizationSurfaceRunnerCode,
		Metadata: map[string]interface{}{
			"gate": "execution",
		},
	}

	if err := triggerer.TriggerOptimization(context.Background(), event); err != nil {
		t.Fatalf("TriggerOptimization: %v", err)
	}

	current := manager.GetStatus(context.Background())
	if strings.TrimSpace(current.LastOptimizationRunID) == "" {
		t.Fatal("expected last optimization run id to be recorded")
	}
	recordPath := filepath.Join(root, "optimization-runs", current.LastOptimizationRunID+".json")
	data, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatalf("read optimization record: %v", err)
	}
	var record map[string]interface{}
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatalf("decode optimization record: %v", err)
	}
	if got := strings.TrimSpace(asStringForOptimizationTest(record["runner_artifact_path"])); got != bin {
		t.Fatalf("runner_artifact_path = %q, want %q", got, bin)
	}
	if got := strings.TrimSpace(asStringForOptimizationTest(record["runner_artifact_sha256"])); got != sum {
		t.Fatalf("runner_artifact_sha256 = %q, want %q", got, sum)
	}
	if got := strings.TrimSpace(asStringForOptimizationTest(record["runner_stop_reason"])); got != "completed" {
		t.Fatalf("runner_stop_reason = %q, want completed; record=%s", got, string(data))
	}
	if _, ok := record["runner_started_at"]; !ok {
		t.Fatalf("runner_started_at missing: record=%s", string(data))
	}
	if _, ok := record["runner_finished_at"]; !ok {
		t.Fatalf("runner_finished_at missing: record=%s", string(data))
	}
	duration := int64(record["runner_duration_ms"].(float64))
	if duration < 0 {
		t.Fatalf("runner_duration_ms = %d, want >= 0; record=%s", duration, string(data))
	}
	responseText := strings.TrimSpace(asStringForOptimizationTest(record["runner_response_text"]))
	if !strings.Contains(responseText, "candidate-123") {
		t.Fatalf("runner_response_text = %q, want candidate id evidence; record=%s", responseText, string(data))
	}
	transcript, ok := record["runner_transcript"].([]interface{})
	if !ok || len(transcript) == 0 {
		t.Fatalf("runner_transcript missing or empty: record=%s", string(data))
	}
}

func TestHarnessOptimizationTriggererPersistsRunnerErrorWhenPreparedBinaryChecksumMismatches(t *testing.T) {
	bin := buildOptimizationTestRunnerBinary(t)
	root := filepath.Join(t.TempDir(), "agentcore-runner")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("MkdirAll root: %v", err)
	}
	status := optimization.Status{
		BinaryReady:    true,
		BinaryPath:     bin,
		BinarySHA256:   strings.Repeat("f", 64),
		ResolvedRef:    "main",
		ResolvedCommit: strings.Repeat("b", 40),
	}
	statusData, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		t.Fatalf("marshal status: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "status.json"), statusData, 0o644); err != nil {
		t.Fatalf("write status.json: %v", err)
	}
	manager, err := optimization.NewManager(root)
	if err != nil {
		t.Fatalf("optimization.NewManager: %v", err)
	}

	settings := serverpkg.NewSettingsHandler(kvstore.NewMemoryStore())
	settings.SetAgentcoreRunnerManager(manager)
	patchAgentcoreRunnerSettingsForOptimizationTest(t, settings, `{
		"experimental_agentcore_runner_enabled": true,
		"experimental_agentcore_runner_repo_url": "https://github.com/IceWhaleTech/ZimaOS-Blue",
		"experimental_agentcore_runner_ref": "main"
	}`)

	triggerer := &harnessOptimizationTriggerer{
		manager:  manager,
		settings: settings,
	}
	err = triggerer.TriggerOptimization(context.Background(), harness.OptimizationTrigger{
		Reason:      harness.OptimizationReasonSelectorGateFailed,
		CandidateID: "candidate-bad-sha",
	})
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("TriggerOptimization error = %v, want checksum mismatch", err)
	}

	current := manager.GetStatus(context.Background())
	if strings.TrimSpace(current.LastOptimizationRunID) == "" {
		t.Fatal("expected last optimization run id to be recorded")
	}
	recordPath := filepath.Join(root, "optimization-runs", current.LastOptimizationRunID+".json")
	data, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatalf("read optimization record: %v", err)
	}
	var record map[string]interface{}
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatalf("decode optimization record: %v", err)
	}
	runnerError := strings.TrimSpace(asStringForOptimizationTest(record["runner_error"]))
	if !strings.Contains(runnerError, "checksum mismatch") {
		t.Fatalf("runner_error = %q, want checksum mismatch; record=%s", runnerError, string(data))
	}
	if _, ok := record["runner_started_at"]; !ok {
		t.Fatalf("runner_started_at missing: record=%s", string(data))
	}
	if _, ok := record["runner_finished_at"]; !ok {
		t.Fatalf("runner_finished_at missing: record=%s", string(data))
	}
	duration := int64(record["runner_duration_ms"].(float64))
	if duration < 0 {
		t.Fatalf("runner_duration_ms = %d, want >= 0; record=%s", duration, string(data))
	}
	if got := strings.TrimSpace(asStringForOptimizationTest(record["runner_stop_reason"])); got != "" {
		t.Fatalf("runner_stop_reason = %q, want empty on checksum failure; record=%s", got, string(data))
	}
}

func buildOptimizationTestRunnerBinary(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	serverDir := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	bin := filepath.Join(t.TempDir(), "agentcore-runner")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/agentcore-runner")
	cmd.Dir = serverDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build runner failed: %v\n%s", err, output)
	}
	return bin
}

func patchAgentcoreRunnerSettingsForOptimizationTest(t *testing.T, settings *serverpkg.SettingsHandler, body string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch, "/api/settings", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e := echo.New()
	if err := settings.Patch(e.NewContext(req, rec)); err != nil {
		t.Fatalf("settings.Patch: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("settings.Patch status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func sha256FileForOptimizationTest(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func asStringForOptimizationTest(value interface{}) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}
