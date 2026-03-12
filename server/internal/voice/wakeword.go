package voice

import (
	"regexp"
	"strings"
	"unicode"
)

func normalizeWakeWordText(input string) string {
	lowered := strings.ToLower(strings.TrimSpace(input))
	if lowered == "" {
		return ""
	}
	cleaned := strings.Map(func(r rune) rune {
		switch {
		case unicode.IsLetter(r), unicode.IsNumber(r), unicode.IsSpace(r):
			return r
		default:
			return ' '
		}
	}, lowered)
	return strings.Join(strings.Fields(cleaned), " ")
}

func transcriptContainsWakeWord(transcript, wakeWord string) bool {
	normalizedWakeWord := normalizeWakeWordText(wakeWord)
	if normalizedWakeWord == "" {
		return false
	}
	normalizedTranscript := normalizeWakeWordText(transcript)
	if normalizedTranscript == "" {
		return false
	}
	return strings.Contains(normalizedTranscript, normalizedWakeWord)
}

func stripWakeWordFromTranscript(transcript, wakeWord string) string {
	trimmedTranscript := strings.TrimSpace(transcript)
	trimmedWakeWord := strings.TrimSpace(wakeWord)
	if trimmedTranscript == "" || trimmedWakeWord == "" {
		return trimmedTranscript
	}

	// Remove a single wake-word occurrence while preserving the original transcript.
	re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(trimmedWakeWord))
	withoutWakeWord := re.ReplaceAllString(trimmedTranscript, "")
	return strings.TrimSpace(withoutWakeWord)
}
