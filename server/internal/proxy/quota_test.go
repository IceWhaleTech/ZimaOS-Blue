package proxy

import (
	"net/http"
	"testing"
	"time"
)

func TestNewQuotaMonitor(t *testing.T) {
	// Test with nil config (should use defaults)
	qm := NewQuotaMonitor(nil)
	if qm == nil {
		t.Fatal("NewQuotaMonitor(nil) returned nil")
	}
	if !qm.config.Enabled {
		t.Error("Default config should have Enabled=true")
	}
	if qm.config.WarningThreshold != 20.0 {
		t.Errorf("Default warning threshold should be 20.0, got %f", qm.config.WarningThreshold)
	}
	if qm.config.CriticalThreshold != 5.0 {
		t.Errorf("Default critical threshold should be 5.0, got %f", qm.config.CriticalThreshold)
	}
}

func TestQuotaMonitor_RecordRequest(t *testing.T) {
	qm := NewQuotaMonitor(nil)

	qm.RecordRequest("anthropic")
	qm.RecordRequest("anthropic")
	qm.RecordRequest("openai")

	quota := qm.GetQuota("anthropic")
	if quota == nil {
		t.Fatal("GetQuota returned nil for recorded provider")
	}
	if quota.UsedQuota != 2 {
		t.Errorf("Expected UsedQuota=2, got %d", quota.UsedQuota)
	}

	quota = qm.GetQuota("openai")
	if quota.UsedQuota != 1 {
		t.Errorf("Expected UsedQuota=1, got %d", quota.UsedQuota)
	}
}

func TestQuotaMonitor_RecordError(t *testing.T) {
	qm := NewQuotaMonitor(nil)

	// Test 429 rate limit
	qm.RecordError("anthropic", 429)
	quota := qm.GetQuota("anthropic")
	if !quota.RateLimited {
		t.Error("Expected RateLimited=true for 429")
	}
	if quota.Status != "rate_limited" {
		t.Errorf("Expected status 'rate_limited', got '%s'", quota.Status)
	}

	// Test 403 ban
	qm.RecordError("openai", 403)
	quota = qm.GetQuota("openai")
	if !quota.BanDetected {
		t.Error("Expected BanDetected=true for 403")
	}
	if quota.Status != "banned" {
		t.Errorf("Expected status 'banned', got '%s'", quota.Status)
	}
}

func TestQuotaMonitor_ClearRateLimit(t *testing.T) {
	qm := NewQuotaMonitor(nil)

	qm.RecordError("anthropic", 429)
	qm.ClearRateLimit("anthropic")

	quota := qm.GetQuota("anthropic")
	if quota.RateLimited {
		t.Error("Expected RateLimited=false after clear")
	}
	if quota.Status == "rate_limited" {
		t.Error("Status should not be 'rate_limited' after clear")
	}
}

func TestQuotaMonitor_ClearBan(t *testing.T) {
	qm := NewQuotaMonitor(nil)

	qm.RecordError("anthropic", 403)
	qm.ClearBan("anthropic")

	quota := qm.GetQuota("anthropic")
	if quota.BanDetected {
		t.Error("Expected BanDetected=false after clear")
	}
	if quota.Status == "banned" {
		t.Error("Status should not be 'banned' after clear")
	}
}

func TestQuotaMonitor_ParseRateLimitHeaders(t *testing.T) {
	qm := NewQuotaMonitor(nil)

	// Create mock response with rate limit headers
	resp := &http.Response{
		StatusCode: 200,
		Header:     make(http.Header),
	}
	resp.Header.Set("X-RateLimit-Limit", "1000")
	resp.Header.Set("X-RateLimit-Remaining", "800")
	resp.Header.Set("X-RateLimit-Reset", "1706600000")

	qm.RecordUsage("anthropic", resp, 0)

	quota := qm.GetQuota("anthropic")
	if quota.TotalQuota != 1000 {
		t.Errorf("Expected TotalQuota=1000, got %d", quota.TotalQuota)
	}
	if quota.RemainingPct != 80.0 {
		t.Errorf("Expected RemainingPct=80.0, got %f", quota.RemainingPct)
	}
	if quota.ResetTime.Unix() != 1706600000 {
		t.Errorf("Expected ResetTime=1706600000, got %d", quota.ResetTime.Unix())
	}
}

func TestQuotaMonitor_ParseAnthropicHeaders(t *testing.T) {
	qm := NewQuotaMonitor(nil)

	resp := &http.Response{
		StatusCode: 200,
		Header:     make(http.Header),
	}
	resp.Header.Set("anthropic-ratelimit-requests-limit", "500")
	resp.Header.Set("anthropic-ratelimit-requests-remaining", "450")

	qm.RecordUsage("anthropic", resp, 0)

	quota := qm.GetQuota("anthropic")
	if quota.TotalQuota != 500 {
		t.Errorf("Expected TotalQuota=500, got %d", quota.TotalQuota)
	}
	if quota.RemainingPct != 90.0 {
		t.Errorf("Expected RemainingPct=90.0, got %f", quota.RemainingPct)
	}
}

func TestQuotaMonitor_ParseOpenAIHeaders(t *testing.T) {
	qm := NewQuotaMonitor(nil)

	resp := &http.Response{
		StatusCode: 200,
		Header:     make(http.Header),
	}
	resp.Header.Set("x-ratelimit-limit-requests", "10000")
	resp.Header.Set("x-ratelimit-remaining-requests", "9500")

	qm.RecordUsage("openai", resp, 0)

	quota := qm.GetQuota("openai")
	if quota.TotalQuota != 10000 {
		t.Errorf("Expected TotalQuota=10000, got %d", quota.TotalQuota)
	}
	if quota.RemainingPct != 95.0 {
		t.Errorf("Expected RemainingPct=95.0, got %f", quota.RemainingPct)
	}
}

func TestQuotaMonitor_UpdateStatus(t *testing.T) {
	config := &QuotaMonitorConfig{
		Enabled:           true,
		WarningThreshold:  20.0,
		CriticalThreshold: 5.0,
	}
	qm := NewQuotaMonitor(config)

	tests := []struct {
		remainingPct float64
		rateLimited  bool
		banDetected  bool
		expected     string
	}{
		{80.0, false, false, "healthy"},
		{15.0, false, false, "warning"},
		{3.0, false, false, "critical"},
		{50.0, true, false, "rate_limited"},
		{50.0, false, true, "banned"},
		{3.0, false, true, "banned"}, // Ban takes precedence
	}

	for i, tt := range tests {
		// Reset quotas
		qm.Reset()

		// Set up quota manually
		qm.mu.Lock()
		qm.quotas["test"] = &QuotaInfo{
			Provider:     "test",
			TotalQuota:   100,
			RemainingPct: tt.remainingPct,
			RateLimited:  tt.rateLimited,
			BanDetected:  tt.banDetected,
		}
		qm.updateStatus(qm.quotas["test"])
		qm.mu.Unlock()

		quota := qm.GetQuota("test")
		if quota.Status != tt.expected {
			t.Errorf("Test %d: expected status '%s', got '%s'", i, tt.expected, quota.Status)
		}
	}
}

func TestQuotaMonitor_GetQuotaSummary(t *testing.T) {
	qm := NewQuotaMonitor(nil)

	// Set up some quotas
	qm.mu.Lock()
	qm.quotas["anthropic"] = &QuotaInfo{
		Provider:     "anthropic",
		RemainingPct: 80.0,
		Status:       "healthy",
	}
	qm.quotas["openai"] = &QuotaInfo{
		Provider:     "openai",
		RemainingPct: 15.0,
		Status:       "warning",
	}
	qm.quotas["gemini"] = &QuotaInfo{
		Provider:     "gemini",
		RemainingPct: 3.0,
		Status:       "critical",
	}
	qm.mu.Unlock()

	summary := qm.GetQuotaSummary()

	if summary.TotalProviders != 3 {
		t.Errorf("Expected TotalProviders=3, got %d", summary.TotalProviders)
	}
	if summary.HealthyCount != 1 {
		t.Errorf("Expected HealthyCount=1, got %d", summary.HealthyCount)
	}
	if summary.WarningCount != 1 {
		t.Errorf("Expected WarningCount=1, got %d", summary.WarningCount)
	}
	if summary.CriticalCount != 1 {
		t.Errorf("Expected CriticalCount=1, got %d", summary.CriticalCount)
	}

	expectedAvg := (80.0 + 15.0 + 3.0) / 3.0
	if summary.AverageRemaining != expectedAvg {
		t.Errorf("Expected AverageRemaining=%f, got %f", expectedAvg, summary.AverageRemaining)
	}

	if summary.BestProvider != "anthropic" {
		t.Errorf("Expected BestProvider='anthropic', got '%s'", summary.BestProvider)
	}
}

func TestQuotaMonitor_GetBestProvider(t *testing.T) {
	qm := NewQuotaMonitor(nil)

	// Set up quotas with different remaining percentages
	qm.mu.Lock()
	qm.quotas["provider1"] = &QuotaInfo{
		Provider:     "provider1",
		RemainingPct: 50.0,
		Status:       "healthy",
	}
	qm.quotas["provider2"] = &QuotaInfo{
		Provider:     "provider2",
		RemainingPct: 80.0,
		Status:       "healthy",
	}
	qm.quotas["provider3"] = &QuotaInfo{
		Provider:     "provider3",
		RemainingPct: 90.0,
		Status:       "banned", // Should be excluded
	}
	qm.mu.Unlock()

	best := qm.GetBestProvider()
	if best != "provider2" {
		t.Errorf("Expected best provider 'provider2', got '%s'", best)
	}
}

func TestQuotaMonitor_IsProviderAvailable(t *testing.T) {
	qm := NewQuotaMonitor(nil)

	// Unknown provider should be available
	if !qm.IsProviderAvailable("unknown") {
		t.Error("Unknown provider should be available")
	}

	// Set up providers
	qm.mu.Lock()
	qm.quotas["healthy"] = &QuotaInfo{Provider: "healthy", Status: "healthy"}
	qm.quotas["warning"] = &QuotaInfo{Provider: "warning", Status: "warning"}
	qm.quotas["banned"] = &QuotaInfo{Provider: "banned", Status: "banned"}
	qm.quotas["rate_limited"] = &QuotaInfo{Provider: "rate_limited", Status: "rate_limited"}
	qm.mu.Unlock()

	if !qm.IsProviderAvailable("healthy") {
		t.Error("Healthy provider should be available")
	}
	if !qm.IsProviderAvailable("warning") {
		t.Error("Warning provider should be available")
	}
	if qm.IsProviderAvailable("banned") {
		t.Error("Banned provider should not be available")
	}
	if qm.IsProviderAvailable("rate_limited") {
		t.Error("Rate limited provider should not be available")
	}
}

func TestQuotaMonitor_Reset(t *testing.T) {
	qm := NewQuotaMonitor(nil)

	qm.RecordRequest("provider1")
	qm.RecordRequest("provider2")

	qm.Reset()

	summary := qm.GetQuotaSummary()
	if summary.TotalProviders != 0 {
		t.Errorf("Expected 0 providers after reset, got %d", summary.TotalProviders)
	}
}

func TestQuotaMonitor_ResetProvider(t *testing.T) {
	qm := NewQuotaMonitor(nil)

	qm.RecordRequest("provider1")
	qm.RecordRequest("provider2")

	qm.ResetProvider("provider1")

	if qm.GetQuota("provider1") != nil {
		t.Error("provider1 should be nil after reset")
	}
	if qm.GetQuota("provider2") == nil {
		t.Error("provider2 should still exist")
	}
}

func TestQuotaMonitor_Disabled(t *testing.T) {
	config := &QuotaMonitorConfig{
		Enabled: false,
	}
	qm := NewQuotaMonitor(config)

	// Operations should be no-ops when disabled
	qm.RecordRequest("provider")
	qm.RecordUsage("provider", &http.Response{StatusCode: 200, Header: make(http.Header)}, 100)

	summary := qm.GetQuotaSummary()
	if summary.TotalProviders != 0 {
		t.Errorf("Disabled monitor should not record, got %d providers", summary.TotalProviders)
	}
}

func TestQuotaMonitor_Stats(t *testing.T) {
	config := &QuotaMonitorConfig{
		Enabled:           true,
		WarningThreshold:  25.0,
		CriticalThreshold: 10.0,
	}
	qm := NewQuotaMonitor(config)

	qm.mu.Lock()
	qm.quotas["provider1"] = &QuotaInfo{Provider: "provider1", RemainingPct: 80.0, Status: "healthy"}
	qm.quotas["provider2"] = &QuotaInfo{Provider: "provider2", RemainingPct: 15.0, Status: "warning"}
	qm.mu.Unlock()

	stats := qm.Stats()

	if stats["enabled"] != true {
		t.Error("Stats should show enabled=true")
	}
	if stats["total_providers"] != 2 {
		t.Errorf("Stats should show 2 providers, got %v", stats["total_providers"])
	}
	if stats["warning_threshold"] != 25.0 {
		t.Errorf("Stats should show warning_threshold=25.0, got %v", stats["warning_threshold"])
	}
	if stats["critical_threshold"] != 10.0 {
		t.Errorf("Stats should show critical_threshold=10.0, got %v", stats["critical_threshold"])
	}
}

func TestQuotaMonitor_RecordUsageWithTokens(t *testing.T) {
	qm := NewQuotaMonitor(nil)

	resp := &http.Response{
		StatusCode: 200,
		Header:     make(http.Header),
	}

	qm.RecordUsage("anthropic", resp, 100)
	qm.RecordUsage("anthropic", resp, 50)

	quota := qm.GetQuota("anthropic")
	if quota.UsedQuota != 150 {
		t.Errorf("Expected UsedQuota=150, got %d", quota.UsedQuota)
	}
}

func TestQuotaMonitor_LastSync(t *testing.T) {
	qm := NewQuotaMonitor(nil)

	before := time.Now().Add(-200 * time.Millisecond)
	qm.RecordRequest("provider")
	after := time.Now().Add(200 * time.Millisecond)

	quota := qm.GetQuota("provider")
	if quota.LastSync.Before(before) || quota.LastSync.After(after) {
		t.Error("LastSync should be between before and after")
	}
}
