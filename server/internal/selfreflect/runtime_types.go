package selfreflect

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

var allowedMutationSuggestionKinds = map[string]struct{}{
	"instruction_add":     {},
	"instruction_replace": {},
	"guardrail_add":       {},
	"workflow_capture":    {},
	"anti_pattern":        {},
}

func normalizeRuntimeEvidenceItem(item RuntimeEvidenceItem) RuntimeEvidenceItem {
	item.ID = strings.TrimSpace(item.ID)
	item.EventType = strings.TrimSpace(item.EventType)
	item.Summary = cleanSentence(item.Summary)
	item.PayloadJSON = strings.TrimSpace(item.PayloadJSON)
	return item
}

func cloneRuntimeSignals(in map[string]interface{}) map[string]interface{} {
	return cloneInterfaceMap(in)
}

func normalizeSignalStrength(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "low", "medium", "high":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

func normalizeMutationSuggestions(in []MutationSuggestion) []MutationSuggestion {
	if len(in) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]MutationSuggestion, 0, len(in))
	for _, suggestion := range in {
		suggestion.Kind = strings.ToLower(strings.TrimSpace(suggestion.Kind))
		if _, ok := allowedMutationSuggestionKinds[suggestion.Kind]; !ok {
			continue
		}
		suggestion.TargetSkillID = strings.TrimSpace(suggestion.TargetSkillID)
		suggestion.Rationale = cleanSentence(suggestion.Rationale)
		suggestion.SuggestedText = cleanSentence(suggestion.SuggestedText)
		filteredIDs := make([]string, 0, len(suggestion.EvidenceIDs))
		for _, evidenceID := range suggestion.EvidenceIDs {
			if id := strings.TrimSpace(evidenceID); id != "" {
				filteredIDs = append(filteredIDs, id)
			}
		}
		suggestion.EvidenceIDs = filteredIDs
		if suggestion.TargetSkillID == "" || suggestion.Rationale == "" || suggestion.SuggestedText == "" || len(suggestion.EvidenceIDs) < 2 {
			continue
		}
		if suggestion.Signature == "" {
			suggestion.Signature = buildMutationSuggestionSignature(suggestion)
		} else {
			suggestion.Signature = normalizeDedupKey(suggestion.Signature)
		}
		key := strings.Join([]string{
			suggestion.Kind,
			suggestion.TargetSkillID,
			suggestion.Signature,
		}, "::")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, suggestion)
	}
	sort.SliceStable(out, func(i, j int) bool {
		left := out[i].Kind + "::" + out[i].TargetSkillID + "::" + out[i].Signature
		right := out[j].Kind + "::" + out[j].TargetSkillID + "::" + out[j].Signature
		return left < right
	})
	if len(out) == 0 {
		return nil
	}
	return out
}

func buildMutationSuggestionSignature(suggestion MutationSuggestion) string {
	base := strings.Join([]string{
		suggestion.Kind,
		suggestion.TargetSkillID,
		normalizeDedupKey(suggestion.Rationale),
		normalizeDedupKey(suggestion.SuggestedText),
	}, "|")
	sum := sha256.Sum256([]byte(base))
	return hex.EncodeToString(sum[:8])
}

func refreshReflectionSignature(result *Result) {
	if result == nil {
		return
	}
	result.ReflectionSignature = strings.TrimSpace(result.ReflectionSignature)
	if result.ReflectionSignature != "" {
		return
	}
	parts := make([]string, 0, len(result.MutationSuggestions)+len(result.Lessons))
	for _, suggestion := range result.MutationSuggestions {
		parts = append(parts, strings.Join([]string{
			suggestion.Kind,
			suggestion.TargetSkillID,
			suggestion.Signature,
		}, "|"))
	}
	for _, lesson := range result.Lessons {
		parts = append(parts, strings.Join([]string{
			string(lesson.Kind),
			normalizeDedupKey(lesson.Lesson),
			normalizeDedupKey(lesson.WhenToApply),
		}, "|"))
	}
	if len(parts) == 0 {
		return
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "||")))
	result.ReflectionSignature = fmt.Sprintf("rr-%s", hex.EncodeToString(sum[:8]))
}
