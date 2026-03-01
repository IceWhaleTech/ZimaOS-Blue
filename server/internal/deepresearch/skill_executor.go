package deepresearch

import (
	"context"
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

func NewSkillExecutor(service *Service) *SkillExecutor {
	return &SkillExecutor{service: service}
}

// Execute runs deep research synchronously for skill invocation and returns a map result.
func (e *SkillExecutor) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	query, _ := args["query"].(string)
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}

	mode := ModeStandard
	if v, ok := args["mode"].(string); ok {
		switch Mode(strings.TrimSpace(v)) {
		case ModeFast, ModeStandard, ModeDeep:
			mode = Mode(strings.TrimSpace(v))
		}
	}

	lang, _ := args["lang"].(string)
	var budget *Budget
	maxSources := parseIntArg(args["max_sources"])
	maxSeconds := parseIntArg(args["max_seconds"])
	if maxSources > 0 || maxSeconds > 0 {
		budget = &Budget{
			MaxSources: maxSources,
			MaxSeconds: maxSeconds,
		}
	}
	job, err := e.service.CreateJob(ctx, CreateJobRequest{
		Query:  query,
		Mode:   mode,
		Lang:   strings.TrimSpace(lang),
		Budget: budget,
	})
	if err != nil {
		return nil, err
	}

	ticker := time.NewTicker(120 * time.Millisecond)
	defer ticker.Stop()
	lastStage := ""
	lastProgress := -1

	for {
		select {
		case <-ctx.Done():
			_ = e.service.CancelJob(job.ID)
			return nil, ctx.Err()
		case <-ticker.C:
			current, err := e.service.GetJob(job.ID)
			if err != nil {
				return nil, err
			}
			if current.Stage != lastStage || current.Progress != lastProgress {
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
			}
			switch current.Status {
			case JobStatusCompleted:
				data := map[string]interface{}{
					"job_id":         current.ID,
					"status":         string(current.Status),
					"mode":           string(current.Mode),
					"query":          current.Query,
					"progress":       current.Progress,
					"evidence_count": len(current.Evidence),
				}
				if current.Report != nil {
					data["answer"] = current.Report.Answer
					data["confidence"] = current.Report.Confidence
					data["citations"] = current.Report.Citations
					data["open_questions"] = current.Report.OpenQuestions
				}
				return data, nil
			case JobStatusFailed:
				if current.Error == "" {
					return nil, fmt.Errorf("deep research failed")
				}
				return nil, fmt.Errorf("deep research failed: %s", current.Error)
			case JobStatusCancelled:
				return nil, fmt.Errorf("deep research cancelled")
			}
		}
	}
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
