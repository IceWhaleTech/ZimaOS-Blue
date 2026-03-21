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
	evidenceCount     int
	eventSeq          int
	latestParallelism int
	recentSources     []map[string]interface{}
}

const (
	deepResearchFormatJSON      = "json"
	deepResearchFormatXML       = "xml"
	deepResearchLiveSourceLimit = 12
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
	query := strings.TrimSpace(deepResearchCompatString(args, "query"))
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	format, err := parseOutputFormat(deepResearchCompatRaw(args, "format"))
	if err != nil {
		return nil, err
	}

	mode := ModeStandard
	if v := deepResearchCompatString(args, "mode"); v != "" {
		switch Mode(strings.TrimSpace(v)) {
		case ModeFast, ModeStandard, ModeDeep:
			mode = Mode(strings.TrimSpace(v))
		}
	}
	routeMode := RouteModeWeb
	if v := deepResearchCompatString(args, "route_mode", "routeMode"); v != "" {
		routeMode = RouteMode(strings.TrimSpace(v))
	}

	lang := deepResearchCompatString(args, "lang")
	strictEntity := parseBoolArg(deepResearchCompatRaw(args, "strict_entity", "strictEntity"))
	timeWindows := parseStringSliceArg(deepResearchCompatRaw(args, "time_windows", "timeWindows"))
	reportStyle := normalizeReportStyle(deepResearchCompatString(args, "report_style", "reportStyle"))
	var budget *Budget
	maxSources := parseIntArg(deepResearchCompatRaw(args, "max_sources", "maxSources"))
	maxSeconds := parseIntArg(deepResearchCompatRaw(args, "max_seconds", "maxSeconds"))
	if maxSources > 0 || maxSeconds > 0 {
		budget = &Budget{
			MaxSources: maxSources,
			MaxSeconds: maxSeconds,
		}
	}
	userID := strings.TrimSpace(tools.GetUserID(ctx))
	conversationID := strings.TrimSpace(tools.GetSessionID(ctx))
	job, err := e.service.CreateJob(ctx, CreateJobRequest{
		Query:          query,
		Mode:           mode,
		RouteMode:      routeMode,
		Lang:           strings.TrimSpace(lang),
		Budget:         budget,
		UserID:         userID,
		ConversationID: conversationID,
		StrictEntity:   strictEntity,
		TimeWindows:    timeWindows,
		ReportStyle:    strings.TrimSpace(reportStyle),
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
				"type":            "deep-research-progress",
				"id":              deepResearchProgressCardID(current.ID),
				"step":            current.Stage,
				"name":            deepResearchStageLabel(current.Stage, current.Lang),
				"job_id":          current.ID,
				"conversation_id": current.ConversationID,
				"query":           current.Query,
				"mode":            string(current.Mode),
				"stage":           current.Stage,
				"status":          string(current.Status),
				"progress":        current.Progress,
				"iteration":       current.Iteration,
				"latest_gap":      current.LatestGap,
				"latest_action":   current.LatestAction,
				"_streaming":      current.Status != JobStatusCompleted && current.Status != JobStatusFailed && current.Status != JobStatusCancelled,
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
		case "brief_augmented":
			payload := deepResearchEventPayloadMap(ev.Payload)
			card := deepResearchEventCard(job, &cardState, "brief", "info", deepResearchLocalized(job.Lang,
				"Research brief prepared",
				"研究摘要已准备",
			))
			brief := map[string]interface{}{}
			if goal := strings.TrimSpace(deepResearchString(payload["goal"])); goal != "" {
				brief["goal"] = goal
			}
			if entity := strings.TrimSpace(deepResearchString(payload["entity"])); entity != "" {
				brief["entity"] = entity
			}
			if windows := normalizeDeepResearchEventStringList(payload["time_windows"]); len(windows) > 0 {
				brief["time_windows"] = windows
			}
			if claims := normalizeDeepResearchEventStringList(payload["must_verify_claims"]); len(claims) > 0 {
				brief["must_verify_claims"] = claims
			}
			if len(brief) > 0 {
				card["brief"] = brief
			}
			tools.EmitCard(ctx, card)
		case "task_planned":
			payload := deepResearchEventPayloadMap(ev.Payload)
			count := parseIntArg(payload["count"])
			card := deepResearchEventCard(job, &cardState, "planning", "info", deepResearchLocalized(job.Lang,
				fmt.Sprintf("Planned %d research task(s)", count),
				fmt.Sprintf("已规划 %d 个研究任务", count),
			))
			if count > 0 {
				card["task_count"] = count
			}
			if tasks := deepResearchTaskCards(payload["tasks"]); len(tasks) > 0 {
				card["tasks"] = tasks
			}
			tools.EmitCard(ctx, card)
		case "evidence_added":
			payload := deepResearchEventPayloadMap(ev.Payload)
			if parallelism := parseIntArg(payload["parallelism"]); parallelism > 0 {
				cardState.latestParallelism = parallelism
			}
			cardState.recentSources = deepResearchMergeSourceCards(cardState.recentSources, deepResearchSourceCards(payload))
			cardState.evidenceCount++
			if !shouldEmitDeepResearchEvidenceCard(cardState.evidenceCount) {
				return
			}
			title, _ := deepResearchEvidenceSummary(ev.Payload)
			card := deepResearchEventCard(job, &cardState, "source", "info", deepResearchLocalized(job.Lang,
				fmt.Sprintf("Collected %d source(s)", cardState.evidenceCount),
				fmt.Sprintf("已收集 %d 条来源", cardState.evidenceCount),
			))
			card["evidence_count"] = cardState.evidenceCount
			if title != "" {
				card["source_title"] = title
			}
			if query := strings.TrimSpace(deepResearchString(payload["query"])); query != "" {
				card["search_query"] = query
			}
			if cardState.latestParallelism > 1 {
				card["parallelism"] = cardState.latestParallelism
			}
			if sources := deepResearchCloneSourceCards(cardState.recentSources); len(sources) > 0 {
				card["sources"] = sources
			}
			tools.EmitCard(ctx, card)
		case "search_retry":
			payload := deepResearchEventPayloadMap(ev.Payload)
			attempt := parseIntArg(payload["attempt"])
			delayMS := parseIntArg(payload["delayMs"])
			query := strings.TrimSpace(deepResearchString(payload["query"]))
			card := deepResearchEventCard(job, &cardState, "warning", "warning", deepResearchLocalized(job.Lang,
				"Retrying a search source",
				"正在重试搜索来源",
			))
			if attempt > 0 {
				card["attempt"] = attempt
			}
			if query != "" {
				card["search_query"] = query
			}
			if delayMS > 0 {
				card["delay_ms"] = delayMS
			}
			tools.EmitCard(ctx, card)
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
			card := deepResearchEventCard(job, &cardState, "warning", "warning", message)
			if stage != "" {
				card["stage"] = stage
			}
			if query := strings.TrimSpace(deepResearchString(payload["query"])); query != "" {
				card["search_query"] = query
			}
			if coverage := deepResearchFloat(payload["citation_coverage"]); coverage > 0 {
				card["citation_coverage"] = coverage
			}
			tools.EmitCard(ctx, card)
		case "citation_coverage_updated":
			payload := deepResearchEventPayloadMap(ev.Payload)
			coverage := deepResearchFloat(payload["citation_coverage"])
			evidenceCount := parseIntArg(payload["evidence_count"])
			card := deepResearchEventCard(job, &cardState, "synthesis", "info", deepResearchLocalized(job.Lang,
				"Draft synthesis ready",
				"研究草稿已生成",
			))
			if evidenceCount > 0 {
				card["evidence_count"] = evidenceCount
			}
			if coverage > 0 {
				card["citation_coverage"] = coverage
			}
			tools.EmitCard(ctx, card)
		case "gap_detected":
			payload := deepResearchEventPayloadMap(ev.Payload)
			card := deepResearchEventCard(job, &cardState, "gap", "warning", deepResearchLocalized(job.Lang,
				"Detected a research gap",
				"发现研究缺口",
			))
			if iteration := parseIntArg(payload["iteration"]); iteration > 0 {
				card["iteration"] = iteration
			}
			if focus := strings.TrimSpace(deepResearchString(payload["focus"])); focus != "" {
				card["focus"] = focus
			}
			if gap := strings.TrimSpace(deepResearchString(payload["gap"])); gap != "" {
				card["gap"] = gap
			}
			tools.EmitCard(ctx, card)
		case "followup_planned":
			payload := deepResearchEventPayloadMap(ev.Payload)
			card := deepResearchEventCard(job, &cardState, "followup", "info", deepResearchLocalized(job.Lang,
				"Planned a follow-up research pass",
				"已规划下一轮深挖",
			))
			if iteration := parseIntArg(payload["iteration"]); iteration > 0 {
				card["iteration"] = iteration
			}
			if gap := strings.TrimSpace(deepResearchString(payload["gap"])); gap != "" {
				card["gap"] = gap
			}
			if followUp := strings.TrimSpace(deepResearchString(payload["follow_up_query"])); followUp != "" {
				card["follow_up_query"] = followUp
			}
			tools.EmitCard(ctx, card)
		case "verification_completed":
			payload := deepResearchEventPayloadMap(ev.Payload)
			card := deepResearchEventCard(job, &cardState, "verification", "info", deepResearchLocalized(job.Lang,
				"Verification pass completed",
				"核验轮次已完成",
			))
			if iteration := parseIntArg(payload["iteration"]); iteration > 0 {
				card["iteration"] = iteration
			}
			card["verification"] = map[string]interface{}{
				"resolved_count":     parseIntArg(payload["resolved_count"]),
				"conflicted_count":   parseIntArg(payload["conflicted_count"]),
				"insufficient_count": parseIntArg(payload["insufficient_count"]),
			}
			if latestGap := strings.TrimSpace(deepResearchString(payload["latest_gap"])); latestGap != "" {
				card["gap"] = latestGap
			}
			tools.EmitCard(ctx, card)
		case "loop_stopped":
			payload := deepResearchEventPayloadMap(ev.Payload)
			card := deepResearchEventCard(job, &cardState, "loop-stopped", "info", deepResearchLocalized(job.Lang,
				"Deep research loop stopped",
				"深挖循环已停止",
			))
			if iteration := parseIntArg(payload["iteration"]); iteration > 0 {
				card["iteration"] = iteration
			}
			if reason := strings.TrimSpace(deepResearchString(payload["stop_reason"])); reason != "" {
				card["stop_reason"] = reason
			}
			if latestGap := strings.TrimSpace(deepResearchString(payload["latest_gap"])); latestGap != "" {
				card["gap"] = latestGap
			}
			tools.EmitCard(ctx, card)
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

func deepResearchProgressCardID(jobID string) string {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return ""
	}
	return "deep-research-progress-" + jobID
}

func deepResearchEventCard(job *Job, state *deepResearchCardState, eventKind, status, summary string) map[string]interface{} {
	if state != nil {
		state.eventSeq++
	}
	card := map[string]interface{}{
		"type":       "deep-research-event",
		"event_kind": strings.TrimSpace(eventKind),
		"status":     strings.TrimSpace(status),
		"summary":    strings.TrimSpace(summary),
	}
	if job != nil {
		card["job_id"] = job.ID
		card["conversation_id"] = job.ConversationID
		card["query"] = job.Query
		card["mode"] = string(job.Mode)
		if state != nil && state.eventSeq > 0 {
			card["id"] = fmt.Sprintf("deep-research-event-%s-%02d", strings.TrimSpace(job.ID), state.eventSeq)
		}
	}
	return card
}

func deepResearchTaskCards(value interface{}) []map[string]interface{} {
	out := make([]map[string]interface{}, 0)
	appendTask := func(m map[string]interface{}) {
		question := strings.TrimSpace(deepResearchString(m["question"]))
		if question == "" {
			return
		}
		entry := map[string]interface{}{"question": question}
		if axis := strings.TrimSpace(deepResearchString(m["axis"])); axis != "" {
			entry["axis"] = axis
		}
		if category := strings.TrimSpace(deepResearchString(m["category"])); category != "" {
			entry["category"] = category
		}
		if timeWindow := strings.TrimSpace(deepResearchString(m["time_window"])); timeWindow != "" {
			entry["time_window"] = timeWindow
		}
		out = append(out, entry)
	}
	switch items := value.(type) {
	case []map[string]interface{}:
		for _, item := range items {
			appendTask(item)
		}
	case []interface{}:
		for _, item := range items {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			appendTask(m)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeDeepResearchEventStringList(value interface{}) []string {
	out := make([]string, 0)
	appendText := func(item interface{}) {
		text := strings.TrimSpace(deepResearchString(item))
		if text != "" {
			out = append(out, text)
		}
	}
	switch items := value.(type) {
	case []string:
		for _, item := range items {
			appendText(item)
		}
	case []interface{}:
		for _, item := range items {
			appendText(item)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func deepResearchSourceCards(payload interface{}) []map[string]interface{} {
	m, ok := payload.(map[string]interface{})
	if !ok {
		return nil
	}
	defaultQuery := strings.TrimSpace(deepResearchString(m["query"]))
	out := make([]map[string]interface{}, 0)
	seen := map[string]struct{}{}
	appendSource := func(raw map[string]interface{}, fallbackQuery string) {
		source := deepResearchNormalizeSourceCard(raw, fallbackQuery)
		if source == nil {
			return
		}
		key := deepResearchSourceIdentity(source)
		if key == "" {
			return
		}
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		out = append(out, source)
	}
	switch sources := m["sources"].(type) {
	case []map[string]interface{}:
		for _, source := range sources {
			appendSource(source, defaultQuery)
		}
	case []interface{}:
		for _, item := range sources {
			source, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			appendSource(source, defaultQuery)
		}
	}
	appendSource(m, defaultQuery)
	if len(out) == 0 {
		return nil
	}
	return out
}

func deepResearchNormalizeSourceCard(raw map[string]interface{}, fallbackQuery string) map[string]interface{} {
	if raw == nil {
		return nil
	}
	title := strings.TrimSpace(deepResearchString(raw["title"]))
	domain := strings.TrimSpace(deepResearchString(raw["domain"]))
	url := strings.TrimSpace(deepResearchString(raw["url"]))
	query := strings.TrimSpace(deepResearchString(raw["query"]))
	if query == "" {
		query = strings.TrimSpace(fallbackQuery)
	}
	if title == "" && domain == "" && url == "" {
		return nil
	}
	source := map[string]interface{}{}
	if title != "" {
		source["title"] = title
	}
	if domain != "" {
		source["domain"] = domain
	}
	if url != "" {
		source["url"] = url
	}
	if query != "" {
		source["query"] = query
	}
	return source
}

func deepResearchSourceIdentity(source map[string]interface{}) string {
	url := strings.ToLower(strings.TrimSpace(deepResearchString(source["url"])))
	if url != "" {
		return "url:" + url
	}
	title := strings.ToLower(strings.TrimSpace(deepResearchString(source["title"])))
	domain := strings.ToLower(strings.TrimSpace(deepResearchString(source["domain"])))
	query := strings.ToLower(strings.TrimSpace(deepResearchString(source["query"])))
	key := strings.TrimSpace(title + "|" + domain + "|" + query)
	if key == "" || key == "||" {
		return ""
	}
	return "meta:" + key
}

func deepResearchMergeSourceCards(current, incoming []map[string]interface{}) []map[string]interface{} {
	if len(current) == 0 && len(incoming) == 0 {
		return nil
	}
	out := deepResearchCloneSourceCards(current)
	indexByKey := make(map[string]int, len(out))
	for idx, source := range out {
		if key := deepResearchSourceIdentity(source); key != "" {
			indexByKey[key] = idx
		}
	}
	for _, candidate := range incoming {
		source := deepResearchNormalizeSourceCard(candidate, "")
		if source == nil {
			continue
		}
		key := deepResearchSourceIdentity(source)
		if key == "" {
			continue
		}
		if idx, ok := indexByKey[key]; ok {
			merged := out[idx]
			for k, v := range source {
				if strings.TrimSpace(deepResearchString(merged[k])) == "" && strings.TrimSpace(deepResearchString(v)) != "" {
					merged[k] = v
				}
			}
			continue
		}
		out = append(out, source)
		indexByKey[key] = len(out) - 1
	}
	if len(out) > deepResearchLiveSourceLimit {
		out = append([]map[string]interface{}(nil), out[len(out)-deepResearchLiveSourceLimit:]...)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func deepResearchCloneSourceCards(sources []map[string]interface{}) []map[string]interface{} {
	if len(sources) == 0 {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(sources))
	for _, source := range sources {
		if source == nil {
			continue
		}
		cp := make(map[string]interface{}, len(source))
		for k, v := range source {
			cp[k] = v
		}
		out = append(out, cp)
	}
	if len(out) == 0 {
		return nil
	}
	return out
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
		"conversation_id":      current.ConversationID,
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
	if isKnowledgeBaseReportStyle(current.ReportStyle) {
		data["workflow_phases"] = deepResearchWorkflowPhases(current.Status, current.Stage)
		data["object_map"] = buildKnowledgeBaseObjectMap(current.Query, current.Lang, current.Tasks)
		data["source_inventory"] = buildKnowledgeBaseSourceInventory(current.Evidence)
		data["coverage_summary"] = buildKnowledgeBaseCoverage(current)
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
		data["calibration"] = current.Report.Calibration
		if current.Report.Calibration != nil {
			data["takeaway_candidates"] = current.Report.Calibration.TakeawayCandidates
		}
	}
	return data
}

type deepResearchXMLResult struct {
	XMLName             xml.Name                            `xml:"deep_research"`
	JobID               string                              `xml:"job_id,attr,omitempty"`
	ConversationID      string                              `xml:"conversation_id,attr,omitempty"`
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
		JobID:          current.ID,
		ConversationID: current.ConversationID,
		Status:         string(current.Status),
		Mode:           string(current.Mode),
		Query:          current.Query,
		Progress:       current.Progress,
		Iteration:      current.Iteration,
		LatestGap:      current.LatestGap,
		LatestAction:   current.LatestAction,
		EvidenceCount:  len(current.Evidence),
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

func deepResearchCompatRaw(args map[string]interface{}, keys ...string) interface{} {
	for _, key := range keys {
		if value, ok := args[key]; ok && value != nil {
			return value
		}
	}
	for _, containerKey := range []string{"arguments", "input", "params", "payload"} {
		nested, ok := deepResearchCompatMap(args[containerKey])
		if !ok {
			continue
		}
		for _, key := range keys {
			if value, ok := nested[key]; ok && value != nil {
				return value
			}
		}
	}
	return nil
}

func deepResearchCompatMap(v interface{}) (map[string]interface{}, bool) {
	typed, ok := v.(map[string]interface{})
	return typed, ok
}

func deepResearchCompatString(args map[string]interface{}, keys ...string) string {
	v := deepResearchCompatRaw(args, keys...)
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
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
