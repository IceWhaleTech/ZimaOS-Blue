package memory

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Test Store creation
func TestNewStore(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	if store == nil {
		t.Fatal("expected store, got nil")
	}
}

// Test Store with in-memory database
func TestNewStoreInMemory(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create in-memory store: %v", err)
	}
	defer store.Close()

	if store == nil {
		t.Fatal("expected store, got nil")
	}
}

// Test Conversation creation
func TestStoreCreateConversation(t *testing.T) {
	store, _ := NewStore(":memory:")
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Test Conversation")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	if conv.ID == "" {
		t.Error("expected conversation ID")
	}
	if conv.Title != "Test Conversation" {
		t.Errorf("expected title 'Test Conversation', got '%s'", conv.Title)
	}
	if conv.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

// Test Conversation retrieval
func TestStoreGetConversation(t *testing.T) {
	store, _ := NewStore(":memory:")
	defer store.Close()

	created, _ := store.CreateConversation(context.Background(), "Test")

	conv, err := store.GetConversation(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("failed to get conversation: %v", err)
	}

	if conv.ID != created.ID {
		t.Errorf("expected ID '%s', got '%s'", created.ID, conv.ID)
	}
}

// Test Conversation not found
func TestStoreGetConversationNotFound(t *testing.T) {
	store, _ := NewStore(":memory:")
	defer store.Close()

	_, err := store.GetConversation(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent conversation")
	}
}

// Test List conversations
func TestStoreListConversations(t *testing.T) {
	store, _ := NewStore(":memory:")
	defer store.Close()

	store.CreateConversation(context.Background(), "Conv 1")
	store.CreateConversation(context.Background(), "Conv 2")
	store.CreateConversation(context.Background(), "Conv 3")

	convs, err := store.ListConversations(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("failed to list conversations: %v", err)
	}

	if len(convs) != 3 {
		t.Errorf("expected 3 conversations, got %d", len(convs))
	}
}

// Test List conversations with pagination
func TestStoreListConversationsPagination(t *testing.T) {
	store, _ := NewStore(":memory:")
	defer store.Close()

	for i := 0; i < 10; i++ {
		store.CreateConversation(context.Background(), "Conv")
	}

	// Get first page
	page1, _ := store.ListConversations(context.Background(), 5, 0)
	if len(page1) != 5 {
		t.Errorf("expected 5 conversations on page 1, got %d", len(page1))
	}

	// Get second page
	page2, _ := store.ListConversations(context.Background(), 5, 5)
	if len(page2) != 5 {
		t.Errorf("expected 5 conversations on page 2, got %d", len(page2))
	}
}

// Test Delete conversation
func TestStoreDeleteConversation(t *testing.T) {
	store, _ := NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "To Delete")

	err := store.DeleteConversation(context.Background(), conv.ID)
	if err != nil {
		t.Fatalf("failed to delete conversation: %v", err)
	}

	_, err = store.GetConversation(context.Background(), conv.ID)
	if err == nil {
		t.Error("expected error after deletion")
	}
}

// Test Update conversation title
func TestStoreUpdateConversationTitle(t *testing.T) {
	store, _ := NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Original")

	err := store.UpdateConversationTitle(context.Background(), conv.ID, "Updated")
	if err != nil {
		t.Fatalf("failed to update title: %v", err)
	}

	updated, _ := store.GetConversation(context.Background(), conv.ID)
	if updated.Title != "Updated" {
		t.Errorf("expected title 'Updated', got '%s'", updated.Title)
	}
}

// Test Add message
func TestStoreAddMessage(t *testing.T) {
	store, _ := NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Test")

	msg, err := store.AddMessage(context.Background(), conv.ID, Message{
		Role:    "user",
		Content: "Hello!",
	})
	if err != nil {
		t.Fatalf("failed to add message: %v", err)
	}

	if msg.ID == "" {
		t.Error("expected message ID")
	}
	if msg.Role != "user" {
		t.Errorf("expected role 'user', got '%s'", msg.Role)
	}
	if msg.Content != "Hello!" {
		t.Errorf("expected content 'Hello!', got '%s'", msg.Content)
	}
}

// Test Get messages
func TestStoreGetMessages(t *testing.T) {
	store, _ := NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Test")

	store.AddMessage(context.Background(), conv.ID, Message{Role: "user", Content: "Hello"})
	store.AddMessage(context.Background(), conv.ID, Message{Role: "assistant", Content: "Hi there!"})
	store.AddMessage(context.Background(), conv.ID, Message{Role: "user", Content: "How are you?"})

	messages, err := store.GetMessages(context.Background(), conv.ID, 10, 0)
	if err != nil {
		t.Fatalf("failed to get messages: %v", err)
	}

	if len(messages) != 3 {
		t.Errorf("expected 3 messages, got %d", len(messages))
	}
}

// Test Get messages order
func TestStoreGetMessagesOrder(t *testing.T) {
	store, _ := NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Test")

	store.AddMessage(context.Background(), conv.ID, Message{Role: "user", Content: "First"})
	time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	store.AddMessage(context.Background(), conv.ID, Message{Role: "assistant", Content: "Second"})

	messages, _ := store.GetMessages(context.Background(), conv.ID, 10, 0)

	if messages[0].Content != "First" {
		t.Error("expected messages in chronological order")
	}
	if messages[1].Content != "Second" {
		t.Error("expected messages in chronological order")
	}
}

// Test Message with tool calls
func TestStoreMessageWithToolCalls(t *testing.T) {
	store, _ := NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Test")

	msg, err := store.AddMessage(context.Background(), conv.ID, Message{
		Role:    "assistant",
		Content: "",
		ToolCalls: []ToolCall{
			{ID: "call_1", Name: "calculator", Arguments: `{"expression": "2+2"}`},
		},
	})
	if err != nil {
		t.Fatalf("failed to add message with tool calls: %v", err)
	}

	messages, _ := store.GetMessages(context.Background(), conv.ID, 10, 0)
	if len(messages[0].ToolCalls) != 1 {
		t.Errorf("expected 1 tool call, got %d", len(messages[0].ToolCalls))
	}
	if messages[0].ToolCalls[0].Name != "calculator" {
		t.Errorf("expected tool name 'calculator', got '%s'", messages[0].ToolCalls[0].Name)
	}

	_ = msg // Use msg to avoid unused variable warning
}

// Test Message with tool call ID
func TestStoreMessageWithToolCallID(t *testing.T) {
	store, _ := NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Test")

	_, err := store.AddMessage(context.Background(), conv.ID, Message{
		Role:       "tool",
		Content:    "4",
		ToolCallID: "call_1",
	})
	if err != nil {
		t.Fatalf("failed to add tool response: %v", err)
	}

	messages, _ := store.GetMessages(context.Background(), conv.ID, 10, 0)
	if messages[0].ToolCallID != "call_1" {
		t.Errorf("expected tool_call_id 'call_1', got '%s'", messages[0].ToolCallID)
	}
}

// Test Delete messages when conversation deleted
func TestStoreDeleteConversationCascade(t *testing.T) {
	store, _ := NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Test")
	store.AddMessage(context.Background(), conv.ID, Message{Role: "user", Content: "Hello"})
	store.AddMessage(context.Background(), conv.ID, Message{Role: "assistant", Content: "Hi"})

	store.DeleteConversation(context.Background(), conv.ID)

	messages, _ := store.GetMessages(context.Background(), conv.ID, 10, 0)
	if len(messages) != 0 {
		t.Errorf("expected 0 messages after cascade delete, got %d", len(messages))
	}
}

// Test Search conversations
func TestStoreSearchConversations(t *testing.T) {
	store, _ := NewStore(":memory:")
	defer store.Close()

	store.CreateConversation(context.Background(), "Python Programming")
	store.CreateConversation(context.Background(), "Go Development")
	store.CreateConversation(context.Background(), "Python Tips")

	results, err := store.SearchConversations(context.Background(), "Python", 10)
	if err != nil {
		t.Fatalf("failed to search: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

// Test Conversation model
func TestConversationModel(t *testing.T) {
	conv := Conversation{
		ID:        "conv-123",
		Title:     "Test Conversation",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if conv.ID != "conv-123" {
		t.Errorf("expected ID 'conv-123', got '%s'", conv.ID)
	}
}

// Test Message model
func TestMessageModel(t *testing.T) {
	msg := Message{
		ID:             "msg-123",
		ConversationID: "conv-123",
		Role:           "user",
		Content:        "Hello",
		CreatedAt:      time.Now(),
	}

	if msg.ID != "msg-123" {
		t.Errorf("expected ID 'msg-123', got '%s'", msg.ID)
	}
}

// Test Store Close
func TestStoreClose(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, _ := NewStore(dbPath)
	err := store.Close()
	if err != nil {
		t.Errorf("failed to close store: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("expected database file to exist")
	}
}

// Test concurrent access
func TestStoreConcurrentAccess(t *testing.T) {
	store, _ := NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Test")

	// Concurrent writes
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(i int) {
			store.AddMessage(context.Background(), conv.ID, Message{
				Role:    "user",
				Content: "Message",
			})
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	messages, _ := store.GetMessages(context.Background(), conv.ID, 100, 0)
	if len(messages) != 10 {
		t.Errorf("expected 10 messages, got %d", len(messages))
	}
}
