package harness

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/labstack/echo/v4"
)

type Handler struct {
	manager                 *Controller
	detailProvider          RunDetailProvider
	evolutionProposalCounts EvolutionProposalSummaryProvider
}

func NewHandler(manager *Controller) *Handler {
	return &Handler{manager: manager}
}

type RunDetailProvider interface {
	PendingApprovals(runID string) []map[string]interface{}
	PendingQuestions(runID string) []map[string]interface{}
}

type EvolutionProposalSummaryProvider interface {
	PendingProposalCount(ctx context.Context, ownerUserID string) (int, error)
}

type RunDetail struct {
	Run              *Run                     `json:"run"`
	Actions          RunActionAvailability    `json:"actions"`
	Events           []RunEvent               `json:"events"`
	Artifacts        []ArtifactRef            `json:"artifacts"`
	RunTrace         *RunTrace                `json:"run_trace,omitempty"`
	PendingApprovals []map[string]interface{} `json:"pending_approvals,omitempty"`
	PendingQuestions []map[string]interface{} `json:"pending_questions,omitempty"`
}

func (h *Handler) SetDetailProvider(provider RunDetailProvider) {
	if h == nil {
		return
	}
	h.detailProvider = provider
}

func (h *Handler) SetEvolutionProposalSummaryProvider(provider EvolutionProposalSummaryProvider) {
	if h == nil {
		return
	}
	h.evolutionProposalCounts = provider
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
	g.POST("/runs/:id/actions/:action", h.PerformRunAction)
	g.GET("/runs/:id/events", h.ListEvents)
	g.GET("/runs/:id/artifacts", h.ListArtifacts)
	g.POST("/datasets", h.CreateDataset)
	g.GET("/datasets", h.ListDatasets)
	g.GET("/datasets/:id", h.GetDataset)
	g.POST("/datasets/:id/versions", h.CreateDatasetVersion)
	g.GET("/datasets/:id/versions", h.ListDatasetVersions)
	g.GET("/dataset-versions/:id", h.GetDatasetVersion)
	g.POST("/dataset-bundles/import", h.ImportDatasetBundle)
	g.POST("/dataset-bundles/preview-source", h.PreviewDatasetBundleFromSource)
	g.POST("/dataset-bundles/import-source", h.ImportDatasetBundleFromSource)
	g.POST("/eval-specs", h.CreateEvalSpec)
	g.GET("/eval-specs", h.ListEvalSpecs)
	g.GET("/eval-specs/:id", h.GetEvalSpec)
	g.POST("/eval-runs", h.CreateEvalRun)
	g.GET("/eval-runs", h.ListEvalRuns)
	g.GET("/eval-runs/:id", h.GetEvalRun)
	g.GET("/eval-runs/:id/report", h.GetEvalRunReport)
	g.POST("/eval-runs/:id/cancel", h.CancelEvalRun)
	g.POST("/eval-runs/:id/compare", h.CompareEvalRun)
	g.POST("/eval-runs/:id/selector-gate", h.EvaluateSelectorGate)
	g.POST("/eval-runs/:id/execution-gate", h.EvaluateExecutionEquivalence)
	g.POST("/eval-runs/:id/budget-gate", h.EvaluateSkillCutoverBudgetGate)
	g.POST("/cutover-readiness", h.EvaluateSkillCutoverReadiness)
	g.GET("/comparison-reports/:id", h.GetComparisonReport)
	g.POST("/baselines", h.CreateBaseline)
	g.GET("/baselines", h.ListBaselines)
	g.POST("/skills/:skill_id/optimize", h.OptimizeSkill)
	g.GET("/evolution/overview", h.GetEvolutionOverview)
	g.GET("/skills/:skill_id/revisions", h.ListSkillRevisions)
	g.GET("/skills/:skill_id/decision-history", h.ListSkillDecisionHistory)
	g.GET("/skills/:skill_id/evolution-cases", h.ListSkillEvolutionCases)
	g.GET("/skill-revisions/:id", h.GetSkillRevision)
	g.GET("/skill-evolution-cases/:id", h.GetSkillEvolutionCase)
	g.POST("/skill-revisions/:id/promote", h.PromoteSkillRevision)
	g.POST("/skill-revisions/:id/rollback", h.RollbackSkillRevision)
	g.POST("/selector-curated/ensure", h.EnsureSelectorCuratedAssets)
	g.POST("/execution-batch1/ensure", h.EnsureBatch1ExecutionAssets)
	g.POST("/groups", h.CreateGroup)
	g.GET("/groups", h.ListGroups)
	g.GET("/groups/:id", h.GetGroup)
	g.GET("/groups/:id/items", h.ListGroupItems)
	g.GET("/groups/:id/report", h.GetGroupReport)
	g.POST("/groups/:id/cancel", h.CancelGroup)
	g.POST("/groups/:id/actions/:action", h.PerformGroupAction)
	g.POST("/groups/:id/retry_failed", h.RetryFailedGroup)
	g.POST("/groups/:id/promote", h.PromoteGroup)
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

func (h *Handler) ImportDatasetBundle(c echo.Context) error {
	var req ImportDatasetBundleRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if userID := harnessUserID(c); userID != "" {
		req.Dataset.OwnerUserID = userID
		req.Version.CreatedBy = userID
		for i := range req.EvalSpecs {
			req.EvalSpecs[i].OwnerUserID = userID
		}
	}
	result, err := h.manager.ImportDatasetBundle(c.Request().Context(), req)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) PreviewDatasetBundleFromSource(c echo.Context) error {
	importReq, err := h.loadImportDatasetBundleRequestFromSource(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	preview, err := previewDatasetBundleImportRequest(importReq)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, preview)
}

func (h *Handler) ImportDatasetBundleFromSource(c echo.Context) error {
	importReq, err := h.loadImportDatasetBundleRequestFromSource(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	result, err := h.manager.ImportDatasetBundle(c.Request().Context(), *importReq)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) loadImportDatasetBundleRequestFromSource(c echo.Context) (*ImportDatasetBundleRequest, error) {
	var req ImportDatasetBundleFromSourceRequest
	if err := c.Bind(&req); err != nil {
		return nil, fmt.Errorf("invalid request")
	}

	var (
		importReq *ImportDatasetBundleRequest
		err       error
	)
	switch strings.TrimSpace(req.SourceType) {
	case "local":
		importReq, err = loadImportDatasetBundleRequestFromDir(req.Path, req.Version)
	case "github":
		importReq, err = loadImportDatasetBundleRequestFromGitHubSource(req.Source, req.BundlePath, req.Version)
	default:
		return nil, fmt.Errorf("source_type must be local or github")
	}
	if err != nil {
		return nil, err
	}
	if userID := harnessUserID(c); userID != "" {
		importReq.Dataset.OwnerUserID = userID
		importReq.Version.CreatedBy = userID
		for i := range importReq.EvalSpecs {
			importReq.EvalSpecs[i].OwnerUserID = userID
		}
	}
	if req.MakeActive != nil {
		importReq.MakeActive = *req.MakeActive
	}
	return importReq, nil
}

func previewDatasetBundleImportRequest(req *ImportDatasetBundleRequest) (*DatasetBundleSourcePreview, error) {
	itemCount, err := validateImportDatasetBundleRequest(req)
	if err != nil {
		return nil, err
	}
	preview := &DatasetBundleSourcePreview{
		SourceType: strings.TrimSpace(req.SourceType),
		SourceRef:  strings.TrimSpace(req.SourceRef),
		Dataset: DatasetSpec{
			Name:           strings.TrimSpace(req.Dataset.Name),
			Description:    strings.TrimSpace(req.Dataset.Description),
			OwnerUserID:    strings.TrimSpace(req.Dataset.OwnerUserID),
			Subject:        strings.TrimSpace(req.Dataset.Subject),
			DefaultRunKind: req.Dataset.DefaultRunKind,
			DefaultProfile: strings.TrimSpace(req.Dataset.DefaultProfile),
			Metadata:       cloneMetadataMap(req.Dataset.Metadata),
		},
		Version: DatasetBundleVersionPreview{
			Version:        normalizeDatasetVersion(req.Version.Version),
			ItemCount:      itemCount,
			ManifestSHA256: manifestSHA256(req.Version.Manifest),
			SourceType:     firstNonEmpty(strings.TrimSpace(req.Version.SourceType), strings.TrimSpace(req.SourceType)),
			SourceRef:      firstNonEmpty(strings.TrimSpace(req.Version.SourceRef), strings.TrimSpace(req.SourceRef)),
		},
		EvalSpecs:  make([]ImportDatasetBundleEvalSpec, 0, len(req.EvalSpecs)),
		MakeActive: req.MakeActive,
	}
	for _, spec := range req.EvalSpecs {
		preview.EvalSpecs = append(preview.EvalSpecs, ImportDatasetBundleEvalSpec{
			Name:            strings.TrimSpace(spec.Name),
			OwnerUserID:     strings.TrimSpace(spec.OwnerUserID),
			Subject:         strings.TrimSpace(spec.Subject),
			RunKind:         spec.RunKind,
			Profile:         strings.TrimSpace(spec.Profile),
			SchedulerConfig: spec.SchedulerConfig,
			ScoringConfig:   spec.ScoringConfig,
			RuntimePolicy:   cloneMetadataMap(spec.RuntimePolicy),
			Metadata:        cloneMetadataMap(spec.Metadata),
		})
	}
	return preview, nil
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

func (h *Handler) OptimizeSkill(c echo.Context) error {
	skillID := strings.TrimSpace(c.Param("skill_id"))
	if skillID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "skill_id is required"})
	}
	var req SkillOptimizeRequest
	if err := c.Bind(&req); err != nil && !errors.Is(err, io.EOF) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	evalRun, err := h.manager.store.GetEvalRun(c.Request().Context(), strings.TrimSpace(req.EvalRunID))
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "eval run not found"})
	}
	if userID := harnessUserID(c); userID != "" && evalRun.OwnerUserID != "" && evalRun.OwnerUserID != userID {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "eval run not found"})
	}
	event, err := h.manager.OptimizeSkill(c.Request().Context(), skillID, req)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusAccepted, event)
}

func (h *Handler) GetEvolutionOverview(c echo.Context) error {
	overview, err := h.manager.BuildEvolutionOverview(
		c.Request().Context(),
		strings.TrimSpace(c.QueryParam("skill_id")),
		harnessUserID(c),
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if h.evolutionProposalCounts != nil {
		if pending, countErr := h.evolutionProposalCounts.PendingProposalCount(
			c.Request().Context(),
			harnessUserID(c),
		); countErr == nil {
			overview.Instructions.Pending = pending
		}
	}
	return c.JSON(http.StatusOK, overview)
}

func (h *Handler) ListSkillRevisions(c echo.Context) error {
	filter := SkillRevisionFilter{
		SkillID: strings.TrimSpace(c.Param("skill_id")),
		Limit:   50,
	}
	if rawLimit := strings.TrimSpace(c.QueryParam("limit")); rawLimit != "" {
		if limit, err := strconv.Atoi(rawLimit); err == nil && limit > 0 {
			filter.Limit = limit
		}
	}
	if statuses := parseSkillRevisionStatuses(c.QueryParams()["status"], c.QueryParams()["statuses"]); len(statuses) > 0 {
		filter.Statuses = statuses
	}
	revisions, err := h.manager.ListSkillRevisions(c.Request().Context(), filter)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, revisions)
}

func (h *Handler) ListSkillDecisionHistory(c echo.Context) error {
	filter := SkillDecisionHistoryFilter{
		SkillID: strings.TrimSpace(c.Param("skill_id")),
		Limit:   50,
	}
	if rawLimit := strings.TrimSpace(c.QueryParam("limit")); rawLimit != "" {
		if limit, err := strconv.Atoi(rawLimit); err == nil && limit > 0 {
			filter.Limit = limit
		}
	}
	if actions := parseSkillRevisionDecisionActions(c.QueryParams()["action"], c.QueryParams()["actions"]); len(actions) > 0 {
		filter.Actions = actions
	}
	history, err := h.manager.ListSkillDecisionHistory(c.Request().Context(), filter)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, history)
}

func (h *Handler) ListSkillEvolutionCases(c echo.Context) error {
	filter := SkillEvolutionCaseFilter{
		SkillID:     strings.TrimSpace(c.Param("skill_id")),
		OwnerUserID: harnessUserID(c),
		Limit:       50,
	}
	if rawLimit := strings.TrimSpace(c.QueryParam("limit")); rawLimit != "" {
		if limit, err := strconv.Atoi(rawLimit); err == nil && limit > 0 {
			filter.Limit = limit
		}
	}
	if statuses := parseSkillEvolutionCaseStatuses(c.QueryParams()["status"], c.QueryParams()["statuses"]); len(statuses) > 0 {
		filter.Statuses = statuses
	}
	cases, err := h.manager.ListSkillEvolutionCases(c.Request().Context(), filter)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, cases)
}

func (h *Handler) GetSkillRevision(c echo.Context) error {
	revision, err := h.manager.GetSkillRevision(c.Request().Context(), c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "skill revision not found"})
	}
	return c.JSON(http.StatusOK, revision)
}

func (h *Handler) GetSkillEvolutionCase(c echo.Context) error {
	evolutionCase, err := h.scopedSkillEvolutionCase(c)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "skill evolution case not found"})
	}
	detail, err := h.manager.BuildSkillEvolutionCaseDetail(c.Request().Context(), evolutionCase)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, detail)
}

func (h *Handler) PromoteSkillRevision(c echo.Context) error {
	var req SkillRevisionDecisionRequest
	if err := c.Bind(&req); err != nil && !errors.Is(err, io.EOF) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	req.ReviewedBy = harnessUserID(c)
	result, err := h.manager.PromoteSkillRevision(c.Request().Context(), c.Param("id"), req)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) RollbackSkillRevision(c echo.Context) error {
	var req SkillRevisionDecisionRequest
	if err := c.Bind(&req); err != nil && !errors.Is(err, io.EOF) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	req.ReviewedBy = harnessUserID(c)
	result, err := h.manager.RollbackSkillRevision(c.Request().Context(), c.Param("id"), req)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) CompareEvalRun(c echo.Context) error {
	evalRun, err := h.scopedEvalRun(c)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "eval run not found"})
	}
	var req CompareEvalRunRequest
	if err := c.Bind(&req); err != nil && !errors.Is(err, io.EOF) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if req.BaselineID != "" {
		if _, err := h.scopedBaselineByID(c, req.BaselineID); err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "baseline not found"})
		}
	}
	if req.BaseEvalRunID != "" {
		if _, err := h.scopedEvalRunByID(c, req.BaseEvalRunID); err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "comparison base eval run not found"})
		}
	}
	report, err := h.manager.CompareEvalRun(c.Request().Context(), evalRun.ID, req)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, report)
}

func (h *Handler) EvaluateSelectorGate(c echo.Context) error {
	evalRun, err := h.scopedEvalRun(c)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "eval run not found"})
	}
	var req SelectorGateRequest
	if err := c.Bind(&req); err != nil && !errors.Is(err, io.EOF) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if req.BaselineID != "" {
		if _, err := h.scopedBaselineByID(c, req.BaselineID); err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "baseline not found"})
		}
	}
	if req.BaseEvalRunID != "" {
		if _, err := h.scopedEvalRunByID(c, req.BaseEvalRunID); err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "selector gate base eval run not found"})
		}
	}
	report, err := h.manager.EvaluateSelectorGate(c.Request().Context(), evalRun.ID, req)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, report)
}

func (h *Handler) EvaluateExecutionEquivalence(c echo.Context) error {
	evalRun, err := h.scopedEvalRun(c)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "eval run not found"})
	}
	var req ExecutionEquivalenceRequest
	if err := c.Bind(&req); err != nil && !errors.Is(err, io.EOF) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if req.BaselineID != "" {
		if _, err := h.scopedBaselineByID(c, req.BaselineID); err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "baseline not found"})
		}
	}
	if req.BaseEvalRunID != "" {
		if _, err := h.scopedEvalRunByID(c, req.BaseEvalRunID); err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "execution gate base eval run not found"})
		}
	}
	report, err := h.manager.EvaluateExecutionEquivalence(c.Request().Context(), evalRun.ID, req)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, report)
}

func (h *Handler) EvaluateSkillCutoverBudgetGate(c echo.Context) error {
	evalRun, err := h.scopedEvalRun(c)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "eval run not found"})
	}
	var req SkillCutoverBudgetRequest
	if err := c.Bind(&req); err != nil && !errors.Is(err, io.EOF) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if req.BaselineID != "" {
		if _, err := h.scopedBaselineByID(c, req.BaselineID); err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "baseline not found"})
		}
	}
	if req.BaseEvalRunID != "" {
		if _, err := h.scopedEvalRunByID(c, req.BaseEvalRunID); err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "budget gate base eval run not found"})
		}
	}
	report, err := h.manager.EvaluateSkillCutoverBudgetGate(c.Request().Context(), evalRun.ID, req)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, report)
}

func (h *Handler) EvaluateSkillCutoverReadiness(c echo.Context) error {
	var req SkillCutoverReadinessRequest
	if err := c.Bind(&req); err != nil && !errors.Is(err, io.EOF) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if userID := harnessUserID(c); userID != "" {
		req.OwnerUserID = userID
	}
	report, err := h.manager.EvaluateSkillCutoverReadiness(c.Request().Context(), req)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, report)
}

func (h *Handler) GetComparisonReport(c echo.Context) error {
	report, err := h.scopedComparisonReport(c)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "comparison report not found"})
	}
	return c.JSON(http.StatusOK, report)
}

func (h *Handler) CreateBaseline(c echo.Context) error {
	var spec BaselineSpec
	if err := c.Bind(&spec); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if userID := harnessUserID(c); userID != "" {
		spec.OwnerUserID = userID
	}
	if spec.EvalRunID != "" {
		if _, err := h.scopedEvalRunByID(c, spec.EvalRunID); err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "eval run not found"})
		}
	}
	baseline, err := h.manager.CreateBaseline(c.Request().Context(), spec)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusCreated, baseline)
}

func (h *Handler) EnsureSelectorCuratedAssets(c echo.Context) error {
	var req struct {
		OwnerUserID string `json:"owner_user_id,omitempty"`
	}
	if err := c.Bind(&req); err != nil && !errors.Is(err, io.EOF) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if userID := harnessUserID(c); userID != "" {
		req.OwnerUserID = userID
	}
	assets, err := h.manager.EnsureSelectorCuratedAssets(c.Request().Context(), req.OwnerUserID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusCreated, assets)
}

func (h *Handler) EnsureBatch1ExecutionAssets(c echo.Context) error {
	var req struct {
		OwnerUserID string `json:"owner_user_id,omitempty"`
	}
	if err := c.Bind(&req); err != nil && !errors.Is(err, io.EOF) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if userID := harnessUserID(c); userID != "" {
		req.OwnerUserID = userID
	}
	assets, err := h.manager.EnsureBatch1ExecutionAssets(c.Request().Context(), req.OwnerUserID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusCreated, assets)
}

func (h *Handler) ListBaselines(c echo.Context) error {
	filter := BaselineFilter{
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
	baselines, err := h.manager.ListBaselines(c.Request().Context(), filter)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, baselines)
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
	if _, err := h.manager.PerformGroupAction(c.Request().Context(), group.ID, "cancel", map[string]interface{}{
		"reason": "cancelled by user",
	}); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "cancelled"})
}

func (h *Handler) PerformGroupAction(c echo.Context) error {
	group, err := h.scopedGroup(c)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "group not found"})
	}
	action := strings.TrimSpace(c.Param("action"))
	if action == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "action is required"})
	}
	var input map[string]interface{}
	if err := c.Bind(&input); err != nil && !errors.Is(err, io.EOF) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	updated, err := h.manager.PerformGroupAction(c.Request().Context(), group.ID, action, input)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, updated)
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

func (h *Handler) PromoteGroup(c echo.Context) error {
	group, err := h.scopedGroup(c)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "group not found"})
	}
	var spec GroupPromotionSpec
	if err := c.Bind(&spec); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if strings.TrimSpace(spec.Subject) == "" {
		spec.Subject = group.Subject
	}
	result, err := h.manager.PromoteGroup(c.Request().Context(), group.ID, spec)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, result)
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
		Actions:   runActionAvailability(run, "/harness/runs/"+run.ID),
		Events:    events,
		Artifacts: artifacts,
	}
	if trace, traceErr := h.manager.RunTraceSnapshot(c.Request().Context(), run.ID); traceErr == nil {
		detail.RunTrace = trace
	} else {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": traceErr.Error()})
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
	if _, err := h.manager.PerformAction(c.Request().Context(), run.ID, "cancel", map[string]interface{}{
		"reason": "cancelled by user",
	}); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "cancelled"})
}

func (h *Handler) PerformRunAction(c echo.Context) error {
	run, err := h.scopedRun(c)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "run not found"})
	}
	action := strings.TrimSpace(c.Param("action"))
	if action == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "action is required"})
	}
	var input map[string]interface{}
	if err := c.Bind(&input); err != nil && !errors.Is(err, io.EOF) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	updated, err := h.manager.PerformAction(c.Request().Context(), run.ID, action, input)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, updated)
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

func (h *Handler) scopedSkillEvolutionCase(c echo.Context) (*SkillEvolutionCase, error) {
	evolutionCase, err := h.manager.GetSkillEvolutionCase(c.Request().Context(), c.Param("id"))
	if err != nil {
		return nil, err
	}
	if userID := harnessUserID(c); userID != "" && evolutionCase.OwnerUserID != "" && evolutionCase.OwnerUserID != userID {
		return nil, echo.ErrNotFound
	}
	return evolutionCase, nil
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
	return h.scopedEvalRunByID(c, c.Param("id"))
}

func (h *Handler) scopedEvalRunByID(c echo.Context, id string) (*EvalRun, error) {
	evalRun, err := h.manager.GetEvalRun(c.Request().Context(), id)
	if err != nil {
		return nil, err
	}
	if userID := harnessUserID(c); userID != "" && evalRun.OwnerUserID != "" && evalRun.OwnerUserID != userID {
		return nil, echo.ErrNotFound
	}
	return evalRun, nil
}

func (h *Handler) scopedBaselineByID(c echo.Context, id string) (*Baseline, error) {
	if h == nil || h.manager == nil || h.manager.store == nil {
		return nil, echo.ErrNotFound
	}
	baseline, err := h.manager.store.GetBaseline(c.Request().Context(), strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	if userID := harnessUserID(c); userID != "" && baseline.OwnerUserID != "" && baseline.OwnerUserID != userID {
		return nil, echo.ErrNotFound
	}
	return baseline, nil
}

func (h *Handler) scopedComparisonReport(c echo.Context) (*ComparisonReport, error) {
	if h == nil || h.manager == nil {
		return nil, echo.ErrNotFound
	}
	report, err := h.manager.GetComparisonReport(c.Request().Context(), c.Param("id"))
	if err != nil {
		return nil, err
	}
	if userID := harnessUserID(c); userID != "" && report.OwnerUserID != "" && report.OwnerUserID != userID {
		return nil, echo.ErrNotFound
	}
	return report, nil
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

func parseSkillRevisionStatuses(values ...[]string) []SkillRevisionStatus {
	raw := flattenQueryValues(values...)
	out := make([]SkillRevisionStatus, 0, len(raw))
	seen := make(map[SkillRevisionStatus]struct{}, len(raw))
	for _, item := range raw {
		status := SkillRevisionStatus(strings.TrimSpace(item))
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

func parseSkillRevisionDecisionActions(values ...[]string) []SkillRevisionDecisionAction {
	raw := flattenQueryValues(values...)
	out := make([]SkillRevisionDecisionAction, 0, len(raw))
	seen := make(map[SkillRevisionDecisionAction]struct{}, len(raw))
	for _, item := range raw {
		action := SkillRevisionDecisionAction(strings.TrimSpace(item))
		if action == "" {
			continue
		}
		if _, ok := seen[action]; ok {
			continue
		}
		seen[action] = struct{}{}
		out = append(out, action)
	}
	return out
}

func parseSkillEvolutionCaseStatuses(values ...[]string) []SkillEvolutionCaseStatus {
	raw := flattenQueryValues(values...)
	out := make([]SkillEvolutionCaseStatus, 0, len(raw))
	seen := make(map[SkillEvolutionCaseStatus]struct{}, len(raw))
	for _, item := range raw {
		status := SkillEvolutionCaseStatus(strings.TrimSpace(item))
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
