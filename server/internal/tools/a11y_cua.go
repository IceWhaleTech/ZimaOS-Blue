package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/png"
	"os"
	"strings"
	"time"

	a11yruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/a11y"
)

type cuaBrain interface {
	Evaluate(ctx context.Context, input cuaBrainInput) (cuaBrainOutput, error)
}

type cuaActor interface {
	Act(ctx context.Context, input cuaActorInput) (cuaActorOutput, error)
}

type cuaBrainInput struct {
	Goal        string                     `json:"goal"`
	Step        int                        `json:"step"`
	MaxSteps    int                        `json:"max_steps"`
	Observation map[string]interface{}     `json:"observation,omitempty"`
	Memory      map[string]string          `json:"memory,omitempty"`
	LastResults []a11yruntime.ActionResult `json:"last_results,omitempty"`
	History     []cuaStepTrace             `json:"history,omitempty"`
}

type cuaBrainOutput struct {
	Analysis     string          `json:"analysis,omitempty"`
	StepEvaluate string          `json:"step_evaluate,omitempty"`
	AskHuman     bool            `json:"ask_human,omitempty"`
	NextGoal     string          `json:"next_goal,omitempty"`
	RecordInfo   []cuaRecordInfo `json:"record_info,omitempty"`
	Done         bool            `json:"done,omitempty"`
}

type cuaActorInput struct {
	Goal        string                 `json:"goal"`
	NextGoal    string                 `json:"next_goal"`
	Step        int                    `json:"step"`
	Observation map[string]interface{} `json:"observation,omitempty"`
	Memory      map[string]string      `json:"memory,omitempty"`
	Brain       cuaBrainOutput         `json:"brain,omitempty"`
}

type cuaActorOutput struct {
	Actions []map[string]interface{} `json:"actions,omitempty"`
}

type cuaRecordInfo struct {
	FileName string `json:"file_name,omitempty"`
	Text     string `json:"text,omitempty"`
}

type cuaStepTrace struct {
	Step        int                        `json:"step"`
	Observation map[string]interface{}     `json:"observation,omitempty"`
	Brain       cuaBrainOutput             `json:"brain,omitempty"`
	Actor       cuaActorOutput             `json:"actor,omitempty"`
	Results     []a11yruntime.ActionResult `json:"results,omitempty"`
	Error       string                     `json:"error,omitempty"`
}

type nativeCUABrain struct{}

type nativeCUAActor struct{}

type llmCUABrain struct {
	bridge LLMBridge
}

type llmCUAActor struct {
	bridge LLMBridge
}

func (nativeCUABrain) Evaluate(_ context.Context, input cuaBrainInput) (cuaBrainOutput, error) {
	return cuaBrainOutput{
		Analysis:     "CUA native Blue loop initialized from compact desktop observation.",
		StepEvaluate: "Success",
		NextGoal:     input.Goal,
		Done:         true,
	}, nil
}

func (nativeCUAActor) Act(context.Context, cuaActorInput) (cuaActorOutput, error) {
	return cuaActorOutput{Actions: []map[string]interface{}{}}, nil
}

func (b llmCUABrain) Evaluate(ctx context.Context, input cuaBrainInput) (cuaBrainOutput, error) {
	if b.bridge == nil {
		return nativeCUABrain{}.Evaluate(ctx, input)
	}
	text, err := b.bridge.Chat(ctx, cuaBrainPrompt(input), 2200)
	if err != nil {
		return cuaBrainOutput{}, err
	}
	var output cuaBrainOutput
	if err := decodeCUAJSON(text, &output); err != nil {
		return cuaBrainOutput{}, err
	}
	return output, nil
}

func (a llmCUAActor) Act(ctx context.Context, input cuaActorInput) (cuaActorOutput, error) {
	if a.bridge == nil {
		return nativeCUAActor{}.Act(ctx, input)
	}
	text, err := a.bridge.Chat(ctx, cuaActorPrompt(input), 2200)
	if err != nil {
		return cuaActorOutput{}, err
	}
	var output cuaActorOutput
	if err := decodeCUAJSON(text, &output); err != nil {
		return cuaActorOutput{}, err
	}
	return output, nil
}

func (t *A11yTool) runNativeCUATask(ctx context.Context, backend a11yruntime.Backend, args map[string]interface{}, initialMemories map[string]string) string {
	goal := firstCompatString(args, "goal", "task")
	if goal == "" {
		goal = "task"
	}
	maxSteps, ok := firstCompatIntDeep(args, "max_steps", "maxSteps")
	if !ok || maxSteps <= 0 {
		maxSteps = 8
	}
	windowID := firstCompatString(args, "window_id", "windowId", "target_id", "targetId")
	brain := t.resolveCUABrain()
	actor := t.resolveCUAActor()
	memories := mergeStringMaps(t.taskMemorySnapshot(), initialMemories)
	planner := map[string]interface{}{
		"role":         "planner",
		"current_goal": goal,
		"max_steps":    maxSteps,
	}
	steps := make([]cuaStepTrace, 0, maxSteps)
	lastResults := []a11yruntime.ActionResult(nil)
	var finalBrain cuaBrainOutput
	var finalActor cuaActorOutput
	var finalObservation map[string]interface{}
	for step := 0; step < maxSteps; step++ {
		observation := t.captureCUAObservation(ctx, backend, windowID, lastResults)
		finalObservation = observation
		if blocked, message := cuaObservationBlocked(observation); blocked {
			finalBrain = cuaBrainOutput{
				Analysis:     message,
				StepEvaluate: "Failed",
				AskHuman:     true,
				NextGoal:     message,
			}
			trace := cuaStepTrace{Step: step + 1, Observation: observation, Brain: finalBrain, Error: message}
			steps = append(steps, trace)
			return t.cuaErrorPayload("permission_required", goal, planner, steps, finalBrain, finalActor, finalObservation, memories, fmt.Errorf("%s", message))
		}
		brainOut, err := brain.Evaluate(ctx, cuaBrainInput{
			Goal:        goal,
			Step:        step + 1,
			MaxSteps:    maxSteps,
			Observation: observation,
			Memory:      cloneStringMapPlain(memories),
			LastResults: append([]a11yruntime.ActionResult(nil), lastResults...),
			History:     append([]cuaStepTrace(nil), steps...),
		})
		if err != nil {
			return t.cuaErrorPayload("brain_error", goal, planner, steps, finalBrain, finalActor, finalObservation, memories, err)
		}
		finalBrain = brainOut
		t.recordCUABrainMemory(brainOut)
		memories = t.taskMemorySnapshot()
		trace := cuaStepTrace{Step: step + 1, Observation: observation, Brain: brainOut}
		if brainOut.Done {
			steps = append(steps, trace)
			return t.cuaCompletionPayload("completed", goal, planner, steps, finalBrain, finalActor, finalObservation, memories, "CUA task runner completed")
		}
		actorOut, err := actor.Act(ctx, cuaActorInput{
			Goal:        goal,
			NextGoal:    strings.TrimSpace(valueOrDefault(brainOut.NextGoal, goal)),
			Step:        step + 1,
			Observation: observation,
			Memory:      cloneStringMapPlain(memories),
			Brain:       brainOut,
		})
		if err != nil {
			trace.Error = err.Error()
			steps = append(steps, trace)
			return t.cuaErrorPayload("actor_error", goal, planner, steps, finalBrain, finalActor, finalObservation, memories, err)
		}
		finalActor = actorOut
		trace.Actor = actorOut
		results, done, err := t.executeCUAActions(ctx, backend, windowID, actorOut.Actions)
		trace.Results = results
		if err != nil {
			trace.Error = err.Error()
			steps = append(steps, trace)
			return t.cuaErrorPayload("action_error", goal, planner, steps, finalBrain, finalActor, finalObservation, t.taskMemorySnapshot(), err)
		}
		steps = append(steps, trace)
		lastResults = results
		memories = t.taskMemorySnapshot()
		if done {
			finalBrain.Done = true
			if finalBrain.StepEvaluate == "" {
				finalBrain.StepEvaluate = "Success"
			}
			return t.cuaCompletionPayload("completed", goal, planner, steps, finalBrain, finalActor, finalObservation, memories, "CUA task runner completed")
		}
	}
	return a11yJSON(map[string]interface{}{
		"status":      "stopped",
		"error_code":  "max_steps",
		"mode":        "cua",
		"goal":        goal,
		"planner":     planner,
		"brain":       finalBrain,
		"actor":       finalActor,
		"memory":      map[string]interface{}{"files": memories},
		"observation": finalObservation,
		"steps":       steps,
		"step_count":  len(steps),
		"message":     "CUA task runner stopped at max_steps",
	})
}

func (t *A11yTool) resolveCUABrain() cuaBrain {
	if t != nil && t.cuaBrain != nil {
		return t.cuaBrain
	}
	if t != nil && t.cuaLLM != nil {
		return llmCUABrain{bridge: t.cuaLLM}
	}
	return nativeCUABrain{}
}

func (t *A11yTool) resolveCUAActor() cuaActor {
	if t != nil && t.cuaActor != nil {
		return t.cuaActor
	}
	if t != nil && t.cuaLLM != nil {
		return llmCUAActor{bridge: t.cuaLLM}
	}
	return nativeCUAActor{}
}

func cuaBrainPrompt(input cuaBrainInput) string {
	payload, _ := json.MarshalIndent(input, "", "  ")
	return strings.TrimSpace(`SYSTEM PROMPT FOR BRAIN MODEL:
You are the CUA brain model for a Go-native computer-use agent.
Return valid JSON only with these fields:
{"analysis":"...","step_evaluate":"Success|Needs action|Failed","ask_human":false,"next_goal":"...","record_info":[{"file_name":"note.txt","text":"..."}],"done":false}
Rules:
- Evaluate the current observation, previous results, and memory.
- If the task is complete, set done=true and step_evaluate="Success".
- If more desktop actions are required, set next_goal to the immediate next goal for the actor.
- Do not include markdown.

INPUT:
`) + "\n" + string(payload)
}

func cuaActorPrompt(input cuaActorInput) string {
	payload, _ := json.MarshalIndent(input, "", "  ")
	return strings.TrimSpace(`SYSTEM PROMPT FOR ACTION MODEL:
You are the CUA actor model for a Go-native macOS computer-use agent.
Return valid JSON only with this shape:
{"actions":[{"action":"Click","position":[500,500]}]}
Allowed actions: open_app, input_text, Click, RightSingle, Drag, move_mouse, scroll_up, scroll_down, Hotkey, multi_Hotkey, wait, record_info, done.
Coordinates use normalized 0-1000 positions. Keep actions minimal and safe.
Do not include markdown.

INPUT:
`) + "\n" + string(payload)
}

func decodeCUAJSON(text string, target interface{}) error {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return fmt.Errorf("empty CUA JSON response")
	}
	if strings.HasPrefix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```")
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "json"))
		if idx := strings.LastIndex(trimmed, "```"); idx >= 0 {
			trimmed = strings.TrimSpace(trimmed[:idx])
		}
	}
	if start := strings.Index(trimmed, "{"); start >= 0 {
		trimmed = trimmed[start:]
	}
	decoder := json.NewDecoder(strings.NewReader(trimmed))
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode CUA JSON: %w", err)
	}
	return nil
}

func (t *A11yTool) captureCUAObservation(ctx context.Context, backend a11yruntime.Backend, windowID string, lastResults []a11yruntime.ActionResult) map[string]interface{} {
	observation := map[string]interface{}{
		"mode":              "cua",
		"coordinate_system": "normalized_0_1000",
	}
	if backend == nil {
		observation["last_error"] = "host computer-use backend not available"
		return observation
	}
	var captureErrors []string
	if capabilities, err := backend.Capabilities(ctx); err == nil {
		if strings.TrimSpace(capabilities.HostOS) != "" {
			observation["host_os"] = strings.TrimSpace(capabilities.HostOS)
		}
		if len(capabilities.Permissions) > 0 {
			observation["permissions"] = capabilities.Permissions
		}
		if blocked, message := cuaObservationPermissionBlock(capabilities.Permissions); blocked {
			observation["permission_blocked"] = true
			observation["last_error"] = message
			observation["model_text"] = message
			return observation
		}
	} else {
		captureErrors = append(captureErrors, "capabilities: "+err.Error())
	}
	var lastAction a11yruntime.ActionResult
	if len(lastResults) > 0 {
		lastAction = lastResults[len(lastResults)-1]
		observation["last_action"] = lastAction
	}
	resolvedWindowID := strings.TrimSpace(windowID)
	if snapshotResult, err := backend.SnapshotInteractive(ctx, resolvedWindowID); err == nil {
		if strings.TrimSpace(snapshotResult.WindowID) != "" {
			resolvedWindowID = strings.TrimSpace(snapshotResult.WindowID)
			observation["window_id"] = resolvedWindowID
		}
		if strings.TrimSpace(snapshotResult.Title) != "" {
			observation["title"] = strings.TrimSpace(snapshotResult.Title)
		}
		if strings.TrimSpace(snapshotResult.Tree) != "" {
			observation["snapshot_tree"] = strings.TrimSpace(snapshotResult.Tree)
		}
		if strings.TrimSpace(snapshotResult.ImagePath) != "" {
			observation["screenshot_path"] = strings.TrimSpace(snapshotResult.ImagePath)
		}
	} else {
		captureErrors = append(captureErrors, "snapshot: "+err.Error())
	}
	if screenshotResult, err := backend.Screenshot(ctx, resolvedWindowID); err == nil {
		if strings.TrimSpace(screenshotResult.WindowID) != "" {
			resolvedWindowID = strings.TrimSpace(screenshotResult.WindowID)
			observation["window_id"] = resolvedWindowID
		}
		if strings.TrimSpace(screenshotResult.ImagePath) != "" {
			observation["screenshot_path"] = strings.TrimSpace(screenshotResult.ImagePath)
		}
	} else {
		captureErrors = append(captureErrors, "screenshot: "+err.Error())
	}
	lastError := strings.Join(captureErrors, "; ")
	if lastError != "" {
		observation["last_error"] = lastError
	}
	screenshotPath, _ := observation["screenshot_path"].(string)
	screenshotWidth, screenshotHeight, hasScreenshotSize := cuaScreenshotSize(screenshotPath)
	if hasScreenshotSize {
		observation["screenshot_size"] = map[string]interface{}{
			"width":  screenshotWidth,
			"height": screenshotHeight,
		}
	}
	var compact a11yruntime.CUAObservation
	if provider, ok := backend.(a11yStructuredSnapshotProvider); ok {
		if snapshot, ok := provider.CurrentStructuredSnapshot(resolvedWindowID); ok && snapshot != nil {
			compact = a11yruntime.BuildCUAObservation(a11yruntime.CUAObservationInput{
				Snapshot:         snapshot,
				ScreenshotPath:   screenshotPath,
				LastActionResult: lastAction,
				LastError:        lastError,
				ElementLimit:     48,
			})
		}
	}
	if compact.WindowID != "" {
		observation["window_id"] = compact.WindowID
	}
	if compact.Title != "" {
		observation["title"] = compact.Title
	}
	if len(compact.Elements) > 0 {
		observation["elements"] = compact.Elements
	}
	if modelText := compact.ModelText(); strings.TrimSpace(modelText) != "" {
		if hasScreenshotSize {
			modelText += fmt.Sprintf("\nScreenshot size: %dx%d", screenshotWidth, screenshotHeight)
		}
		observation["model_text"] = modelText
	} else if strings.TrimSpace(screenshotPath) != "" {
		modelText := strings.TrimSpace("Screenshot: " + screenshotPath)
		if hasScreenshotSize {
			modelText += fmt.Sprintf("\nScreenshot size: %dx%d", screenshotWidth, screenshotHeight)
		}
		observation["model_text"] = modelText
	}
	return observation
}

func cuaObservationBlocked(observation map[string]interface{}) (bool, string) {
	if observation == nil {
		return false, ""
	}
	blocked, _ := observation["permission_blocked"].(bool)
	if !blocked {
		return false, ""
	}
	message := strings.TrimSpace(asString(observation["last_error"]))
	if message == "" {
		message = "required desktop permission is not granted"
	}
	return true, message
}

func cuaObservationPermissionBlock(permissions []a11yruntime.PermissionStatus) (bool, string) {
	for _, permission := range permissions {
		if !permission.Required || permission.Granted {
			continue
		}
		name := strings.TrimSpace(permission.Name)
		if name == "" {
			name = "unknown"
		}
		message := strings.TrimSpace(permission.Message)
		if message == "" {
			message = "grant " + name
		}
		return true, "required desktop permission is not granted: " + name + "; " + message
	}
	return false, ""
}

func cuaScreenshotSize(path string) (int, int, bool) {
	if strings.TrimSpace(path) == "" {
		return 0, 0, false
	}
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, false
	}
	defer file.Close()
	config, _, err := image.DecodeConfig(file)
	if err != nil || config.Width <= 0 || config.Height <= 0 {
		return 0, 0, false
	}
	return config.Width, config.Height, true
}

func (t *A11yTool) executeCUAActions(ctx context.Context, backend a11yruntime.Backend, windowID string, actions []map[string]interface{}) ([]a11yruntime.ActionResult, bool, error) {
	results := make([]a11yruntime.ActionResult, 0, len(actions))
	for _, actionArgs := range actions {
		result, done, err := t.executeCUAAction(ctx, backend, windowID, actionArgs)
		if err != nil {
			return results, false, err
		}
		if strings.TrimSpace(result.HostOS) == "" && backend != nil {
			result.HostOS = backend.HostOS()
		}
		if strings.TrimSpace(result.WindowID) == "" {
			result.WindowID = windowID
		}
		results = append(results, result)
		if done {
			return results, true, nil
		}
	}
	return results, false, nil
}

func (t *A11yTool) executeCUAAction(ctx context.Context, backend a11yruntime.Backend, windowID string, actionArgs map[string]interface{}) (a11yruntime.ActionResult, bool, error) {
	if actionArgs == nil {
		return a11yruntime.ActionResult{}, false, nil
	}
	action := normalizeA11yAction(firstCompatString(actionArgs, "action", "type", "name"))
	if action == "" {
		return a11yruntime.ActionResult{}, false, fmt.Errorf("CUA action is missing action")
	}
	if overrideWindow := firstCompatString(actionArgs, "window_id", "windowId", "target_id", "targetId"); overrideWindow != "" {
		windowID = overrideWindow
	}
	holdMS, ok := firstCompatIntDeep(actionArgs, "hold_ms", "holdMs")
	if !ok {
		holdMS = a11yruntime.DefaultHoldMS
	}
	holdMS = a11yruntime.NormalizeHoldMS(holdMS)
	switch action {
	case "record_info":
		fileName := strings.TrimSpace(firstCompatString(actionArgs, "file_name", "fileName", "name"))
		text := firstCompatString(actionArgs, "text", "value", "content")
		if fileName == "" {
			fileName = "memory.txt"
		}
		t.recordTaskMemory(fileName, text)
		return a11yruntime.ActionResult{HostOS: backend.HostOS(), WindowID: windowID, ExecutionMode: "memory", TargetHit: true, VerificationPassed: true, VerificationMethod: "record_info", Message: "Computer-use memory recorded"}, false, nil
	case "done":
		message := valueOrDefault(firstCompatString(actionArgs, "text", "message"), "Task completed")
		return a11yruntime.ActionResult{HostOS: backend.HostOS(), WindowID: windowID, ExecutionMode: "semantic", TargetHit: true, VerificationPassed: true, VerificationMethod: "done", Message: message}, true, nil
	case "wait":
		ms, ok := firstCompatIntDeep(actionArgs, "ms", "milliseconds")
		if !ok {
			if seconds, secondsOK := firstCompatIntDeep(actionArgs, "seconds", "sec"); secondsOK {
				ms = seconds * 1000
			}
		}
		if ms > 0 {
			select {
			case <-ctx.Done():
				return a11yruntime.ActionResult{}, false, ctx.Err()
			case <-time.After(time.Duration(ms) * time.Millisecond):
			}
		}
		return a11yruntime.ActionResult{HostOS: backend.HostOS(), WindowID: windowID, ExecutionMode: "wait", TargetHit: true, VerificationPassed: true, VerificationMethod: "wait", Message: "Wait completed"}, false, nil
	case "click":
		if point, ok := coerceA11yNormalizedPoint(firstCompatRawValue(actionArgs, "position")); ok {
			result, err := backend.ClickWindowPoint(ctx, windowID, point, holdMS)
			return result, false, err
		}
		ref, ok := firstCompatIntDeep(actionArgs, "ref")
		if !ok {
			return a11yruntime.ActionResult{}, false, fmt.Errorf("CUA click requires position or ref")
		}
		result, err := backend.Act(ctx, windowID, ref, nil, "click", "", holdMS)
		return result, false, err
	case "drag":
		start, startOK := coerceA11yNormalizedPoint(firstCompatRawValue(actionArgs, "start", "from"))
		end, endOK := coerceA11yNormalizedPoint(firstCompatRawValue(actionArgs, "end", "to"))
		if !startOK || !endOK {
			return a11yruntime.ActionResult{}, false, fmt.Errorf("CUA drag requires start and end positions")
		}
		dragger, ok := backend.(hostWindowPointDragger)
		if !ok {
			return a11yruntime.ActionResult{}, false, fmt.Errorf("CUA drag is unavailable on this backend")
		}
		result, err := dragger.DragWindowPoint(ctx, windowID, start, end, holdMS)
		return result, false, err
	case "type":
		value := firstCompatString(actionArgs, "value", "text", "content")
		if typer, ok := backend.(hostFocusedTextTyper); ok {
			result, err := typer.TypeFocusedText(ctx, windowID, value, holdMS)
			return result, false, err
		}
		result, err := backend.Act(ctx, windowID, 0, nil, "type", value, holdMS)
		return result, false, err
	case a11yruntime.ActionKey:
		keys, ok := resolveA11yKeySequenceArgs(actionArgs)
		if !ok {
			keys = cuaHotkeyArgs(actionArgs)
		}
		if len(keys) == 0 {
			return a11yruntime.ActionResult{}, false, fmt.Errorf("CUA hotkey requires keys")
		}
		result, err := backend.Key(ctx, windowID, keys, holdMS)
		return result, false, err
	case a11yruntime.ActionScroll:
		direction := firstCompatString(actionArgs, "direction")
		if direction == "" {
			direction = "down"
		}
		lines, ok := firstCompatIntDeep(actionArgs, "lines", "dy")
		if !ok || lines <= 0 {
			lines = 5
		}
		result, err := backend.Scroll(ctx, windowID, direction, lines)
		return result, false, err
	case a11yruntime.ActionPointerMove:
		point, ok := coerceA11yNormalizedPoint(firstCompatRawValue(actionArgs, "position"))
		if ok {
			result, err := backend.PointerMove(ctx, int(point.X*1000), int(point.Y*1000))
			return result, false, err
		}
		x, xOK := firstCompatIntDeep(actionArgs, "x")
		y, yOK := firstCompatIntDeep(actionArgs, "y")
		if !xOK || !yOK {
			return a11yruntime.ActionResult{}, false, fmt.Errorf("CUA move_mouse requires position or x/y")
		}
		result, err := backend.PointerMove(ctx, x, y)
		return result, false, err
	case a11yruntime.ActionFocus:
		appName := firstCompatString(actionArgs, "app_name", "appName", "name")
		if appName != "" {
			if activator, ok := backend.(hostAppActivator); ok {
				result, err := activator.ActivateApp(ctx, appName)
				return result, false, err
			}
		}
		result, err := backend.FocusWindow(ctx, windowID)
		return result, false, err
	default:
		return a11yruntime.ActionResult{}, false, fmt.Errorf("unsupported CUA action %q", action)
	}
}

func (t *A11yTool) recordTaskMemory(fileName string, text string) {
	if t == nil {
		return
	}
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		fileName = "memory.txt"
	}
	t.mu.Lock()
	if t.taskMemory == nil {
		t.taskMemory = make(map[string]string)
	}
	t.taskMemory[fileName] = text
	t.mu.Unlock()
}

func (t *A11yTool) recordCUABrainMemory(output cuaBrainOutput) {
	for _, info := range output.RecordInfo {
		if strings.TrimSpace(info.Text) == "" {
			continue
		}
		t.recordTaskMemory(info.FileName, info.Text)
	}
}

func (t *A11yTool) taskMemorySnapshot() map[string]string {
	out := map[string]string{}
	if t == nil {
		return out
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	for key, value := range t.taskMemory {
		out[key] = value
	}
	return out
}

func (t *A11yTool) cuaCompletionPayload(status string, goal string, planner map[string]interface{}, steps []cuaStepTrace, brain cuaBrainOutput, actor cuaActorOutput, observation map[string]interface{}, memories map[string]string, message string) string {
	return a11yJSON(map[string]interface{}{
		"status":      status,
		"mode":        "cua",
		"goal":        goal,
		"planner":     planner,
		"brain":       brain,
		"actor":       actor,
		"memory":      map[string]interface{}{"files": memories},
		"observation": observation,
		"steps":       steps,
		"step_count":  len(steps),
		"message":     message,
	})
}

func (t *A11yTool) cuaErrorPayload(code string, goal string, planner map[string]interface{}, steps []cuaStepTrace, brain cuaBrainOutput, actor cuaActorOutput, observation map[string]interface{}, memories map[string]string, err error) string {
	message := "CUA task runner failed"
	if err != nil {
		message = err.Error()
	}
	return a11yJSON(map[string]interface{}{
		"status":      "blocked",
		"error_code":  code,
		"error":       message,
		"mode":        "cua",
		"goal":        goal,
		"planner":     planner,
		"brain":       brain,
		"actor":       actor,
		"memory":      map[string]interface{}{"files": memories},
		"observation": observation,
		"steps":       steps,
		"step_count":  len(steps),
	})
}

func mergeStringMaps(base map[string]string, overlay map[string]string) map[string]string {
	out := cloneStringMapPlain(base)
	for key, value := range overlay {
		out[key] = value
	}
	return out
}

func cloneStringMapPlain(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
