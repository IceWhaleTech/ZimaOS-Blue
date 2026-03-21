package claudecode

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"

	"github.com/labstack/echo/v4"
)

const (
	// Antigravity API endpoints
	antigravityQuotaAPI        = "https://cloudcode-pa.googleapis.com/v1internal:fetchAvailableModels"
	antigravitySubscriptionAPI = "https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist"
)

// ModelQuota represents quota information for a single model.
type ModelQuota struct {
	Name       string  `json:"name"`
	Percentage float64 `json:"percentage"`
	ResetTime  string  `json:"reset_time,omitempty"`
	Group      string  `json:"group"` // "gemini" or "claude"
}

// QuotaData represents the complete quota information.
type QuotaData struct {
	Models           []ModelQuota `json:"models"`
	LastUpdated      int64        `json:"last_updated"`
	SubscriptionTier string       `json:"subscription_tier,omitempty"`
	Error            string       `json:"error,omitempty"`
}

// AntigravityQuotaRequest is the request for fetching quota.
type AntigravityQuotaRequest struct {
	AccessToken string `json:"access_token"`
}

// AntigravityQuotaResponse is the response for quota API.
type AntigravityQuotaResponse struct {
	Success bool      `json:"success"`
	Data    QuotaData `json:"data,omitempty"`
	Error   string    `json:"error,omitempty"`
}

// antigravityModelResponse represents the API response structure.
type antigravityModelResponse struct {
	Models []struct {
		Name             string  `json:"name"`
		RemainingFraction float64 `json:"remainingFraction"`
		ResetTime        string  `json:"resetTime,omitempty"`
	} `json:"models"`
}

// antigravitySubscriptionResponse represents the subscription API response.
type antigravitySubscriptionResponse struct {
	SubscriptionTier string `json:"subscriptionTier"`
	CurrentTier      string `json:"currentTier"`
	PaidTier         string `json:"paidTier"`
}

// AntigravityHandler handles Antigravity quota API endpoints.
type AntigravityHandler struct {
	httpClient *http.Client
	cacheMu    sync.RWMutex
	cache      *QuotaData
	cacheTTL   time.Duration
}

// NewAntigravityHandler creates a new AntigravityHandler.
func NewAntigravityHandler() *AntigravityHandler {
	return &AntigravityHandler{
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
		cacheTTL: 5 * time.Minute,
	}
}

// RegisterRoutes registers the Antigravity API routes.
func (h *AntigravityHandler) RegisterRoutes(g *echo.Group) {
	g.POST("/antigravity/quota", h.GetQuota)
}

// GetQuota fetches the Antigravity quota information.
// POST /api/v1/claudecode/antigravity/quota
func (h *AntigravityHandler) GetQuota(c echo.Context) error {
	var req AntigravityQuotaRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, AntigravityQuotaResponse{
			Success: false,
			Error:   "invalid request body",
		})
	}

	if req.AccessToken == "" {
		return c.JSON(http.StatusBadRequest, AntigravityQuotaResponse{
			Success: false,
			Error:   "access_token is required",
		})
	}

	ctx := c.Request().Context()
	quotaData, err := h.fetchQuota(ctx, req.AccessToken)
	if err != nil {
		return c.JSON(http.StatusOK, AntigravityQuotaResponse{
			Success: false,
			Error:   err.Error(),
		})
	}

	return c.JSON(http.StatusOK, AntigravityQuotaResponse{
		Success: true,
		Data:    *quotaData,
	})
}

// fetchQuota fetches quota data from Antigravity API.
func (h *AntigravityHandler) fetchQuota(ctx context.Context, accessToken string) (*QuotaData, error) {
	// Fetch subscription tier and quota in parallel
	var wg sync.WaitGroup
	var subscriptionTier string
	var models []ModelQuota
	var quotaErr, subErr error

	wg.Add(2)

	// Fetch subscription tier
	go func() {
		defer wg.Done()
		subscriptionTier, subErr = h.fetchSubscriptionTier(ctx, accessToken)
	}()

	// Fetch quota
	go func() {
		defer wg.Done()
		models, quotaErr = h.fetchModelsQuota(ctx, accessToken)
	}()

	wg.Wait()

	// Handle errors
	if quotaErr != nil {
		return nil, fmt.Errorf("failed to fetch quota: %w", quotaErr)
	}

	// Subscription error is not fatal
	if subErr != nil {
		subscriptionTier = "unknown"
	}

	return &QuotaData{
		Models:           models,
		LastUpdated:      timeutil.Now(),
		SubscriptionTier: subscriptionTier,
	}, nil
}

// fetchSubscriptionTier fetches the subscription tier from Antigravity API.
func (h *AntigravityHandler) fetchSubscriptionTier(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", antigravitySubscriptionAPI, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("subscription API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var subResp antigravitySubscriptionResponse
	if err := json.Unmarshal(body, &subResp); err != nil {
		return "", err
	}

	// Prioritize paid tier over current tier (for account rotation priority)
	if subResp.PaidTier != "" {
		return subResp.PaidTier, nil
	}
	if subResp.CurrentTier != "" {
		return subResp.CurrentTier, nil
	}
	return subResp.SubscriptionTier, nil
}

// fetchModelsQuota fetches the models quota from Antigravity API.
func (h *AntigravityHandler) fetchModelsQuota(ctx context.Context, accessToken string) ([]ModelQuota, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", antigravityQuotaAPI, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("quota API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var modelResp antigravityModelResponse
	if err := json.Unmarshal(body, &modelResp); err != nil {
		return nil, err
	}

	// Filter and convert models
	var models []ModelQuota
	for _, m := range modelResp.Models {
		nameLower := strings.ToLower(m.Name)

		// Only include Gemini and Claude models
		var group string
		if strings.Contains(nameLower, "gemini") {
			group = "gemini"
		} else if strings.Contains(nameLower, "claude") {
			group = "claude"
		} else {
			continue
		}

		// Convert fraction to percentage
		percentage := m.RemainingFraction * 100
		if percentage < 0 {
			percentage = 0
		}
		if percentage > 100 {
			percentage = 100
		}

		models = append(models, ModelQuota{
			Name:       m.Name,
			Percentage: percentage,
			ResetTime:  m.ResetTime,
			Group:      group,
		})
	}

	// Sort models alphabetically
	sort.Slice(models, func(i, j int) bool {
		return models[i].Name < models[j].Name
	})

	return models, nil
}
