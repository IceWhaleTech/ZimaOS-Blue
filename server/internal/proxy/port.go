package proxy

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// PortAllocator handles dynamic port allocation
type PortAllocator struct {
	config *PortConfig
	port   int
	mu     sync.RWMutex
}

// NewPortAllocator creates a new port allocator
func NewPortAllocator(config *PortConfig) *PortAllocator {
	return &PortAllocator{
		config: config,
	}
}

// Allocate finds and binds to an available port
func (pa *PortAllocator) Allocate() (int, error) {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	// If specific port is configured, use it
	if pa.config.Value > 0 {
		if pa.isPortAvailable(pa.config.Value) {
			pa.port = pa.config.Value
			if err := pa.writePortFile(); err != nil {
				return 0, err
			}
			return pa.port, nil
		}
		return 0, ErrPortInUse
	}

	// Parse port range
	minPort, maxPort, err := pa.parsePortRange()
	if err != nil {
		return 0, err
	}

	// Try ports in range
	for port := minPort; port <= maxPort; port++ {
		if pa.isPortAvailable(port) {
			pa.port = port
			if err := pa.writePortFile(); err != nil {
				return 0, err
			}
			return pa.port, nil
		}
	}

	return 0, ErrPortAllocationFailed
}

// GetPort returns the currently allocated port
func (pa *PortAllocator) GetPort() int {
	pa.mu.RLock()
	defer pa.mu.RUnlock()
	return pa.port
}

// GetEndpoint returns the full proxy endpoint URL
func (pa *PortAllocator) GetEndpoint() string {
	pa.mu.RLock()
	defer pa.mu.RUnlock()
	return fmt.Sprintf("http://%s:%d", pa.config.BindAddress, pa.port)
}

// GetBindAddress returns the bind address with port
func (pa *PortAllocator) GetBindAddress() string {
	pa.mu.RLock()
	defer pa.mu.RUnlock()
	return fmt.Sprintf("%s:%d", pa.config.BindAddress, pa.port)
}

// Release releases the allocated port
func (pa *PortAllocator) Release() error {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	pa.port = 0

	// Remove port file
	portFile := pa.getPortFilePath()
	if portFile != "" {
		os.Remove(portFile)
	}

	return nil
}

// parsePortRange parses the port range string
func (pa *PortAllocator) parsePortRange() (int, int, error) {
	if pa.config.Range == "" {
		return 9000, 9100, nil // Default range
	}

	parts := strings.Split(pa.config.Range, "-")
	if len(parts) != 2 {
		return 0, 0, ErrPortRangeInvalid
	}

	minPort, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, ErrPortRangeInvalid
	}

	maxPort, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, ErrPortRangeInvalid
	}

	if minPort < 1 || maxPort > 65535 || minPort > maxPort {
		return 0, 0, ErrPortRangeInvalid
	}

	return minPort, maxPort, nil
}

// isPortAvailable checks if a port is available
func (pa *PortAllocator) isPortAvailable(port int) bool {
	addr := fmt.Sprintf("%s:%d", pa.config.BindAddress, port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	listener.Close()
	return true
}

// writePortFile writes the allocated port to file and exports env var
func (pa *PortAllocator) writePortFile() error {
	// Export environment variable
	os.Setenv("BLUE_PROXY_PORT", fmt.Sprintf("%d", pa.port))

	portFile := pa.getPortFilePath()
	if portFile == "" {
		return nil
	}

	// Ensure directory exists
	dir := filepath.Dir(portFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Write port to file
	content := fmt.Sprintf("%d", pa.port)
	return os.WriteFile(portFile, []byte(content), 0644)
}

// getPortFilePath returns the port file path
func (pa *PortAllocator) getPortFilePath() string {
	if pa.config.PortFile != "" {
		return pa.config.PortFile
	}

	// Default path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(homeDir, ".local", "share", "zimaos-blue", "proxy.port")
}

// ReadPortFromFile reads the proxy port from file
func ReadPortFromFile(portFile string) (int, error) {
	if portFile == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return 0, err
		}
		portFile = filepath.Join(homeDir, ".local", "share", "zimaos-blue", "proxy.port")
	}

	data, err := os.ReadFile(portFile)
	if err != nil {
		return 0, err
	}

	port, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, err
	}

	return port, nil
}
