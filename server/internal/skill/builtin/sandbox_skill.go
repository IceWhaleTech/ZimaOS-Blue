package builtin

import (
	"context"
	"fmt"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
)

// SandboxServiceInterface defines the interface for sandbox service used by the skill.
type SandboxServiceInterface interface {
	Execute(ctx context.Context, command string, args []string, stdin string, timeoutSecs int) (SandboxResultInfo, error)
	GetStatus(id string) (SandboxResultInfo, error)
	Kill(id string) error
	IsSupported() bool
}

// SandboxResultInfo represents a sandbox execution result.
type SandboxResultInfo struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	Duration string `json:"duration"`
	Error    string `json:"error,omitempty"`
}

// Sandbox is a built-in skill for sandboxed code execution.
type Sandbox struct {
	manifest *skill.Manifest
	mu       sync.RWMutex
	svc      SandboxServiceInterface
}

// NewSandbox creates a new sandbox skill.
func NewSandbox() *Sandbox {
	return &Sandbox{
		manifest: &skill.Manifest{
			ID:          "sandbox",
			Name:        "Sandbox",
			Version:     "1.0.0",
			Description: "Execute commands in a sandboxed environment with resource limits (memory, CPU, timeout). Supports running shell commands, scripts, and code safely with isolation.",
			Category:    "system",
			Icon:        "sandbox",
			Tags:        []string{"sandbox", "execute", "shell", "code", "isolation", "safe"},
			Inputs: []skill.Parameter{
				{
					Name:        "action",
					Type:        "string",
					Description: "Action: execute, status, kill, info",
					Required:    true,
				},
				{
					Name:        "command",
					Type:        "string",
					Description: "Command to execute (required for execute)",
					Required:    false,
				},
				{
					Name:        "args",
					Type:        "array",
					Description: "Command arguments (optional for execute)",
					Required:    false,
				},
				{
					Name:        "stdin",
					Type:        "string",
					Description: "Standard input to pass to the command (optional for execute)",
					Required:    false,
				},
				{
					Name:        "timeout",
					Type:        "number",
					Description: "Timeout in seconds (default: 30, max: 300)",
					Required:    false,
				},
				{
					Name:        "id",
					Type:        "string",
					Description: "Execution ID (required for status, kill)",
					Required:    false,
				},
				{
					Name:        "locale",
					Type:        "string",
					Description: "Language/locale code for localized responses (e.g., en-US, zh-CN)",
					Required:    false,
				},
			},
			Outputs: []skill.Parameter{
				{
					Name:        "result",
					Type:        "object",
					Description: "Execution result with stdout, stderr, exit code",
				},
			},
		},
	}
}

// SetSandboxService injects the sandbox service.
func (s *Sandbox) SetSandboxService(svc SandboxServiceInterface) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.svc = svc
}

func (s *Sandbox) Manifest() *skill.Manifest { return s.manifest }

func (s *Sandbox) Validate(input map[string]any) error {
	action, ok := input["action"]
	if !ok {
		return fmt.Errorf("action is required")
	}
	actionStr, ok := action.(string)
	if !ok {
		return fmt.Errorf("action must be a string")
	}
	valid := map[string]bool{"execute": true, "status": true, "kill": true, "info": true}
	if !valid[actionStr] {
		return fmt.Errorf("invalid action: %s", actionStr)
	}
	switch actionStr {
	case "execute":
		if _, ok := input["command"]; !ok {
			return fmt.Errorf("command is required for execute")
		}
	case "status", "kill":
		if _, ok := input["id"]; !ok {
			return fmt.Errorf("id is required for %s", actionStr)
		}
	}
	return nil
}

func (s *Sandbox) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	s.mu.RLock()
	svc := s.svc
	s.mu.RUnlock()

	if svc == nil {
		return skill.NewErrorResult(fmt.Errorf("sandbox service not available")), nil
	}

	action := input["action"].(string)

	switch action {
	case "execute":
		command := input["command"].(string)

		var args []string
		switch a := input["args"].(type) {
		case []string:
			args = a
		case []interface{}:
			for _, v := range a {
				if str, ok := v.(string); ok {
					args = append(args, str)
				}
			}
		}

		stdin, _ := input["stdin"].(string)

		timeoutSecs := 30
		if t, ok := input["timeout"].(float64); ok && t > 0 {
			timeoutSecs = int(t)
			if timeoutSecs > 300 {
				timeoutSecs = 300
			}
		}

		result, err := svc.Execute(ctx, command, args, stdin, timeoutSecs)
		if err != nil {
			return skill.NewErrorResult(fmt.Errorf("execution failed: %w", err)), nil
		}

		return skill.NewResult(map[string]any{
			"result":  result,
			"message": fmt.Sprintf("Command '%s' completed (exit: %d, duration: %s)", command, result.ExitCode, result.Duration),
		}), nil

	case "status":
		id := input["id"].(string)
		result, err := svc.GetStatus(id)
		if err != nil {
			return skill.NewErrorResult(err), nil
		}
		return skill.NewResult(map[string]any{"result": result}), nil

	case "kill":
		id := input["id"].(string)
		if err := svc.Kill(id); err != nil {
			return skill.NewErrorResult(err), nil
		}
		return skill.NewResult(map[string]any{"id": id, "killed": true, "message": fmt.Sprintf("Execution %s killed", id)}), nil

	case "info":
		supported := svc.IsSupported()
		return skill.NewResult(map[string]any{
			"supported": supported,
			"message": func() string {
				if supported {
					return "Sandbox is supported on this platform"
				}
				return "Sandbox is not supported on this platform"
			}(),
		}), nil
	}

	return skill.NewErrorResult(fmt.Errorf("unknown action: %s", action)), nil
}
