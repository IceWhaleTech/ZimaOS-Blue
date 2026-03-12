package contextpack

import "time"

type EntryType string

const (
	EntryTypeDoc   EntryType = "doc"
	EntryTypeSkill EntryType = "skill"
)

type SourceTrust string

const (
	SourceTrustOfficial   SourceTrust = "official"
	SourceTrustMaintainer SourceTrust = "maintainer"
	SourceTrustCommunity  SourceTrust = "community"
	SourceTrustLocal      SourceTrust = "local"
)

type Entry struct {
	ID          string      `json:"id"`
	Type        EntryType   `json:"type"`
	Description string      `json:"description,omitempty"`
	SourceTrust SourceTrust `json:"source_trust,omitempty"`
	Tags        []string    `json:"tags,omitempty"`
	Languages   []string    `json:"languages,omitempty"`
	Versions    []string    `json:"versions,omitempty"`
	Revision    string      `json:"revision,omitempty"`
	UpdatedOn   string      `json:"updated_on,omitempty"`
	Author      string      `json:"author,omitempty"`
	RootPath    string      `json:"root_path,omitempty"`
	Variants    []Variant   `json:"variants,omitempty"`
}

type Variant struct {
	Language       string   `json:"language,omitempty"`
	Version        string   `json:"version,omitempty"`
	PrimaryFile    string   `json:"primary_file"`
	ReferenceFiles []string `json:"reference_files,omitempty"`
}

type SearchOptions struct {
	Query         string
	Language      string
	Version       string
	SelectedSkill string
	Limit         int
}

type SearchResult struct {
	Entry   Entry    `json:"entry"`
	Variant Variant  `json:"variant"`
	Score   float64  `json:"score"`
	Reasons []string `json:"reasons,omitempty"`
}

type GetOptions struct {
	Language string
	Version  string
	File     string
	Full     bool
}

type FetchResult struct {
	Entry   Entry         `json:"entry"`
	Variant Variant       `json:"variant"`
	Files   []FetchedFile `json:"files"`
}

type FetchedFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	SHA256  string `json:"sha256"`
	Tokens  int    `json:"tokens"`
}

type ValidationIssue struct {
	ID      string `json:"id,omitempty"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
}

type Annotation struct {
	TenantID  string    `json:"tenant_id,omitempty"`
	UserID    string    `json:"user_id,omitempty"`
	EntryID   string    `json:"entry_id"`
	Language  string    `json:"language,omitempty"`
	Version   string    `json:"version,omitempty"`
	File      string    `json:"file,omitempty"`
	Note      string    `json:"note"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AnnotationFilter struct {
	TenantID string
	UserID   string
	EntryID  string
	Language string
	Version  string
	File     string
}

type SelectionSet struct {
	Query         string            `json:"query,omitempty"`
	SelectedSkill string            `json:"selected_skill,omitempty"`
	Files         []SelectedFile    `json:"files,omitempty"`
	TotalTokens   int               `json:"total_tokens,omitempty"`
	Truncated     bool              `json:"truncated,omitempty"`
	GeneratedAt   time.Time         `json:"generated_at"`
	Issues        []ValidationIssue `json:"issues,omitempty"`
}

type SelectedFile struct {
	EntryID       string      `json:"entry_id"`
	Type          EntryType   `json:"type"`
	SourceTrust   SourceTrust `json:"source_trust,omitempty"`
	Language      string      `json:"language,omitempty"`
	Version       string      `json:"version,omitempty"`
	File          string      `json:"file"`
	SHA256        string      `json:"sha256"`
	Tokens        int         `json:"tokens"`
	Annotated     bool        `json:"annotated,omitempty"`
	AnnotationIDs []string    `json:"annotation_ids,omitempty"`
	Truncated     bool        `json:"truncated,omitempty"`
	PromptBlock   string      `json:"-"`
}

func (s *SelectionSet) Clone() *SelectionSet {
	if s == nil {
		return nil
	}
	out := *s
	out.Files = append([]SelectedFile(nil), s.Files...)
	out.Issues = append([]ValidationIssue(nil), s.Issues...)
	return &out
}

func (s *SelectionSet) AuditMetadata() []map[string]interface{} {
	if s == nil || len(s.Files) == 0 {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(s.Files))
	for _, file := range s.Files {
		out = append(out, map[string]interface{}{
			"entry_id":         file.EntryID,
			"type":             string(file.Type),
			"source_trust":     string(file.SourceTrust),
			"language":         file.Language,
			"version":          file.Version,
			"file":             file.File,
			"sha256":           file.SHA256,
			"tokens":           file.Tokens,
			"annotated":        file.Annotated,
			"annotation_count": len(file.AnnotationIDs),
			"truncated":        file.Truncated,
		})
	}
	return out
}
