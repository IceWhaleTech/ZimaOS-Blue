package api

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/adapters"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providers"
)

// ToolsHandler handles tool calling API requests.
type ToolsHandler struct {
	router             *adapters.CapabilityRouter
	capabilityDetector *providers.CapabilityDetector
}

// NewToolsHandler creates a new tools handler.
func NewToolsHandler(router *adapters.CapabilityRouter) *ToolsHandler {
	return &ToolsHandler{
		router:             router,
		capabilityDetector: providers.NewCapabilityDetector(),
	}
}

// RegisterRoutes registers tool calling routes.
func (h *ToolsHandler) RegisterRoutes(e *echo.Echo) {
	g := e.Group("/api/v1/tools")
	g.GET("/compatibility", h.GetCompatibility)
	g.POST("/test", h.TestToolCalling)

	// Provider capabilities
	p := e.Group("/api/v1/providers")
	p.GET("/:name/capabilities", h.GetProviderCapabilities)
	p.GET("/capabilities", h.GetAllCapabilities)
}

// CompatibilityResponse represents the tool compatibility response.
type CompatibilityResponse struct {
	Providers map[string]ProviderCompatibility `json:"providers"`
}

// ProviderCompatibility represents a provider's tool calling compatibility.
type ProviderCompatibility struct {
	Name           string `json:"name"`
	ToolCalling    string `json:"tool_calling"` // native, partial, none
	AdapterType    string `json:"adapter_type"` // native, ccnexus, cliproxy
	Streaming      bool   `json:"streaming"`
	Vision         bool   `json:"vision"`
	MaxTokens      int    `json:"max_tokens"`
	NeedsAdapter   bool   `json:"needs_adapter"`
}

// GetCompatibility returns tool calling compatibility for all providers.
func (h *ToolsHandler) GetCompatibility(c echo.Context) error {
	matrix := h.capabilityDetector.GetCapabilityMatrix()

	result := make(map[string]ProviderCompatibility)
	for name, cap := range matrix {
		adapterType := "native"
		needsAdapter := false

		switch cap.ToolCalling {
		case providers.ToolCallingPartial:
			adapterType = "ccnexus"
			needsAdapter = true
		case providers.ToolCallingNone:
			adapterType = "cliproxy"
			needsAdapter = true
		}

		result[name] = ProviderCompatibility{
			Name:         name,
			ToolCalling:  string(cap.ToolCalling),
			AdapterType:  adapterType,
			Streaming:    cap.Streaming,
			Vision:       cap.Vision,
			MaxTokens:    cap.MaxTokens,
			NeedsAdapter: needsAdapter,
		}
	}

	return c.JSON(http.StatusOK, &CompatibilityResponse{
		Providers: result,
	})
}

// GetProviderCapabilities returns capabilities for a specific provider.
func (h *ToolsHandler) GetProviderCapabilities(c echo.Context) error {
	name := c.Param("name")

	cap, found := h.capabilityDetector.GetCapability(name)
	if !found {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error": "Provider not found",
		})
	}

	adapterType := "native"
	needsAdapter := false

	switch cap.ToolCalling {
	case providers.ToolCallingPartial:
		adapterType = "ccnexus"
		needsAdapter = true
	case providers.ToolCallingNone:
		adapterType = "cliproxy"
		needsAdapter = true
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"name":            name,
		"tool_calling":    string(cap.ToolCalling),
		"adapter_type":    adapterType,
		"streaming":       cap.Streaming,
		"vision":          cap.Vision,
		"max_tokens":      cap.MaxTokens,
		"supported_models": cap.SupportedModels,
		"needs_adapter":   needsAdapter,
	})
}

// GetAllCapabilities returns capabilities for all known providers.
func (h *ToolsHandler) GetAllCapabilities(c echo.Context) error {
	matrix := h.capabilityDetector.GetCapabilityMatrix()

	result := make(map[string]interface{})
	for name, cap := range matrix {
		result[name] = map[string]interface{}{
			"tool_calling":     string(cap.ToolCalling),
			"streaming":        cap.Streaming,
			"vision":           cap.Vision,
			"max_tokens":       cap.MaxTokens,
			"supported_models": cap.SupportedModels,
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"providers": result,
	})
}

// TestToolCallingRequest represents the test tool calling request.
type TestToolCallingRequest struct {
	Provider string `json:"provider"`
	Model    string `json:"model,omitempty"`
}

// TestToolCallingResponse represents the test tool calling response.
type TestToolCallingResponse struct {
	Success      bool   `json:"success"`
	Provider     string `json:"provider"`
	Model        string `json:"model,omitempty"`
	ToolCalling  string `json:"tool_calling"`
	AdapterUsed  string `json:"adapter_used"`
	TestResult   string `json:"test_result"`
	ErrorMessage string `json:"error_message,omitempty"`
	LatencyMs    int64  `json:"latency_ms"`
}

// TestToolCalling tests tool calling with a specific provider.
func (h *ToolsHandler) TestToolCalling(c echo.Context) error {
	var req TestToolCallingRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	if req.Provider == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Provider is required",
		})
	}

	// Get capability
	cap, found := h.capabilityDetector.GetCapability(req.Provider)
	if !found {
		return c.JSON(http.StatusNotFound, &TestToolCallingResponse{
			Success:      false,
			Provider:     req.Provider,
			ErrorMessage: "Provider not found",
		})
	}

	// Determine adapter type
	adapterType := "native"
	switch cap.ToolCalling {
	case providers.ToolCallingPartial:
		adapterType = "ccnexus"
	case providers.ToolCallingNone:
		adapterType = "cliproxy"
	}

	// Test tool calling
	ctx, cancel := context.WithTimeout(c.Request().Context(), 30*time.Second)
	defer cancel()

	start := time.Now()
	testResult, err := h.testToolCallingWithProvider(ctx, req.Provider, req.Model)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return c.JSON(http.StatusOK, &TestToolCallingResponse{
			Success:      false,
			Provider:     req.Provider,
			Model:        req.Model,
			ToolCalling:  string(cap.ToolCalling),
			AdapterUsed:  adapterType,
			TestResult:   "failed",
			ErrorMessage: err.Error(),
			LatencyMs:    latency,
		})
	}

	return c.JSON(http.StatusOK, &TestToolCallingResponse{
		Success:     true,
		Provider:    req.Provider,
		Model:       req.Model,
		ToolCalling: string(cap.ToolCalling),
		AdapterUsed: adapterType,
		TestResult:  testResult,
		LatencyMs:   latency,
	})
}

// testToolCallingWithProvider tests tool calling with a specific provider.
func (h *ToolsHandler) testToolCallingWithProvider(ctx context.Context, provider, model string) (string, error) {
	// This is a simplified test - in production, this would actually call the provider
	// with a test tool and verify the response

	cap, found := h.capabilityDetector.GetCapability(provider)
	if !found {
		return "", nil
	}

	switch cap.ToolCalling {
	case providers.ToolCallingNative:
		return "native_supported", nil
	case providers.ToolCallingPartial:
		return "partial_with_adapter", nil
	case providers.ToolCallingNone:
		return "requires_cliproxy", nil
	default:
		return "unknown", nil
	}
}

// ConvertToolsRequest represents the convert tools request.
type ConvertToolsRequest struct {
	Provider string          `json:"provider"`
	Tools    []adapters.Tool `json:"tools"`
}

// ConvertTools converts tools to the appropriate format for a provider.
func (h *ToolsHandler) ConvertTools(c echo.Context) error {
	var req ConvertToolsRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	if req.Provider == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Provider is required",
		})
	}

	if len(req.Tools) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "At least one tool is required",
		})
	}

	converted, err := h.router.ConvertToolsForProvider(req.Provider, req.Tools)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":   true,
		"provider":  req.Provider,
		"converted": converted,
	})
}
