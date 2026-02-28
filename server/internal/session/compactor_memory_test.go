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
	messageCount  int
}

func (m *mockMemoryRefresher) RefreshMemory(ctx context.Context, messages []sessionctx.Message, sessionID string) error {
	m.refreshCalled = true
	m.sessionID = sessionID
	m.messageCount = len(messages)
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
