package main

import "go.uber.org/zap"

func auxServiceLoggerConfig() zap.Config {
	cfg := zap.NewProductionConfig()
	// Disable sampler counters so the auxiliary service logger keeps production
	// formatting/levels without paying the extra cold-start memory overhead.
	cfg.Sampling = nil
	return cfg
}

func newAuxServiceLogger() (*zap.Logger, error) {
	return auxServiceLoggerConfig().Build()
}
