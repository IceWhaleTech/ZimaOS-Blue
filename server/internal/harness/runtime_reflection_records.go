package harness

import (
	"encoding/json"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selfreflect"
)

type runtimeReflectionRecord struct {
	RecordID            string                           `json:"record_id,omitempty"`
	RunID               string                           `json:"run_id,omitempty"`
	TriggerKind         string                           `json:"trigger_kind,omitempty"`
	ReviewSeq           int                              `json:"review_seq,omitempty"`
	StepIndex           int                              `json:"step_index,omitempty"`
	ToolCountWindow     int                              `json:"tool_count_window,omitempty"`
	EvidenceEventIDs    []string                         `json:"evidence_event_ids,omitempty"`
	Signals             map[string]interface{}           `json:"signals,omitempty"`
	Summary             string                           `json:"summary,omitempty"`
	Lessons             []selfreflect.Lesson             `json:"lessons,omitempty"`
	MutationSuggestions []selfreflect.MutationSuggestion `json:"mutation_suggestions,omitempty"`
	ReflectionSignature string                           `json:"reflection_signature,omitempty"`
	Status              string                           `json:"status,omitempty"`
}

func runtimeReflectionRecordsFromEvents(events []RunEvent) []runtimeReflectionRecord {
	if len(events) == 0 {
		return nil
	}
	out := make([]runtimeReflectionRecord, 0, 4)
	for _, event := range events {
		eventType := strings.TrimSpace(event.Type)
		if eventType != "runtime_reflection_recorded" && eventType != "runtime_reflection_skipped" {
			continue
		}
		record := runtimeReflectionRecord{
			RecordID:  strings.TrimSpace(event.ID),
			RunID:     strings.TrimSpace(event.RunID),
			StepIndex: event.StepIndex,
			Summary:   strings.TrimSpace(event.Message),
			Status:    strings.TrimPrefix(eventType, "runtime_reflection_"),
		}
		if payload := strings.TrimSpace(event.PayloadJSON); payload != "" {
			_ = json.Unmarshal([]byte(payload), &record)
		}
		if record.RecordID == "" {
			record.RecordID = strings.TrimSpace(event.ID)
		}
		if record.RunID == "" {
			record.RunID = strings.TrimSpace(event.RunID)
		}
		if record.StepIndex == 0 {
			record.StepIndex = event.StepIndex
		}
		if record.Summary == "" {
			record.Summary = strings.TrimSpace(event.Message)
		}
		if record.Status == "" {
			record.Status = strings.TrimPrefix(eventType, "runtime_reflection_")
		}
		out = append(out, record)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func runtimeReflectionRecordMaps(records []runtimeReflectionRecord, limit int) []interface{} {
	if len(records) == 0 {
		return nil
	}
	if limit > 0 && len(records) > limit {
		records = records[len(records)-limit:]
	}
	out := make([]interface{}, 0, len(records))
	for _, record := range records {
		item := map[string]interface{}{
			"record_id":            strings.TrimSpace(record.RecordID),
			"run_id":               strings.TrimSpace(record.RunID),
			"trigger_kind":         strings.TrimSpace(record.TriggerKind),
			"review_seq":           record.ReviewSeq,
			"step_index":           record.StepIndex,
			"tool_count_window":    record.ToolCountWindow,
			"evidence_event_ids":   append([]string(nil), record.EvidenceEventIDs...),
			"signals":              cloneMetadataMap(record.Signals),
			"summary":              strings.TrimSpace(record.Summary),
			"lessons":              record.Lessons,
			"mutation_suggestions": record.MutationSuggestions,
			"reflection_signature": strings.TrimSpace(record.ReflectionSignature),
			"status":               strings.TrimSpace(record.Status),
		}
		out = append(out, item)
	}
	return out
}

func runtimeReflectionLessons(records []runtimeReflectionRecord) []string {
	if len(records) == 0 {
		return nil
	}
	out := make([]string, 0, len(records)*2)
	for _, record := range records {
		for _, lesson := range record.Lessons {
			value := strings.TrimSpace(lesson.Lesson)
			if value == "" {
				continue
			}
			out = append(out, value)
		}
		if len(record.Lessons) == 0 {
			for _, suggestion := range record.MutationSuggestions {
				value := strings.TrimSpace(suggestion.SuggestedText)
				if value == "" {
					continue
				}
				out = append(out, value)
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func runtimeReflectionMutationSuggestionMaps(records []runtimeReflectionRecord) []interface{} {
	if len(records) == 0 {
		return nil
	}
	out := make([]interface{}, 0, len(records))
	for _, record := range records {
		for _, suggestion := range record.MutationSuggestions {
			if strings.TrimSpace(suggestion.Kind) == "" {
				continue
			}
			out = append(out, map[string]interface{}{
				"kind":            strings.TrimSpace(suggestion.Kind),
				"target_skill_id": strings.TrimSpace(suggestion.TargetSkillID),
				"rationale":       strings.TrimSpace(suggestion.Rationale),
				"suggested_text":  strings.TrimSpace(suggestion.SuggestedText),
				"evidence_ids":    append([]string(nil), suggestion.EvidenceIDs...),
				"signature":       strings.TrimSpace(suggestion.Signature),
			})
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
