package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	gatewayPort int
	gatewayBind string
)

// gatewayCmd represents the gateway command
var gatewayCmd = &cobra.Command{
	Use:   "gateway",
	Short: "Service control commands",
	Long: `Control the ZimaOS-Echo service.

Subcommands:
  run       Run the service in foreground
  status    Show service status
  start     Start the service (background)
  stop      Stop the service
  restart   Restart the service
  install   Install as system service
  uninstall Uninstall system service`,
}

var gatewayRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the service in foreground",
	Run: func(cmd *cobra.Command, args []string) {
		// Override port if specified
		if gatewayPort != 0 {
			// TODO: Pass to server config
		}
		runServer()
	},
}

var gatewayStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show service status",
	Run: func(cmd *cobra.Command, args []string) {
		// Reuse status command logic
		runStatus(cmd, args)
	},
}

var gatewayStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the service (background)",
	Run: func(cmd *cobra.Command, args []string) {
		if jsonOutput {
			printJSON(map[string]interface{}{
				"success": false,
				"error":   "Use system service manager to start in background",
			})
		} else {
			fmt.Println("To start in background, use:")
			fmt.Println("  - Windows: echo install && net start ZimaOS-Echo")
			fmt.Println("  - Linux: systemctl start zimaos-echo")
			fmt.Println("  - macOS: launchctl load ~/Library/LaunchAgents/com.zimaos.echo.plist")
			fmt.Println()
			fmt.Println("Or run in foreground with: echo gateway run")
		}
	},
}

var gatewayStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the service",
	Run: func(cmd *cobra.Command, args []string) {
		if jsonOutput {
			printJSON(map[string]interface{}{
				"success": false,
				"error":   "Use system service manager to stop",
			})
		} else {
			fmt.Println("To stop the service, use:")
			fmt.Println("  - Windows: net stop ZimaOS-Echo")
			fmt.Println("  - Linux: systemctl stop zimaos-echo")
			fmt.Println("  - macOS: launchctl unload ~/Library/LaunchAgents/com.zimaos.echo.plist")
		}
	},
}

var gatewayRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the service",
	Run: func(cmd *cobra.Command, args []string) {
		if jsonOutput {
			printJSON(map[string]interface{}{
				"success": false,
				"error":   "Use system service manager to restart",
			})
		} else {
			fmt.Println("To restart the service, use:")
			fmt.Println("  - Windows: net stop ZimaOS-Echo && net start ZimaOS-Echo")
			fmt.Println("  - Linux: systemctl restart zimaos-echo")
			fmt.Println("  - macOS: launchctl unload && launchctl load ~/Library/LaunchAgents/com.zimaos.echo.plist")
		}
	},
}

var gatewayInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install as system service",
	Run: func(cmd *cobra.Command, args []string) {
		// Delegate to existing service install logic
		if HandleServiceCommand([]string{"", "install"}) {
			if jsonOutput {
				printJSON(map[string]interface{}{
					"success": true,
					"message": "Service installed",
				})
			}
		} else {
			if jsonOutput {
				printJSON(map[string]interface{}{
					"success": false,
					"error":   "Service installation not supported on this platform",
				})
			} else {
				fmt.Println("Service installation not supported on this platform")
			}
		}
	},
}

var gatewayUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall system service",
	Run: func(cmd *cobra.Command, args []string) {
		// Delegate to existing service uninstall logic
		if HandleServiceCommand([]string{"", "uninstall"}) {
			if jsonOutput {
				printJSON(map[string]interface{}{
					"success": true,
					"message": "Service uninstalled",
				})
			}
		} else {
			if jsonOutput {
				printJSON(map[string]interface{}{
					"success": false,
					"error":   "Service uninstallation not supported on this platform",
				})
			} else {
				fmt.Println("Service uninstallation not supported on this platform")
			}
		}
	},
}

func init() {
	// Gateway flags
	gatewayRunCmd.Flags().IntVar(&gatewayPort, "port", 0, "port to listen on (default 8080)")
	gatewayRunCmd.Flags().StringVar(&gatewayBind, "bind", "", "address to bind to (default 0.0.0.0)")

	// Add subcommands
	gatewayCmd.AddCommand(gatewayRunCmd)
	gatewayCmd.AddCommand(gatewayStatusCmd)
	gatewayCmd.AddCommand(gatewayStartCmd)
	gatewayCmd.AddCommand(gatewayStopCmd)
	gatewayCmd.AddCommand(gatewayRestartCmd)
	gatewayCmd.AddCommand(gatewayInstallCmd)
	gatewayCmd.AddCommand(gatewayUninstallCmd)
}
