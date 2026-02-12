// Package stats provides usage statistics collection and reporting.
package stats

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// APICallEvent represents a single API call event.
type APICallEvent struct {
	ID            string        `json:"id"`
	Timestamp     time.Time     `json:"timestamp"`
	Provider      string        `json:"provider"`
	Model         string        `json:"model"`
	InputTokens   int64         `json:"input_tokens"`
	OutputTokens  int64         `json:"output_tokens"`
	CacheHitTokens int64        `json:"cache_hit_tokens,omitempty"`
	TotalTokens   int64         `json:"total_tokens"`
	LatencyMs     int64         `json:"latency_ms"`
	Success       bool          `json:"success"`
	ErrorType     string        `json:"error_type,omitempty"`
	ErrorMessage  string        `json:"error_message,omitempty"`
	ToolCalls     int           `json:"tool_calls,omitempty"`
	Streaming     bool          `json:"streaming"`
}

// UsageStats represents aggregated usage statistics.
type UsageStats struct {
	// Time range
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`

	// API Calls
	TotalCalls      int64            `json:"total_calls"`
	SuccessfulCalls int64            `json:"successful_calls"`
	FailedCalls     int64            `json:"failed_calls"`
	CallsByProvider map[string]int64 `json:"calls_by_provider"`
	CallsByModel    map[string]int64 `json:"calls_by_model"`

	// Token Usage
	InputTokens      int64            `json:"input_tokens"`
	OutputTokens     int64            `json:"output_tokens"`
	CacheHitTokens   int64            `json:"cache_hit_tokens"`
	TotalTokens      int64            `json:"total_tokens"`
	TokensByProvider map[string]int64 `json:"tokens_by_provider"`
	TokensByModel    map[string]int64 `json:"tokens_by_model"`

	// Performance
	AvgLatencyMs    float64          `json:"avg_latency_ms"`
	P50LatencyMs    float64          `json:"p50_latency_ms"`
	P95LatencyMs    float64          `json:"p95_latency_ms"`
	P99LatencyMs    float64          `json:"p99_latency_ms"`
	MaxLatencyMs    int64            `json:"max_latency_ms"`
	MinLatencyMs    int64            `json:"min_latency_ms"`

	// Errors
	ErrorCount      int64            `json:"error_count"`
	ErrorsByType    map[string]int64 `json:"errors_by_type"`

	// Tool Calling
	TotalToolCalls  int64            `json:"total_tool_calls"`

	// Cost Estimation (USD)
	EstimatedCostUSD float64         `json:"estimated_cost_usd"`
	CostByProvider   map[string]float64 `json:"cost_by_provider"`
	CostByModel      map[string]float64 `json:"cost_by_model"`
}

// StatisticsCollector collects and aggregates usage statistics.
type StatisticsCollector struct {
	mu          sync.RWMutex
	enabled     bool
	storagePath string
	events      []APICallEvent
	maxEvents   int
	pricing     map[string]ModelPricing
	syncPersist bool // For testing: persist synchronously
	wg          sync.WaitGroup
}

// ModelPricing represents pricing for a model (per 1M tokens).
type ModelPricing struct {
	InputPricePerMillion  float64 `json:"input_price_per_million"`
	OutputPricePerMillion float64 `json:"output_price_per_million"`
	CachePricePerMillion  float64 `json:"cache_price_per_million,omitempty"`
}

// NewStatisticsCollector creates a new statistics collector.
func NewStatisticsCollector(storagePath string, enabled bool) *StatisticsCollector {
	if storagePath == "" {
		home, _ := os.UserHomeDir()
		storagePath = filepath.Join(home, ".local", "share", "zimaos-blue", "stats")
	}

	collector := &StatisticsCollector{
		enabled:     enabled,
		storagePath: storagePath,
		events:      make([]APICallEvent, 0),
		maxEvents:   100000, // Keep last 100k events in memory
		pricing:     getDefaultPricing(),
	}

	// Load existing events
	collector.loadEvents()

	return collector
}

// getDefaultPricing returns default pricing for common models.
func getDefaultPricing() map[string]ModelPricing {
	return map[string]ModelPricing{
		// Anthropic
		"claude-3-opus":       {InputPricePerMillion: 15.0, OutputPricePerMillion: 75.0},
		"claude-3-sonnet":     {InputPricePerMillion: 3.0, OutputPricePerMillion: 15.0},
		"claude-3-haiku":      {InputPricePerMillion: 0.25, OutputPricePerMillion: 1.25},
		"claude-3-5-sonnet":   {InputPricePerMillion: 3.0, OutputPricePerMillion: 15.0},
		"claude-3-5-haiku":    {InputPricePerMillion: 1.0, OutputPricePerMillion: 5.0},

		// OpenAI
		"gpt-4o":              {InputPricePerMillion: 2.5, OutputPricePerMillion: 10.0},
		"gpt-4o-mini":         {InputPricePerMillion: 0.15, OutputPricePerMillion: 0.6},
		"gpt-4-turbo":         {InputPricePerMillion: 10.0, OutputPricePerMillion: 30.0},
		"gpt-4":               {InputPricePerMillion: 30.0, OutputPricePerMillion: 60.0},
		"gpt-3.5-turbo":       {InputPricePerMillion: 0.5, OutputPricePerMillion: 1.5},

		// DeepSeek
		"deepseek-chat":       {InputPricePerMillion: 0.14, OutputPricePerMillion: 0.28},
		"deepseek-coder":      {InputPricePerMillion: 0.14, OutputPricePerMillion: 0.28},

		// Gemini
		"gemini-1.5-pro":      {InputPricePerMillion: 1.25, OutputPricePerMillion: 5.0},
		"gemini-1.5-flash":    {InputPricePerMillion: 0.075, OutputPricePerMillion: 0.3},

		// Ollama (free)
		"llama3.2":            {InputPricePerMillion: 0, OutputPricePerMillion: 0},
		"llama3.1":            {InputPricePerMillion: 0, OutputPricePerMillion: 0},
		"mistral":             {InputPricePerMillion: 0, OutputPricePerMillion: 0},
		"codellama":           {InputPricePerMillion: 0, OutputPricePerMillion: 0},
	}
}

// SetEnabled enables or disables statistics collection.
func (c *StatisticsCollector) SetEnabled(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enabled = enabled
}

// IsEnabled returns whether statistics collection is enabled.
func (c *StatisticsCollector) IsEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.enabled
}

// Record records an API call event.
func (c *StatisticsCollector) Record(event *APICallEvent) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.enabled {
		return nil
	}

	// Generate ID if not set
	if event.ID == "" {
		event.ID = generateEventID()
	}

	// Set timestamp if not set
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Calculate total tokens
	event.TotalTokens = event.InputTokens + event.OutputTokens

	// Add to events
	c.events = append(c.events, *event)

	// Trim if exceeds max
	if len(c.events) > c.maxEvents {
		c.events = c.events[len(c.events)-c.maxEvents:]
	}

	// Persist to disk (async unless syncPersist is set)
	if c.syncPersist {
		c.persistEvent(event)
	} else {
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			c.persistEvent(event)
		}()
	}

	return nil
}

// GetStats returns aggregated statistics for a time period.
func (c *StatisticsCollector) GetStats(period string) (*UsageStats, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()
	var start time.Time

	switch period {
	case "hour":
		start = now.Add(-time.Hour)
	case "day":
		start = now.Add(-24 * time.Hour)
	case "week":
		start = now.Add(-7 * 24 * time.Hour)
	case "month":
		start = now.Add(-30 * 24 * time.Hour)
	case "all":
		start = time.Time{}
	default:
		start = now.Add(-24 * time.Hour) // Default to day
	}

	return c.aggregateStats(start, now)
}

// GetStatsByDateRange returns statistics for a specific date range.
func (c *StatisticsCollector) GetStatsByDateRange(start, end time.Time) (*UsageStats, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.aggregateStats(start, end)
}

// aggregateStats aggregates statistics for a time range.
func (c *StatisticsCollector) aggregateStats(start, end time.Time) (*UsageStats, error) {
	stats := &UsageStats{
		PeriodStart:      start,
		PeriodEnd:        end,
		CallsByProvider:  make(map[string]int64),
		CallsByModel:     make(map[string]int64),
		TokensByProvider: make(map[string]int64),
		TokensByModel:    make(map[string]int64),
		ErrorsByType:     make(map[string]int64),
		CostByProvider:   make(map[string]float64),
		CostByModel:      make(map[string]float64),
		MinLatencyMs:     -1,
	}

	var latencies []int64

	for _, event := range c.events {
		// Filter by time range
		if !start.IsZero() && event.Timestamp.Before(start) {
			continue
		}
		if event.Timestamp.After(end) {
			continue
		}

		// Count calls
		stats.TotalCalls++
		if event.Success {
			stats.SuccessfulCalls++
		} else {
			stats.FailedCalls++
			stats.ErrorCount++
			if event.ErrorType != "" {
				stats.ErrorsByType[event.ErrorType]++
			}
		}

		// Count by provider and model
		stats.CallsByProvider[event.Provider]++
		stats.CallsByModel[event.Model]++

		// Sum tokens
		stats.InputTokens += event.InputTokens
		stats.OutputTokens += event.OutputTokens
		stats.CacheHitTokens += event.CacheHitTokens
		stats.TotalTokens += event.TotalTokens
		stats.TokensByProvider[event.Provider] += event.TotalTokens
		stats.TokensByModel[event.Model] += event.TotalTokens

		// Track latency
		latencies = append(latencies, event.LatencyMs)
		if stats.MinLatencyMs < 0 || event.LatencyMs < stats.MinLatencyMs {
			stats.MinLatencyMs = event.LatencyMs
		}
		if event.LatencyMs > stats.MaxLatencyMs {
			stats.MaxLatencyMs = event.LatencyMs
		}

		// Count tool calls
		stats.TotalToolCalls += int64(event.ToolCalls)

		// Calculate cost
		cost := c.calculateCost(event.Model, event.InputTokens, event.OutputTokens, event.CacheHitTokens)
		stats.EstimatedCostUSD += cost
		stats.CostByProvider[event.Provider] += cost
		stats.CostByModel[event.Model] += cost
	}

	// Calculate latency percentiles
	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

		var sum int64
		for _, l := range latencies {
			sum += l
		}
		stats.AvgLatencyMs = float64(sum) / float64(len(latencies))
		stats.P50LatencyMs = float64(latencies[len(latencies)*50/100])
		stats.P95LatencyMs = float64(latencies[len(latencies)*95/100])
		stats.P99LatencyMs = float64(latencies[len(latencies)*99/100])
	}

	if stats.MinLatencyMs < 0 {
		stats.MinLatencyMs = 0
	}

	return stats, nil
}

// calculateCost calculates the cost for a request.
func (c *StatisticsCollector) calculateCost(model string, inputTokens, outputTokens, cacheTokens int64) float64 {
	pricing, ok := c.pricing[model]
	if !ok {
		return 0
	}

	inputCost := float64(inputTokens) * pricing.InputPricePerMillion / 1000000
	outputCost := float64(outputTokens) * pricing.OutputPricePerMillion / 1000000
	cacheCost := float64(cacheTokens) * pricing.CachePricePerMillion / 1000000

	return inputCost + outputCost + cacheCost
}

// Export exports statistics to a file.
func (c *StatisticsCollector) Export(format string) ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	switch format {
	case "json":
		return json.MarshalIndent(c.events, "", "  ")
	default:
		return json.MarshalIndent(c.events, "", "  ")
	}
}

// Clear clears all statistics.
func (c *StatisticsCollector) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.events = make([]APICallEvent, 0)

	// Remove persisted data
	eventsFile := filepath.Join(c.storagePath, "events.json")
	os.Remove(eventsFile)

	return nil
}

// GetRecentEvents returns the most recent events.
func (c *StatisticsCollector) GetRecentEvents(limit int) []APICallEvent {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if limit <= 0 || limit > len(c.events) {
		limit = len(c.events)
	}

	start := len(c.events) - limit
	result := make([]APICallEvent, limit)
	copy(result, c.events[start:])

	return result
}

// persistEvent persists an event to disk.
func (c *StatisticsCollector) persistEvent(event *APICallEvent) {
	if err := os.MkdirAll(c.storagePath, 0755); err != nil {
		return
	}

	// Append to events file
	eventsFile := filepath.Join(c.storagePath, "events.jsonl")
	f, err := os.OpenFile(eventsFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	f.Write(data)
	f.Write([]byte("\n"))
}

// loadEvents loads events from disk.
func (c *StatisticsCollector) loadEvents() {
	eventsFile := filepath.Join(c.storagePath, "events.jsonl")
	data, err := os.ReadFile(eventsFile)
	if err != nil {
		return
	}

	lines := splitLines(string(data))
	for _, line := range lines {
		if line == "" {
			continue
		}
		var event APICallEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		c.events = append(c.events, event)
	}

	// Trim if exceeds max
	if len(c.events) > c.maxEvents {
		c.events = c.events[len(c.events)-c.maxEvents:]
	}
}

// splitLines splits a string into lines.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// generateEventID generates a unique event ID.
func generateEventID() string {
	return time.Now().Format("20060102150405.000000")
}

// SetPricing sets custom pricing for a model.
func (c *StatisticsCollector) SetPricing(model string, pricing ModelPricing) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pricing[model] = pricing
}

// GetEventCount returns the total number of events.
func (c *StatisticsCollector) GetEventCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.events)
}

// Flush waits for all pending async operations to complete.
func (c *StatisticsCollector) Flush() {
	c.wg.Wait()
}

// SetSyncPersist sets whether to persist synchronously (for testing).
func (c *StatisticsCollector) SetSyncPersist(sync bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.syncPersist = sync
}
