package contextpack

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pruner"
	"gopkg.in/yaml.v3"
)

type Registry struct {
	root       string
	refreshTTL time.Duration

	mu       sync.RWMutex
	loadedAt time.Time
	entries  []loadedEntry
	issues   []ValidationIssue
}

type loadedEntry struct {
	Entry
	searchText string
	byVariant  map[string]Variant
}

type frontmatter struct {
	ID          string   `yaml:"id"`
	Name        string   `yaml:"name"`
	Type        string   `yaml:"type"`
	Description string   `yaml:"description"`
	SourceTrust string   `yaml:"source_trust"`
	Tags        []string `yaml:"tags"`
	Languages   []string `yaml:"languages"`
	Versions    []string `yaml:"versions"`
	Revision    string   `yaml:"revision"`
	UpdatedOn   string   `yaml:"updated_on"`
	Language    string   `yaml:"language"`
	Version     string   `yaml:"version"`
	References  []string `yaml:"references"`
}

func NewRegistry(root string) *Registry {
	return &Registry{root: root, refreshTTL: 5 * time.Second}
}

func (r *Registry) SetRefreshTTL(ttl time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.refreshTTL = ttl
}

func (r *Registry) Root() string { return r.root }

func (r *Registry) Refresh(ctx context.Context) error {
	_ = ctx
	entries, issues, err := scanRegistry(r.root)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.entries = entries
	r.issues = issues
	r.loadedAt = time.Now()
	r.mu.Unlock()
	return nil
}

func (r *Registry) ensureFresh(ctx context.Context) error {
	r.mu.RLock()
	loadedAt := r.loadedAt
	ttl := r.refreshTTL
	hasEntries := len(r.entries) > 0 || len(r.issues) > 0
	r.mu.RUnlock()
	if hasEntries && ttl > 0 && time.Since(loadedAt) < ttl {
		return nil
	}
	return r.Refresh(ctx)
}

func (r *Registry) Issues(ctx context.Context) ([]ValidationIssue, error) {
	if err := r.ensureFresh(ctx); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]ValidationIssue(nil), r.issues...), nil
}

func (r *Registry) Search(ctx context.Context, opts SearchOptions) ([]SearchResult, error) {
	if err := r.ensureFresh(ctx); err != nil {
		return nil, err
	}
	query := strings.TrimSpace(opts.Query)
	if query == "" {
		return nil, nil
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 8
	}

	r.mu.RLock()
	entries := append([]loadedEntry(nil), r.entries...)
	r.mu.RUnlock()
	if len(entries) == 0 {
		return nil, nil
	}

	segments := make([]pruner.Segment, 0, len(entries))
	for i, entry := range entries {
		segments = append(segments, pruner.Segment{Content: entry.searchText, Tokens: pruner.TextTokenize(entry.searchText), StartLine: i, EndLine: i})
	}
	scored := pruner.NewBM25Scorer(1.2, 0.75).Score(query, segments)
	queryLower := strings.ToLower(query)
	queryTokens := pruner.TextTokenize(query)

	results := make([]SearchResult, 0, len(entries))
	seen := make(map[string]float64, len(entries))
	for _, score := range scored {
		idx := score.Segment.StartLine
		if idx < 0 || idx >= len(entries) {
			continue
		}
		entry := entries[idx]
		boost, reasons := scoreBoost(entry, queryLower, queryTokens, opts.SelectedSkill)
		total := score.Score + boost
		if total <= 0 {
			continue
		}
		if prev, ok := seen[entry.ID]; ok && prev >= total {
			continue
		}
		seen[entry.ID] = total
		variant := selectVariant(entry.Entry, opts.Language, opts.Version)
		results = append(results, SearchResult{Entry: entry.Entry, Variant: variant, Score: total, Reasons: reasons})
	}

	if len(results) == 0 {
		for _, entry := range entries {
			if strings.EqualFold(entry.ID, query) || strings.Contains(strings.ToLower(entry.ID), queryLower) {
				results = append(results, SearchResult{Entry: entry.Entry, Variant: selectVariant(entry.Entry, opts.Language, opts.Version), Score: 1.0, Reasons: []string{"id_match"}})
			}
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].Entry.ID < results[j].Entry.ID
		}
		return results[i].Score > results[j].Score
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func (r *Registry) Get(ctx context.Context, id string, opts GetOptions) (*FetchResult, error) {
	if err := r.ensureFresh(ctx); err != nil {
		return nil, err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("entry id is required")
	}

	r.mu.RLock()
	var entry *loadedEntry
	for i := range r.entries {
		if strings.EqualFold(r.entries[i].ID, id) {
			copied := r.entries[i]
			entry = &copied
			break
		}
		if strings.EqualFold(filepath.Base(r.entries[i].ID), id) {
			copied := r.entries[i]
			entry = &copied
		}
	}
	r.mu.RUnlock()
	if entry == nil {
		return nil, os.ErrNotExist
	}

	variant := selectVariant(entry.Entry, opts.Language, opts.Version)
	if variant.PrimaryFile == "" {
		return nil, fmt.Errorf("entry %q has no primary file", entry.ID)
	}

	paths, err := selectFilesForFetch(variant, opts)
	if err != nil {
		return nil, err
	}
	files := make([]FetchedFile, 0, len(paths))
	for _, rel := range paths {
		fullPath := filepath.Join(entry.RootPath, filepath.FromSlash(rel))
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, err
		}
		files = append(files, FetchedFile{Path: filepath.ToSlash(rel), Content: string(data), SHA256: sha256Hex(data), Tokens: pruner.EstimateTokens(string(data))})
	}
	return &FetchResult{Entry: entry.Entry, Variant: variant, Files: files}, nil
}

func scanRegistry(root string) ([]loadedEntry, []ValidationIssue, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, nil, nil
	}
	if _, err := os.Stat(root); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, nil
		}
		return nil, nil, err
	}

	authors, err := os.ReadDir(root)
	if err != nil {
		return nil, nil, err
	}

	entries := make([]loadedEntry, 0, 32)
	issues := make([]ValidationIssue, 0)
	for _, author := range authors {
		if !author.IsDir() {
			continue
		}
		authorDir := filepath.Join(root, author.Name())
		for _, bucket := range []struct {
			name      string
			entryType EntryType
			fileName  string
		}{{name: "docs", entryType: EntryTypeDoc, fileName: "DOC.md"}, {name: "skills", entryType: EntryTypeSkill, fileName: "SKILL.md"}} {
			bucketDir := filepath.Join(authorDir, bucket.name)
			entryDirs, err := os.ReadDir(bucketDir)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				issues = append(issues, ValidationIssue{Path: bucketDir, Message: err.Error()})
				continue
			}
			for _, entryDir := range entryDirs {
				if !entryDir.IsDir() {
					continue
				}
				loaded, entryIssues, err := scanEntry(author.Name(), bucket.entryType, bucket.fileName, filepath.Join(bucketDir, entryDir.Name()))
				issues = append(issues, entryIssues...)
				if err != nil {
					issues = append(issues, ValidationIssue{Path: filepath.Join(bucketDir, entryDir.Name()), Message: err.Error()})
					continue
				}
				entries = append(entries, loaded)
			}
		}
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].Path == issues[j].Path {
			return issues[i].Message < issues[j].Message
		}
		return issues[i].Path < issues[j].Path
	})
	return entries, issues, nil
}

func scanEntry(author string, entryType EntryType, primaryName, entryRoot string) (loadedEntry, []ValidationIssue, error) {
	issues := make([]ValidationIssue, 0)
	primaryFiles := make([]string, 0, 4)
	err := filepath.WalkDir(entryRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Base(path) == primaryName {
			primaryFiles = append(primaryFiles, path)
		}
		return nil
	})
	if err != nil {
		return loadedEntry{}, issues, err
	}
	if len(primaryFiles) == 0 {
		return loadedEntry{}, issues, fmt.Errorf("missing %s", primaryName)
	}

	sort.Strings(primaryFiles)
	entry := loadedEntry{Entry: Entry{Type: entryType, Author: author, RootPath: entryRoot}, byVariant: make(map[string]Variant)}
	var baseMeta frontmatter
	searchParts := make([]string, 0, len(primaryFiles)+4)
	for i, primaryPath := range primaryFiles {
		data, err := os.ReadFile(primaryPath)
		if err != nil {
			issues = append(issues, ValidationIssue{Path: primaryPath, Message: err.Error()})
			continue
		}
		meta, body := parseFrontmatter(data)
		if i == 0 {
			baseMeta = meta
			entry.ID = strings.TrimSpace(meta.ID)
			if entry.ID == "" {
				entry.ID = filepath.ToSlash(filepath.Join(author, filepath.Base(entryRoot)))
			}
			entry.Description = strings.TrimSpace(meta.Description)
			entry.SourceTrust = normalizeSourceTrust(meta.SourceTrust)
			entry.Tags = dedupeStrings(meta.Tags)
			entry.Languages = dedupeStrings(meta.Languages)
			entry.Versions = dedupeStrings(meta.Versions)
			entry.Revision = strings.TrimSpace(meta.Revision)
			entry.UpdatedOn = strings.TrimSpace(meta.UpdatedOn)
		} else if strings.TrimSpace(meta.ID) != "" && !strings.EqualFold(strings.TrimSpace(meta.ID), entry.ID) {
			issues = append(issues, ValidationIssue{ID: entry.ID, Path: primaryPath, Message: "variant frontmatter id differs from entry id"})
		}

		rel, _ := filepath.Rel(entryRoot, primaryPath)
		lang, version := deriveVariantFromPrimary(rel, meta)
		refDir := filepath.Join(filepath.Dir(primaryPath), "references")
		refs := loadReferences(entryRoot, refDir)
		variant := Variant{Language: lang, Version: version, PrimaryFile: filepath.ToSlash(rel), ReferenceFiles: refs}
		key := variantKey(lang, version)
		if _, exists := entry.byVariant[key]; exists {
			issues = append(issues, ValidationIssue{ID: entry.ID, Path: primaryPath, Message: "duplicate variant for language/version"})
			continue
		}
		for _, ref := range meta.References {
			candidate := filepath.Join(filepath.Dir(primaryPath), filepath.FromSlash(ref))
			if _, err := os.Stat(candidate); err != nil {
				issues = append(issues, ValidationIssue{ID: entry.ID, Path: candidate, Message: "frontmatter reference does not exist"})
			}
		}
		entry.byVariant[key] = variant
		entry.Variants = append(entry.Variants, variant)
		searchParts = append(searchParts, body)
	}

	if entry.ID == "" {
		entry.ID = filepath.ToSlash(filepath.Join(author, filepath.Base(entryRoot)))
	}
	if entry.SourceTrust == "" {
		entry.SourceTrust = SourceTrustLocal
	}
	if len(entry.Languages) == 0 && len(entry.Variants) == 1 && entry.Variants[0].Language != "" {
		entry.Languages = []string{entry.Variants[0].Language}
	}
	if len(entry.Versions) == 0 && len(entry.Variants) == 1 && entry.Variants[0].Version != "" {
		entry.Versions = []string{entry.Variants[0].Version}
	}
	if entry.Description == "" {
		entry.Description = extractExcerpt(strings.Join(searchParts, "\n\n"), 220)
	}
	if baseMeta.Type != "" {
		want := normalizeEntryType(baseMeta.Type)
		if want != "" && want != entry.Type {
			issues = append(issues, ValidationIssue{ID: entry.ID, Path: entryRoot, Message: "frontmatter type does not match directory kind"})
		}
	}
	sort.Slice(entry.Variants, func(i, j int) bool {
		if entry.Variants[i].Language == entry.Variants[j].Language {
			return entry.Variants[i].Version < entry.Variants[j].Version
		}
		return entry.Variants[i].Language < entry.Variants[j].Language
	})
	searchText := strings.Join([]string{
		entry.ID,
		entry.Description,
		strings.Join(entry.Tags, " "),
		strings.Join(entry.Languages, " "),
		strings.Join(entry.Versions, " "),
		strings.Join(searchParts, "\n"),
	}, "\n")
	entry.searchText = searchText
	return entry, issues, nil
}

func parseFrontmatter(data []byte) (frontmatter, string) {
	content := string(data)
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(content, "---\n") {
		return frontmatter{}, strings.TrimSpace(content)
	}
	parts := strings.SplitN(content, "\n---\n", 2)
	if len(parts) != 2 {
		return frontmatter{}, strings.TrimSpace(content)
	}
	var meta frontmatter
	_ = yaml.Unmarshal([]byte(strings.TrimPrefix(parts[0], "---\n")), &meta)
	return meta, strings.TrimSpace(parts[1])
}

func normalizeSourceTrust(v string) SourceTrust {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case string(SourceTrustOfficial):
		return SourceTrustOfficial
	case string(SourceTrustMaintainer):
		return SourceTrustMaintainer
	case string(SourceTrustCommunity):
		return SourceTrustCommunity
	case string(SourceTrustLocal):
		return SourceTrustLocal
	default:
		return ""
	}
}

func normalizeEntryType(v string) EntryType {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case string(EntryTypeDoc):
		return EntryTypeDoc
	case string(EntryTypeSkill):
		return EntryTypeSkill
	default:
		return ""
	}
}

func deriveVariantFromPrimary(rel string, meta frontmatter) (string, string) {
	segments := strings.Split(filepath.ToSlash(filepath.Dir(rel)), "/")
	if len(segments) == 1 && segments[0] == "." {
		return strings.TrimSpace(meta.Language), strings.TrimSpace(meta.Version)
	}
	segments = filterSegments(segments)
	lang := strings.TrimSpace(meta.Language)
	version := strings.TrimSpace(meta.Version)
	if len(segments) == 1 {
		if lang == "" && isLanguageSegment(segments[0]) {
			lang = segments[0]
		} else if version == "" {
			version = segments[0]
		}
	}
	if len(segments) >= 2 {
		if lang == "" {
			lang = segments[0]
		}
		if version == "" {
			version = segments[1]
		}
	}
	return lang, version
}

func filterSegments(parts []string) []string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." || part == "references" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func isLanguageSegment(seg string) bool {
	seg = strings.TrimSpace(seg)
	if seg == "" {
		return false
	}
	if strings.Contains(seg, ".") {
		return false
	}
	if len(seg) == 2 || len(seg) == 5 {
		return true
	}
	return strings.Contains(seg, "-")
}

func loadReferences(entryRoot, refDir string) []string {
	entries, err := os.ReadDir(refDir)
	if err != nil {
		return nil
	}
	refs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		rel, err := filepath.Rel(entryRoot, filepath.Join(refDir, entry.Name()))
		if err != nil {
			continue
		}
		refs = append(refs, filepath.ToSlash(rel))
	}
	sort.Strings(refs)
	return refs
}

func selectVariant(entry Entry, lang, version string) Variant {
	if len(entry.Variants) == 0 {
		return Variant{}
	}
	lang = strings.TrimSpace(lang)
	version = strings.TrimSpace(version)
	for _, variant := range entry.Variants {
		if variant.Language == lang && variant.Version == version {
			return variant
		}
	}
	if lang != "" {
		for _, variant := range entry.Variants {
			if variant.Language == lang && variant.PrimaryFile != "" {
				return variant
			}
		}
	}
	if version != "" {
		for _, variant := range entry.Variants {
			if variant.Version == version && variant.PrimaryFile != "" {
				return variant
			}
		}
	}
	for _, variant := range entry.Variants {
		if variant.Language == "" && variant.Version == "" {
			return variant
		}
	}
	return entry.Variants[0]
}

func selectFilesForFetch(variant Variant, opts GetOptions) ([]string, error) {
	if opts.File != "" {
		want := filepath.ToSlash(strings.TrimPrefix(opts.File, "./"))
		if want == variant.PrimaryFile {
			return []string{want}, nil
		}
		for _, ref := range variant.ReferenceFiles {
			if want == ref {
				return []string{want}, nil
			}
		}
		return nil, fmt.Errorf("file %q not found in selected variant", opts.File)
	}
	if opts.Full {
		files := []string{variant.PrimaryFile}
		files = append(files, variant.ReferenceFiles...)
		return files, nil
	}
	return []string{variant.PrimaryFile}, nil
}

func variantKey(lang, version string) string {
	return lang + "\x00" + version
}

func scoreBoost(entry loadedEntry, queryLower string, queryTokens []string, selectedSkill string) (float64, []string) {
	boost := 0.0
	reasons := make([]string, 0, 4)
	entryLower := strings.ToLower(entry.ID)
	if strings.EqualFold(entry.ID, queryLower) || strings.EqualFold(filepath.Base(entry.ID), queryLower) {
		boost += 3.5
		reasons = append(reasons, "exact_id")
	}
	if strings.Contains(entryLower, queryLower) {
		boost += 1.5
		reasons = append(reasons, "id_contains")
	}
	textLower := strings.ToLower(entry.searchText)
	matched := 0
	for _, tok := range queryTokens {
		if strings.Contains(textLower, tok) {
			matched++
		}
	}
	if len(queryTokens) > 0 && matched > 0 {
		boost += float64(matched) / float64(len(queryTokens))
	}
	if selected := strings.ToLower(strings.TrimSpace(selectedSkill)); selected != "" {
		if strings.Contains(entryLower, selected) || strings.Contains(strings.ToLower(strings.Join(entry.Tags, " ")), selected) {
			boost += 1.2
			reasons = append(reasons, "skill_boost")
		}
	}
	return boost, reasons
}

func extractExcerpt(s string, max int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if len(s) <= max {
		return s
	}
	return strings.TrimSpace(s[:max-3]) + "..."
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
