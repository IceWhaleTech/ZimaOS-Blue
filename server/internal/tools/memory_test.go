package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// mockMemoryService implements MemoryServiceInterface for testing.
type mockMemoryService struct {
	recallResults []MemorySearchResult
	recallErr     error
	getResult     *MemoryChunkResult
	getErr        error
	statsResult   *MemoryStatsResult
	statsErr      error
	backend       string
	supermemory   bool
}

func (m *mockMemoryService) Recall(ctx context.Context, query string, limit int) ([]MemorySearchResult, error) {
	if m.recallErr != nil {
		return nil, m.recallErr
	}
	return m.recallResults, nil
}

func (m *mockMemoryService) Get(ctx context.Context, id string) (*MemoryChunkResult, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.getResult, nil
}

func (m *mockMemoryService) Stats(ctx context.Context) (*MemoryStatsResult, error) {
	if m.statsErr != nil {
		return nil, m.statsErr
	}
	return m.statsResult, nil
}

func (m *mockMemoryService) GetActiveBackend() string {
	return m.backend
}

func (m *mockMemoryService) IsSupermemoryAvailable() bool {
	return m.supermemory
}

func TestMemorySearchTool_Definition(t *testing.T) {
	tool := NewMemorySearchToolWithInterface(nil)
	def := tool.Definition()

	if def.Name != "memory_search" {
		t.Errorf("expected name 'memory_search', got '%s'", def.Name)
	}

	if def.Description == "" {
		t.Error("description should not be empty")
	}

	params, ok := def.Parameters["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("parameters should have properties")
	}

	if _, ok := params["query"]; !ok {
		t.Error("parameters should have 'query' property")
	}
}

func TestMemorySearchTool_Execute(t *testing.T) {
	now := time.Now()
	mockService := &mockMemoryService{
		recallResults: []MemorySearchResult{
			{
				Chunk: MemoryChunkResult{
					ID:        "test-id-1",
					Content:   "Test memory content",
					Metadata:  map[string]string{"tag": "test"},
					CreatedAt: now,
					UpdatedAt: now,
				},
				VectorScore:   0.9,
				KeywordScore:  0.8,
				CombinedScore: 0.87,
				MatchTypes:    []string{"vector", "keyword"},
			},
		},
		backend: "local",
	}

	tool := NewMemorySearchToolWithInterface(mockService)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"query": "test query",
		"limit": float64(5),
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultStr, ok := result.(string)
	if !ok {
		t.Fatal("result should be a string")
	}

	var response map[string]interface{}
	if err := json.Unmarshal([]byte(resultStr), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response["query"] != "test query" {
		t.Errorf("expected query 'test query', got '%v'", response["query"])
	}

	if response["count"].(float64) != 1 {
		t.Errorf("expected count 1, got %v", response["count"])
	}

	if response["backend"] != "local" {
		t.Errorf("expected backend 'local', got '%v'", response["backend"])
	}
}

func TestMemorySearchTool_Execute_MinScore(t *testing.T) {
	now := time.Now()
	mockService := &mockMemoryService{
		recallResults: []MemorySearchResult{
			{
				Chunk:         MemoryChunkResult{ID: "high-score", Content: "High score", CreatedAt: now, UpdatedAt: now},
				CombinedScore: 0.9,
			},
			{
				Chunk:         MemoryChunkResult{ID: "low-score", Content: "Low score", CreatedAt: now, UpdatedAt: now},
				CombinedScore: 0.3,
			},
		},
		backend: "local",
	}

	tool := NewMemorySearchToolWithInterface(mockService)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"query":     "test",
		"min_score": float64(0.5),
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var response map[string]interface{}
	json.Unmarshal([]byte(result.(string)), &response)

	if response["count"].(float64) != 1 {
		t.Errorf("expected 1 result after min_score filter, got %v", response["count"])
	}
}

func TestMemorySearchTool_Execute_NoService(t *testing.T) {
	tool := NewMemorySearchToolWithInterface(nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"query": "test",
	})

	if err == nil {
		t.Error("expected error when service is nil")
	}
}

func TestMemorySearchTool_Execute_NoQuery(t *testing.T) {
	mockService := &mockMemoryService{backend: "local"}
	tool := NewMemorySearchToolWithInterface(mockService)

	_, err := tool.Execute(context.Background(), map[string]interface{}{})

	if err == nil {
		t.Error("expected error when query is missing")
	}
}

func TestMemoryGetTool_Definition(t *testing.T) {
	tool := NewMemoryGetToolWithInterface(nil)
	def := tool.Definition()

	if def.Name != "memory_get" {
		t.Errorf("expected name 'memory_get', got '%s'", def.Name)
	}

	params, ok := def.Parameters["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("parameters should have properties")
	}

	if _, ok := params["id"]; !ok {
		t.Error("parameters should have 'id' property")
	}
}

func TestMemoryGetTool_Execute(t *testing.T) {
	now := time.Now()
	mockService := &mockMemoryService{
		getResult: &MemoryChunkResult{
			ID:        "test-id",
			Content:   "Test content",
			Metadata:  map[string]string{"key": "value"},
			CreatedAt: now,
			UpdatedAt: now,
		},
		backend: "local",
	}

	tool := NewMemoryGetToolWithInterface(mockService)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"id": "test-id",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var response map[string]interface{}
	json.Unmarshal([]byte(result.(string)), &response)

	if response["id"] != "test-id" {
		t.Errorf("expected id 'test-id', got '%v'", response["id"])
	}

	if response["content"] != "Test content" {
		t.Errorf("expected content 'Test content', got '%v'", response["content"])
	}
}

func TestMemoryGetTool_Execute_NoID(t *testing.T) {
	mockService := &mockMemoryService{backend: "local"}
	tool := NewMemoryGetToolWithInterface(mockService)

	_, err := tool.Execute(context.Background(), map[string]interface{}{})

	if err == nil {
		t.Error("expected error when id is missing")
	}
}

func TestMemoryStatsTool_Definition(t *testing.T) {
	tool := NewMemoryStatsToolWithInterface(nil)
	def := tool.Definition()

	if def.Name != "memory_stats" {
		t.Errorf("expected name 'memory_stats', got '%s'", def.Name)
	}
}

func TestMemoryStatsTool_Execute(t *testing.T) {
	mockService := &mockMemoryService{
		statsResult: &MemoryStatsResult{
			TotalChunks:    100,
			TotalSizeBytes: 1024000,
			OldestChunk:    "2024-01-01T00:00:00Z",
			NewestChunk:    "2024-02-01T00:00:00Z",
			Backend:        "local",
		},
		backend:     "local",
		supermemory: true,
	}

	tool := NewMemoryStatsToolWithInterface(mockService)

	result, err := tool.Execute(context.Background(), map[string]interface{}{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var response map[string]interface{}
	json.Unmarshal([]byte(result.(string)), &response)

	if response["total_chunks"].(float64) != 100 {
		t.Errorf("expected total_chunks 100, got %v", response["total_chunks"])
	}

	if response["backend"] != "local" {
		t.Errorf("expected backend 'local', got '%v'", response["backend"])
	}

	if response["supermemory_available"] != true {
		t.Errorf("expected supermemory_available true, got %v", response["supermemory_available"])
	}
}
