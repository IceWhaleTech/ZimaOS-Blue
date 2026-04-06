package preview

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/user"
	"github.com/labstack/echo/v4"
)

// Handler handles preview mode HTTP requests.
type Handler struct {
	modeService      *ModeService
	upgradeService   *UpgradeService
	questionsService *QuestionsService
	jwtService       *auth.JWTService
	userService      *user.Service
	dataDir          string
}

// NewHandler creates a new preview Handler.
func NewHandler(modeService *ModeService, upgradeService *UpgradeService, jwtService *auth.JWTService, userService *user.Service) *Handler {
	return &Handler{
		modeService:      modeService,
		upgradeService:   upgradeService,
		questionsService: NewQuestionsService(),
		jwtService:       jwtService,
		userService:      userService,
		dataDir:          "./data",
	}
}

// SetDataDir sets the data directory for storing onboarding state.
func (h *Handler) SetDataDir(dataDir string) {
	h.dataDir = dataDir
}

// RegisterRoutes registers preview mode routes.
func (h *Handler) RegisterRoutes(e *echo.Echo) {
	g := e.Group("/api/v1")

	// Public endpoints (no auth required)
	g.GET("/system/mode", h.GetSystemMode)
	g.GET("/preview/status", h.GetPreviewStatus)
	g.POST("/preview/upgrade", h.Upgrade)
	g.POST("/preview/token", h.GetPreviewToken)
	g.GET("/preset-questions", h.GetPresetQuestions)
	g.GET("/preview/onboarding", h.GetOnboardingStatus)
	g.POST("/preview/onboarding", h.SetOnboardingSeen)
}

// GetSystemMode returns the current system mode.
// GET /api/v1/system/mode
func (h *Handler) GetSystemMode(c echo.Context) error {
	mode, err := h.modeService.GetSystemMode(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get system mode")
	}
	return c.JSON(http.StatusOK, mode)
}

// PreviewStatusResponse represents the preview status response.
type PreviewStatusResponse struct {
	Active  bool   `json:"active"`
	Message string `json:"message"`
}

// GetPreviewStatus returns the preview mode status.
// GET /api/v1/preview/status
func (h *Handler) GetPreviewStatus(c echo.Context) error {
	isPreview, err := h.modeService.IsPreviewMode(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get preview status")
	}

	resp := PreviewStatusResponse{
		Active: isPreview,
	}

	if isPreview {
		resp.Message = "预览模式 - 创建账户后数据将自动保留"
	} else {
		resp.Message = "正常模式"
	}

	return c.JSON(http.StatusOK, resp)
}

// Upgrade handles the upgrade from preview mode to normal mode.
// POST /api/v1/preview/upgrade
func (h *Handler) Upgrade(c echo.Context) error {
	var req UpgradeRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	resp, err := h.upgradeService.Upgrade(c.Request().Context(), &req)
	if err != nil {
		switch err {
		case ErrUsernameRequired:
			return echo.NewHTTPError(http.StatusBadRequest, "username is required")
		case ErrPasswordRequired:
			return echo.NewHTTPError(http.StatusBadRequest, "password is required")
		case ErrUsersAlreadyExist:
			return echo.NewHTTPError(http.StatusConflict, "preview mode is only available before the first user is created")
		case ErrAdminAlreadyExists:
			return echo.NewHTTPError(http.StatusConflict, "admin already exists")
		default:
			// Check for user service errors
			if err.Error() == "username already exists" {
				return echo.NewHTTPError(http.StatusConflict, "username already exists")
			}
			if err.Error() == "password does not meet requirements" {
				return echo.NewHTTPError(http.StatusBadRequest, "password does not meet requirements")
			}
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to create admin")
		}
	}

	return c.JSON(http.StatusOK, resp)
}

// PresetQuestionsResponse represents the preset questions response.
type PresetQuestionsResponse struct {
	Questions  []PresetQuestion `json:"questions"`
	Total      int              `json:"total"`
	NextOffset int              `json:"next_offset"`
	HasMore    bool             `json:"has_more"`
}

// GetPresetQuestions returns a random selection of preset questions.
// GET /api/v1/preset-questions?count=4&offset=0&lang=en
func (h *Handler) GetPresetQuestions(c echo.Context) error {
	countStr := c.QueryParam("count")
	count := 4 // default
	if countStr != "" {
		if n, err := strconv.Atoi(countStr); err == nil && n > 0 {
			count = n
		}
	}
	offsetStr := c.QueryParam("offset")
	offset := 0
	if offsetStr != "" {
		if n, err := strconv.Atoi(offsetStr); err == nil && n >= 0 {
			offset = n
		}
	}

	// Get language from query param, default to "en"
	lang := c.QueryParam("lang")
	if lang == "" {
		// Try to get from Accept-Language header
		acceptLang := c.Request().Header.Get("Accept-Language")
		if len(acceptLang) >= 2 {
			lang = acceptLang[:2]
		}
	}
	// Normalize language code (support zh, en, ja, ko; others fallback to en)
	switch {
	case lang == "zh" || strings.HasPrefix(lang, "zh"):
		lang = "zh"
	case lang == "ja" || strings.HasPrefix(lang, "ja"):
		lang = "ja"
	case lang == "ko" || strings.HasPrefix(lang, "ko"):
		lang = "ko"
	default:
		lang = "en"
	}

	questions, total := h.questionsService.GetPresetQuestionPage(offset, count, lang)
	nextOffset := offset + len(questions)
	return c.JSON(http.StatusOK, PresetQuestionsResponse{
		Questions:  questions,
		Total:      total,
		NextOffset: nextOffset,
		HasMore:    nextOffset < total,
	})
}

// PreviewTokenResponse represents the preview token response.
type PreviewTokenResponse struct {
	Token     string `json:"token"`
	ExpiresIn int    `json:"expires_in"`
}

// GetPreviewToken generates a temporary token for preview mode.
// POST /api/v1/preview/token
func (h *Handler) GetPreviewToken(c echo.Context) error {
	// Only allow in preview mode
	isPreview, err := h.modeService.IsPreviewMode(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to check preview mode")
	}

	if !isPreview {
		return echo.NewHTTPError(http.StatusForbidden, "preview token only available in preview mode")
	}

	// Security check: ensure no real users exist
	if h.userService != nil {
		userExists, err := h.userService.AnyUserExists(c.Request().Context())
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to check user status")
		}
		if userExists {
			return echo.NewHTTPError(http.StatusForbidden, "preview token not available after a user has been created")
		}
	}

	// Generate a temporary token for the preview user
	userClaims := &auth.UserClaims{
		UserID:   "preview-user",
		Username: "preview",
		Role:     "user", // Limited role for preview mode
	}

	token, err := h.jwtService.GenerateAccessToken(userClaims)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate token")
	}

	return c.JSON(http.StatusOK, PreviewTokenResponse{
		Token:     token,
		ExpiresIn: 86400, // 24 hours
	})
}

// OnboardingStatusResponse represents the onboarding status response.
type OnboardingStatusResponse struct {
	Seen bool `json:"seen"`
}

// GetOnboardingStatus returns whether the user has seen the onboarding modal.
// GET /api/v1/preview/onboarding
func (h *Handler) GetOnboardingStatus(c echo.Context) error {
	// If dataDir is not set, return not seen
	if h.dataDir == "" {
		return c.JSON(http.StatusOK, OnboardingStatusResponse{
			Seen: false,
		})
	}

	// Check if the marker file exists
	markerPath := filepath.Join(h.dataDir, ".onboarding_seen")
	_, err := os.Stat(markerPath)
	seen := err == nil

	return c.JSON(http.StatusOK, OnboardingStatusResponse{
		Seen: seen,
	})
}

// SetOnboardingSeen marks the onboarding as seen.
// POST /api/v1/preview/onboarding
func (h *Handler) SetOnboardingSeen(c echo.Context) error {
	// If dataDir is not set, just return success (for echolib/Tauri)
	if h.dataDir == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success": true,
		})
	}

	// Create the marker file
	markerPath := filepath.Join(h.dataDir, ".onboarding_seen")

	// Ensure data directory exists
	if err := os.MkdirAll(h.dataDir, 0750); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create data directory")
	}

	// Create the marker file
	file, err := os.Create(markerPath)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to save onboarding status")
	}
	file.Close()

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
	})
}
