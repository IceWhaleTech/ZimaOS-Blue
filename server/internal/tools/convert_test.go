package tools

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	convertpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/convert"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	_ "github.com/mattn/go-sqlite3"
)

func TestParseConvertTaskRequestSupportsNestedCamelCaseArgs(t *testing.T) {
	parsed, err := parseConvertTaskRequest(map[string]interface{}{
		"input": map[string]interface{}{
			"action":       "tts",
			"targetFormat": "wav",
			"message":      "hello world",
			"taskId":       "task-1",
			"waitMs":       1500,
			"sources":      []interface{}{"/tmp/a.txt", "/tmp/b.txt"},
			"options": map[string]interface{}{
				"speech": map[string]interface{}{
					"tts": map[string]interface{}{
						"format": "mp3",
					},
					"asr": map[string]interface{}{
						"onDeviceOnly": true,
					},
				},
				"video": map[string]interface{}{
					"startMs":         100,
					"endMs":           200,
					"frameIntervalMs": 50,
					"segments": []interface{}{
						map[string]interface{}{"startMs": 1, "endMs": 2},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("parseConvertTaskRequest() error = %v", err)
	}
	req := parsed.TaskRequest
	if req.Action != string(convertpkg.ActionTTS) {
		t.Fatalf("action = %q, want %q", req.Action, convertpkg.ActionTTS)
	}
	if req.TargetFormat != "mp3" {
		t.Fatalf("target_format = %q, want mp3", req.TargetFormat)
	}
	if req.Text != "hello world" {
		t.Fatalf("text = %q, want hello world", req.Text)
	}
	if req.TaskID != "task-1" || req.WaitMS != 1500 {
		t.Fatalf("unexpected task fields: %#v", req)
	}
	if len(req.Sources) != 2 {
		t.Fatalf("sources = %#v, want 2", req.Sources)
	}
	if !req.Options.Speech.ASR.OnDeviceOnly {
		t.Fatalf("expected on-device-only true")
	}
	if req.Options.Video.StartMS != 100 || req.Options.Video.EndMS != 200 || req.Options.Video.FrameIntervalMS != 50 {
		t.Fatalf("unexpected video options: %#v", req.Options.Video)
	}
	if len(req.Options.Video.Segments) != 1 || req.Options.Video.Segments[0].StartMS != 1 || req.Options.Video.Segments[0].EndMS != 2 {
		t.Fatalf("unexpected segments: %#v", req.Options.Video.Segments)
	}
}

func TestParseConvertTaskRequestSupportsSimpleInputOutputPaths(t *testing.T) {
	parsed, err := parseConvertTaskRequest(map[string]interface{}{
		"input":  "docs/report.docx",
		"output": "exports/report.pdf",
	})
	if err != nil {
		t.Fatalf("parseConvertTaskRequest() error = %v", err)
	}
	if !parsed.SimpleMode {
		t.Fatal("expected simple mode to be enabled")
	}
	req := parsed.TaskRequest
	if req.Action != string(convertpkg.ActionConvert) {
		t.Fatalf("action = %q, want %q", req.Action, convertpkg.ActionConvert)
	}
	if len(req.Sources) != 1 || req.Sources[0] != "docs/report.docx" {
		t.Fatalf("sources = %#v, want docs/report.docx", req.Sources)
	}
	if req.OutputPath != "exports/report.pdf" {
		t.Fatalf("output_path = %q, want exports/report.pdf", req.OutputPath)
	}
	if req.TargetFormat != "pdf" {
		t.Fatalf("target_format = %q, want pdf", req.TargetFormat)
	}
}

func TestParseConvertTaskRequestSupportsPresentationOptions(t *testing.T) {
	parsed, err := parseConvertTaskRequest(map[string]interface{}{
		"input_path":  "docs/deck.md",
		"output_path": "exports/deck.pptx",
		"theme":       "coral",
		"subtitle":    "Launch Week",
		"options": map[string]interface{}{
			"presentation": map[string]interface{}{
				"theme":     "forest",
				"styleHint": "startup pitch",
				"title":     "Quarterly Product Launch",
			},
			"pptx": map[string]interface{}{
				"summary": map[string]interface{}{
					"text": "Fast install and smoother onboarding",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("parseConvertTaskRequest() error = %v", err)
	}

	presentation := parsed.TaskRequest.Options.Presentation
	if presentation.Theme != "coral" {
		t.Fatalf("theme = %q, want coral", presentation.Theme)
	}
	if presentation.StyleHint != "startup pitch" {
		t.Fatalf("style_hint = %q, want startup pitch", presentation.StyleHint)
	}
	if presentation.Title != "Quarterly Product Launch" {
		t.Fatalf("title = %q, want Quarterly Product Launch", presentation.Title)
	}
	if presentation.Subtitle != "Launch Week" {
		t.Fatalf("subtitle = %q, want Launch Week", presentation.Subtitle)
	}
	if presentation.Summary != "Fast install and smoother onboarding" {
		t.Fatalf("summary = %q, want Fast install and smoother onboarding", presentation.Summary)
	}
}

func TestConvertToolMaybeHandleNativeOfficeConvertDOCX(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "convert_tool_docx_native.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	service, err := convertpkg.NewService(db, t.TempDir())
	if err != nil {
		t.Fatalf("new convert service: %v", err)
	}
	defer service.Close()

	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(workspace): %v", err)
	}
	sourcePath := filepath.Join(workspaceDir, "launch.md")
	sourceMarkdown := "# Launch Brief\n\nVisit [Portal](https://example.com) for rollout details.\n"
	if err := os.WriteFile(sourcePath, []byte(sourceMarkdown), 0o644); err != nil {
		t.Fatalf("WriteFile(source): %v", err)
	}

	tool := NewConvertTool(service, nil, nil, []string{workspaceDir})
	args := map[string]interface{}{
		"input_path":  "launch.md",
		"output_path": "exports/launch.docx",
		"theme":       "forest",
		"summary":     "Executive recap",
	}

	parsed, err := parseConvertTaskRequest(args)
	if err != nil {
		t.Fatalf("parseConvertTaskRequest() error = %v", err)
	}
	req, err := tool.resolveTaskPaths(context.Background(), parsed.TaskRequest)
	if err != nil {
		t.Fatalf("resolveTaskPaths() error = %v", err)
	}

	raw, handled, err := tool.maybeHandleNativeOfficeConvert(context.Background(), args, req)
	if err != nil {
		t.Fatalf("maybeHandleNativeOfficeConvert() error = %v", err)
	}
	if !handled {
		t.Fatal("expected native office convert path to handle docx request")
	}

	result, ok := raw.(map[string]interface{})
	if !ok {
		t.Fatalf("result type = %T, want map[string]interface{}", raw)
	}
	outputPath, _ := result["output_path"].(string)
	if outputPath == "" {
		t.Fatalf("missing output_path in result: %#v", result)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile(output): %v", err)
	}
	stylesXML := officeZipEntryText(t, data, "word/styles.xml")
	for _, needle := range []string{
		`w:ascii="Times New Roman"`,
		`w:ascii="Open Sans"`,
	} {
		if !strings.Contains(stylesXML, needle) {
			t.Fatalf("expected styles.xml to include %q, got %s", needle, stylesXML)
		}
	}

	documentXML := officeZipEntryText(t, data, "word/document.xml")
	if !strings.Contains(documentXML, `w:color w:val="D4A017"`) {
		t.Fatalf("expected document.xml to include forest accent color, got %s", documentXML)
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), outputPath)
	if err != nil {
		t.Fatalf("ReadDocument(%s) error = %v", outputPath, err)
	}
	for _, needle := range []string{"Launch Brief", "Executive recap", "Portal"} {
		if !strings.Contains(doc.Text, needle) {
			t.Fatalf("document text missing %q in %q", needle, doc.Text)
		}
	}
}

func TestConvertToolMaybeHandleNativeOfficeConvertDOCXDefaultsToMidnightTheme(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "convert_tool_docx_native_default_theme.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	service, err := convertpkg.NewService(db, t.TempDir())
	if err != nil {
		t.Fatalf("new convert service: %v", err)
	}
	defer service.Close()

	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(workspace): %v", err)
	}
	sourcePath := filepath.Join(workspaceDir, "brief.md")
	sourceMarkdown := "# Weekly Brief\n\nA stronger default theme should still look intentional.\n"
	if err := os.WriteFile(sourcePath, []byte(sourceMarkdown), 0o644); err != nil {
		t.Fatalf("WriteFile(source): %v", err)
	}

	tool := NewConvertTool(service, nil, nil, []string{workspaceDir})
	args := map[string]interface{}{
		"input_path":  "brief.md",
		"output_path": "exports/brief.docx",
		"summary":     "No explicit theme provided",
	}

	parsed, err := parseConvertTaskRequest(args)
	if err != nil {
		t.Fatalf("parseConvertTaskRequest() error = %v", err)
	}
	req, err := tool.resolveTaskPaths(context.Background(), parsed.TaskRequest)
	if err != nil {
		t.Fatalf("resolveTaskPaths() error = %v", err)
	}

	raw, handled, err := tool.maybeHandleNativeOfficeConvert(context.Background(), args, req)
	if err != nil {
		t.Fatalf("maybeHandleNativeOfficeConvert() error = %v", err)
	}
	if !handled {
		t.Fatal("expected native office convert path to handle docx request")
	}

	result, ok := raw.(map[string]interface{})
	if !ok {
		t.Fatalf("result type = %T, want map[string]interface{}", raw)
	}
	outputPath, _ := result["output_path"].(string)
	if outputPath == "" {
		t.Fatalf("missing output_path in result: %#v", result)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile(output): %v", err)
	}
	stylesXML := officeZipEntryText(t, data, "word/styles.xml")
	for _, needle := range []string{
		`w:ascii="Helvetica Neue"`,
		`w:ascii="Helvetica"`,
	} {
		if !strings.Contains(stylesXML, needle) {
			t.Fatalf("expected styles.xml to include %q for the default midnight theme, got %s", needle, stylesXML)
		}
	}
}

func TestConvertToolMaybeHandleNativeOfficeConvertXLSXFromCSV(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "convert_tool_xlsx_native.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	service, err := convertpkg.NewService(db, t.TempDir())
	if err != nil {
		t.Fatalf("new convert service: %v", err)
	}
	defer service.Close()

	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(workspace): %v", err)
	}
	sourcePath := filepath.Join(workspaceDir, "metrics.csv")
	sourceCSV := "Metric,Value\nSignups,42\nActivation,0.63\n"
	if err := os.WriteFile(sourcePath, []byte(sourceCSV), 0o644); err != nil {
		t.Fatalf("WriteFile(source): %v", err)
	}

	tool := NewConvertTool(service, nil, nil, []string{workspaceDir})
	args := map[string]interface{}{
		"input_path":  "metrics.csv",
		"output_path": "exports/metrics.xlsx",
		"theme":       "midnight",
		"title":       "Launch Scorecard",
		"subtitle":    "Spring 2026",
	}

	parsed, err := parseConvertTaskRequest(args)
	if err != nil {
		t.Fatalf("parseConvertTaskRequest() error = %v", err)
	}
	req, err := tool.resolveTaskPaths(context.Background(), parsed.TaskRequest)
	if err != nil {
		t.Fatalf("resolveTaskPaths() error = %v", err)
	}

	raw, handled, err := tool.maybeHandleNativeOfficeConvert(context.Background(), args, req)
	if err != nil {
		t.Fatalf("maybeHandleNativeOfficeConvert() error = %v", err)
	}
	if !handled {
		t.Fatal("expected native office convert path to handle xlsx request")
	}

	result, ok := raw.(map[string]interface{})
	if !ok {
		t.Fatalf("result type = %T, want map[string]interface{}", raw)
	}
	outputPath, _ := result["output_path"].(string)
	if outputPath == "" {
		t.Fatalf("missing output_path in result: %#v", result)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile(output): %v", err)
	}
	stylesXML := officeZipEntryText(t, data, "xl/styles.xml")
	for _, needle := range []string{
		`rgb="FF242C38"`,
		`name val="Helvetica Neue"`,
		`name val="Helvetica"`,
	} {
		if !strings.Contains(stylesXML, needle) {
			t.Fatalf("expected xl/styles.xml to include %q, got %s", needle, stylesXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), outputPath)
	if err != nil {
		t.Fatalf("ReadDocument(%s) error = %v", outputPath, err)
	}
	for _, needle := range []string{"Launch Scorecard", "Signups", "Activation", "42"} {
		if !strings.Contains(doc.Text, needle) {
			t.Fatalf("workbook text missing %q in %q", needle, doc.Text)
		}
	}
}

func TestConvertToolMaybeHandleNativeOfficeConvertPDF(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "convert_tool_pdf_native.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	service, err := convertpkg.NewService(db, t.TempDir())
	if err != nil {
		t.Fatalf("new convert service: %v", err)
	}
	defer service.Close()

	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(workspace): %v", err)
	}
	sourcePath := filepath.Join(workspaceDir, "weekly.md")
	sourceMarkdown := "# Weekly Update\n\n## Highlights\n\n- Faster setup\n- Smoother onboarding\n"
	if err := os.WriteFile(sourcePath, []byte(sourceMarkdown), 0o644); err != nil {
		t.Fatalf("WriteFile(source): %v", err)
	}

	tool := NewConvertTool(service, nil, nil, []string{workspaceDir})
	args := map[string]interface{}{
		"input_path":  "weekly.md",
		"output_path": "exports/weekly.pdf",
		"title":       "Weekly Update",
		"summary":     "Executive recap",
		"theme":       "midnight",
	}

	parsed, err := parseConvertTaskRequest(args)
	if err != nil {
		t.Fatalf("parseConvertTaskRequest() error = %v", err)
	}
	req, err := tool.resolveTaskPaths(context.Background(), parsed.TaskRequest)
	if err != nil {
		t.Fatalf("resolveTaskPaths() error = %v", err)
	}

	raw, handled, err := tool.maybeHandleNativeOfficeConvert(context.Background(), args, req)
	if err != nil {
		t.Fatalf("maybeHandleNativeOfficeConvert() error = %v", err)
	}
	if !handled {
		t.Fatal("expected native office convert path to handle pdf request")
	}

	result, ok := raw.(map[string]interface{})
	if !ok {
		t.Fatalf("result type = %T, want map[string]interface{}", raw)
	}
	outputPath, _ := result["output_path"].(string)
	if outputPath == "" {
		t.Fatalf("missing output_path in result: %#v", result)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile(output): %v", err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF")) {
		t.Fatalf("expected PDF header, got %q", string(data[:minInt(len(data), 8)]))
	}
	for _, needle := range [][]byte{
		[]byte("Weekly Update"),
		[]byte("Executive recap"),
		[]byte("Faster setup"),
	} {
		if !bytes.Contains(data, needle) {
			t.Fatalf("expected generated PDF bytes to contain %q", string(needle))
		}
	}
}

func TestConvertToolResolveLocalPathsSupportsRelativeWorkspacePaths(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "convert_tool_relative.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	service, err := convertpkg.NewService(db, t.TempDir())
	if err != nil {
		t.Fatalf("new convert service: %v", err)
	}
	defer service.Close()

	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	tool := NewConvertTool(service, nil, nil, []string{workspaceDir})

	paths, err := tool.resolveLocalPaths(context.Background(), []string{"docs/input.md", "exports/output.pdf"})
	if err != nil {
		t.Fatalf("resolveLocalPaths() error = %v", err)
	}
	if got, want := paths[0], filepath.Join(workspaceDir, "docs", "input.md"); got != want {
		t.Fatalf("paths[0] = %q, want %q", got, want)
	}
	if got, want := paths[1], filepath.Join(workspaceDir, "exports", "output.pdf"); got != want {
		t.Fatalf("paths[1] = %q, want %q", got, want)
	}
}

func TestConvertToolResolveLocalPathsPreservesWorkspaceRootPrecedenceOverAgentRoots(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "convert_tool_root_precedence.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	service, err := convertpkg.NewService(db, t.TempDir())
	if err != nil {
		t.Fatalf("new convert service: %v", err)
	}
	defer service.Close()

	baseDir := t.TempDir()
	workspaceDir := filepath.Join(baseDir, ".zimaos-blue", "data", "workspace")
	agentsDir := filepath.Join(baseDir, ".agents")
	tool := NewConvertTool(service, nil, nil, []string{workspaceDir, agentsDir})

	paths, err := tool.resolveLocalPaths(context.Background(), []string{"leave_application_template.md", "leave_application.docx"})
	if err != nil {
		t.Fatalf("resolveLocalPaths() error = %v", err)
	}
	if got, want := paths[0], filepath.Join(workspaceDir, "leave_application_template.md"); got != want {
		t.Fatalf("paths[0] = %q, want %q", got, want)
	}
	if got, want := paths[1], filepath.Join(workspaceDir, "leave_application.docx"); got != want {
		t.Fatalf("paths[1] = %q, want %q", got, want)
	}
}

func TestConvertToolRequestLocalPathApprovalAllowAlwaysPersistsDirectory(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "convert_tool_test.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	service, err := convertpkg.NewService(db, t.TempDir())
	if err != nil {
		t.Fatalf("new convert service: %v", err)
	}
	defer service.Close()

	dirStore, err := NewDirAllowlistStore(db)
	if err != nil {
		t.Fatalf("new dir allowlist store: %v", err)
	}

	userID := "user-convert-test"
	broker := sse.NewBroker()
	sub := broker.Subscribe(userID)
	defer broker.Unsubscribe(userID, sub)
	approvals := NewApprovalManager(broker)
	tool := NewConvertTool(service, approvals, dirStore, nil)

	localFile := filepath.Join(t.TempDir(), "outside", "input.txt")
	localDir := filepath.Dir(localFile)

	done := make(chan error, 1)
	ctx, cancel := context.WithTimeout(WithUserID(context.Background(), userID), 5*time.Second)
	defer cancel()
	go func() {
		done <- tool.requestLocalPathApproval(ctx, []string{localFile})
	}()

	deadline := time.After(5 * time.Second)
	var req *ApprovalRequest
	for req == nil {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for approval request")
		default:
			req = approvals.GetPending(userID)
			if req == nil {
				time.Sleep(10 * time.Millisecond)
			}
		}
	}
	if !approvals.ResolveApprovalWithBinding(req.ID, ApprovalAllowAlways, req.BindingHash) {
		t.Fatalf("failed to resolve pending approval: %+v", req)
	}

	if err := <-done; err != nil {
		t.Fatalf("first requestLocalPathApproval() error = %v", err)
	}

	if entry := dirStore.Match(localDir); entry == nil {
		t.Fatalf("expected %q to be persisted in dir allowlist", localDir)
	}

	ctx2, cancel2 := context.WithTimeout(WithUserID(context.Background(), userID), 200*time.Millisecond)
	defer cancel2()
	if err := tool.requestLocalPathApproval(ctx2, []string{localFile}); err != nil {
		t.Fatalf("second requestLocalPathApproval() should skip approval for persisted dir, got %v", err)
	}
}

func TestSimpleConvertTaskResultOmitsTaskIDForSyncResults(t *testing.T) {
	task := &convertpkg.ConvertTask{
		ID:           "task-sync",
		Status:       convertpkg.StatusSucceeded,
		Action:       convertpkg.ActionConvert,
		Message:      "Document converted",
		TargetFormat: "pdf",
	}

	got := simpleConvertTaskResult(task, false)
	if _, ok := got["task_id"]; ok {
		t.Fatalf("task_id should be omitted for sync results: %#v", got)
	}
	if got["async"] != false {
		t.Fatalf("async = %#v, want false", got["async"])
	}
}

func TestSimpleConvertTaskResultIncludesTaskIDForAsyncResults(t *testing.T) {
	task := &convertpkg.ConvertTask{
		ID:           "task-async",
		Status:       convertpkg.StatusProcessing,
		Action:       convertpkg.ActionConvert,
		Message:      "Processing",
		TargetFormat: "pdf",
	}

	got := simpleConvertTaskResult(task, true)
	if got["task_id"] != "task-async" {
		t.Fatalf("task_id = %#v, want task-async", got["task_id"])
	}
	if got["async"] != true {
		t.Fatalf("async = %#v, want true", got["async"])
	}
}

func TestConvertWaitDefaultsAndCapsAreLonger(t *testing.T) {
	if defaultConvertSyncWait != 5*time.Minute {
		t.Fatalf("defaultConvertSyncWait = %v, want %v", defaultConvertSyncWait, 5*time.Minute)
	}
	if maxConvertWait != 300*time.Second {
		t.Fatalf("maxConvertWait = %v, want %v", maxConvertWait, 300*time.Second)
	}
	if got := requestedConvertWait(600_000); got != 300*time.Second {
		t.Fatalf("requestedConvertWait clamp = %v, want %v", got, 300*time.Second)
	}
}
