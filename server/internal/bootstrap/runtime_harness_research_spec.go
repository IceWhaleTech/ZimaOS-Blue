package bootstrap

import (
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
)

type researchHarnessRunInput struct {
	Query          string
	UserID         string
	ConversationID string
	WorkspaceRoot  string
	Mode           string
	ResearchDepth  string
	RouteMode      string
	Lang           string
	ReportStyle    string
	TimeWindows    []string
	StrictEntity   bool
	MaxSources     int
	MaxSeconds     int
	Topic          string
	URLs           []string
	Text           string
	SearchQueries  []string
	OutputMode     string
	Action         string
	URL            string
	Image          string
	Device         string
	Channel        string
	WaitMS         int
	Threshold      float64
	Format         string
	Profile        string
}

func newResearchHarnessRunSpec(input researchHarnessRunInput) harness.RunSpec {
	metadata := map[string]interface{}{
		"mode":           strings.TrimSpace(input.Mode),
		"research_depth": strings.TrimSpace(input.ResearchDepth),
		"route_mode":     strings.TrimSpace(input.RouteMode),
		"lang":           strings.TrimSpace(input.Lang),
		"report_style":   strings.TrimSpace(input.ReportStyle),
		"time_windows":   append([]string(nil), input.TimeWindows...),
		"strict_entity":  input.StrictEntity,
		"max_sources":    input.MaxSources,
		"max_seconds":    input.MaxSeconds,
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
		WorkspaceRoot:  strings.TrimSpace(input.WorkspaceRoot),
		Metadata:       metadata,
	}
}
