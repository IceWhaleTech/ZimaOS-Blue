package skill

import (
	"context"
	"fmt"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/i18n"
)

// mockSkill is a mock skill for testing
type mockSkill struct {
	manifest *Manifest
	execFunc func(ctx context.Context, input map[string]any) (*Result, error)
}

func (m *mockSkill) Manifest() *Manifest {
	return m.manifest
}

func (m *mockSkill) Validate(input map[string]any) error {
	return nil
}

func (m *mockSkill) Execute(ctx context.Context, input map[string]any) (*Result, error) {
	if m.execFunc != nil {
		return m.execFunc(ctx, input)
	}
	return NewResult("mock result"), nil
}

func newMockSkill(id, name string) *mockSkill {
	return &mockSkill{
		manifest: &Manifest{
			ID:          id,
			Name:        name,
			Version:     "1.0.0",
			Description: "A mock skill for testing",
		},
	}
}

func TestRegistry_Register(t *testing.T) {
	registry := NewRegistry()

	t.Run("register skill", func(t *testing.T) {
		skill := newMockSkill("test-skill", "Test Skill")
		err := registry.Register(skill, false)
		if err != nil {
			t.Fatalf("failed to register skill: %v", err)
		}

		if registry.Count() != 1 {
			t.Errorf("expected 1 skill, got %d", registry.Count())
		}
	})

	t.Run("register duplicate skill", func(t *testing.T) {
		skill := newMockSkill("test-skill", "Test Skill")
		err := registry.Register(skill, false)
		if err == nil {
			t.Error("expected error for duplicate skill")
		}
	})

	t.Run("register skill with empty ID", func(t *testing.T) {
		skill := newMockSkill("", "Empty ID Skill")
		err := registry.Register(skill, false)
		if err == nil {
			t.Error("expected error for empty ID")
		}
	})
}

func TestRegistry_Get(t *testing.T) {
	registry := NewRegistry()
	skill := newMockSkill("test-skill", "Test Skill")
	registry.Register(skill, false)

	t.Run("get existing skill", func(t *testing.T) {
		s := registry.Get("test-skill")
		if s == nil {
			t.Fatal("expected skill, got nil")
		}
		if s.Manifest().Name != "Test Skill" {
			t.Errorf("expected name 'Test Skill', got '%s'", s.Manifest().Name)
		}
	})

	t.Run("get non-existent skill", func(t *testing.T) {
		s := registry.Get("non-existent")
		if s != nil {
			t.Error("expected nil for non-existent skill")
		}
	})
}

func TestRegistry_EnableDisable(t *testing.T) {
	registry := NewRegistry()
	skill := newMockSkill("test-skill", "Test Skill")
	registry.Register(skill, false)

	t.Run("skill enabled by default", func(t *testing.T) {
		if !registry.IsEnabled("test-skill") {
			t.Error("expected skill to be enabled by default")
		}
	})

	t.Run("disable skill", func(t *testing.T) {
		err := registry.Disable("test-skill")
		if err != nil {
			t.Fatalf("failed to disable skill: %v", err)
		}
		if registry.IsEnabled("test-skill") {
			t.Error("expected skill to be disabled")
		}
	})

	t.Run("enable skill", func(t *testing.T) {
		err := registry.Enable("test-skill")
		if err != nil {
			t.Fatalf("failed to enable skill: %v", err)
		}
		if !registry.IsEnabled("test-skill") {
			t.Error("expected skill to be enabled")
		}
	})
}

func TestRegistry_List(t *testing.T) {
	registry := NewRegistry()
	registry.Register(newMockSkill("skill1", "Skill 1"), true)
	registry.Register(newMockSkill("skill2", "Skill 2"), false)
	registry.Register(newMockSkill("skill3", "Skill 3"), true)

	t.Run("list all skills", func(t *testing.T) {
		skills := registry.List()
		if len(skills) != 3 {
			t.Errorf("expected 3 skills, got %d", len(skills))
		}
	})

	t.Run("list enabled skills", func(t *testing.T) {
		registry.Disable("skill2")
		skills := registry.ListEnabled()
		if len(skills) != 2 {
			t.Errorf("expected 2 enabled skills, got %d", len(skills))
		}
	})
}

func TestRegistry_Unregister(t *testing.T) {
	registry := NewRegistry()
	registry.Register(newMockSkill("test-skill", "Test Skill"), false)

	t.Run("unregister existing skill", func(t *testing.T) {
		err := registry.Unregister("test-skill")
		if err != nil {
			t.Fatalf("failed to unregister skill: %v", err)
		}
		if registry.Count() != 0 {
			t.Errorf("expected 0 skills, got %d", registry.Count())
		}
	})

	t.Run("unregister non-existent skill", func(t *testing.T) {
		err := registry.Unregister("non-existent")
		if err == nil {
			t.Error("expected error for non-existent skill")
		}
	})
}

func TestExecutor_Execute(t *testing.T) {
	registry := NewRegistry()
	executor := NewExecutor(registry, nil)

	skill := newMockSkill("test-skill", "Test Skill")
	skill.execFunc = func(ctx context.Context, input map[string]any) (*Result, error) {
		return NewResult(map[string]any{"echo": input["message"]}), nil
	}
	registry.Register(skill, false)

	t.Run("execute skill", func(t *testing.T) {
		result, err := executor.Execute(context.Background(), "test-skill", map[string]any{
			"message": "hello",
		}, nil)
		if err != nil {
			t.Fatalf("failed to execute skill: %v", err)
		}
		if !result.Success {
			t.Error("expected success")
		}
	})

	t.Run("execute non-existent skill", func(t *testing.T) {
		_, err := executor.Execute(context.Background(), "non-existent", nil, nil)
		if err == nil {
			t.Error("expected error for non-existent skill")
		}
	})

	t.Run("execute disabled skill", func(t *testing.T) {
		registry.Disable("test-skill")
		_, err := executor.Execute(context.Background(), "test-skill", nil, nil)
		if err == nil {
			t.Error("expected error for disabled skill")
		}
		registry.Enable("test-skill")
	})
}

func TestResult(t *testing.T) {
	t.Run("new result", func(t *testing.T) {
		result := NewResult("test data")
		if !result.Success {
			t.Error("expected success")
		}
		if result.Data != "test data" {
			t.Errorf("expected 'test data', got '%v'", result.Data)
		}
	})

	t.Run("new error result", func(t *testing.T) {
		result := NewErrorResult(fmt.Errorf("test error"))
		if result.Success {
			t.Error("expected failure")
		}
		if result.Error != "test error" {
			t.Errorf("expected 'test error', got '%s'", result.Error)
		}
	})

	t.Run("to JSON", func(t *testing.T) {
		result := NewResult("test")
		json, err := result.ToJSON()
		if err != nil {
			t.Fatalf("failed to convert to JSON: %v", err)
		}
		if len(json) == 0 {
			t.Error("expected non-empty JSON")
		}
	})
}

func TestLangContext(t *testing.T) {
	ctx := context.Background()
	if got := GetLang(ctx); got != "en-US" {
		t.Fatalf("GetLang() on empty ctx = %q, want %q", got, "en-US")
	}

	ctx = WithLang(ctx, "zh-CN")
	if got := GetLang(ctx); got != "zh-CN" {
		t.Fatalf("GetLang() = %q, want %q", got, "zh-CN")
	}
}

func TestManifestSkillExecute_LocalizesDeclarativeMessage(t *testing.T) {
	s := NewManifestSkill(&Manifest{ID: "a11y"})
	ctx := WithLang(context.Background(), "zh-CN")

	result, err := s.Execute(ctx, map[string]any{"foo": "bar"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("result.Data type = %T, want map[string]any", result.Data)
	}

	want := i18n.T(i18n.LangZhCN, i18n.MsgSkillDeclarativeHandledByLLM, "a11y")
	if got := data["message"]; got != want {
		t.Fatalf("message = %#v, want %#v", got, want)
	}
}
