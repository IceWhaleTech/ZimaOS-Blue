package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/embedding"
)

// MultiAgentMemoryManager manages isolated memory services for multiple agents.
type MultiAgentMemoryManager struct {
	baseDir           string
	config            config.MemoryConfig
	embeddingProvider embedding.Provider
	agents            map[string]*AgentMemory
	sharedMemory      *UnifiedMemoryService
	mu                sync.RWMutex
}

// AgentMemory holds memory services for a single agent.
type AgentMemory struct {
	AgentID        string
	UnifiedService *UnifiedMemoryService
	LayeredService *LayeredMemoryService
	baseDir        string
}

// MultiAgentConfig holds configuration for multi-agent memory.
type MultiAgentConfig struct {
	BaseDir              string `json:"base_dir" yaml:"base_dir"`
	EnableSharedPool     bool   `json:"enable_shared_pool" yaml:"enable_shared_pool"`
	DefaultRetentionDays int    `json:"default_retention_days" yaml:"default_retention_days"`
}

// NewMultiAgentMemoryManager creates a new multi-agent memory manager.
func NewMultiAgentMemoryManager(cfg config.MemoryConfig, multiCfg MultiAgentConfig, embProvider embedding.Provider) (*MultiAgentMemoryManager, error) {
	if multiCfg.BaseDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		multiCfg.BaseDir = filepath.Join(homeDir, ".zimaos-echo", "memory")
	}

	if multiCfg.DefaultRetentionDays == 0 {
		multiCfg.DefaultRetentionDays = 30
	}

	if err := os.MkdirAll(multiCfg.BaseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	mgr := &MultiAgentMemoryManager{
		baseDir:           multiCfg.BaseDir,
		config:            cfg,
		embeddingProvider: embProvider,
		agents:            make(map[string]*AgentMemory),
	}

	if multiCfg.EnableSharedPool {
		sharedDir := filepath.Join(multiCfg.BaseDir, "_shared")
		localService, err := mgr.createLocalService(sharedDir)
		if err != nil {
			return nil, fmt.Errorf("failed to create shared memory: %w", err)
		}
		mgr.sharedMemory = NewUnifiedMemoryService(localService, cfg)
	}

	return mgr, nil
}

// createLocalService creates a local memory service for a directory.
func (m *MultiAgentMemoryManager) createLocalService(dir string) (*MemoryService, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(dir, "memory.db")
	store, err := NewVectorStore(VectorStoreConfig{
		DBPath:    dbPath,
		EnableFTS: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create vector store: %w", err)
	}

	searcher := NewHybridSearcher(store, m.embeddingProvider, m.config)
	return NewMemoryService(searcher), nil
}

// GetAgentMemory gets or creates memory services for an agent.
func (m *MultiAgentMemoryManager) GetAgentMemory(agentID string) (*AgentMemory, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if agent, ok := m.agents[agentID]; ok {
		return agent, nil
	}

	agent, err := m.createAgentMemory(agentID)
	if err != nil {
		return nil, err
	}

	m.agents[agentID] = agent
	return agent, nil
}

// createAgentMemory creates memory services for a new agent.
func (m *MultiAgentMemoryManager) createAgentMemory(agentID string) (*AgentMemory, error) {
	agentDir := filepath.Join(m.baseDir, "agents", agentID)

	localService, err := m.createLocalService(agentDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create local memory: %w", err)
	}

	unifiedService := NewUnifiedMemoryService(localService, m.config)

	layeredService, err := NewLayeredMemoryService(unifiedService, LayeredMemoryConfig{
		BaseDir:            agentDir,
		DailyRetentionDays: 30,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create layered memory: %w", err)
	}

	return &AgentMemory{
		AgentID:        agentID,
		UnifiedService: unifiedService,
		LayeredService: layeredService,
		baseDir:        agentDir,
	}, nil
}

// ListAgents returns all agent IDs.
func (m *MultiAgentMemoryManager) ListAgents() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	agents := make([]string, 0, len(m.agents))
	for id := range m.agents {
		agents = append(agents, id)
	}
	return agents
}

// GetSharedMemory returns the shared memory pool.
func (m *MultiAgentMemoryManager) GetSharedMemory() *UnifiedMemoryService {
	return m.sharedMemory
}

// DeleteAgentMemory removes all memory for an agent.
func (m *MultiAgentMemoryManager) DeleteAgentMemory(agentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Close the agent's services before deleting
	if agent, ok := m.agents[agentID]; ok {
		agent.Close()
	}

	delete(m.agents, agentID)
	agentDir := filepath.Join(m.baseDir, "agents", agentID)
	return os.RemoveAll(agentDir)
}

// CrossAgentSearch searches across multiple agents' memories.
func (m *MultiAgentMemoryManager) CrossAgentSearch(ctx context.Context, query string, agentIDs []string, limit int) (map[string][]HybridSearchResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make(map[string][]HybridSearchResult)
	for _, agentID := range agentIDs {
		if agent, ok := m.agents[agentID]; ok {
			if r, err := agent.UnifiedService.Recall(ctx, query, limit); err == nil {
				results[agentID] = r
			}
		}
	}
	return results, nil
}

// Stats returns statistics for all agents.
func (m *MultiAgentMemoryManager) Stats(ctx context.Context) (map[string]*MemoryStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[string]*MemoryStats)
	for agentID, agent := range m.agents {
		if s, err := agent.UnifiedService.Stats(ctx); err == nil {
			stats[agentID] = s
		}
	}
	return stats, nil
}

// AgentMemory methods
func (a *AgentMemory) Remember(ctx context.Context, content string, tags []string) (*MemoryChunk, error) {
	return a.UnifiedService.Remember(ctx, content, tags)
}

func (a *AgentMemory) Recall(ctx context.Context, query string, limit int) ([]HybridSearchResult, error) {
	return a.UnifiedService.Recall(ctx, query, limit)
}

func (a *AgentMemory) AppendToDaily(ctx context.Context, content string, tags []string) error {
	return a.LayeredService.AppendToDaily(ctx, content, tags)
}

func (a *AgentMemory) GetLongTermMemory(ctx context.Context) (string, error) {
	return a.LayeredService.GetLongTermMemory(ctx)
}

func (a *AgentMemory) PromoteToLongTerm(ctx context.Context, content string, category string) error {
	return a.LayeredService.PromoteToLongTerm(ctx, content, category)
}

// Close closes the agent's memory services.
func (a *AgentMemory) Close() error {
	// Note: Database connections are managed by the VectorStore
	// On Windows, files may remain locked until GC runs
	// This is a best-effort cleanup
	return nil
}
