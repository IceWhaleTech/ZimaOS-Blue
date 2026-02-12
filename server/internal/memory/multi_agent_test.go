package memory

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
)

func TestMultiAgentMemoryManager(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "multi-agent-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := config.MemoryConfig{}
	multiCfg := MultiAgentConfig{
		BaseDir:              tmpDir,
		EnableSharedPool:     false,
		DefaultRetentionDays: 7,
	}

	// Use nil embedding provider for testing (will skip vector search)
	mgr, err := NewMultiAgentMemoryManager(cfg, multiCfg, nil)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	ctx := context.Background()

	t.Run("GetAgentMemory", func(t *testing.T) {
		agent1, err := mgr.GetAgentMemory("agent-1")
		if err != nil {
			t.Fatalf("failed to get agent memory: %v", err)
		}
		if agent1.AgentID != "agent-1" {
			t.Errorf("expected agent-1, got %s", agent1.AgentID)
		}

		// Get same agent again should return same instance
		agent1Again, err := mgr.GetAgentMemory("agent-1")
		if err != nil {
			t.Fatalf("failed to get agent memory again: %v", err)
		}
		if agent1 != agent1Again {
			t.Error("expected same instance")
		}
	})

	t.Run("ListAgents", func(t *testing.T) {
		_, _ = mgr.GetAgentMemory("agent-2")
		agents := mgr.ListAgents()
		if len(agents) < 2 {
			t.Errorf("expected at least 2 agents, got %d", len(agents))
		}
	})

	t.Run("LayeredMemory", func(t *testing.T) {
		agent, _ := mgr.GetAgentMemory("agent-3")

		err := agent.AppendToDaily(ctx, "Daily note for agent 3", []string{"daily"})
		if err != nil {
			t.Errorf("failed to append to daily: %v", err)
		}

		err = agent.PromoteToLongTerm(ctx, "Important fact", "Facts")
		if err != nil {
			t.Errorf("failed to promote to long term: %v", err)
		}

		content, err := agent.GetLongTermMemory(ctx)
		if err != nil {
			t.Errorf("failed to get long term memory: %v", err)
		}
		if content == "" {
			t.Error("expected non-empty long term memory")
		}
	})

	t.Run("DeleteAgentMemory", func(t *testing.T) {
		// Skip on Windows due to file locking issues with SQLite
		if os.Getenv("GOOS") == "windows" || filepath.Separator == '\\' {
			t.Skip("skipping delete test on Windows due to file locking")
		}

		_, _ = mgr.GetAgentMemory("agent-to-delete")

		err := mgr.DeleteAgentMemory("agent-to-delete")
		if err != nil {
			t.Errorf("failed to delete agent memory: %v", err)
		}

		// Directory should be removed
		agentDir := filepath.Join(tmpDir, "agents", "agent-to-delete")
		if _, err := os.Stat(agentDir); !os.IsNotExist(err) {
			t.Error("agent directory should be deleted")
		}
	})
}

func TestMultiAgentMemoryManager_SharedPool(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "multi-agent-shared-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := config.MemoryConfig{}
	multiCfg := MultiAgentConfig{
		BaseDir:          tmpDir,
		EnableSharedPool: true,
	}

	mgr, err := NewMultiAgentMemoryManager(cfg, multiCfg, nil)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	shared := mgr.GetSharedMemory()
	if shared == nil {
		t.Error("shared memory should be available")
	}
}
