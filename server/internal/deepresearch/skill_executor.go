package deepresearch

import (
	"context"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

// SkillExecutor adapts deep research service to builtin skill executor interface.
type SkillExecutor struct {
	service *Service
}

type deepResearchCardState struct {
	evidenceCount int
}

const (
	deepResearchFormatJSON = "json"
	deepResearchFormatXML  = "xml"
)

func NewSkillExecutor(service *Service) *SkillExecutor {
	return &SkillExecutor{service: service}
}

func (e *SkillExecutor) SetV2Enabled(enabled bool) {
	if e == nil || e.service == nil {
		return
	}
	e.service.SetV2Enabled(enabled)
}

// Execute runs deep research synchronously for skill invocation and returns a map result.
func (e *SkillExecutor) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	query, _ := args["query"].(string)
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	format, err := parseOutputFormat(args["format"])
	if err != nil {
		return nil, err
	}

	mode := ModeStandard
	if v, ok := args["mode"].(string); ok {
		switch Mode(strings.TrimSpace(v)) {
		case ModeFast, ModeStandard, ModeDeep:
			mode = Mode(strings.TrimSpace(v))
		}
	}
	routeMode := RouteModeAuto
	if v, ok := args["route_mode"].(string); ok {
		routeMode = RouteMode(strings.TrimSpace(v))
	}

	lang, _ := args["lang"].(string)
	strictEntity := parseBoolArg(args["strict_entity"])
	timeWindows := parseStringSliceArg(args["time_windows"])
	reportStyle, _ := args["report_style"].(string)
	var budget *Budget
	maxSources := parseIntArg(args["max_sources"])
	maxSeconds := parseIntArg(args["max_seconds"])
	if maxSources > 0 || maxSeconds > 0 {
		budget = &Budget{
			MaxSources: maxSources,
			MaxSeconds: maxSeconds,
		}
	}
	userID := strings.TrimSpace(tools.GetUserID(ctx))
	job, err := e.service.CreateJob(ctx, CreateJobRequest{
		Query:        query,
		Mode:         mode,
		RouteMode:    routeMode,
		Lang:         strings.TrimSpace(lang),
		Budget:       budget,
		UserID:       userID,
		StrictEntity: strictEntity,
		TimeWindows:  timeWindows,
		ReportStyle:  strings.TrimSpace(reportStyle),
	})
	if err != nil {
		return nil, err
	}

	lastStage := ""
	lastProgress := -1
	lastStatus := JobStatus("")
	cardState := deepResearchCardState{}

	fetchCurrent := func() (*Job, error) {
		if userID != "" {
			return e.service.GetJobForUser(job.ID, userID, "")
		}
		return e.service.GetJob(job.ID)
	}

	cancelJob := func() {
		if userID != "" {
			_ = e.service.CancelJobForUser(job.ID, userID, "")
			return
		}
		_ = e.service.CancelJob(job.ID)
	}

	handleCurrent := func(current *Job) (interface{}, error) {
		if current == nil {
			return nil, fmt.Errorf("deep research job state unavailable")
		}
		if current.Stage != lastStage || current.Progress != lastProgress || current.Status != lastStatus {
			tools.EmitCard(ctx, map[string]interface{}{
				"type":          "deep-research-progress",
				"step":          current.Stage,
				"name":          deepResearchStageLabel(current.Stage, current.Lang),
				"job_id":        current.ID,
				"query":         current.Query,
				"mode":          string(current.Mode),
				"stage":         current.Stage,
				"status":        string(current.Status),
				"progress":      current.Progress,
				"iteration":     current.Iteration,
				"latest_gap":    current.LatestGap,
				"latest_action": current.LatestAction,
				"_streaming":    current.Status != JobStatusCompleted && current.Status != JobStatusFailed && current.Status != JobStatusCancelled,
			})
			lastStage = current.Stage
			lastProgress = current.Progress
			lastStatus = current.Status
		}
		switch current.Status {
		case JobStatusCompleted:
			if format == deepResearchFormatXML {
				return marshalDeepResearchXML(current)
			}
			return buildDeepResearchResult(current), nil
		case JobStatusFailed:
			if current.Error == "" {
				return nil, fmt.Errorf("deep research failed")
			}
			return nil, fmt.Errorf("deep research failed: %s", current.Error)
		case JobStatusCancelled:
			return nil, fmt.Errorf("deep research cancelled")
		default:
			return nil, nil
		}
	}

	handleEvent := func(ev Event) {
		switch ev.Type {
		case "task_planned":
			payload := deepResearchEventPayloadMap(ev.Payload)
			count := parseIntArg(payload["count"])
			tools.EmitCard(ctx, deepResearchInfoCard(job, "info", deepResearchLocalized(job.Lang,
				fmt.Sprintf("Planned %d research task(s)", count),
				fmt.Sprintf("已规划 %d 个研究任务", count),
			), []map[string]interface{}{
				deepResearchDetail("tasks", count),
				deepResearchDetail("mode", string(job.Mode)),
			}))
		case "evidence_added":
			cardState.evidenceCount++
			if !shouldEmitDeepResearchEvidenceCard(cardState.evidenceCount) {
				return
			}
			title, latestSource := deepResearchEvidenceSummary(ev.Payload)
			details := []map[string]interface{}{
				deepResearchDetail("evidence", cardState.evidenceCount),
			}
			if latestSource != "" {
				details = append(details, deepResearchDetail("latest_source", latestSource))
			}
			if title != "" {
				details = append(details, deepResearchDetail("title", title))
			}
			tools.EmitCard(ctx, deepResearchInfoCard(job, "info", deepResearchLocalized(job.Lang,
				fmt.Sprintf("Collected %d source(s)", cardState.evidenceCount),
				fmt.Sprintf("已收集 %d 条来源", cardState.evidenceCount),
			), details))
		case "search_retry":
			payload := deepResearchEventPayloadMap(ev.Payload)
			attempt := parseIntArg(payload["attempt"])
			delayMS := parseIntArg(payload["delayMs"])
			query := strings.TrimSpace(deepResearchString(payload["query"]))
			details := []map[string]interface{}{
				deepResearchDetail("attempt", attempt),
			}
			if query != "" {
				details = append(details, deepResearchDetail("query", query))
			}
			if delayMS > 0 {
				details = append(details, deepResearchDetail("delay_ms", delayMS))
			}
			tools.EmitCard(ctx, deepResearchInfoCard(job, "warning", deepResearchLocalized(job.Lang,
				"Retrying a search source",
				"正在重试搜索来源",
			), details))
		case "stage_warning":
			payload := deepResearchEventPayloadMap(ev.Payload)
			stage := strings.TrimSpace(deepResearchString(payload["stage"]))
			message := strings.TrimSpace(deepResearchString(payload["message"]))
			if message == "" {
				message = strings.TrimSpace(deepResearchString(payload["error"]))
			}
			if message == "" {
				message = deepResearchLocalized(job.Lang, "A research stage reported a warning", "研究阶段发出了警告")
			}
			details := make([]map[string]interface{}, 0, 3)
			if stage != "" {
				details = append(details, deepResearchDetail("stage", stage))
			}
			if query := strings.TrimSpace(deepResearchString(payload["query"])); query != "" {
				details = append(details, deepResearchDetail("query", query))
			}
			if coverage := deepResearchFloat(payload["citation_coverage"]); coverage > 0 {
				details = append(details, deepResearchDetail("citation_coverage", fmt.Sprintf("%.0f%%", coverage*100)))
			}
			tools.EmitCard(ctx, deepResearchInfoCard(job, "warning", message, details))
		case "citation_coverage_updated":
			payload := deepResearchEventPayloadMap(ev.Payload)
			coverage := deepResearchFloat(payload["citation_coverage"])
			evidenceCount := parseIntArg(payload["evidence_count"])
			details := make([]map[string]interface{}, 0, 2)
			if evidenceCount > 0 {
				details = append(details, deepResearchDetail("evidence", evidenceCount))
			}
			if coverage > 0 {
				details = append(details, deepResearchDetail("citation_coverage", fmt.Sprintf("%.0f%%", coverage*100)))
			}
			tools.EmitCard(ctx, deepResearchInfoCard(job, "info", deepResearchLocalized(job.Lang,
				"Draft synthesis ready",
				"研究草稿已生成",
			), details))
		case "gap_detected":
			payload := deepResearchEventPayloadMap(ev.Payload)
			details := make([]map[string]interface{}, 0, 4)
			if iteration := parseIntArg(payload["iteration"]); iteration > 0 {
				details = append(details, deepResearchDetail("iteration", iteration))
			}
			if focus := strings.TrimSpace(deepResearchString(payload["focus"])); focus != "" {
				details = append(details, deepResearchDetail("focus", focus))
			}
			if gap := strings.TrimSpace(deepResearchString(payload["gap"])); gap != "" {
				details = append(details, deepResearchDetail("gap", gap))
			}
			tools.EmitCard(ctx, deepResearchInfoCard(job, "warning", deepResearchLocalized(job.Lang,
				"Detected a research gap",
				"发现研究缺口",
			), details))
		case "followup_planned":
			payload := deepResearchEventPayloadMap(ev.Payload)
			details := make([]map[string]interface{}, 0, 4)
			if iteration := parseIntArg(payload["iteration"]); iteration > 0 {
				details = append(details, deepResearchDetail("iteration", iteration))
			}
			if gap := strings.TrimSpace(deepResearchString(payload["gap"])); gap != "" {
				details = append(details, deepResearchDetail("gap", gap))
			}
			if followUp := strings.TrimSpace(deepResearchString(payload["follow_up_query"])); followUp != "" {
				details = append(details, deepResearchDetail("follow_up_query", followUp))
			}
			tools.EmitCard(ctx, deepResearchInfoCard(job, "info", deepResearchLocalized(job.Lang,
				"Planned a follow-up research pass",
				"已规划下一轮深挖",
			), details))
		case "verification_completed":
			payload := deepResearchEventPayloadMap(ev.Payload)
			details := make([]map[string]interface{}, 0, 5)
			for _, key := range []string{"iteration", "resolved_count", "conflicted_count", "insufficient_count"} {
				if value := parseIntArg(payload[key]); value > 0 || key == "iteration" {
					details = append(details, deepResearchDetail(key, value))
				}
			}
			if latestGap := strings.TrimSpace(deepResearchString(payload["latest_gap"])); latestGap != "" {
				details = append(details, deepResearchDetail("latest_gap", latestGap))
			}
			tools.EmitCard(ctx, deepResearchInfoCard(job, "info", deepResearchLocalized(job.Lang,
				"Verification pass completed",
				"核验轮次已完成",
			), details))
		case "loop_stopped":
			payload := deepResearchEventPayloadMap(ev.Payload)
			details := make([]map[string]interface{}, 0, 4)
			if iteration := parseIntArg(payload["iteration"]); iteration > 0 {
				details = append(details, deepResearchDetail("iteration", iteration))
			}
			if reason := strings.TrimSpace(deepResearchString(payload["stop_reason"])); reason != "" {
				details = append(details, deepResearchDetail("stop_reason", reason))
			}
			if latestGap := strings.TrimSpace(deepResearchString(payload["latest_gap"])); latestGap != "" {
				details = append(details, deepResearchDetail("latest_gap", latestGap))
			}
			tools.EmitCard(ctx, deepResearchInfoCard(job, "info", deepResearchLocalized(job.Lang,
				"Deep research loop stopped",
				"深挖循环已停止",
			), details))
		}
	}

	events, unsubscribe, subErr := e.service.SubscribeForUser(job.ID, userID, "")
	if subErr == nil {
		defer unsubscribe()
	}
	sanityTicker := time.NewTicker(2 * time.Second)
	defer sanityTicker.Stop()
	pollTicker := time.NewTicker(800 * time.Millisecond)
	defer pollTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			cancelJob()
			return nil, ctx.Err()
		case <-sanityTicker.C:
			current, err := fetchCurrent()
			if err != nil {
				return nil, err
			}
			if data, doneErr := handleCurrent(current); data != nil || doneErr != nil {
				return data, doneErr
			}
		case <-pollTicker.C:
			// Fallback if subscription is unavailable.
			if subErr == nil {
				continue
			}
			current, err := fetchCurrent()
			if err != nil {
				return nil, err
			}
			if data, doneErr := handleCurrent(current); data != nil || doneErr != nil {
				return data, doneErr
			}
		case ev, ok := <-events:
			if subErr != nil {
				continue
			}
			if !ok {
				current, err := fetchCurrent()
				if err != nil {
					return nil, err
				}
				if data, doneErr := handleCurrent(current); data != nil || doneErr != nil {
					return data, doneErr
				}
				return nil, fmt.Errorf("deep research event stream closed unexpectedly")
			}
			handleEvent(ev)
			current, err := fetchCurrent()
			if err != nil {
				return nil, err
			}
			if data, doneErr := handleCurrent(current); data != nil || doneErr != nil {
				return data, doneErr
			}
		}
	}
}

func deepResearchLocalized(lang, en, zh string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(lang)), "zh") {
		return zh
	}
	return en
}

func deepResearchStageLabel(stage, lang string) string {
	switch strings.TrimSpace(stage) {
	case "planning":
		return deepResearchLocalized(lang, "Planning", "规划")
	case "retrieve":
		return deepResearchLocalized(lang, "Retrieving", "检索中")
	case "verify":
		return deepResearchLocalized(lang, "Verifying", "核验中")
	case "synthesize":
		return deepResearchLocalized(lang, "Synthesizing", "综合中")
	case "completed":
		return deepResearchLocalized(lang, "Completed", "已完成")
	case "failed":
		return deepResearchLocalized(lang, "Failed", "失败")
	case "cancelled":
		return deepResearchLocalized(lang, "Cancelled", "已取消")
	default:
		return stage
	}
}

func deepResearchDetail(label string, value interface{}) map[string]interface{} {
	return map[string]interface{}{"label": label, "value": value}
}

func deepResearchInfoCard(job *Job, status, message string, details []map[string]interface{}) map[string]interface{} {
	card := map[string]interface{}{
		"type":    "result",
		"title":   "deep_research",
		"status":  status,
		"message": message,
	}
	if job != nil {
		card["job_id"] = job.ID
	}
	if len(details) > 0 {
		card["details"] = details
	}
	return card
}

func deepResearchEventPayloadMap(payload interface{}) map[string]interface{} {
	if payload == nil {
		return nil
	}
	if m, ok := payload.(map[string]interface{}); ok {
		return m
	}
	return nil
}

func deepResearchString(value interface{}) string {
	s, _ := value.(string)
	return s
}

func deepResearchFloat(value interface{}) float64 {
	switch tv := value.(type) {
	case float64:
		return tv
	case float32:
		return float64(tv)
	case int:
		return float64(tv)
	case int64:
		return float64(tv)
	default:
		return 0
	}
}

func deepResearchEvidenceSummary(payload interface{}) (title, latestSource string) {
	switch ev := payload.(type) {
	case Evidence:
		title = strings.TrimSpace(ev.Title)
		latestSource = strings.TrimSpace(ev.Domain)
		if latestSource == "" {
			latestSource = strings.TrimSpace(ev.URL)
		}
	case *Evidence:
		if ev != nil {
			title = strings.TrimSpace(ev.Title)
			latestSource = strings.TrimSpace(ev.Domain)
			if latestSource == "" {
				latestSource = strings.TrimSpace(ev.URL)
			}
		}
	case map[string]interface{}:
		title = strings.TrimSpace(deepResearchString(ev["title"]))
		latestSource = strings.TrimSpace(deepResearchString(ev["domain"]))
		if latestSource == "" {
			latestSource = strings.TrimSpace(deepResearchString(ev["url"]))
		}
	}
	return title, latestSource
}

func shouldEmitDeepResearchEvidenceCard(count int) bool {
	if count <= 0 {
		return false
	}
	return count <= 3 || count%5 == 0
}

func buildDeepResearchResult(current *Job) map[string]interface{} {
	data := map[string]interface{}{
		"job_id":               current.ID,
		"status":               string(current.Status),
		"mode":                 string(current.Mode),
		"requested_route_mode": string(current.RequestedRouteMode),
		"effective_route_mode": string(current.EffectiveRouteMode),
		"route_reason":         current.RouteReason,
		"query":                current.Query,
		"progress":             current.Progress,
		"iteration":            current.Iteration,
		"latest_gap":           current.LatestGap,
		"latest_action":        current.LatestAction,
		"evidence_count":       len(current.Evidence),
		"strict_entity":        current.StrictEntity,
		"time_windows":         current.TimeWindows,
		"report_style":         current.ReportStyle,
	}
	if current.Report != nil {
		data["answer"] = current.Report.Answer
		data["confidence"] = current.Report.Confidence
		data["citations"] = current.Report.Citations
		data["open_questions"] = current.Report.OpenQuestions
		data["support_count"] = current.Report.SupportCount
		data["conflict_count"] = current.Report.ConflictCount
		data["has_conflict"] = current.Report.HasConflict
		data["iterations"] = current.Report.Iterations
		data["stop_reason"] = current.Report.StopReason
		data["citation_coverage"] = current.Report.CitationCoverage
		data["entity_disambiguation"] = current.Report.EntityDisambiguation
		data["stage_errors"] = current.Report.StageErrors
		data["timeline_sections"] = current.Report.TimelineSections
		data["research_trace"] = current.Report.ResearchTrace
		data["verification_summary"] = current.Report.VerificationSummary
		data["experiment"] = current.Report.Experiment
	}
	return data
}

type deepResearchXMLResult struct {
	XMLName             xml.Name                            `xml:"deep_research"`
	JobID               string                              `xml:"job_id,attr,omitempty"`
	Status              string                              `xml:"status,attr,omitempty"`
	Mode                string                              `xml:"mode,attr,omitempty"`
	Query               string                              `xml:"query"`
	Progress            int                                 `xml:"progress"`
	Iteration           int                                 `xml:"iteration,omitempty"`
	LatestGap           string                              `xml:"latest_gap,omitempty"`
	LatestAction        string                              `xml:"latest_action,omitempty"`
	EvidenceCount       int                                 `xml:"evidence_count"`
	Answer              string                              `xml:"answer,omitempty"`
	Confidence          *float64                            `xml:"confidence,omitempty"`
	Citations           []deepResearchXMLCitation           `xml:"citations>citation,omitempty"`
	OpenQuestions       []string                            `xml:"open_questions>question,omitempty"`
	SupportCount        *int                                `xml:"support_count,omitempty"`
	ConflictCount       *int                                `xml:"conflict_count,omitempty"`
	HasConflict         *bool                               `xml:"has_conflict,omitempty"`
	Iterations          *int                                `xml:"iterations,omitempty"`
	StopReason          string                              `xml:"stop_reason,omitempty"`
	CitationCoverage    *float64                            `xml:"citation_coverage,omitempty"`
	ResearchTrace       []deepResearchXMLTrace              `xml:"research_trace>entry,omitempty"`
	VerificationSummary *deepResearchXMLVerificationSummary `xml:"verification_summary,omitempty"`
}

type deepResearchXMLCitation struct {
	EvidenceID string `xml:"evidence_id,attr,omitempty"`
	Title      string `xml:"title"`
	URL        string `xml:"url"`
}

type deepResearchXMLTrace struct {
	Iteration           int    `xml:"iteration,attr,omitempty"`
	Focus               string `xml:"focus,omitempty"`
	Gap                 string `xml:"gap,omitempty"`
	FollowUpQuery       string `xml:"follow_up_query,omitempty"`
	EvidenceAdded       int    `xml:"evidence_added,omitempty"`
	VerificationOutcome string `xml:"verification_outcome,omitempty"`
}

type deepResearchXMLVerificationSummary struct {
	ResolvedCount     int                               `xml:"resolved_count,omitempty"`
	ConflictedCount   int                               `xml:"conflicted_count,omitempty"`
	InsufficientCount int                               `xml:"insufficient_count,omitempty"`
	Items             []deepResearchXMLVerificationItem `xml:"items>item,omitempty"`
}

type deepResearchXMLVerificationItem struct {
	Focus       string   `xml:"focus,omitempty"`
	Gap         string   `xml:"gap,omitempty"`
	Status      string   `xml:"status,omitempty"`
	Summary     string   `xml:"summary,omitempty"`
	EvidenceIDs []string `xml:"evidence_ids>evidence_id,omitempty"`
}

func marshalDeepResearchXML(current *Job) (string, error) {
	payload := deepResearchXMLResult{
		JobID:         current.ID,
		Status:        string(current.Status),
		Mode:          string(current.Mode),
		Query:         current.Query,
		Progress:      current.Progress,
		Iteration:     current.Iteration,
		LatestGap:     current.LatestGap,
		LatestAction:  current.LatestAction,
		EvidenceCount: len(current.Evidence),
	}

	if current.Report != nil {
		payload.Answer = current.Report.Answer
		payload.Confidence = &current.Report.Confidence
		payload.OpenQuestions = append(payload.OpenQuestions, current.Report.OpenQuestions...)
		payload.SupportCount = &current.Report.SupportCount
		payload.ConflictCount = &current.Report.ConflictCount
		payload.HasConflict = &current.Report.HasConflict
		payload.Iterations = &current.Report.Iterations
		payload.StopReason = current.Report.StopReason
		payload.CitationCoverage = &current.Report.CitationCoverage
		payload.Citations = make([]deepResearchXMLCitation, len(current.Report.Citations))
		for i, c := range current.Report.Citations {
			payload.Citations[i] = deepResearchXMLCitation{
				EvidenceID: c.EvidenceID,
				Title:      c.Title,
				URL:        c.URL,
			}
		}
		payload.ResearchTrace = make([]deepResearchXMLTrace, len(current.Report.ResearchTrace))
		for i, entry := range current.Report.ResearchTrace {
			payload.ResearchTrace[i] = deepResearchXMLTrace{
				Iteration:           entry.Iteration,
				Focus:               entry.Focus,
				Gap:                 entry.Gap,
				FollowUpQuery:       entry.FollowUpQuery,
				EvidenceAdded:       entry.EvidenceAdded,
				VerificationOutcome: entry.VerificationOutcome,
			}
		}
		if current.Report.VerificationSummary != nil {
			payload.VerificationSummary = &deepResearchXMLVerificationSummary{
				ResolvedCount:     current.Report.VerificationSummary.ResolvedCount,
				ConflictedCount:   current.Report.VerificationSummary.ConflictedCount,
				InsufficientCount: current.Report.VerificationSummary.InsufficientCount,
				Items:             make([]deepResearchXMLVerificationItem, len(current.Report.VerificationSummary.Items)),
			}
			for i, item := range current.Report.VerificationSummary.Items {
				payload.VerificationSummary.Items[i] = deepResearchXMLVerificationItem{
					Focus:       item.Focus,
					Gap:         item.Gap,
					Status:      item.Status,
					Summary:     item.Summary,
					EvidenceIDs: append([]string(nil), item.EvidenceIDs...),
				}
			}
		}
	}

	encoded, err := xml.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to encode deep research response as XML: %w", err)
	}
	return string(encoded), nil
}

func parseIntArg(v interface{}) int {
	switch tv := v.(type) {
	case int:
		return tv
	case int32:
		return int(tv)
	case int64:
		return int(tv)
	case float64:
		return int(tv)
	case float32:
		return int(tv)
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(tv))
		if err == nil {
			return n
		}
	}
	return 0
}

func parseBoolArg(v interface{}) *bool {
	switch tv := v.(type) {
	case bool:
		return &tv
	case string:
		switch strings.ToLower(strings.TrimSpace(tv)) {
		case "true", "1", "yes":
			b := true
			return &b
		case "false", "0", "no":
			b := false
			return &b
		}
	}
	return nil
}

func parseStringSliceArg(v interface{}) []string {
	switch tv := v.(type) {
	case []string:
		out := make([]string, 0, len(tv))
		for _, item := range tv {
			item = strings.TrimSpace(item)
			if item != "" {
				out = append(out, item)
			}
		}
		return out
	case []interface{}:
		out := make([]string, 0, len(tv))
		for _, item := range tv {
			s, _ := item.(string)
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		s := strings.TrimSpace(tv)
		if s != "" {
			return []string{s}
		}
	}
	return nil
}

func parseOutputFormat(v interface{}) (string, error) {
	if v == nil {
		return deepResearchFormatJSON, nil
	}
	tv, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("format must be a string")
	}

	format, err := normalizeDeepResearchFormat(tv)
	if err != nil {
		return "", err
	}
	if format == "" {
		return deepResearchFormatJSON, nil
	}
	return format, nil
}

func normalizeDeepResearchFormat(raw string) (string, error) {
	format := strings.ToLower(strings.TrimSpace(raw))
	format = strings.Trim(format, ",.;:!?")
	if idx := strings.Index(format, ";"); idx >= 0 {
		format = strings.TrimSpace(format[:idx])
	}

	switch format {
	case "", deepResearchFormatJSON, deepResearchFormatXML:
		return format, nil
	case "md", "markdown", "text", "txt", "plain", "plaintext", "human", "jsonl", "application/json":
		return deepResearchFormatJSON, nil
	case "application/xml", "text/xml":
		return deepResearchFormatXML, nil
	default:
		return "", fmt.Errorf("format must be one of: json, xml")
	}
}
