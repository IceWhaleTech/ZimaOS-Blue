package server

import (
	"encoding/json"
	"strings"
)

const (
	finalizationModeReplace           = "replace"
	finalizationModeMergeProcessCards = "merge_process_cards"
)

var streamProcessCardTypes = map[string]struct{}{
	"ui-review-progress":     {},
	"analyze-progress":       {},
	"browser-progress":       {},
	"deep-research-progress": {},
	"deep-research-event":    {},
	"deep-research-timeline": {},
}

type typelessBlockSummary struct {
	Raw  string
	Type string
	Key  string
}

func extractTypelessBlocksByType(content string) []typelessBlockSummary {
	if !strings.Contains(content, "```typeless") {
		return nil
	}

	matches := reTypelessBlock.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}

	out := make([]typelessBlockSummary, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		raw := strings.TrimSpace(match[0])
		payload := strings.TrimSpace(match[1])
		if raw == "" || payload == "" {
			continue
		}

		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
			continue
		}

		cardType, _ := parsed["type"].(string)
		cardType = strings.TrimSpace(cardType)
		if cardType == "" {
			continue
		}

		cardID, _ := parsed["id"].(string)
		cardID = strings.TrimSpace(cardID)
		key := raw
		if cardID != "" {
			key = cardType + ":" + cardID
		}

		out = append(out, typelessBlockSummary{
			Raw:  raw,
			Type: cardType,
			Key:  key,
		})
	}

	return out
}

func resolveFinalStreamContentMode(previousContent, finalContent string) string {
	previousBlocks := extractTypelessBlocksByType(previousContent)
	if len(previousBlocks) == 0 {
		return finalizationModeReplace
	}

	previousProcessKeys := make(map[string]struct{})
	for _, block := range previousBlocks {
		if _, ok := streamProcessCardTypes[block.Type]; !ok {
			continue
		}
		previousProcessKeys[block.Key] = struct{}{}
	}
	if len(previousProcessKeys) == 0 {
		return finalizationModeReplace
	}

	finalBlocks := extractTypelessBlocksByType(finalContent)
	finalProcessKeys := make(map[string]struct{})
	for _, block := range finalBlocks {
		if _, ok := streamProcessCardTypes[block.Type]; !ok {
			continue
		}
		finalProcessKeys[block.Key] = struct{}{}
	}

	for key := range previousProcessKeys {
		if _, ok := finalProcessKeys[key]; !ok {
			return finalizationModeMergeProcessCards
		}
	}

	return finalizationModeReplace
}
