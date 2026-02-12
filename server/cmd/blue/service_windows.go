//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/service"
)

// ServiceCommands handles Windows service management commands.
type ServiceCommands struct {
	config *service.Config
}

// NewServiceCommands creates a new service commands handler.
func NewServiceCommands() *ServiceCommands {
	exePath, _ := os.Executable()
	installDir := filepath.Dir(filepath.Dir(exePath)) // Go up from bin/ to install dir
	configPath := filepath.Join(installDir, "config", "config.yaml")

	return &ServiceCommands{
		config: &service.Config{
			Name:        "ZimaOS-Blue",
			DisplayName: "ZimaOS Blue",
			Description: "ZimaOS Blue - NAS-Native Agent Runtime",
			Executable:  exePath,
			Arguments:   []string{"--config", configPath},
			WorkingDir:  installDir,
			StartType:   service.StartAutomatic,
		},
	}
}

// Install installs the Windows service.
func (sc *ServiceCommands) Install() error {
	fmt.Println("Installing ZimaOS-Blue service...")

	if err := service.Install(sc.config); err != nil {
		return fmt.Errorf("failed to install service: %w", err)
	}

	fmt.Println("Service installed successfully!")
	fmt.Println("")
	fmt.Println("To start the service, run:")
	fmt.Println("  net start ZimaOS-Blue")
	fmt.Println("  or")
	fmt.Println("  sc start ZimaOS-Blue")
	fmt.Println("")
	fmt.Println("To view service status:")
	fmt.Println("  sc query ZimaOS-Blue")
	fmt.Println("")
	fmt.Println("To view logs:")
	fmt.Println("  Get-EventLog -LogName Application -Source ZimaOS-Blue")

	return nil
}

// Uninstall removes the Windows service.
func (sc *ServiceCommands) Uninstall() error {
	fmt.Println("Uninstalling ZimaOS-Blue service...")

	if err := service.Uninstall(sc.config); err != nil {
		return fmt.Errorf("failed to uninstall service: %w", err)
	}

	fmt.Println("Service uninstalled successfully!")
	return nil
}

// Start starts the Windows service.
func (sc *ServiceCommands) Start() error {
	fmt.Println("Starting ZimaOS-Blue service...")

	if err := service.Start(sc.config); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	fmt.Println("Service started successfully!")
	return nil
}

// Stop stops the Windows service.
func (sc *ServiceCommands) Stop() error {
	fmt.Println("Stopping ZimaOS-Blue service...")

	if err := service.StopService(sc.config); err != nil {
		return fmt.Errorf("failed to stop service: %w", err)
	}

	fmt.Println("Service stopped successfully!")
	return nil
}

// Status queries the Windows service status.
func (sc *ServiceCommands) Status() error {
	status, err := service.QueryStatus(sc.config)
	if err != nil {
		return fmt.Errorf("failed to query service status: %w", err)
	}

	fmt.Printf("Service: %s\n", sc.config.Name)
	fmt.Printf("Status:  %s\n", status.String())
	return nil
}

// HandleServiceCommand handles service-related command line arguments.
// Returns true if a service command was handled.
func HandleServiceCommand(args []string) bool {
	if len(args) < 2 {
		return false
	}

	cmd := args[1]
	sc := NewServiceCommands()

	var err error
	switch cmd {
	case "install":
		err = sc.Install()
	case "uninstall", "remove":
		err = sc.Uninstall()
	case "start":
		err = sc.Start()
	case "stop":
		err = sc.Stop()
	case "status":
		err = sc.Status()
	case "restart":
		if err = sc.Stop(); err != nil {
			fmt.Printf("Warning: %v\n", err)
		}
		err = sc.Start()
	default:
		return false
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	return true
}

// PrintServiceHelp prints help for service commands.
func PrintServiceHelp() {
	fmt.Println("Windows Service Commands:")
	fmt.Println("  install    Install the Windows service")
	fmt.Println("  uninstall  Uninstall the Windows service")
	fmt.Println("  start      Start the Windows service")
	fmt.Println("  stop       Stop the Windows service")
	fmt.Println("  restart    Restart the Windows service")
	fmt.Println("  status     Query the Windows service status")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  echo.exe install")
	fmt.Println("  echo.exe start")
	fmt.Println("  echo.exe status")
}
