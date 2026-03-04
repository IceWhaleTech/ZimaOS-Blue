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
				"type":       "deep-research-progress",
				"job_id":     current.ID,
				"query":      current.Query,
				"mode":       string(current.Mode),
				"stage":      current.Stage,
				"status":     string(current.Status),
				"progress":   current.Progress,
				"_streaming": current.Status != JobStatusCompleted && current.Status != JobStatusFailed && current.Status != JobStatusCancelled,
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
		case _, ok := <-events:
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

func buildDeepResearchResult(current *Job) map[string]interface{} {
	data := map[string]interface{}{
		"job_id":         current.ID,
		"status":         string(current.Status),
		"mode":           string(current.Mode),
		"query":          current.Query,
		"progress":       current.Progress,
		"evidence_count": len(current.Evidence),
		"strict_entity":  current.StrictEntity,
		"time_windows":   current.TimeWindows,
		"report_style":   current.ReportStyle,
	}
	if current.Report != nil {
		data["answer"] = current.Report.Answer
		data["confidence"] = current.Report.Confidence
		data["citations"] = current.Report.Citations
		data["open_questions"] = current.Report.OpenQuestions
		data["support_count"] = current.Report.SupportCount
		data["conflict_count"] = current.Report.ConflictCount
		data["has_conflict"] = current.Report.HasConflict
		data["citation_coverage"] = current.Report.CitationCoverage
		data["entity_disambiguation"] = current.Report.EntityDisambiguation
		data["stage_errors"] = current.Report.StageErrors
		data["timeline_sections"] = current.Report.TimelineSections
	}
	return data
}

type deepResearchXMLResult struct {
	XMLName          xml.Name                  `xml:"deep_research"`
	JobID            string                    `xml:"job_id,attr,omitempty"`
	Status           string                    `xml:"status,attr,omitempty"`
	Mode             string                    `xml:"mode,attr,omitempty"`
	Query            string                    `xml:"query"`
	Progress         int                       `xml:"progress"`
	EvidenceCount    int                       `xml:"evidence_count"`
	Answer           string                    `xml:"answer,omitempty"`
	Confidence       *float64                  `xml:"confidence,omitempty"`
	Citations        []deepResearchXMLCitation `xml:"citations>citation,omitempty"`
	OpenQuestions    []string                  `xml:"open_questions>question,omitempty"`
	SupportCount     *int                      `xml:"support_count,omitempty"`
	ConflictCount    *int                      `xml:"conflict_count,omitempty"`
	HasConflict      *bool                     `xml:"has_conflict,omitempty"`
	CitationCoverage *float64                  `xml:"citation_coverage,omitempty"`
}

type deepResearchXMLCitation struct {
	EvidenceID string `xml:"evidence_id,attr,omitempty"`
	Title      string `xml:"title"`
	URL        string `xml:"url"`
}

func marshalDeepResearchXML(current *Job) (string, error) {
	payload := deepResearchXMLResult{
		JobID:         current.ID,
		Status:        string(current.Status),
		Mode:          string(current.Mode),
		Query:         current.Query,
		Progress:      current.Progress,
		EvidenceCount: len(current.Evidence),
	}

	if current.Report != nil {
		payload.Answer = current.Report.Answer
		payload.Confidence = &current.Report.Confidence
		payload.OpenQuestions = append(payload.OpenQuestions, current.Report.OpenQuestions...)
		payload.SupportCount = &current.Report.SupportCount
		payload.ConflictCount = &current.Report.ConflictCount
		payload.HasConflict = &current.Report.HasConflict
		payload.CitationCoverage = &current.Report.CitationCoverage
		payload.Citations = make([]deepResearchXMLCitation, len(current.Report.Citations))
		for i, c := range current.Report.Citations {
			payload.Citations[i] = deepResearchXMLCitation{
				EvidenceID: c.EvidenceID,
				Title:      c.Title,
				URL:        c.URL,
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
