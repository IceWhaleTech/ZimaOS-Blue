package harness

import (
	"os"
	"strconv"
	"strings"
)

const (
	harnessMaxConcurrencyEnv                = "ZIMA_HARNESS_MAX_CONCURRENCY"
	legacyHarnessMaxConcurrencyEnv          = "BLUE_HARNESS_MAX_CONCURRENCY"
	harnessSelectorMaxConcurrencyEnv        = "ZIMA_HARNESS_SELECTOR_MAX_CONCURRENCY"
	legacyHarnessSelectorMaxConcurrencyEnv  = "BLUE_HARNESS_SELECTOR_MAX_CONCURRENCY"
	harnessExecutionMaxConcurrencyEnv       = "ZIMA_HARNESS_EXECUTION_MAX_CONCURRENCY"
	legacyHarnessExecutionMaxConcurrencyEnv = "BLUE_HARNESS_EXECUTION_MAX_CONCURRENCY"
)

func resolvePositiveConcurrencyOverride(defaultValue int, envNames ...string) int {
	for _, envName := range envNames {
		raw := strings.TrimSpace(os.Getenv(envName))
		if raw == "" {
			continue
		}
		value, err := strconv.Atoi(raw)
		if err != nil || value <= 0 {
			continue
		}
		return value
	}
	return defaultValue
}

func selectorCuratedRuntimeMaxConcurrency() int {
	return resolvePositiveConcurrencyOverride(
		selectorCuratedMaxConcurrency,
		harnessSelectorMaxConcurrencyEnv,
		legacyHarnessSelectorMaxConcurrencyEnv,
		harnessMaxConcurrencyEnv,
		legacyHarnessMaxConcurrencyEnv,
	)
}

func batch1ExecutionRuntimeMaxConcurrency() int {
	return resolvePositiveConcurrencyOverride(
		batch1ExecutionMaxConcurrency,
		harnessExecutionMaxConcurrencyEnv,
		legacyHarnessExecutionMaxConcurrencyEnv,
		harnessMaxConcurrencyEnv,
		legacyHarnessMaxConcurrencyEnv,
	)
}

func applyRuntimeSchedulerConcurrencyOverride(subject string, scheduler GroupSchedulerConfig) GroupSchedulerConfig {
	out := scheduler
	switch strings.TrimSpace(subject) {
	case SelectorCuratedDatasetSubject:
		out.MaxConcurrency = selectorCuratedRuntimeMaxConcurrency()
	case Batch1ExecutionDatasetSubject:
		out.MaxConcurrency = batch1ExecutionRuntimeMaxConcurrency()
	default:
		out.MaxConcurrency = resolvePositiveConcurrencyOverride(
			out.MaxConcurrency,
			harnessMaxConcurrencyEnv,
			legacyHarnessMaxConcurrencyEnv,
		)
	}
	return out
}
