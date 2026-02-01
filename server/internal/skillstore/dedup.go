package skillstore

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// GenerateDedupKey generates a deduplication key from name and author.
// Format: normalized_name:normalized_author
func GenerateDedupKey(name, author string) string {
	normalizedName := NormalizeString(name)
	normalizedAuthor := NormalizeString(author)
	return normalizedName + ":" + normalizedAuthor
}

// NormalizeString normalizes a string for deduplication.
// - Converts to lowercase
// - Removes accents/diacritics
// - Replaces multiple spaces with single space
// - Replaces spaces with hyphens
// - Removes special characters except hyphen and underscore
func NormalizeString(s string) string {
	if s == "" {
		return ""
	}

	// Convert to lowercase
	s = strings.ToLower(s)

	// Remove accents/diacritics
	s = removeAccents(s)

	// Trim whitespace
	s = strings.TrimSpace(s)

	// Replace multiple spaces with single space
	spaceRegex := regexp.MustCompile(`\s+`)
	s = spaceRegex.ReplaceAllString(s, " ")

	// Replace spaces with hyphens
	s = strings.ReplaceAll(s, " ", "-")

	// Remove special characters except hyphen, underscore, and alphanumeric
	cleanRegex := regexp.MustCompile(`[^a-z0-9\-_]`)
	s = cleanRegex.ReplaceAllString(s, "")

	// Remove consecutive hyphens
	hyphenRegex := regexp.MustCompile(`-+`)
	s = hyphenRegex.ReplaceAllString(s, "-")

	// Trim hyphens from start and end
	s = strings.Trim(s, "-")

	return s
}

// removeAccents removes accents/diacritics from a string.
func removeAccents(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	result, _, _ := transform.String(t, s)
	return result
}

// DeduplicateSkills removes duplicate skills based on dedup_key.
// When duplicates are found, keeps the one with higher downloads.
func DeduplicateSkills(skills []*Skill) []*Skill {
	seen := make(map[string]*Skill)

	for _, skill := range skills {
		// Generate dedup key if not set
		if skill.DedupKey == "" {
			skill.DedupKey = GenerateDedupKey(skill.Name, skill.Author)
		}

		existing, exists := seen[skill.DedupKey]
		if !exists {
			seen[skill.DedupKey] = skill
			continue
		}

		// Keep the one with more downloads
		if skill.Downloads > existing.Downloads {
			seen[skill.DedupKey] = skill
		}
	}

	result := make([]*Skill, 0, len(seen))
	for _, skill := range seen {
		result = append(result, skill)
	}

	return result
}
