// Package benchmark provides performance benchmarking utilities.
package benchmark

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sort"
	"sync"
	"time"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// Result represents the result of a benchmark run.
type Result struct {
	Name           string        `json:"name"`
	Iterations     int64         `json:"iterations"`
	TotalDuration  time.Duration `json:"total_duration"`
	AvgDuration    time.Duration `json:"avg_duration"`
	MinDuration    time.Duration `json:"min_duration"`
	MaxDuration    time.Duration `json:"max_duration"`
	P50Duration    time.Duration `json:"p50_duration"`
	P90Duration    time.Duration `json:"p90_duration"`
	P99Duration    time.Duration `json:"p99_duration"`
	OpsPerSecond   float64       `json:"ops_per_second"`
	AllocsPerOp    int64         `json:"allocs_per_op"`
	BytesPerOp     int64         `json:"bytes_per_op"`
	Errors         int64         `json:"errors"`
	StartTime      time.Time     `json:"start_time"`
	EndTime        time.Time     `json:"end_time"`
}

// Suite represents a collection of benchmarks.
type Suite struct {
	Name       string
	benchmarks map[string]*Benchmark
	results    map[string]*Result
	mu         sync.Mutex
}

// NewSuite creates a new benchmark suite.
func NewSuite(name string) *Suite {
	return &Suite{
		Name:       name,
		benchmarks: make(map[string]*Benchmark),
		results:    make(map[string]*Result),
	}
}

// Add adds a benchmark to the suite.
func (s *Suite) Add(name string, fn func(ctx context.Context) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.benchmarks[name] = &Benchmark{
		Name: name,
		Fn:   fn,
	}
}

// Run runs all benchmarks in the suite.
func (s *Suite) Run(ctx context.Context, config Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for name, bench := range s.benchmarks {
		result, err := bench.Run(ctx, config)
		if err != nil {
			return fmt.Errorf("benchmark %s failed: %w", name, err)
		}
		s.results[name] = result
	}

	return nil
}

// Results returns all benchmark results.
func (s *Suite) Results() map[string]*Result {
	s.mu.Lock()
	defer s.mu.Unlock()

	results := make(map[string]*Result)
	for k, v := range s.results {
		results[k] = v
	}
	return results
}

// Report generates a report of all benchmark results.
func (s *Suite) Report() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	var report string
	report += fmt.Sprintf("Benchmark Suite: %s\n", s.Name)
	report += fmt.Sprintf("================\n\n")

	for name, result := range s.results {
		report += fmt.Sprintf("Benchmark: %s\n", name)
		report += fmt.Sprintf("  Iterations:    %d\n", result.Iterations)
		report += fmt.Sprintf("  Total Time:    %v\n", result.TotalDuration)
		report += fmt.Sprintf("  Avg Time:      %v\n", result.AvgDuration)
		report += fmt.Sprintf("  Min Time:      %v\n", result.MinDuration)
		report += fmt.Sprintf("  Max Time:      %v\n", result.MaxDuration)
		report += fmt.Sprintf("  P50:           %v\n", result.P50Duration)
		report += fmt.Sprintf("  P90:           %v\n", result.P90Duration)
		report += fmt.Sprintf("  P99:           %v\n", result.P99Duration)
		report += fmt.Sprintf("  Ops/sec:       %.2f\n", result.OpsPerSecond)
		report += fmt.Sprintf("  Allocs/op:     %d\n", result.AllocsPerOp)
		report += fmt.Sprintf("  Bytes/op:      %d\n", result.BytesPerOp)
		report += fmt.Sprintf("  Errors:        %d\n", result.Errors)
		report += "\n"
	}

	return report
}

// SaveJSON saves results to a JSON file.
func (s *Suite) SaveJSON(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(s.results, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal results: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// Benchmark represents a single benchmark.
type Benchmark struct {
	Name string
	Fn   func(ctx context.Context) error
}

// Config holds benchmark configuration.
type Config struct {
	// Duration is the minimum duration to run the benchmark.
	Duration time.Duration
	// Iterations is the number of iterations to run (0 = auto).
	Iterations int64
	// Warmup is the number of warmup iterations.
	Warmup int
	// Parallel is the number of parallel goroutines.
	Parallel int
	// CollectMemStats enables memory statistics collection.
	CollectMemStats bool
}

// DefaultConfig returns the default benchmark configuration.
func DefaultConfig() Config {
	return Config{
		Duration:        5 * time.Second,
		Iterations:      0,
		Warmup:          10,
		Parallel:        1,
		CollectMemStats: true,
	}
}

// Run runs the benchmark and returns the result.
func (b *Benchmark) Run(ctx context.Context, config Config) (*Result, error) {
	if config.Parallel <= 0 {
		config.Parallel = 1
	}

	// Warmup
	for i := 0; i < config.Warmup; i++ {
		b.Fn(ctx)
	}

	// Force GC before benchmark
	runtime.GC()

	var memStatsBefore, memStatsAfter runtime.MemStats
	if config.CollectMemStats {
		runtime.ReadMemStats(&memStatsBefore)
	}

	// Run benchmark
	var durations []time.Duration
	var errors int64
	var iterations int64

	startTime := timeutil.NowTime()
	deadline := startTime.Add(config.Duration)

	if config.Iterations > 0 {
		// Fixed iterations
		iterations = config.Iterations
		durations = make([]time.Duration, 0, iterations)

		for i := int64(0); i < iterations; i++ {
			start := timeutil.NowTime()
			if err := b.Fn(ctx); err != nil {
				errors++
			}
			durations = append(durations, timeutil.SinceTime(start))
		}
	} else {
		// Time-based
		durations = make([]time.Duration, 0, 10000)

		for timeutil.NowTime().Before(deadline) {
			start := timeutil.NowTime()
			if err := b.Fn(ctx); err != nil {
				errors++
			}
			durations = append(durations, timeutil.SinceTime(start))
			iterations++
		}
	}

	endTime := timeutil.NowTime()
	totalDuration := endTime.Sub(startTime)

	if config.CollectMemStats {
		runtime.ReadMemStats(&memStatsAfter)
	}

	// Calculate statistics
	sort.Slice(durations, func(i, j int) bool {
		return durations[i] < durations[j]
	})

	var totalNanos int64
	for _, d := range durations {
		totalNanos += d.Nanoseconds()
	}

	result := &Result{
		Name:          b.Name,
		Iterations:    iterations,
		TotalDuration: totalDuration,
		Errors:        errors,
		StartTime:     startTime,
		EndTime:       endTime,
	}

	if len(durations) > 0 {
		result.AvgDuration = time.Duration(totalNanos / int64(len(durations)))
		result.MinDuration = durations[0]
		result.MaxDuration = durations[len(durations)-1]
		result.P50Duration = percentile(durations, 50)
		result.P90Duration = percentile(durations, 90)
		result.P99Duration = percentile(durations, 99)
		result.OpsPerSecond = float64(iterations) / totalDuration.Seconds()
	}

	if config.CollectMemStats && iterations > 0 {
		result.AllocsPerOp = int64(memStatsAfter.Mallocs-memStatsBefore.Mallocs) / iterations
		result.BytesPerOp = int64(memStatsAfter.TotalAlloc-memStatsBefore.TotalAlloc) / iterations
	}

	return result, nil
}

// percentile calculates the p-th percentile of sorted durations.
func percentile(durations []time.Duration, p int) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	index := (p * len(durations)) / 100
	if index >= len(durations) {
		index = len(durations) - 1
	}

	return durations[index]
}

// Compare compares two benchmark results and returns the difference.
func Compare(baseline, current *Result) *Comparison {
	if baseline == nil || current == nil {
		return nil
	}

	return &Comparison{
		Name:             current.Name,
		BaselineOps:     baseline.OpsPerSecond,
		CurrentOps:      current.OpsPerSecond,
		OpsChange:       (current.OpsPerSecond - baseline.OpsPerSecond) / baseline.OpsPerSecond * 100,
		BaselineAvg:     baseline.AvgDuration,
		CurrentAvg:      current.AvgDuration,
		AvgChange:       float64(current.AvgDuration-baseline.AvgDuration) / float64(baseline.AvgDuration) * 100,
		BaselineP99:     baseline.P99Duration,
		CurrentP99:      current.P99Duration,
		P99Change:       float64(current.P99Duration-baseline.P99Duration) / float64(baseline.P99Duration) * 100,
		BaselineAllocs:  baseline.AllocsPerOp,
		CurrentAllocs:   current.AllocsPerOp,
		AllocsChange:    float64(current.AllocsPerOp-baseline.AllocsPerOp) / float64(baseline.AllocsPerOp) * 100,
		BaselineBytes:   baseline.BytesPerOp,
		CurrentBytes:    current.BytesPerOp,
		BytesChange:     float64(current.BytesPerOp-baseline.BytesPerOp) / float64(baseline.BytesPerOp) * 100,
	}
}

// Comparison represents a comparison between two benchmark results.
type Comparison struct {
	Name            string        `json:"name"`
	BaselineOps     float64       `json:"baseline_ops"`
	CurrentOps      float64       `json:"current_ops"`
	OpsChange       float64       `json:"ops_change_percent"`
	BaselineAvg     time.Duration `json:"baseline_avg"`
	CurrentAvg      time.Duration `json:"current_avg"`
	AvgChange       float64       `json:"avg_change_percent"`
	BaselineP99     time.Duration `json:"baseline_p99"`
	CurrentP99      time.Duration `json:"current_p99"`
	P99Change       float64       `json:"p99_change_percent"`
	BaselineAllocs  int64         `json:"baseline_allocs"`
	CurrentAllocs   int64         `json:"current_allocs"`
	AllocsChange    float64       `json:"allocs_change_percent"`
	BaselineBytes   int64         `json:"baseline_bytes"`
	CurrentBytes    int64         `json:"current_bytes"`
	BytesChange     float64       `json:"bytes_change_percent"`
}

// IsRegression checks if the comparison indicates a performance regression.
func (c *Comparison) IsRegression(threshold float64) bool {
	// Regression if ops decreased or latency increased by more than threshold
	return c.OpsChange < -threshold || c.AvgChange > threshold || c.P99Change > threshold
}

// Report generates a comparison report.
func (c *Comparison) Report() string {
	var report string
	report += fmt.Sprintf("Comparison: %s\n", c.Name)
	report += fmt.Sprintf("  Ops/sec:    %.2f -> %.2f (%.2f%%)\n", c.BaselineOps, c.CurrentOps, c.OpsChange)
	report += fmt.Sprintf("  Avg Time:   %v -> %v (%.2f%%)\n", c.BaselineAvg, c.CurrentAvg, c.AvgChange)
	report += fmt.Sprintf("  P99 Time:   %v -> %v (%.2f%%)\n", c.BaselineP99, c.CurrentP99, c.P99Change)
	report += fmt.Sprintf("  Allocs/op:  %d -> %d (%.2f%%)\n", c.BaselineAllocs, c.CurrentAllocs, c.AllocsChange)
	report += fmt.Sprintf("  Bytes/op:   %d -> %d (%.2f%%)\n", c.BaselineBytes, c.CurrentBytes, c.BytesChange)
	return report
}
