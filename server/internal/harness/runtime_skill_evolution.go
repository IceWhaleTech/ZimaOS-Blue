package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	runtimeSkillEvolutionEventType              = "runtime_skill_evolution_triggered"
	runtimeSkillEvolutionMaxEventScan           = 200
	runtimeSkillEvolutionMaxEvidenceEvents      = 8
	runtimeSkillEvolutionCaptureRepeatThreshold = 2
	runtimeSkillEvolutionFollowupGate           = "execution"
)

func (c *Controller) maybeEmitRuntimeSkillEvolutionTrigger(ctx context.Context, current *Run, snapshot *Run) {
	if c == nil || snapshot == nil {
		return
	}
	if current != nil && current.Status == snapshot.Status {
		return
	}
	if !isTerminalRunStatus(snapshot.Status) || runtimeSkillEvolutionIsOptimizationChild(snapshot.Metadata) {
		return
	}

	trigger, auditPayload, ok := c.buildRuntimeSkillEvolutionTrigger(ctx, snapshot)
	if !ok || trigger == nil {
		return
	}

	c.emitOptimizationTrigger(ctx, *trigger)
	_ = c.AppendEvent(ctx, RunEvent{
		RunID:       snapshot.ID,
		RootRunID:   snapshot.RootRunID,
		ParentRunID: snapshot.ParentRunID,
		Type:        runtimeSkillEvolutionEventType,
		Message:     strings.TrimSpace(string(trigger.Reason)),
		PayloadJSON: marshalMetadata(auditPayload),
	})
}

func (c *Controller) buildRuntimeSkillEvolutionTrigger(ctx context.Context, run *Run) (*OptimizationTrigger, map[string]interface{}, bool) {
	if c == nil || run == nil || strings.TrimSpace(run.ID) == "" {
		return nil, nil, false
	}
	skillID := runtimeSkillEvolutionCanonicalSkillID(run)
	if skillID == "" {
		return nil, nil, false
	}

	sourcePath := runtimeSkillEvolutionSourcePath(skillID)
	sourceState, err := ResolveCanonicalSkillSourceState(skillID, sourcePath)
	if err != nil {
		return nil, nil, false
	}

	events, err := c.store.ListEvents(ctx, run.ID, runtimeSkillEvolutionMaxEventScan)
	if err != nil {
		events = nil
	}
	reflectionRecords := runtimeReflectionRecordsFromEvents(events)

	candidateID := runtimeSkillEvolutionCandidateID(skillID, sourceState.ContentSHA256)
	metadata := map[string]interface{}{
		"runtime_run_id":           strings.TrimSpace(run.ID),
		"owner_user_id":            strings.TrimSpace(run.UserID),
		"runtime_kind":             strings.TrimSpace(string(run.Kind)),
		"runtime_status":           strings.TrimSpace(string(run.Status)),
		"runtime_state":            strings.TrimSpace(string(run.RuntimeState)),
		"goal":                     strings.TrimSpace(run.Goal),
		"result_summary":           runtimeSkillEvolutionTrimmedValue(run.Result, 2000),
		"error":                    runtimeSkillEvolutionTrimmedValue(run.Error, 800),
		"selected_canonical_skill": skillID,
		"followup_gate":            runtimeSkillEvolutionFollowupGate,
		"runtime_event_summaries":  runtimeSkillEvolutionEventSummaries(events, runtimeSkillEvolutionMaxEvidenceEvents),
		"skill_candidate": map[string]interface{}{
			"skill_id":     skillID,
			"candidate_id": candidateID,
			"source_path":  strings.TrimSpace(sourceState.NormalizedPath),
			"content":      sourceState.Content,
			"sha256":       strings.TrimSpace(sourceState.ContentSHA256),
		},
	}
	if model := strings.TrimSpace(run.Model); model != "" {
		metadata["runtime_model"] = model
	}
	if metrics := runtimeSkillEvolutionMetrics(run, events); len(metrics) > 0 {
		metadata["runtime_metrics"] = metrics
	}
	if usage := runtimeSkillEvolutionUsage(run, events); len(usage) > 0 {
		metadata["runtime_usage"] = usage
	}
	if quality := runtimeSkillEvolutionQuality(run, events); len(quality) > 0 {
		metadata["runtime_quality"] = quality
	}
	if validation := runtimeSkillEvolutionValidation(run, events); len(validation) > 0 {
		metadata["validation"] = validation
	}
	if reflections := runtimeReflectionRecordMaps(reflectionRecords, 3); len(reflections) > 0 {
		metadata["runtime_reflections"] = reflections
	}
	if suggestions := runtimeReflectionMutationSuggestionMaps(reflectionRecords); len(suggestions) > 0 {
		metadata["runtime_mutation_suggestions"] = suggestions
	}

	if structured := nestedMetadataMap(run.Metadata, "selector_dry_run_response"); len(structured) > 0 {
		metadata["selector_dry_run_response"] = cloneMetadataMap(structured)
	}

	switch run.Status {
	case RunStatusFailed, RunStatusAborted:
		failureSignature := runtimeSkillEvolutionFailureSignature(run, events)
		metadata["failure_signature"] = failureSignature
		metadata["runtime_failure_signature"] = failureSignature
		metadata["runtime_reason_summary"] = fmt.Sprintf("Runtime execution failed after selecting canonical skill %s.", skillID)
		metadata["reflective_evidence_packet"] = runtimeSkillEvolutionReflectiveEvidencePacket("runtime_failure", skillID, metadata)
		return &OptimizationTrigger{
				Reason:              OptimizationReasonRuntimeSkillFailure,
				CandidateID:         candidateID,
				OptimizationSurface: OptimizationSurfaceSkillDefinition,
				Metadata:            metadata,
			}, map[string]interface{}{
				"skill_id":          skillID,
				"candidate_id":      candidateID,
				"reason":            string(OptimizationReasonRuntimeSkillFailure),
				"failure_signature": failureSignature,
				"followup_gate":     runtimeSkillEvolutionFollowupGate,
			}, true
	case RunStatusCompleted:
		captureSignature, lessons := runtimeSkillEvolutionCaptureSignature(events)
		if captureSignature == "" || len(lessons) == 0 {
			return nil, nil, false
		}
		occurrences := c.runtimeSkillEvolutionCaptureOccurrences(ctx, run, skillID, captureSignature)
		if occurrences < runtimeSkillEvolutionCaptureRepeatThreshold {
			return nil, nil, false
		}
		metadata["capture_signature"] = captureSignature
		metadata["runtime_capture_lessons"] = append([]string(nil), lessons...)
		metadata["runtime_capture_occurrences"] = occurrences
		metadata["runtime_capture_threshold"] = runtimeSkillEvolutionCaptureRepeatThreshold
		metadata["runtime_reason_summary"] = fmt.Sprintf("Runtime execution repeated grounded lessons for canonical skill %s.", skillID)
		metadata["reflective_evidence_packet"] = runtimeSkillEvolutionReflectiveEvidencePacket("runtime_capture", skillID, metadata)
		return &OptimizationTrigger{
				Reason:              OptimizationReasonRuntimeSkillCapture,
				CandidateID:         candidateID,
				OptimizationSurface: OptimizationSurfaceSkillDefinition,
				Metadata:            metadata,
			}, map[string]interface{}{
				"skill_id":            skillID,
				"candidate_id":        candidateID,
				"reason":              string(OptimizationReasonRuntimeSkillCapture),
				"capture_signature":   captureSignature,
				"capture_occurrences": occurrences,
				"followup_gate":       runtimeSkillEvolutionFollowupGate,
			}, true
	default:
		return nil, nil, false
	}
}

func runtimeSkillEvolutionReflectiveEvidencePacket(kind string, skillID string, metadata map[string]interface{}) map[string]interface{} {
	packet := map[string]interface{}{
		"kind":                     strings.TrimSpace(kind),
		"selected_canonical_skill": strings.TrimSpace(skillID),
		"reason_summary":           strings.TrimSpace(metadataString(metadata, "runtime_reason_summary")),
		"failure_signature":        strings.TrimSpace(metadataString(metadata, "failure_signature")),
		"capture_signature":        strings.TrimSpace(metadataString(metadata, "capture_signature")),
		"grounded_lessons":         stringSliceAsInterfaces(stringSliceMetadataValue(metadata["runtime_capture_lessons"])),
		"self_reflect_lessons":     stringSliceAsInterfaces(stringSliceMetadataValue(metadata["runtime_capture_lessons"])),
		"diagnostic_signals": map[string]interface{}{
			"runtime_metrics":         cloneMetadataMap(nestedMetadataMap(metadata, "runtime_metrics")),
			"runtime_usage":           cloneMetadataMap(nestedMetadataMap(metadata, "runtime_usage")),
			"runtime_quality":         cloneMetadataMap(nestedMetadataMap(metadata, "runtime_quality")),
			"runtime_validation":      cloneMetadataMap(nestedMetadataMap(metadata, "validation")),
			"runtime_event_summaries": cloneInterfaceSlice(metadata["runtime_event_summaries"]),
		},
	}
	if reflections := cloneInterfaceSlice(metadata["runtime_reflections"]); len(reflections) > 0 {
		packet["runtime_reflections"] = reflections
	}
	if suggestions := cloneInterfaceSlice(metadata["runtime_mutation_suggestions"]); len(suggestions) > 0 {
		packet["mutation_suggestions"] = suggestions
	}
	if occurrences := metadata["runtime_capture_occurrences"]; occurrences != nil {
		packet["capture_occurrences"] = occurrences
	}
	if threshold := metadata["runtime_capture_threshold"]; threshold != nil {
		packet["capture_threshold"] = threshold
	}
	return packet
}

func stringSliceMetadataValue(raw interface{}) []string {
	items, ok := raw.([]string)
	if ok {
		return append([]string(nil), items...)
	}
	typed, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(typed))
	for _, item := range typed {
		value := strings.TrimSpace(fmt.Sprint(item))
		if value == "" || value == "<nil>" {
			continue
		}
		out = append(out, value)
	}
	return out
}

func cloneInterfaceSlice(raw interface{}) []interface{} {
	items, ok := raw.([]interface{})
	if !ok || len(items) == 0 {
		return nil
	}
	out := make([]interface{}, 0, len(items))
	for _, item := range items {
		switch typed := item.(type) {
		case map[string]interface{}:
			out = append(out, cloneMetadataMap(typed))
		default:
			out = append(out, typed)
		}
	}
	return out
}

func stringSliceAsInterfaces(items []string) []interface{} {
	if len(items) == 0 {
		return nil
	}
	out := make([]interface{}, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	return out
}

func (c *Controller) runtimeSkillEvolutionCaptureOccurrences(ctx context.Context, run *Run, skillID string, signature string) int {
	if c == nil || c.store == nil || run == nil || signature == "" {
		return 1
	}
	count := 1
	if strings.TrimSpace(run.UserID) == "" {
		return count
	}

	runs, err := c.store.ListRuns(ctx, RunFilter{
		UserID:   strings.TrimSpace(run.UserID),
		Statuses: []RunStatus{RunStatusCompleted},
		Limit:    runtimeSkillEvolutionMaxEventScan,
	})
	if err != nil {
		return count
	}
	for i := range runs {
		candidate := runs[i]
		if strings.TrimSpace(candidate.ID) == strings.TrimSpace(run.ID) {
			continue
		}
		if runtimeSkillEvolutionIsOptimizationChild(candidate.Metadata) {
			continue
		}
		if runtimeSkillEvolutionCanonicalSkillID(&candidate) != skillID {
			continue
		}
		events, eventErr := c.store.ListEvents(ctx, candidate.ID, runtimeSkillEvolutionMaxEventScan)
		if eventErr != nil {
			continue
		}
		candidateSignature, _ := runtimeSkillEvolutionCaptureSignature(events)
		if candidateSignature == signature {
			count++
			if count >= runtimeSkillEvolutionCaptureRepeatThreshold {
				return count
			}
		}
	}
	return count
}

func runtimeSkillEvolutionCanonicalSkillID(run *Run) string {
	if run == nil {
		return ""
	}
	meta := run.Metadata
	structured := nestedMetadataMap(meta, "selector_dry_run_response")
	discovery := decodeDiscoverFirstObservation(structured)
	for _, raw := range []string{
		metadataString(meta, "selected_canonical_skill"),
		metadataString(meta, "canonical_skill_id"),
		metadataString(nestedMetadataMap(meta, "contextpack_snapshot"), "selected_skill"),
		metadataString(meta, "selected_skill"),
		discovery.SelectedCanonicalSkill,
	} {
		if normalized := normalizeDiscoverFirstCanonicalSkill(raw); normalized != "" {
			return normalized
		}
	}
	return ""
}

func runtimeSkillEvolutionSourcePath(skillID string) string {
	skillID = strings.TrimSpace(strings.ReplaceAll(skillID, "\\", "/"))
	if skillID == "" {
		return ""
	}
	return normalizeSkillSourcePath("assets/skills/" + skillID + "/SKILL.md")
}

func runtimeSkillEvolutionCandidateID(skillID string, contentSHA string) string {
	normalizedSkillID := strings.Trim(strings.ReplaceAll(skillID, "/", "-"), "-")
	contentSHA = strings.TrimSpace(contentSHA)
	if len(contentSHA) > 12 {
		contentSHA = contentSHA[:12]
	}
	if normalizedSkillID == "" {
		normalizedSkillID = "skill"
	}
	if contentSHA == "" {
		return "runtime-" + normalizedSkillID
	}
	return fmt.Sprintf("runtime-%s-%s", normalizedSkillID, contentSHA)
}

func runtimeSkillEvolutionIsOptimizationChild(metadata map[string]interface{}) bool {
	if len(metadata) == 0 {
		return false
	}
	return metadataBoolValue(metadata, "optimization_run") ||
		strings.TrimSpace(metadataString(metadata, "optimization_parent_run_id")) != ""
}

func runtimeSkillEvolutionFailureSignature(run *Run, events []RunEvent) string {
	if run == nil {
		return "runtime-failure"
	}
	for i := len(events) - 1; i >= 0; i-- {
		event := events[i]
		if strings.TrimSpace(event.Type) == "run_failed" && strings.TrimSpace(event.Message) != "" {
			return runtimeSkillEvolutionNormalizeSignature("failed:" + event.Message)
		}
	}
	if errText := strings.TrimSpace(run.Error); errText != "" {
		return runtimeSkillEvolutionNormalizeSignature("failed:" + errText)
	}
	if state := strings.TrimSpace(string(run.RuntimeState)); state != "" {
		return runtimeSkillEvolutionNormalizeSignature("failed:" + state)
	}
	return "runtime-failure"
}

func runtimeSkillEvolutionCaptureSignature(events []RunEvent) (string, []string) {
	records := runtimeReflectionRecordsFromEvents(events)
	for i := len(records) - 1; i >= 0; i-- {
		record := records[i]
		if strings.TrimSpace(record.Status) != "recorded" {
			continue
		}
		signature := strings.TrimSpace(record.ReflectionSignature)
		lessons := runtimeReflectionLessons([]runtimeReflectionRecord{record})
		if signature == "" || len(lessons) == 0 {
			continue
		}
		return signature, lessons
	}
	for i := len(events) - 1; i >= 0; i-- {
		event := events[i]
		if strings.TrimSpace(event.Type) != "task_reflection_completed" {
			continue
		}
		output := runtimeSkillEvolutionReflectionOutput(event)
		lessons := runtimeSkillEvolutionExtractLessons(output)
		if len(lessons) == 0 {
			continue
		}
		return runtimeSkillEvolutionNormalizeSignature(strings.Join(lessons, " | ")), lessons
	}
	return "", nil
}

func runtimeSkillEvolutionReflectionOutput(event RunEvent) string {
	if payload := strings.TrimSpace(event.PayloadJSON); payload != "" {
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(payload), &parsed); err == nil {
			if output := strings.TrimSpace(fmt.Sprint(parsed["output"])); output != "" && output != "<nil>" {
				return output
			}
		}
	}
	return strings.TrimSpace(event.Message)
}

func runtimeSkillEvolutionExtractLessons(output string) []string {
	output = strings.ReplaceAll(output, "\r\n", "\n")
	index := strings.Index(output, "Learned:")
	if index == -1 {
		return nil
	}
	lines := strings.Split(output[index+len("Learned:"):], "\n")
	lessons := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "- ") {
			continue
		}
		lesson := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
		if lesson == "" {
			continue
		}
		lessons = append(lessons, lesson)
	}
	return lessons
}

func runtimeSkillEvolutionNormalizeSignature(raw string) string {
	parts := strings.Fields(strings.ToLower(strings.TrimSpace(raw)))
	if len(parts) == 0 {
		return ""
	}
	signature := strings.Join(parts, "-")
	if len(signature) > 160 {
		signature = signature[:160]
	}
	return signature
}

func runtimeSkillEvolutionTrimmedValue(raw string, maxLen int) string {
	value := strings.TrimSpace(raw)
	if maxLen <= 0 || len(value) <= maxLen {
		return value
	}
	return strings.TrimSpace(value[:maxLen])
}

func runtimeSkillEvolutionEventSummaries(events []RunEvent, limit int) []map[string]interface{} {
	if len(events) == 0 || limit <= 0 {
		return nil
	}
	if limit > len(events) {
		limit = len(events)
	}
	summaries := make([]map[string]interface{}, 0, limit)
	start := len(events) - limit
	if start < 0 {
		start = 0
	}
	for _, event := range events[start:] {
		summary := map[string]interface{}{
			"type":       strings.TrimSpace(event.Type),
			"message":    runtimeSkillEvolutionTrimmedValue(event.Message, 240),
			"step_index": event.StepIndex,
		}
		if toolName := strings.TrimSpace(event.ToolName); toolName != "" {
			summary["tool_name"] = toolName
		}
		summaries = append(summaries, summary)
	}
	return summaries
}

func runtimeSkillEvolutionMetrics(run *Run, events []RunEvent) map[string]interface{} {
	metrics := map[string]interface{}{}
	if durationMs := runtimeSkillEvolutionRunDurationMs(run); durationMs >= 0 {
		metrics["duration_ms"] = durationMs
	}
	if len(events) > 0 {
		metrics["event_count"] = len(events)
	}
	toolEventCount := 0
	stepIndexes := make(map[int]struct{})
	for _, event := range events {
		if strings.TrimSpace(event.ToolName) != "" || strings.HasPrefix(strings.TrimSpace(event.Type), "tool_") {
			toolEventCount++
		}
		if event.StepIndex > 0 {
			stepIndexes[event.StepIndex] = struct{}{}
		}
	}
	if toolEventCount > 0 {
		metrics["tool_event_count"] = toolEventCount
	}
	if len(stepIndexes) > 0 {
		metrics["step_count"] = len(stepIndexes)
	}
	return metrics
}

func runtimeSkillEvolutionRunDurationMs(run *Run) int64 {
	if run == nil {
		return -1
	}
	if run.StartedAt != nil && run.FinishedAt != nil && !run.FinishedAt.Before(*run.StartedAt) {
		return run.FinishedAt.Sub(*run.StartedAt).Milliseconds()
	}
	if run.StartedAt != nil && !run.UpdatedAt.Before(*run.StartedAt) {
		return run.UpdatedAt.Sub(*run.StartedAt).Milliseconds()
	}
	if !run.UpdatedAt.Before(run.CreatedAt) {
		return run.UpdatedAt.Sub(run.CreatedAt).Milliseconds()
	}
	return -1
}

func runtimeSkillEvolutionUsage(run *Run, events []RunEvent) map[string]interface{} {
	sources := runtimeSkillEvolutionMetadataSources(run, events)
	usage := map[string]interface{}{}
	if value, ok := runtimeSkillEvolutionFindNumber(sources, "input_tokens", "prompt_tokens"); ok {
		usage["input_tokens"] = value
	}
	if value, ok := runtimeSkillEvolutionFindNumber(sources, "output_tokens", "completion_tokens"); ok {
		usage["output_tokens"] = value
	}
	if value, ok := runtimeSkillEvolutionFindNumber(sources, "total_tokens"); ok {
		usage["total_tokens"] = value
	}
	if value, ok := runtimeSkillEvolutionFindNumber(sources, "cache_read_tokens"); ok {
		usage["cache_read_tokens"] = value
	}
	if value, ok := runtimeSkillEvolutionFindNumber(sources, "cache_write_tokens"); ok {
		usage["cache_write_tokens"] = value
	}
	if value, ok := runtimeSkillEvolutionFindNumber(sources, "estimated_cost"); ok {
		usage["estimated_cost"] = value
	}
	return usage
}

func runtimeSkillEvolutionQuality(run *Run, events []RunEvent) map[string]interface{} {
	sources := runtimeSkillEvolutionMetadataSources(run, events)
	quality := map[string]interface{}{}
	if value, ok := runtimeSkillEvolutionFindBool(sources, "verification_passed"); ok {
		quality["verification_passed"] = value
	}
	if value := runtimeSkillEvolutionFindString(sources, "failure_label"); value != "" {
		quality["failure_label"] = value
	}
	if value, ok := runtimeSkillEvolutionFindNumber(sources, "outcome_score"); ok {
		quality["outcome_score"] = value
	}
	if value, ok := runtimeSkillEvolutionFindNumber(sources, "evidence_score"); ok {
		quality["evidence_score"] = value
	}
	if value, ok := runtimeSkillEvolutionFindNumber(sources, "execution_score"); ok {
		quality["execution_score"] = value
	}
	return quality
}

func runtimeSkillEvolutionValidation(run *Run, events []RunEvent) map[string]interface{} {
	sources := runtimeSkillEvolutionMetadataSources(run, events)
	validation := map[string]interface{}{}
	if value, ok := runtimeSkillEvolutionFindBool(sources, "recovered"); ok {
		validation["recovered"] = value
	}
	if value, ok := runtimeSkillEvolutionFindNumber(sources, "failure_count"); ok {
		validation["failure_count"] = value
	}
	return validation
}

func runtimeSkillEvolutionMetadataSources(run *Run, events []RunEvent) []map[string]interface{} {
	sources := make([]map[string]interface{}, 0, len(events)+1)
	for i := len(events) - 1; i >= 0; i-- {
		runtimeSkillEvolutionCollectMaps(decodeJSONMap(events[i].PayloadJSON), &sources)
	}
	runtimeSkillEvolutionCollectMaps(runMetadataMap(run), &sources)
	return sources
}

func runMetadataMap(run *Run) map[string]interface{} {
	if run == nil {
		return nil
	}
	return run.Metadata
}

func runtimeSkillEvolutionCollectMaps(raw map[string]interface{}, out *[]map[string]interface{}) {
	if len(raw) == 0 || out == nil {
		return
	}
	*out = append(*out, raw)
	for _, value := range raw {
		runtimeSkillEvolutionCollectNestedMaps(value, out)
	}
}

func runtimeSkillEvolutionCollectNestedMaps(raw interface{}, out *[]map[string]interface{}) {
	switch typed := raw.(type) {
	case map[string]interface{}:
		runtimeSkillEvolutionCollectMaps(typed, out)
	case []interface{}:
		for _, item := range typed {
			runtimeSkillEvolutionCollectNestedMaps(item, out)
		}
	}
}

func runtimeSkillEvolutionFindNumber(sources []map[string]interface{}, keys ...string) (float64, bool) {
	for _, source := range sources {
		for _, key := range keys {
			if value, ok := floatMetadataWithPresence(source[key]); ok {
				return value, true
			}
		}
	}
	return 0, false
}

func runtimeSkillEvolutionFindBool(sources []map[string]interface{}, keys ...string) (bool, bool) {
	for _, source := range sources {
		for _, key := range keys {
			if value, ok := source[key]; ok {
				switch typed := value.(type) {
				case bool:
					return typed, true
				case string:
					if strings.TrimSpace(typed) == "" {
						continue
					}
					return strings.EqualFold(strings.TrimSpace(typed), "true"), true
				}
			}
		}
	}
	return false, false
}

func runtimeSkillEvolutionFindString(sources []map[string]interface{}, keys ...string) string {
	for _, source := range sources {
		for _, key := range keys {
			if value := metadataString(source, key); value != "" {
				return value
			}
		}
	}
	return ""
}
