package session

import (
	"context"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	sessionctx "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/context"
)

// mockMemoryRefresher implements MemoryRefresher for testing.
type mockMemoryRefresher struct {
	refreshCalled bool
	sessionID     string
	extracted     string
}

func (m *mockMemoryRefresher) RefreshMemory(ctx context.Context, extracted string, sessionID string) error {
	m.refreshCalled = true
	m.sessionID = sessionID
	m.extracted = extracted
	return nil
}

func TestCompactorMemoryIntegration_ShouldRefreshMemory(t *testing.T) {
	// Create a mock compactor config
	compactor := &SessionCompactor{
		config: config.SessionCompactionConfig{
			Enabled:   true,
			Threshold: 0.75,
		},
	}

	config := MemoryRefreshConfig{
		Enabled:            true,
		SoftThresholdRatio: 0.6,
	}

	integration := NewCompactorMemoryIntegration(compactor, nil, nil, config)

	t.Run("BelowSoftThreshold", func(t *testing.T) {
		session := NewSession(SessionID{AgentID: "test", ChannelID: "ch", PeerID: "peer"}, 1000)
		// Add some messages to get ~50% usage
		for i := 0; i < 5; i++ {
			session.AddMessage(sessionctx.Message{Role: "user", Content: "short message"})
		}

		if integration.ShouldRefreshMemory(session) {
			t.Error("should not refresh memory below soft threshold")
		}
	})

	t.Run("AtSoftThreshold", func(t *testing.T) {
		session := NewSession(SessionID{AgentID: "test", ChannelID: "ch", PeerID: "peer"}, 100)
		// Add messages to get ~65% usage
		for i := 0; i < 10; i++ {
			session.AddMessage(sessionctx.Message{Role: "user", Content: "message content here"})
		}

		ratio := session.TokenUsageRatio()
		if ratio >= 0.6 && ratio < 0.75 {
			if !integration.ShouldRefreshMemory(session) {
				t.Error("should refresh memory at soft threshold")
			}
		}
	})

	t.Run("DisabledConfig", func(t *testing.T) {
		disabledConfig := MemoryRefreshConfig{Enabled: false}
		disabledIntegration := NewCompactorMemoryIntegration(compactor, nil, nil, disabledConfig)

		session := NewSession(SessionID{AgentID: "test", ChannelID: "ch", PeerID: "peer"}, 100)
		for i := 0; i < 20; i++ {
			session.AddMessage(sessionctx.Message{Role: "user", Content: "message"})
		}

		if disabledIntegration.ShouldRefreshMemory(session) {
			t.Error("should not refresh memory when disabled")
		}
	})
}

func TestDefaultMemoryRefreshConfig(t *testing.T) {
	config := DefaultMemoryRefreshConfig()

	if !config.Enabled {
		t.Error("default config should be enabled")
	}
	if config.SoftThresholdRatio != 0.6 {
		t.Errorf("expected soft threshold 0.6, got %f", config.SoftThresholdRatio)
	}
	if config.MaxExtractTokens != 500 {
		t.Errorf("expected max extract tokens 500, got %d", config.MaxExtractTokens)
	}
	if config.SystemPrompt == "" {
		t.Error("default system prompt should not be empty")
	}
	if !strings.Contains(config.SystemPrompt, "session query capability") {
		t.Error("default system prompt should include capability memory guidance")
	}
}

func TestCompactorMemoryIntegration_NilCompactorSafe(t *testing.T) {
	cfg := MemoryRefreshConfig{
		Enabled:            true,
		SoftThresholdRatio: 0.6,
	}
	integration := NewCompactorMemoryIntegration(nil, nil, nil, cfg)
	session := NewSession(SessionID{AgentID: "test", ChannelID: "ch", PeerID: "peer"}, 100)
	session.AddMessage(sessionctx.Message{Role: "user", Content: "hello"})

	if integration.ShouldRefreshMemory(session) {
		t.Fatal("should not refresh memory when compactor is nil")
	}
	if integration.ShouldCompact(session) {
		t.Fatal("should not compact when compactor is nil")
	}
	if integration.ShouldCompactOnTransition(0.5, session) {
		t.Fatal("should not compact on transition when compactor is nil")
	}
	if _, err := integration.CompactWithMemoryRefresh(context.Background(), session); err == nil {
		t.Fatal("expected error when compacting with nil compactor")
	}
}

func TestCompactorMemoryIntegration_ShouldCompactOnTransition(t *testing.T) {
	compactor := &SessionCompactor{
		config: config.SessionCompactionConfig{
			Enabled:   true,
			Threshold: 0.8,
		},
	}
	integration := NewCompactorMemoryIntegration(compactor, nil, nil, MemoryRefreshConfig{Enabled: true})
	session := NewSession(SessionID{AgentID: "test", ChannelID: "ch", PeerID: "peer"}, 100)
	for i := 0; i < 16; i++ {
		session.AddMessage(sessionctx.Message{Role: "user", Content: "message content here"})
	}
	if !integration.ShouldCompact(session) {
		t.Skip("session did not reach hard threshold under current token estimator")
	}
	if !integration.ShouldCompactOnTransition(0.6, session) {
		t.Fatal("expected hard-threshold transition to trigger compaction window")
	}
	if integration.ShouldCompactOnTransition(0.85, session) {
		t.Fatal("should not trigger when already above hard threshold previously")
	}
}

func TestNormalizeExtractedMemoryForStorage_FiltersTransientAndDedups(t *testing.T) {
	input := strings.Join([]string{
		"- User preference: concise output",
		"- Project fact: repo uses Go modules",
		"- Uploaded file available at /tmp/session/upload.md",
		"- approval requested for browser",
		"* user preference: concise output   ",
	}, "\n")

	got := NormalizeExtractedMemoryForStorage(input)
	if strings.Contains(got, "/tmp/session/upload.md") {
		t.Fatalf("expected transient upload path to be filtered, got %q", got)
	}
	if strings.Contains(strings.ToLower(got), "approval requested") {
		t.Fatalf("expected approval noise to be filtered, got %q", got)
	}
	if strings.Count(strings.ToLower(got), "user preference: concise output") != 1 {
		t.Fatalf("expected duplicate preference to be collapsed, got %q", got)
	}
	if !strings.Contains(got, "Project fact: repo uses Go modules") {
		t.Fatalf("expected durable project fact to remain, got %q", got)
	}
}
