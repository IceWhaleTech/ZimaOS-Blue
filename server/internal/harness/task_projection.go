package harness

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type UserTaskProjection struct {
	ID              string                   `json:"id"`
	Kind            RunKind                  `json:"kind"`
	ConversationID  string                   `json:"conversation_id,omitempty"`
	Scope           string                   `json:"scope"`
	Title           string                   `json:"title"`
	Subtitle        string                   `json:"subtitle,omitempty"`
	Status          string                   `json:"status"`
	Stage           string                   `json:"stage"`
	Progress        int                      `json:"progress"`
	Blocker         *UserTaskBlocker         `json:"blocker,omitempty"`
	ResultPreview   string                   `json:"result_preview,omitempty"`
	ErrorPreview    string                   `json:"error_preview,omitempty"`
	Artifacts       []UserTaskArtifact       `json:"artifacts,omitempty"`
	ResearchSources []UserTaskResearchSource `json:"research_sources,omitempty"`
	Actions         UserTaskActions          `json:"actions"`
	UpdatedAt       time.Time                `json:"updated_at"`
	FinishedAt      *time.Time               `json:"finished_at,omitempty"`
}

type UserTaskBlocker struct {
	Kind         string `json:"kind"`
	Label        string `json:"label"`
	PendingCount int    `json:"pending_count"`
	ModalOnly    bool   `json:"modal_only"`
}

type UserTaskArtifact struct {
	Kind  string `json:"kind"`
	Label string `json:"label"`
	URL   string `json:"url,omitempty"`
}

type UserTaskResearchSource struct {
	Title            string  `json:"title"`
	URL              string  `json:"url,omitempty"`
	Domain           string  `json:"domain,omitempty"`
	SourceType       string  `json:"source_type,omitempty"`
	PublishedAt      string  `json:"published_at,omitempty"`
	FetchedAt        string  `json:"fetched_at,omitempty"`
	RelevanceScore   float64 `json:"relevance_score,omitempty"`
	CredibilityScore float64 `json:"credibility_score,omitempty"`
}

type UserTaskActions struct {
	CanCancel     bool `json:"can_cancel"`
	CanOpenChat   bool `json:"can_open_chat"`
	CanSendUpdate bool `json:"can_send_update"`
}

type UserTaskProjectionFilter struct {
	UserID         string
	ConversationID string
	Scope          string
	Limit          int
}

type UserTaskProjectionService struct {
	manager        *Controller
	detailProvider RunDetailProvider
}

func NewUserTaskProjectionService(manager *Controller, detailProvider RunDetailProvider) *UserTaskProjectionService {
	if manager == nil {
		return nil
	}
	return &UserTaskProjectionService{
		manager:        manager,
		detailProvider: detailProvider,
	}
}

var (
	userTaskProjectionKinds = []RunKind{RunKindAgentTask, RunKindResearch}
	activeRunStatuses       = []RunStatus{
		RunStatusPending,
		RunStatusPlanning,
		RunStatusWaitingInput,
		RunStatusExecuting,
		RunStatusVerifying,
	}
	terminalRunStatuses = []RunStatus{
		RunStatusCompleted,
		RunStatusFailed,
		RunStatusCancelled,
		RunStatusAborted,
	}
)

func (s *UserTaskProjectionService) List(ctx context.Context, filter UserTaskProjectionFilter) ([]UserTaskProjection, error) {
	if s == nil || s.manager == nil {
		return nil, nil
	}
	filter.Scope = normalizedProjectionScope(filter.Scope)
	filter.Limit = normalizedProjectionLimit(filter.Scope, filter.Limit)

	switch filter.Scope {
	case "background":
		return s.listBackground(ctx, filter)
	case "all":
		return s.listAll(ctx, filter)
	default:
		return s.listCurrent(ctx, filter)
	}
}

func (s *UserTaskProjectionService) Get(ctx context.Context, runID string, scope string) (*UserTaskProjection, error) {
	if s == nil || s.manager == nil {
		return nil, nil
	}
	run, err := s.manager.Get(ctx, strings.TrimSpace(runID))
	if err != nil {
		return nil, err
	}
	return s.projectionForRun(ctx, run, normalizedProjectionScope(scope))
}

func (s *UserTaskProjectionService) listCurrent(ctx context.Context, filter UserTaskProjectionFilter) ([]UserTaskProjection, error) {
	conversationID := strings.TrimSpace(filter.ConversationID)
	if conversationID == "" {
		return []UserTaskProjection{}, nil
	}
	activeRuns, err := s.manager.List(ctx, RunFilter{
		UserID:         strings.TrimSpace(filter.UserID),
		Kinds:          userTaskProjectionKinds,
		ConversationID: conversationID,
		Statuses:       activeRunStatuses,
		Limit:          max(filter.Limit*3, 25),
	})
	if err != nil {
		return nil, err
	}
	active := s.projectRuns(ctx, activeRuns, "current")
	if len(active) >= filter.Limit {
		return active[:filter.Limit], nil
	}

	terminalRuns, err := s.manager.List(ctx, RunFilter{
		UserID:         strings.TrimSpace(filter.UserID),
		Kinds:          userTaskProjectionKinds,
		ConversationID: conversationID,
		Statuses:       terminalRunStatuses,
		Limit:          max(filter.Limit, 10),
	})
	if err != nil {
		return nil, err
	}
	terminal := s.projectRuns(ctx, terminalRuns, "current")
	return appendWithLimit(active, terminal, filter.Limit), nil
}

func (s *UserTaskProjectionService) listBackground(ctx context.Context, filter UserTaskProjectionFilter) ([]UserTaskProjection, error) {
	runs, err := s.manager.List(ctx, RunFilter{
		UserID:   strings.TrimSpace(filter.UserID),
		Kinds:    userTaskProjectionKinds,
		Statuses: activeRunStatuses,
		Limit:    max(filter.Limit*5, 25),
	})
	if err != nil {
		return nil, err
	}
	filtered := make([]Run, 0, len(runs))
	currentConversationID := strings.TrimSpace(filter.ConversationID)
	for _, run := range runs {
		if currentConversationID != "" && strings.TrimSpace(run.ConversationID) == currentConversationID {
			continue
		}
		filtered = append(filtered, run)
	}
	projected := s.projectRuns(ctx, filtered, "background")
	if len(projected) > filter.Limit {
		projected = projected[:filter.Limit]
	}
	return projected, nil
}

func (s *UserTaskProjectionService) listAll(ctx context.Context, filter UserTaskProjectionFilter) ([]UserTaskProjection, error) {
	current, err := s.listCurrent(ctx, UserTaskProjectionFilter{
		UserID:         filter.UserID,
		ConversationID: filter.ConversationID,
		Scope:          "current",
		Limit:          filter.Limit,
	})
	if err != nil {
		return nil, err
	}
	background, err := s.listBackground(ctx, UserTaskProjectionFilter{
		UserID:         filter.UserID,
		ConversationID: filter.ConversationID,
		Scope:          "background",
		Limit:          filter.Limit,
	})
	if err != nil {
		return nil, err
	}
	merged := make([]UserTaskProjection, 0, len(current)+len(background))
	seen := make(map[string]struct{}, len(current)+len(background))
	for _, projection := range current {
		merged = append(merged, projection)
		seen[projection.ID] = struct{}{}
	}
	for _, projection := range background {
		if _, ok := seen[projection.ID]; ok {
			continue
		}
		merged = append(merged, projection)
	}
	sort.SliceStable(merged, func(i, j int) bool {
		return merged[i].UpdatedAt.After(merged[j].UpdatedAt)
	})
	if len(merged) > filter.Limit {
		merged = merged[:filter.Limit]
	}
	return merged, nil
}

func (s *UserTaskProjectionService) projectRuns(ctx context.Context, runs []Run, scope string) []UserTaskProjection {
	projected := make([]UserTaskProjection, 0, len(runs))
	for i := range runs {
		projection, err := s.projectionForRun(ctx, &runs[i], scope)
		if err != nil || projection == nil {
			continue
		}
		projected = append(projected, *projection)
	}
	sort.SliceStable(projected, func(i, j int) bool {
		return projected[i].UpdatedAt.After(projected[j].UpdatedAt)
	})
	return projected
}

func (s *UserTaskProjectionService) projectionForRun(ctx context.Context, run *Run, scope string) (*UserTaskProjection, error) {
	if !isVisibleUserTaskRun(run) {
		return nil, nil
	}
	artifacts, err := s.manager.ListArtifacts(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	var approvals []map[string]interface{}
	var questions []map[string]interface{}
	if s.detailProvider != nil {
		approvals = s.detailProvider.PendingApprovals(run.ID)
		questions = s.detailProvider.PendingQuestions(run.ID)
	}
	stage, status, blocker := userTaskStatusParts(run, approvals, questions)
	title, subtitle := userTaskTitleAndSubtitle(run)
	projection := &UserTaskProjection{
		ID:              run.ID,
		Kind:            run.Kind,
		ConversationID:  strings.TrimSpace(run.ConversationID),
		Scope:           scope,
		Title:           title,
		Subtitle:        subtitle,
		Status:          status,
		Stage:           stage,
		Progress:        normalizedProjectionProgress(run),
		Blocker:         blocker,
		ResultPreview:   trimmedPreview(run.Result, 280),
		ErrorPreview:    trimmedPreview(run.Error, 240),
		Artifacts:       projectUserArtifacts(artifacts),
		ResearchSources: projectResearchSources(run),
		Actions: UserTaskActions{
			CanCancel:     canCancelUserTask(run),
			CanOpenChat:   scope == "background" && strings.TrimSpace(run.ConversationID) != "",
			CanSendUpdate: scope == "current" && run.Kind == RunKindAgentTask && canSendUpdate(run),
		},
		UpdatedAt:  run.UpdatedAt,
		FinishedAt: run.FinishedAt,
	}
	return projection, nil
}

func isVisibleUserTaskRun(run *Run) bool {
	if run == nil {
		return false
	}
	if run.Kind != RunKindAgentTask && run.Kind != RunKindResearch {
		return false
	}
	if strings.TrimSpace(run.ParentRunID) != "" {
		return false
	}
	if strings.TrimSpace(run.GroupID) != "" || strings.TrimSpace(run.GroupItemID) != "" {
		return false
	}
	return true
}

func normalizedProjectionScope(scope string) string {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case "background":
		return "background"
	case "all":
		return "all"
	default:
		return "current"
	}
}

func normalizedProjectionLimit(scope string, limit int) int {
	if limit > 0 {
		return limit
	}
	switch normalizedProjectionScope(scope) {
	case "background":
		return 5
	default:
		return 10
	}
}

func userTaskStatusParts(run *Run, approvals []map[string]interface{}, questions []map[string]interface{}) (string, string, *UserTaskBlocker) {
	switch run.Status {
	case RunStatusPending, RunStatusPlanning:
		return "planning", "running", nil
	case RunStatusExecuting:
		return "working", "running", nil
	case RunStatusVerifying:
		return "verifying", "running", nil
	case RunStatusWaitingInput:
		if count := len(approvals); count > 0 {
			return "waiting_user", "waiting_user", &UserTaskBlocker{
				Kind:         "approval",
				Label:        "Waiting for your approval",
				PendingCount: count,
				ModalOnly:    true,
			}
		}
		if count := len(questions); count > 0 {
			return "waiting_user", "waiting_user", &UserTaskBlocker{
				Kind:         "question",
				Label:        "Waiting for your answer",
				PendingCount: count,
				ModalOnly:    true,
			}
		}
		return "waiting_user", "waiting_user", nil
	case RunStatusCompleted:
		return "completed", "completed", nil
	case RunStatusCancelled:
		return "cancelled", "cancelled", nil
	case RunStatusFailed, RunStatusAborted:
		return "failed", "failed", nil
	default:
		return "planning", "running", nil
	}
}

func userTaskTitleAndSubtitle(run *Run) (string, string) {
	title := strings.TrimSpace(run.Goal)
	if title == "" {
		switch run.Kind {
		case RunKindResearch:
			title = "Research task"
		default:
			title = "Agent task"
		}
	}
	title = trimmedPreview(title, 120)

	switch run.Kind {
	case RunKindResearch:
		stage := metadataString(run.Metadata, "stage")
		latestAction := metadataString(run.Metadata, "latest_action")
		latestGap := metadataString(run.Metadata, "latest_gap")
		parts := make([]string, 0, 3)
		if stage != "" {
			parts = append(parts, stage)
		}
		if latestAction != "" {
			parts = append(parts, latestAction)
		}
		if latestGap != "" {
			parts = append(parts, latestGap)
		}
		return title, trimmedPreview(strings.Join(parts, " • "), 180)
	default:
		if state := strings.TrimSpace(string(run.RuntimeState)); state != "" {
			return title, trimmedPreview(strings.ToLower(state), 120)
		}
		return title, ""
	}
}

func normalizedProjectionProgress(run *Run) int {
	if run == nil {
		return 0
	}
	progress := run.Progress
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	if run.Status == RunStatusCompleted && progress == 0 {
		return 100
	}
	return progress
}

func canCancelUserTask(run *Run) bool {
	if run == nil {
		return false
	}
	switch run.Status {
	case RunStatusPending, RunStatusPlanning, RunStatusWaitingInput, RunStatusExecuting, RunStatusVerifying:
		return true
	default:
		return false
	}
}

func canSendUpdate(run *Run) bool {
	if run == nil || run.Kind != RunKindAgentTask {
		return false
	}
	switch run.Status {
	case RunStatusPending, RunStatusPlanning, RunStatusWaitingInput, RunStatusExecuting, RunStatusVerifying:
		return true
	default:
		return false
	}
}

func trimmedPreview(raw string, limit int) string {
	value := strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
	if value == "" || limit <= 0 || len(value) <= limit {
		return value
	}
	if limit <= 3 {
		return value[:limit]
	}
	return strings.TrimSpace(value[:limit-1]) + "…"
}

func projectUserArtifacts(artifacts []ArtifactRef) []UserTaskArtifact {
	out := make([]UserTaskArtifact, 0, len(artifacts))
	for _, artifact := range artifacts {
		kind := strings.ToLower(strings.TrimSpace(artifact.Kind))
		switch kind {
		case "file", "url", "report", "snapshot":
		default:
			continue
		}
		label := strings.TrimSpace(artifact.Label)
		if label == "" {
			label = basenameForArtifact(artifact.PathOrURL, kind)
		}
		if label == "" {
			label = fallbackArtifactLabel(kind)
		}
		out = append(out, UserTaskArtifact{
			Kind:  kind,
			Label: label,
			URL:   projectionArtifactURL(artifact.PathOrURL),
		})
	}
	return out
}

func projectResearchSources(run *Run) []UserTaskResearchSource {
	if run == nil || run.Kind != RunKindResearch || run.Metadata == nil {
		return nil
	}

	if inventory := metadataObjectSlice(run.Metadata["source_inventory"]); len(inventory) > 0 {
		out := make([]UserTaskResearchSource, 0, len(inventory))
		for _, record := range inventory {
			source := UserTaskResearchSource{
				Title:            metadataStringValue(record, "title"),
				URL:              metadataStringValue(record, "url"),
				Domain:           metadataStringValue(record, "domain"),
				SourceType:       metadataStringValue(record, "source_type"),
				PublishedAt:      metadataStringValue(record, "published_at"),
				FetchedAt:        metadataStringValue(record, "fetched_at"),
				RelevanceScore:   floatMetadata(record["relevance_score"]),
				CredibilityScore: floatMetadata(record["credibility_score"]),
			}
			if source.Title == "" {
				source.Title = source.URL
			}
			if source.Title == "" {
				continue
			}
			out = append(out, source)
		}
		if len(out) > 0 {
			return out
		}
	}

	citations := metadataObjectSlice(run.Metadata["citations"])
	if len(citations) == 0 {
		return nil
	}
	out := make([]UserTaskResearchSource, 0, len(citations))
	for _, record := range citations {
		source := UserTaskResearchSource{
			Title: metadataStringValue(record, "title"),
			URL:   metadataStringValue(record, "url"),
		}
		if source.Title == "" {
			source.Title = source.URL
		}
		if source.Title == "" {
			continue
		}
		out = append(out, source)
	}
	return out
}

func metadataStringValue(record map[string]interface{}, key string) string {
	value, ok := record[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func metadataObjectSlice(raw interface{}) []map[string]interface{} {
	switch value := raw.(type) {
	case []map[string]interface{}:
		return value
	case []interface{}:
		out := make([]map[string]interface{}, 0, len(value))
		for _, item := range value {
			record, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			out = append(out, record)
		}
		return out
	default:
		return nil
	}
}

func floatMetadata(raw interface{}) float64 {
	switch value := raw.(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	case int:
		return float64(value)
	case int64:
		return float64(value)
	default:
		return 0
	}
}

func basenameForArtifact(pathOrURL string, kind string) string {
	trimmed := strings.TrimSpace(pathOrURL)
	if trimmed == "" {
		return ""
	}
	trimmed = strings.TrimRight(trimmed, "/")
	if strings.Contains(trimmed, "://") {
		parts := strings.Split(trimmed, "/")
		return strings.TrimSpace(parts[len(parts)-1])
	}
	base := strings.TrimSpace(filepath.Base(trimmed))
	if base == "." || base == "/" {
		return ""
	}
	return base
}

func projectionArtifactURL(pathOrURL string) string {
	trimmed := strings.TrimSpace(pathOrURL)
	if trimmed == "" {
		return ""
	}
	if strings.Contains(trimmed, "://") {
		return trimmed
	}
	return ""
}

func fallbackArtifactLabel(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "report":
		return "Report"
	case "snapshot":
		return "Snapshot"
	case "url":
		return "Link"
	default:
		return "File"
	}
}

func appendWithLimit(active []UserTaskProjection, terminal []UserTaskProjection, limit int) []UserTaskProjection {
	if limit <= 0 {
		limit = len(active) + len(terminal)
	}
	out := make([]UserTaskProjection, 0, min(limit, len(active)+len(terminal)))
	seen := make(map[string]struct{}, len(active)+len(terminal))
	for _, item := range active {
		if len(out) >= limit {
			return out
		}
		out = append(out, item)
		seen[item.ID] = struct{}{}
	}
	for _, item := range terminal {
		if len(out) >= limit {
			break
		}
		if _, ok := seen[item.ID]; ok {
			continue
		}
		out = append(out, item)
	}
	return out
}

func min(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

func max(left int, right int) int {
	if left > right {
		return left
	}
	return right
}
