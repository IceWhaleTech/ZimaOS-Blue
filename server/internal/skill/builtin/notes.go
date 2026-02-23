package builtin

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// NoteItem represents a single note
type NoteItem struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Tags      []string  `json:"tags,omitempty"`
	Created   time.Time `json:"created"`
	Updated   time.Time `json:"updated"`
}

// Notes is a built-in notes skill
type Notes struct {
	manifest *skill.Manifest
	notes    map[string]*NoteItem
	mu       sync.RWMutex
	counter  int
}

// NewNotes creates a new notes skill
func NewNotes() *Notes {
	return &Notes{
		manifest: &skill.Manifest{
			ID:          "notes",
			Name:        "Notes",
			Version:     "1.0.0",
			Description: "Take and manage notes",
			Category:    "productivity",
			Icon:        "notes",
			Tags:        []string{"notes", "memo", "text", "productivity"},
			Inputs: []skill.Parameter{
				{
					Name:        "action",
					Type:        "string",
					Description: "Action to perform: create, read, update, delete, list, search",
					Required:    true,
				},
				{
					Name:        "id",
					Type:        "string",
					Description: "Note ID (required for read, update, delete)",
					Required:    false,
				},
				{
					Name:        "title",
					Type:        "string",
					Description: "Note title",
					Required:    false,
				},
				{
					Name:        "content",
					Type:        "string",
					Description: "Note content",
					Required:    false,
				},
				{
					Name:        "tags",
					Type:        "array",
					Description: "Note tags for categorization",
					Required:    false,
				},
				{
					Name:        "query",
					Type:        "string",
					Description: "Search query (for search action)",
					Required:    false,
				},
			},
			Outputs: []skill.Parameter{
				{
					Name:        "note",
					Type:        "object",
					Description: "Note object",
				},
				{
					Name:        "notes",
					Type:        "array",
					Description: "List of notes",
				},
			},
		},
		notes: make(map[string]*NoteItem),
	}
}

// Manifest returns the skill manifest
func (n *Notes) Manifest() *skill.Manifest {
	return n.manifest
}

// Validate validates the input parameters
func (n *Notes) Validate(input map[string]any) error {
	action, ok := input["action"]
	if !ok {
		return fmt.Errorf("action is required")
	}

	actionStr, ok := action.(string)
	if !ok {
		return fmt.Errorf("action must be a string")
	}

	validActions := map[string]bool{
		"create": true, "read": true, "update": true,
		"delete": true, "list": true, "search": true,
	}
	if !validActions[actionStr] {
		return fmt.Errorf("invalid action: %s", actionStr)
	}

	switch actionStr {
	case "create":
		if _, ok := input["content"]; !ok {
			return fmt.Errorf("content is required for create action")
		}
	case "read", "delete":
		if _, ok := input["id"]; !ok {
			return fmt.Errorf("id is required for %s action", actionStr)
		}
	case "update":
		if _, ok := input["id"]; !ok {
			return fmt.Errorf("id is required for update action")
		}
	case "search":
		if _, ok := input["query"]; !ok {
			return fmt.Errorf("query is required for search action")
		}
	}

	return nil
}

// Execute executes the notes skill
func (n *Notes) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	action := input["action"].(string)

	switch action {
	case "create":
		return n.createNote(input)
	case "read":
		return n.readNote(input)
	case "update":
		return n.updateNote(input)
	case "delete":
		return n.deleteNote(input)
	case "list":
		return n.listNotes()
	case "search":
		return n.searchNotes(input)
	}

	return skill.NewErrorResult(fmt.Errorf("unknown action: %s", action)), nil
}

func (n *Notes) createNote(input map[string]any) (*skill.Result, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.counter++
	id := fmt.Sprintf("note-%d", n.counter)
	now := timeutil.NowTime()

	note := &NoteItem{
		ID:      id,
		Content: input["content"].(string),
		Created: now,
		Updated: now,
	}

	if title, ok := input["title"].(string); ok {
		note.Title = title
	} else {
		// Use first 50 chars of content as title
		content := note.Content
		if len(content) > 50 {
			content = content[:50] + "..."
		}
		note.Title = content
	}

	if tags, ok := input["tags"].([]interface{}); ok {
		for _, t := range tags {
			if tag, ok := t.(string); ok {
				note.Tags = append(note.Tags, tag)
			}
		}
	}

	n.notes[id] = note

	return skill.NewResult(map[string]any{
		"note":    note,
		"message": fmt.Sprintf("Note '%s' created", id),
	}), nil
}

func (n *Notes) readNote(input map[string]any) (*skill.Result, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	id := input["id"].(string)

	if note, ok := n.notes[id]; ok {
		return skill.NewResult(map[string]any{
			"note": note,
		}), nil
	}

	return skill.NewResult(map[string]any{
		"note":    nil,
		"message": fmt.Sprintf("Note '%s' not found", id),
	}), nil
}

func (n *Notes) updateNote(input map[string]any) (*skill.Result, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	id := input["id"].(string)

	note, ok := n.notes[id]
	if !ok {
		return skill.NewResult(map[string]any{
			"updated": false,
			"message": fmt.Sprintf("Note '%s' not found", id),
		}), nil
	}

	if title, ok := input["title"].(string); ok {
		note.Title = title
	}
	if content, ok := input["content"].(string); ok {
		note.Content = content
	}
	if tags, ok := input["tags"].([]interface{}); ok {
		note.Tags = nil
		for _, t := range tags {
			if tag, ok := t.(string); ok {
				note.Tags = append(note.Tags, tag)
			}
		}
	}
	note.Updated = timeutil.NowTime()

	return skill.NewResult(map[string]any{
		"updated": true,
		"note":    note,
		"message": fmt.Sprintf("Note '%s' updated", id),
	}), nil
}

func (n *Notes) deleteNote(input map[string]any) (*skill.Result, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	id := input["id"].(string)

	if note, ok := n.notes[id]; ok {
		delete(n.notes, id)
		return skill.NewResult(map[string]any{
			"deleted": true,
			"note":    note,
			"message": fmt.Sprintf("Note '%s' deleted", id),
		}), nil
	}

	return skill.NewResult(map[string]any{
		"deleted": false,
		"message": fmt.Sprintf("Note '%s' not found", id),
	}), nil
}

func (n *Notes) listNotes() (*skill.Result, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	notes := make([]*NoteItem, 0, len(n.notes))
	for _, note := range n.notes {
		notes = append(notes, note)
	}

	return skill.NewResult(map[string]any{
		"notes": notes,
		"count": len(notes),
	}), nil
}

func (n *Notes) searchNotes(input map[string]any) (*skill.Result, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	query := input["query"].(string)
	var results []*NoteItem

	for _, note := range n.notes {
		// Simple substring search in title and content
		if containsIgnoreCase(note.Title, query) || containsIgnoreCase(note.Content, query) {
			results = append(results, note)
			continue
		}
		// Search in tags
		for _, tag := range note.Tags {
			if containsIgnoreCase(tag, query) {
				results = append(results, note)
				break
			}
		}
	}

	return skill.NewResult(map[string]any{
		"notes":   results,
		"count":   len(results),
		"query":   query,
		"message": fmt.Sprintf("Found %d notes matching '%s'", len(results), query),
	}), nil
}

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(substr) == 0 ||
		findIgnoreCase(s, substr) >= 0)
}

func findIgnoreCase(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalIgnoreCase(s[i:i+len(substr)], substr) {
			return i
		}
	}
	return -1
}

func equalIgnoreCase(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

