// Package bootstrap provides shared server initialization logic
package bootstrap

// ServerConfig holds configuration for server initialization
type ServerConfig struct {
	Port      int
	DataDir   string
	Version   string
	BuildTime string
	GitCommit string
	Mode      string // "standalone" or "embedded"
}

// DefaultConfig returns default server configuration
func DefaultConfig() *ServerConfig {
	return &ServerConfig{
		Port:      80,
		DataDir:   "./data",
		Version:   "dev",
		BuildTime: "unknown",
		GitCommit: "unknown",
		Mode:      "standalone",
	}
}
