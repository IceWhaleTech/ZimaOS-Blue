//go:build !windows

package service

import (
	"context"
	"sync"
)

// isInteractiveWindows is a stub for Unix systems.
// On Unix, we use isInteractiveUnix from service.go instead.
func isInteractiveWindows() bool {
	return true // Always returns true on Unix (not applicable)
}

// UnixService implements the Service interface for Unix-like systems.
// On Unix, the service is typically managed by systemd or launchd,
// so this is a simple wrapper that just runs the handler.
type UnixService struct {
	config     *Config
	handler    func(ctx context.Context) error
	stopFunc   func()
	status     Status
	statusLock sync.RWMutex
}

// NewUnixService creates a new Unix service wrapper.
func NewUnixService(config *Config, handler func(ctx context.Context) error, stopFunc func()) *UnixService {
	return &UnixService{
		config:   config,
		handler:  handler,
		stopFunc: stopFunc,
		status:   StatusStopped,
	}
}

// Run starts the service.
func (s *UnixService) Run(ctx context.Context) error {
	s.setStatus(StatusStarting)
	s.setStatus(StatusRunning)
	defer s.setStatus(StatusStopped)

	return s.handler(ctx)
}

// Stop gracefully stops the service.
func (s *UnixService) Stop() error {
	s.setStatus(StatusStopping)
	if s.stopFunc != nil {
		s.stopFunc()
	}
	return nil
}

// Status returns the current service status.
func (s *UnixService) Status() Status {
	s.statusLock.RLock()
	defer s.statusLock.RUnlock()
	return s.status
}

func (s *UnixService) setStatus(status Status) {
	s.statusLock.Lock()
	defer s.statusLock.Unlock()
	s.status = status
}

// Install is a no-op on Unix (use systemd/launchd directly).
func Install(config *Config) error {
	// On Unix, installation is done via systemd/launchd configuration files
	return nil
}

// Uninstall is a no-op on Unix (use systemd/launchd directly).
func Uninstall(config *Config) error {
	// On Unix, uninstallation is done via systemd/launchd configuration files
	return nil
}

// Start is a no-op on Unix (use systemctl/launchctl directly).
func Start(config *Config) error {
	// On Unix, use: systemctl start zimaos-echo
	return nil
}

// StopService is a no-op on Unix (use systemctl/launchctl directly).
func StopService(config *Config) error {
	// On Unix, use: systemctl stop zimaos-echo
	return nil
}

// QueryStatus is a no-op on Unix (use systemctl/launchctl directly).
func QueryStatus(config *Config) (Status, error) {
	// On Unix, use: systemctl status zimaos-echo
	return StatusUnknown, nil
}

// EventLog is a no-op on Unix (use syslog/journald).
type EventLog struct{}

// NewEventLog creates a new event log writer (no-op on Unix).
func NewEventLog(name string) (*EventLog, error) {
	return &EventLog{}, nil
}

// Close closes the event log (no-op on Unix).
func (e *EventLog) Close() error {
	return nil
}

// Info logs an informational message (no-op on Unix).
func (e *EventLog) Info(eventID uint32, msg string) error {
	return nil
}

// Warning logs a warning message (no-op on Unix).
func (e *EventLog) Warning(eventID uint32, msg string) error {
	return nil
}

// Error logs an error message (no-op on Unix).
func (e *EventLog) Error(eventID uint32, msg string) error {
	return nil
}
