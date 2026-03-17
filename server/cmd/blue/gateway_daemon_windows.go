//go:build windows

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func runGatewayStart(cmd *cobra.Command, args []string) {
	if HandleServiceCommand([]string{"", "start"}) {
		return
	}
	if jsonOutput {
		printJSON(map[string]interface{}{
			"success": false,
			"error":   "Use Windows Service mode on this platform",
		})
		return
	}
	fmt.Println("Use Windows Service mode on this platform:")
	fmt.Println("  blue install")
	fmt.Println("  net start ZimaOS-Blue")
}

func runGatewayStop(cmd *cobra.Command, args []string) {
	if HandleServiceCommand([]string{"", "stop"}) {
		return
	}
	if jsonOutput {
		printJSON(map[string]interface{}{
			"success": false,
			"error":   "Use Windows Service mode on this platform",
		})
		return
	}
	fmt.Println("Use Windows Service mode on this platform:")
	fmt.Println("  net stop ZimaOS-Blue")
}

func runGatewayRestart(cmd *cobra.Command, args []string) {
	if HandleServiceCommand([]string{"", "restart"}) {
		return
	}
	if jsonOutput {
		printJSON(map[string]interface{}{
			"success": false,
			"error":   "Use Windows Service mode on this platform",
		})
		return
	}
	fmt.Println("Use Windows Service mode on this platform:")
	fmt.Println("  net stop ZimaOS-Blue && net start ZimaOS-Blue")
}

func runGatewayStatus(cmd *cobra.Command, args []string) {
	runStatus(cmd, args)
}

func runGatewaySupervisor(cmd *cobra.Command, args []string) {
	fmt.Fprintln(os.Stderr, "gateway supervise is only available on Unix-like systems")
	os.Exit(1)
}
