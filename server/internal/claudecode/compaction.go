package claudecode

import (
	"context"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

const (
	// BaseChunkRatio is the default ratio of context to use per chunk.
	BaseChunkRatio = 0.4
	// MinChunkRatio is the minimum ratio to prevent chunks from being too small.
	MinChunkRatio = 0.15
	// SafetyMargin accounts for token estimation inaccuracy.
	SafetyMargin = 1.2
	// DefaultContextTokens is the default context window size.
	DefaultContextTokens = 200000
	// DefaultSummaryFallback is returned when summarization fails.
	DefaultSummaryFallback = "No prior history."
	// DefaultParts is the default number of parts to split messages into.
	DefaultParts = 2
)

const maxRelevantFiles = 12

var (
	structuredSummarySectionOrder = []string{
		"Goal",
		"Instructions",
		"Discoveries",
		"Accomplished",
		"Relevant Files",
	}

	structuredSummarySectionAliases = map[string]string{
		"goal":               "Goal",
		"objective":          "Goal",
		"objectives":         "Goal",
		"instructions":       "Instructions",
		"instruction":        "Instructions",
		"preferences":        "Instructions",
		"preference":         "Instructions",
		"constraints":        "Instructions",
		"constraint":         "Instructions",
		"external approvals": "Instructions",
		"external approval":  "Instructions",
		"approvals":          "Instructions",
		"approval":           "Instructions",
		"reminders":          "Instructions",
		"reminder":           "Instructions",
		"discoveries":        "Discoveries",
		"discovery":          "Discoveries",
		"decisions":          "Discoveries",
		"decision":           "Discoveries",
		"confirmed facts":    "Discoveries",
		"confirmed fact":     "Discoveries",
		"facts":              "Discoveries",
		"fact":               "Discoveries",
		"findings":           "Discoveries",
		"finding":            "Discoveries",
		"accomplished":       "Accomplished",
		"completed":          "Accomplished",
		"completed work":     "Accomplished",
		"status":             "Accomplished",
		"pending":            "Accomplished",
		"pending tasks":      "Accomplished",
		"pending task":       "Accomplished",
		"todo":               "Accomplished",
		"todos":              "Accomplished",
		"to do":              "Accomplished",
		"open questions":     "Accomplished",
		"open question":      "Accomplished",
		"next steps":         "Accomplished",
		"next step":          "Accomplished",
		"relevant files":     "Relevant Files",
		"relevant file":      "Relevant Files",
		"file paths":         "Relevant Files",
		"file path":          "Relevant Files",
		"files":              "Relevant Files",
		"read files":         "Relevant Files",
		"read file":          "Relevant Files",
		"modified files":     "Relevant Files",
		"modified file":      "Relevant Files",
		"written files":      "Relevant Files",
		"written file":       "Relevant Files",
		"edited files":       "Relevant Files",
		"edited file":        "Relevant Files",
	}

	reMarkdownFileLink  = regexp.MustCompile(`\[[^\]]+\]\(([^)\s]+)\)`)
	rePatchFileLine     = regexp.MustCompile(`(?m)^\*{3} (?:Add File|Update File|Delete File|Move to): (.+)$`)
	rePathLineRefSuffix = regexp.MustCompile(`(?i)(?::\d+(?::\d+)?)$|#L\d+(?:C\d+)?$`)
	reToolNameToken     = regexp.MustCompile(`[a-z0-9]+`)
)

type structuredSummarySections map[string][]string

type relevantFileCollector struct {
	modified  map[string]struct{}
	read      map[string]struct{}
	mentioned map[string]struct{}
}

// CompactionConfig holds configuration for context compaction.
type CompactionConfig struct {
	// MaxContextTokens is the maximum context window size.
	MaxContextTokens int
	// MaxHistoryShare is the maximum share of context for history (0.0-1.0).
	MaxHistoryShare float64
	// ReserveTokens is the number of tokens to reserve for the response.
	ReserveTokens int
	// CustomInstructions are additional instructions for summarization.
	CustomInstructions string
}

// DefaultCompactionConfig returns the default compaction configuration.
func DefaultCompactionConfig() CompactionConfig {
	return CompactionConfig{
		MaxContextTokens: DefaultContextTokens,
		MaxHistoryShare:  0.5,
		ReserveTokens:    20000, // system prompt + memory + tools + response headroom
	}
}

// Compactor handles context compaction and summarization.
type Compactor struct {
	config   CompactionConfig
	provider llm.Provider
}

// NewCompactor creates a new Compactor.
func NewCompactor(config CompactionConfig, provider llm.Provider) *Compactor {
	return &Compactor{
		config:   config,
		provider: provider,
	}
}

// StructuredSummaryInstructions returns the canonical summary instructions shared
// by small-model and bridge/LLM compaction paths.
func StructuredSummaryInstructions(custom string) string {
	lines := []string{
		"Format requirements:",
		"- Output ONLY the summary text.",
		"- Use these sections in this exact order and omit any empty section: Goal, Instructions, Discoveries, Accomplished, Relevant Files.",
		"- Put each section label on its own line, followed by concise '- ' bullets.",
		"- Preserve durable goals, explicit user instructions, confirmed discoveries, completed work, and pending work that still matters.",
		"- Preserve exact filenames, directories, IDs, dates, and other opaque identifiers when they matter.",
		"- In Relevant Files, list the most relevant paths only, prioritizing modified/written files before read-only files.",
		"- Do not include chain-of-thought, transient speculation, or failed exploratory branches unless they changed the outcome.",
	}
	if trimmed := strings.TrimSpace(custom); trimmed != "" {
		lines = append(lines, "- Additional focus: "+trimmed)
	}
	return strings.Join(lines, "\n")
}

// NormalizeStructuredSummary rewrites summaries into the canonical five-section
// format and enriches the Relevant Files section from message/tool context.
func NormalizeStructuredSummary(rawSummary, previousSummary string, messages []llm.Message) string {
	sections := parseStructuredSummary(rawSummary)
	relevantFiles := collectRelevantFiles(messages, rawSummary, previousSummary)
	if len(relevantFiles) > 0 {
		sections["Relevant Files"] = relevantFiles
	}
	return formatStructuredSummary(sections)
}

func parseStructuredSummary(raw string) structuredSummarySections {
	sections := make(structuredSummarySections, len(structuredSummarySectionOrder))
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return sections
	}

	current := ""
	recognized := false
	for _, line := range strings.Split(trimmed, "\n") {
		text := strings.TrimSpace(line)
		if text == "" {
			continue
		}
		if section, item, ok := parseStructuredSummaryLine(text); ok {
			current = section
			recognized = true
			if item != "" {
				appendStructuredSummaryItem(sections, section, item)
			}
			continue
		}
		if current == "" {
			continue
		}
		appendStructuredSummaryItem(sections, current, trimStructuredSummaryBullet(text))
	}

	if recognized {
		return sections
	}
	for _, line := range strings.Split(trimmed, "\n") {
		text := trimStructuredSummaryBullet(line)
		if text == "" {
			continue
		}
		appendStructuredSummaryItem(sections, "Discoveries", text)
	}
	return sections
}

func parseStructuredSummaryLine(line string) (section, item string, ok bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return "", "", false
	}

	inline := trimStructuredSummaryBullet(trimmed)
	if idx := strings.Index(inline, ":"); idx > 0 {
		if section = canonicalStructuredSummarySection(inline[:idx]); section != "" {
			return section, strings.TrimSpace(inline[idx+1:]), true
		}
	}

	heading := strings.TrimLeft(trimmed, "#-*• \t")
	heading = strings.TrimSpace(strings.TrimSuffix(heading, ":"))
	if section = canonicalStructuredSummarySection(heading); section != "" {
		return section, "", true
	}
	return "", "", false
}

func canonicalStructuredSummarySection(label string) string {
	normalized := normalizeStructuredSummaryLabel(label)
	if normalized == "" {
		return ""
	}
	return structuredSummarySectionAliases[normalized]
}

func normalizeStructuredSummaryLabel(label string) string {
	if strings.TrimSpace(label) == "" {
		return ""
	}
	var b strings.Builder
	lastSpace := false
	for _, r := range strings.ToLower(label) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastSpace = false
		case unicode.IsSpace(r) || r == '_' || r == '-' || r == '/':
			if !lastSpace && b.Len() > 0 {
				b.WriteByte(' ')
				lastSpace = true
			}
		}
	}
	return strings.TrimSpace(b.String())
}

func trimStructuredSummaryBullet(line string) string {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimLeft(trimmed, "-*• \t")
	return strings.TrimSpace(trimmed)
}

func appendStructuredSummaryItem(sections structuredSummarySections, section, item string) {
	item = strings.TrimSpace(item)
	if item == "" {
		return
	}
	for _, existing := range sections[section] {
		if strings.EqualFold(existing, item) {
			return
		}
	}
	sections[section] = append(sections[section], item)
}

func formatStructuredSummary(sections structuredSummarySections) string {
	blocks := make([]string, 0, len(structuredSummarySectionOrder))
	for _, section := range structuredSummarySectionOrder {
		items := sections[section]
		if len(items) == 0 {
			continue
		}
		var b strings.Builder
		b.WriteString(section)
		for _, item := range items {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			b.WriteString("\n- ")
			b.WriteString(item)
		}
		if b.Len() > len(section) {
			blocks = append(blocks, b.String())
		}
	}
	return strings.TrimSpace(strings.Join(blocks, "\n\n"))
}

func collectRelevantFiles(messages []llm.Message, summaryTexts ...string) []string {
	collector := relevantFileCollector{
		modified:  make(map[string]struct{}),
		read:      make(map[string]struct{}),
		mentioned: make(map[string]struct{}),
	}

	for _, msg := range messages {
		for _, tc := range msg.ToolCalls {
			toolMode := classifyToolFileAccess(tc.Name)
			for _, path := range extractPathsFromStructuredPayload(tc.Arguments) {
				collector.add(toolMode, path)
			}
			if toolMode == "modified" {
				for _, path := range extractPatchPaths(tc.Arguments) {
					collector.add("modified", path)
				}
			}
		}
		if mode := classifyToolFileAccess(msg.ToolName); mode != "" {
			for _, path := range extractPathsFromStructuredPayload(msg.Content) {
				collector.add(mode, path)
			}
		}
		if msg.Role == llm.RoleUser || msg.Role == llm.RoleAssistant || msg.Role == llm.RoleSystem {
			for _, path := range extractLikelyPathsFromText(messageSummaryText(msg)) {
				collector.add("mentioned", path)
			}
		}
	}

	for _, text := range summaryTexts {
		for _, path := range extractLikelyPathsFromText(text) {
			collector.add("mentioned", path)
		}
		for _, path := range extractPatchPaths(text) {
			collector.add("mentioned", path)
		}
	}

	return collector.finalize()
}

func (c *relevantFileCollector) add(mode, path string) {
	path = normalizePathCandidate(path)
	if path == "" {
		return
	}
	switch mode {
	case "modified":
		c.modified[path] = struct{}{}
	case "read":
		if _, exists := c.modified[path]; !exists {
			c.read[path] = struct{}{}
		}
	default:
		if _, exists := c.modified[path]; exists {
			return
		}
		if _, exists := c.read[path]; exists {
			return
		}
		c.mentioned[path] = struct{}{}
	}
}

func (c *relevantFileCollector) finalize() []string {
	modified := mapKeysSorted(c.modified)
	read := mapKeysSorted(c.read)
	mentioned := mapKeysSorted(c.mentioned)

	out := make([]string, 0, len(modified)+len(read)+len(mentioned))
	seen := make(map[string]struct{}, len(modified)+len(read)+len(mentioned))
	appendPaths := func(paths []string) {
		for _, path := range paths {
			if _, exists := seen[path]; exists {
				continue
			}
			seen[path] = struct{}{}
			out = append(out, path)
			if len(out) >= maxRelevantFiles {
				return
			}
		}
	}
	appendPaths(modified)
	if len(out) < maxRelevantFiles {
		appendPaths(read)
	}
	if len(out) < maxRelevantFiles {
		appendPaths(mentioned)
	}
	return out
}

func mapKeysSorted(set map[string]struct{}) []string {
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func classifyToolFileAccess(name string) string {
	if strings.TrimSpace(name) == "" {
		return ""
	}
	tokens := reToolNameToken.FindAllString(strings.ToLower(name), -1)
	for _, token := range tokens {
		switch token {
		case "write", "edit", "replace", "patch", "update", "create":
			return "modified"
		}
	}
	for _, token := range tokens {
		switch token {
		case "read", "open", "view":
			return "read"
		}
	}
	return ""
}

func extractPathsFromStructuredPayload(text string) []string {
	paths := make([]string, 0, 4)
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return paths
	}

	var payload any
	if err := json.Unmarshal([]byte(trimmed), &payload); err == nil {
		paths = append(paths, extractPathsFromJSONValue(payload)...)
	}
	paths = append(paths, extractPatchPaths(trimmed)...)
	return dedupeStrings(paths)
}

func extractPathsFromJSONValue(v any) []string {
	out := make([]string, 0, 4)
	switch value := v.(type) {
	case map[string]any:
		for key, child := range value {
			if isLikelyPathKey(key) {
				out = append(out, extractStringValues(child)...)
			}
			out = append(out, extractPathsFromJSONValue(child)...)
		}
	case []any:
		for _, child := range value {
			out = append(out, extractPathsFromJSONValue(child)...)
		}
	}
	return dedupeStrings(out)
}

func extractStringValues(v any) []string {
	switch value := v.(type) {
	case string:
		return []string{value}
	case []any:
		out := make([]string, 0, len(value))
		for _, item := range value {
			out = append(out, extractStringValues(item)...)
		}
		return out
	default:
		return nil
	}
}

func isLikelyPathKey(key string) bool {
	switch normalizeStructuredSummaryLabel(key) {
	case "path", "paths", "file", "files", "file path", "file paths", "filepath", "filepaths",
		"filename", "filenames", "target", "targets", "source", "sources", "destination",
		"dest", "old path", "new path", "workdir", "cwd", "directory", "directories", "dir":
		return true
	default:
		return false
	}
}

func extractPatchPaths(text string) []string {
	matches := rePatchFileLine.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	paths := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		paths = append(paths, match[1])
	}
	return dedupeStrings(paths)
}

func extractLikelyPathsFromText(text string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	candidates := make([]string, 0, 8)
	for _, match := range reMarkdownFileLink.FindAllStringSubmatch(text, -1) {
		if len(match) >= 2 {
			candidates = append(candidates, match[1])
		}
	}
	fields := strings.FieldsFunc(text, func(r rune) bool {
		switch r {
		case ' ', '\n', '\r', '\t', ',', ';', '(', ')', '[', ']', '{', '}', '<', '>', '"', '\'':
			return true
		default:
			return false
		}
	})
	for _, field := range fields {
		candidates = append(candidates, field)
	}
	return dedupeStrings(candidates)
}

func normalizePathCandidate(candidate string) string {
	path := strings.TrimSpace(candidate)
	if path == "" {
		return ""
	}
	path = strings.Trim(path, "`*.,!?;:")
	path = strings.Trim(path, "\"'")
	path = strings.Trim(path, "()[]{}<>")
	path = strings.TrimSpace(path)
	if path == "" || strings.Contains(path, "://") {
		return ""
	}
	path = rePathLineRefSuffix.ReplaceAllString(path, "")
	path = strings.Trim(path, "`*.,!?;:")
	if path == "" {
		return ""
	}
	if strings.Count(path, "/")+strings.Count(path, "\\") == 0 {
		return ""
	}
	hasLetter := false
	for _, r := range path {
		if unicode.IsLetter(r) {
			hasLetter = true
			break
		}
	}
	if !hasLetter {
		return ""
	}
	return path
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func messageSummaryText(msg llm.Message) string {
	if trimmed := strings.TrimSpace(msg.Content); trimmed != "" {
		return trimmed
	}
	parts := make([]string, 0, len(msg.ContentParts)+1)
	for _, part := range msg.ContentParts {
		switch part.Type {
		case "text":
			if trimmed := strings.TrimSpace(part.Text); trimmed != "" {
				parts = append(parts, trimmed)
			}
		case "image":
			parts = append(parts, "[image attachment]")
		default:
			if trimmed := strings.TrimSpace(part.Text); trimmed != "" {
				parts = append(parts, trimmed)
			}
		}
	}
	if len(parts) == 0 && len(msg.ToolCalls) > 0 {
		names := make([]string, 0, len(msg.ToolCalls))
		for _, tc := range msg.ToolCalls {
			if name := strings.TrimSpace(tc.Name); name != "" {
				names = append(names, name)
			}
		}
		if len(names) > 0 {
			parts = append(parts, "Tool calls: "+strings.Join(names, ", "))
		}
	}
	return strings.Join(parts, "\n")
}

// countTokensInString estimates tokens using a hybrid heuristic:
//   - Short ASCII words (≤10 bytes): 1 token each
//   - Long ASCII words (>10 bytes, e.g. URLs, JSON): ceil(len/4)
//   - Non-ASCII runes (CJK, emoji): ~1 token each
//   - ASCII bytes adjacent to non-ASCII: ceil(len/4)
//
// Optimized: starts in fast ASCII-only mode, falls back to mixed-mode
// mid-word only when a high-bit byte is encountered.
func countTokensInString(s string) int {
	n := len(s)
	if n == 0 {
		return 0
	}
	count := 0
	i := 0
	for i < n {
		c := s[i]
		// skip whitespace
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			i++
			continue
		}
		// start of a word — try ASCII fast path
		start := i
		for i < n {
			c = s[i]
			if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
				break
			}
			if c&0x80 != 0 {
				// hit non-ASCII mid-word: switch to mixed counting for this word
				asciiBytes := i - start
				nonASCIIRunes := 0
				if asciiBytes > 0 {
					count += (asciiBytes + 3) / 4
					asciiBytes = 0
				}
				for i < n {
					c = s[i]
					if c <= ' ' && (c == ' ' || c == '\t' || c == '\n' || c == '\r') {
						break
					}
					if c&0x80 == 0 {
						asciiBytes++
						i++
					} else {
						if asciiBytes > 0 && nonASCIIRunes > 0 {
							count += (asciiBytes + 3) / 4
							asciiBytes = 0
						}
						i++
						for i < n && s[i]&0xC0 == 0x80 {
							i++
						}
						nonASCIIRunes++
					}
				}
				count += nonASCIIRunes
				if asciiBytes > 0 {
					count += (asciiBytes + 3) / 4
				}
				goto nextWord
			}
			i++
		}
		// pure ASCII word
		{
			wordLen := i - start
			if wordLen > 10 {
				count += (wordLen + 3) / 4
			} else {
				count++
			}
		}
	nextWord:
	}
	return count
}

// EstimateTokens estimates the number of tokens in a message.
// Uses a hybrid whitespace-split + byte-length heuristic for better accuracy
// across English, CJK, and mixed content.
// It accounts for Content, ContentParts (multimodal), and ToolCalls.
func EstimateTokens(msg llm.Message) int {
	total := 0

	// Primary content field
	if msg.Content != "" {
		total += countTokensInString(msg.Content)
	}

	// ContentParts (used for multimodal messages where Content is cleared)
	for _, part := range msg.ContentParts {
		switch part.Type {
		case "text":
			total += countTokensInString(part.Text)
		case "image":
			// Images consume ~85 tokens for low-res, ~765 for high-res.
			// Use a conservative estimate since we can't know the resolution.
			total += 765
		}
	}

	// ToolCalls (assistant requesting tool use)
	for _, tc := range msg.ToolCalls {
		total += countTokensInString(tc.Name) + countTokensInString(tc.Arguments) + 10 // +10 for structural overhead
	}

	// Minimum 4 tokens per message for role/structural overhead
	if total == 0 && (msg.Role != "" || msg.ToolCallID != "") {
		total = 4
	}

	return total
}

// EstimateMessagesTokens estimates the total tokens in a slice of messages.
func EstimateMessagesTokens(messages []llm.Message) int {
	total := 0
	for _, msg := range messages {
		total += EstimateTokens(msg)
	}
	return total
}

// SplitMessagesByTokenShare splits messages into parts based on token share.
func SplitMessagesByTokenShare(messages []llm.Message, parts int) [][]llm.Message {
	if len(messages) == 0 {
		return nil
	}

	normalizedParts := normalizeParts(parts, len(messages))
	if normalizedParts <= 1 {
		return [][]llm.Message{messages}
	}

	totalTokens := EstimateMessagesTokens(messages)
	targetTokens := totalTokens / normalizedParts
	chunks := make([][]llm.Message, 0, normalizedParts)
	current := make([]llm.Message, 0)
	currentTokens := 0

	for _, msg := range messages {
		msgTokens := EstimateTokens(msg)
		if len(chunks) < normalizedParts-1 &&
			len(current) > 0 &&
			currentTokens+msgTokens > targetTokens {
			chunks = append(chunks, current)
			current = make([]llm.Message, 0)
			currentTokens = 0
		}

		current = append(current, msg)
		currentTokens += msgTokens
	}

	if len(current) > 0 {
		chunks = append(chunks, current)
	}

	return chunks
}

// ChunkMessagesByMaxTokens splits messages into chunks with a maximum token count.
func ChunkMessagesByMaxTokens(messages []llm.Message, maxTokens int) [][]llm.Message {
	if len(messages) == 0 {
		return nil
	}

	chunks := make([][]llm.Message, 0)
	currentChunk := make([]llm.Message, 0)
	currentTokens := 0

	for _, msg := range messages {
		msgTokens := EstimateTokens(msg)
		if len(currentChunk) > 0 && currentTokens+msgTokens > maxTokens {
			chunks = append(chunks, currentChunk)
			currentChunk = make([]llm.Message, 0)
			currentTokens = 0
		}

		currentChunk = append(currentChunk, msg)
		currentTokens += msgTokens

		// Split oversized messages to avoid unbounded chunk growth
		if msgTokens > maxTokens {
			chunks = append(chunks, currentChunk)
			currentChunk = make([]llm.Message, 0)
			currentTokens = 0
		}
	}

	if len(currentChunk) > 0 {
		chunks = append(chunks, currentChunk)
	}

	return chunks
}

// ComputeAdaptiveChunkRatio computes an adaptive chunk ratio based on message size.
func ComputeAdaptiveChunkRatio(messages []llm.Message, contextWindow int) float64 {
	if len(messages) == 0 {
		return BaseChunkRatio
	}

	totalTokens := EstimateMessagesTokens(messages)
	avgTokens := float64(totalTokens) / float64(len(messages))

	// Apply safety margin
	safeAvgTokens := avgTokens * SafetyMargin
	avgRatio := safeAvgTokens / float64(contextWindow)

	// If average message is > 10% of context, reduce chunk ratio
	if avgRatio > 0.1 {
		reduction := min(avgRatio*2, BaseChunkRatio-MinChunkRatio)
		return max(MinChunkRatio, BaseChunkRatio-reduction)
	}

	return BaseChunkRatio
}

// IsOversizedForSummary checks if a message is too large to summarize.
func IsOversizedForSummary(msg llm.Message, contextWindow int) bool {
	tokens := float64(EstimateTokens(msg)) * SafetyMargin
	return tokens > float64(contextWindow)*0.5
}

// PruneResult contains the result of pruning history.
type PruneResult struct {
	Messages        []llm.Message
	DroppedMessages []llm.Message
	DroppedChunks   int
	DroppedCount    int
	DroppedTokens   int
	KeptTokens      int
	BudgetTokens    int
}

// PruneHistoryForContextShare prunes history to fit within context budget.
// It ensures tool call/result pairs are kept together — orphaned tool_result
// messages (whose tool_use_id has no matching assistant tool_call) are removed.
func PruneHistoryForContextShare(messages []llm.Message, maxContextTokens int, maxHistoryShare float64, parts int) PruneResult {
	if maxHistoryShare <= 0 {
		maxHistoryShare = 0.5
	}
	budgetTokens := int(float64(maxContextTokens) * maxHistoryShare)
	if budgetTokens < 1 {
		budgetTokens = 1
	}

	keptMessages := messages
	allDroppedMessages := make([]llm.Message, 0)
	droppedChunks := 0
	droppedCount := 0
	droppedTokens := 0

	normalizedParts := normalizeParts(parts, len(keptMessages))

	for len(keptMessages) > 0 && EstimateMessagesTokens(keptMessages) > budgetTokens {
		chunks := SplitMessagesByTokenShare(keptMessages, normalizedParts)
		if len(chunks) <= 1 {
			// Single chunk still over budget — drop oldest messages one by one
			for len(keptMessages) > 1 && EstimateMessagesTokens(keptMessages) > budgetTokens {
				dropped := keptMessages[0]
				keptMessages = keptMessages[1:]
				droppedChunks++
				droppedCount++
				droppedTokens += EstimateTokens(dropped)
				allDroppedMessages = append(allDroppedMessages, dropped)
			}
			break
		}

		// Drop the first (oldest) chunk
		dropped := chunks[0]
		droppedChunks++
		droppedCount += len(dropped)
		droppedTokens += EstimateMessagesTokens(dropped)
		allDroppedMessages = append(allDroppedMessages, dropped...)

		// Keep the rest
		keptMessages = make([]llm.Message, 0)
		for i := 1; i < len(chunks); i++ {
			keptMessages = append(keptMessages, chunks[i]...)
		}
	}

	// Sanitize: remove orphaned tool results whose tool_use has been pruned.
	keptMessages, orphaned := sanitizeToolPairs(keptMessages)
	if len(orphaned) > 0 {
		allDroppedMessages = append(allDroppedMessages, orphaned...)
		droppedCount += len(orphaned)
		droppedTokens += EstimateMessagesTokens(orphaned)
	}

	return PruneResult{
		Messages:        keptMessages,
		DroppedMessages: allDroppedMessages,
		DroppedChunks:   droppedChunks,
		DroppedCount:    droppedCount,
		DroppedTokens:   droppedTokens,
		KeptTokens:      EstimateMessagesTokens(keptMessages),
		BudgetTokens:    budgetTokens,
	}
}

// sanitizeToolPairs removes orphaned tool result messages whose ToolCallID
// doesn't match any ToolCall.ID in the kept assistant messages. It also
// removes assistant messages with ToolCalls if none of their tool results
// are present (to avoid the API complaining about missing tool results).
func sanitizeToolPairs(messages []llm.Message) (kept []llm.Message, dropped []llm.Message) {
	// Build set of all tool_call IDs from assistant messages.
	toolCallIDs := make(map[string]struct{})
	for _, msg := range messages {
		if msg.Role == llm.RoleAssistant {
			for _, tc := range msg.ToolCalls {
				toolCallIDs[tc.ID] = struct{}{}
			}
		}
	}

	// Build set of all tool_result IDs from tool messages.
	toolResultIDs := make(map[string]struct{})
	for _, msg := range messages {
		if msg.Role == llm.RoleTool && msg.ToolCallID != "" {
			toolResultIDs[msg.ToolCallID] = struct{}{}
		}
	}

	kept = make([]llm.Message, 0, len(messages))
	for _, msg := range messages {
		if msg.Role == llm.RoleTool && msg.ToolCallID != "" {
			// Drop tool result if its tool_use was pruned.
			if _, ok := toolCallIDs[msg.ToolCallID]; !ok {
				dropped = append(dropped, msg)
				continue
			}
		}
		if msg.Role == llm.RoleAssistant && len(msg.ToolCalls) > 0 {
			// Drop assistant tool_use if ALL of its results were pruned.
			hasAnyResult := false
			for _, tc := range msg.ToolCalls {
				if _, ok := toolResultIDs[tc.ID]; ok {
					hasAnyResult = true
					break
				}
			}
			if !hasAnyResult {
				dropped = append(dropped, msg)
				continue
			}
		}
		kept = append(kept, msg)
	}
	return kept, dropped
}

// Summarize generates a summary of the given messages.
func (c *Compactor) Summarize(ctx context.Context, messages []llm.Message, previousSummary string) (string, error) {
	if len(messages) == 0 {
		if previousSummary != "" {
			return previousSummary, nil
		}
		return DefaultSummaryFallback, nil
	}

	// Build summarization prompt
	var sb strings.Builder
	sb.WriteString("Summarize this conversation for future continuation context.\n")
	sb.WriteString(StructuredSummaryInstructions(c.config.CustomInstructions))
	sb.WriteString("\n\n")

	if previousSummary != "" {
		sb.WriteString("Previous context summary:\n")
		sb.WriteString(previousSummary)
		sb.WriteString("\n\n")
	}

	sb.WriteString("Conversation to summarize:\n")
	for _, msg := range messages {
		sb.WriteString(string(msg.Role))
		sb.WriteString(": ")
		sb.WriteString(messageSummaryText(msg))
		sb.WriteString("\n\n")
	}

	// Call LLM for summarization
	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: sb.String()},
		},
		MaxTokens: c.config.ReserveTokens,
	}

	resp, err := c.provider.Chat(ctx, req)
	if err != nil {
		return DefaultSummaryFallback, err
	}

	summary := NormalizeStructuredSummary(resp.Message.Content, previousSummary, messages)
	if summary == "" {
		return DefaultSummaryFallback, nil
	}
	return summary, nil
}

// CompactMessages compacts messages to fit within context limits.
func (c *Compactor) CompactMessages(ctx context.Context, messages []llm.Message) ([]llm.Message, string, error) {
	totalTokens := EstimateMessagesTokens(messages)
	// Reserve space for system prompt, memory context, tools, and response.
	// These are injected AFTER compaction, so we must account for them here.
	reserveTokens := c.config.ReserveTokens
	if reserveTokens < 4096 {
		reserveTokens = 4096
	}
	budgetTokens := int(float64(c.config.MaxContextTokens)*c.config.MaxHistoryShare) - reserveTokens

	// If within budget, no compaction needed
	if totalTokens <= budgetTokens {
		return messages, "", nil
	}

	// Prune history
	result := PruneHistoryForContextShare(
		messages,
		c.config.MaxContextTokens,
		c.config.MaxHistoryShare,
		DefaultParts,
	)

	// If we dropped messages, generate a summary
	var summary string
	if len(result.DroppedMessages) > 0 {
		var err error
		summary, err = c.Summarize(ctx, result.DroppedMessages, "")
		if err != nil {
			// Log error but continue with pruned messages
			summary = DefaultSummaryFallback
		}
	}

	return result.Messages, summary, nil
}

// normalizeParts ensures parts is within valid range.
func normalizeParts(parts, messageCount int) int {
	if parts <= 1 {
		return 1
	}
	if parts > messageCount {
		return messageCount
	}
	return parts
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
