package tunnel

import "fmt"

func resolveTargetPort(cfg *Config) (int, error) {
	if cfg == nil {
		return 0, fmt.Errorf("tunnel configuration is required")
	}
	if cfg.Port <= 0 {
		return 0, fmt.Errorf("tunnel target port is required")
	}
	return cfg.Port, nil
}
