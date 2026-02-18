package zalo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestZaloValidator_MissingAccessToken(t *testing.T) {
	v := NewValidator()
	result := v.Validate(context.Background(), map[string]string{})

	if result.Success {
		t.Error("expected failure for missing access_token")
	}
	if result.MessageKey != "accessTokenRequired" {
		t.Errorf("expected MessageKey 'accessTokenRequired', got '%s'", result.MessageKey)
	}
}

func TestZaloValidator_ValidCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request path
		if r.URL.Path != "/v2.0/oa/getoa" {
			t.Errorf("unexpected path: got %s", r.URL.Path)
		}

		// Verify access_token header
		accessToken := r.Header.Get("access_token")
		if accessToken != "test-access-token" {
			t.Errorf("unexpected access_token header: got %s", accessToken)
		}

		response := map[string]interface{}{
			"error":   0,
			"message": "Success",
			"data": map[string]interface{}{
				"oa_id":        "180789",
				"name":         "Test OA",
				"description":  "Test Official Account",
				"is_verified":  true,
				"num_follower": 1000,
				"oa_type":      2,
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{
		"access_token": "test-access-token",
	})

	if !result.Success {
		t.Errorf("expected success, got error: %s - %s", result.MessageKey, result.Error)
	}
	if result.MessageKey != "testSuccess" {
		t.Errorf("expected MessageKey 'testSuccess', got '%s'", result.MessageKey)
	}
	if result.Data["oa_id"] != "180789" {
		t.Errorf("expected oa_id '180789', got '%v'", result.Data["oa_id"])
	}
	if result.Data["oa_name"] != "Test OA" {
		t.Errorf("expected oa_name 'Test OA', got '%v'", result.Data["oa_name"])
	}
	if result.Data["is_verified"] != true {
		t.Errorf("expected is_verified true, got '%v'", result.Data["is_verified"])
	}
}

func TestZaloValidator_InvalidAccessToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"error":   -124,
			"message": "Invalid access token",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{
		"access_token": "invalid-token",
	})

	if result.Success {
		t.Error("expected failure for invalid access token")
	}
	if result.MessageKey != "invalidToken" {
		t.Errorf("expected MessageKey 'invalidToken', got '%s'", result.MessageKey)
	}
}

func TestZaloValidator_ExpiredAccessToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"error":   -216,
			"message": "Access token expired",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{
		"access_token": "expired-token",
	})

	if result.Success {
		t.Error("expected failure for expired access token")
	}
	if result.MessageKey != "invalidToken" {
		t.Errorf("expected MessageKey 'invalidToken', got '%s'", result.MessageKey)
	}
}

func TestZaloValidator_InvalidOA(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"error":   -201,
			"message": "Invalid OA",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{
		"access_token": "test-token",
	})

	if result.Success {
		t.Error("expected failure for invalid OA")
	}
	if result.MessageKey != "invalidToken" {
		t.Errorf("expected MessageKey 'invalidToken', got '%s'", result.MessageKey)
	}
}

func TestZaloValidator_OtherError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"error":   -999,
			"message": "Unknown error",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{
		"access_token": "test-token",
	})

	if result.Success {
		t.Error("expected failure for other error")
	}
	if result.MessageKey != "connectionFailed" {
		t.Errorf("expected MessageKey 'connectionFailed', got '%s'", result.MessageKey)
	}
}

func TestZaloValidator_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": 0})
	}))
	defer server.Close()

	v := NewValidatorWithOptions(100*time.Millisecond, server.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result := v.Validate(ctx, map[string]string{
		"access_token": "test-token",
	})

	if result.Success {
		t.Error("expected timeout or connection failure")
	}
}

func TestZaloValidator_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{
		"access_token": "test-token",
	})

	if result.Success {
		t.Error("expected failure for invalid JSON")
	}
	if result.MessageKey != "connectionFailed" {
		t.Errorf("expected MessageKey 'connectionFailed', got '%s'", result.MessageKey)
	}
}

func TestZaloValidator_NetworkError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	serverURL := server.URL
	server.Close()

	v := NewValidatorWithOptions(1*time.Second, serverURL)
	result := v.Validate(context.Background(), map[string]string{
		"access_token": "test-token",
	})

	if result.Success {
		t.Error("expected connection failure")
	}
	if result.MessageKey != "connectionFailed" {
		t.Errorf("expected MessageKey 'connectionFailed', got '%s'", result.MessageKey)
	}
}

func TestZaloValidator_NoOAData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"error":   0,
			"message": "Success",
			// No data field
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{
		"access_token": "test-token",
	})

	if result.Success {
		t.Error("expected failure for no OA data")
	}
	if result.MessageKey != "connectionFailed" {
		t.Errorf("expected MessageKey 'connectionFailed', got '%s'", result.MessageKey)
	}
}

func TestZaloValidator_UnverifiedOA(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"error":   0,
			"message": "Success",
			"data": map[string]interface{}{
				"oa_id":        "987654321",
				"name":         "Unverified OA",
				"is_verified":  false,
				"num_follower": 50,
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	v := NewValidatorWithOptions(10*time.Second, server.URL)
	result := v.Validate(context.Background(), map[string]string{
		"access_token": "test-token",
	})

	// Should still succeed, just indicate unverified
	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if result.Data["is_verified"] != false {
		t.Errorf("expected is_verified false, got '%v'", result.Data["is_verified"])
	}
}
