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
	RouteMode      string
	Lang           string
	ReportStyle    string
	TimeWindows    []string
	StrictEntity   bool
	MaxSources     int
	MaxSeconds     int
}

func newResearchHarnessRunSpec(input researchHarnessRunInput) harness.RunSpec {
	return harness.RunSpec{
		Kind:           harness.RunKindResearch,
		Goal:           strings.TrimSpace(input.Query),
		UserID:         strings.TrimSpace(input.UserID),
		ConversationID: strings.TrimSpace(input.ConversationID),
		SessionID:      strings.TrimSpace(input.ConversationID),
		WorkspaceRoot:  strings.TrimSpace(input.WorkspaceRoot),
		Metadata: map[string]interface{}{
			"mode":          strings.TrimSpace(input.Mode),
			"route_mode":    strings.TrimSpace(input.RouteMode),
			"lang":          strings.TrimSpace(input.Lang),
			"report_style":  strings.TrimSpace(input.ReportStyle),
			"time_windows":  append([]string(nil), input.TimeWindows...),
			"strict_entity": input.StrictEntity,
			"max_sources":   input.MaxSources,
			"max_seconds":   input.MaxSeconds,
		},
	}
}
