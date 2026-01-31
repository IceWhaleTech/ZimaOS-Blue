package metrics

import (
	"sort"
	"sync"
	"time"
)

// LatencyTracker tracks latency and speed metrics.
type LatencyTracker struct {
	mu sync.RWMutex

	// Latency samples by model
	modelLatencies map[string]*latencyAccumulator

	// Global latency samples
	globalLatencies *latencyAccumulator

	// Speed samples by model
	modelSpeeds map[string]*speedAccumulator

	// Global speed samples
	globalSpeeds *speedAccumulator

	// Configuration
	maxSamples int
}

// latencyAccumulator accumulates latency samples.
type latencyAccumulator struct {
	samples     []float64
	totalMs     float64
	minMs       float64
	maxMs       float64
	sampleCount int64
}

// speedAccumulator accumulates speed samples.
type speedAccumulator struct {
	tpsSamples     []float64
	ttftSamples    []float64
	decodeSamples  []float64
	totalTPS       float64
	totalTTFT      float64
	totalDecode    float64
	currentTPS     float64
	currentTTFT    float64
	currentDecode  float64
	maxTPS         float64
}

// NewLatencyTracker creates a new LatencyTracker.
func NewLatencyTracker(maxSamples int) *LatencyTracker {
	return &LatencyTracker{
		modelLatencies:  make(map[string]*latencyAccumulator),
		globalLatencies: newLatencyAccumulator(maxSamples),
		modelSpeeds:     make(map[string]*speedAccumulator),
		globalSpeeds:    newSpeedAccumulator(maxSamples),
		maxSamples:      maxSamples,
	}
}

func newLatencyAccumulator(maxSamples int) *latencyAccumulator {
	return &latencyAccumulator{
		samples: make([]float64, 0, maxSamples),
		minMs:   -1, // -1 indicates not set
	}
}

func newSpeedAccumulator(maxSamples int) *speedAccumulator {
	return &speedAccumulator{
		tpsSamples:    make([]float64, 0, maxSamples),
		ttftSamples:   make([]float64, 0, maxSamples),
		decodeSamples: make([]float64, 0, maxSamples),
	}
}

// RecordLatency records a latency sample.
func (t *LatencyTracker) RecordLatency(model string, latencyMs float64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if model == "" {
		model = "unknown"
	}

	// Update model latencies
	acc, ok := t.modelLatencies[model]
	if !ok {
		acc = newLatencyAccumulator(t.maxSamples)
		t.modelLatencies[model] = acc
	}
	t.recordLatencyToAccumulator(acc, latencyMs)

	// Update global latencies
	t.recordLatencyToAccumulator(t.globalLatencies, latencyMs)
}

func (t *LatencyTracker) recordLatencyToAccumulator(acc *latencyAccumulator, latencyMs float64) {
	acc.samples = append(acc.samples, latencyMs)
	if len(acc.samples) > t.maxSamples {
		acc.samples = acc.samples[1:]
	}

	acc.totalMs += latencyMs
	acc.sampleCount++

	if acc.minMs < 0 || latencyMs < acc.minMs {
		acc.minMs = latencyMs
	}
	if latencyMs > acc.maxMs {
		acc.maxMs = latencyMs
	}
}

// RecordSpeed records speed metrics.
func (t *LatencyTracker) RecordSpeed(model string, tokensPerSecond, ttftMs, decodeSpeed float64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if model == "" {
		model = "unknown"
	}

	// Update model speeds
	acc, ok := t.modelSpeeds[model]
	if !ok {
		acc = newSpeedAccumulator(t.maxSamples)
		t.modelSpeeds[model] = acc
	}
	t.recordSpeedToAccumulator(acc, tokensPerSecond, ttftMs, decodeSpeed)

	// Update global speeds
	t.recordSpeedToAccumulator(t.globalSpeeds, tokensPerSecond, ttftMs, decodeSpeed)
}

func (t *LatencyTracker) recordSpeedToAccumulator(acc *speedAccumulator, tps, ttft, decode float64) {
	if tps > 0 {
		acc.tpsSamples = append(acc.tpsSamples, tps)
		if len(acc.tpsSamples) > t.maxSamples {
			acc.tpsSamples = acc.tpsSamples[1:]
		}
		acc.totalTPS += tps
		acc.currentTPS = tps
		if tps > acc.maxTPS {
			acc.maxTPS = tps
		}
	}

	if ttft > 0 {
		acc.ttftSamples = append(acc.ttftSamples, ttft)
		if len(acc.ttftSamples) > t.maxSamples {
			acc.ttftSamples = acc.ttftSamples[1:]
		}
		acc.totalTTFT += ttft
		acc.currentTTFT = ttft
	}

	if decode > 0 {
		acc.decodeSamples = append(acc.decodeSamples, decode)
		if len(acc.decodeSamples) > t.maxSamples {
			acc.decodeSamples = acc.decodeSamples[1:]
		}
		acc.totalDecode += decode
		acc.currentDecode = decode
	}
}

// GetLatencyStats returns global latency statistics.
func (t *LatencyTracker) GetLatencyStats() *LatencyStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.buildLatencyStats(t.globalLatencies)
}

// GetLatencyStatsByModel returns latency statistics for a specific model.
func (t *LatencyTracker) GetLatencyStatsByModel(model string) *LatencyStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	acc, ok := t.modelLatencies[model]
	if !ok {
		return nil
	}

	return t.buildLatencyStats(acc)
}

// GetAllModelLatencyStats returns latency statistics for all models.
func (t *LatencyTracker) GetAllModelLatencyStats() map[string]*LatencyStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make(map[string]*LatencyStats)
	for model, acc := range t.modelLatencies {
		result[model] = t.buildLatencyStats(acc)
	}

	return result
}

func (t *LatencyTracker) buildLatencyStats(acc *latencyAccumulator) *LatencyStats {
	if acc.sampleCount == 0 {
		return &LatencyStats{}
	}

	stats := &LatencyStats{
		Min:     acc.minMs,
		Max:     acc.maxMs,
		Avg:     acc.totalMs / float64(acc.sampleCount),
		Samples: acc.sampleCount,
	}

	if len(acc.samples) > 0 {
		stats.P50 = percentile(acc.samples, 50)
		stats.P90 = percentile(acc.samples, 90)
		stats.P95 = percentile(acc.samples, 95)
		stats.P99 = percentile(acc.samples, 99)
	}

	return stats
}

// GetSpeedStats returns global speed statistics.
func (t *LatencyTracker) GetSpeedStats() *GenerationSpeed {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.buildSpeedStats(t.globalSpeeds)
}

// GetSpeedStatsByModel returns speed statistics for a specific model.
func (t *LatencyTracker) GetSpeedStatsByModel(model string) *GenerationSpeed {
	t.mu.RLock()
	defer t.mu.RUnlock()

	acc, ok := t.modelSpeeds[model]
	if !ok {
		return nil
	}

	return t.buildSpeedStats(acc)
}

// GetAllModelSpeedStats returns speed statistics for all models.
func (t *LatencyTracker) GetAllModelSpeedStats() map[string]*GenerationSpeed {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make(map[string]*GenerationSpeed)
	for model, acc := range t.modelSpeeds {
		result[model] = t.buildSpeedStats(acc)
	}

	return result
}

func (t *LatencyTracker) buildSpeedStats(acc *speedAccumulator) *GenerationSpeed {
	stats := &GenerationSpeed{
		TokensPerSecond:    acc.currentTPS,
		MaxTokensPerSecond: acc.maxTPS,
		TimeToFirstToken:   acc.currentTTFT,
		DecodeSpeed:        acc.currentDecode,
	}

	if len(acc.tpsSamples) > 0 {
		stats.AvgTokensPerSecond = acc.totalTPS / float64(len(acc.tpsSamples))
	}

	if len(acc.ttftSamples) > 0 {
		stats.AvgTimeToFirstToken = acc.totalTTFT / float64(len(acc.ttftSamples))
	}

	return stats
}

// GetSpeedPercentiles returns speed percentiles.
func (t *LatencyTracker) GetSpeedPercentiles() *SpeedPercentiles {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.buildSpeedPercentiles(t.globalSpeeds)
}

// GetSpeedPercentilesByModel returns speed percentiles for a specific model.
func (t *LatencyTracker) GetSpeedPercentilesByModel(model string) *SpeedPercentiles {
	t.mu.RLock()
	defer t.mu.RUnlock()

	acc, ok := t.modelSpeeds[model]
	if !ok {
		return nil
	}

	return t.buildSpeedPercentiles(acc)
}

func (t *LatencyTracker) buildSpeedPercentiles(acc *speedAccumulator) *SpeedPercentiles {
	percentiles := &SpeedPercentiles{}

	if len(acc.ttftSamples) > 0 {
		percentiles.TTFTP50Ms = percentile(acc.ttftSamples, 50)
		percentiles.TTFTP95Ms = percentile(acc.ttftSamples, 95)
		percentiles.TTFTP99Ms = percentile(acc.ttftSamples, 99)
	}

	if len(acc.tpsSamples) > 0 {
		percentiles.TPSP50 = percentile(acc.tpsSamples, 50)
		percentiles.TPSP95 = percentile(acc.tpsSamples, 95)
		percentiles.TPSP99 = percentile(acc.tpsSamples, 99)
	}

	return percentiles
}

// RecordTimeBreakdown records a time breakdown for a request.
func (t *LatencyTracker) RecordTimeBreakdown(model string, breakdown TimeBreakdown) {
	// Record latency
	t.RecordLatency(model, float64(breakdown.TotalTime.Milliseconds()))

	// Record speed metrics
	ttftMs := float64(breakdown.TimeToFirstToken.Milliseconds())
	t.RecordSpeed(model, 0, ttftMs, 0)
}

// Reset resets all latency and speed data.
func (t *LatencyTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.modelLatencies = make(map[string]*latencyAccumulator)
	t.globalLatencies = newLatencyAccumulator(t.maxSamples)
	t.modelSpeeds = make(map[string]*speedAccumulator)
	t.globalSpeeds = newSpeedAccumulator(t.maxSamples)
}

// ModelSpeedResponse represents speed response for a model.
type ModelSpeedResponse struct {
	Model       string            `json:"model"`
	Current     *SpeedStats       `json:"current"`
	Average     *SpeedStats       `json:"average"`
	Percentiles *SpeedPercentiles `json:"percentiles"`
}

// GetModelSpeedResponses returns speed responses for all models.
func (t *LatencyTracker) GetModelSpeedResponses() []ModelSpeedResponse {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]ModelSpeedResponse, 0, len(t.modelSpeeds))

	for model, acc := range t.modelSpeeds {
		speed := t.buildSpeedStats(acc)
		percentiles := t.buildSpeedPercentiles(acc)

		response := ModelSpeedResponse{
			Model: model,
			Current: &SpeedStats{
				TokensPerSecond:    acc.currentTPS,
				TimeToFirstTokenMs: acc.currentTTFT,
				DecodeSpeed:        acc.currentDecode,
			},
			Average: &SpeedStats{
				TokensPerSecond:    speed.AvgTokensPerSecond,
				TimeToFirstTokenMs: speed.AvgTimeToFirstToken,
				DecodeSpeed:        speed.DecodeSpeed,
			},
			Percentiles: percentiles,
		}

		result = append(result, response)
	}

	// Sort by model name
	sort.Slice(result, func(i, j int) bool {
		return result[i].Model < result[j].Model
	})

	return result
}

// TimeBreakdownTracker tracks time breakdown for requests.
type TimeBreakdownTracker struct {
	mu sync.RWMutex

	// Breakdown samples by model
	modelBreakdowns map[string][]TimeBreakdown

	// Global breakdown samples
	globalBreakdowns []TimeBreakdown

	maxSamples int
}

// NewTimeBreakdownTracker creates a new TimeBreakdownTracker.
func NewTimeBreakdownTracker(maxSamples int) *TimeBreakdownTracker {
	return &TimeBreakdownTracker{
		modelBreakdowns:  make(map[string][]TimeBreakdown),
		globalBreakdowns: make([]TimeBreakdown, 0, maxSamples),
		maxSamples:       maxSamples,
	}
}

// RecordBreakdown records a time breakdown.
func (t *TimeBreakdownTracker) RecordBreakdown(model string, breakdown TimeBreakdown) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if model == "" {
		model = "unknown"
	}

	// Update model breakdowns
	breakdowns, ok := t.modelBreakdowns[model]
	if !ok {
		breakdowns = make([]TimeBreakdown, 0, t.maxSamples)
	}
	breakdowns = append(breakdowns, breakdown)
	if len(breakdowns) > t.maxSamples {
		breakdowns = breakdowns[1:]
	}
	t.modelBreakdowns[model] = breakdowns

	// Update global breakdowns
	t.globalBreakdowns = append(t.globalBreakdowns, breakdown)
	if len(t.globalBreakdowns) > t.maxSamples {
		t.globalBreakdowns = t.globalBreakdowns[1:]
	}
}

// GetAverageBreakdown returns the average time breakdown.
func (t *TimeBreakdownTracker) GetAverageBreakdown() *TimeBreakdown {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.calculateAverageBreakdown(t.globalBreakdowns)
}

// GetAverageBreakdownByModel returns the average time breakdown for a model.
func (t *TimeBreakdownTracker) GetAverageBreakdownByModel(model string) *TimeBreakdown {
	t.mu.RLock()
	defer t.mu.RUnlock()

	breakdowns, ok := t.modelBreakdowns[model]
	if !ok {
		return nil
	}

	return t.calculateAverageBreakdown(breakdowns)
}

func (t *TimeBreakdownTracker) calculateAverageBreakdown(breakdowns []TimeBreakdown) *TimeBreakdown {
	if len(breakdowns) == 0 {
		return &TimeBreakdown{}
	}

	var totalQueue, totalTTFT, totalGen, totalTotal time.Duration
	for _, b := range breakdowns {
		totalQueue += b.QueueTime
		totalTTFT += b.TimeToFirstToken
		totalGen += b.GenerationTime
		totalTotal += b.TotalTime
	}

	count := time.Duration(len(breakdowns))
	return &TimeBreakdown{
		QueueTime:        totalQueue / count,
		TimeToFirstToken: totalTTFT / count,
		GenerationTime:   totalGen / count,
		TotalTime:        totalTotal / count,
	}
}

// Reset resets all breakdown data.
func (t *TimeBreakdownTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.modelBreakdowns = make(map[string][]TimeBreakdown)
	t.globalBreakdowns = make([]TimeBreakdown, 0, t.maxSamples)
}

// ExportLatencySamples exports all latency samples for persistence.
func (t *LatencyTracker) ExportLatencySamples() []LatencySampleExport {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var exports []LatencySampleExport

	// Export global latency samples
	if t.globalLatencies != nil && len(t.globalLatencies.samples) > 0 {
		exports = append(exports, LatencySampleExport{
			Model:       "_global",
			SampleType:  "latency",
			Samples:     append([]float64{}, t.globalLatencies.samples...),
			TotalValue:  t.globalLatencies.totalMs,
			MinValue:    t.globalLatencies.minMs,
			MaxValue:    t.globalLatencies.maxMs,
			SampleCount: t.globalLatencies.sampleCount,
		})
	}

	// Export model latency samples
	for model, acc := range t.modelLatencies {
		if len(acc.samples) > 0 {
			exports = append(exports, LatencySampleExport{
				Model:       model,
				SampleType:  "latency",
				Samples:     append([]float64{}, acc.samples...),
				TotalValue:  acc.totalMs,
				MinValue:    acc.minMs,
				MaxValue:    acc.maxMs,
				SampleCount: acc.sampleCount,
			})
		}
	}

	// Export global speed samples
	if t.globalSpeeds != nil {
		if len(t.globalSpeeds.tpsSamples) > 0 {
			exports = append(exports, LatencySampleExport{
				Model:      "_global",
				SampleType: "tps",
				Samples:    append([]float64{}, t.globalSpeeds.tpsSamples...),
				TotalValue: t.globalSpeeds.totalTPS,
				MaxValue:   t.globalSpeeds.maxTPS,
			})
		}
		if len(t.globalSpeeds.ttftSamples) > 0 {
			exports = append(exports, LatencySampleExport{
				Model:      "_global",
				SampleType: "ttft",
				Samples:    append([]float64{}, t.globalSpeeds.ttftSamples...),
				TotalValue: t.globalSpeeds.totalTTFT,
			})
		}
		if len(t.globalSpeeds.decodeSamples) > 0 {
			exports = append(exports, LatencySampleExport{
				Model:      "_global",
				SampleType: "decode",
				Samples:    append([]float64{}, t.globalSpeeds.decodeSamples...),
				TotalValue: t.globalSpeeds.totalDecode,
			})
		}
	}

	// Export model speed samples
	for model, acc := range t.modelSpeeds {
		if len(acc.tpsSamples) > 0 {
			exports = append(exports, LatencySampleExport{
				Model:      model,
				SampleType: "tps",
				Samples:    append([]float64{}, acc.tpsSamples...),
				TotalValue: acc.totalTPS,
				MaxValue:   acc.maxTPS,
			})
		}
		if len(acc.ttftSamples) > 0 {
			exports = append(exports, LatencySampleExport{
				Model:      model,
				SampleType: "ttft",
				Samples:    append([]float64{}, acc.ttftSamples...),
				TotalValue: acc.totalTTFT,
			})
		}
		if len(acc.decodeSamples) > 0 {
			exports = append(exports, LatencySampleExport{
				Model:      model,
				SampleType: "decode",
				Samples:    append([]float64{}, acc.decodeSamples...),
				TotalValue: acc.totalDecode,
			})
		}
	}

	return exports
}

// LatencySampleExport represents exported latency sample data.
type LatencySampleExport struct {
	Model       string
	SampleType  string // "latency", "tps", "ttft", "decode"
	Samples     []float64
	TotalValue  float64
	MinValue    float64
	MaxValue    float64
	SampleCount int64
}

// LoadLatencySamples loads latency samples from persisted data.
func (t *LatencyTracker) LoadLatencySamples(exports []LatencySampleExport) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, exp := range exports {
		switch exp.SampleType {
		case "latency":
			if exp.Model == "_global" {
				t.globalLatencies.samples = append([]float64{}, exp.Samples...)
				t.globalLatencies.totalMs = exp.TotalValue
				t.globalLatencies.minMs = exp.MinValue
				t.globalLatencies.maxMs = exp.MaxValue
				t.globalLatencies.sampleCount = exp.SampleCount
			} else {
				acc, ok := t.modelLatencies[exp.Model]
				if !ok {
					acc = newLatencyAccumulator(t.maxSamples)
					t.modelLatencies[exp.Model] = acc
				}
				acc.samples = append([]float64{}, exp.Samples...)
				acc.totalMs = exp.TotalValue
				acc.minMs = exp.MinValue
				acc.maxMs = exp.MaxValue
				acc.sampleCount = exp.SampleCount
			}
		case "tps":
			if exp.Model == "_global" {
				t.globalSpeeds.tpsSamples = append([]float64{}, exp.Samples...)
				t.globalSpeeds.totalTPS = exp.TotalValue
				t.globalSpeeds.maxTPS = exp.MaxValue
				if len(exp.Samples) > 0 {
					t.globalSpeeds.currentTPS = exp.Samples[len(exp.Samples)-1]
				}
			} else {
				acc, ok := t.modelSpeeds[exp.Model]
				if !ok {
					acc = newSpeedAccumulator(t.maxSamples)
					t.modelSpeeds[exp.Model] = acc
				}
				acc.tpsSamples = append([]float64{}, exp.Samples...)
				acc.totalTPS = exp.TotalValue
				acc.maxTPS = exp.MaxValue
				if len(exp.Samples) > 0 {
					acc.currentTPS = exp.Samples[len(exp.Samples)-1]
				}
			}
		case "ttft":
			if exp.Model == "_global" {
				t.globalSpeeds.ttftSamples = append([]float64{}, exp.Samples...)
				t.globalSpeeds.totalTTFT = exp.TotalValue
				if len(exp.Samples) > 0 {
					t.globalSpeeds.currentTTFT = exp.Samples[len(exp.Samples)-1]
				}
			} else {
				acc, ok := t.modelSpeeds[exp.Model]
				if !ok {
					acc = newSpeedAccumulator(t.maxSamples)
					t.modelSpeeds[exp.Model] = acc
				}
				acc.ttftSamples = append([]float64{}, exp.Samples...)
				acc.totalTTFT = exp.TotalValue
				if len(exp.Samples) > 0 {
					acc.currentTTFT = exp.Samples[len(exp.Samples)-1]
				}
			}
		case "decode":
			if exp.Model == "_global" {
				t.globalSpeeds.decodeSamples = append([]float64{}, exp.Samples...)
				t.globalSpeeds.totalDecode = exp.TotalValue
				if len(exp.Samples) > 0 {
					t.globalSpeeds.currentDecode = exp.Samples[len(exp.Samples)-1]
				}
			} else {
				acc, ok := t.modelSpeeds[exp.Model]
				if !ok {
					acc = newSpeedAccumulator(t.maxSamples)
					t.modelSpeeds[exp.Model] = acc
				}
				acc.decodeSamples = append([]float64{}, exp.Samples...)
				acc.totalDecode = exp.TotalValue
				if len(exp.Samples) > 0 {
					acc.currentDecode = exp.Samples[len(exp.Samples)-1]
				}
			}
		}
	}
}
