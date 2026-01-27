package builtin

import (
	"context"
	"testing"
)

func TestTimer(t *testing.T) {
	timer := NewTimer()

	// Test manifest
	if timer.Manifest().ID != "timer" {
		t.Errorf("expected ID 'timer', got '%s'", timer.Manifest().ID)
	}

	// Test start action
	result, err := timer.Execute(context.Background(), map[string]any{
		"action":   "start",
		"duration": "5s",
		"name":     "test-timer",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}

	// Test status action
	result, err = timer.Execute(context.Background(), map[string]any{
		"action": "status",
		"name":   "test-timer",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.Data.(map[string]any)["status"] != "running" {
		t.Errorf("expected status 'running', got '%v'", result.Data.(map[string]any)["status"])
	}

	// Test stop action
	result, err = timer.Execute(context.Background(), map[string]any{
		"action": "stop",
		"name":   "test-timer",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.Data.(map[string]any)["status"] != "stopped" {
		t.Errorf("expected status 'stopped', got '%v'", result.Data.(map[string]any)["status"])
	}
}

func TestReminders(t *testing.T) {
	reminders := NewReminders()

	// Test add reminder
	result, err := reminders.Execute(context.Background(), map[string]any{
		"action":  "add",
		"message": "Test reminder",
		"time":    "1h",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}

	// Test list reminders
	result, err = reminders.Execute(context.Background(), map[string]any{
		"action": "list",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.Data.(map[string]any)["count"].(int) != 1 {
		t.Errorf("expected 1 reminder, got %v", result.Data.(map[string]any)["count"])
	}

	// Test clear reminders
	result, err = reminders.Execute(context.Background(), map[string]any{
		"action": "clear",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.Data.(map[string]any)["cleared"].(int) != 1 {
		t.Errorf("expected 1 cleared, got %v", result.Data.(map[string]any)["cleared"])
	}
}

func TestNotes(t *testing.T) {
	notes := NewNotes()

	// Test create note
	result, err := notes.Execute(context.Background(), map[string]any{
		"action":  "create",
		"title":   "Test Note",
		"content": "This is a test note",
		"tags":    []interface{}{"test", "example"},
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}

	note := result.Data.(map[string]any)["note"].(*NoteItem)
	noteID := note.ID

	// Test read note
	result, err = notes.Execute(context.Background(), map[string]any{
		"action": "read",
		"id":     noteID,
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	readNote := result.Data.(map[string]any)["note"].(*NoteItem)
	if readNote.Title != "Test Note" {
		t.Errorf("expected title 'Test Note', got '%s'", readNote.Title)
	}

	// Test search notes
	result, err = notes.Execute(context.Background(), map[string]any{
		"action": "search",
		"query":  "test",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.Data.(map[string]any)["count"].(int) != 1 {
		t.Errorf("expected 1 result, got %v", result.Data.(map[string]any)["count"])
	}

	// Test delete note
	result, err = notes.Execute(context.Background(), map[string]any{
		"action": "delete",
		"id":     noteID,
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.Data.(map[string]any)["deleted"] != true {
		t.Error("expected deleted to be true")
	}
}

func TestTasks(t *testing.T) {
	tasks := NewTasks()

	// Test create task
	result, err := tasks.Execute(context.Background(), map[string]any{
		"action":      "create",
		"title":       "Test Task",
		"description": "This is a test task",
		"priority":    "high",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}

	task := result.Data.(map[string]any)["task"].(*TaskItem)
	taskID := task.ID

	// Test complete task
	result, err = tasks.Execute(context.Background(), map[string]any{
		"action": "complete",
		"id":     taskID,
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.Data.(map[string]any)["completed"] != true {
		t.Error("expected completed to be true")
	}

	// Test list tasks
	result, err = tasks.Execute(context.Background(), map[string]any{
		"action": "list",
		"status": "completed",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.Data.(map[string]any)["count"].(int) != 1 {
		t.Errorf("expected 1 completed task, got %v", result.Data.(map[string]any)["count"])
	}

	// Test reopen task
	result, err = tasks.Execute(context.Background(), map[string]any{
		"action": "reopen",
		"id":     taskID,
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.Data.(map[string]any)["reopened"] != true {
		t.Error("expected reopened to be true")
	}
}

func TestUnitConverter(t *testing.T) {
	converter := NewUnitConverter()

	tests := []struct {
		value    float64
		from     string
		to       string
		expected float64
		delta    float64
	}{
		{1, "km", "m", 1000, 0.01},
		{1, "mi", "km", 1.609344, 0.001},
		{1, "kg", "lb", 2.20462, 0.001},
		{0, "c", "f", 32, 0.01},
		{100, "c", "f", 212, 0.01},
		{32, "f", "c", 0, 0.01},
	}

	for _, tt := range tests {
		result, err := converter.Execute(context.Background(), map[string]any{
			"value": tt.value,
			"from":  tt.from,
			"to":    tt.to,
		})
		if err != nil {
			t.Errorf("unexpected error for %v %s to %s: %v", tt.value, tt.from, tt.to, err)
			continue
		}
		if !result.Success {
			t.Errorf("expected success for %v %s to %s", tt.value, tt.from, tt.to)
			continue
		}
		got := result.Data.(map[string]any)["result"].(float64)
		if got < tt.expected-tt.delta || got > tt.expected+tt.delta {
			t.Errorf("convert %v %s to %s: expected %v, got %v", tt.value, tt.from, tt.to, tt.expected, got)
		}
	}
}

func TestTranslate(t *testing.T) {
	translate := NewTranslate()

	result, err := translate.Execute(context.Background(), map[string]any{
		"text": "Hello",
		"from": "en",
		"to":   "zh",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if result.Data.(map[string]any)["to"] != "zh" {
		t.Errorf("expected to 'zh', got '%v'", result.Data.(map[string]any)["to"])
	}
}

func TestNotifications(t *testing.T) {
	notifications := NewNotifications()

	// Test send notification
	result, err := notifications.Execute(context.Background(), map[string]any{
		"action":   "send",
		"title":    "Test",
		"message":  "Test notification",
		"priority": "high",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.Data.(map[string]any)["sent"] != true {
		t.Error("expected sent to be true")
	}

	// Test list notifications
	result, err = notifications.Execute(context.Background(), map[string]any{
		"action": "list",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.Data.(map[string]any)["count"].(int) != 1 {
		t.Errorf("expected 1 notification, got %v", result.Data.(map[string]any)["count"])
	}

	// Test clear notifications
	result, err = notifications.Execute(context.Background(), map[string]any{
		"action": "clear",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.Data.(map[string]any)["cleared"].(int) != 1 {
		t.Errorf("expected 1 cleared, got %v", result.Data.(map[string]any)["cleared"])
	}
}

func TestNetwork(t *testing.T) {
	network := NewNetwork()

	// Test info action
	result, err := network.Execute(context.Background(), map[string]any{
		"action": "info",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if _, ok := result.Data.(map[string]any)["hostname"]; !ok {
		t.Error("expected hostname in result")
	}
}

func TestProcesses(t *testing.T) {
	processes := NewProcesses()

	// Test list action
	result, err := processes.Execute(context.Background(), map[string]any{
		"action": "list",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
}

func TestFiles(t *testing.T) {
	files := NewFiles(t.TempDir())

	// Test write file
	result, err := files.Execute(context.Background(), map[string]any{
		"action":  "write",
		"path":    "test.txt",
		"content": "Hello, World!",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}

	// Test read file
	result, err = files.Execute(context.Background(), map[string]any{
		"action": "read",
		"path":   "test.txt",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.Data.(map[string]any)["content"] != "Hello, World!" {
		t.Errorf("expected content 'Hello, World!', got '%v'", result.Data.(map[string]any)["content"])
	}

	// Test list files
	result, err = files.Execute(context.Background(), map[string]any{
		"action": "list",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.Data.(map[string]any)["count"].(int) != 1 {
		t.Errorf("expected 1 file, got %v", result.Data.(map[string]any)["count"])
	}

	// Test file info
	result, err = files.Execute(context.Background(), map[string]any{
		"action": "info",
		"path":   "test.txt",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.Data.(map[string]any)["name"] != "test.txt" {
		t.Errorf("expected name 'test.txt', got '%v'", result.Data.(map[string]any)["name"])
	}

	// Test delete file
	result, err = files.Execute(context.Background(), map[string]any{
		"action": "delete",
		"path":   "test.txt",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.Data.(map[string]any)["deleted"] != true {
		t.Error("expected deleted to be true")
	}
}
