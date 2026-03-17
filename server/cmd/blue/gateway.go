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
	Long: `Control the ZimaOS-Blue service.

Subcommands:
  run        Run the service in foreground
  daemon     Start the managed background daemon
  start      Alias for daemon
  status     Show daemon/service status
  stop       Stop the daemon or running service
  restart    Restart the daemon
  install    Install as system service
  uninstall  Uninstall system service`,
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
		runGatewayStatus(cmd, args)
	},
}

var gatewayStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the service in daemon mode",
	Run: func(cmd *cobra.Command, args []string) {
		runGatewayStart(cmd, args)
	},
}

var gatewayStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the service",
	Run: func(cmd *cobra.Command, args []string) {
		runGatewayStop(cmd, args)
	},
}

var gatewayRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the service",
	Run: func(cmd *cobra.Command, args []string) {
		runGatewayRestart(cmd, args)
	},
}

var gatewayDaemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Start the managed background daemon",
	Run: func(cmd *cobra.Command, args []string) {
		runGatewayStart(cmd, args)
	},
}

var gatewaySuperviseCmd = &cobra.Command{
	Use:    "supervise",
	Short:  "Run the internal gateway supervisor",
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		runGatewaySupervisor(cmd, args)
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
	gatewayCmd.AddCommand(gatewayDaemonCmd)
	gatewayCmd.AddCommand(gatewayStatusCmd)
	gatewayCmd.AddCommand(gatewayStartCmd)
	gatewayCmd.AddCommand(gatewayStopCmd)
	gatewayCmd.AddCommand(gatewayRestartCmd)
	gatewayCmd.AddCommand(gatewayInstallCmd)
	gatewayCmd.AddCommand(gatewayUninstallCmd)
	gatewayCmd.AddCommand(gatewaySuperviseCmd)
}
