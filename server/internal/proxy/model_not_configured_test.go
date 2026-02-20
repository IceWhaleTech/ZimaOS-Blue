package proxy

import (
	"testing"
)

// TestIsModelNotConfiguredError tests the detection of "not configured" error patterns
func TestIsModelNotConfiguredError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		expected   bool
	}{
		// Positive cases — should be detected as "not configured"
		{
			name:       "Chinese not configured message",
			statusCode: 404,
			body:       `{"error":{"message":"模型 gpt-4 not configured on this provider"}}`,
			expected:   true,
		},
		{
			name:       "Model not enabled",
			statusCode: 403,
			body:       `{"error":{"message":"Model gpt-4-turbo is not enabled for your account"}}`,
			expected:   true,
		},
		{
			name:       "Model not available",
			statusCode: 404,
			body:       `{"error":{"message":"The model claude-3-opus is not available"}}`,
			expected:   true,
		},
		{
			name:       "Model not supported",
			statusCode: 400,
			body:       `{"error":{"message":"Model not supported on this endpoint"}}`,
			expected:   true,
		},
		{
			name:       "No access to model",
			statusCode: 403,
			body:       `{"error":{"message":"You have no access to model gpt-4-32k"}}`,
			expected:   true,
		},
		{
			name:       "Model disabled",
			statusCode: 400,
			body:       `{"error":{"message":"model disabled by administrator"}}`,
			expected:   true,
		},
		{
			name:       "Model unavailable",
			statusCode: 404,
			body:       `{"error":{"message":"model unavailable at this time"}}`,
			expected:   true,
		},
		{
			name:       "Model not found",
			statusCode: 404,
			body:       `{"error":{"message":"The model gpt-5 was model not found"}}`,
			expected:   true,
		},
		{
			name:       "Model does not exist",
			statusCode: 404,
			body:       `{"error":{"message":"The model does not exist"}}`,
			expected:   true,
		},
		{
			name:       "Invalid model",
			statusCode: 404,
			body:       `{"error":{"message":"invalid model: gpt-4-turbo-preview"}}`,
			expected:   true,
		},
		{
			name:       "Unknown model",
			statusCode: 404,
			body:       `{"error":{"message":"unknown model specified"}}`,
			expected:   true,
		},
		{
			name:       "Not authorized for model",
			statusCode: 403,
			body:       `{"error":{"message":"not authorized to use this model"}}`,
			expected:   true,
		},
		{
			name:       "Permission denied",
			statusCode: 403,
			body:       `{"error":{"message":"permission denied for model access"}}`,
			expected:   true,
		},
		{
			name:       "422 not configured",
			statusCode: 422,
			body:       `{"error":{"message":"Model not configured"}}`,
			expected:   true,
		},

		// Negative cases — should NOT be detected as "not configured"
		{
			name:       "Rate limit error",
			statusCode: 429,
			body:       `{"error":{"message":"Rate limit exceeded"}}`,
			expected:   false,
		},
		{
			name:       "Server error",
			statusCode: 500,
			body:       `{"error":{"message":"Internal server error"}}`,
			expected:   false,
		},
		{
			name:       "Generic 400 bad request",
			statusCode: 400,
			body:       `{"error":{"message":"Invalid JSON in request body"}}`,
			expected:   false,
		},
		{
			name:       "Context too long",
			statusCode: 400,
			body:       `{"error":{"message":"Context size exceeds maximum allowed"}}`,
			expected:   false,
		},
		{
			name:       "Auth error without model mention",
			statusCode: 401,
			body:       `{"error":{"message":"Invalid API key"}}`,
			expected:   false,
		},
		{
			name:       "200 success with not configured in body",
			statusCode: 200,
			body:       `{"message":"not configured"}`,
			expected:   false,
		},
		{
			name:       "Empty body 404",
			statusCode: 404,
			body:       ``,
			expected:   false,
		},
		// Chinese error messages
		{
			name:       "Chinese: model not configured (未配置)",
			statusCode: 400,
			body:       `{"error":"端点/claude-aws未配置模型claude-3-7-sonnet"}`,
			expected:   true,
		},
		{
			name:       "Chinese: model not supported (不支持)",
			statusCode: 400,
			body:       `{"error":"该端点不支持此模型"}`,
			expected:   true,
		},
		{
			name:       "Chinese: model not found (未找到)",
			statusCode: 404,
			body:       `{"error":"未找到模型"}`,
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isModelNotConfiguredError(tt.statusCode, []byte(tt.body))
			if got != tt.expected {
				t.Errorf("isModelNotConfiguredError(%d, %q) = %v, want %v",
					tt.statusCode, tt.body, got, tt.expected)
			}
		})
	}
}

// TestClassifier_ModelNotConfigured tests the classifier recognizes "not configured" errors
func TestClassifier_ModelNotConfigured(t *testing.T) {
	classifier := NewAPIErrorClassifier()

	tests := []struct {
		name           string
		provider       string
		statusCode     int
		responseBody   string
		expectedType   RetryableErrorType
		expectedCat    ErrorCategory
		shouldFailover bool
	}{
		{
			name:           "Anthropic model not configured",
			provider:       "anthropic",
			statusCode:     404,
			responseBody:   `{"error":{"type":"not_found_error","message":"Model claude-3-opus is not available in your region"}}`,
			expectedType:   ErrorTypeModelNotConfigured,
			expectedCat:    ErrorCategoryFailover,
			shouldFailover: true,
		},
		{
			name:           "Anthropic model not enabled",
			provider:       "anthropic",
			statusCode:     403,
			responseBody:   `{"error":{"type":"forbidden","message":"Model not enabled for this API key"}}`,
			expectedType:   ErrorTypeModelNotConfigured,
			expectedCat:    ErrorCategoryFailover,
			shouldFailover: true,
		},
		{
			name:           "Anthropic model not found with failover",
			provider:       "anthropic",
			statusCode:     404,
			responseBody:   `{"error":{"type":"not_found_error","message":"model not found: claude-3-opus-20240229"}}`,
			expectedType:   ErrorTypeModelNotFound,
			expectedCat:    ErrorCategoryFailover,
			shouldFailover: true,
		},
		{
			name:           "Generic provider model not configured",
			provider:       "custom-relay",
			statusCode:     404,
			responseBody:   `{"error":{"message":"Model gpt-4 is not configured"}}`,
			expectedType:   ErrorTypeModelNotConfigured,
			expectedCat:    ErrorCategoryFailover,
			shouldFailover: true,
		},
		{
			name:           "Generic provider model not available",
			provider:       "custom-relay",
			statusCode:     400,
			responseBody:   `{"message":"This model is not available on this endpoint"}`,
			expectedType:   ErrorTypeModelNotConfigured,
			expectedCat:    ErrorCategoryFailover,
			shouldFailover: true,
		},
		{
			name:           "Generic provider model disabled",
			provider:       "custom-relay",
			statusCode:     403,
			responseBody:   `{"error":{"message":"model disabled by policy"}}`,
			expectedType:   ErrorTypeModelNotConfigured,
			expectedCat:    ErrorCategoryFailover,
			shouldFailover: true,
		},
		{
			name:           "Generic 404 model not found triggers failover",
			provider:       "unknown-provider",
			statusCode:     404,
			responseBody:   `{"error":{"message":"Resource not found"}}`,
			expectedType:   ErrorTypeModelNotFound,
			expectedCat:    ErrorCategoryFailover,
			shouldFailover: true,
		},
		{
			name:           "Auth error still non-retryable",
			provider:       "anthropic",
			statusCode:     401,
			responseBody:   `{"error":{"type":"authentication_error","message":"Invalid API key"}}`,
			expectedType:   ErrorTypeAuthFailed,
			expectedCat:    ErrorCategoryNonRetryable,
			shouldFailover: false,
		},
		{
			name:           "Rate limit still failover",
			provider:       "anthropic",
			statusCode:     429,
			responseBody:   `{"error":{"type":"rate_limit_error","message":"Rate limit exceeded"}}`,
			expectedType:   ErrorTypeRateLimited,
			expectedCat:    ErrorCategoryFailover,
			shouldFailover: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			classification := classifier.ClassifyError(tt.provider, tt.statusCode, []byte(tt.responseBody))

			if classification.Type != tt.expectedType {
				t.Errorf("expected type %s, got %s", tt.expectedType, classification.Type)
			}
			if classification.Category != tt.expectedCat {
				t.Errorf("expected category %s, got %s", tt.expectedCat, classification.Category)
			}
			if classification.ShouldFailover != tt.shouldFailover {
				t.Errorf("expected shouldFailover %v, got %v", tt.shouldFailover, classification.ShouldFailover)
			}
		})
	}
}

// TestClassifier_ModelNotConfigured_AllProviders tests across all provider patterns
func TestClassifier_ModelNotConfigured_AllProviders(t *testing.T) {
	classifier := NewAPIErrorClassifier()

	// "not configured" should be detected as failover for all providers
	providers := []string{"anthropic", "openai", "deepseek", "google", "custom-relay", "unknown"}
	messages := []string{
		`{"error":{"message":"Model not configured"}}`,
		`{"error":{"message":"Model is not enabled"}}`,
		`{"error":{"message":"Model not available"}}`,
		`{"error":{"message":"Model not supported"}}`,
	}

	for _, provider := range providers {
		for _, body := range messages {
			t.Run(provider+"_"+body[:40], func(t *testing.T) {
				classification := classifier.ClassifyError(provider, 404, []byte(body))

				if classification.Category != ErrorCategoryFailover {
					t.Errorf("provider=%s body=%s: expected failover, got %s (type=%s)",
						provider, body, classification.Category, classification.Type)
				}
				if !classification.ShouldFailover {
					t.Errorf("provider=%s body=%s: expected shouldFailover=true", provider, body)
				}
			})
		}
	}
}