package memory

import (
	"math"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pruner"
)

// ConversationTurn represents a single turn in a conversation.
type ConversationTurn struct {
	Role    string
	Content string
}

// IRExtractResult holds the result of IR-based memory extraction.
type IRExtractResult struct {
	// KeySentences are the most important sentences extracted from the conversation.
	KeySentences []string
	// Score is the overall importance score (0-1). Below threshold means "nothing worth remembering".
	Score float32
}

// ExtractFromConversation performs offline IR-based extraction of key information
// from a conversation. No LLM needed — uses TF-IDF scoring + signal word boosting.
//
// Returns nil if the conversation has nothing worth remembering.
func ExtractFromConversation(turns []ConversationTurn, maxSentences int) *IRExtractResult {
	if len(turns) == 0 {
		return nil
	}
	if maxSentences <= 0 {
		maxSentences = 8
	}

	type scoredSentence struct {
		text  string
		score float64
		role  string
	}

	var allSentences []scoredSentence

	// Build corpus-level document frequency
	var allTokenSets []map[string]bool

	for _, turn := range turns {
		if turn.Role == "system" {
			continue
		}
		sentences := pruner.SplitSentences(turn.Content)
		for _, s := range sentences {
			s = strings.TrimSpace(s)
			if len(s) < 10 {
				continue
			}
			tokens := pruner.TextTokenize(s)
			tokenSet := make(map[string]bool, len(tokens))
			for _, t := range tokens {
				tokenSet[t] = true
			}
			allTokenSets = append(allTokenSets, tokenSet)
			allSentences = append(allSentences, scoredSentence{
				text: s,
				role: turn.Role,
			})
		}
	}

	if len(allSentences) == 0 {
		return nil
	}

	// Compute document frequency
	n := float64(len(allSentences))
	df := make(map[string]int)
	for _, ts := range allTokenSets {
		for t := range ts {
			df[t]++
		}
	}

	// Score each sentence using TF-IDF + signal word boost
	var maxScore float64
	for i := range allSentences {
		tokens := pruner.TextTokenize(allSentences[i].text)
		if len(tokens) == 0 {
			continue
		}

		// TF-IDF score
		tf := make(map[string]int)
		for _, t := range tokens {
			tf[t]++
		}
		var tfidfScore float64
		for term, freq := range tf {
			docFreq := float64(df[term])
			idf := math.Log((n + 1) / (docFreq + 1))
			tfidfScore += float64(freq) * idf
		}
		// Normalize by sentence length
		tfidfScore /= float64(len(tokens))

		// Signal word boost
		signalBoost := pruner.SignalWordScore(allSentences[i].text, pruner.DefaultMemorySignals)

		// User messages are more important for memory
		roleBoost := 1.0
		if allSentences[i].role == "user" {
			roleBoost = 1.3
		}

		allSentences[i].score = tfidfScore * roleBoost * (1.0 + signalBoost)
		if allSentences[i].score > maxScore {
			maxScore = allSentences[i].score
		}
	}

	// Normalize scores to 0-1
	if maxScore > 0 {
		for i := range allSentences {
			allSentences[i].score /= maxScore
		}
	}

	// Sort by score descending (simple selection sort — small N)
	for i := 0; i < len(allSentences)-1; i++ {
		for j := i + 1; j < len(allSentences); j++ {
			if allSentences[j].score > allSentences[i].score {
				allSentences[i], allSentences[j] = allSentences[j], allSentences[i]
			}
		}
	}

	// Filter: only keep sentences with score above threshold
	const minScore = 0.3
	var keySentences []string
	var totalScore float32
	for i := 0; i < len(allSentences) && len(keySentences) < maxSentences; i++ {
		if allSentences[i].score < minScore {
			break
		}
		keySentences = append(keySentences, allSentences[i].text)
		totalScore += float32(allSentences[i].score)
	}

	if len(keySentences) == 0 {
		return nil
	}

	avgScore := totalScore / float32(len(keySentences))
	return &IRExtractResult{
		KeySentences: keySentences,
		Score:        avgScore,
	}
}

// FormatAsMemory formats IR extraction results as a markdown memory entry.
func (r *IRExtractResult) FormatAsMemory() string {
	if r == nil || len(r.KeySentences) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, s := range r.KeySentences {
		sb.WriteString("- ")
		sb.WriteString(s)
		sb.WriteString("\n")
	}
	return sb.String()
}
