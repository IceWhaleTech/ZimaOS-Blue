package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Run: func(cmd *cobra.Command, args []string) {
		if jsonOutput {
			printJSON(map[string]interface{}{
				"version":    version,
				"build_time": buildTime,
				"git_commit": gitCommit,
			})
		} else {
			fmt.Printf("ZimaOS-Echo %s\n", version)
			fmt.Printf("Build time: %s\n", buildTime)
			fmt.Printf("Git commit: %s\n", gitCommit)
		}
	},
}
