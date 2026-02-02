# PRD-0.10.14: Skill Store Redesign

## Overview

This PRD describes a comprehensive redesign of the Skill Store to fetch skills from ClawdHub on startup, store them in a local database with deduplication, and provide full-text search capabilities across all skill metadata.

## Goals

1. Fetch skill list from ClawdHub on application startup
2. Implement intelligent deduplication based on name + identifier combination
3. Store skills in local SQLite database with daily sync limit
4. Support full-text search across all queryable fields
5. Display download counts, review counts, and homepage information

---

## 1. ClawdHub Integration

### 1.1 Startup Fetch Behavior

**Description**: On application startup, fetch the complete skill list from ClawdHub API.

**Requirements**:
- Fetch skills from ClawdHub API endpoint on server startup
- Run fetch in background goroutine to avoid blocking startup
- Implement rate limiting: maximum one fetch per 24 hours
- Store last fetch timestamp in database
- Skip fetch if last successful fetch was within 24 hours

**Fetch Flow**:
```
Server Startup
    │
    ▼
Check last_fetch_time in DB
    │
    ├── If > 24h ago OR never fetched
    │       │
    │       ▼
    │   Fetch from ClawdHub API
    │       │
    │       ▼
    │   Deduplicate skills
    │       │
    │       ▼
    │   Upsert to local DB
    │       │
    │       ▼
    │   Update last_fetch_time
    │
    └── If < 24h ago
            │
            ▼
        Skip fetch, use cached data
```

### 1.2 ClawdHub API Integration

**Endpoint**: `GET https://api.clawdhub.com/v1/skills`

**Expected Response**:
```json
{
  "skills": [
    {
      "id": "unique-skill-id",
      "name": "skill-name",
      "author": "author-name",
      "description": "Skill description",
      "category": "productivity",
      "tags": ["automation", "cli"],
      "version": "1.0.0",
      "source_url": "https://github.com/...",
      "homepage": "https://example.com/skill",
      "downloads": 1500,
      "reviews": 42,
      "rating": 4.5,
      "created_at": "2025-01-15T10:00:00Z",
      "updated_at": "2026-01-20T15:30:00Z"
    }
  ],
  "total": 500,
  "page": 1,
  "per_page": 100
}
```

**Pagination Handling**:
- Fetch all pages until `total` is reached
- Implement exponential backoff on rate limit errors
- Maximum 3 retry attempts per page

---

## 2. Deduplication Strategy

### 2.1 Deduplication Key

**Primary Key**: `name` + `author` combination

**Rationale**:
- Same skill name from different authors should be treated as different skills
- Same author may publish updated versions of the same skill
- Using name alone would incorrectly merge different skills with common names

**Deduplication Algorithm**:
```
For each skill from ClawdHub:
    dedup_key = normalize(skill.name) + ":" + normalize(skill.author)

    IF dedup_key exists in local DB:
        IF remote.updated_at > local.updated_at:
            UPDATE local record with remote data
        ELSE:
            SKIP (local is newer or same)
    ELSE:
        INSERT new record
```

### 2.2 Normalization Rules

- Convert name to lowercase
- Trim whitespace
- Replace multiple spaces with single space
- Remove special characters except hyphen and underscore

**Example**:
```
"My Awesome Skill" + "John Doe" → "my-awesome-skill:john-doe"
"  My  Awesome  Skill  " + "john doe" → "my-awesome-skill:john-doe"
```

---

## 3. Database Schema

### 3.1 New Table: clawdhub_skills

```sql
CREATE TABLE clawdhub_skills (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    name_normalized TEXT NOT NULL,
    author TEXT NOT NULL,
    author_normalized TEXT NOT NULL,
    dedup_key TEXT NOT NULL UNIQUE,
    description TEXT,
    category TEXT,
    tags TEXT,                    -- JSON array stored as text
    version TEXT,
    source_url TEXT,
    homepage TEXT,
    downloads INTEGER DEFAULT 0,
    reviews INTEGER DEFAULT 0,
    rating REAL DEFAULT 0.0,
    readme TEXT,                  -- Full README/homepage content for search
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    fetched_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for search performance
CREATE INDEX idx_clawdhub_skills_name ON clawdhub_skills(name_normalized);
CREATE INDEX idx_clawdhub_skills_author ON clawdhub_skills(author_normalized);
CREATE INDEX idx_clawdhub_skills_category ON clawdhub_skills(category);
CREATE INDEX idx_clawdhub_skills_dedup ON clawdhub_skills(dedup_key);
CREATE INDEX idx_clawdhub_skills_downloads ON clawdhub_skills(downloads DESC);
CREATE INDEX idx_clawdhub_skills_rating ON clawdhub_skills(rating DESC);
```

### 3.2 Full-Text Search Table

```sql
-- FTS5 virtual table for full-text search
CREATE VIRTUAL TABLE clawdhub_skills_fts USING fts5(
    id,
    name,
    author,
    description,
    category,
    tags,
    readme,
    content='clawdhub_skills',
    content_rowid='rowid'
);

-- Triggers to keep FTS in sync
CREATE TRIGGER clawdhub_skills_ai AFTER INSERT ON clawdhub_skills BEGIN
    INSERT INTO clawdhub_skills_fts(rowid, id, name, author, description, category, tags, readme)
    VALUES (new.rowid, new.id, new.name, new.author, new.description, new.category, new.tags, new.readme);
END;

CREATE TRIGGER clawdhub_skills_ad AFTER DELETE ON clawdhub_skills BEGIN
    INSERT INTO clawdhub_skills_fts(clawdhub_skills_fts, rowid, id, name, author, description, category, tags, readme)
    VALUES ('delete', old.rowid, old.id, old.name, old.author, old.description, old.category, old.tags, old.readme);
END;

CREATE TRIGGER clawdhub_skills_au AFTER UPDATE ON clawdhub_skills BEGIN
    INSERT INTO clawdhub_skills_fts(clawdhub_skills_fts, rowid, id, name, author, description, category, tags, readme)
    VALUES ('delete', old.rowid, old.id, old.name, old.author, old.description, old.category, old.tags, old.readme);
    INSERT INTO clawdhub_skills_fts(rowid, id, name, author, description, category, tags, readme)
    VALUES (new.rowid, new.id, new.name, new.author, new.description, new.category, new.tags, new.readme);
END;
```

### 3.3 Sync Metadata Table

```sql
CREATE TABLE skill_sync_metadata (
    id INTEGER PRIMARY KEY,
    source TEXT NOT NULL,         -- 'clawdhub', 'github', etc.
    last_fetch_at TIMESTAMP,
    last_success_at TIMESTAMP,
    total_skills INTEGER DEFAULT 0,
    fetch_duration_ms INTEGER,
    error_message TEXT
);
```

---

## 4. Full-Text Search

### 4.1 Search Capabilities

**Searchable Fields**:
- `name` - Skill name
- `author` - Author name
- `description` - Skill description
- `category` - Category name
- `tags` - All tags (JSON array)
- `readme` - Full homepage/README content

**Search Features**:
- Prefix matching: `auto*` matches "automation", "automate"
- Phrase matching: `"code review"` matches exact phrase
- Boolean operators: `git AND review`, `git OR svn`, `git NOT svn`
- Field-specific search: `name:commit` searches only name field

### 4.2 Search API

**Endpoint**: `GET /api/skill-store/search`

**Query Parameters**:
| Parameter | Type | Description |
|-----------|------|-------------|
| `q` | string | Full-text search query |
| `category` | string | Filter by category |
| `author` | string | Filter by author |
| `sort` | string | Sort field: `downloads`, `rating`, `updated`, `name` |
| `order` | string | Sort order: `asc`, `desc` (default: `desc`) |
| `page` | int | Page number (default: 1) |
| `per_page` | int | Results per page (default: 20, max: 100) |

**Example Requests**:
```
GET /api/skill-store/search?q=git+commit
GET /api/skill-store/search?q=automation&category=productivity
GET /api/skill-store/search?q=name:review&sort=downloads
GET /api/skill-store/search?author=anthropic&sort=rating&order=desc
```

**Response**:
```json
{
  "skills": [
    {
      "id": "skill-id",
      "name": "Git Commit Helper",
      "author": "developer",
      "description": "Helps write better commit messages",
      "category": "development",
      "tags": ["git", "commit", "automation"],
      "version": "2.1.0",
      "homepage": "https://example.com/skill",
      "downloads": 5000,
      "reviews": 120,
      "rating": 4.8,
      "updated_at": "2026-01-15T10:00:00Z",
      "highlight": {
        "name": "<mark>Git</mark> <mark>Commit</mark> Helper",
        "description": "Helps write better <mark>commit</mark> messages"
      }
    }
  ],
  "total": 45,
  "page": 1,
  "per_page": 20,
  "query": "git commit"
}
```

---

## 5. Skill Homepage Content

### 5.1 Homepage Fetching

**Description**: Fetch and store skill homepage/README content for enhanced search.

**Requirements**:
- Fetch homepage content when skill is first added or updated
- Store content in `readme` field
- Strip HTML tags, keep plain text for search
- Limit content to 50KB per skill
- Fetch in background, don't block main sync

**Fetch Priority**:
1. `source_url` if points to raw markdown (GitHub raw)
2. `homepage` URL content
3. Skip if neither available

### 5.2 Content Processing

```
Fetch URL content
    │
    ▼
Detect content type
    │
    ├── Markdown → Store as-is
    │
    ├── HTML → Extract text, strip tags
    │
    └── Other → Skip

    │
    ▼
Truncate to 50KB
    │
    ▼
Store in readme field
```

---

## 6. API Endpoints

### 6.1 New Endpoints

```
GET  /api/skill-store/search          - Full-text search with filters
GET  /api/skill-store/categories      - List all categories with counts
GET  /api/skill-store/popular         - Top skills by downloads
GET  /api/skill-store/recent          - Recently updated skills
GET  /api/skill-store/skill/:id       - Get skill details with full readme
POST /api/skill-store/sync            - Trigger manual sync (admin only)
GET  /api/skill-store/sync/status     - Get last sync status
```

### 6.2 Modified Endpoints

```
GET  /api/skill-store/browse          - Now queries local DB instead of remote API
```

---

## 7. Data Model

### 7.1 ClawdHubSkill Struct

```go
type ClawdHubSkill struct {
    ID               string    `json:"id"`
    Name             string    `json:"name"`
    NameNormalized   string    `json:"name_normalized"`
    Author           string    `json:"author"`
    AuthorNormalized string    `json:"author_normalized"`
    DedupKey         string    `json:"dedup_key"`
    Description      string    `json:"description"`
    Category         string    `json:"category"`
    Tags             []string  `json:"tags"`
    Version          string    `json:"version"`
    SourceURL        string    `json:"source_url"`
    Homepage         string    `json:"homepage"`
    Downloads        int       `json:"downloads"`
    Reviews          int       `json:"reviews"`
    Rating           float64   `json:"rating"`
    Readme           string    `json:"readme,omitempty"`
    CreatedAt        time.Time `json:"created_at"`
    UpdatedAt        time.Time `json:"updated_at"`
    FetchedAt        time.Time `json:"fetched_at"`
}
```

### 7.2 SearchResult Struct

```go
type SkillSearchResult struct {
    Skills    []*ClawdHubSkill `json:"skills"`
    Total     int              `json:"total"`
    Page      int              `json:"page"`
    PerPage   int              `json:"per_page"`
    Query     string           `json:"query"`
    Highlight map[string]map[string]string `json:"highlight,omitempty"`
}
```

---

## 8. Implementation Tasks

### Phase 1: Database Schema
- [ ] Create `clawdhub_skills` table
- [ ] Create FTS5 virtual table and triggers
- [ ] Create `skill_sync_metadata` table
- [ ] Add database migration

### Phase 2: ClawdHub Sync
- [ ] Implement ClawdHub API client
- [ ] Implement pagination handling
- [ ] Implement deduplication logic
- [ ] Implement 24-hour rate limiting
- [ ] Add startup sync goroutine

### Phase 3: Full-Text Search
- [ ] Implement FTS5 search queries
- [ ] Add search highlighting
- [ ] Implement filter combinations
- [ ] Add sorting options

### Phase 4: Homepage Content
- [ ] Implement homepage content fetcher
- [ ] Add HTML to text conversion
- [ ] Implement content truncation
- [ ] Add background fetch queue

### Phase 5: API & Frontend
- [ ] Implement new API endpoints
- [ ] Update frontend skill store view
- [ ] Add search UI with filters
- [ ] Display download/review counts

---

## 9. Success Metrics

1. **Sync Reliability**: 99% successful daily syncs
2. **Search Performance**: < 100ms for full-text queries
3. **Deduplication Accuracy**: 0 duplicate skills in database
4. **Data Freshness**: Skills updated within 24 hours of ClawdHub changes

---

## 10. Technical Notes

### Files to Create

**Backend**:
- `server/internal/skillstore/clawdhub.go` - ClawdHub API client
- `server/internal/skillstore/sync.go` - Sync logic and scheduler
- `server/internal/skillstore/search.go` - FTS5 search implementation
- `server/internal/skillstore/dedup.go` - Deduplication utilities

**Frontend**:
- `web/src/api/skillStore.ts` - Updated API client
- `web/src/components/skill-store/SkillSearch.vue` - Search component
- `web/src/components/skill-store/SkillCard.vue` - Skill display card

### Files to Modify

**Backend**:
- `server/internal/server/skill_handler.go` - Add new endpoints
- `server/internal/server/server.go` - Initialize sync on startup
- `server/internal/memory/sqlite.go` - Add new tables

**Frontend**:
- `web/src/views/PluginsView.vue` - Update skill store tab

---

## 11. Open Questions

1. Should we support multiple skill sources (GitHub, GitLab) in addition to ClawdHub?
2. What is the ClawdHub API rate limit? Need to confirm pagination limits.
3. Should we cache skill icons/thumbnails locally?
4. Do we need skill dependency resolution for related skills?

---

## References

- [ClawdHub API Documentation](https://clawdhub.com/docs/api)
- [SQLite FTS5 Documentation](https://www.sqlite.org/fts5.html)
- Previous PRD: [PRD-0.10.8-skill-store.md](./archived/PRD-0.10.8-skill-store.md)
