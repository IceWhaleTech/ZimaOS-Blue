package ngrok

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// RenewalService handles automatic tunnel renewal before expiry.
type RenewalService struct {
	tunnelManager    *TunnelManager
	checkInterval    time.Duration
	renewalThreshold time.Duration
	logger           *zap.Logger

	mu        sync.RWMutex
	running   bool
	stopChan  chan struct{}
	ctx       context.Context
	cancelFn  context.CancelFunc

	// Callbacks
	OnRenewal func(oldURL, newURL string)
	OnError   func(err error)
}

// NewRenewalService creates a new renewal service.
func NewRenewalService(tm *TunnelManager) *RenewalService {
	return &RenewalService{
		tunnelManager:    tm,
		checkInterval:    30 * time.Minute,
		renewalThreshold: 1 * time.Hour,
		logger:           zap.NewNop(),
	}
}

// SetLogger sets the logger for the renewal service.
func (rs *RenewalService) SetLogger(logger *zap.Logger) {
	rs.logger = logger
}

// SetCheckInterval sets the interval between renewal checks.
func (rs *RenewalService) SetCheckInterval(interval time.Duration) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.checkInterval = interval
}

// SetRenewalThreshold sets the time before expiry to trigger renewal.
func (rs *RenewalService) SetRenewalThreshold(threshold time.Duration) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.renewalThreshold = threshold
}

// Start starts the renewal service.
func (rs *RenewalService) Start(ctx context.Context) {
	rs.mu.Lock()
	if rs.running {
		rs.mu.Unlock()
		return
	}

	rs.running = true
	rs.stopChan = make(chan struct{})
	rs.ctx, rs.cancelFn = context.WithCancel(ctx)
	rs.mu.Unlock()

	go rs.runLoop()
}

// Stop stops the renewal service.
func (rs *RenewalService) Stop() {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if !rs.running {
		return
	}

	rs.running = false
	if rs.cancelFn != nil {
		rs.cancelFn()
	}
	close(rs.stopChan)
}

// IsRunning returns true if the renewal service is running.
func (rs *RenewalService) IsRunning() bool {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.running
}

// ShouldRenew returns true if the tunnel should be renewed.
func (rs *RenewalService) ShouldRenew(expiresAt time.Time) bool {
	rs.mu.RLock()
	threshold := rs.renewalThreshold
	rs.mu.RUnlock()

	remaining := time.Until(expiresAt)
	return remaining <= threshold
}

// runLoop is the main loop that checks for renewal.
func (rs *RenewalService) runLoop() {
	rs.mu.RLock()
	interval := rs.checkInterval
	rs.mu.RUnlock()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-rs.stopChan:
			return
		case <-rs.ctx.Done():
			return
		case <-ticker.C:
			rs.checkAndRenew()
		}
	}
}

// checkAndRenew checks if renewal is needed and performs it.
func (rs *RenewalService) checkAndRenew() {
	// Check if tunnel is running
	if !rs.tunnelManager.IsRunning() {
		return
	}

	status := rs.tunnelManager.GetStatus()
	if !status.Active {
		return
	}

	// Check if renewal is needed
	if !rs.ShouldRenew(status.ExpiresAt) {
		rs.logger.Debug("Renewal not needed",
			zap.Time("expires_at", status.ExpiresAt),
			zap.Duration("remaining", time.Until(status.ExpiresAt)),
		)
		return
	}

	rs.logger.Info("Starting tunnel renewal",
		zap.String("old_url", status.URL),
		zap.Time("expires_at", status.ExpiresAt),
	)

	// Perform renewal
	if err := rs.Renew(rs.ctx); err != nil {
		rs.logger.Error("Renewal failed", zap.Error(err))
		if rs.OnError != nil {
			rs.OnError(err)
		}
		return
	}

	rs.logger.Info("Tunnel renewed successfully")
}

// Renew performs the tunnel renewal.
func (rs *RenewalService) Renew(ctx context.Context) error {
	oldStatus := rs.tunnelManager.GetStatus()
	oldURL := oldStatus.URL

	// Get current port and authtoken from tunnel manager
	rs.tunnelManager.mu.RLock()
	port := rs.tunnelManager.port
	authtoken := rs.tunnelManager.authtoken
	rs.tunnelManager.mu.RUnlock()

	// Stop the old tunnel
	if err := rs.tunnelManager.Stop(); err != nil {
		return err
	}

	// Start a new tunnel
	if err := rs.tunnelManager.Start(ctx, port, authtoken); err != nil {
		return err
	}

	// Wait for the new URL
	var newURL string
	for i := 0; i < 30; i++ { // Wait up to 30 seconds
		time.Sleep(1 * time.Second)
		newStatus := rs.tunnelManager.GetStatus()
		if newStatus.URL != "" {
			newURL = newStatus.URL
			break
		}
	}

	// Increment renewal count
	rs.tunnelManager.IncrementRenewedCount()

	// Call the renewal callback
	if rs.OnRenewal != nil && newURL != "" {
		rs.OnRenewal(oldURL, newURL)
	}

	return nil
}

// GetNextRenewalTime returns the estimated next renewal time.
func (rs *RenewalService) GetNextRenewalTime() time.Time {
	if !rs.tunnelManager.IsRunning() {
		return time.Time{}
	}

	status := rs.tunnelManager.GetStatus()
	if !status.Active {
		return time.Time{}
	}

	rs.mu.RLock()
	threshold := rs.renewalThreshold
	rs.mu.RUnlock()

	return status.ExpiresAt.Add(-threshold)
}
