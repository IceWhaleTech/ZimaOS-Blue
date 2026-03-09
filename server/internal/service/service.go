// Package service provides cross-platform service management for ZimaOS-Blue.
// It supports running as a Windows Service, systemd service, or launchd daemon.
package service

import (
	"context"
	"os"
	"runtime"

	"golang.org/x/term"
)

// Service represents a system service that can be started, stopped, and managed.
type Service interface {
	// Run starts the service and blocks until it's stopped.
	Run(ctx context.Context) error

	// Stop gracefully stops the service.
	Stop() error

	// Status returns the current service status.
	Status() Status
}

// Status represents the current state of the service.
type Status int

const (
	StatusUnknown Status = iota
	StatusStopped
	StatusStarting
	StatusRunning
	StatusStopping
	StatusPaused
)

func (s Status) String() string {
	switch s {
	case StatusStopped:
		return "stopped"
	case StatusStarting:
		return "starting"
	case StatusRunning:
		return "running"
	case StatusStopping:
		return "stopping"
	case StatusPaused:
		return "paused"
	default:
		return "unknown"
	}
}

// Config holds the service configuration.
type Config struct {
	Name        string // Service name (e.g., "ZimaOS-Blue")
	DisplayName string // Display name shown in service manager
	Description string // Service description
	Executable  string // Path to the executable
	Arguments   []string
	WorkingDir  string
	StartType   StartType
	Account     string // Service account (Windows only)
	Password    string // Service account password (Windows only)
}

// StartType defines how the service should be started.
type StartType int

const (
	StartManual StartType = iota
	StartAutomatic
	StartDisabled
	StartDelayedAutomatic
)

// IsInteractive returns true if the process is running interactively (not as a service).
func IsInteractive() bool {
	// On Windows, check if we're running as a service
	if runtime.GOOS == "windows" {
		return isInteractiveWindows()
	}
	// On Unix-like systems, check if we have a controlling terminal
	return isInteractiveUnix()
}

// isInteractiveUnix checks if running interactively on Unix systems.
func isInteractiveUnix() bool {
	return isTerminalFile(os.Stdin)
}

func isTerminalFile(file *os.File) bool {
	if file == nil {
		return false
	}
	return term.IsTerminal(int(file.Fd()))
}

// DefaultConfig returns a default service configuration.
func DefaultConfig() *Config {
	return &Config{
		Name:        "ZimaOS-Blue",
		DisplayName: "ZimaOS Blue",
		Description: "ZimaOS Blue - A Local-first Agent Runtime for Builders with Bolder Mind",
		StartType:   StartAutomatic,
	}
}
