package pruner

import (
	"strings"
	"unicode"
)

// stopwords is a set of common English words to filter from non-code content.
var stopwords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "as": true, "at": true,
	"be": true, "by": true, "do": true, "for": true, "from": true,
	"has": true, "have": true, "he": true, "her": true, "his": true,
	"how": true, "if": true, "in": true, "is": true, "it": true, "its": true,
	"me": true, "my": true, "no": true, "not": true, "of": true, "on": true,
	"or": true, "our": true, "she": true, "so": true, "that": true,
	"the": true, "their": true, "them": true, "then": true, "there": true,
	"these": true, "they": true, "this": true, "to": true, "up": true,
	"us": true, "was": true, "we": true, "were": true, "what": true,
	"when": true, "which": true, "who": true, "will": true, "with": true,
	"you": true, "your": true,
}

// textTokenize splits text into lowercase tokens with stopword removal.
// Designed for non-code content (docs, logs, prose).
func textTokenize(text string) []string {
	if text == "" {
		return nil
	}

	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	result := make([]string, 0, len(words))
	for _, w := range words {
		if len(w) >= 2 && !stopwords[w] {
			result = append(result, w)
		}
	}
	return result
}

// UnifiedTokenize dispatches to the appropriate tokenizer based on content type.
func UnifiedTokenize(text string, ct ContentType) []string {
	switch ct {
	case ContentCode:
		return codeTokenize(text)
	default:
		return textTokenize(text)
	}
}
