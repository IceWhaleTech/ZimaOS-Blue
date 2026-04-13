package bootstrap

import (
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
)

type researchHarnessRunInput struct {
	Query            string
	UserID           string
	ConversationID   string
	WorkspaceRoot    string
	ProviderID       string
	Mode             string
	ResearchDepth    string
	RetrievalProfile string
	RouteMode        string
	Lang             string
	ReportStyle      string
	TimeWindows      []string
	StrictEntity     bool
	MaxSources       int
	MaxSeconds       int
	Topic            string
	URLs             []string
	Text             string
	SearchQueries    []string
	OutputMode       string
	Question         string
	Category         string
	DecisionMode     string
	Candidates       []string
	Context          map[string]interface{}
	Constraints      map[string]interface{}
	Grounding        string
	Depth            string
	Output           string
	ScorecardPack    string
	ScorecardWeights map[string]float64
	Action           string
	URL              string
	Image            string
	Device           string
	Channel          string
	WaitMS           int
	Threshold        float64
	Format           string
	Profile          string
}

func newResearchHarnessRunSpec(input researchHarnessRunInput) harness.RunSpec {
	metadata := map[string]interface{}{
		"mode":              strings.TrimSpace(input.Mode),
		"research_depth":    strings.TrimSpace(input.ResearchDepth),
		"retrieval_profile": strings.TrimSpace(input.RetrievalProfile),
		"route_mode":        strings.TrimSpace(input.RouteMode),
		"lang":              strings.TrimSpace(input.Lang),
		"report_style":      strings.TrimSpace(input.ReportStyle),
		"time_windows":      append([]string(nil), input.TimeWindows...),
		"strict_entity":     input.StrictEntity,
		"max_sources":       input.MaxSources,
		"max_seconds":       input.MaxSeconds,
	}
	if topic := strings.TrimSpace(input.Topic); topic != "" {
		metadata["topic"] = topic
	}
	if len(input.URLs) > 0 {
		metadata["urls"] = append([]string(nil), input.URLs...)
	}
	if text := strings.TrimSpace(input.Text); text != "" {
		metadata["text"] = text
	}
	if len(input.SearchQueries) > 0 {
		metadata["search_queries"] = append([]string(nil), input.SearchQueries...)
	}
	if outputMode := strings.TrimSpace(input.OutputMode); outputMode != "" {
		metadata["output_mode"] = outputMode
	}
	if question := strings.TrimSpace(input.Question); question != "" {
		metadata["question"] = question
	}
	if category := strings.TrimSpace(input.Category); category != "" {
		metadata["category"] = category
	}
	if decisionMode := strings.TrimSpace(input.DecisionMode); decisionMode != "" {
		metadata["decision_mode"] = decisionMode
	}
	if len(input.Candidates) > 0 {
		metadata["candidates"] = append([]string(nil), input.Candidates...)
	}
	if len(input.Context) > 0 {
		metadata["context"] = cloneStringAnyMap(input.Context)
	}
	if len(input.Constraints) > 0 {
		metadata["constraints"] = cloneStringAnyMap(input.Constraints)
	}
	if grounding := strings.TrimSpace(input.Grounding); grounding != "" {
		metadata["grounding"] = grounding
	}
	if depth := strings.TrimSpace(input.Depth); depth != "" {
		metadata["depth"] = depth
	}
	if output := strings.TrimSpace(input.Output); output != "" {
		metadata["output"] = output
	}
	if scorecardPack := strings.TrimSpace(input.ScorecardPack); scorecardPack != "" {
		metadata["scorecard_pack"] = scorecardPack
	}
	if len(input.ScorecardWeights) > 0 {
		metadata["scorecard_weights"] = cloneStringFloatMap(input.ScorecardWeights)
	}
	if action := strings.TrimSpace(input.Action); action != "" {
		metadata["action"] = action
	}
	if url := strings.TrimSpace(input.URL); url != "" {
		metadata["url"] = url
	}
	if image := strings.TrimSpace(input.Image); image != "" {
		metadata["image"] = image
	}
	if device := strings.TrimSpace(input.Device); device != "" {
		metadata["device"] = device
	}
	if channel := strings.TrimSpace(input.Channel); channel != "" {
		metadata["channel"] = channel
	}
	if input.WaitMS > 0 {
		metadata["wait_ms"] = input.WaitMS
	}
	if input.Threshold > 0 {
		metadata["threshold"] = input.Threshold
	}
	if format := strings.TrimSpace(input.Format); format != "" {
		metadata["format"] = format
	}
	if profile := strings.TrimSpace(input.Profile); profile != "" {
		metadata["profile"] = profile
	}

	return harness.RunSpec{
		Kind:           harness.RunKindResearch,
		Goal:           strings.TrimSpace(input.Query),
		UserID:         strings.TrimSpace(input.UserID),
		ConversationID: strings.TrimSpace(input.ConversationID),
		SessionID:      strings.TrimSpace(input.ConversationID),
		ProviderID:     strings.TrimSpace(input.ProviderID),
		WorkspaceRoot:  strings.TrimSpace(input.WorkspaceRoot),
		Metadata:       metadata,
	}
}

func cloneStringAnyMap(in map[string]interface{}) map[string]interface{} {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneStringFloatMap(in map[string]float64) map[string]float64 {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]float64, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
