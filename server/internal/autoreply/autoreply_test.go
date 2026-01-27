package autoreply

import (
	"context"
	"testing"

	"go.uber.org/zap"
)

func TestNewService(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()

	s := NewService(cfg, logger)
	if s == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestCreate(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	rule, err := s.Create("greeting", TriggerKeyword, "hello", []string{"Hi there!"}, 10)
	if err != nil {
		t.Fatalf("failed to create rule: %v", err)
	}

	if rule.ID == "" {
		t.Error("expected non-empty ID")
	}

	if rule.Name != "greeting" {
		t.Errorf("expected name 'greeting', got '%s'", rule.Name)
	}

	if rule.TriggerType != TriggerKeyword {
		t.Errorf("expected trigger type %s, got %s", TriggerKeyword, rule.TriggerType)
	}

	if !rule.Enabled {
		t.Error("expected rule to be enabled")
	}
}

func TestCreateInvalidRegex(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	_, err := s.Create("invalid", TriggerRegex, "[invalid", []string{"response"}, 10)
	if err == nil {
		t.Error("expected error for invalid regex")
	}
}

func TestCreateEmptyTrigger(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	_, err := s.Create("empty", TriggerKeyword, "", []string{"response"}, 10)
	if err == nil {
		t.Error("expected error for empty trigger")
	}
}

func TestCreateEmptyResponses(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	_, err := s.Create("empty", TriggerKeyword, "trigger", []string{}, 10)
	if err == nil {
		t.Error("expected error for empty responses")
	}
}

func TestGet(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	rule, _ := s.Create("test", TriggerKeyword, "test", []string{"response"}, 10)

	found, exists := s.Get(rule.ID)
	if !exists {
		t.Fatal("rule should exist")
	}

	if found.ID != rule.ID {
		t.Errorf("expected ID %s, got %s", rule.ID, found.ID)
	}

	_, exists = s.Get("non-existent")
	if exists {
		t.Error("rule should not exist")
	}
}

func TestList(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	s.Create("rule1", TriggerKeyword, "test1", []string{"response1"}, 5)
	s.Create("rule2", TriggerKeyword, "test2", []string{"response2"}, 10)
	s.Create("rule3", TriggerKeyword, "test3", []string{"response3"}, 1)

	rules := s.List()
	if len(rules) != 3 {
		t.Errorf("expected 3 rules, got %d", len(rules))
	}

	// Check priority ordering
	if rules[0].Priority != 10 {
		t.Errorf("expected first rule priority 10, got %d", rules[0].Priority)
	}
	if rules[1].Priority != 5 {
		t.Errorf("expected second rule priority 5, got %d", rules[1].Priority)
	}
	if rules[2].Priority != 1 {
		t.Errorf("expected third rule priority 1, got %d", rules[2].Priority)
	}
}

func TestUpdate(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	rule, _ := s.Create("original", TriggerKeyword, "original", []string{"original"}, 10)

	err := s.Update(rule.ID, "updated", TriggerContains, "updated", []string{"updated response"}, 20)
	if err != nil {
		t.Fatalf("failed to update: %v", err)
	}

	updated, _ := s.Get(rule.ID)
	if updated.Name != "updated" {
		t.Errorf("expected name 'updated', got '%s'", updated.Name)
	}
	if updated.TriggerType != TriggerContains {
		t.Errorf("expected trigger type %s, got %s", TriggerContains, updated.TriggerType)
	}
	if updated.Priority != 20 {
		t.Errorf("expected priority 20, got %d", updated.Priority)
	}
}

func TestDelete(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	rule, _ := s.Create("to-delete", TriggerKeyword, "test", []string{"response"}, 10)

	err := s.Delete(rule.ID)
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	_, exists := s.Get(rule.ID)
	if exists {
		t.Error("rule should have been deleted")
	}

	err = s.Delete("non-existent")
	if err == nil {
		t.Error("expected error when deleting non-existent rule")
	}
}

func TestEnableDisable(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	rule, _ := s.Create("test", TriggerKeyword, "test", []string{"response"}, 10)

	// Disable
	err := s.Disable(rule.ID)
	if err != nil {
		t.Fatalf("failed to disable: %v", err)
	}

	disabled, _ := s.Get(rule.ID)
	if disabled.Enabled {
		t.Error("expected rule to be disabled")
	}

	// Enable
	err = s.Enable(rule.ID)
	if err != nil {
		t.Fatalf("failed to enable: %v", err)
	}

	enabled, _ := s.Get(rule.ID)
	if !enabled.Enabled {
		t.Error("expected rule to be enabled")
	}
}

func TestMatchKeyword(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	s.Create("greeting", TriggerKeyword, "hello", []string{"Hi there, {{user}}!"}, 10)

	ctx := context.Background()
	response, rule, err := s.Match(ctx, "hello", "telegram", "user123", "John", "chat456")
	if err != nil {
		t.Fatalf("match failed: %v", err)
	}

	if rule == nil {
		t.Fatal("expected rule to match")
	}

	if response != "Hi there, John!" {
		t.Errorf("expected 'Hi there, John!', got '%s'", response)
	}
}

func TestMatchContains(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	s.Create("help", TriggerContains, "help", []string{"How can I help you?"}, 10)

	ctx := context.Background()
	response, rule, _ := s.Match(ctx, "I need help please", "telegram", "user123", "John", "chat456")

	if rule == nil {
		t.Fatal("expected rule to match")
	}

	if response != "How can I help you?" {
		t.Errorf("expected 'How can I help you?', got '%s'", response)
	}
}

func TestMatchPrefix(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	s.Create("command", TriggerPrefix, "/start", []string{"Welcome!"}, 10)

	ctx := context.Background()
	_, rule, _ := s.Match(ctx, "/start now", "telegram", "user123", "John", "chat456")

	if rule == nil {
		t.Fatal("expected rule to match")
	}
}

func TestMatchSuffix(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	s.Create("question", TriggerSuffix, "?", []string{"That's a good question!"}, 10)

	ctx := context.Background()
	_, rule, _ := s.Match(ctx, "What is this?", "telegram", "user123", "John", "chat456")

	if rule == nil {
		t.Fatal("expected rule to match")
	}
}

func TestMatchRegex(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	s.Create("email", TriggerRegex, `(\w+@\w+\.\w+)`, []string{"I see you mentioned {{$1}}"}, 10)

	ctx := context.Background()
	response, rule, _ := s.Match(ctx, "Contact me at test@example.com", "telegram", "user123", "John", "chat456")

	if rule == nil {
		t.Fatal("expected rule to match")
	}

	if response != "I see you mentioned test@example.com" {
		t.Errorf("expected 'I see you mentioned test@example.com', got '%s'", response)
	}
}

func TestMatchCaseInsensitive(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	s.Create("greeting", TriggerKeyword, "hello", []string{"Hi!"}, 10)

	ctx := context.Background()
	_, rule, _ := s.Match(ctx, "HELLO", "telegram", "user123", "John", "chat456")

	if rule == nil {
		t.Fatal("expected rule to match (case insensitive)")
	}
}

func TestMatchPriority(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	s.Create("low", TriggerContains, "test", []string{"Low priority"}, 1)
	s.Create("high", TriggerContains, "test", []string{"High priority"}, 10)

	ctx := context.Background()
	response, _, _ := s.Match(ctx, "this is a test", "telegram", "user123", "John", "chat456")

	if response != "High priority" {
		t.Errorf("expected 'High priority', got '%s'", response)
	}
}

func TestMatchChannelFilter(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	rule, _ := s.Create("telegram-only", TriggerKeyword, "hello", []string{"Hi!"}, 10)
	s.SetChannels(rule.ID, []string{"telegram"})

	ctx := context.Background()

	// Should match on telegram
	_, matched, _ := s.Match(ctx, "hello", "telegram", "user123", "John", "chat456")
	if matched == nil {
		t.Error("expected rule to match on telegram")
	}

	// Should not match on discord
	_, matched, _ = s.Match(ctx, "hello", "discord", "user123", "John", "chat456")
	if matched != nil {
		t.Error("expected rule not to match on discord")
	}
}

func TestMatchNoMatch(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	s.Create("greeting", TriggerKeyword, "hello", []string{"Hi!"}, 10)

	ctx := context.Background()
	_, rule, _ := s.Match(ctx, "goodbye", "telegram", "user123", "John", "chat456")

	if rule != nil {
		t.Error("expected no match")
	}
}

func TestMatchDisabledRule(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	rule, _ := s.Create("greeting", TriggerKeyword, "hello", []string{"Hi!"}, 10)
	s.Disable(rule.ID)

	ctx := context.Background()
	_, matched, _ := s.Match(ctx, "hello", "telegram", "user123", "John", "chat456")

	if matched != nil {
		t.Error("expected disabled rule not to match")
	}
}

func TestTest(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	s.Create("greeting", TriggerKeyword, "hello", []string{"Hi!"}, 10)

	response, rule, err := s.Test("hello", "telegram")
	if err != nil {
		t.Fatalf("test failed: %v", err)
	}

	if rule == nil {
		t.Fatal("expected rule to match")
	}

	if response != "Hi!" {
		t.Errorf("expected 'Hi!', got '%s'", response)
	}
}

func TestTemplateRendering(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	s.Create("template", TriggerKeyword, "test", []string{
		"Hello {{user}}! You said: {{message}}. Time: {{time}}",
	}, 10)

	ctx := context.Background()
	response, _, _ := s.Match(ctx, "test", "telegram", "user123", "John", "chat456")

	if response == "" {
		t.Error("expected non-empty response")
	}

	// Check that template variables were replaced
	if response == "Hello {{user}}! You said: {{message}}. Time: {{time}}" {
		t.Error("template variables were not replaced")
	}
}

func TestRandomResponse(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	s := NewService(cfg, logger)

	s.Create("random", TriggerKeyword, "test", []string{
		"Response 1",
		"Response 2",
		"Response 3",
	}, 10)

	ctx := context.Background()
	responses := make(map[string]bool)

	// Run multiple times to check randomness
	for i := 0; i < 100; i++ {
		response, _, _ := s.Match(ctx, "test", "telegram", "user123", "John", "chat456")
		responses[response] = true
	}

	// Should have gotten at least 2 different responses
	if len(responses) < 2 {
		t.Error("expected random responses, but got same response every time")
	}
}
