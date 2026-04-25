package tools

import (
	"context"
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	a11yruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/a11y"
)

func writeCUATestPNG(t *testing.T, width int, height int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "screenshot.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create test png: %v", err)
	}
	defer file.Close()
	if err := png.Encode(file, image.NewRGBA(image.Rect(0, 0, width, height))); err != nil {
		t.Fatalf("encode test png: %v", err)
	}
	return path
}

func TestNormalizeA11yAction_MapsCUAActionNames(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "open_app", want: a11yruntime.ActionFocus},
		{in: "input_text", want: "type"},
		{in: "Click", want: "click"},
		{in: "RightSingle", want: "click"},
		{in: "Drag", want: "drag"},
		{in: "move_mouse", want: a11yruntime.ActionPointerMove},
		{in: "scroll_down", want: a11yruntime.ActionScroll},
		{in: "scroll_up", want: a11yruntime.ActionScroll},
		{in: "Hotkey", want: a11yruntime.ActionKey},
		{in: "multi_Hotkey", want: a11yruntime.ActionKey},
		{in: "wait", want: "wait"},
		{in: "record_info", want: "record_info"},
		{in: "done", want: "done"},
		{in: "task", want: "task"},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := normalizeA11yAction(tt.in); got != tt.want {
				t.Fatalf("normalizeA11yAction(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestDecodeCUAJSONUsesFirstCompleteObject(t *testing.T) {
	raw := `{"analysis":"ok","step_evaluate":"Needs action","next_goal":"click target","done":false}
Thought: now I will provide actor guidance {"ignored":true}`

	var output cuaBrainOutput
	if err := decodeCUAJSON(raw, &output); err != nil {
		t.Fatalf("decodeCUAJSON() error = %v", err)
	}
	if output.Analysis != "ok" || output.NextGoal != "click target" || output.Done {
		t.Fatalf("decoded output = %#v", output)
	}
}

func TestResolveA11yActionUsesCUANameAndTypeAliases(t *testing.T) {
	if got := resolveA11yAction(map[string]interface{}{"name": "Click"}); got != "click" {
		t.Fatalf("resolveA11yAction(name=Click) = %q, want click", got)
	}
	if got := resolveA11yAction(map[string]interface{}{"type": "Hotkey", "key": "enter"}); got != a11yruntime.ActionKey {
		t.Fatalf("resolveA11yAction(type=Hotkey) = %q, want %s", got, a11yruntime.ActionKey)
	}
}

func TestA11yToolExecute_CUAClickAcceptsCoordinateObjectAliases(t *testing.T) {
	backend := &a11yCompatBackend{}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"name":      "Click",
		"window_id": "win-1",
		"coordinate": map[string]interface{}{
			"x": 250,
			"y": 750,
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if backend.lastPointClickWindow != "win-1" {
		t.Fatalf("lastPointClickWindow = %q, want win-1", backend.lastPointClickWindow)
	}
	if backend.lastPointClick.X != 0.25 || backend.lastPointClick.Y != 0.75 {
		t.Fatalf("lastPointClick = %#v, want 0.25,0.75", backend.lastPointClick)
	}
}

func TestA11yToolExecute_CUAInputTextAcceptsBodyAlias(t *testing.T) {
	backend := &a11yCompatBackend{}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"type":      "input_text",
		"window_id": "win-1",
		"body":      "hello from body",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if backend.lastFocusedTypeWindowID != "win-1" {
		t.Fatalf("lastFocusedTypeWindowID = %q, want win-1", backend.lastFocusedTypeWindowID)
	}
	if backend.lastFocusedTypeValue != "hello from body" {
		t.Fatalf("lastFocusedTypeValue = %q, want body text", backend.lastFocusedTypeValue)
	}
}

func TestA11yToolExecute_CUAHotkeyAcceptsShortcutAlias(t *testing.T) {
	backend := &a11yCompatBackend{}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"type":      "Hotkey",
		"window_id": "win-1",
		"shortcut":  "cmd+k",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := strings.Join(backend.lastKeys, "+"); got != "cmd+k" {
		t.Fatalf("lastKeys = %q, want cmd+k", got)
	}
}

func TestA11yToolExecute_CUAScrollAcceptsAmountAlias(t *testing.T) {
	backend := &a11yCompatBackend{}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"name":      "scroll_down",
		"window_id": "win-1",
		"amount":    7,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if backend.lastScrollWindowID != "win-1" {
		t.Fatalf("lastScrollWindowID = %q, want win-1", backend.lastScrollWindowID)
	}
	if backend.lastScrollDirection != "down" || backend.lastScrollLines != 7 {
		t.Fatalf("scroll = %s/%d, want down/7", backend.lastScrollDirection, backend.lastScrollLines)
	}
}

func TestA11yToolExecute_CUAPointerMoveAcceptsCoordinateAlias(t *testing.T) {
	backend := &a11yCompatBackend{}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"name": "move_mouse",
		"coordinate": map[string]interface{}{
			"x": 125,
			"y": 875,
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if backend.lastPointerMoveX != 125 || backend.lastPointerMoveY != 875 {
		t.Fatalf("pointer = %d,%d, want 125,875", backend.lastPointerMoveX, backend.lastPointerMoveY)
	}
}

func TestA11yToolExecute_CUAClickUsesNormalizedPosition(t *testing.T) {
	backend := &a11yCompatBackend{}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "Click",
		"window_id": "win-1",
		"position":  []interface{}{0.25, 0.75},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if backend.lastPointClickWindow != "win-1" {
		t.Fatalf("lastPointClickWindow = %q, want win-1", backend.lastPointClickWindow)
	}
	if backend.lastPointClick.X != 0.25 || backend.lastPointClick.Y != 0.75 {
		t.Fatalf("lastPointClick = %#v, want normalized 0.25,0.75", backend.lastPointClick)
	}
}

func TestA11yToolExecute_CUAClickParsesCLIPositionString(t *testing.T) {
	backend := &a11yCompatBackend{}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "Click",
		"window_id": "win-1",
		"position":  "[0.25,0.75]",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if backend.lastPointClickWindow != "win-1" {
		t.Fatalf("lastPointClickWindow = %q, want win-1", backend.lastPointClickWindow)
	}
	if backend.lastPointClick.X != 0.25 || backend.lastPointClick.Y != 0.75 {
		t.Fatalf("lastPointClick = %#v, want normalized 0.25,0.75", backend.lastPointClick)
	}
}

func TestA11yToolExecute_CUAClickScalesZeroToThousandCoordinates(t *testing.T) {
	backend := &a11yCompatBackend{}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "Click",
		"window_id": "win-1",
		"position":  []interface{}{250, 750},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if backend.lastPointClick.X != 0.25 || backend.lastPointClick.Y != 0.75 {
		t.Fatalf("lastPointClick = %#v, want scaled normalized 0.25,0.75", backend.lastPointClick)
	}
}

func TestA11yToolExecute_CUAHotkeysMapToKeySequence(t *testing.T) {
	backend := &a11yCompatBackend{}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "multi_Hotkey",
		"window_id": "win-1",
		"key1":      "command",
		"key2":      "shift",
		"key3":      "2",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := strings.Join(backend.lastKeys, "+"); got != "cmd+shift+2" {
		t.Fatalf("lastKeys = %q, want cmd+shift+2", got)
	}
}

func TestA11yToolExecute_RecordInfoStoresTaskMemory(t *testing.T) {
	tool := NewA11yTool()

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "record_info",
		"file_name": "price.txt",
		"text":      "iPhone costs $799",
	})
	if err != nil {
		t.Fatalf("Execute(record_info) error = %v", err)
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "task",
		"goal":   "read recorded files",
		"read_files": []interface{}{
			"price.txt",
		},
		"max_steps": 1,
	})
	if err != nil {
		t.Fatalf("Execute(task) error = %v", err)
	}
	if !strings.Contains(asString(result), "iPhone costs $799") {
		t.Fatalf("task result = %v, want recorded memory content", result)
	}
}

func TestA11yToolExecute_TaskDefaultReturnsMultiRoleEnvelope(t *testing.T) {
	backend := &a11yCompatBackend{}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "task",
		"goal":      "open notes and summarize latest message",
		"max_steps": 1,
	})
	if err != nil {
		t.Fatalf("Execute(task cua) error = %v", err)
	}
	text := asString(result)
	for _, want := range []string{"\"mode\":\"cua\"", "\"planner\"", "\"brain\"", "\"actor\"", "\"memory\"", "\"step_evaluate\""} {
		if !strings.Contains(text, want) {
			t.Fatalf("task cua result = %s, want %s", text, want)
		}
	}
}

func TestA11yToolExecute_TaskDefaultUsesCUALoop(t *testing.T) {
	backend := &a11yCompatBackend{}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "task",
		"goal":   "open notes",
	})
	if err != nil {
		t.Fatalf("Execute(task default) error = %v", err)
	}
	text := asString(result)
	if !strings.Contains(text, "\"mode\":\"cua\"") {
		t.Fatalf("default task result = %s, want cua mode", text)
	}
}

func TestA11yToolExecute_TaskDefaultSkipsObservationWhenRequiredPermissionMissing(t *testing.T) {
	backend := &a11yCompatBackend{
		capabilities: a11yruntime.CapabilitiesResult{
			HostOS: "darwin",
			Permissions: []a11yruntime.PermissionStatus{{
				Name:     "accessibility",
				Required: true,
				Granted:  false,
				Message:  "grant accessibility",
			}},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "task",
		"goal":      "inspect safely",
		"max_steps": 1,
	})
	if err != nil {
		t.Fatalf("Execute(task) error = %v", err)
	}
	text := asString(raw)
	if !strings.Contains(text, "\"status\":\"blocked\"") || !strings.Contains(text, "\"error_code\":\"permission_required\"") || !strings.Contains(text, "required desktop permission is not granted") {
		t.Fatalf("task result = %s, want blocked payload with permission explanation", text)
	}
	if backend.interactiveCalls != 0 {
		t.Fatalf("interactiveCalls = %d, want 0 when required permission is missing", backend.interactiveCalls)
	}
	if backend.lastScreenshotWindow != "" {
		t.Fatalf("lastScreenshotWindow = %q, want no screenshot when required permission is missing", backend.lastScreenshotWindow)
	}
}

func TestA11yToolExecute_TaskDefaultDoesNotCallLLMWhenRequiredPermissionMissing(t *testing.T) {
	backend := &a11yCompatBackend{
		capabilities: a11yruntime.CapabilitiesResult{
			HostOS:      "darwin",
			Permissions: []a11yruntime.PermissionStatus{{Name: "accessibility", Required: true, Granted: false}},
		},
	}
	bridge := &mockLLMBridge{responses: []string{
		`{"analysis":"should not run","step_evaluate":"Success","done":true}`,
	}}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	tool.SetLLMBridge(bridge)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "task",
		"goal":   "inspect safely",
	})
	if err != nil {
		t.Fatalf("Execute(task) error = %v", err)
	}
	if len(bridge.calls) != 0 {
		t.Fatalf("LLM calls = %d, want 0 for missing required permission", len(bridge.calls))
	}
	if !strings.Contains(asString(raw), "\"error_code\":\"permission_required\"") {
		t.Fatalf("task result = %s, want permission_required", asString(raw))
	}
}

func TestA11yToolExecute_TaskDefaultRunsBrainActorLoop(t *testing.T) {
	backend := &a11yCompatBackend{}
	brain := &fakeCUABrain{outputs: []cuaBrainOutput{
		{
			Analysis:     "need to remember and click",
			StepEvaluate: "Needs action",
			NextGoal:     "remember target and click center",
		},
		{
			Analysis:     "clicked target",
			StepEvaluate: "Success",
			Done:         true,
		},
	}}
	actor := &fakeCUAActor{outputs: []cuaActorOutput{
		{Actions: []map[string]interface{}{
			{"action": "record_info", "file_name": "target.txt", "text": "continue button is centered"},
			{"action": "Click", "position": []interface{}{500, 750}},
		}},
	}}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	tool.cuaBrain = brain
	tool.cuaActor = actor

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "task",
		"goal":      "click the continue button",
		"window_id": "win-1",
		"max_steps": 3,
	})
	if err != nil {
		t.Fatalf("Execute(task cua) error = %v", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(asString(raw)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["status"] != "completed" {
		t.Fatalf("status = %v, want completed; out=%s", out["status"], asString(raw))
	}
	if backend.lastPointClickWindow != "win-1" {
		t.Fatalf("lastPointClickWindow = %q, want win-1", backend.lastPointClickWindow)
	}
	if backend.lastPointClick.X != 0.5 || backend.lastPointClick.Y != 0.75 {
		t.Fatalf("lastPointClick = %#v, want scaled 0.5,0.75", backend.lastPointClick)
	}
	if len(brain.inputs) != 2 {
		t.Fatalf("brain inputs = %d, want 2", len(brain.inputs))
	}
	if len(actor.inputs) != 1 {
		t.Fatalf("actor inputs = %d, want 1", len(actor.inputs))
	}
	readBack, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":     "task",
		"read_files": []interface{}{"target.txt"},
	})
	if err != nil {
		t.Fatalf("read memory error = %v", err)
	}
	if !strings.Contains(asString(readBack), "continue button is centered") {
		t.Fatalf("memory readback = %s, want recorded info", asString(readBack))
	}
}

func TestA11yToolExecute_TaskDefaultStopsAtMaxSteps(t *testing.T) {
	backend := &a11yCompatBackend{}
	brain := &fakeCUABrain{outputs: []cuaBrainOutput{
		{Analysis: "first", StepEvaluate: "Needs action", NextGoal: "wait"},
		{Analysis: "second", StepEvaluate: "Needs action", NextGoal: "wait again"},
	}}
	actor := &fakeCUAActor{outputs: []cuaActorOutput{
		{Actions: []map[string]interface{}{{"action": "wait", "ms": 1}}},
		{Actions: []map[string]interface{}{{"action": "wait", "ms": 1}}},
	}}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	tool.cuaBrain = brain
	tool.cuaActor = actor

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "task",
		"goal":      "never done",
		"max_steps": 2,
	})
	if err != nil {
		t.Fatalf("Execute(task cua) error = %v", err)
	}
	text := asString(raw)
	if !strings.Contains(text, "\"status\":\"stopped\"") || !strings.Contains(text, "\"error_code\":\"max_steps\"") {
		t.Fatalf("task result = %s, want max_steps stopped", text)
	}
}

func TestA11yToolExecute_TaskDefaultExecutesInputHotkeyAndDone(t *testing.T) {
	backend := &a11yCompatBackend{}
	brain := &fakeCUABrain{outputs: []cuaBrainOutput{
		{Analysis: "type and submit", StepEvaluate: "Needs action", NextGoal: "type hello and press enter"},
	}}
	actor := &fakeCUAActor{outputs: []cuaActorOutput{
		{Actions: []map[string]interface{}{
			{"action": "input_text", "text": "hello"},
			{"action": "Hotkey", "key1": "enter"},
			{"action": "done", "message": "submitted"},
		}},
	}}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	tool.cuaBrain = brain
	tool.cuaActor = actor

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "task",
		"goal":      "submit hello",
		"window_id": "win-1",
		"max_steps": 3,
	})
	if err != nil {
		t.Fatalf("Execute(task cua) error = %v", err)
	}
	if !strings.Contains(asString(raw), "\"status\":\"completed\"") {
		t.Fatalf("task result = %s, want completed", asString(raw))
	}
	if backend.lastFocusedTypeValue != "hello" {
		t.Fatalf("lastFocusedTypeValue = %q, want hello", backend.lastFocusedTypeValue)
	}
	if got := strings.Join(backend.lastKeys, "+"); got != "enter" {
		t.Fatalf("lastKeys = %q, want enter", got)
	}
}

func TestA11yToolExecute_TaskDefaultExecutesDrag(t *testing.T) {
	backend := &a11yCompatBackend{}
	brain := &fakeCUABrain{outputs: []cuaBrainOutput{{Analysis: "drag", StepEvaluate: "Needs action", NextGoal: "drag item"}}}
	actor := &fakeCUAActor{outputs: []cuaActorOutput{
		{Actions: []map[string]interface{}{
			{"action": "Drag", "start": []interface{}{100, 200}, "end": []interface{}{800, 900}},
			{"action": "done"},
		}},
	}}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	tool.cuaBrain = brain
	tool.cuaActor = actor

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "task",
		"goal":      "drag item",
		"window_id": "win-1",
		"max_steps": 2,
	})
	if err != nil {
		t.Fatalf("Execute(task cua) error = %v", err)
	}
	if !strings.Contains(asString(raw), "\"status\":\"completed\"") {
		t.Fatalf("task result = %s, want completed", asString(raw))
	}
	if backend.lastDragWindow != "win-1" {
		t.Fatalf("lastDragWindow = %q, want win-1", backend.lastDragWindow)
	}
	if backend.lastDragStart.X != 0.1 || backend.lastDragStart.Y != 0.2 || backend.lastDragEnd.X != 0.8 || backend.lastDragEnd.Y != 0.9 {
		t.Fatalf("drag = %#v -> %#v, want 0.1,0.2 -> 0.8,0.9", backend.lastDragStart, backend.lastDragEnd)
	}
}

func TestA11yToolExecute_TaskDefaultCapturesObservationBeforeBrain(t *testing.T) {
	screenshotPath := writeCUATestPNG(t, 320, 240)
	backend := &a11yCompatBackend{
		screenshotImagePath: screenshotPath,
		structuredSnapshot: a11yruntime.BuildStructuredSnapshot(a11yruntime.BuildStructuredSnapshotOptions{WindowID: "win-1", Title: "Feishu", Mode: "ax"}, &a11yruntime.Node{
			Role: "window",
			Name: "Feishu",
			Children: []*a11yruntime.Node{
				{Token: "composer", Role: "text_field", Name: "Message", Interactive: true, ValueSettable: true, Bounds: a11yruntime.NormalizedRect{X: 0.1, Y: 0.8, Width: 0.7, Height: 0.1}},
				{Token: "send", Role: "button", Name: "Send", Interactive: true, Actions: []string{"AXPress"}, Bounds: a11yruntime.NormalizedRect{X: 0.9, Y: 0.84, Width: 0.05, Height: 0.05}},
			},
		}),
	}
	brain := &fakeCUABrain{outputs: []cuaBrainOutput{{Analysis: "observed", StepEvaluate: "Success", Done: true}}}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	tool.cuaBrain = brain

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "task",
		"goal":      "inspect Feishu",
		"window_id": "win-1",
		"max_steps": 1,
	})
	if err != nil {
		t.Fatalf("Execute(task cua) error = %v", err)
	}
	if !strings.Contains(asString(raw), "\"status\":\"completed\"") {
		t.Fatalf("task result = %s, want completed", asString(raw))
	}
	if backend.interactiveCalls == 0 {
		t.Fatal("SnapshotInteractive was not called before brain evaluation")
	}
	if backend.lastScreenshotWindow != "win-1" {
		t.Fatalf("lastScreenshotWindow = %q, want win-1", backend.lastScreenshotWindow)
	}
	if len(brain.inputs) != 1 {
		t.Fatalf("brain inputs = %d, want 1", len(brain.inputs))
	}
	observation := brain.inputs[0].Observation
	if got := observation["screenshot_path"]; got != screenshotPath {
		t.Fatalf("observation screenshot_path = %#v, want %s", got, screenshotPath)
	}
	size, ok := observation["screenshot_size"].(map[string]interface{})
	if !ok {
		t.Fatalf("observation screenshot_size = %#v, want object", observation["screenshot_size"])
	}
	if size["width"] != 320 || size["height"] != 240 {
		t.Fatalf("observation screenshot_size = %#v, want width=320 height=240", size)
	}
	modelText, _ := observation["model_text"].(string)
	if !strings.Contains(modelText, "Feishu") || !strings.Contains(modelText, "Message") || !strings.Contains(modelText, screenshotPath) || !strings.Contains(modelText, "Screenshot size: 320x240") {
		t.Fatalf("observation model_text = %q, want title, composer, screenshot path and size", modelText)
	}
}

func TestA11yToolExecute_TaskDefaultUsesLLMBridgeForBrainAndActor(t *testing.T) {
	backend := &a11yCompatBackend{}
	bridge := &mockLLMBridge{responses: []string{
		`{"analysis":"need click","step_evaluate":"Needs action","next_goal":"click center","done":false}`,
		`{"actions":[{"action":"Click","position":[500,500]}]}`,
		`{"analysis":"finished","step_evaluate":"Success","done":true}`,
	}}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	tool.SetLLMBridge(bridge)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "task",
		"goal":      "click center",
		"window_id": "win-1",
		"max_steps": 3,
	})
	if err != nil {
		t.Fatalf("Execute(task cua) error = %v", err)
	}
	if !strings.Contains(asString(raw), "\"status\":\"completed\"") {
		t.Fatalf("task result = %s, want completed", asString(raw))
	}
	if backend.lastPointClick.X != 0.5 || backend.lastPointClick.Y != 0.5 {
		t.Fatalf("lastPointClick = %#v, want 0.5,0.5", backend.lastPointClick)
	}
	if len(bridge.calls) != 3 {
		t.Fatalf("bridge calls = %d, want 3", len(bridge.calls))
	}
	if !strings.Contains(bridge.calls[0], "SYSTEM PROMPT FOR BRAIN MODEL") {
		t.Fatalf("brain prompt = %q, want CUA brain prompt", bridge.calls[0])
	}
	if !strings.Contains(bridge.calls[1], "SYSTEM PROMPT FOR ACTION MODEL") {
		t.Fatalf("actor prompt = %q, want CUA actor prompt", bridge.calls[1])
	}
}

func TestDecodeCUAJSONAcceptsFencedModelOutput(t *testing.T) {
	var output cuaActorOutput
	err := decodeCUAJSON("```json\n{\"actions\":[{\"action\":\"Click\",\"position\":[500,500]}]}\n```", &output)
	if err != nil {
		t.Fatalf("decodeCUAJSON() error = %v", err)
	}
	if len(output.Actions) != 1 || output.Actions[0]["action"] != "Click" {
		t.Fatalf("output = %#v, want Click action", output)
	}
}

type fakeCUABrain struct {
	outputs []cuaBrainOutput
	inputs  []cuaBrainInput
}

func (f *fakeCUABrain) Evaluate(_ context.Context, input cuaBrainInput) (cuaBrainOutput, error) {
	f.inputs = append(f.inputs, input)
	if len(f.outputs) == 0 {
		return cuaBrainOutput{StepEvaluate: "Success", Done: true}, nil
	}
	out := f.outputs[0]
	f.outputs = f.outputs[1:]
	return out, nil
}

type fakeCUAActor struct {
	outputs []cuaActorOutput
	inputs  []cuaActorInput
}

func (f *fakeCUAActor) Act(_ context.Context, input cuaActorInput) (cuaActorOutput, error) {
	f.inputs = append(f.inputs, input)
	if len(f.outputs) == 0 {
		return cuaActorOutput{}, nil
	}
	out := f.outputs[0]
	f.outputs = f.outputs[1:]
	return out, nil
}
