package tools

import (
	"context"
	"strings"
	"testing"
)

func TestExecutorDoesNotInferSessionActionFromCommand(t *testing.T) {
	registry := NewRegistry()
	registry.Register(NewExecTool(DefaultExecConfig(), NewSessionRegistry(), nil, nil, nil))
	executor := NewExecutor(registry)
	for _, command := range []string{
		`printf 'kill ANSWER=70581'`,
		`printf 'poll log list'`,
		`printf 'blue web_query query="Vyhledej nejnovejsi dokumentaci k OpenAI Responses API."'`,
	} {
		result, err := executor.Execute(context.Background(), "exec", map[string]interface{}{"command": command})
		if err != nil {
			t.Errorf("ordinary command became a session action: %s: %v", command, err)
			continue
		}
		if !strings.Contains(result.(string), "stdout") {
			t.Errorf("command did not execute: %v", result)
		}
	}
	if _, err := executor.Execute(context.Background(), "exec", map[string]interface{}{"action": "kill"}); err == nil || !strings.Contains(err.Error(), "session_id is required") {
		t.Fatalf("explicit session action must still validate its session: %v", err)
	}
}
