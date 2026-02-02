package memory

import (
	"regexp"
	"strings"
	"time"
)

// ImportanceScorer calculates importance scores for memory chunks.
type ImportanceScorer struct {
	config ImportanceConfig
}

// ImportanceConfig holds configuration for importance scoring.
type ImportanceConfig struct {
	// Weight factors (0-1)
	RecencyWeight    float32 // How much recency matters
	LengthWeight     float32 // How much content length matters
	KeywordWeight    float32 // How much important keywords matter
	AccessWeight     float32 // How much access frequency matters

	// Important keywords that boost score
	ImportantKeywords []string

	// Decay settings
	RecencyDecayDays int // Days after which recency score starts decaying
}

// DefaultImportanceConfig returns default importance scoring configuration.
func DefaultImportanceConfig() ImportanceConfig {
	return ImportanceConfig{
		RecencyWeight:    0.3,
		LengthWeight:     0.1,
		KeywordWeight:    0.4,
		AccessWeight:     0.2,
		RecencyDecayDays: 7,
		ImportantKeywords: []string{
			"important", "critical", "remember", "key", "essential",
			"password", "secret", "api", "token", "credential",
			"deadline", "meeting", "appointment", "schedule",
			"bug", "fix", "error", "issue", "problem",
			"decision", "agreed", "confirmed", "approved",
			"todo", "task", "action", "follow-up",
		},
	}
}

// NewImportanceScorer creates a new importance scorer.
func NewImportanceScorer(cfg ImportanceConfig) *ImportanceScorer {
	if cfg.RecencyWeight == 0 && cfg.LengthWeight == 0 && cfg.KeywordWeight == 0 {
		cfg = DefaultImportanceConfig()
	}
	return &ImportanceScorer{config: cfg}
}

// Score calculates the importance score for a memory chunk.
func (s *ImportanceScorer) Score(chunk *MemoryChunk) float32 {
	var score float32

	// Recency score
	recencyScore := s.calculateRecencyScore(chunk.CreatedAt)
	score += recencyScore * s.config.RecencyWeight

	// Length score (longer content may be more important)
	lengthScore := s.calculateLengthScore(chunk.Content)
	score += lengthScore * s.config.LengthWeight

	// Keyword score
	keywordScore := s.calculateKeywordScore(chunk.Content)
	score += keywordScore * s.config.KeywordWeight

	// Access score (from metadata if available)
	accessScore := s.calculateAccessScore(chunk.Metadata)
	score += accessScore * s.config.AccessWeight

	// Normalize to 0-1 range
	if score > 1.0 {
		score = 1.0
	}

	return score
}

// ScoreWithDetails returns detailed scoring breakdown.
func (s *ImportanceScorer) ScoreWithDetails(chunk *MemoryChunk) ImportanceDetails {
	recencyScore := s.calculateRecencyScore(chunk.CreatedAt)
	lengthScore := s.calculateLengthScore(chunk.Content)
	keywordScore := s.calculateKeywordScore(chunk.Content)
	accessScore := s.calculateAccessScore(chunk.Metadata)

	combined := recencyScore*s.config.RecencyWeight +
		lengthScore*s.config.LengthWeight +
		keywordScore*s.config.KeywordWeight +
		accessScore*s.config.AccessWeight

	if combined > 1.0 {
		combined = 1.0
	}

	return ImportanceDetails{
		CombinedScore:  combined,
		RecencyScore:   recencyScore,
		LengthScore:    lengthScore,
		KeywordScore:   keywordScore,
		AccessScore:    accessScore,
		MatchedKeywords: s.findMatchedKeywords(chunk.Content),
	}
}

// ImportanceDetails holds detailed importance scoring breakdown.
type ImportanceDetails struct {
	CombinedScore   float32  `json:"combined_score"`
	RecencyScore    float32  `json:"recency_score"`
	LengthScore     float32  `json:"length_score"`
	KeywordScore    float32  `json:"keyword_score"`
	AccessScore     float32  `json:"access_score"`
	MatchedKeywords []string `json:"matched_keywords,omitempty"`
}

// calculateRecencyScore returns a score based on how recent the memory is.
func (s *ImportanceScorer) calculateRecencyScore(createdAt time.Time) float32 {
	daysSince := time.Since(createdAt).Hours() / 24

	if daysSince <= float64(s.config.RecencyDecayDays) {
		return 1.0
	}

	// Exponential decay after threshold
	decayFactor := daysSince - float64(s.config.RecencyDecayDays)
	score := float32(1.0 / (1.0 + decayFactor/30.0)) // Half-life of ~30 days

	if score < 0.1 {
		score = 0.1 // Minimum score
	}

	return score
}

// calculateLengthScore returns a score based on content length.
func (s *ImportanceScorer) calculateLengthScore(content string) float32 {
	length := len(content)

	// Optimal length range: 100-500 characters
	if length < 50 {
		return 0.3 // Too short
	}
	if length < 100 {
		return 0.5
	}
	if length <= 500 {
		return 1.0 // Optimal
	}
	if length <= 1000 {
		return 0.8
	}
	return 0.6 // Very long
}

// calculateKeywordScore returns a score based on important keywords.
func (s *ImportanceScorer) calculateKeywordScore(content string) float32 {
	contentLower := strings.ToLower(content)
	matchCount := 0

	for _, keyword := range s.config.ImportantKeywords {
		if strings.Contains(contentLower, keyword) {
			matchCount++
		}
	}

	if matchCount == 0 {
		return 0.2 // Base score
	}

	// Diminishing returns for multiple matches
	score := float32(0.2 + 0.3*float64(matchCount))
	if score > 1.0 {
		score = 1.0
	}

	return score
}

// calculateAccessScore returns a score based on access frequency.
func (s *ImportanceScorer) calculateAccessScore(metadata map[string]string) float32 {
	if metadata == nil {
		return 0.5 // Default
	}

	// Check for access count in metadata
	if accessStr, ok := metadata["access_count"]; ok {
		count := parseIntFromString(accessStr)
		if count > 10 {
			return 1.0
		}
		if count > 5 {
			return 0.8
		}
		if count > 1 {
			return 0.6
		}
	}

	return 0.5
}

// findMatchedKeywords returns keywords found in content.
func (s *ImportanceScorer) findMatchedKeywords(content string) []string {
	contentLower := strings.ToLower(content)
	var matched []string

	for _, keyword := range s.config.ImportantKeywords {
		if strings.Contains(contentLower, keyword) {
			matched = append(matched, keyword)
		}
	}

	return matched
}

// parseIntFromString parses an integer from string.
func parseIntFromString(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			break
		}
	}
	return n
}

// ExtractImportantContent extracts important sentences from content.
func ExtractImportantContent(content string, maxSentences int) string {
	if maxSentences <= 0 {
		maxSentences = 3
	}

	// Split into sentences
	sentencePattern := regexp.MustCompile(`[.!?]+\s*`)
	sentences := sentencePattern.Split(content, -1)

	// Score each sentence
	scorer := NewImportanceScorer(DefaultImportanceConfig())
	type scoredSentence struct {
		text  string
		score float32
	}

	var scored []scoredSentence
	for _, s := range sentences {
		s = strings.TrimSpace(s)
		if len(s) < 10 {
			continue
		}
		score := scorer.calculateKeywordScore(s)
		scored = append(scored, scoredSentence{text: s, score: score})
	}

	// Sort by score descending
	for i := 0; i < len(scored)-1; i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// Take top sentences
	var result []string
	for i := 0; i < len(scored) && i < maxSentences; i++ {
		result = append(result, scored[i].text)
	}

	return strings.Join(result, ". ")
}
