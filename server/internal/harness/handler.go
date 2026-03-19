package harness

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/labstack/echo/v4"
)

type Handler struct {
	manager        *Controller
	detailProvider RunDetailProvider
}

func NewHandler(manager *Controller) *Handler {
	return &Handler{manager: manager}
}

type RunDetailProvider interface {
	PendingApprovals(runID string) []map[string]interface{}
	PendingQuestions(runID string) []map[string]interface{}
}

type RunDetail struct {
	Run              *Run                     `json:"run"`
	Events           []RunEvent               `json:"events"`
	Artifacts        []ArtifactRef            `json:"artifacts"`
	PendingApprovals []map[string]interface{} `json:"pending_approvals,omitempty"`
	PendingQuestions []map[string]interface{} `json:"pending_questions,omitempty"`
}

func (h *Handler) SetDetailProvider(provider RunDetailProvider) {
	if h == nil {
		return
	}
	h.detailProvider = provider
}

func (h *Handler) RegisterRoutes(g *echo.Group) {
	if h == nil || h.manager == nil || g == nil {
		return
	}
	g.POST("/runs", h.CreateRun)
	g.GET("/runs", h.ListRuns)
	g.GET("/runs/:id", h.GetRun)
	g.GET("/runs/:id/detail", h.GetRunDetail)
	g.POST("/runs/:id/cancel", h.CancelRun)
	g.GET("/runs/:id/events", h.ListEvents)
	g.GET("/runs/:id/artifacts", h.ListArtifacts)
}

func (h *Handler) CreateRun(c echo.Context) error {
	var spec RunSpec
	if err := c.Bind(&spec); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if userID := harnessUserID(c); userID != "" {
		spec.UserID = userID
	}
	run, err := h.manager.Submit(c.Request().Context(), spec)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusCreated, run)
}

func (h *Handler) ListRuns(c echo.Context) error {
	filter := RunFilter{
		UserID: harnessUserID(c),
		Limit:  50,
	}
	if rawLimit := strings.TrimSpace(c.QueryParam("limit")); rawLimit != "" {
		if limit, err := strconv.Atoi(rawLimit); err == nil && limit > 0 {
			filter.Limit = limit
		}
	}
	if kinds := parseRunKinds(c.QueryParams()["kind"], c.QueryParams()["kinds"]); len(kinds) == 1 {
		filter.Kind = kinds[0]
	} else if len(kinds) > 1 {
		filter.Kinds = kinds
	}
	if statuses := parseRunStatuses(c.QueryParams()["status"], c.QueryParams()["statuses"]); len(statuses) > 0 {
		filter.Statuses = statuses
	}
	if parentID := strings.TrimSpace(c.QueryParam("parent_run_id")); parentID != "" {
		filter.ParentRunID = parentID
	}
	if rootID := strings.TrimSpace(c.QueryParam("root_run_id")); rootID != "" {
		filter.RootRunID = rootID
	}
	runs, err := h.manager.List(c.Request().Context(), filter)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, runs)
}

func (h *Handler) GetRun(c echo.Context) error {
	run, err := h.scopedRun(c)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "run not found"})
	}
	return c.JSON(http.StatusOK, run)
}

func (h *Handler) GetRunDetail(c echo.Context) error {
	run, err := h.scopedRun(c)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "run not found"})
	}
	events, err := h.manager.ListEvents(c.Request().Context(), run.ID, defaultEventLimit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	artifacts, err := h.manager.ListArtifacts(c.Request().Context(), run.ID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	detail := RunDetail{
		Run:       run,
		Events:    events,
		Artifacts: artifacts,
	}
	if h.detailProvider != nil {
		detail.PendingApprovals = h.detailProvider.PendingApprovals(run.ID)
		detail.PendingQuestions = h.detailProvider.PendingQuestions(run.ID)
	}
	return c.JSON(http.StatusOK, detail)
}

func (h *Handler) CancelRun(c echo.Context) error {
	run, err := h.manager.Get(c.Request().Context(), c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "run not found"})
	}
	if userID := harnessUserID(c); userID != "" && run.UserID != "" && run.UserID != userID {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "run not found"})
	}
	if err := h.manager.Cancel(c.Request().Context(), run.ID, "cancelled by user"); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "cancelled"})
}

func (h *Handler) ListEvents(c echo.Context) error {
	run, err := h.scopedRun(c)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "run not found"})
	}
	events, err := h.manager.ListEvents(c.Request().Context(), run.ID, defaultEventLimit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, events)
}

func (h *Handler) ListArtifacts(c echo.Context) error {
	run, err := h.scopedRun(c)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "run not found"})
	}
	artifacts, err := h.manager.ListArtifacts(c.Request().Context(), run.ID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, artifacts)
}

func harnessUserID(c echo.Context) string {
	if claims := auth.GetUserFromContext(c); claims != nil {
		return strings.TrimSpace(claims.UserID)
	}
	return ""
}

func (h *Handler) scopedRun(c echo.Context) (*Run, error) {
	run, err := h.manager.Get(c.Request().Context(), c.Param("id"))
	if err != nil {
		return nil, err
	}
	if userID := harnessUserID(c); userID != "" && run.UserID != "" && run.UserID != userID {
		return nil, echo.ErrNotFound
	}
	return run, nil
}

func parseRunKinds(values ...[]string) []RunKind {
	raw := flattenQueryValues(values...)
	out := make([]RunKind, 0, len(raw))
	seen := make(map[RunKind]struct{}, len(raw))
	for _, item := range raw {
		kind := RunKind(strings.TrimSpace(item))
		if kind == "" {
			continue
		}
		if _, ok := seen[kind]; ok {
			continue
		}
		seen[kind] = struct{}{}
		out = append(out, kind)
	}
	return out
}

func parseRunStatuses(values ...[]string) []RunStatus {
	raw := flattenQueryValues(values...)
	out := make([]RunStatus, 0, len(raw))
	seen := make(map[RunStatus]struct{}, len(raw))
	for _, item := range raw {
		status := RunStatus(strings.TrimSpace(item))
		if status == "" {
			continue
		}
		if _, ok := seen[status]; ok {
			continue
		}
		seen[status] = struct{}{}
		out = append(out, status)
	}
	return out
}

func flattenQueryValues(groups ...[]string) []string {
	var out []string
	for _, group := range groups {
		for _, raw := range group {
			for _, part := range strings.Split(raw, ",") {
				trimmed := strings.TrimSpace(part)
				if trimmed == "" {
					continue
				}
				out = append(out, trimmed)
			}
		}
	}
	return out
}
