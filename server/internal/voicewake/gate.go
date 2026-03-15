package voicewake

import (
	"regexp"
	"strings"
	"unicode"
)

type Segment struct {
	Text     string
	Start    float64
	Duration float64
}

func (s Segment) End() float64 {
	return s.Start + s.Duration
}

type GateConfig struct {
	Triggers          []string
	MinPostTriggerGap float64
	MinCommandLength  int
}

type GateMatch struct {
	TriggerFound      bool
	TriggerEndSeconds float64
	PostGapSeconds    float64
	Command           string
}

type gateToken struct {
	normalized string
	start      float64
	end        float64
	text       string
}

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

func TranscriptContainsWakeWord(transcript string, triggers []string) bool {
	normalizedTranscript := normalizeWakeWordText(transcript)
	if normalizedTranscript == "" {
		return false
	}
	for _, trigger := range defaultTriggers(triggers) {
		normalizedTrigger := normalizeWakeWordText(trigger)
		if normalizedTrigger == "" {
			continue
		}
		if strings.Contains(normalizedTranscript, normalizedTrigger) {
			return true
		}
	}
	return false
}

func StripWakeWordFromTranscript(transcript string, triggers []string) string {
	trimmed := strings.TrimSpace(transcript)
	if trimmed == "" {
		return ""
	}
	out := trimmed
	for _, trigger := range defaultTriggers(triggers) {
		trigger = strings.TrimSpace(trigger)
		if trigger == "" {
			continue
		}
		re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(trigger))
		out = re.ReplaceAllString(out, "")
	}
	return strings.TrimSpace(out)
}

func CommandTextAfterTrigger(transcript string, segments []Segment, triggerEndSeconds float64) string {
	if strings.TrimSpace(transcript) == "" {
		return ""
	}
	threshold := triggerEndSeconds + 0.001
	parts := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment.Start < threshold {
			continue
		}
		text := strings.TrimSpace(segment.Text)
		if normalizeWakeWordText(text) == "" {
			continue
		}
		parts = append(parts, text)
	}
	if len(parts) == 0 {
		return strings.TrimSpace(transcript)
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func MatchSegments(transcript string, segments []Segment, cfg GateConfig) *GateMatch {
	triggerTokens := normalizeTriggerTokens(cfg.Triggers)
	if len(triggerTokens) == 0 {
		return nil
	}
	tokens := normalizeSegments(segments)
	if len(tokens) == 0 {
		if TranscriptContainsWakeWord(transcript, cfg.Triggers) {
			return &GateMatch{TriggerFound: true}
		}
		return nil
	}

	bestIndex := -1
	best := GateMatch{}
	for _, trigger := range triggerTokens {
		count := len(trigger)
		if count == 0 || len(tokens) < count {
			continue
		}
		for i := 0; i <= len(tokens)-count; i++ {
			matched := true
			for j := 0; j < count; j++ {
				if tokens[i+j].normalized != trigger[j] {
					matched = false
					break
				}
			}
			if !matched {
				continue
			}
			candidate := GateMatch{TriggerFound: true, TriggerEndSeconds: tokens[i+count-1].end}
			if i+count < len(tokens) {
				candidate.PostGapSeconds = tokens[i+count].start - candidate.TriggerEndSeconds
				if candidate.PostGapSeconds >= cfg.MinPostTriggerGap {
					candidate.Command = strings.TrimSpace(CommandTextAfterTrigger(transcript, segments, candidate.TriggerEndSeconds))
				}
			}
			if candidate.Command != "" && len([]rune(candidate.Command)) < cfg.MinCommandLength {
				candidate.Command = ""
			}
			if i >= bestIndex {
				bestIndex = i
				best = candidate
			}
		}
	}
	if bestIndex < 0 {
		return nil
	}
	return &best
}

func normalizeTriggerTokens(triggers []string) [][]string {
	out := make([][]string, 0, len(triggers))
	for _, trigger := range defaultTriggers(triggers) {
		normalized := normalizeWakeWordText(trigger)
		if normalized == "" {
			continue
		}
		parts := strings.Fields(normalized)
		if len(parts) == 0 {
			continue
		}
		out = append(out, parts)
	}
	return out
}

func normalizeSegments(segments []Segment) []gateToken {
	out := make([]gateToken, 0, len(segments))
	for _, segment := range segments {
		normalized := normalizeWakeWordText(segment.Text)
		if normalized == "" {
			continue
		}
		parts := strings.Fields(normalized)
		if len(parts) == 0 {
			continue
		}
		partDuration := segment.Duration
		if partDuration <= 0 {
			partDuration = 0.2
		}
		chunkDuration := partDuration / float64(len(parts))
		if chunkDuration <= 0 {
			chunkDuration = 0.2
		}
		for idx, part := range parts {
			start := segment.Start + (float64(idx) * chunkDuration)
			out = append(out, gateToken{
				normalized: part,
				start:      start,
				end:        start + chunkDuration,
				text:       part,
			})
		}
	}
	return out
}
