package proxy

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
)

// FailoverMetrics tracks failover statistics
type FailoverMetrics struct {
	mu sync.RWMutex

	// Error counts by type
	ErrorsByType map[RetryableErrorType]int64 `json:"errors_by_type"`

	// Failover counts
	FailoverTotal   int64 `json:"failover_total"`
	FailoverSuccess int64 `json:"failover_success"`
	FailoverFailure int64 `json:"failover_failure"`

	// Provider-specific stats
	ProviderErrors    map[string]map[RetryableErrorType]int64 `json:"provider_errors"`
	ProviderFailovers map[string]int64                        `json:"provider_failovers"`

	// Streaming anomaly stats
	StreamAnomalies int64 `json:"stream_anomalies"`
}

// NewFailoverMetrics creates new metrics tracker
func NewFailoverMetrics() *FailoverMetrics {
	return &FailoverMetrics{
		ErrorsByType:      make(map[RetryableErrorType]int64),
		ProviderErrors:    make(map[string]map[RetryableErrorType]int64),
		ProviderFailovers: make(map[string]int64),
	}
}

// RecordError records an error occurrence
func (m *FailoverMetrics) RecordError(provider string, classification *ErrorClassification) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ErrorsByType[classification.Type]++

	if _, ok := m.ProviderErrors[provider]; !ok {
		m.ProviderErrors[provider] = make(map[RetryableErrorType]int64)
	}
	m.ProviderErrors[provider][classification.Type]++
}

// RecordFailover records a failover event
func (m *FailoverMetrics) RecordFailover(fromProvider, toProvider string, success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.FailoverTotal++
	if success {
		m.FailoverSuccess++
	} else {
		m.FailoverFailure++
	}
	m.ProviderFailovers[fromProvider]++
}

func mapFailoverReasonToErrorType(reason providerpool.FailoverReason) RetryableErrorType {
	switch reason {
	case providerpool.FailoverReasonTimeout:
		return ErrorTypeTimeout
	case providerpool.FailoverReasonRateLimit:
		return ErrorTypeRateLimited
	case providerpool.FailoverReasonAuthError:
		return ErrorTypeAuthFailed
	case providerpool.FailoverReasonModelNotFound:
		return ErrorTypeModelNotFound
	case providerpool.FailoverReasonCooldown, providerpool.FailoverReasonAPIError:
		return ErrorTypeServiceUnavailable
	default:
		return ErrorTypeUnknown
	}
}

// RecordProviderPoolResult maps ProviderPool failover callbacks into smart failover metrics.
func (m *FailoverMetrics) RecordProviderPoolResult(result *providerpool.FailoverResult) {
	if result == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.ErrorsByType == nil {
		m.ErrorsByType = make(map[RetryableErrorType]int64)
	}
	if m.ProviderErrors == nil {
		m.ProviderErrors = make(map[string]map[RetryableErrorType]int64)
	}
	if m.ProviderFailovers == nil {
		m.ProviderFailovers = make(map[string]int64)
	}

	m.FailoverTotal++
	if result.SuccessProvider != "" {
		m.FailoverSuccess++
	} else {
		m.FailoverFailure++
	}

	for _, attempt := range result.FailedAttempts {
		if attempt == nil {
			continue
		}

		provider := attempt.ProviderID
		if provider == "" {
			provider = attempt.ProviderName
		}
		errType := mapFailoverReasonToErrorType(attempt.Reason)

		m.ErrorsByType[errType]++
		if provider == "" {
			continue
		}

		if _, ok := m.ProviderErrors[provider]; !ok {
			m.ProviderErrors[provider] = make(map[RetryableErrorType]int64)
		}
		m.ProviderErrors[provider][errType]++
		m.ProviderFailovers[provider]++
	}
}

// RecordStreamAnomaly records a streaming anomaly
func (m *FailoverMetrics) RecordStreamAnomaly() {
	atomic.AddInt64(&m.StreamAnomalies, 1)
}

// GetStats returns a copy of current stats
func (m *FailoverMetrics) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"errors_by_type":     m.ErrorsByType,
		"failover_total":     m.FailoverTotal,
		"failover_success":   m.FailoverSuccess,
		"failover_failure":   m.FailoverFailure,
		"provider_errors":    m.ProviderErrors,
		"provider_failovers": m.ProviderFailovers,
		"stream_anomalies":   atomic.LoadInt64(&m.StreamAnomalies),
	}
}

// SmartFailoverHandler extends FailoverHandler with intelligent error classification
type SmartFailoverHandler struct {
	*FailoverHandler
	classifier      *APIErrorClassifier
	anomalyDetector *StreamingAnomalyDetector
	metrics         *FailoverMetrics
	config          *FailoverConfig
}

// NewSmartFailoverHandler creates a new smart failover handler
func NewSmartFailoverHandler(config *FailoverConfig, router *Router) *SmartFailoverHandler {
	baseHandler := NewFailoverHandler(config, router)

	var anomalyDetector *StreamingAnomalyDetector
	if config.StreamingAnomaly.Enabled {
		anomalyDetector = NewStreamingAnomalyDetector(config.StreamingAnomaly)
	}

	return &SmartFailoverHandler{
		FailoverHandler: baseHandler,
		classifier:      NewAPIErrorClassifier(),
		anomalyDetector: anomalyDetector,
		metrics:         NewFailoverMetrics(),
		config:          config,
	}
}

// ExecuteWithSmartFailover executes request with intelligent failover
func (sfh *SmartFailoverHandler) ExecuteWithSmartFailover(
	ctx context.Context,
	provider *Provider,
	fn func(*Provider) (*http.Response, error),
) (*http.Response, error) {
	if !sfh.config.Enabled {
		return fn(provider)
	}

	// Execute request
	resp, err := fn(provider)

	// Check for errors that need classification
	if err != nil || (resp != nil && resp.StatusCode >= 400) {
		classification := sfh.classifyResponse(provider.Config.Name, resp, err)

		if classification != nil {
			// Log the error classification for debugging
			slog.Info("[proxy] error classified",
				"provider", provider.Config.Name,
				"status", classification.OriginalStatusCode,
				"type", classification.Type,
				"category", classification.Category,
				"message", classification.Message)

			sfh.metrics.RecordError(provider.Config.Name, classification)

			// Handle based on classification
			switch classification.Category {
			case ErrorCategoryFailover:
				return sfh.handleFailover(ctx, provider, classification, fn)

			case ErrorCategoryRetryable:
				return sfh.handleRetry(ctx, provider, classification, fn)
			}
		}
	}

	return resp, err
}

// classifyResponse classifies the response error
func (sfh *SmartFailoverHandler) classifyResponse(
	providerName string,
	resp *http.Response,
	err error,
) *ErrorClassification {
	if err != nil {
		// Network or timeout error
		return &ErrorClassification{
			Type:           ErrorTypeTimeout,
			Category:       ErrorCategoryRetryable,
			Message:        err.Error(),
			Retryable:      true,
			ShouldFailover: false,
		}
	}

	if resp == nil || resp.StatusCode < 400 {
		return nil
	}

	// Read response body for classification
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		body = []byte{}
	}
	// Restore body for potential re-reading
	resp.Body = io.NopCloser(bytes.NewReader(body))

	return sfh.classifier.ClassifyError(providerName, resp.StatusCode, body)
}

// handleFailover handles failover to alternative provider
func (sfh *SmartFailoverHandler) handleFailover(
	ctx context.Context,
	failedProvider *Provider,
	classification *ErrorClassification,
	fn func(*Provider) (*http.Response, error),
) (*http.Response, error) {
	// Find alternative provider
	altProvider := sfh.selectAlternativeProvider(failedProvider, classification)
	if altProvider == nil {
		sfh.metrics.RecordFailover(failedProvider.Config.Name, "", false)
		return nil, ErrAllProvidersFailed
	}

	// Try alternative provider
	resp, err := fn(altProvider)
	success := err == nil && (resp == nil || resp.StatusCode < 400)
	sfh.metrics.RecordFailover(failedProvider.Config.Name, altProvider.Config.Name, success)

	if !success {
		// Try more providers
		return sfh.tryRemainingProviders(ctx, failedProvider, altProvider, fn)
	}

	return resp, err
}

// handleRetry handles retry with same provider
func (sfh *SmartFailoverHandler) handleRetry(
	ctx context.Context,
	provider *Provider,
	classification *ErrorClassification,
	fn func(*Provider) (*http.Response, error),
) (*http.Response, error) {
	// Wait before retry
	delay := sfh.config.RetryDelay
	if classification.RetryAfter > 0 {
		delay = classification.RetryAfter
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(delay):
	}

	// Retry with same provider
	return fn(provider)
}

// selectAlternativeProvider selects best alternative based on error type
func (sfh *SmartFailoverHandler) selectAlternativeProvider(
	failed *Provider,
	classification *ErrorClassification,
) *Provider {
	providers := sfh.router.GetAvailableProviders()

	for _, p := range providers {
		if p.Config.Name == failed.Config.Name {
			continue
		}

		// Skip providers with open circuit breakers
		if sfh.config.CircuitBreaker {
			breaker := sfh.getBreaker(p.Config.Name)
			if breaker != nil && breaker.State().String() == "open" {
				continue
			}
		}

		// For context errors, check if provider supports larger context
		if classification.Type == ErrorTypeContextTooLong && sfh.config.ContextWindowCheck {
			maxCtx := estimateMaxContext(p.Config.Name)
			if override, ok := sfh.config.ContextWindowOverride[p.Config.Name]; ok {
				maxCtx = override
			}
			if classification.SuggestedContextWindow > 0 && maxCtx < classification.SuggestedContextWindow {
				continue
			}
			return p
		}

		// For quota errors, prefer providers that haven't errored recently
		if classification.Type == ErrorTypeQuotaExceeded && sfh.config.QuotaCooldown > 0 {
			if p.LastError != nil && time.Since(p.LastCheck) < sfh.config.QuotaCooldown {
				continue
			}
			return p
		}

		// Default: return first available provider
		return p
	}

	return nil
}

// tryRemainingProviders tries remaining providers after first failover failed
func (sfh *SmartFailoverHandler) tryRemainingProviders(
	ctx context.Context,
	originalProvider *Provider,
	firstAltProvider *Provider,
	fn func(*Provider) (*http.Response, error),
) (*http.Response, error) {
	providers := sfh.router.GetAvailableProviders()
	tried := map[string]bool{
		originalProvider.Config.Name: true,
		firstAltProvider.Config.Name: true,
	}

	for _, p := range providers {
		if tried[p.Config.Name] {
			continue
		}
		tried[p.Config.Name] = true

		resp, err := fn(p)
		if err == nil && (resp == nil || resp.StatusCode < 400) {
			sfh.metrics.RecordFailover(originalProvider.Config.Name, p.Config.Name, true)
			return resp, nil
		}
	}

	return nil, ErrAllProvidersFailed
}

// ExecuteStreamingWithAnomalyDetection executes streaming request with anomaly detection
func (sfh *SmartFailoverHandler) ExecuteStreamingWithAnomalyDetection(
	ctx context.Context,
	provider *Provider,
	fn func(*Provider) (*http.Response, error),
	onAnomaly func(*StreamAnomaly, *StreamRecoveryStrategy),
) (*http.Response, error) {
	if sfh.anomalyDetector == nil {
		return sfh.ExecuteWithSmartFailover(ctx, provider, fn)
	}

	resp, err := fn(provider)
	if err != nil {
		return resp, err
	}

	// Wrap response body with anomaly detection
	if resp != nil && resp.Body != nil && isStreamingResponse(resp) {
		streamBuffer := NewStreamBuffer(sfh.anomalyDetector)
		resp.Body = &anomalyDetectingReader{
			reader:        resp.Body,
			buffer:        streamBuffer,
			onAnomaly:     onAnomaly,
			metrics:       sfh.metrics,
			anomalyConfig: sfh.config.StreamingAnomaly,
		}
	}

	return resp, nil
}

// anomalyDetectingReader wraps a reader to detect streaming anomalies
type anomalyDetectingReader struct {
	reader        io.ReadCloser
	buffer        *StreamBuffer
	onAnomaly     func(*StreamAnomaly, *StreamRecoveryStrategy)
	metrics       *FailoverMetrics
	anomalyConfig StreamingAnomalyConfig
	closed        bool
}

func (r *anomalyDetectingReader) Read(p []byte) (n int, err error) {
	if r.closed {
		return 0, io.EOF
	}

	n, err = r.reader.Read(p)
	if n > 0 {
		// Check for anomalies
		anomaly, _ := r.buffer.Write(p[:n])
		if anomaly != nil {
			r.metrics.RecordStreamAnomaly()
			if r.onAnomaly != nil {
				strategy := GetRecoveryStrategy(anomaly, r.anomalyConfig)
				r.onAnomaly(anomaly, strategy)

				// If strategy says force stop, close the reader
				if strategy != nil && strategy.ForceStop {
					r.closed = true
					r.reader.Close()
					return n, io.EOF
				}
			}
		}
	}

	return n, err
}

func (r *anomalyDetectingReader) Close() error {
	r.closed = true
	return r.reader.Close()
}

// GetMetrics returns failover metrics
func (sfh *SmartFailoverHandler) GetMetrics() *FailoverMetrics {
	return sfh.metrics
}

// GetClassifier returns the error classifier
func (sfh *SmartFailoverHandler) GetClassifier() *APIErrorClassifier {
	return sfh.classifier
}

// estimateMaxContext returns estimated max context tokens for a provider by name.
func estimateMaxContext(providerName string) int {
	name := strings.ToLower(providerName)
	switch {
	case strings.Contains(name, "claude"):
		return 200000
	case strings.Contains(name, "gpt-4o"), strings.Contains(name, "gpt-4-turbo"):
		return 128000
	case strings.Contains(name, "gpt-4"):
		return 8192
	case strings.Contains(name, "gpt-3.5"):
		return 16385
	case strings.Contains(name, "deepseek"):
		return 128000
	case strings.Contains(name, "qwen"):
		return 32768
	default:
		return 8192
	}
}
