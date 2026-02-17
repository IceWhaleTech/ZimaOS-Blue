package memory

import (
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/google/uuid"
)

// EntryStatus represents the lifecycle state of a memory entry.
type EntryStatus string

const (
	EntryStatusActive  EntryStatus = "active"
	EntryStatusExpired EntryStatus = "expired"
	EntryStatusDeleted EntryStatus = "deleted"
)

// ContentType represents the format of memory content.
type ContentType string

const (
	ContentTypeText     ContentType = "text"
	ContentTypeJSON     ContentType = "json"
	ContentTypeMarkdown ContentType = "markdown"
)

// MemoryEntry is the enhanced memory unit with versioning, TTL, and namespace support.
// It extends the existing MemoryChunk with additional fields from the PRD.
type MemoryEntry struct {
	ID          string         `json:"id"`
	Namespace   string         `json:"namespace"`
	Content     string         `json:"content"`
	ContentType ContentType    `json:"content_type"`
	Category    string         `json:"category,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	Embedding   []float32      `json:"-"`
	Importance  float32        `json:"importance"`
	Source      string         `json:"source,omitempty"`

	// Versioning
	Version  int    `json:"version"`
	ParentID string `json:"parent_id,omitempty"`

	// Lifecycle
	Status    EntryStatus `json:"status"`
	TTL       Duration    `json:"ttl,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
	ExpiresAt *time.Time  `json:"expires_at,omitempty"`
	DeletedAt *time.Time  `json:"deleted_at,omitempty"`
}

// Duration wraps time.Duration for JSON serialization as a string (e.g. "24h", "30m").
type Duration time.Duration

// MarshalJSON implements json.Marshaler.
func (d Duration) MarshalJSON() ([]byte, error) {
	return []byte(`"` + time.Duration(d).String() + `"`), nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (d *Duration) UnmarshalJSON(b []byte) error {
	s := string(b)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	if s == "" || s == "0" {
		*d = 0
		return nil
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(dur)
	return nil
}

// NewEntryID generates a new unique entry ID.
func NewEntryID() string {
	return "mem_" + uuid.New().String()[:12]
}

// NewMemoryEntry creates a new MemoryEntry with defaults.
func NewMemoryEntry(namespace, content string) *MemoryEntry {
	now := timeutil.NowTime()
	return &MemoryEntry{
		ID:          NewEntryID(),
		Namespace:   namespace,
		Content:     content,
		ContentType: ContentTypeText,
		Version:     1,
		Status:      EntryStatusActive,
		Importance:  0.5,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// ComputeExpiresAt sets ExpiresAt based on TTL and CreatedAt.
// If TTL is 0, ExpiresAt is nil (permanent).
func (e *MemoryEntry) ComputeExpiresAt() {
	if time.Duration(e.TTL) <= 0 {
		e.ExpiresAt = nil
		return
	}
	t := e.CreatedAt.Add(time.Duration(e.TTL))
	e.ExpiresAt = &t
}

// IsExpired returns true if the entry has a TTL and it has passed.
func (e *MemoryEntry) IsExpired() bool {
	if e.ExpiresAt == nil {
		return false
	}
	return timeutil.NowNano() > e.ExpiresAt.UnixNano()
}

// NewVersion creates a new version of this entry with updated content.
// The new entry gets a new ID, incremented version, and parent_id pointing to this entry.
func (e *MemoryEntry) NewVersion(content string) *MemoryEntry {
	now := timeutil.NowTime()
	return &MemoryEntry{
		ID:          NewEntryID(),
		Namespace:   e.Namespace,
		Content:     content,
		ContentType: e.ContentType,
		Category:    e.Category,
		Tags:        e.Tags,
		Metadata:    e.Metadata,
		Importance:  e.Importance,
		Source:      e.Source,
		Version:     e.Version + 1,
		ParentID:    e.ID,
		Status:      EntryStatusActive,
		TTL:         e.TTL,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// SoftDelete marks the entry as deleted.
func (e *MemoryEntry) SoftDelete() {
	now := timeutil.NowTime()
	e.Status = EntryStatusDeleted
	e.DeletedAt = &now
	e.UpdatedAt = now
}

// SearchQuery represents a search request against the memory store.
type SearchQuery struct {
	Query         string         `json:"query"`
	Namespace     string         `json:"namespace"`
	TopK          int            `json:"top_k"`
	MinSimilarity float32        `json:"min_similarity,omitempty"`
	MinImportance float32        `json:"min_importance,omitempty"`
	Filters       *SearchFilters `json:"filters,omitempty"`
}

// SearchFilters contains optional filters for search.
type SearchFilters struct {
	Categories   []string       `json:"categories,omitempty"`
	Tags         []string       `json:"tags,omitempty"`
	Source       string         `json:"source,omitempty"`
	CreatedAfter *time.Time     `json:"created_after,omitempty"`
	CreatedBefore *time.Time    `json:"created_before,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// SearchResult represents a single search result.
type SearchResult struct {
	Entry      *MemoryEntry `json:"entry"`
	Score      float32      `json:"score"`
	MatchTypes []string     `json:"match_types,omitempty"`
}
