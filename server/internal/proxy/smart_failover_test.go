package proxy

import (
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
)

func TestAPIErrorClassifier_ClassifyError(t *testing.T) {
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
			name:           "Anthropic context too long",
			provider:       "anthropic",
			statusCode:     400,
			responseBody:   `{"error":{"type":"invalid_request_error","message":"Request context size (202154 tokens) exceeds maximum allowed (200000 tokens)"}}`,
			expectedType:   ErrorTypeContextTooLong,
			expectedCat:    ErrorCategoryFailover,
			shouldFailover: true,
		},
		{
			name:           "Anthropic rate limited",
			provider:       "anthropic",
			statusCode:     429,
			responseBody:   `{"error":{"type":"rate_limit_error","message":"Rate limit exceeded"}}`,
			expectedType:   ErrorTypeRateLimited,
			expectedCat:    ErrorCategoryFailover,
			shouldFailover: true,
		},
		{
			name:           "Anthropic quota exceeded",
			provider:       "anthropic",
			statusCode:     400,
			responseBody:   `{"error":{"type":"invalid_request_error","message":"Your credit balance is too low"}}`,
			expectedType:   ErrorTypeQuotaExceeded,
			expectedCat:    ErrorCategoryFailover,
			shouldFailover: true,
		},
		{
			name:           "Anthropic overloaded",
			provider:       "anthropic",
			statusCode:     529,
			responseBody:   `{"error":{"type":"overloaded_error","message":"Overloaded"}}`,
			expectedType:   ErrorTypeModelOverloaded,
			expectedCat:    ErrorCategoryFailover,
			shouldFailover: true,
		},
		{
			name:           "Anthropic auth failed",
			provider:       "anthropic",
			statusCode:     401,
			responseBody:   `{"error":{"type":"authentication_error","message":"Invalid API key"}}`,
			expectedType:   ErrorTypeAuthFailed,
			expectedCat:    ErrorCategoryNonRetryable,
			shouldFailover: false,
		},
		{
			name:           "OpenAI context too long",
			provider:       "openai",
			statusCode:     400,
			responseBody:   `{"error":{"message":"This model's maximum context length is 128000 tokens. However, your messages resulted in 150000 tokens.","type":"invalid_request_error","code":"context_length_exceeded"}}`,
			expectedType:   ErrorTypeContextTooLong,
			expectedCat:    ErrorCategoryFailover,
			shouldFailover: true,
		},
		{
			name:           "OpenAI rate limited",
			provider:       "openai",
			statusCode:     429,
			responseBody:   `{"error":{"message":"Rate limit exceeded","type":"rate_limit_exceeded"}}`,
			expectedType:   ErrorTypeRateLimited,
			expectedCat:    ErrorCategoryFailover,
			shouldFailover: true,
		},
		{
			name:           "OpenAI quota exceeded",
			provider:       "openai",
			statusCode:     429,
			responseBody:   `{"error":{"message":"You exceeded your current quota","type":"insufficient_quota"}}`,
			expectedType:   ErrorTypeQuotaExceeded,
			expectedCat:    ErrorCategoryFailover,
			shouldFailover: true,
		},
		{
			name:           "DeepSeek context too long",
			provider:       "deepseek",
			statusCode:     400,
			responseBody:   `{"error":{"message":"Context is too long, please reduce the input"}}`,
			expectedType:   ErrorTypeContextTooLong,
			expectedCat:    ErrorCategoryFailover,
			shouldFailover: true,
		},
		{
			name:           "Generic 500 error",
			provider:       "unknown",
			statusCode:     500,
			responseBody:   `{"error":{"message":"Internal server error"}}`,
			expectedType:   ErrorTypeServiceUnavailable,
			expectedCat:    ErrorCategoryFailover,
			shouldFailover: true,
		},
		{
			name:           "Relay wrapped context window full",
			provider:       "unknown",
			statusCode:     502,
			responseBody:   `{"error":{"message":"Context window is full. Reduce conversation history, system prompt, or tools."}}`,
			expectedType:   ErrorTypeContextTooLong,
			expectedCat:    ErrorCategoryFailover,
			shouldFailover: true,
		},
		{
			name:           "Generic 503 error",
			provider:       "unknown",
			statusCode:     503,
			responseBody:   `{"error":{"message":"Service temporarily unavailable"}}`,
			expectedType:   ErrorTypeServiceUnavailable,
			expectedCat:    ErrorCategoryFailover,
			shouldFailover: true,
		},
		{
			name:           "Generic 404 error",
			provider:       "unknown",
			statusCode:     404,
			responseBody:   `{"error":{"message":"Model not found"}}`,
			expectedType:   ErrorTypeModelNotFound,
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

func TestAPIErrorClassifier_InitializesPatternsOnDemand(t *testing.T) {
	classifier := NewAPIErrorClassifier()
	if classifier.patterns != nil {
		t.Fatal("expected classifier patterns to start nil")
	}

	classification := classifier.ClassifyError(
		"anthropic",
		400,
		[]byte(`{"error":{"message":"Request context size (202154 tokens) exceeds maximum allowed (200000 tokens)"}}`),
	)
	if classification == nil {
		t.Fatal("expected classification result")
	}
	if classification.Type != ErrorTypeContextTooLong {
		t.Fatalf("classification.Type = %s, want %s", classification.Type, ErrorTypeContextTooLong)
	}
	if len(classifier.patterns) == 0 {
		t.Fatal("expected classifier patterns to initialize on first classify")
	}
}

func TestAPIErrorClassifier_ExtractContextSize(t *testing.T) {
	classifier := NewAPIErrorClassifier()

	tests := []struct {
		name         string
		provider     string
		responseBody string
		expectedSize int
	}{
		{
			name:         "Anthropic format",
			provider:     "anthropic",
			responseBody: `{"error":{"message":"Request context size (202154 tokens) exceeds maximum allowed (200000 tokens)"}}`,
			expectedSize: 202154,
		},
		{
			name:         "OpenAI format",
			provider:     "openai",
			responseBody: `{"error":{"message":"This model's maximum context length is 128000 tokens. However, your messages resulted in 150000 tokens."}}`,
			expectedSize: 150000,
		},
		{
			name:         "No token count",
			provider:     "anthropic",
			responseBody: `{"error":{"message":"Context is too long"}}`,
			expectedSize: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			classification := classifier.ClassifyError(tt.provider, 400, []byte(tt.responseBody))

			if classification.SuggestedContextWindow != tt.expectedSize {
				t.Errorf("expected context size %d, got %d", tt.expectedSize, classification.SuggestedContextWindow)
			}
		})
	}
}

func TestStreamingAnomalyDetector_DetectRepetition(t *testing.T) {
	config := DefaultStreamingAnomalyConfig()
	config.MinPatternLength = 10
	config.RepeatThreshold = 3

	detector := NewStreamingAnomalyDetector(config)
	buffer := NewStreamBuffer(detector)

	tests := []struct {
		name           string
		chunks         []string
		expectAnomaly  bool
		expectedRepeat int
	}{
		{
			name: "No repetition",
			chunks: []string{
				"Hello, this is a normal response.",
				" It contains different content.",
				" And more unique text here.",
			},
			expectAnomaly: false,
		},
		{
			name: "Repetitive pattern",
			chunks: []string{
				"This is repeating content.",
				"This is repeating content.",
				"This is repeating content.",
				"This is repeating content.",
			},
			expectAnomaly:  true,
			expectedRepeat: 3,
		},
		{
			name: "Short repetition (below threshold)",
			chunks: []string{
				"Short.",
				"Short.",
			},
			expectAnomaly: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buffer.Reset()

			var lastAnomaly *StreamAnomaly
			for _, chunk := range tt.chunks {
				anomaly, _ := buffer.Write([]byte(chunk))
				if anomaly != nil {
					lastAnomaly = anomaly
				}
			}

			if tt.expectAnomaly && lastAnomaly == nil {
				t.Error("expected anomaly but got none")
			}
			if !tt.expectAnomaly && lastAnomaly != nil {
				t.Errorf("expected no anomaly but got: %v", lastAnomaly)
			}
			if tt.expectAnomaly && lastAnomaly != nil && lastAnomaly.RepeatCount < tt.expectedRepeat {
				t.Errorf("expected at least %d repeats, got %d", tt.expectedRepeat, lastAnomaly.RepeatCount)
			}
		})
	}
}

func TestStreamBuffer_GetValidContent(t *testing.T) {
	config := DefaultStreamingAnomalyConfig()
	config.MinPatternLength = 10
	config.RepeatThreshold = 3

	detector := NewStreamingAnomalyDetector(config)
	buffer := NewStreamBuffer(detector)

	// Write some valid content
	buffer.Write([]byte("This is valid content. "))

	// Write repetitive content
	for i := 0; i < 5; i++ {
		buffer.Write([]byte("Repeating text here."))
	}

	if !buffer.HasAnomaly() {
		t.Error("expected anomaly to be detected")
	}

	validContent := buffer.GetValidContent()
	if len(validContent) == 0 {
		t.Error("expected some valid content")
	}
}

func TestGetRecoveryStrategy(t *testing.T) {
	config := DefaultStreamingAnomalyConfig()

	tests := []struct {
		name             string
		anomaly          *StreamAnomaly
		recoveryStrategy string
		expectForceStop  bool
		expectTruncate   bool
	}{
		{
			name: "Truncate and retry",
			anomaly: &StreamAnomaly{
				Type:        ErrorTypeRepetitiveOutput,
				Pattern:     "repeating pattern",
				RepeatCount: 5,
			},
			recoveryStrategy: "truncate_and_retry",
			expectForceStop:  true,
			expectTruncate:   true,
		},
		{
			name: "Failover",
			anomaly: &StreamAnomaly{
				Type:        ErrorTypeRepetitiveOutput,
				Pattern:     "repeating pattern",
				RepeatCount: 5,
			},
			recoveryStrategy: "failover",
			expectForceStop:  true,
			expectTruncate:   false,
		},
		{
			name: "Stop only",
			anomaly: &StreamAnomaly{
				Type:        ErrorTypeRepetitiveOutput,
				Pattern:     "repeating pattern",
				RepeatCount: 5,
			},
			recoveryStrategy: "stop",
			expectForceStop:  true,
			expectTruncate:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.RecoveryStrategy = tt.recoveryStrategy
			strategy := GetRecoveryStrategy(tt.anomaly, config)

			if strategy.ForceStop != tt.expectForceStop {
				t.Errorf("expected ForceStop=%v, got %v", tt.expectForceStop, strategy.ForceStop)
			}
			if strategy.RetryWithTruncation != tt.expectTruncate {
				t.Errorf("expected RetryWithTruncation=%v, got %v", tt.expectTruncate, strategy.RetryWithTruncation)
			}
		})
	}
}

func TestFailoverMetrics(t *testing.T) {
	metrics := NewFailoverMetrics()

	// Record some errors
	metrics.RecordError("anthropic", &ErrorClassification{
		Type:     ErrorTypeContextTooLong,
		Category: ErrorCategoryFailover,
	})
	metrics.RecordError("anthropic", &ErrorClassification{
		Type:     ErrorTypeRateLimited,
		Category: ErrorCategoryFailover,
	})
	metrics.RecordError("openai", &ErrorClassification{
		Type:     ErrorTypeQuotaExceeded,
		Category: ErrorCategoryFailover,
	})

	// Record failovers
	metrics.RecordFailover("anthropic", "openai", true)
	metrics.RecordFailover("openai", "deepseek", false)

	// Record stream anomaly
	metrics.RecordStreamAnomaly()

	stats := metrics.GetStats()

	if stats["failover_total"].(int64) != 2 {
		t.Errorf("expected 2 total failovers, got %v", stats["failover_total"])
	}
	if stats["failover_success"].(int64) != 1 {
		t.Errorf("expected 1 successful failover, got %v", stats["failover_success"])
	}
	if stats["stream_anomalies"].(int64) != 1 {
		t.Errorf("expected 1 stream anomaly, got %v", stats["stream_anomalies"])
	}
}

func TestFailoverMetrics_RecordProviderPoolResult(t *testing.T) {
	metrics := NewFailoverMetrics()

	metrics.RecordProviderPoolResult(&providerpool.FailoverResult{
		SuccessProvider: "provider-b",
		FailedAttempts: []*providerpool.FailoverRecord{
			{ProviderID: "provider-a", Reason: providerpool.FailoverReasonRateLimit},
			{ProviderID: "provider-c", Reason: providerpool.FailoverReasonAuthError},
		},
	})

	stats := metrics.GetStats()

	if stats["failover_total"].(int64) != 1 {
		t.Errorf("expected 1 total failover, got %v", stats["failover_total"])
	}
	if stats["failover_success"].(int64) != 1 {
		t.Errorf("expected 1 successful failover, got %v", stats["failover_success"])
	}
	if stats["failover_failure"].(int64) != 0 {
		t.Errorf("expected 0 failed failovers, got %v", stats["failover_failure"])
	}

	byProvider := stats["provider_failovers"].(map[string]int64)
	if byProvider["provider-a"] != 1 {
		t.Errorf("expected provider-a failovers=1, got %v", byProvider["provider-a"])
	}
	if byProvider["provider-c"] != 1 {
		t.Errorf("expected provider-c failovers=1, got %v", byProvider["provider-c"])
	}

	byType := stats["errors_by_type"].(map[RetryableErrorType]int64)
	if byType[ErrorTypeRateLimited] != 1 {
		t.Errorf("expected rate_limited=1, got %v", byType[ErrorTypeRateLimited])
	}
	if byType[ErrorTypeAuthFailed] != 1 {
		t.Errorf("expected auth_failed=1, got %v", byType[ErrorTypeAuthFailed])
	}
}
