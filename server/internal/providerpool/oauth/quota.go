package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
)

// OAuthQuotaInfo is the unified response for OAuth provider quota/tier queries.
type OAuthQuotaInfo struct {
	ProviderType string           `json:"provider_type"`
	Tier         string           `json:"tier"`
	TierName     string           `json:"tier_name"`
	ModelQuotas  []ModelQuotaInfo `json:"model_quotas,omitempty"`
	Error        string           `json:"error,omitempty"`
	FetchedAt    int64            `json:"fetched_at"`
}

// ModelQuotaInfo represents per-model remaining quota.
type ModelQuotaInfo struct {
	Model            string `json:"model"`
	RemainingPercent int    `json:"remaining_percent"`
	ResetTime        string `json:"reset_time,omitempty"`
}

const cloudCodeBaseURL = "https://cloudcode-pa.googleapis.com"

// --- Cloud Code (Antigravity / Gemini CLI) ---

type loadProjectResponse struct {
	CloudAICompanionProject string `json:"cloudaicompanionProject"`
	CurrentTier             *tier  `json:"currentTier"`
	PaidTier                *tier  `json:"paidTier"`
}

type tier struct {
	ID        string `json:"id"`
	QuotaTier string `json:"quotaTier"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
}

type fetchModelsResponse struct {
	Models map[string]modelInfo `json:"models"`
}

type modelInfo struct {
	QuotaInfo *quotaInfo `json:"quotaInfo"`
}

type quotaInfo struct {
	RemainingFraction *float64 `json:"remainingFraction"`
	ResetTime         string   `json:"resetTime"`
}

// FetchCloudCodeQuota fetches subscription tier and per-model quota from Google Cloud Code API.
func FetchCloudCodeQuota(ctx context.Context, accessToken, projectID, providerType string) (*OAuthQuotaInfo, error) {
	info := &OAuthQuotaInfo{
		ProviderType: providerType,
		FetchedAt:    time.Now().UnixMilli(),
	}

	// Determine ideType for the tier request
	ideType := "ANTIGRAVITY"
	if providerType == "gemini-cli" {
		ideType = "GEMINI_CLI"
	}

	// Fetch tier
	tierResp, err := fetchCloudCodeTier(ctx, accessToken, ideType)
	if err == nil && tierResp != nil {
		// Priority: paidTier > currentTier
		if tierResp.PaidTier != nil && tierResp.PaidTier.ID != "" {
			info.Tier = tierResp.PaidTier.ID
			info.TierName = tierResp.PaidTier.Name
		} else if tierResp.CurrentTier != nil && tierResp.CurrentTier.ID != "" {
			info.Tier = tierResp.CurrentTier.ID
			info.TierName = tierResp.CurrentTier.Name
		}
	}

	// Use cloudaicompanionProject from tier response as fallback project ID
	effectiveProjectID := projectID
	if effectiveProjectID == "" && tierResp != nil && tierResp.CloudAICompanionProject != "" {
		effectiveProjectID = tierResp.CloudAICompanionProject
	}

	// Fetch per-model quotas if we have a project ID
	if effectiveProjectID != "" {
		quotas, qErr := fetchCloudCodeModelQuotas(ctx, accessToken, effectiveProjectID)
		if qErr == nil {
			info.ModelQuotas = quotas
		}
	}

	// If we got nothing at all, report error
	if info.Tier == "" && len(info.ModelQuotas) == 0 {
		if err != nil {
			info.Error = "fetch_failed"
		}
	}

	return info, nil
}

func fetchCloudCodeTier(ctx context.Context, accessToken, ideType string) (*loadProjectResponse, error) {
	body := fmt.Sprintf(`{"metadata":{"ideType":"%s"}}`, ideType)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		cloudCodeBaseURL+"/v1internal:loadCodeAssist", strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "blue/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("loadCodeAssist failed (%d): %s", resp.StatusCode, respBody)
	}

	var result loadProjectResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func fetchCloudCodeModelQuotas(ctx context.Context, accessToken, projectID string) ([]ModelQuotaInfo, error) {
	body := fmt.Sprintf(`{"project":"%s"}`, projectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		cloudCodeBaseURL+"/v1internal:fetchAvailableModels", strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "blue/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetchAvailableModels failed (%d): %s", resp.StatusCode, respBody)
	}

	var result fetchModelsResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	var quotas []ModelQuotaInfo
	for name, info := range result.Models {
		if info.QuotaInfo == nil || info.QuotaInfo.RemainingFraction == nil {
			continue
		}
		// Only include gemini and claude models
		if !strings.Contains(name, "gemini") && !strings.Contains(name, "claude") {
			continue
		}
		quotas = append(quotas, ModelQuotaInfo{
			Model:            name,
			RemainingPercent: int(math.Round(*info.QuotaInfo.RemainingFraction * 100)),
			ResetTime:        info.QuotaInfo.ResetTime,
		})
	}
	sort.Slice(quotas, func(i, j int) bool { return quotas[i].Model < quotas[j].Model })
	return quotas, nil
}

// --- GitHub Copilot ---

// ParseCopilotSKU extracts the sku field from a Copilot token string.
// Token format: "tid=...;exp=...;sku=copilot_individual;..."
func ParseCopilotSKU(tokenStr string) string {
	for _, part := range strings.Split(tokenStr, ";") {
		if strings.HasPrefix(part, "sku=") {
			return strings.TrimPrefix(part, "sku=")
		}
	}
	return ""
}

// CopilotSKUToTier maps a Copilot SKU string to a display tier and name.
// Known SKUs: copilot_for_individual, copilot_for_business_seat, copilot_enterprise,
// free_educational, copilot_free, copilot_for_individual_pro_plus
func CopilotSKUToTier(sku string) (string, string) {
	lower := strings.ToLower(sku)
	switch {
	case sku == "":
		return "Free", "GitHub Copilot Free"
	case strings.Contains(lower, "enterprise"):
		return "Enterprise", "GitHub Copilot Enterprise"
	case strings.Contains(lower, "business"):
		return "Business", "GitHub Copilot Business"
	case strings.Contains(lower, "pro_plus"):
		return "Pro+", "GitHub Copilot Pro+"
	case strings.Contains(lower, "individual") || lower == "copilot_pro":
		return "Pro", "GitHub Copilot Pro"
	case strings.Contains(lower, "free") || strings.Contains(lower, "educational"):
		return "Free", "GitHub Copilot Free"
	default:
		return sku, "GitHub Copilot " + sku
	}
}
