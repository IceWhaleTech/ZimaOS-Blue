package session

import (
	"regexp"
	"strings"
)

type memoryIntentBucket int

const (
	memoryIntentPreference memoryIntentBucket = iota
	memoryIntentProjectFact
	memoryIntentSessionSummary
)

var memoryBulletPrefixRE = regexp.MustCompile(`^\s*(?:[-*•]+|\d+[.)])\s*`)
var memoryWhitespaceRE = regexp.MustCompile(`\s+`)

var transientMemoryLinePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(?:/tmp/|\.tmp/|/mnt/user-data/(?:workspace|uploads|outputs)|\bworkspace_root\b|\bartifact_root\b)`),
	regexp.MustCompile(`(?i)\b(?:stdout|stderr|exit code|trace id|run id|approval requested|approval resolved|pending approval|tool call|tool output|audit trail|stack trace|debug log)\b`),
	regexp.MustCompile(`(?i)\b(?:uploaded (?:file|files|attachment)|attachment path|screenshot attached|temporary file|temp file|ephemeral upload)\b`),
	regexp.MustCompile(`(?i)\b(?:session id|conversation id)\b`),
	regexp.MustCompile(`(?i)\b(?:system prompt|prompt injection defense|raw tool payload)\b`),
}

// NormalizeExtractedMemoryForStorage cleans extracted session memory so that
// only durable user/project context is persisted into long-term storage.
func NormalizeExtractedMemoryForStorage(extracted string) string {
	content := strings.TrimSpace(extracted)
	if content == "" || strings.EqualFold(content, "NO_MEMORY_NEEDED") {
		return content
	}

	buckets := map[memoryIntentBucket][]string{
		memoryIntentPreference:     {},
		memoryIntentProjectFact:    {},
		memoryIntentSessionSummary: {},
	}
	seen := map[string]struct{}{}

	for _, raw := range strings.Split(content, "\n") {
		line := normalizeMemoryLine(raw)
		if line == "" || strings.EqualFold(line, "NO_MEMORY_NEEDED") || isTransientMemoryLine(line) {
			continue
		}
		fingerprint := NormalizeMemoryFingerprint(line)
		if fingerprint == "" {
			continue
		}
		if _, exists := seen[fingerprint]; exists {
			continue
		}
		seen[fingerprint] = struct{}{}
		bucket := classifyMemoryIntent(line)
		buckets[bucket] = append(buckets[bucket], "- "+line)
	}

	var ordered []string
	for _, bucket := range []memoryIntentBucket{
		memoryIntentPreference,
		memoryIntentProjectFact,
		memoryIntentSessionSummary,
	} {
		ordered = append(ordered, buckets[bucket]...)
	}
	if len(ordered) == 0 {
		return "NO_MEMORY_NEEDED"
	}
	return strings.Join(ordered, "\n")
}

// NormalizeMemoryFingerprint returns a canonical form used for dedupe.
func NormalizeMemoryFingerprint(content string) string {
	line := normalizeMemoryLine(content)
	line = strings.ToLower(line)
	line = strings.Trim(line, " .,:;`\"'")
	line = memoryWhitespaceRE.ReplaceAllString(line, " ")
	return strings.TrimSpace(line)
}

func normalizeMemoryLine(raw string) string {
	line := strings.TrimSpace(raw)
	if line == "" {
		return ""
	}
	line = strings.Trim(line, "`")
	line = memoryBulletPrefixRE.ReplaceAllString(line, "")
	line = memoryWhitespaceRE.ReplaceAllString(line, " ")
	return strings.TrimSpace(line)
}

func isTransientMemoryLine(line string) bool {
	for _, pattern := range transientMemoryLinePatterns {
		if pattern.MatchString(line) {
			return true
		}
	}
	return false
}

func classifyMemoryIntent(line string) memoryIntentBucket {
	lower := strings.ToLower(strings.TrimSpace(line))
	switch {
	case strings.Contains(lower, "preference") ||
		strings.Contains(lower, "prefers") ||
		strings.Contains(lower, "likes") ||
		strings.Contains(lower, "wants") ||
		strings.Contains(lower, "expects") ||
		strings.Contains(lower, "default"):
		return memoryIntentPreference
	case strings.Contains(lower, "project") ||
		strings.Contains(lower, "repo") ||
		strings.Contains(lower, "repository") ||
		strings.Contains(lower, "working on") ||
		strings.Contains(lower, "codebase") ||
		strings.Contains(lower, "uses ") ||
		strings.Contains(lower, "builds "):
		return memoryIntentProjectFact
	default:
		return memoryIntentSessionSummary
	}
}
