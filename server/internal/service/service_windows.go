//go:build windows

package service

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

// WindowsService implements the Service interface for Windows.
type WindowsService struct {
	config     *Config
	handler    func(ctx context.Context) error
	stopFunc   func()
	status     Status
	statusLock sync.RWMutex
	elog       *eventlog.Log
}

// NewWindowsService creates a new Windows service wrapper.
func NewWindowsService(config *Config, handler func(ctx context.Context) error, stopFunc func()) *WindowsService {
	return &WindowsService{
		config:   config,
		handler:  handler,
		stopFunc: stopFunc,
		status:   StatusStopped,
	}
}

// Run starts the service. If running interactively, it runs in debug mode.
func (s *WindowsService) Run(ctx context.Context) error {
	if IsInteractive() {
		return s.runInteractive(ctx)
	}
	return s.runService()
}

// runInteractive runs the service in interactive/debug mode.
func (s *WindowsService) runInteractive(ctx context.Context) error {
	s.setStatus(StatusStarting)
	s.setStatus(StatusRunning)
	defer s.setStatus(StatusStopped)

	return s.handler(ctx)
}

// runService runs as a Windows service.
func (s *WindowsService) runService() error {
	// Open event log for logging
	var err error
	s.elog, err = eventlog.Open(s.config.Name)
	if err != nil {
		// If event log doesn't exist, try to install it
		err = eventlog.InstallAsEventCreate(s.config.Name, eventlog.Error|eventlog.Warning|eventlog.Info)
		if err != nil {
			return fmt.Errorf("failed to install event log: %w", err)
		}
		s.elog, err = eventlog.Open(s.config.Name)
		if err != nil {
			return fmt.Errorf("failed to open event log: %w", err)
		}
	}
	defer s.elog.Close()

	s.elog.Info(1, fmt.Sprintf("%s service starting", s.config.Name))

	// Run the service
	err = svc.Run(s.config.Name, s)
	if err != nil {
		s.elog.Error(1, fmt.Sprintf("%s service failed: %v", s.config.Name, err))
		return fmt.Errorf("service run failed: %w", err)
	}

	s.elog.Info(1, fmt.Sprintf("%s service stopped", s.config.Name))
	return nil
}

// Execute implements svc.Handler interface.
func (s *WindowsService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown | svc.AcceptPauseAndContinue

	changes <- svc.Status{State: svc.StartPending}
	s.setStatus(StatusStarting)

	// Create context for the handler
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the main handler in a goroutine
	done := make(chan error, 1)
	go func() {
		done <- s.handler(ctx)
	}()

	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	s.setStatus(StatusRunning)

	if s.elog != nil {
		s.elog.Info(1, fmt.Sprintf("%s service is now running", s.config.Name))
	}

loop:
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus

			case svc.Stop, svc.Shutdown:
				if s.elog != nil {
					s.elog.Info(1, fmt.Sprintf("%s service received stop command", s.config.Name))
				}
				changes <- svc.Status{State: svc.StopPending}
				s.setStatus(StatusStopping)

				// Cancel context and call stop function
				cancel()
				if s.stopFunc != nil {
					s.stopFunc()
				}

				// Wait for handler to finish with timeout
				select {
				case <-done:
				case <-time.After(30 * time.Second):
					if s.elog != nil {
						s.elog.Warning(1, fmt.Sprintf("%s service shutdown timed out", s.config.Name))
					}
				}
				break loop

			case svc.Pause:
				changes <- svc.Status{State: svc.Paused, Accepts: cmdsAccepted}
				s.setStatus(StatusPaused)
				if s.elog != nil {
					s.elog.Info(1, fmt.Sprintf("%s service paused", s.config.Name))
				}

			case svc.Continue:
				changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
				s.setStatus(StatusRunning)
				if s.elog != nil {
					s.elog.Info(1, fmt.Sprintf("%s service resumed", s.config.Name))
				}

			default:
				if s.elog != nil {
					s.elog.Error(1, fmt.Sprintf("%s service received unexpected control request #%d", s.config.Name, c))
				}
			}

		case err := <-done:
			if err != nil && s.elog != nil {
				s.elog.Error(1, fmt.Sprintf("%s service handler error: %v", s.config.Name, err))
			}
			break loop
		}
	}

	changes <- svc.Status{State: svc.Stopped}
	s.setStatus(StatusStopped)
	return false, 0
}

// Stop gracefully stops the service.
func (s *WindowsService) Stop() error {
	if s.stopFunc != nil {
		s.stopFunc()
	}
	return nil
}

// Status returns the current service status.
func (s *WindowsService) Status() Status {
	s.statusLock.RLock()
	defer s.statusLock.RUnlock()
	return s.status
}

func (s *WindowsService) setStatus(status Status) {
	s.statusLock.Lock()
	defer s.statusLock.Unlock()
	s.status = status
}

// isInteractiveWindows checks if running interactively on Windows.
func isInteractiveWindows() bool {
	interactive, err := svc.IsWindowsService()
	if err != nil {
		// If we can't determine, assume interactive
		return true
	}
	return !interactive
}

// Install installs the service on Windows.
func Install(config *Config) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to service manager: %w", err)
	}
	defer m.Disconnect()

	// Check if service already exists
	s, err := m.OpenService(config.Name)
	if err == nil {
		s.Close()
		return fmt.Errorf("service %s already exists", config.Name)
	}

	// Determine executable path
	exePath := config.Executable
	if exePath == "" {
		exePath, err = os.Executable()
		if err != nil {
			return fmt.Errorf("failed to get executable path: %w", err)
		}
	}

	// Create the service
	svcConfig := mgr.Config{
		DisplayName:      config.DisplayName,
		Description:      config.Description,
		StartType:        convertStartType(config.StartType),
		ServiceStartName: config.Account,
		Password:         config.Password,
	}

	s, err = m.CreateService(config.Name, exePath, svcConfig, config.Arguments...)
	if err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}
	defer s.Close()

	// Configure recovery options
	recoveryActions := []mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: 5 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 10 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 30 * time.Second},
	}
	err = s.SetRecoveryActions(recoveryActions, 86400) // Reset after 24 hours
	if err != nil {
		// Non-fatal, just log
		fmt.Printf("Warning: failed to set recovery actions: %v\n", err)
	}

	// Install event log source
	err = eventlog.InstallAsEventCreate(config.Name, eventlog.Error|eventlog.Warning|eventlog.Info)
	if err != nil {
		// Non-fatal, just log
		fmt.Printf("Warning: failed to install event log: %v\n", err)
	}

	return nil
}

// Uninstall removes the service from Windows.
func Uninstall(config *Config) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(config.Name)
	if err != nil {
		return fmt.Errorf("service %s not found: %w", config.Name, err)
	}
	defer s.Close()

	// Stop the service if running
	status, err := s.Query()
	if err == nil && status.State != svc.Stopped {
		_, err = s.Control(svc.Stop)
		if err != nil {
			fmt.Printf("Warning: failed to stop service: %v\n", err)
		}
		// Wait for service to stop
		for i := 0; i < 30; i++ {
			status, err = s.Query()
			if err != nil || status.State == svc.Stopped {
				break
			}
			time.Sleep(time.Second)
		}
	}

	// Delete the service
	err = s.Delete()
	if err != nil {
		return fmt.Errorf("failed to delete service: %w", err)
	}

	// Remove event log source
	err = eventlog.Remove(config.Name)
	if err != nil {
		// Non-fatal, just log
		fmt.Printf("Warning: failed to remove event log: %v\n", err)
	}

	return nil
}

// Start starts the installed service.
func Start(config *Config) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(config.Name)
	if err != nil {
		return fmt.Errorf("service %s not found: %w", config.Name, err)
	}
	defer s.Close()

	err = s.Start()
	if err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	return nil
}

// StopService stops the installed service.
func StopService(config *Config) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(config.Name)
	if err != nil {
		return fmt.Errorf("service %s not found: %w", config.Name, err)
	}
	defer s.Close()

	status, err := s.Control(svc.Stop)
	if err != nil {
		return fmt.Errorf("failed to stop service: %w", err)
	}

	// Wait for service to stop
	timeout := time.Now().Add(30 * time.Second)
	for status.State != svc.Stopped {
		if time.Now().After(timeout) {
			return fmt.Errorf("timeout waiting for service to stop")
		}
		time.Sleep(500 * time.Millisecond)
		status, err = s.Query()
		if err != nil {
			return fmt.Errorf("failed to query service status: %w", err)
		}
	}

	return nil
}

// QueryStatus queries the current status of the installed service.
func QueryStatus(config *Config) (Status, error) {
	m, err := mgr.Connect()
	if err != nil {
		return StatusUnknown, fmt.Errorf("failed to connect to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(config.Name)
	if err != nil {
		return StatusUnknown, fmt.Errorf("service %s not found: %w", config.Name, err)
	}
	defer s.Close()

	status, err := s.Query()
	if err != nil {
		return StatusUnknown, fmt.Errorf("failed to query service status: %w", err)
	}

	return convertSvcState(status.State), nil
}

func convertStartType(st StartType) uint32 {
	switch st {
	case StartManual:
		return mgr.StartManual
	case StartAutomatic:
		return mgr.StartAutomatic
	case StartDisabled:
		return mgr.StartDisabled
	case StartDelayedAutomatic:
		return mgr.StartAutomatic // Will set delayed flag separately
	default:
		return mgr.StartAutomatic
	}
}

func convertSvcState(state svc.State) Status {
	switch state {
	case svc.Stopped:
		return StatusStopped
	case svc.StartPending:
		return StatusStarting
	case svc.StopPending:
		return StatusStopping
	case svc.Running:
		return StatusRunning
	case svc.ContinuePending:
		return StatusStarting
	case svc.PausePending:
		return StatusStopping
	case svc.Paused:
		return StatusPaused
	default:
		return StatusUnknown
	}
}

// EventLog provides Windows Event Log integration.
type EventLog struct {
	log *eventlog.Log
}

// NewEventLog creates a new event log writer.
func NewEventLog(name string) (*EventLog, error) {
	log, err := eventlog.Open(name)
	if err != nil {
		// Try to install the event source
		err = eventlog.InstallAsEventCreate(name, eventlog.Error|eventlog.Warning|eventlog.Info)
		if err != nil {
			return nil, fmt.Errorf("failed to install event log: %w", err)
		}
		log, err = eventlog.Open(name)
		if err != nil {
			return nil, fmt.Errorf("failed to open event log: %w", err)
		}
	}
	return &EventLog{log: log}, nil
}

// Close closes the event log.
func (e *EventLog) Close() error {
	if e.log != nil {
		return e.log.Close()
	}
	return nil
}

// Info logs an informational message.
func (e *EventLog) Info(eventID uint32, msg string) error {
	if e.log != nil {
		return e.log.Info(eventID, msg)
	}
	return nil
}

// Warning logs a warning message.
func (e *EventLog) Warning(eventID uint32, msg string) error {
	if e.log != nil {
		return e.log.Warning(eventID, msg)
	}
	return nil
}

// Error logs an error message.
func (e *EventLog) Error(eventID uint32, msg string) error {
	if e.log != nil {
		return e.log.Error(eventID, msg)
	}
	return nil
}

// Ensure WindowsService implements svc.Handler
var _ svc.Handler = (*WindowsService)(nil)

// Ensure debug.Log is available for testing
var _ debug.Log = (*eventlog.Log)(nil)
