package providerpool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

type ProviderAccountStatusKind string

const (
	ProviderAccountStatusKindBalance     ProviderAccountStatusKind = "balance"
	ProviderAccountStatusKindCredits     ProviderAccountStatusKind = "credits"
	ProviderAccountStatusKindUnsupported ProviderAccountStatusKind = "unsupported"
)

type ProviderAccountStatusItem struct {
	Key      string  `json:"key"`
	Value    float64 `json:"value"`
	Currency string  `json:"currency,omitempty"`
}

type ProviderAccountStatus struct {
	ProviderID     string                      `json:"provider_id"`
	KeyID          string                      `json:"key_id,omitempty"`
	KeyHash        string                      `json:"key_hash,omitempty"`
	Kind           ProviderAccountStatusKind   `json:"kind"`
	PrimaryItemKey string                      `json:"primary_item_key,omitempty"`
	Items          []ProviderAccountStatusItem `json:"items,omitempty"`
	Error          string                      `json:"error,omitempty"`
	FetchedAt      int64                       `json:"fetched_at"`
}

var newAccountStatusHTTPClient = func() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

func unsupportedProviderAccountStatus(providerID, keyID, keyHash, errCode string) *ProviderAccountStatus {
	return &ProviderAccountStatus{
		ProviderID: providerID,
		KeyID:      keyID,
		KeyHash:    keyHash,
		Kind:       ProviderAccountStatusKindUnsupported,
		Error:      errCode,
		FetchedAt:  timeutil.NowMilli(),
	}
}

func FetchProviderAccountStatus(ctx context.Context, provider *Provider, keyID string) *ProviderAccountStatus {
	if provider == nil {
		return unsupportedProviderAccountStatus("", keyID, "", "provider_not_found")
	}

	apiKey, status := resolveProviderAccountStatusKey(provider, keyID)
	if status != nil {
		return status
	}

	switch provider.ID {
	case "openrouter", "openrouter-free":
		return fetchOpenRouterAccountStatus(ctx, provider, apiKey)
	case "deepseek":
		return fetchDeepSeekAccountStatus(ctx, provider, apiKey)
	default:
		return unsupportedProviderAccountStatus(provider.ID, apiKey.ID, apiKey.KeyHash, "unsupported_provider")
	}
}

func resolveProviderAccountStatusKey(provider *Provider, keyID string) (*APIKey, *ProviderAccountStatus) {
	if provider == nil {
		return nil, unsupportedProviderAccountStatus("", keyID, "", "provider_not_found")
	}

	if keyID != "" {
		for i := range provider.APIKeys {
			if provider.APIKeys[i].ID == keyID {
				if !provider.APIKeys[i].Enabled || strings.TrimSpace(provider.APIKeys[i].Key) == "" {
					return nil, unsupportedProviderAccountStatus(provider.ID, keyID, provider.APIKeys[i].KeyHash, "api_key_disabled")
				}
				return &provider.APIKeys[i], nil
			}
		}
		return nil, unsupportedProviderAccountStatus(provider.ID, keyID, "", "api_key_not_found")
	}

	for i := range provider.APIKeys {
		if provider.APIKeys[i].Enabled && strings.TrimSpace(provider.APIKeys[i].Key) != "" {
			return &provider.APIKeys[i], nil
		}
	}
	for i := range provider.APIKeys {
		if strings.TrimSpace(provider.APIKeys[i].Key) != "" {
			return &provider.APIKeys[i], nil
		}
	}

	return nil, unsupportedProviderAccountStatus(provider.ID, keyID, "", "api_key_not_configured")
}

func fetchOpenRouterAccountStatus(ctx context.Context, provider *Provider, key *APIKey) *ProviderAccountStatus {
	endpoint, err := appendProviderPath(provider.EffectiveBaseURL(), "/v1/key")
	if err != nil {
		return unsupportedProviderAccountStatus(provider.ID, key.ID, key.KeyHash, "invalid_base_url")
	}

	body, err := fetchProviderAccountStatusJSON(ctx, endpoint, key.Key)
	if err != nil {
		return unsupportedProviderAccountStatus(provider.ID, key.ID, key.KeyHash, err.Error())
	}

	var payload map[string]interface{}
	decoder := json.NewDecoder(strings.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return unsupportedProviderAccountStatus(provider.ID, key.ID, key.KeyHash, "invalid_response")
	}

	data := payload
	if raw, ok := payload["data"].(map[string]interface{}); ok {
		data = raw
	}

	remaining, hasRemaining := jsonValueToFloat(data["limit_remaining"])
	used, hasUsed := jsonValueToFloat(data["usage"])
	limit, hasLimit := jsonValueToFloat(data["limit"])

	items := make([]ProviderAccountStatusItem, 0, 3)
	if hasRemaining {
		items = append(items, ProviderAccountStatusItem{
			Key:      "remaining",
			Value:    remaining,
			Currency: "USD",
		})
	}
	if hasUsed {
		items = append(items, ProviderAccountStatusItem{
			Key:      "used",
			Value:    used,
			Currency: "USD",
		})
	}
	if hasLimit {
		items = append(items, ProviderAccountStatusItem{
			Key:      "limit",
			Value:    limit,
			Currency: "USD",
		})
	}

	if len(items) == 0 {
		return unsupportedProviderAccountStatus(provider.ID, key.ID, key.KeyHash, "no_account_status_data")
	}

	return &ProviderAccountStatus{
		ProviderID:     provider.ID,
		KeyID:          key.ID,
		KeyHash:        key.KeyHash,
		Kind:           ProviderAccountStatusKindCredits,
		PrimaryItemKey: "remaining",
		Items:          items,
		FetchedAt:      timeutil.NowMilli(),
	}
}

func fetchDeepSeekAccountStatus(ctx context.Context, provider *Provider, key *APIKey) *ProviderAccountStatus {
	endpoint, err := appendProviderPath(provider.EffectiveBaseURL(), "/user/balance")
	if err != nil {
		return unsupportedProviderAccountStatus(provider.ID, key.ID, key.KeyHash, "invalid_base_url")
	}

	body, err := fetchProviderAccountStatusJSON(ctx, endpoint, key.Key)
	if err != nil {
		return unsupportedProviderAccountStatus(provider.ID, key.ID, key.KeyHash, err.Error())
	}

	var payload struct {
		IsAvailable  bool `json:"is_available"`
		BalanceInfos []struct {
			Currency        string `json:"currency"`
			TotalBalance    string `json:"total_balance"`
			GrantedBalance  string `json:"granted_balance"`
			ToppedUpBalance string `json:"topped_up_balance"`
		} `json:"balance_infos"`
	}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return unsupportedProviderAccountStatus(provider.ID, key.ID, key.KeyHash, "invalid_response")
	}

	if len(payload.BalanceInfos) == 0 {
		return unsupportedProviderAccountStatus(provider.ID, key.ID, key.KeyHash, "no_account_status_data")
	}

	info := payload.BalanceInfos[0]
	totalBalance, err := strconv.ParseFloat(strings.TrimSpace(info.TotalBalance), 64)
	if err != nil {
		return unsupportedProviderAccountStatus(provider.ID, key.ID, key.KeyHash, "invalid_response")
	}
	grantedBalance, _ := strconv.ParseFloat(strings.TrimSpace(info.GrantedBalance), 64)
	toppedUpBalance, _ := strconv.ParseFloat(strings.TrimSpace(info.ToppedUpBalance), 64)

	currency := strings.TrimSpace(info.Currency)
	items := []ProviderAccountStatusItem{
		{
			Key:      "remaining",
			Value:    totalBalance,
			Currency: currency,
		},
		{
			Key:      "granted",
			Value:    grantedBalance,
			Currency: currency,
		},
		{
			Key:      "topped_up",
			Value:    toppedUpBalance,
			Currency: currency,
		},
	}

	return &ProviderAccountStatus{
		ProviderID:     provider.ID,
		KeyID:          key.ID,
		KeyHash:        key.KeyHash,
		Kind:           ProviderAccountStatusKindBalance,
		PrimaryItemKey: "remaining",
		Items:          items,
		FetchedAt:      timeutil.NowMilli(),
	}
}

func fetchProviderAccountStatusJSON(ctx context.Context, endpoint, apiKey string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("request_build_failed")
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "blue/1.0")

	resp, err := newAccountStatusHTTPClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("request_failed")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("response_read_failed")
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("http_%d", resp.StatusCode)
	}
	return string(body), nil
}

func appendProviderPath(rawBaseURL, suffix string) (string, error) {
	baseURL := strings.TrimSpace(rawBaseURL)
	if baseURL == "" {
		return "", fmt.Errorf("base_url_not_configured")
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}

	basePath := strings.TrimSuffix(u.Path, "/")
	suffixPath := "/" + strings.TrimPrefix(strings.TrimSpace(suffix), "/")
	u.Path = basePath + suffixPath
	u.RawPath = ""

	return u.String(), nil
}

func jsonValueToFloat(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case nil:
		return 0, false
	case float64:
		return v, true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return f, err == nil
	default:
		return 0, false
	}
}
