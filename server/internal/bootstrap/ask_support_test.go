package bootstrap

import (
	"context"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill/builtin"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestWireAskSupportRegistersAskToolAndSkillBackend(t *testing.T) {
	broker := sse.NewBroker()
	defer broker.Close()

	toolRegistry := tools.NewRegistry()
	skillRegistry := skill.NewRegistry()
	askSkill := builtin.NewAsk()
	if err := skillRegistry.Register(askSkill, true); err != nil {
		t.Fatalf("register ask skill: %v", err)
	}

	questionMgr := wireAskSupport(toolRegistry, skillRegistry, broker, time.Second)
	if questionMgr == nil {
		t.Fatal("expected question manager")
	}

	askTool := toolRegistry.Get("ask")
	if askTool == nil {
		t.Fatal("expected ask tool to be registered")
	}
	if _, ok := askTool.(*tools.AskTool); !ok {
		t.Fatalf("expected ask tool type, got %T", askTool)
	}

	result, err := askSkill.Execute(context.Background(), map[string]any{
		"q": "Choose a style",
		"a": []any{"Minimal", "Rich"},
	})
	if err != nil {
		t.Fatalf("execute ask skill: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success result, got error=%q", result.Error)
	}

	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected result data type %T", result.Data)
	}
	if silent, _ := data["silent"].(bool); !silent {
		t.Fatalf("expected unattended ask execution to fall back to silent defaults, got %#v", data["silent"])
	}
}

func TestWireAskSupportWithoutBrokerDoesNothing(t *testing.T) {
	toolRegistry := tools.NewRegistry()
	skillRegistry := skill.NewRegistry()

	questionMgr := wireAskSupport(toolRegistry, skillRegistry, nil, time.Second)
	if questionMgr != nil {
		t.Fatal("expected nil question manager without broker")
	}
	if toolRegistry.Get("ask") != nil {
		t.Fatal("expected ask tool not to be registered without broker")
	}
}
