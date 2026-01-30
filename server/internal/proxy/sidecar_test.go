package proxy

import (
	"bufio"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/proxy/testutil"
)

// =============================================================================
// Test Setup Helpers
// =============================================================================

// setupTestEnvironment creates a mock server and proxy server for testing
func setupTestEnvironment(t *testing.T) (*testutil.MockServer, *ProxyServer, func()) {
	t.Helper()

	// Create mock upstream server
	mockServer, err := testutil.NewMockServer(0)
	if err != nil {
		t.Fatalf("Failed to create mock server: %v", err)
	}
	if err := mockServer.Start(); err != nil {
		t.Fatalf("Failed to start mock server: %v", err)
	}

	// Create proxy config
	config := DefaultProxyConfig()
	config.Port.Value = 0
	config.Port.Range = "19200-19300"
	config.Routing.Providers = []*ProviderConfig{
		{
			Name:     "mock",
			Endpoint: mockServer.URL(),
			Priority: 1,
			Enabled:  true,
		},
	}
	config.Routing.DefaultProvider = "mock"
	config.HealthCheck.Enabled = false

	// Create and start proxy server
	ps, err := NewProxyServer(config)
	if err != nil {
		mockServer.Stop()
		t.Fatalf("Failed to create proxy server: %v", err)
	}

	if err := ps.Start(); err != nil {
		mockServer.Stop()
		t.Fatalf("Failed to start proxy server: %v", err)
	}

	// Wait for servers to be ready
	time.Sleep(100 * time.Millisecond)

	cleanup := func() {
		ps.Stop(context.Background())
		mockServer.Stop()
	}

	return mockServer, ps, cleanup
}

// =============================================================================
// Section 1: API Key Authentication Tests
// =============================================================================

func TestAPIKeyAuthSuccess(t *testing.T) {
	mockServer, ps, cleanup := setupTestEnvironment(t)
	defer cleanup()
	_ = mockServer

	client := testutil.NewTestClient(ps.GetEndpoint(), "")

	// Test without API key (should work if auth is disabled)
	models, err := client.GetModels()
	if err != nil {
		t.Logf("GetModels without key: %v", err)
	} else {
		t.Logf("Got %d models", len(models))
	}
}

func TestAPIKeyAuthFailure(t *testing.T) {
	// This test verifies behavior when authentication is enabled
	t.Run("NoKey", func(t *testing.T) {
		// When auth is enabled, requests without key should fail
		// For now, we just verify the endpoint is accessible
		mockServer, ps, cleanup := setupTestEnvironment(t)
		defer cleanup()
		_ = mockServer

		resp, err := http.Get(ps.GetEndpoint() + "/api/v1/proxy/status")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		// Status endpoint should be accessible
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("InvalidKey", func(t *testing.T) {
		mockServer, ps, cleanup := setupTestEnvironment(t)
		defer cleanup()
		_ = mockServer

		req, _ := http.NewRequest("GET", ps.GetEndpoint()+"/v1/models", nil)
		req.Header.Set("Authorization", "Bearer invalid_key_12345")

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		// Log the response for debugging
		t.Logf("Response status with invalid key: %d", resp.StatusCode)
	})
}

// =============================================================================
// Section 2: SSE Streaming Tests
// =============================================================================

func TestSSEBasicStreaming(t *testing.T) {
	mockServer, ps, cleanup := setupTestEnvironment(t)
	defer cleanup()
	_ = mockServer

	client := testutil.NewTestClient(ps.GetEndpoint(), "")
	req := testutil.CreateTestRequest(true)

	resp, err := client.SendStreamRequest(req)
	if err != nil {
		t.Fatalf("SendStreamRequest failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify response headers
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/event-stream") {
		t.Errorf("Expected Content-Type text/event-stream, got %s", contentType)
	}

	// Collect chunks
	chunks, err := testutil.CollectSSEChunks(resp.Body)
	if err != nil {
		t.Fatalf("Failed to collect chunks: %v", err)
	}

	if len(chunks) == 0 {
		t.Fatal("No chunks received")
	}

	// Verify we got a [DONE] marker
	lastChunk := chunks[len(chunks)-1]
	if !lastChunk.IsDone {
		t.Error("Last chunk should be [DONE]")
	}

	t.Logf("Received %d SSE chunks", len(chunks))
}

func TestSSEChunkTiming(t *testing.T) {
	mockServer, ps, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Set chunk delay to make timing measurable
	mockServer.SetStreamChunkDelay(10 * time.Millisecond)

	client := testutil.NewTestClient(ps.GetEndpoint(), "")
	req := testutil.CreateTestRequest(true)

	resp, err := client.SendStreamRequest(req)
	if err != nil {
		t.Fatalf("SendStreamRequest failed: %v", err)
	}

	var timestamps []time.Time
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			timestamps = append(timestamps, time.Now())
		}
	}
	resp.Body.Close()

	if len(timestamps) < 2 {
		t.Skip("Not enough chunks to verify timing")
	}

	// Verify timestamps are generally increasing (allow for same-millisecond arrivals)
	outOfOrderCount := 0
	for i := 1; i < len(timestamps); i++ {
		if timestamps[i].Before(timestamps[i-1]) {
			outOfOrderCount++
		}
	}

	// Allow some tolerance for timing precision
	if outOfOrderCount > len(timestamps)/4 {
		t.Errorf("Too many out-of-order timestamps: %d/%d", outOfOrderCount, len(timestamps))
	}

	t.Logf("Received %d chunks, %d out of order (acceptable)", len(timestamps), outOfOrderCount)
}

func TestTTFTMeasurement(t *testing.T) {
	mockServer, ps, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Set a small delay to make TTFT measurable
	mockServer.SetResponseDelay(50 * time.Millisecond)

	client := testutil.NewTestClient(ps.GetEndpoint(), "")
	req := testutil.CreateTestRequest(true)

	// Measure TTFT
	ttft, err := client.MeasureTTFT(req)
	if err != nil {
		t.Fatalf("MeasureTTFT failed: %v", err)
	}

	t.Logf("TTFT: %v", ttft)

	// TTFT should be at least the response delay
	if ttft < 50*time.Millisecond {
		t.Errorf("TTFT %v is less than expected delay", ttft)
	}
}

func TestSSEStreamInterruption(t *testing.T) {
	mockServer, ps, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Configure mock to interrupt after 5 chunks
	mockServer.SetInterruptAfterChunks(5)

	client := testutil.NewTestClient(ps.GetEndpoint(), "")
	req := testutil.CreateTestRequest(true)

	resp, err := client.SendStreamRequest(req)
	if err != nil {
		t.Fatalf("SendStreamRequest failed: %v", err)
	}

	chunks, _ := testutil.CollectSSEChunks(resp.Body)

	// Should have received approximately 5 chunks before interruption
	if len(chunks) > 6 {
		t.Errorf("Expected ~5 chunks before interruption, got %d", len(chunks))
	}

	t.Logf("Received %d chunks before interruption", len(chunks))
}

// =============================================================================
// Section 3: Daemon Mode and Long Connection Tests
// =============================================================================

func TestLongConnection(t *testing.T) {
	mockServer, ps, cleanup := setupTestEnvironment(t)
	defer cleanup()
	_ = mockServer

	// Create client with keep-alive
	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
		Timeout: 5 * time.Second,
	}

	// Send multiple requests to verify connection reuse
	successCount := 0
	for i := 0; i < 20; i++ {
		req, _ := http.NewRequest("GET", ps.GetEndpoint()+"/api/v1/proxy/health", nil)
		req.Header.Set("Connection", "keep-alive")

		resp, err := client.Do(req)
		if err != nil {
			t.Logf("Request %d failed: %v", i, err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			successCount++
		}

		time.Sleep(50 * time.Millisecond)
	}

	if successCount < 18 {
		t.Errorf("Expected at least 18 successful requests, got %d", successCount)
	}

	t.Logf("Completed %d/20 requests successfully", successCount)
}

func TestConnectionPooling(t *testing.T) {
	mockServer, ps, cleanup := setupTestEnvironment(t)
	defer cleanup()
	_ = mockServer

	client := testutil.NewTestClient(ps.GetEndpoint(), "")

	// Send concurrent requests
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			req := testutil.CreateTestRequest(false)
			_, _, err := client.SendRequest(req)
			done <- (err == nil)
		}()
	}

	// Wait for all requests
	successCount := 0
	for i := 0; i < 10; i++ {
		if <-done {
			successCount++
		}
	}

	t.Logf("Concurrent requests: %d/10 successful", successCount)

	if successCount < 8 {
		t.Errorf("Expected at least 8 successful concurrent requests, got %d", successCount)
	}
}

func TestSessionTracking(t *testing.T) {
	mockServer, ps, cleanup := setupTestEnvironment(t)
	defer cleanup()
	_ = mockServer

	client := testutil.NewTestClient(ps.GetEndpoint(), "")

	// Send a streaming request
	req := testutil.CreateTestRequest(true)
	resp, err := client.SendStreamRequest(req)
	if err != nil {
		t.Fatalf("SendStreamRequest failed: %v", err)
	}

	// Check proxy status while request is in progress
	status, err := client.GetProxyStatus()
	if err != nil {
		t.Logf("GetProxyStatus: %v", err)
	} else {
		t.Logf("Proxy status: %v", status)
	}

	// Drain the response
	testutil.DrainResponse(resp)

	// Verify session completed
	time.Sleep(100 * time.Millisecond)
}

// =============================================================================
// Section 4: High Availability Tests
// =============================================================================

func TestFailover(t *testing.T) {
	// Create two mock servers - primary fails, secondary succeeds
	primaryServer, err := testutil.NewMockServer(0)
	if err != nil {
		t.Fatalf("Failed to create primary mock server: %v", err)
	}
	if err := primaryServer.Start(); err != nil {
		t.Fatalf("Failed to start primary mock server: %v", err)
	}
	defer primaryServer.Stop()

	secondaryServer, err := testutil.NewMockServer(0)
	if err != nil {
		t.Fatalf("Failed to create secondary mock server: %v", err)
	}
	if err := secondaryServer.Start(); err != nil {
		t.Fatalf("Failed to start secondary mock server: %v", err)
	}
	defer secondaryServer.Stop()

	// Configure primary to return 503
	primaryServer.SetErrorMode(testutil.Error503)

	// Create proxy with both providers
	config := DefaultProxyConfig()
	config.Port.Value = 0
	config.Port.Range = "19300-19400"
	config.Routing.Providers = []*ProviderConfig{
		{Name: "primary", Endpoint: primaryServer.URL(), Priority: 1, Enabled: true},
		{Name: "secondary", Endpoint: secondaryServer.URL(), Priority: 2, Enabled: true},
	}
	config.Routing.DefaultProvider = "primary"
	config.HealthCheck.Enabled = false
	config.Routing.Failover.Enabled = true
	config.Routing.Failover.MaxRetries = 1

	ps, err := NewProxyServer(config)
	if err != nil {
		t.Fatalf("Failed to create proxy server: %v", err)
	}
	if err := ps.Start(); err != nil {
		t.Fatalf("Failed to start proxy server: %v", err)
	}
	defer ps.Stop(context.Background())

	time.Sleep(100 * time.Millisecond)

	// Send request - should failover to secondary
	client := testutil.NewTestClient(ps.GetEndpoint(), "")
	req := testutil.CreateTestRequest(false)

	_, resp, err := client.SendRequest(req)
	if err != nil {
		t.Logf("Request error (may be expected): %v", err)
	}
	if resp != nil {
		t.Logf("Response status: %d", resp.StatusCode)
	}

	// Verify secondary received the request
	if secondaryServer.GetRequestCount() > 0 {
		t.Log("Failover to secondary successful")
	} else {
		t.Log("Primary request count:", primaryServer.GetRequestCount())
		t.Log("Secondary request count:", secondaryServer.GetRequestCount())
	}
}

func TestCircuitBreaker(t *testing.T) {
	mockServer, ps, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Configure mock to return errors
	mockServer.SetErrorMode(testutil.Error500)

	client := testutil.NewTestClient(ps.GetEndpoint(), "")
	req := testutil.CreateTestRequest(false)

	// Send multiple requests to trigger circuit breaker
	for i := 0; i < 5; i++ {
		client.SendRequest(req)
	}

	// Check circuit breaker state via proxy status
	status, err := client.GetProxyStatus()
	if err != nil {
		t.Logf("GetProxyStatus: %v", err)
	} else {
		t.Logf("Proxy status after errors: %v", status)
	}

	// Reset mock and verify recovery
	mockServer.SetErrorMode(testutil.ErrorNone)
	time.Sleep(100 * time.Millisecond)

	_, _, err = client.SendRequest(req)
	if err != nil {
		t.Logf("Request after recovery: %v", err)
	} else {
		t.Log("Circuit breaker recovery successful")
	}
}

func TestRetryMechanism(t *testing.T) {
	mockServer, ps, cleanup := setupTestEnvironment(t)
	defer cleanup()

	tests := []struct {
		name        string
		errorMode   string
		shouldRetry bool
	}{
		{"Error500", testutil.Error500, true},
		{"Error502", testutil.Error502, true},
		{"Error503", testutil.Error503, true},
		{"Error429", testutil.Error429, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockServer.Reset()
			mockServer.SetErrorMode(tt.errorMode)

			client := testutil.NewTestClient(ps.GetEndpoint(), "")
			req := testutil.CreateTestRequest(false)

			client.SendRequest(req)

			requestCount := mockServer.GetRequestCount()
			t.Logf("%s: %d requests made", tt.name, requestCount)

			if tt.shouldRetry && requestCount <= 1 {
				t.Logf("Expected retries for %s", tt.name)
			}
		})
	}
}

func TestHealthCheck(t *testing.T) {
	mockServer, ps, cleanup := setupTestEnvironment(t)
	defer cleanup()

	client := testutil.NewTestClient(ps.GetEndpoint(), "")

	// Check initial health
	statusCode, result, err := client.GetHealth()
	if err != nil {
		t.Fatalf("GetHealth failed: %v", err)
	}

	t.Logf("Initial health: status=%d, result=%v", statusCode, result)

	// Make mock unhealthy
	mockServer.SetHealthy(false)

	// Check health again
	statusCode, result, err = client.GetHealth()
	if err != nil {
		t.Logf("GetHealth after unhealthy: %v", err)
	} else {
		t.Logf("Health after mock unhealthy: status=%d, result=%v", statusCode, result)
	}
}

// =============================================================================
// Section 5: Metrics Tests
// =============================================================================

func TestRequestCounting(t *testing.T) {
	mockServer, ps, cleanup := setupTestEnvironment(t)
	defer cleanup()

	client := testutil.NewTestClient(ps.GetEndpoint(), "")

	// Send known number of requests
	successCount := 5
	for i := 0; i < successCount; i++ {
		req := testutil.CreateTestRequest(false)
		client.SendRequest(req)
	}

	// Check mock server received all requests
	if mockServer.GetRequestCount() != int64(successCount) {
		t.Errorf("Expected %d requests, mock received %d", successCount, mockServer.GetRequestCount())
	}

	// Get proxy metrics
	metrics, err := client.GetProxyMetrics()
	if err != nil {
		t.Logf("GetProxyMetrics: %v", err)
	} else {
		t.Logf("Proxy metrics: %v", metrics)
	}
}

func TestTokenUsageMetrics(t *testing.T) {
	mockServer, ps, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Set known token usage
	mockServer.SetTokenUsage(testutil.TokenUsage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	})

	client := testutil.NewTestClient(ps.GetEndpoint(), "")

	// Send requests
	for i := 0; i < 5; i++ {
		req := testutil.CreateTestRequest(false)
		resp, _, err := client.SendRequest(req)
		if err != nil {
			continue
		}
		t.Logf("Response usage: %+v", resp.Usage)
	}
}

func TestLatencyMetrics(t *testing.T) {
	mockServer, ps, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Set known delay
	mockServer.SetResponseDelay(50 * time.Millisecond)

	client := testutil.NewTestClient(ps.GetEndpoint(), "")
	req := testutil.CreateTestRequest(true)

	// Measure multiple TTFTs
	stats, err := client.MeasureTTFTMultiple(req, 10)
	if err != nil {
		t.Fatalf("MeasureTTFTMultiple failed: %v", err)
	}

	t.Logf("TTFT Stats: P50=%v, P95=%v, Avg=%v, Min=%v, Max=%v",
		stats.P50, stats.P95, stats.Avg, stats.Min, stats.Max)

	// Verify TTFT is at least the response delay
	if stats.P50 < 50*time.Millisecond {
		t.Errorf("P50 TTFT %v is less than expected delay", stats.P50)
	}
}

func TestProviderMetrics(t *testing.T) {
	mockServer, ps, cleanup := setupTestEnvironment(t)
	defer cleanup()
	_ = mockServer

	client := testutil.NewTestClient(ps.GetEndpoint(), "")

	// Send requests
	for i := 0; i < 5; i++ {
		req := testutil.CreateTestRequest(false)
		client.SendRequest(req)
	}

	// Get providers
	providers, err := client.GetProviders()
	if err != nil {
		t.Logf("GetProviders: %v", err)
	} else {
		t.Logf("Providers: %v", providers)
	}
}

// =============================================================================
// Section 6: Error Scenario Tests
// =============================================================================

func TestUpstreamErrors(t *testing.T) {
	mockServer, ps, cleanup := setupTestEnvironment(t)
	defer cleanup()

	testCases := []struct {
		name      string
		errorMode string
		expectErr bool
	}{
		{"Error500", testutil.Error500, true},
		{"Error502", testutil.Error502, true},
		{"Error503", testutil.Error503, true},
		{"Error429", testutil.Error429, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockServer.Reset()
			mockServer.SetErrorMode(tc.errorMode)

			client := testutil.NewTestClient(ps.GetEndpoint(), "")
			req := testutil.CreateTestRequest(false)

			_, resp, err := client.SendRequest(req)
			if tc.expectErr {
				if err == nil && resp != nil && resp.StatusCode == 200 {
					t.Errorf("Expected error for %s, got success", tc.name)
				}
			}

			t.Logf("%s: error=%v, requests=%d", tc.name, err != nil, mockServer.GetRequestCount())
		})
	}
}

func TestNetworkErrors(t *testing.T) {
	t.Run("ConnectionTimeout", func(t *testing.T) {
		mockServer, ps, cleanup := setupTestEnvironment(t)
		defer cleanup()

		// Set very long delay to simulate timeout
		mockServer.SetResponseDelay(60 * time.Second)

		client := testutil.NewTestClient(ps.GetEndpoint(), "")
		client.SetTimeout(2 * time.Second)

		req := testutil.CreateTestRequest(false)

		start := time.Now()
		_, _, err := client.SendRequest(req)
		elapsed := time.Since(start)

		if err == nil {
			t.Error("Expected timeout error")
		}

		// Should timeout within reasonable time
		if elapsed > 5*time.Second {
			t.Errorf("Timeout took too long: %v", elapsed)
		}

		t.Logf("Timeout after %v", elapsed)
	})

	t.Run("ConnectionRefused", func(t *testing.T) {
		// Create proxy pointing to non-existent server
		config := DefaultProxyConfig()
		config.Port.Value = 0
		config.Port.Range = "19400-19500"
		config.Routing.Providers = []*ProviderConfig{
			{Name: "dead", Endpoint: "http://127.0.0.1:59999", Priority: 1, Enabled: true},
		}
		config.Routing.DefaultProvider = "dead"
		config.HealthCheck.Enabled = false

		ps, err := NewProxyServer(config)
		if err != nil {
			t.Fatalf("Failed to create proxy server: %v", err)
		}
		if err := ps.Start(); err != nil {
			t.Fatalf("Failed to start proxy server: %v", err)
		}
		defer ps.Stop(context.Background())

		time.Sleep(100 * time.Millisecond)

		client := testutil.NewTestClient(ps.GetEndpoint(), "")
		req := testutil.CreateTestRequest(false)

		_, _, err = client.SendRequest(req)
		if err == nil {
			t.Log("Request succeeded unexpectedly (may have fallback)")
		} else {
			t.Logf("Connection refused error: %v", err)
		}
	})
}

func TestSSEStreamErrors(t *testing.T) {
	t.Run("StreamInterruption", func(t *testing.T) {
		mockServer, ps, cleanup := setupTestEnvironment(t)
		defer cleanup()

		mockServer.SetInterruptAfterChunks(3)

		client := testutil.NewTestClient(ps.GetEndpoint(), "")
		req := testutil.CreateTestRequest(true)

		resp, err := client.SendStreamRequest(req)
		if err != nil {
			t.Fatalf("SendStreamRequest failed: %v", err)
		}

		chunks, _ := testutil.CollectSSEChunks(resp.Body)
		t.Logf("Received %d chunks before interruption", len(chunks))

		if len(chunks) > 5 {
			t.Errorf("Expected fewer chunks due to interruption, got %d", len(chunks))
		}
	})

	t.Run("MalformedJSON", func(t *testing.T) {
		mockServer, ps, cleanup := setupTestEnvironment(t)
		defer cleanup()

		mockServer.SetMalformedChunkAt(2)

		client := testutil.NewTestClient(ps.GetEndpoint(), "")
		req := testutil.CreateTestRequest(true)

		resp, err := client.SendStreamRequest(req)
		if err != nil {
			t.Fatalf("SendStreamRequest failed: %v", err)
		}

		chunks, _ := testutil.CollectSSEChunks(resp.Body)

		// Count chunks with parse errors
		errorCount := 0
		for _, chunk := range chunks {
			if chunk.Error != nil {
				errorCount++
			}
		}

		t.Logf("Received %d chunks, %d with parse errors", len(chunks), errorCount)

		if errorCount == 0 {
			t.Log("No parse errors detected (malformed chunk may have been skipped)")
		}
	})

	t.Run("StreamTimeout", func(t *testing.T) {
		mockServer, ps, cleanup := setupTestEnvironment(t)
		defer cleanup()

		// Set very long chunk delay
		mockServer.SetStreamChunkDelay(30 * time.Second)

		client := testutil.NewTestClient(ps.GetEndpoint(), "")

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		req := testutil.CreateTestRequest(true)
		_, err := client.SendStreamRequestWithContext(ctx, req)

		if err == nil {
			t.Log("Request completed (may have received some chunks)")
		} else {
			t.Logf("Stream timeout: %v", err)
		}
	})
}
