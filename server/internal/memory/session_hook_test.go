package memory

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/session"
)

func TestSessionMemoryHook(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "session-hook-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create layered memory service
	layeredMemory, err := NewLayeredMemoryService(nil, LayeredMemoryConfig{
		BaseDir:            tmpDir,
		DailyRetentionDays: 7,
	})
	if err != nil {
		t.Fatalf("failed to create layered memory: %v", err)
	}

	// Create hook
	hook := NewSessionMemoryHook(layeredMemory)
	hook.SetMinMessages(1) // Lower threshold for testing

	ctx := context.Background()

	t.Run("SavesSessionOnArchive", func(t *testing.T) {
		// Create a mock session
		sess := createMockSession("test-agent", "test-channel", "test-peer", 5)
		sess.Metadata.Title = "Test Conversation"
		sess.Metadata.Summary = "This is a test summary"

		// Trigger hook
		err := hook.OnSessionEnd(ctx, sess, session.EndReasonArchive)
		if err != nil {
			t.Errorf("OnSessionEnd failed: %v", err)
		}

		// Verify daily log was created
		today := time.Now().Format("2006-01-02")
		content, err := layeredMemory.GetDailyLog(ctx, today)
		if err != nil {
			t.Errorf("GetDailyLog failed: %v", err)
		}

		if !strings.Contains(content, "test-agent:test-channel:test-peer") {
			t.Error("daily log should contain session ID")
		}
		if !strings.Contains(content, "archive") {
			t.Error("daily log should contain end reason")
		}
		if !strings.Contains(content, "Test Conversation") {
			t.Error("daily log should contain session title")
		}
	})

	t.Run("SkipsSessionWithFewMessages", func(t *testing.T) {
		// Create a new temp dir for this test
		tmpDir2, _ := os.MkdirTemp("", "session-hook-test2")
		defer os.RemoveAll(tmpDir2)

		layeredMemory2, _ := NewLayeredMemoryService(nil, LayeredMemoryConfig{
			BaseDir: tmpDir2,
		})
		hook2 := NewSessionMemoryHook(layeredMemory2)
		hook2.SetMinMessages(10) // High threshold

		// Create session with few messages
		sess := createMockSession("agent", "channel", "peer", 2)

		// Trigger hook
		err := hook2.OnSessionEnd(ctx, sess, session.EndReasonReset)
		if err != nil {
			t.Errorf("OnSessionEnd failed: %v", err)
		}

		// Verify no daily log was created
		today := time.Now().Format("2006-01-02")
		logPath := filepath.Join(tmpDir2, "daily", today+".md")
		if _, err := os.Stat(logPath); !os.IsNotExist(err) {
			t.Error("daily log should not be created for sessions with few messages")
		}
	})

	t.Run("HandlesNilLayeredMemory", func(t *testing.T) {
		hook3 := NewSessionMemoryHook(nil)
		sess := createMockSession("agent", "channel", "peer", 5)

		// Should not panic or error
		err := hook3.OnSessionEnd(ctx, sess, session.EndReasonDelete)
		if err != nil {
			t.Errorf("OnSessionEnd should not error with nil layered memory: %v", err)
		}
	})
}

func TestHookManager(t *testing.T) {
	manager := session.NewHookManager()

	// Create a mock hook that tracks calls
	callCount := 0
	mockHook := &mockSessionHook{
		onEnd: func(ctx context.Context, sess *session.Session, reason session.EndReason) error {
			callCount++
			return nil
		},
	}

	manager.Register(mockHook)

	// Trigger
	sess := createMockSession("agent", "channel", "peer", 3)
	manager.TriggerSessionEnd(context.Background(), sess, session.EndReasonArchive)

	if callCount != 1 {
		t.Errorf("expected hook to be called once, got %d", callCount)
	}
}

// mockSessionHook is a mock implementation of SessionHook for testing.
type mockSessionHook struct {
	onEnd func(ctx context.Context, sess *session.Session, reason session.EndReason) error
}

func (m *mockSessionHook) OnSessionEnd(ctx context.Context, sess *session.Session, reason session.EndReason) error {
	if m.onEnd != nil {
		return m.onEnd(ctx, sess, reason)
	}
	return nil
}

// createMockSession creates a mock session for testing.
func createMockSession(agentID, channelID, peerID string, messageCount int) *session.Session {
	id := session.SessionID{
		AgentID:   agentID,
		ChannelID: channelID,
		PeerID:    peerID,
	}
	sess := session.NewSession(id, 4096)
	sess.Metadata.MessageCount = messageCount
	return sess
}
