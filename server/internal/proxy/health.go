package proxy

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// HealthChecker performs health checks on providers
type HealthChecker struct {
	router   *Router
	connPool *ConnectionPool
	config   *HealthCheckConfig

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(router *Router, connPool *ConnectionPool, config *HealthCheckConfig) *HealthChecker {
	ctx, cancel := context.WithCancel(context.Background())
	return &HealthChecker{
		router:   router,
		connPool: connPool,
		config:   config,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start starts the health check loop
func (hc *HealthChecker) Start() {
	if !hc.config.Enabled {
		return
	}

	hc.wg.Add(1)
	go hc.run()
}

// Stop stops the health checker
func (hc *HealthChecker) Stop() {
	hc.cancel()
	hc.wg.Wait()
}

// run is the main health check loop
func (hc *HealthChecker) run() {
	defer hc.wg.Done()

	ticker := time.NewTicker(hc.config.Interval)
	defer ticker.Stop()

	// Initial check
	hc.checkAll()

	for {
		select {
		case <-hc.ctx.Done():
			return
		case <-ticker.C:
			hc.checkAll()
		}
	}
}

// checkAll checks all providers
func (hc *HealthChecker) checkAll() {
	providers := hc.router.GetAllProviders()

	var wg sync.WaitGroup
	for _, p := range providers {
		if !p.Config.Enabled {
			continue
		}

		wg.Add(1)
		go func(provider *Provider) {
			defer wg.Done()
			hc.checkProvider(provider)
		}(p)
	}
	wg.Wait()
}

// checkProvider checks a single provider
func (hc *HealthChecker) checkProvider(provider *Provider) {
	if provider.Config.HealthCheck == "" {
		// No health check endpoint configured, assume healthy
		return
	}

	ctx, cancel := context.WithTimeout(hc.ctx, hc.config.Timeout)
	defer cancel()

	url := provider.Config.Endpoint + provider.Config.HealthCheck
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		hc.router.UpdateHealth(provider.Config.Name, false, err)
		return
	}

	// Add API key if required
	if provider.Config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+provider.Config.APIKey)
	}

	client := hc.connPool.GetClient(provider.Config.Name)
	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)

	if err != nil {
		hc.router.UpdateHealthWithLatency(provider.Config.Name, false, err, latency)
		return
	}
	defer resp.Body.Close()

	healthy := resp.StatusCode >= 200 && resp.StatusCode < 300
	hc.router.UpdateHealthWithLatency(provider.Config.Name, healthy, nil, latency)
}

// CheckNow performs an immediate health check on all providers
func (hc *HealthChecker) CheckNow() {
	hc.checkAll()
}

// CheckProvider performs an immediate health check on a specific provider
func (hc *HealthChecker) CheckProvider(name string) error {
	provider, ok := hc.router.GetProvider(name)
	if !ok {
		return ErrProviderNotFound
	}

	hc.checkProvider(provider)
	return nil
}
