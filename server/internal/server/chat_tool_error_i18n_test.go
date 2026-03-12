package server

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/i18n"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type workspaceEscapeErrorTool struct{}

func (t *workspaceEscapeErrorTool) Definition() tools.ToolDefinition {
	return tools.ToolDefinition{
		Name:        "workspace_escape_error_tool",
		Description: "returns workspace-boundary error",
	}
}

func (t *workspaceEscapeErrorTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	return nil, errors.New("path escapes workspace root")
}

func TestLocalizeToolExecutionErrorMessage(t *testing.T) {
	tests := []struct {
		name string
		lang i18n.Language
		raw  string
		want string
	}{
		{
			name: "known key translated",
			lang: i18n.LangZhCN,
			raw:  "path escapes workspace root",
			want: i18n.T(i18n.LangZhCN, i18n.MsgPathEscapesWorkspaceRoot),
		},
		{
			name: "unknown error unchanged",
			lang: i18n.LangZhCN,
			raw:  "some custom error",
			want: "some custom error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := localizeToolExecutionErrorMessage(tt.lang, tt.raw)
			if got != tt.want {
				t.Fatalf("localizeToolExecutionErrorMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExecuteToolCalls_LocalizesWorkspaceRootBoundaryError(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(&workspaceEscapeErrorTool{})

	handler := NewChatHandler(nil, llm.NewProviderRegistry(), registry)
	defer handler.Shutdown()

	ctx := tools.WithLang(context.Background(), "ja-JP")
	results := handler.executeToolCalls(ctx, []llm.ToolCall{
		{
			ID:        "tc-workspace-boundary",
			Name:      "workspace_escape_error_tool",
			Arguments: "{}",
		},
	})
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	var payload struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(results[0].Content), &payload); err != nil {
		t.Fatalf("unmarshal tool result: %v", err)
	}
	want := i18n.T(i18n.LangJaJP, i18n.MsgPathEscapesWorkspaceRoot)
	if payload.Error != want {
		t.Fatalf("tool error = %q, want %q", payload.Error, want)
	}
}
