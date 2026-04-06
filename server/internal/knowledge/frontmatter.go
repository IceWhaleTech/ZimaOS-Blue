package knowledge

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type pageDocument struct {
	Summary KnowledgePageSummary
	Content string
}

type archivedAnswerDocument struct {
	Answer KnowledgeArchivedAnswer
	Body   string
}

func renderPageDocument(doc pageDocument) string {
	var builder strings.Builder
	builder.WriteString("---\n")
	builder.WriteString(renderFrontmatterLine("title", doc.Summary.Title))
	builder.WriteString(renderFrontmatterLine("slug", doc.Summary.Slug))
	builder.WriteString(renderFrontmatterLine("page_type", doc.Summary.PageType))
	builder.WriteString(renderFrontmatterLine("summary", doc.Summary.Summary))
	builder.WriteString(renderFrontmatterArray("source_refs", doc.Summary.SourceRefs))
	builder.WriteString(renderFrontmatterArray("keywords", doc.Summary.Keywords))
	builder.WriteString(renderFrontmatterArray("backlinks", doc.Summary.Backlinks))
	builder.WriteString(renderFrontmatterLine("generated_at", doc.Summary.GeneratedAt.UTC().Format(time.RFC3339)))
	builder.WriteString(renderFrontmatterLine("updated_at", doc.Summary.UpdatedAt.UTC().Format(time.RFC3339)))
	builder.WriteString(renderFrontmatterLine("source_hash", doc.Summary.SourceHash))
	builder.WriteString(renderFrontmatterLine("status", string(doc.Summary.Status)))
	builder.WriteString(renderFrontmatterLine("confidence", string(doc.Summary.Confidence)))
	builder.WriteString(renderFrontmatterArray("conflicts_with", doc.Summary.ConflictsWith))
	builder.WriteString(renderFrontmatterArray("superseded_by", doc.Summary.SupersededBy))
	builder.WriteString(renderFrontmatterLine("derived_from_query", doc.Summary.DerivedFromQuery))
	builder.WriteString("---\n")
	body := strings.TrimSpace(doc.Content)
	if body != "" {
		builder.WriteString(body)
		builder.WriteString("\n")
	}
	return builder.String()
}

func parsePageDocument(raw string) (pageDocument, error) {
	fields, body, err := parseFrontmatterDocument(raw)
	if err != nil {
		return pageDocument{}, err
	}
	doc := pageDocument{
		Summary: KnowledgePageSummary{
			Title:            parseFrontmatterString(fields["title"]),
			Slug:             parseFrontmatterString(fields["slug"]),
			PageType:         parseFrontmatterString(fields["page_type"]),
			Summary:          parseFrontmatterString(fields["summary"]),
			SourceRefs:       parseFrontmatterArray(fields["source_refs"]),
			Keywords:         parseFrontmatterArray(fields["keywords"]),
			Backlinks:        parseFrontmatterArray(fields["backlinks"]),
			SourceHash:       parseFrontmatterString(fields["source_hash"]),
			Status:           KnowledgeStatus(parseFrontmatterString(fields["status"])),
			Confidence:       KnowledgeConfidence(parseFrontmatterString(fields["confidence"])),
			ConflictsWith:    parseFrontmatterArray(fields["conflicts_with"]),
			SupersededBy:     parseFrontmatterArray(fields["superseded_by"]),
			DerivedFromQuery: parseFrontmatterString(fields["derived_from_query"]),
		},
		Content: strings.TrimSpace(body),
	}
	if generatedAt := parseFrontmatterString(fields["generated_at"]); generatedAt != "" {
		if ts, parseErr := time.Parse(time.RFC3339, generatedAt); parseErr == nil {
			doc.Summary.GeneratedAt = ts.UTC()
		}
	}
	if updatedAt := parseFrontmatterString(fields["updated_at"]); updatedAt != "" {
		if ts, parseErr := time.Parse(time.RFC3339, updatedAt); parseErr == nil {
			doc.Summary.UpdatedAt = ts.UTC()
		}
	}
	if doc.Summary.Status == "" {
		doc.Summary.Status = KnowledgeStatusActive
	}
	if doc.Summary.Confidence == "" {
		doc.Summary.Confidence = KnowledgeConfidenceLow
	}
	if doc.Summary.UpdatedAt.IsZero() {
		doc.Summary.UpdatedAt = doc.Summary.GeneratedAt
	}
	return doc, nil
}

func renderArchivedAnswerDocument(doc archivedAnswerDocument) string {
	var builder strings.Builder
	builder.WriteString("---\n")
	builder.WriteString(renderFrontmatterLine("title", doc.Answer.Title))
	builder.WriteString(renderFrontmatterLine("path", doc.Answer.Path))
	builder.WriteString(renderFrontmatterLine("page_slug", doc.Answer.PageSlug))
	builder.WriteString(renderFrontmatterLine("query", doc.Answer.Query))
	builder.WriteString(renderFrontmatterLine("summary", doc.Answer.Summary))
	builder.WriteString(renderFrontmatterLine("generated_at", doc.Answer.GeneratedAt.UTC().Format(time.RFC3339)))
	builder.WriteString("---\n")
	body := strings.TrimSpace(doc.Body)
	if body != "" {
		builder.WriteString(body)
		builder.WriteString("\n")
	}
	return builder.String()
}

func parseArchivedAnswerDocument(raw string) (archivedAnswerDocument, error) {
	fields, body, err := parseFrontmatterDocument(raw)
	if err != nil {
		return archivedAnswerDocument{}, err
	}
	doc := archivedAnswerDocument{
		Answer: KnowledgeArchivedAnswer{
			Title:    parseFrontmatterString(fields["title"]),
			Path:     parseFrontmatterString(fields["path"]),
			PageSlug: parseFrontmatterString(fields["page_slug"]),
			Query:    parseFrontmatterString(fields["query"]),
			Summary:  parseFrontmatterString(fields["summary"]),
		},
		Body: strings.TrimSpace(body),
	}
	if generatedAt := parseFrontmatterString(fields["generated_at"]); generatedAt != "" {
		if ts, parseErr := time.Parse(time.RFC3339, generatedAt); parseErr == nil {
			doc.Answer.GeneratedAt = ts.UTC()
		}
	}
	return doc, nil
}

func parseFrontmatterDocument(raw string) (map[string]string, string, error) {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return nil, strings.TrimSpace(normalized), nil
	}
	rest := normalized[len("---\n"):]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return nil, "", fmt.Errorf("frontmatter terminator not found")
	}
	frontmatter := rest[:end]
	body := rest[end+len("\n---\n"):]
	fields := make(map[string]string)
	for _, line := range strings.Split(frontmatter, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		fields[key] = value
	}
	return fields, body, nil
}

func parseLooseFrontmatterDocument(raw string) (map[string]string, string) {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return nil, strings.TrimSpace(normalized)
	}
	lines := strings.Split(normalized, "\n")
	fields := make(map[string]string)
	idx := 1
	for idx < len(lines) {
		line := strings.TrimSpace(lines[idx])
		if line == "" {
			idx++
			continue
		}
		if line == "---" {
			idx++
			break
		}
		if !looksLikeFrontmatterField(line) {
			break
		}
		parts := strings.SplitN(line, ":", 2)
		fields[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		idx++
	}
	return fields, strings.TrimSpace(strings.Join(lines[idx:], "\n"))
}

func looksLikeFrontmatterField(line string) bool {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return false
	}
	key := strings.TrimSpace(parts[0])
	if key == "" {
		return false
	}
	for _, r := range key {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '_' || r == '-':
		default:
			return false
		}
	}
	return true
}

func renderFrontmatterLine(key string, value string) string {
	return fmt.Sprintf("%s: %s\n", key, strconv.Quote(value))
}

func renderFrontmatterArray(key string, values []string) string {
	if len(values) == 0 {
		return fmt.Sprintf("%s: []\n", key)
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return fmt.Sprintf("%s: []\n", key)
	}
	return fmt.Sprintf("%s: %s\n", key, string(encoded))
}

func parseFrontmatterString(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if unquoted, err := strconv.Unquote(raw); err == nil {
		return unquoted
	}
	return raw
}

func parseFrontmatterArray(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return nil
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err == nil {
		return values
	}
	raw = strings.TrimPrefix(raw, "[")
	raw = strings.TrimSuffix(raw, "]")
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values = make([]string, 0, len(parts))
	for _, part := range parts {
		value := parseFrontmatterString(strings.TrimSpace(part))
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}
