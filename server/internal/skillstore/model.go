// Package skillstore provides skill storage and synchronization with ClawHub.
package skillstore

import (
	"time"
)

// Skill represents a skill stored in the local database.
type Skill struct {
	ID          string    `json:"id" db:"id"`                     // Unique identifier (slug)
	Name        string    `json:"name" db:"name"`                 // Display name
	Version     string    `json:"version" db:"version"`           // Latest version
	Summary     string    `json:"summary" db:"summary"`           // Short description
	Description string    `json:"description" db:"description"`   // Full description (from detail page)
	Author      string    `json:"author" db:"author"`             // Author name
	Category    string    `json:"category" db:"category"`         // Category
	Tags        string    `json:"tags" db:"tags"`                 // Comma-separated tags
	SourceID    string    `json:"source_id" db:"source_id"`       // Source identifier (e.g., "clawhub")
	SourceName  string    `json:"source_name" db:"source_name"`   // Source display name
	Homepage    string    `json:"homepage" db:"homepage"`         // Homepage URL
	DownloadURL string    `json:"download_url" db:"download_url"` // Download URL
	Stars       int       `json:"stars" db:"stars"`               // Star count
	Downloads   int       `json:"downloads" db:"downloads"`       // Download count
	Reviews     int       `json:"reviews" db:"reviews"`           // Review/comment count
	Rating      float64   `json:"rating" db:"rating"`             // Average rating (0-5)
	Versions    int       `json:"versions" db:"versions"`         // Number of versions
	Changelog   string    `json:"changelog" db:"changelog"`       // Latest changelog
	Readme      string    `json:"readme,omitempty" db:"readme"`   // Full README/homepage content
	DedupKey    string    `json:"dedup_key" db:"dedup_key"`       // Deduplication key (name:author normalized)
	Installed   bool      `json:"installed" db:"installed"`       // Whether installed locally
	Enabled     bool      `json:"enabled" db:"enabled"`           // Whether enabled
	CreatedAt   time.Time `json:"created_at" db:"created_at"`     // First seen timestamp
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`     // Last update timestamp
	SyncedAt    time.Time `json:"synced_at" db:"synced_at"`       // Last sync timestamp
	// Full-text search content (combined searchable text)
	SearchContent string `json:"-" db:"search_content"`
}

// SyncStatus represents the synchronization status.
type SyncStatus struct {
	ID            int64     `json:"id" db:"id"`
	SourceID      string    `json:"source_id" db:"source_id"`
	LastSyncAt    time.Time `json:"last_sync_at" db:"last_sync_at"`
	SkillCount    int       `json:"skill_count" db:"skill_count"`
	SyncDuration  int64     `json:"sync_duration_ms" db:"sync_duration_ms"`
	Status        string    `json:"status" db:"status"` // "success", "failed", "in_progress"
	ErrorMessage  string    `json:"error_message,omitempty" db:"error_message"`
	NextSyncAt    time.Time `json:"next_sync_at" db:"next_sync_at"`
}

// SearchResult represents a skill search result with relevance score.
type SearchResult struct {
	Skill
	Score float64 `json:"score"` // Relevance score
}

// SearchOptions represents search options.
type SearchOptions struct {
	Query      string   `json:"query"`       // Search query
	Categories []string `json:"categories"`  // Filter by categories
	Sources    []string `json:"sources"`     // Filter by sources
	MinStars   int      `json:"min_stars"`   // Minimum stars
	SortBy     string   `json:"sort_by"`     // Sort field: "relevance", "stars", "downloads", "updated"
	SortOrder  string   `json:"sort_order"`  // "asc" or "desc"
	Page       int      `json:"page"`        // Page number (1-based)
	PageSize   int      `json:"page_size"`   // Items per page
}

// DefaultSearchOptions returns default search options.
func DefaultSearchOptions() SearchOptions {
	return SearchOptions{
		SortBy:    "relevance",
		SortOrder: "desc",
		Page:      1,
		PageSize:  24,
	}
}

// SearchResponse represents a paginated search response.
type SearchResponse struct {
	Skills     []SearchResult `json:"skills"`
	Total      int64          `json:"total"`
	Page       int            `json:"page"`
	PageSize   int            `json:"page_size"`
	TotalPages int            `json:"total_pages"`
}
