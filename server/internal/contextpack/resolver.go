package contextpack

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pruner"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/security"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tenant"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type ResolverConfig struct {
	MaxFiles    int
	MaxTokens   int
	SearchLimit int
}

type Resolver struct {
	registry    *Registry
	annotations *AnnotationStore
	sanitizer   *security.ExternalContentSanitizer
	config      ResolverConfig
}

func NewResolver(registry *Registry, annotations *AnnotationStore, cfg ResolverConfig) *Resolver {
	if cfg.MaxFiles <= 0 {
		cfg.MaxFiles = 3
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 1500
	}
	if cfg.SearchLimit <= 0 {
		cfg.SearchLimit = 5
	}
	return &Resolver{registry: registry, annotations: annotations, sanitizer: security.DefaultExternalContentSanitizer(), config: cfg}
}

func (r *Resolver) ResolvePrompt(ctx context.Context) (string, *SelectionSet, error) {
	if r == nil || r.registry == nil {
		return "", nil, nil
	}
	query := strings.TrimSpace(PromptQueryFromContext(ctx))
	if query == "" {
		return "", nil, nil
	}
	lang := strings.TrimSpace(tools.GetLang(ctx))
	selectedSkill := strings.TrimSpace(SelectedSkillFromContext(ctx))
	budget := BudgetTokensFromContext(ctx)
	if budget <= 0 || budget > r.config.MaxTokens {
		budget = r.config.MaxTokens
	}

	results, err := r.registry.Search(ctx, SearchOptions{Query: query, Language: lang, SelectedSkill: selectedSkill, Limit: r.config.SearchLimit})
	if err != nil || len(results) == 0 {
		return "", nil, err
	}

	sel := &SelectionSet{Query: query, SelectedSkill: selectedSkill, GeneratedAt: time.Now().UTC()}
	var sb strings.Builder
	sb.WriteString("<context_packs query=")
	sb.WriteString(fmt.Sprintf("%q", xmlAttr(query)))
	if selectedSkill != "" {
		sb.WriteString(" selected_skill=")
		sb.WriteString(fmt.Sprintf("%q", xmlAttr(selectedSkill)))
	}
	sb.WriteString(">")

	remaining := budget
	fileCount := 0
	for _, result := range results {
		if fileCount >= r.config.MaxFiles || remaining <= 0 {
			break
		}
		fetch, err := r.registry.Get(ctx, result.Entry.ID, GetOptions{Language: result.Variant.Language, Version: result.Variant.Version})
		if err != nil || fetch == nil || len(fetch.Files) == 0 {
			continue
		}
		candidateFiles := append([]FetchedFile(nil), fetch.Files...)
		for _, ref := range result.Variant.ReferenceFiles {
			if fileCount+len(candidateFiles) >= r.config.MaxFiles {
				break
			}
			if !shouldIncludeReference(query, ref) {
				continue
			}
			refFetch, err := r.registry.Get(ctx, result.Entry.ID, GetOptions{Language: result.Variant.Language, Version: result.Variant.Version, File: ref})
			if err != nil || refFetch == nil || len(refFetch.Files) == 0 {
				continue
			}
			candidateFiles = append(candidateFiles, refFetch.Files[0])
		}

		for _, file := range candidateFiles {
			if fileCount >= r.config.MaxFiles || remaining <= 0 {
				break
			}
			anns, _ := r.applicableAnnotations(ctx, result.Entry.ID, result.Variant.Language, result.Variant.Version, file.Path)
			block, selected, usedTokens := r.renderPromptBlock(result.Entry, result.Variant, file, anns, remaining)
			if block == "" || usedTokens <= 0 {
				continue
			}
			remaining -= usedTokens
			sel.TotalTokens += usedTokens
			fileCount++
			sel.Files = append(sel.Files, selected)
			sb.WriteString(block)
		}
	}

	if len(sel.Files) == 0 {
		return "", nil, nil
	}
	sb.WriteString("</context_packs>")
	return sb.String(), sel, nil
}

func (r *Resolver) renderPromptBlock(entry Entry, variant Variant, file FetchedFile, anns []Annotation, budget int) (string, SelectedFile, int) {
	selected := SelectedFile{
		EntryID:     entry.ID,
		Type:        entry.Type,
		SourceTrust: entry.SourceTrust,
		Language:    variant.Language,
		Version:     variant.Version,
		File:        file.Path,
		SHA256:      file.SHA256,
	}
	content := file.Content
	annotated := false
	if len(anns) > 0 {
		annotated = true
		for _, ann := range anns {
			selected.AnnotationIDs = append(selected.AnnotationIDs, annotationID(ann))
		}
		content = content + "\n\n<annotation_notes>\n" + annotationsText(anns) + "\n</annotation_notes>"
	}
	sourceLabel := fmt.Sprintf("contextpack:%s:%s", entry.ID, file.Path)
	wrapped, _ := r.sanitizer.SanitizeExternalContent(content, sourceLabel)
	full := buildPackBlock(entry, variant, file, wrapped)
	usedTokens := pruner.EstimateTokens(full)
	truncated := false
	if usedTokens > budget {
		trimmed := truncateToApproxTokens(content, maxInt(64, budget/2))
		wrapped, _ = r.sanitizer.SanitizeExternalContent(trimmed, sourceLabel)
		full = buildPackBlock(entry, variant, file, wrapped)
		usedTokens = pruner.EstimateTokens(full)
		truncated = true
	}
	if usedTokens > budget || usedTokens <= 0 {
		return "", SelectedFile{}, 0
	}
	selected.Tokens = usedTokens
	selected.Annotated = annotated
	selected.Truncated = truncated
	return full, selected, usedTokens
}

func (r *Resolver) applicableAnnotations(ctx context.Context, entryID, lang, version, file string) ([]Annotation, error) {
	if r.annotations == nil {
		return nil, nil
	}
	tenantID := tenantIDFromContext(ctx)
	userID := strings.TrimSpace(tools.GetUserID(ctx))
	return r.annotations.Applicable(ctx, AnnotationFilter{TenantID: tenantID, UserID: userID, EntryID: entryID, Language: lang, Version: version, File: file})
}

func buildPackBlock(entry Entry, variant Variant, file FetchedFile, wrapped string) string {
	var sb strings.Builder
	sb.WriteString("<context_pack id=")
	sb.WriteString(fmt.Sprintf("%q", xmlAttr(entry.ID)))
	sb.WriteString(" type=")
	sb.WriteString(fmt.Sprintf("%q", string(entry.Type)))
	sb.WriteString(" source_trust=")
	sb.WriteString(fmt.Sprintf("%q", string(entry.SourceTrust)))
	if variant.Language != "" {
		sb.WriteString(" language=")
		sb.WriteString(fmt.Sprintf("%q", xmlAttr(variant.Language)))
	}
	if variant.Version != "" {
		sb.WriteString(" version=")
		sb.WriteString(fmt.Sprintf("%q", xmlAttr(variant.Version)))
	}
	sb.WriteString(" file=")
	sb.WriteString(fmt.Sprintf("%q", xmlAttr(file.Path)))
	sb.WriteString(" sha256=")
	sb.WriteString(fmt.Sprintf("%q", file.SHA256))
	sb.WriteString(">")
	sb.WriteString(wrapped)
	sb.WriteString("</context_pack>")
	return sb.String()
}

func annotationsText(anns []Annotation) string {
	if len(anns) == 0 {
		return ""
	}
	lines := make([]string, 0, len(anns))
	for _, ann := range anns {
		lines = append(lines, "- "+strings.TrimSpace(ann.Note))
	}
	return strings.Join(lines, "\n")
}

func annotationID(ann Annotation) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{ann.TenantID, ann.UserID, ann.EntryID, ann.Language, ann.Version, ann.File, ann.Note}, "|")))
	return hex.EncodeToString(sum[:8])
}

func shouldIncludeReference(query, ref string) bool {
	base := strings.ToLower(strings.TrimSuffix(filepath.Base(ref), filepath.Ext(ref)))
	base = strings.ReplaceAll(base, "-", " ")
	query = strings.ToLower(strings.TrimSpace(query))
	return base != "" && strings.Contains(query, base)
}

func truncateToApproxTokens(s string, maxTokens int) string {
	if maxTokens <= 0 || strings.TrimSpace(s) == "" {
		return ""
	}
	if pruner.EstimateTokens(s) <= maxTokens {
		return s
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return s
	}
	var sb strings.Builder
	for i, word := range words {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(word)
		if pruner.EstimateTokens(sb.String()) >= maxTokens {
			break
		}
	}
	out := strings.TrimSpace(sb.String())
	if out == "" {
		return ""
	}
	return out + "\n\n[truncated]"
}

func tenantIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return "local"
	}
	id := tenant.GetTenantIDFromContext(ctx)
	if id.String() == "00000000-0000-0000-0000-000000000000" {
		return "local"
	}
	return id.String()
}

func xmlAttr(v string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", `'`, "&apos;")
	return replacer.Replace(v)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func SelectionSummaryJSON(sel *SelectionSet) string {
	if sel == nil {
		return `{"selected":0}`
	}
	payload := map[string]interface{}{
		"selected":       len(sel.Files),
		"total_tokens":   sel.TotalTokens,
		"selected_skill": sel.SelectedSkill,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return `{"selected":0}`
	}
	return string(b)
}
