package metrics

import (
	"time"
)

// CallStats represents API call statistics.
type CallStats struct {
	// Call counts
	TotalCalls      int64 `json:"total_calls"`
	SuccessfulCalls int64 `json:"successful_calls"`
	FailedCalls     int64 `json:"failed_calls"`

	// By error type
	TimeoutCalls   int64 `json:"timeout_calls"`
	RateLimitCalls int64 `json:"rate_limit_calls"`
	ErrorCalls     int64 `json:"error_calls"`

	// Success rate
	SuccessRate float64 `json:"success_rate"`
}

// ModelStats represents statistics for a specific model.
type ModelStats struct {
	Model           string  `json:"model"`
	Calls           int64   `json:"calls"`
	SuccessfulCalls int64   `json:"successful_calls"`
	FailedCalls     int64   `json:"failed_calls"`
	SuccessRate     float64 `json:"success_rate"`

	// Latency statistics (ms)
	AvgLatency float64 `json:"avg_latency_ms"`
	P50Latency float64 `json:"p50_latency_ms"`
	P95Latency float64 `json:"p95_latency_ms"`
	P99Latency float64 `json:"p99_latency_ms"`

	// Token statistics
	InputTokens      int64 `json:"input_tokens"`
	OutputTokens     int64 `json:"output_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
	CacheReadTokens  int64 `json:"cache_read_tokens"`
	CacheWriteTokens int64 `json:"cache_write_tokens"`

	// Performance statistics
	AvgTokensPerSecond  float64 `json:"avg_tokens_per_second"`
	AvgTimeToFirstToken float64 `json:"avg_ttft_ms"`

	// Cost statistics
	EstimatedCost float64 `json:"estimated_cost"`
}

// ModelStatsHistory represents historical statistics for a model.
type ModelStatsHistory struct {
	Model      string           `json:"model"`
	Period     string           `json:"period"` // hourly, daily, monthly
	DataPoints []ModelDataPoint `json:"data_points"`
}

// ModelDataPoint represents a single data point in model statistics history.
type ModelDataPoint struct {
	Timestamp     time.Time `json:"timestamp"`
	Calls         int64     `json:"calls"`
	SuccessRate   float64   `json:"success_rate"`
	AvgLatency    float64   `json:"avg_latency_ms"`
	TotalTokens   int64     `json:"total_tokens"`
	EstimatedCost float64   `json:"estimated_cost"`
}

// ModelComparison represents a comparison between models.
type ModelComparison struct {
	Period         string               `json:"period"`
	Models         []ModelStats         `json:"models"`
	Recommendation *ModelRecommendation `json:"recommendation,omitempty"`
	Insights       []string             `json:"insights,omitempty"`
}

// ModelRecommendation represents model recommendations based on statistics.
type ModelRecommendation struct {
	BestPerformance string `json:"best_performance"` // Best quality output
	BestCostValue   string `json:"best_cost_value"`  // Best cost-effectiveness
	MostReliable    string `json:"most_reliable"`    // Most stable
	Fastest         string `json:"fastest"`          // Fastest response
}

// TokenUsage represents token usage statistics.
type TokenUsage struct {
	// Input/Output
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
	TotalTokens  int64 `json:"total_tokens"`

	// Cache related
	CacheReadTokens  int64 `json:"cache_read_tokens"`
	CacheWriteTokens int64 `json:"cache_write_tokens"`

	// Cost estimation (USD)
	EstimatedCost float64 `json:"estimated_cost"`
}

// TokenPricing represents token pricing configuration.
type TokenPricing struct {
	// Model matching pattern (supports wildcards)
	Pattern string `json:"pattern" yaml:"pattern"` // e.g., "opus*", "sonnet*", "haiku*"

	// Price per million tokens (USD)
	InputPrice  float64 `json:"input_price" yaml:"input"`
	OutputPrice float64 `json:"output_price" yaml:"output"`
	CacheRead   float64 `json:"cache_read" yaml:"cache_read"`
	CacheWrite  float64 `json:"cache_write" yaml:"cache_write"`
}

// LatencyStats represents latency statistics.
type LatencyStats struct {
	// Basic statistics (ms)
	Min float64 `json:"min_ms"`
	Max float64 `json:"max_ms"`
	Avg float64 `json:"avg_ms"`

	// Percentiles (ms)
	P50 float64 `json:"p50_ms"`
	P90 float64 `json:"p90_ms"`
	P95 float64 `json:"p95_ms"`
	P99 float64 `json:"p99_ms"`

	// Sample count
	Samples int64 `json:"samples"`
}

// GenerationSpeed represents generation speed statistics.
type GenerationSpeed struct {
	// Token generation rate
	TokensPerSecond    float64 `json:"tokens_per_second"`
	AvgTokensPerSecond float64 `json:"avg_tokens_per_second"`
	MaxTokensPerSecond float64 `json:"max_tokens_per_second"`

	// Prefill rate (time to first token)
	PrefillSpeed        float64 `json:"prefill_speed"`
	TimeToFirstToken    float64 `json:"time_to_first_token_ms"`
	AvgTimeToFirstToken float64 `json:"avg_ttft_ms"`

	// Decode rate
	DecodeSpeed float64 `json:"decode_speed"`
}

// TimeBreakdown represents time breakdown for a request.
type TimeBreakdown struct {
	QueueTime        time.Duration `json:"queue_time_ms"`
	TimeToFirstToken time.Duration `json:"ttft_ms"`
	GenerationTime   time.Duration `json:"generation_time_ms"`
	TotalTime        time.Duration `json:"total_time_ms"`
}

// ProcessMetrics represents CLI process metrics.
type ProcessMetrics struct {
	// Process info
	PID     int    `json:"pid"`
	Command string `json:"command"`
	State   string `json:"state"` // running, sleeping, etc.

	// CPU usage
	CPUPercent float64 `json:"cpu_percent"`
	CPUTime    float64 `json:"cpu_time_s"`

	// Memory usage
	MemoryRSS     int64   `json:"memory_rss_bytes"`
	MemoryVMS     int64   `json:"memory_vms_bytes"`
	MemoryPercent float64 `json:"memory_percent"`

	// I/O statistics
	IOReadBytes  int64 `json:"io_read_bytes"`
	IOWriteBytes int64 `json:"io_write_bytes"`

	// Threads
	NumThreads int `json:"num_threads"`

	// Runtime
	StartTime time.Time     `json:"start_time"`
	Uptime    time.Duration `json:"uptime"`
}

// SystemResourceMetrics represents system resource metrics.
type SystemResourceMetrics struct {
	// CPU
	CPUCount        int     `json:"cpu_count"`
	CPUUsagePercent float64 `json:"cpu_usage_percent"`
	LoadAvg1        float64 `json:"load_avg_1"`
	LoadAvg5        float64 `json:"load_avg_5"`
	LoadAvg15       float64 `json:"load_avg_15"`

	// Memory
	MemoryTotal   int64   `json:"memory_total_bytes"`
	MemoryUsed    int64   `json:"memory_used_bytes"`
	MemoryFree    int64   `json:"memory_free_bytes"`
	MemoryPercent float64 `json:"memory_percent"`

	// Disk
	DiskTotal   int64   `json:"disk_total_bytes"`
	DiskUsed    int64   `json:"disk_used_bytes"`
	DiskFree    int64   `json:"disk_free_bytes"`
	DiskPercent float64 `json:"disk_percent"`

	// Network
	NetworkBytesSent int64 `json:"network_bytes_sent"`
	NetworkBytesRecv int64 `json:"network_bytes_recv"`
}

// ResourceHistory represents resource history data point.
type ResourceHistory struct {
	Timestamp time.Time `json:"timestamp"`
	CPU       float64   `json:"cpu_percent"`
	Memory    float64   `json:"memory_percent"`
	Disk      float64   `json:"disk_percent"`
}

// CallRecord represents a single API call record for tracking.
type CallRecord struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Model     string    `json:"model"`
	Success   bool      `json:"success"`
	ErrorType string    `json:"error_type,omitempty"` // timeout, rate_limit, error

	// Latency
	LatencyMs        float64 `json:"latency_ms"`
	TimeToFirstToken float64 `json:"ttft_ms,omitempty"`

	// Tokens
	InputTokens      int64 `json:"input_tokens"`
	OutputTokens     int64 `json:"output_tokens"`
	CacheReadTokens  int64 `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens int64 `json:"cache_write_tokens,omitempty"`

	// Speed
	TokensPerSecond float64 `json:"tokens_per_second,omitempty"`
}

// ErrorType constants for call records.
const (
	ErrorTypeNone      = ""
	ErrorTypeTimeout   = "timeout"
	ErrorTypeRateLimit = "rate_limit"
	ErrorTypeError     = "error"
)

// Period constants for time-based queries.
const (
	PeriodRealtime = "realtime" // Last 1 minute
	PeriodHourly   = "hourly"   // Last hour
	PeriodDaily    = "daily"    // Last 24 hours
	PeriodWeekly   = "weekly"   // Last 7 days
	PeriodMonthly  = "monthly"  // Last 30 days
)

// MetricsQuery represents a query for metrics data.
type MetricsQuery struct {
	Period    string    `json:"period"`
	StartTime time.Time `json:"start_time,omitempty"`
	EndTime   time.Time `json:"end_time,omitempty"`
	Model     string    `json:"model,omitempty"`
	Limit     int       `json:"limit,omitempty"`
}

// CallStatsResponse represents the API response for call statistics.
type CallStatsResponse struct {
	Period string     `json:"period"`
	Stats  *CallStats `json:"stats"`
	ByHour []HourlyStats `json:"by_hour,omitempty"`
}

// HourlyStats represents hourly statistics.
type HourlyStats struct {
	Hour        time.Time `json:"hour"`
	Calls       int64     `json:"calls"`
	SuccessRate float64   `json:"success_rate"`
}

// ModelStatsResponse represents the API response for model statistics.
type ModelStatsResponse struct {
	Period  string       `json:"period"`
	Models  []ModelStats `json:"models"`
	Summary *StatsSummary `json:"summary,omitempty"`
}

// StatsSummary represents a summary of statistics.
type StatsSummary struct {
	TotalCalls  int64   `json:"total_calls"`
	TotalTokens int64   `json:"total_tokens"`
	TotalCost   float64 `json:"total_cost"`
}

// TokenUsageResponse represents the API response for token usage.
type TokenUsageResponse struct {
	Period  string       `json:"period"`
	Usage   *TokenUsage  `json:"usage"`
	ByModel []ModelTokenUsage `json:"by_model,omitempty"`
}

// ModelTokenUsage represents token usage for a specific model.
type ModelTokenUsage struct {
	Model            string  `json:"model"`
	InputTokens      int64   `json:"input_tokens"`
	OutputTokens     int64   `json:"output_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	CacheReadTokens  int64   `json:"cache_read_tokens"`
	CacheWriteTokens int64   `json:"cache_write_tokens"`
	EstimatedCost    float64 `json:"estimated_cost"`
}

// SpeedResponse represents the API response for speed statistics.
type SpeedResponse struct {
	Current     *SpeedStats      `json:"current"`
	Average     *SpeedStats      `json:"average"`
	Percentiles *SpeedPercentiles `json:"percentiles"`
}

// SpeedStats represents speed statistics.
type SpeedStats struct {
	TokensPerSecond    float64 `json:"tokens_per_second"`
	TimeToFirstTokenMs float64 `json:"time_to_first_token_ms"`
	DecodeSpeed        float64 `json:"decode_speed"`
}

// SpeedPercentiles represents speed percentiles.
type SpeedPercentiles struct {
	TTFTP50Ms float64 `json:"ttft_p50_ms"`
	TTFTP95Ms float64 `json:"ttft_p95_ms"`
	TTFTP99Ms float64 `json:"ttft_p99_ms"`
	TPSP50    float64 `json:"tps_p50"`
	TPSP95    float64 `json:"tps_p95"`
	TPSP99    float64 `json:"tps_p99"`
}

// ProcessMetricsResponse represents the API response for process metrics.
type ProcessMetricsResponse struct {
	Processes []ProcessMetrics `json:"processes"`
	Summary   *ProcessSummary  `json:"summary"`
}

// ProcessSummary represents a summary of process metrics.
type ProcessSummary struct {
	TotalProcesses   int     `json:"total_processes"`
	TotalCPUPercent  float64 `json:"total_cpu_percent"`
	TotalMemoryBytes int64   `json:"total_memory_bytes"`
}

// UserTokenUsage represents token usage for a specific user.
type UserTokenUsage struct {
	UserID           string  `json:"user_id"`
	InputTokens      int64   `json:"input_tokens"`
	OutputTokens     int64   `json:"output_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	CacheReadTokens  int64   `json:"cache_read_tokens"`
	CacheWriteTokens int64   `json:"cache_write_tokens"`
	EstimatedCost    float64 `json:"estimated_cost"`
	RequestCount     int64   `json:"request_count"`
}

// UserTokenUsageResponse represents the API response for user token usage.
type UserTokenUsageResponse struct {
	Period  string           `json:"period"`
	Users   []UserTokenUsage `json:"users"`
	Summary *UserUsageSummary `json:"summary,omitempty"`
}

// UserUsageSummary represents a summary of user token usage.
type UserUsageSummary struct {
	TotalUsers    int     `json:"total_users"`
	TotalTokens   int64   `json:"total_tokens"`
	TotalCost     float64 `json:"total_cost"`
	TotalRequests int64   `json:"total_requests"`
}
