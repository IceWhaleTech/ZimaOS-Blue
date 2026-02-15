package memory

import "time"

// APIConfig holds configuration for the Memory Service API.
type APIConfig struct {
	Enabled       bool          `json:"enabled" yaml:"enabled"`
	DBPath        string        `json:"db_path" yaml:"db_path"`
	PurgeInterval time.Duration `json:"purge_interval" yaml:"purge_interval"`
}

// DefaultAPIConfig returns sensible defaults.
func DefaultAPIConfig() APIConfig {
	return APIConfig{
		Enabled:       true,
		DBPath:        "data/memory_api.db",
		PurgeInterval: 6 * time.Hour,
	}
}
