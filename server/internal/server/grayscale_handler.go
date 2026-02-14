package server

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
)

// GrayscaleHandler handles grayscale/feature flag API endpoints.
type GrayscaleHandler struct {
	evaluator *config.FlagEvaluator
}

// NewGrayscaleHandler creates a new grayscale handler.
func NewGrayscaleHandler(evaluator *config.FlagEvaluator) *GrayscaleHandler {
	return &GrayscaleHandler{
		evaluator: evaluator,
	}
}

// RegisterRoutes registers grayscale routes.
func (h *GrayscaleHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/grayscale/flags", h.ListFlags)
	g.POST("/grayscale/flags/evaluate", h.EvaluateFlag)
	g.GET("/grayscale/abtests", h.ListABTests)
	g.POST("/grayscale/abtests/evaluate", h.EvaluateABTest)
	g.GET("/grayscale/version", h.GetConfigVersion)
}

// FlagListResponse represents the list of feature flags.
type FlagListResponse struct {
	Flags []FlagInfo `json:"flags"`
	Total int        `json:"total"`
}

// FlagInfo represents a feature flag summary.
type FlagInfo struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Enabled     bool    `json:"enabled"`
	Percentage  float64 `json:"percentage"`
	UserCount   int     `json:"user_count"`
	GroupCount  int     `json:"group_count"`
	RuleCount   int     `json:"rule_count"`
}

// ListFlags handles GET /api/v1/grayscale/flags
// @Summary List all feature flags
// @Description Returns all configured feature flags
// @Tags grayscale
// @Produce json
// @Success 200 {object} FlagListResponse
// @Router /api/v1/grayscale/flags [get]
func (h *GrayscaleHandler) ListFlags(c echo.Context) error {
	if h.evaluator == nil {
		return c.JSON(http.StatusOK, FlagListResponse{
			Flags: []FlagInfo{},
			Total: 0,
		})
	}

	flags := h.evaluator.ListFlags()
	infos := make([]FlagInfo, len(flags))

	for i, f := range flags {
		infos[i] = FlagInfo{
			Name:        f.Name,
			Description: f.Description,
			Enabled:     f.Enabled,
			Percentage:  f.Percentage,
			UserCount:   len(f.Users),
			GroupCount:  len(f.Groups),
			RuleCount:   len(f.Rules),
		}
	}

	return c.JSON(http.StatusOK, FlagListResponse{
		Flags: infos,
		Total: len(infos),
	})
}

// EvaluateFlagRequest represents a flag evaluation request.
type EvaluateFlagRequest struct {
	FlagName   string            `json:"flag_name" validate:"required"`
	UserID     string            `json:"user_id"`
	Groups     []string          `json:"groups"`
	Attributes map[string]string `json:"attributes"`
}

// EvaluateFlagResponse represents a flag evaluation response.
type EvaluateFlagResponse struct {
	FlagName string      `json:"flag_name"`
	Enabled  bool        `json:"enabled"`
	Variant  interface{} `json:"variant,omitempty"`
}

// EvaluateFlag handles POST /api/v1/grayscale/flags/evaluate
// @Summary Evaluate a feature flag
// @Description Evaluates a feature flag for the given context
// @Tags grayscale
// @Accept json
// @Produce json
// @Param request body EvaluateFlagRequest true "Evaluation context"
// @Success 200 {object} EvaluateFlagResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/grayscale/flags/evaluate [post]
func (h *GrayscaleHandler) EvaluateFlag(c echo.Context) error {
	var req EvaluateFlagRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid request body",
		})
	}

	if req.FlagName == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "missing_flag_name",
			Message: "Flag name is required",
		})
	}

	if h.evaluator == nil {
		return c.JSON(http.StatusOK, EvaluateFlagResponse{
			FlagName: req.FlagName,
			Enabled:  false,
		})
	}

	ctx := &config.EvaluationContext{
		UserID:     req.UserID,
		Groups:     req.Groups,
		Attributes: req.Attributes,
	}

	enabled := h.evaluator.IsEnabled(req.FlagName, ctx)
	variant := h.evaluator.GetVariant(req.FlagName, ctx)

	return c.JSON(http.StatusOK, EvaluateFlagResponse{
		FlagName: req.FlagName,
		Enabled:  enabled,
		Variant:  variant,
	})
}

// ABTestListResponse represents the list of A/B tests.
type ABTestListResponse struct {
	Tests []ABTestInfo `json:"tests"`
	Total int          `json:"total"`
}

// ABTestInfo represents an A/B test summary.
type ABTestInfo struct {
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	Enabled      bool    `json:"enabled"`
	TrafficPct   float64 `json:"traffic_pct"`
	VariantCount int     `json:"variant_count"`
	StartTime    string  `json:"start_time,omitempty"`
	EndTime      string  `json:"end_time,omitempty"`
}

// ListABTests handles GET /api/v1/grayscale/abtests
// @Summary List all A/B tests
// @Description Returns all configured A/B tests
// @Tags grayscale
// @Produce json
// @Success 200 {object} ABTestListResponse
// @Router /api/v1/grayscale/abtests [get]
func (h *GrayscaleHandler) ListABTests(c echo.Context) error {
	if h.evaluator == nil {
		return c.JSON(http.StatusOK, ABTestListResponse{
			Tests: []ABTestInfo{},
			Total: 0,
		})
	}

	tests := h.evaluator.ListABTests()
	infos := make([]ABTestInfo, len(tests))

	for i, t := range tests {
		info := ABTestInfo{
			Name:         t.Name,
			Description:  t.Description,
			Enabled:      t.Enabled,
			TrafficPct:   t.TrafficPct,
			VariantCount: len(t.Variants),
		}
		if !t.StartTime.IsZero() {
			info.StartTime = t.StartTime.Format("2006-01-02T15:04:05Z07:00")
		}
		if !t.EndTime.IsZero() {
			info.EndTime = t.EndTime.Format("2006-01-02T15:04:05Z07:00")
		}
		infos[i] = info
	}

	return c.JSON(http.StatusOK, ABTestListResponse{
		Tests: infos,
		Total: len(infos),
	})
}

// EvaluateABTestRequest represents an A/B test evaluation request.
type EvaluateABTestRequest struct {
	TestName string `json:"test_name" validate:"required"`
	UserID   string `json:"user_id"`
}

// EvaluateABTestResponse represents an A/B test evaluation response.
type EvaluateABTestResponse struct {
	TestName    string      `json:"test_name"`
	InTest      bool        `json:"in_test"`
	VariantName string      `json:"variant_name,omitempty"`
	VariantValue interface{} `json:"variant_value,omitempty"`
	IsControl   bool        `json:"is_control,omitempty"`
}

// EvaluateABTest handles POST /api/v1/grayscale/abtests/evaluate
// @Summary Evaluate an A/B test
// @Description Evaluates an A/B test for the given user
// @Tags grayscale
// @Accept json
// @Produce json
// @Param request body EvaluateABTestRequest true "Evaluation context"
// @Success 200 {object} EvaluateABTestResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/grayscale/abtests/evaluate [post]
func (h *GrayscaleHandler) EvaluateABTest(c echo.Context) error {
	var req EvaluateABTestRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid request body",
		})
	}

	if req.TestName == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "missing_test_name",
			Message: "Test name is required",
		})
	}

	if h.evaluator == nil {
		return c.JSON(http.StatusOK, EvaluateABTestResponse{
			TestName: req.TestName,
			InTest:   false,
		})
	}

	ctx := &config.EvaluationContext{
		UserID: req.UserID,
	}

	variant := h.evaluator.GetABTestVariant(req.TestName, ctx)
	if variant == nil {
		return c.JSON(http.StatusOK, EvaluateABTestResponse{
			TestName: req.TestName,
			InTest:   false,
		})
	}

	return c.JSON(http.StatusOK, EvaluateABTestResponse{
		TestName:     req.TestName,
		InTest:       true,
		VariantName:  variant.Name,
		VariantValue: variant.Value,
		IsControl:    variant.IsControl,
	})
}

// ConfigVersionResponse represents the active config version.
type ConfigVersionResponse struct {
	HasVersion  bool                   `json:"has_version"`
	Version     string                 `json:"version,omitempty"`
	Description string                 `json:"description,omitempty"`
	CreatedAt   string                 `json:"created_at,omitempty"`
	Values      map[string]interface{} `json:"values,omitempty"`
}

// GetConfigVersion handles GET /api/v1/grayscale/version
// @Summary Get active config version
// @Description Returns the currently active configuration version
// @Tags grayscale
// @Produce json
// @Success 200 {object} ConfigVersionResponse
// @Router /api/v1/grayscale/version [get]
func (h *GrayscaleHandler) GetConfigVersion(c echo.Context) error {
	if h.evaluator == nil {
		return c.JSON(http.StatusOK, ConfigVersionResponse{
			HasVersion: false,
		})
	}

	version := h.evaluator.GetConfigVersion()
	if version == nil {
		return c.JSON(http.StatusOK, ConfigVersionResponse{
			HasVersion: false,
		})
	}

	response := ConfigVersionResponse{
		HasVersion:  true,
		Version:     version.Version,
		Description: version.Description,
		Values:      version.Values,
	}

	if !version.CreatedAt.IsZero() {
		response.CreatedAt = version.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
	}

	return c.JSON(http.StatusOK, response)
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}
