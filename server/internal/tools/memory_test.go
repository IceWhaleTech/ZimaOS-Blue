package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// mockMemoryService implements MemoryServiceInterface for testing.
type mockMemoryService struct {
	recallResults   []MemorySearchResult
	recallErr       error
	getResult       *MemoryChunkResult
	getErr          error
	statsResult     *MemoryStatsResult
	statsErr        error
	rememberResult  *MemoryChunkResult
	rememberErr     error
	forgetErr       error
	backend         string
	lastForgetID    string
	lastRememberContent string
	lastRememberTags    []string
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

func (m *mockMemoryService) Remember(ctx context.Context, content string, tags []string) (*MemoryChunkResult, error) {
	m.lastRememberContent = content
	m.lastRememberTags = tags
	if m.rememberErr != nil {
		return nil, m.rememberErr
	}
	return m.rememberResult, nil
}

func (m *mockMemoryService) Forget(ctx context.Context, id string) error {
	m.lastForgetID = id
	return m.forgetErr
}

func (m *mockMemoryService) GetActiveBackend() string {
	return m.backend
}

func TestMemoryTool_Definition(t *testing.T) {
	tool := NewMemoryTool(nil)
	def := tool.Definition()

	if def.Name != "memory" {
		t.Errorf("expected name 'memory', got '%s'", def.Name)
	}

	if def.Description == "" {
		t.Error("description should not be empty")
	}

	params, ok := def.Parameters["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("parameters should have properties")
	}

	if _, ok := params["action"]; !ok {
		t.Error("parameters should have 'action' property")
	}
	if _, ok := params["query"]; !ok {
		t.Error("parameters should have 'query' property")
	}
	if _, ok := params["id"]; !ok {
		t.Error("parameters should have 'id' property")
	}
}

func TestMemoryTool_Search(t *testing.T) {
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

	tool := NewMemoryTool(mockService)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "search",
		"query":  "test query",
		"limit":  float64(5),
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var response map[string]interface{}
	json.Unmarshal([]byte(result.(string)), &response)

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

func TestMemoryTool_Search_MinScore(t *testing.T) {
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

	tool := NewMemoryTool(mockService)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "search",
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

func TestMemoryTool_Search_NoService(t *testing.T) {
	tool := NewMemoryTool(nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "search",
		"query":  "test",
	})

	if err == nil {
		t.Error("expected error when service is nil")
	}
}

func TestMemoryTool_Search_NoQuery(t *testing.T) {
	mockService := &mockMemoryService{backend: "local"}
	tool := NewMemoryTool(mockService)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "search",
	})

	if err == nil {
		t.Error("expected error when query is missing")
	}
}

func TestMemoryTool_Get(t *testing.T) {
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

	tool := NewMemoryTool(mockService)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "get",
		"id":     "test-id",
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

func TestMemoryTool_Get_NoID(t *testing.T) {
	mockService := &mockMemoryService{backend: "local"}
	tool := NewMemoryTool(mockService)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "get",
	})

	if err == nil {
		t.Error("expected error when id is missing")
	}
}

func TestMemoryTool_Stats(t *testing.T) {
	mockService := &mockMemoryService{
		statsResult: &MemoryStatsResult{
			TotalChunks:    100,
			TotalSizeBytes: 1024000,
			OldestChunk:    "2024-01-01T00:00:00Z",
			NewestChunk:    "2024-02-01T00:00:00Z",
			Backend:        "local",
		},
		backend: "local",
	}

	tool := NewMemoryTool(mockService)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "stats",
	})

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
}

func TestMemoryTool_UnknownAction(t *testing.T) {
	mockService := &mockMemoryService{backend: "local"}
	tool := NewMemoryTool(mockService)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "delete",
	})

	if err == nil {
		t.Error("expected error for unknown action")
	}
}

func TestMemoryTool_Remember(t *testing.T) {
	now := time.Now()
	mockService := &mockMemoryService{
		rememberResult: &MemoryChunkResult{
			ID:        "new-id",
			Content:   "Remember this fact",
			CreatedAt: now,
			UpdatedAt: now,
		},
		backend: "local",
	}

	tool := NewMemoryTool(mockService)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":  "remember",
		"content": "Remember this fact",
		"tags":    []interface{}{"important", "test"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var response map[string]interface{}
	json.Unmarshal([]byte(result.(string)), &response)

	if response["id"] != "new-id" {
		t.Errorf("expected id 'new-id', got '%v'", response["id"])
	}
	if response["success"] != true {
		t.Errorf("expected success true, got '%v'", response["success"])
	}
	if mockService.lastRememberContent != "Remember this fact" {
		t.Errorf("expected content 'Remember this fact', got '%s'", mockService.lastRememberContent)
	}
	if len(mockService.lastRememberTags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(mockService.lastRememberTags))
	}
}

func TestMemoryTool_Remember_NoContent(t *testing.T) {
	mockService := &mockMemoryService{backend: "local"}
	tool := NewMemoryTool(mockService)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "remember",
	})

	if err == nil {
		t.Error("expected error when content is missing")
	}
}

func TestMemoryTool_Forget(t *testing.T) {
	mockService := &mockMemoryService{backend: "local"}
	tool := NewMemoryTool(mockService)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "forget",
		"id":     "mem-123",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var response map[string]interface{}
	json.Unmarshal([]byte(result.(string)), &response)

	if response["id"] != "mem-123" {
		t.Errorf("expected id 'mem-123', got '%v'", response["id"])
	}
	if response["success"] != true {
		t.Errorf("expected success true, got '%v'", response["success"])
	}
	if mockService.lastForgetID != "mem-123" {
		t.Errorf("expected forget ID 'mem-123', got '%s'", mockService.lastForgetID)
	}
}

func TestMemoryTool_Forget_NoID(t *testing.T) {
	mockService := &mockMemoryService{backend: "local"}
	tool := NewMemoryTool(mockService)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "forget",
	})

	if err == nil {
		t.Error("expected error when id is missing")
	}
}
