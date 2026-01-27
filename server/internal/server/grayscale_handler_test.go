package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/config"
)

func TestGrayscaleHandler_ListFlags(t *testing.T) {
	cfg := &config.GrayscaleConfig{
		Enabled: true,
		Flags: []config.FeatureFlag{
			{
				Name:        "feature_a",
				Description: "Test feature A",
				Enabled:     true,
				Percentage:  50,
				Users:       []string{"user1", "user2"},
				Groups:      []string{"beta"},
				Rules: []config.TargetingRule{
					{Attribute: "country", Operator: "eq", Values: []string{"US"}},
				},
			},
			{
				Name:        "feature_b",
				Description: "Test feature B",
				Enabled:     false,
				Percentage:  100,
			},
		},
	}

	evaluator := config.NewFlagEvaluator(cfg)
	handler := NewGrayscaleHandler(evaluator)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/grayscale/flags", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.ListFlags(c)
	if err != nil {
		t.Fatalf("ListFlags failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var resp FlagListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Total != 2 {
		t.Errorf("Expected 2 flags, got %d", resp.Total)
	}

	if resp.Flags[0].Name != "feature_a" {
		t.Errorf("Expected feature_a, got %s", resp.Flags[0].Name)
	}

	if resp.Flags[0].UserCount != 2 {
		t.Errorf("Expected 2 users, got %d", resp.Flags[0].UserCount)
	}

	if resp.Flags[0].GroupCount != 1 {
		t.Errorf("Expected 1 group, got %d", resp.Flags[0].GroupCount)
	}

	if resp.Flags[0].RuleCount != 1 {
		t.Errorf("Expected 1 rule, got %d", resp.Flags[0].RuleCount)
	}
}

func TestGrayscaleHandler_ListFlags_NilEvaluator(t *testing.T) {
	handler := NewGrayscaleHandler(nil)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/grayscale/flags", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.ListFlags(c)
	if err != nil {
		t.Fatalf("ListFlags failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var resp FlagListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Total != 0 {
		t.Errorf("Expected 0 flags, got %d", resp.Total)
	}
}

func TestGrayscaleHandler_EvaluateFlag(t *testing.T) {
	cfg := &config.GrayscaleConfig{
		Enabled: true,
		Flags: []config.FeatureFlag{
			{
				Name:       "beta_feature",
				Enabled:    true,
				Users:      []string{"user1"},
				Percentage: 0,
			},
		},
	}

	evaluator := config.NewFlagEvaluator(cfg)
	handler := NewGrayscaleHandler(evaluator)

	tests := []struct {
		name           string
		request        EvaluateFlagRequest
		expectedStatus int
		expectedEnabled bool
	}{
		{
			name: "user_in_list",
			request: EvaluateFlagRequest{
				FlagName: "beta_feature",
				UserID:   "user1",
			},
			expectedStatus:  http.StatusOK,
			expectedEnabled: true,
		},
		{
			name: "user_not_in_list",
			request: EvaluateFlagRequest{
				FlagName: "beta_feature",
				UserID:   "user2",
			},
			expectedStatus:  http.StatusOK,
			expectedEnabled: false,
		},
		{
			name: "nonexistent_flag",
			request: EvaluateFlagRequest{
				FlagName: "nonexistent",
				UserID:   "user1",
			},
			expectedStatus:  http.StatusOK,
			expectedEnabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/grayscale/flags/evaluate", bytes.NewReader(body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			err := handler.EvaluateFlag(c)
			if err != nil {
				t.Fatalf("EvaluateFlag failed: %v", err)
			}

			if rec.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rec.Code)
			}

			var resp EvaluateFlagResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}

			if resp.Enabled != tt.expectedEnabled {
				t.Errorf("Expected enabled=%v, got %v", tt.expectedEnabled, resp.Enabled)
			}
		})
	}
}

func TestGrayscaleHandler_EvaluateFlag_MissingFlagName(t *testing.T) {
	handler := NewGrayscaleHandler(nil)

	body, _ := json.Marshal(EvaluateFlagRequest{})
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/grayscale/flags/evaluate", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.EvaluateFlag(c)
	if err != nil {
		t.Fatalf("EvaluateFlag failed: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rec.Code)
	}
}

func TestGrayscaleHandler_ListABTests(t *testing.T) {
	now := time.Now()
	cfg := &config.GrayscaleConfig{
		Enabled: true,
		ABTests: []config.ABTest{
			{
				Name:        "checkout_test",
				Description: "Checkout flow A/B test",
				Enabled:     true,
				StartTime:   now.Add(-time.Hour),
				EndTime:     now.Add(time.Hour),
				TrafficPct:  50,
				Variants: []config.ABVariant{
					{Name: "control", Value: "old", Weight: 50, IsControl: true},
					{Name: "treatment", Value: "new", Weight: 50},
				},
			},
		},
	}

	evaluator := config.NewFlagEvaluator(cfg)
	handler := NewGrayscaleHandler(evaluator)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/grayscale/abtests", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.ListABTests(c)
	if err != nil {
		t.Fatalf("ListABTests failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var resp ABTestListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Total != 1 {
		t.Errorf("Expected 1 test, got %d", resp.Total)
	}

	if resp.Tests[0].Name != "checkout_test" {
		t.Errorf("Expected checkout_test, got %s", resp.Tests[0].Name)
	}

	if resp.Tests[0].VariantCount != 2 {
		t.Errorf("Expected 2 variants, got %d", resp.Tests[0].VariantCount)
	}
}

func TestGrayscaleHandler_EvaluateABTest(t *testing.T) {
	now := time.Now()
	cfg := &config.GrayscaleConfig{
		Enabled: true,
		ABTests: []config.ABTest{
			{
				Name:       "active_test",
				Enabled:    true,
				StartTime:  now.Add(-time.Hour),
				EndTime:    now.Add(time.Hour),
				TrafficPct: 100,
				Variants: []config.ABVariant{
					{Name: "control", Value: "a", Weight: 100, IsControl: true},
				},
			},
		},
	}

	evaluator := config.NewFlagEvaluator(cfg)
	handler := NewGrayscaleHandler(evaluator)

	body, _ := json.Marshal(EvaluateABTestRequest{
		TestName: "active_test",
		UserID:   "user1",
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/grayscale/abtests/evaluate", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.EvaluateABTest(c)
	if err != nil {
		t.Fatalf("EvaluateABTest failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var resp EvaluateABTestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !resp.InTest {
		t.Error("Expected user to be in test")
	}

	if resp.VariantName != "control" {
		t.Errorf("Expected control variant, got %s", resp.VariantName)
	}

	if !resp.IsControl {
		t.Error("Expected IsControl to be true")
	}
}

func TestGrayscaleHandler_EvaluateABTest_NotInTest(t *testing.T) {
	now := time.Now()
	cfg := &config.GrayscaleConfig{
		Enabled: true,
		ABTests: []config.ABTest{
			{
				Name:       "ended_test",
				Enabled:    true,
				StartTime:  now.Add(-2 * time.Hour),
				EndTime:    now.Add(-time.Hour), // Already ended
				TrafficPct: 100,
				Variants: []config.ABVariant{
					{Name: "control", Value: "a", Weight: 100},
				},
			},
		},
	}

	evaluator := config.NewFlagEvaluator(cfg)
	handler := NewGrayscaleHandler(evaluator)

	body, _ := json.Marshal(EvaluateABTestRequest{
		TestName: "ended_test",
		UserID:   "user1",
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/grayscale/abtests/evaluate", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.EvaluateABTest(c)
	if err != nil {
		t.Fatalf("EvaluateABTest failed: %v", err)
	}

	var resp EvaluateABTestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.InTest {
		t.Error("Expected user to not be in test (test ended)")
	}
}

func TestGrayscaleHandler_EvaluateABTest_MissingTestName(t *testing.T) {
	handler := NewGrayscaleHandler(nil)

	body, _ := json.Marshal(EvaluateABTestRequest{})
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/grayscale/abtests/evaluate", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.EvaluateABTest(c)
	if err != nil {
		t.Fatalf("EvaluateABTest failed: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rec.Code)
	}
}

func TestGrayscaleHandler_GetConfigVersion(t *testing.T) {
	now := time.Now()
	cfg := &config.GrayscaleConfig{
		Enabled: true,
		Versions: []config.ConfigVersion{
			{
				Version:     "1.0.0",
				Description: "Initial version",
				Active:      false,
			},
			{
				Version:     "1.1.0",
				Description: "Current version",
				CreatedAt:   now,
				Active:      true,
				Values: map[string]interface{}{
					"max_connections": 100,
				},
			},
		},
	}

	evaluator := config.NewFlagEvaluator(cfg)
	handler := NewGrayscaleHandler(evaluator)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/grayscale/version", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.GetConfigVersion(c)
	if err != nil {
		t.Fatalf("GetConfigVersion failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var resp ConfigVersionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !resp.HasVersion {
		t.Error("Expected HasVersion to be true")
	}

	if resp.Version != "1.1.0" {
		t.Errorf("Expected version 1.1.0, got %s", resp.Version)
	}

	if resp.Values["max_connections"] != float64(100) {
		t.Errorf("Expected max_connections=100, got %v", resp.Values["max_connections"])
	}
}

func TestGrayscaleHandler_GetConfigVersion_NoActiveVersion(t *testing.T) {
	cfg := &config.GrayscaleConfig{
		Enabled: true,
		Versions: []config.ConfigVersion{
			{
				Version: "1.0.0",
				Active:  false,
			},
		},
	}

	evaluator := config.NewFlagEvaluator(cfg)
	handler := NewGrayscaleHandler(evaluator)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/grayscale/version", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.GetConfigVersion(c)
	if err != nil {
		t.Fatalf("GetConfigVersion failed: %v", err)
	}

	var resp ConfigVersionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.HasVersion {
		t.Error("Expected HasVersion to be false")
	}
}

func TestGrayscaleHandler_RegisterRoutes(t *testing.T) {
	handler := NewGrayscaleHandler(nil)
	e := echo.New()
	g := e.Group("/api/v1")

	handler.RegisterRoutes(g)

	routes := e.Routes()
	expectedPaths := map[string]bool{
		"/api/v1/grayscale/flags":           false,
		"/api/v1/grayscale/flags/evaluate":  false,
		"/api/v1/grayscale/abtests":         false,
		"/api/v1/grayscale/abtests/evaluate": false,
		"/api/v1/grayscale/version":         false,
	}

	for _, r := range routes {
		if _, ok := expectedPaths[r.Path]; ok {
			expectedPaths[r.Path] = true
		}
	}

	for path, found := range expectedPaths {
		if !found {
			t.Errorf("Expected route %s not found", path)
		}
	}
}
