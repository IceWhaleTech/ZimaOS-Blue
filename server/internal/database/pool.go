package database

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

// PoolManager manages database connection pool configuration and monitoring.
type PoolManager struct {
	db     *sql.DB
	config PoolConfig
	logger zerolog.Logger

	// Health check
	stopCh   chan struct{}
	wg       sync.WaitGroup
	running  bool
	runningMu sync.Mutex

	// Statistics
	stats   *PoolStats
	healthy atomic.Bool
}

// PoolStats holds connection pool statistics.
type PoolStats struct {
	MaxOpenConnections int           `json:"max_open_connections"`
	OpenConnections    int           `json:"open_connections"`
	InUse              int           `json:"in_use"`
	Idle               int           `json:"idle"`
	WaitCount          int64         `json:"wait_count"`
	WaitDuration       time.Duration `json:"wait_duration"`
	MaxIdleClosed      int64         `json:"max_idle_closed"`
	MaxIdleTimeClosed  int64         `json:"max_idle_time_closed"`
	MaxLifetimeClosed  int64         `json:"max_lifetime_closed"`
	HealthChecksPassed int64         `json:"health_checks_passed"`
	HealthChecksFailed int64         `json:"health_checks_failed"`
	LastHealthCheck    time.Time     `json:"last_health_check"`
}

// NewPoolManager creates a new pool manager.
func NewPoolManager(db *sql.DB, config PoolConfig, logger zerolog.Logger) *PoolManager {
	pm := &PoolManager{
		db:     db,
		config: config,
		logger: logger.With().Str("component", "pool-manager").Logger(),
		stopCh: make(chan struct{}),
		stats:  &PoolStats{},
	}
	pm.healthy.Store(true)
	return pm
}

// Configure applies pool configuration to the database.
func (p *PoolManager) Configure() {
	p.db.SetMaxOpenConns(p.config.MaxOpenConns)
	p.db.SetMaxIdleConns(p.config.MaxIdleConns)
	p.db.SetConnMaxLifetime(p.config.ConnMaxLifetime)
	p.db.SetConnMaxIdleTime(p.config.ConnMaxIdleTime)

	p.logger.Info().
		Int("max_open_conns", p.config.MaxOpenConns).
		Int("max_idle_conns", p.config.MaxIdleConns).
		Dur("conn_max_lifetime", p.config.ConnMaxLifetime).
		Dur("conn_max_idle_time", p.config.ConnMaxIdleTime).
		Msg("Connection pool configured")
}

// StartHealthCheck starts periodic health checks.
func (p *PoolManager) StartHealthCheck() {
	p.runningMu.Lock()
	defer p.runningMu.Unlock()

	if p.running {
		return
	}

	p.running = true
	p.wg.Add(1)

	go func() {
		defer p.wg.Done()

		ticker := time.NewTicker(p.config.HealthCheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				healthy := p.checkHealth(ctx)
				p.healthy.Store(healthy)
				cancel()
			case <-p.stopCh:
				return
			}
		}
	}()

	p.logger.Info().
		Dur("interval", p.config.HealthCheckInterval).
		Msg("Health check started")
}

// StopHealthCheck stops periodic health checks.
func (p *PoolManager) StopHealthCheck() {
	p.runningMu.Lock()
	defer p.runningMu.Unlock()

	if !p.running {
		return
	}

	close(p.stopCh)
	p.wg.Wait()
	p.running = false
	p.stopCh = make(chan struct{})

	p.logger.Info().Msg("Health check stopped")
}

// checkHealth performs a health check on the connection pool.
func (p *PoolManager) checkHealth(ctx context.Context) bool {
	start := time.Now()

	// Try to ping the database
	err := p.db.PingContext(ctx)
	if err != nil {
		p.stats.HealthChecksFailed++
		p.logger.Warn().Err(err).Msg("Health check failed")
		return false
	}

	// Try a simple query
	var result int
	err = p.db.QueryRowContext(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		p.stats.HealthChecksFailed++
		p.logger.Warn().Err(err).Msg("Health check query failed")
		return false
	}

	p.stats.HealthChecksPassed++
	p.stats.LastHealthCheck = time.Now()

	p.logger.Debug().
		Dur("duration", time.Since(start)).
		Msg("Health check passed")

	return true
}

// IsHealthy returns whether the pool is healthy.
func (p *PoolManager) IsHealthy() bool {
	return p.healthy.Load()
}

// GetStats returns current pool statistics.
func (p *PoolManager) GetStats() PoolStats {
	dbStats := p.db.Stats()

	stats := *p.stats
	stats.MaxOpenConnections = dbStats.MaxOpenConnections
	stats.OpenConnections = dbStats.OpenConnections
	stats.InUse = dbStats.InUse
	stats.Idle = dbStats.Idle
	stats.WaitCount = dbStats.WaitCount
	stats.WaitDuration = dbStats.WaitDuration
	stats.MaxIdleClosed = dbStats.MaxIdleClosed
	stats.MaxIdleTimeClosed = dbStats.MaxIdleTimeClosed
	stats.MaxLifetimeClosed = dbStats.MaxLifetimeClosed

	return stats
}

// GetUtilization returns the pool utilization percentage.
func (p *PoolManager) GetUtilization() float64 {
	stats := p.db.Stats()
	if stats.MaxOpenConnections == 0 {
		return 0
	}
	return float64(stats.InUse) / float64(stats.MaxOpenConnections) * 100
}

// WaitForConnection waits for a connection to become available.
func (p *PoolManager) WaitForConnection(ctx context.Context) (*sql.Conn, error) {
	conn, err := p.db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	return conn, nil
}

// WithConnection executes a function with a dedicated connection.
func (p *PoolManager) WithConnection(ctx context.Context, fn func(*sql.Conn) error) error {
	conn, err := p.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("failed to get connection: %w", err)
	}
	defer conn.Close()

	return fn(conn)
}

// WithTransaction executes a function within a transaction.
func (p *PoolManager) WithTransaction(ctx context.Context, opts *sql.TxOptions, fn func(*sql.Tx) error) error {
	tx, err := p.db.BeginTx(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			p.logger.Error().Err(rbErr).Msg("Failed to rollback transaction")
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Warmup pre-creates connections to warm up the pool.
func (p *PoolManager) Warmup(ctx context.Context, count int) error {
	if count <= 0 {
		count = p.config.MaxIdleConns
	}

	p.logger.Info().Int("count", count).Msg("Warming up connection pool")

	conns := make([]*sql.Conn, 0, count)
	defer func() {
		for _, conn := range conns {
			conn.Close()
		}
	}()

	for i := 0; i < count; i++ {
		conn, err := p.db.Conn(ctx)
		if err != nil {
			return fmt.Errorf("failed to create connection %d: %w", i, err)
		}

		// Verify connection is working
		if err := conn.PingContext(ctx); err != nil {
			conn.Close()
			return fmt.Errorf("connection %d ping failed: %w", i, err)
		}

		conns = append(conns, conn)
	}

	p.logger.Info().Int("count", len(conns)).Msg("Connection pool warmed up")
	return nil
}

// DrainAndRefresh drains all connections and creates new ones.
func (p *PoolManager) DrainAndRefresh(ctx context.Context) error {
	p.logger.Info().Msg("Draining and refreshing connection pool")

	// Set max connections to 0 to drain
	originalMax := p.config.MaxOpenConns
	p.db.SetMaxOpenConns(0)

	// Wait a bit for connections to close
	time.Sleep(100 * time.Millisecond)

	// Restore max connections
	p.db.SetMaxOpenConns(originalMax)

	// Warmup the pool
	return p.Warmup(ctx, p.config.MaxIdleConns)
}

// RecommendedPoolSize calculates recommended pool size based on workload.
func RecommendedPoolSize(cpuCores int, ioMultiplier float64) int {
	if cpuCores <= 0 {
		cpuCores = 1
	}
	if ioMultiplier <= 0 {
		ioMultiplier = 2.0 // Default for I/O bound workloads
	}

	// Formula: connections = (core_count * 2) + effective_spindle_count
	// For SSDs, effective_spindle_count is typically 1
	// For I/O bound workloads, multiply by ioMultiplier
	recommended := int(float64(cpuCores*2+1) * ioMultiplier)

	// Ensure minimum of 2 and maximum of 100
	if recommended < 2 {
		recommended = 2
	}
	if recommended > 100 {
		recommended = 100
	}

	return recommended
}
