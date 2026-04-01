package bootstrap

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxybridge"
)

const (
	defaultRuntimeLLMMaxRetries     = 2
	defaultRuntimeLLMAttemptTimeout = 45 * time.Second
)

type llmChatInvoker func(context.Context, llm.ChatRequest) (*llm.ChatResponse, error)

type llmChatRetryConfig struct {
	maxRetries     int
	classifier     llm.ErrorClassifier
	sleep          func(context.Context, time.Duration) error
	attemptTimeout time.Duration
}

func defaultLLMChatRetryConfig() llmChatRetryConfig {
	return llmChatRetryConfig{
		maxRetries:     defaultRuntimeLLMMaxRetries,
		classifier:     llm.NewDefaultErrorClassifier(),
		sleep:          sleepWithContext,
		attemptTimeout: defaultRuntimeLLMAttemptTimeout,
	}
}

func (c llmChatRetryConfig) withDefaults() llmChatRetryConfig {
	defaults := defaultLLMChatRetryConfig()
	if c.maxRetries < 0 {
		c.maxRetries = 0
	}
	if c.maxRetries == 0 {
		c.maxRetries = defaults.maxRetries
	}
	if c.classifier == nil {
		c.classifier = defaults.classifier
	}
	if c.sleep == nil {
		c.sleep = defaults.sleep
	}
	if c.attemptTimeout <= 0 {
		c.attemptTimeout = defaults.attemptTimeout
	}
	return c
}

func (c llmChatRetryConfig) chat(ctx context.Context, req llm.ChatRequest, invoke llmChatInvoker) (*llm.ChatResponse, error) {
	cfg := c.withDefaults()
	var lastErr error

	for attempt := 0; attempt <= cfg.maxRetries; attempt++ {
		attemptCtx, cancel := withRetryAttemptTimeout(ctx, cfg.attemptTimeout)
		resp, err := invoke(attemptCtx, req)
		cancel()
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if attempt == cfg.maxRetries || !shouldRetryLLMChatError(cfg.classifier, err) {
			return nil, err
		}
		if err := cfg.sleep(ctx, retryDelayForLLMChatError(cfg.classifier, lastErr, attempt)); err != nil {
			return nil, err
		}
	}

	return nil, lastErr
}

func withRetryAttemptTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

func shouldRetryLLMChatError(classifier llm.ErrorClassifier, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}

	var proxyErr *proxybridge.ProxyError
	if errors.As(err, &proxyErr) {
		if proxyErr.IsNoProvider() {
			return false
		}
		if proxyErr.IsOverloaded() {
			return true
		}
		switch proxyErr.StatusCode {
		case http.StatusRequestTimeout,
			http.StatusTooManyRequests,
			http.StatusInternalServerError,
			http.StatusBadGateway,
			http.StatusServiceUnavailable,
			http.StatusGatewayTimeout,
			529:
			return true
		default:
			return false
		}
	}

	if classifier == nil {
		classifier = llm.NewDefaultErrorClassifier()
	}
	return classifier.ShouldRetry(err)
}

func retryDelayForLLMChatError(classifier llm.ErrorClassifier, err error, attempt int) time.Duration {
	if classifier == nil {
		classifier = llm.NewDefaultErrorClassifier()
	}
	delay := classifier.GetRetryDelay(err, attempt)
	if delay <= 0 {
		delay = time.Second
	}
	return delay
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
