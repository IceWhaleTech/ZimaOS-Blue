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
	g.POST("/datasets", h.CreateDataset)
	g.GET("/datasets", h.ListDatasets)
	g.GET("/datasets/:id", h.GetDataset)
	g.POST("/datasets/:id/versions", h.CreateDatasetVersion)
	g.GET("/datasets/:id/versions", h.ListDatasetVersions)
	g.GET("/dataset-versions/:id", h.GetDatasetVersion)
	g.POST("/eval-specs", h.CreateEvalSpec)
	g.GET("/eval-specs", h.ListEvalSpecs)
	g.GET("/eval-specs/:id", h.GetEvalSpec)
	g.POST("/eval-runs", h.CreateEvalRun)
	g.GET("/eval-runs", h.ListEvalRuns)
	g.GET("/eval-runs/:id", h.GetEvalRun)
	g.GET("/eval-runs/:id/report", h.GetEvalRunReport)
	g.POST("/eval-runs/:id/cancel", h.CancelEvalRun)
	g.POST("/groups", h.CreateGroup)
	g.GET("/groups", h.ListGroups)
	g.GET("/groups/:id", h.GetGroup)
	g.GET("/groups/:id/items", h.ListGroupItems)
	g.GET("/groups/:id/report", h.GetGroupReport)
	g.POST("/groups/:id/cancel", h.CancelGroup)
	g.POST("/groups/:id/retry_failed", h.RetryFailedGroup)
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

func (h *Handler) CreateDataset(c echo.Context) error {
	var spec DatasetSpec
	if err := c.Bind(&spec); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if userID := harnessUserID(c); userID != "" {
		spec.OwnerUserID = userID
	}
	dataset, err := h.manager.CreateDataset(c.Request().Context(), spec)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusCreated, dataset)
}

func (h *Handler) ListDatasets(c echo.Context) error {
	filter := DatasetFilter{
		OwnerUserID: harnessUserID(c),
		Limit:       50,
	}
	if rawLimit := strings.TrimSpace(c.QueryParam("limit")); rawLimit != "" {
		if limit, err := strconv.Atoi(rawLimit); err == nil && limit > 0 {
			filter.Limit = limit
		}
	}
	datasets, err := h.manager.ListDatasets(c.Request().Context(), filter)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, datasets)
}

func (h *Handler) GetDataset(c echo.Context) error {
	dataset, err := h.scopedDataset(c)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "dataset not found"})
	}
	return c.JSON(http.StatusOK, dataset)
}

func (h *Handler) CreateDatasetVersion(c echo.Context) error {
	dataset, err := h.scopedDataset(c)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "dataset not found"})
	}
	var spec DatasetVersionSpec
	if err := c.Bind(&spec); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if userID := harnessUserID(c); userID != "" {
		spec.CreatedBy = userID
	}
	version, err := h.manager.CreateDatasetVersion(c.Request().Context(), dataset.ID, spec)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusCreated, version)
}

func (h *Handler) ListDatasetVersions(c echo.Context) error {
	dataset, err := h.scopedDataset(c)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "dataset not found"})
	}
	limit := 50
	if rawLimit := strings.TrimSpace(c.QueryParam("limit")); rawLimit != "" {
		if parsed, parseErr := strconv.Atoi(rawLimit); parseErr == nil && parsed > 0 {
			limit = parsed
		}
	}
	versions, err := h.manager.ListDatasetVersions(c.Request().Context(), dataset.ID, limit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, versions)
}

func (h *Handler) GetDatasetVersion(c echo.Context) error {
	version, err := h.scopedDatasetVersion(c)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "dataset version not found"})
	}
	return c.JSON(http.StatusOK, version)
}

func (h *Handler) CreateEvalSpec(c echo.Context) error {
	var spec EvalSpecSpec
	if err := c.Bind(&spec); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if userID := harnessUserID(c); userID != "" {
		spec.OwnerUserID = userID
	}
	evalSpec, err := h.manager.CreateEvalSpec(c.Request().Context(), spec)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusCreated, evalSpec)
}

func (h *Handler) ListEvalSpecs(c echo.Context) error {
	filter := EvalSpecFilter{
		OwnerUserID: harnessUserID(c),
		Limit:       50,
	}
	if rawLimit := strings.TrimSpace(c.QueryParam("limit")); rawLimit != "" {
		if limit, err := strconv.Atoi(rawLimit); err == nil && limit > 0 {
			filter.Limit = limit
		}
	}
	if datasetID := strings.TrimSpace(c.QueryParam("dataset_id")); datasetID != "" {
		filter.DatasetID = datasetID
	}
	specs, err := h.manager.ListEvalSpecs(c.Request().Context(), filter)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, specs)
}

func (h *Handler) GetEvalSpec(c echo.Context) error {
	evalSpec, err := h.scopedEvalSpec(c)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "eval spec not found"})
	}
	return c.JSON(http.StatusOK, evalSpec)
}

func (h *Handler) CreateEvalRun(c echo.Context) error {
	var spec EvalRunSpec
	if err := c.Bind(&spec); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if userID := harnessUserID(c); userID != "" {
		spec.OwnerUserID = userID
	}
	evalRun, err := h.manager.SubmitEvalRun(c.Request().Context(), spec)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusCreated, evalRun)
}

func (h *Handler) ListEvalRuns(c echo.Context) error {
	filter := EvalRunFilter{
		OwnerUserID: harnessUserID(c),
		Limit:       50,
	}
	if rawLimit := strings.TrimSpace(c.QueryParam("limit")); rawLimit != "" {
		if limit, err := strconv.Atoi(rawLimit); err == nil && limit > 0 {
			filter.Limit = limit
		}
	}
	if evalSpecID := strings.TrimSpace(c.QueryParam("eval_spec_id")); evalSpecID != "" {
		filter.EvalSpecID = evalSpecID
	}
	if statuses := parseRunGroupStatuses(c.QueryParams()["status"], c.QueryParams()["statuses"]); len(statuses) > 0 {
		filter.Statuses = statuses
	}
	evalRuns, err := h.manager.ListEvalRuns(c.Request().Context(), filter)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, evalRuns)
}

func (h *Handler) GetEvalRun(c echo.Context) error {
	evalRun, err := h.scopedEvalRun(c)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "eval run not found"})
	}
	return c.JSON(http.StatusOK, evalRun)
}

func (h *Handler) GetEvalRunReport(c echo.Context) error {
	evalRun, err := h.scopedEvalRun(c)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "eval run not found"})
	}
	report, err := h.manager.GetEvalRunReport(c.Request().Context(), evalRun.ID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, report)
}

func (h *Handler) CancelEvalRun(c echo.Context) error {
	evalRun, err := h.scopedEvalRun(c)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "eval run not found"})
	}
	if err := h.manager.CancelEvalRun(c.Request().Context(), evalRun.ID, "cancelled by user"); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "cancelled"})
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
	if groupID := strings.TrimSpace(c.QueryParam("group_id")); groupID != "" {
		filter.GroupID = groupID
	}
	if groupItemID := strings.TrimSpace(c.QueryParam("group_item_id")); groupItemID != "" {
		filter.GroupItemID = groupItemID
	}
	runs, err := h.manager.List(c.Request().Context(), filter)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, runs)
}

func (h *Handler) CreateGroup(c echo.Context) error {
	var spec RunGroupSpec
	if err := c.Bind(&spec); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if userID := harnessUserID(c); userID != "" {
		spec.OwnerUserID = userID
	}
	group, err := h.manager.SubmitGroup(c.Request().Context(), spec)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusCreated, group)
}

func (h *Handler) ListGroups(c echo.Context) error {
	filter := RunGroupFilter{
		OwnerUserID: harnessUserID(c),
		Limit:       50,
	}
	if rawLimit := strings.TrimSpace(c.QueryParam("limit")); rawLimit != "" {
		if limit, err := strconv.Atoi(rawLimit); err == nil && limit > 0 {
			filter.Limit = limit
		}
	}
	if kinds := parseRunGroupKinds(c.QueryParams()["kind"], c.QueryParams()["kinds"]); len(kinds) > 0 {
		filter.Kinds = kinds
	}
	if statuses := parseRunGroupStatuses(c.QueryParams()["status"], c.QueryParams()["statuses"]); len(statuses) > 0 {
		filter.Statuses = statuses
	}
	groups, err := h.manager.ListGroups(c.Request().Context(), filter)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, groups)
}

func (h *Handler) GetGroup(c echo.Context) error {
	group, err := h.scopedGroup(c)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "group not found"})
	}
	return c.JSON(http.StatusOK, group)
}

func (h *Handler) ListGroupItems(c echo.Context) error {
	group, err := h.scopedGroup(c)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "group not found"})
	}
	items, err := h.manager.ListGroupItems(c.Request().Context(), group.ID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, items)
}

func (h *Handler) GetGroupReport(c echo.Context) error {
	group, err := h.scopedGroup(c)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "group not found"})
	}
	report, err := h.manager.GetGroupReport(c.Request().Context(), group.ID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, report)
}

func (h *Handler) CancelGroup(c echo.Context) error {
	group, err := h.scopedGroup(c)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "group not found"})
	}
	if err := h.manager.CancelGroup(c.Request().Context(), group.ID, "cancelled by user"); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "cancelled"})
}

func (h *Handler) RetryFailedGroup(c echo.Context) error {
	group, err := h.scopedGroup(c)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "group not found"})
	}
	retried, err := h.manager.RetryFailedGroup(c.Request().Context(), group.ID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"retried": retried})
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

func (h *Handler) scopedGroup(c echo.Context) (*RunGroup, error) {
	group, err := h.manager.GetGroup(c.Request().Context(), c.Param("id"))
	if err != nil {
		return nil, err
	}
	if userID := harnessUserID(c); userID != "" && group.OwnerUserID != "" && group.OwnerUserID != userID {
		return nil, echo.ErrNotFound
	}
	return group, nil
}

func (h *Handler) scopedDataset(c echo.Context) (*Dataset, error) {
	dataset, err := h.manager.GetDataset(c.Request().Context(), c.Param("id"))
	if err != nil {
		return nil, err
	}
	if userID := harnessUserID(c); userID != "" && dataset.OwnerUserID != "" && dataset.OwnerUserID != userID {
		return nil, echo.ErrNotFound
	}
	return dataset, nil
}

func (h *Handler) scopedDatasetVersion(c echo.Context) (*DatasetVersion, error) {
	version, err := h.manager.GetDatasetVersion(c.Request().Context(), c.Param("id"))
	if err != nil {
		return nil, err
	}
	dataset, err := h.manager.GetDataset(c.Request().Context(), version.DatasetID)
	if err != nil {
		return nil, err
	}
	if userID := harnessUserID(c); userID != "" && dataset.OwnerUserID != "" && dataset.OwnerUserID != userID {
		return nil, echo.ErrNotFound
	}
	return version, nil
}

func (h *Handler) scopedEvalSpec(c echo.Context) (*EvalSpec, error) {
	evalSpec, err := h.manager.GetEvalSpec(c.Request().Context(), c.Param("id"))
	if err != nil {
		return nil, err
	}
	if userID := harnessUserID(c); userID != "" && evalSpec.OwnerUserID != "" && evalSpec.OwnerUserID != userID {
		return nil, echo.ErrNotFound
	}
	return evalSpec, nil
}

func (h *Handler) scopedEvalRun(c echo.Context) (*EvalRun, error) {
	evalRun, err := h.manager.GetEvalRun(c.Request().Context(), c.Param("id"))
	if err != nil {
		return nil, err
	}
	if userID := harnessUserID(c); userID != "" && evalRun.OwnerUserID != "" && evalRun.OwnerUserID != userID {
		return nil, echo.ErrNotFound
	}
	return evalRun, nil
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

func parseRunGroupKinds(values ...[]string) []RunGroupKind {
	raw := flattenQueryValues(values...)
	out := make([]RunGroupKind, 0, len(raw))
	seen := make(map[RunGroupKind]struct{}, len(raw))
	for _, item := range raw {
		kind := RunGroupKind(strings.TrimSpace(item))
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

func parseRunGroupStatuses(values ...[]string) []RunGroupStatus {
	raw := flattenQueryValues(values...)
	out := make([]RunGroupStatus, 0, len(raw))
	seen := make(map[RunGroupStatus]struct{}, len(raw))
	for _, item := range raw {
		status := RunGroupStatus(strings.TrimSpace(item))
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
