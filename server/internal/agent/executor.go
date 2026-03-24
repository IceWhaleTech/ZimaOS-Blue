package agent

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

const (
	groundedExitSuccess       = 0
	groundedExitToolFailure   = 1
	groundedExitPlannerReject = 2
	groundedExitInternal      = 70
)

type GroundedToolCall struct {
	ToolCallID   string         `json:"tool_call_id"`
	TaskID       string         `json:"task_id"`
	StepIndex    int            `json:"step_index"`
	PlannerRound int            `json:"planner_round"`
	Tool         string         `json:"tool"`
	Args         map[string]any `json:"args"`
	CreatedAt    time.Time      `json:"created_at"`
}

type GroundedToolResult struct {
	ToolCallID string    `json:"tool_call_id"`
	Tool       string    `json:"tool"`
	ExitCode   int       `json:"exit_code"`
	OK         bool      `json:"ok"`
	Result     any       `json:"result,omitempty"`
	Stderr     string    `json:"stderr,omitempty"`
	AuthTag    string    `json:"auth_tag"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
}

type GroundedExecution struct {
	Call    GroundedToolCall   `json:"call"`
	Result  GroundedToolResult `json:"result"`
	Updates []StateUpdate      `json:"updates,omitempty"`
}

type GroundedExecutor struct {
	executor    *tools.Executor
	toolGateway *tools.ToolGateway
	registry    *tools.Registry
	stateStore  *GroundTruthStateStore
	secret      []byte
	askHandler  func(ctx context.Context, task *Task, argsJSON string) string
}

func NewGroundedExecutor(executor *tools.Executor, registry *tools.Registry, stateStore *GroundTruthStateStore, secret []byte, askHandler func(ctx context.Context, task *Task, argsJSON string) string) *GroundedExecutor {
	if len(secret) == 0 {
		secret = make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			secret = []byte("zimaos-grounded-runtime-fallback-secret")
		}
	}
	return &GroundedExecutor{
		executor:   executor,
		registry:   registry,
		stateStore: stateStore,
		secret:     append([]byte(nil), secret...),
		askHandler: askHandler,
	}
}

func (e *GroundedExecutor) SetToolGateway(gateway *tools.ToolGateway) {
	if e == nil {
		return
	}
	e.toolGateway = gateway
}

func (e *GroundedExecutor) Execute(ctx context.Context, task *Task, stepIndex, plannerRound int, nextTool PlannerToolCall) (*GroundedExecution, error) {
	if task == nil {
		return nil, errors.New("task is required")
	}
	if e == nil || e.stateStore == nil {
		return nil, errors.New("grounded executor is not configured")
	}
	if task.GroundState == nil {
		task.GroundState = NewGroundTruthState()
	}

	call := GroundedToolCall{
		ToolCallID:   fmt.Sprintf("task/%s/tc/%d", task.ID, len(task.GroundState.Calls)+1),
		TaskID:       task.ID,
		StepIndex:    stepIndex,
		PlannerRound: plannerRound,
		Tool:         normalizeGroundToolName(nextTool.Tool),
		Args:         normalizeGroundToolArgs(normalizeGroundToolName(nextTool.Tool), cloneJSONMap(nextTool.Args)),
		CreatedAt:    time.Now().UTC(),
	}
	startedAt := time.Now().UTC()

	if call.Tool == "" {
		result := e.makeResult(call, groundedExitPlannerReject, false, map[string]any{"error": "planner did not specify a tool"}, "planner did not specify a tool", startedAt, time.Now().UTC())
		return &GroundedExecution{Call: call, Result: result}, fmt.Errorf("planner rejected: missing tool name")
	}

	resolvedTool, err := e.resolveTool(call.Tool)
	if err != nil {
		result := e.makeResult(call, groundedExitPlannerReject, false, map[string]any{"error": err.Error()}, err.Error(), startedAt, time.Now().UTC())
		return &GroundedExecution{Call: call, Result: result}, err
	}

	argsJSON, err := json.Marshal(call.Args)
	if err != nil {
		result := e.makeResult(call, groundedExitInternal, false, map[string]any{"error": err.Error()}, err.Error(), startedAt, time.Now().UTC())
		return &GroundedExecution{Call: call, Result: result}, err
	}

	var rawResult any
	var execErr error
	switch resolvedTool {
	case "ask":
		if e.askHandler == nil {
			execErr = errors.New("ask tool is unavailable in grounded runtime")
			break
		}
		rawResult = e.askHandler(ctx, task, string(argsJSON))
	default:
		if e.toolGateway != nil {
			gatewayResult, gatewayErr := e.toolGateway.Execute(ctx, tools.ToolGatewayRequest{
				ToolCallID: call.ToolCallID,
				ToolName:   resolvedTool,
				Arguments:  string(argsJSON),
				SessionID:  strings.TrimSpace(task.ConversationID),
				RouteKind:  tools.ToolRouteKindAgent,
				UserID:     strings.TrimSpace(task.UserID),
			})
			if gatewayResult != nil {
				if gatewayErr == nil && gatewayResult.ExecutionResult != nil {
					rawResult = gatewayResult.ExecutionResult
				} else if gatewayResult.AuditPayload != nil {
					rawResult = gatewayResult.AuditPayload
				}
			}
			execErr = gatewayErr
			break
		}
		if e.executor == nil {
			execErr = errors.New("tool executor is unavailable")
			break
		}
		rawResult, execErr = e.executor.ExecuteJSON(ctx, resolvedTool, string(argsJSON))
	}

	finishedAt := time.Now().UTC()
	normalized := normalizeToolResultValue(rawResult)
	exitCode := groundedExitSuccess
	ok := execErr == nil
	stderr := ""
	if execErr != nil {
		exitCode = groundedExitToolFailure
		stderr = execErr.Error()
		normalized = map[string]any{"error": execErr.Error()}
	}
	if normalizeGroundToolName(resolvedTool) == "bash" {
		if exit, okExit := int64FromAny(extractField(normalized, "exit_code")); okExit {
			exitCode = int(exit)
		}
	}
	result := e.makeResult(call, exitCode, ok, normalized, stderr, startedAt, finishedAt)
	updates := e.stateStore.DeriveStateUpdates(call, result)
	return &GroundedExecution{
		Call:    call,
		Result:  result,
		Updates: updates,
	}, execErr
}

func (e *GroundedExecutor) resolveTool(name string) (string, error) {
	normalized := normalizeGroundToolName(name)
	switch normalized {
	case "read":
		if e.registry != nil {
			if e.registry.Get("read") != nil {
				return "read", nil
			}
			if e.registry.Get("file_read") != nil {
				return "file_read", nil
			}
		}
		return "read", nil
	case "write":
		if e.registry != nil {
			if e.registry.Get("write") != nil {
				return "write", nil
			}
			if e.registry.Get("file_write") != nil {
				return "file_write", nil
			}
		}
		return "write", nil
	case "bash":
		if e.registry != nil {
			if e.registry.Get("bash") != nil {
				return "bash", nil
			}
			if e.registry.Get("exec") != nil {
				return "exec", nil
			}
		}
		return "bash", nil
	case "ask":
		return "ask", nil
	default:
		if e.registry == nil {
			return "", fmt.Errorf("tool %q is not registered", name)
		}
		if e.registry.Get(normalized) == nil && e.registry.Get(name) == nil {
			return "", fmt.Errorf("tool %q is not registered", name)
		}
		if e.registry.Get(normalized) != nil {
			return normalized, nil
		}
		return name, nil
	}
}

func normalizeGroundToolArgs(tool string, args map[string]any) map[string]any {
	if len(args) == 0 {
		return args
	}
	normalized := cloneJSONMap(args)
	switch normalizeGroundToolName(tool) {
	case "read", "write":
		if strings.TrimSpace(asString(normalized["path"])) == "" {
			for _, key := range []string{"file_path", "filePath", "filename"} {
				if path := strings.TrimSpace(asString(normalized[key])); path != "" {
					normalized["path"] = path
					break
				}
			}
		}
	}
	if normalizeGroundToolName(tool) == "write" {
		if _, ok := normalized["content"]; !ok {
			for _, key := range []string{"text", "body", "value"} {
				if value, ok := normalized[key]; ok {
					normalized["content"] = value
					break
				}
			}
		}
	}
	return normalized
}

func (e *GroundedExecutor) makeResult(call GroundedToolCall, exitCode int, ok bool, normalized any, stderr string, startedAt, finishedAt time.Time) GroundedToolResult {
	result := GroundedToolResult{
		ToolCallID: call.ToolCallID,
		Tool:       call.Tool,
		ExitCode:   exitCode,
		OK:         ok && exitCode == groundedExitSuccess,
		Result:     normalized,
		Stderr:     strings.TrimSpace(stderr),
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
	}
	result.AuthTag = e.signResult(result)
	return result
}

func (e *GroundedExecutor) signResult(result GroundedToolResult) string {
	payload := map[string]any{
		"tool_call_id": result.ToolCallID,
		"tool":         result.Tool,
		"exit_code":    result.ExitCode,
		"ok":           result.OK,
		"result":       result.Result,
		"stderr":       result.Stderr,
		"started_at":   result.StartedAt.UTC().Format(time.RFC3339Nano),
		"finished_at":  result.FinishedAt.UTC().Format(time.RFC3339Nano),
	}
	b, _ := json.Marshal(payload)
	mac := hmac.New(sha256.New, e.secret)
	_, _ = mac.Write(b)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func (e *GroundedExecutor) VerifyResult(result GroundedToolResult) bool {
	if e == nil || len(e.secret) == 0 {
		return false
	}
	return hmac.Equal([]byte(result.AuthTag), []byte(e.signResult(result)))
}

func normalizeToolResultValue(raw any) any {
	switch value := raw.(type) {
	case *tools.ForwardedResult:
		return normalizeToolResultValue(value.Result)
	case string:
		trimmed := trimStructuredContent(value)
		if trimmed == "" {
			return map[string]any{}
		}
		var decoded any
		if err := json.Unmarshal([]byte(trimmed), &decoded); err == nil {
			return decoded
		}
		return map[string]any{"text": value}
	case nil:
		return map[string]any{}
	default:
		b, err := json.Marshal(value)
		if err != nil {
			return tools.SafeToolPayloadValue(value, 64*1024)
		}
		var decoded any
		if err := json.Unmarshal(b, &decoded); err == nil {
			return decoded
		}
		return map[string]any{"text": string(b)}
	}
}

func cloneJSONMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	b, _ := json.Marshal(in)
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	if out == nil {
		out = map[string]any{}
	}
	return out
}
