package main

import (
	"fmt"
	"os"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sockipc"
	"github.com/spf13/cobra"
)

var (
	mediaToken    string
	mediaCategory string
	mediaModel    string
	mediaSize     string
	mediaPoll     bool
	mediaPollSec  int
)

var mediaCmd = &cobra.Command{
	Use:   "media",
	Short: "Media generation via socket IPC",
}

var mediaGenerateCmd = &cobra.Command{
	Use:   "generate [prompt]",
	Short: "Submit a media generation task",
	Args:  cobra.MinimumNArgs(1),
	Run:   runMediaGenerate,
}

var mediaStatusCmd = &cobra.Command{
	Use:   "status [task_id]",
	Short: "Query media task status",
	Args:  cobra.ExactArgs(1),
	Run:   runMediaStatus,
}

func init() {
	mediaGenerateCmd.Flags().StringVar(&mediaToken, "token", "", "API token (or BLUE_TOKEN env)")
	mediaGenerateCmd.Flags().StringVar(&mediaCategory, "category", "t2i", "category: t2i, t2v, i2v, i2i")
	mediaGenerateCmd.Flags().StringVar(&mediaModel, "model", "", "model name")
	mediaGenerateCmd.Flags().StringVar(&mediaSize, "size", "", "output size (e.g. 1024x1024)")
	mediaGenerateCmd.Flags().BoolVar(&mediaPoll, "poll", false, "poll until task completes")
	mediaGenerateCmd.Flags().IntVar(&mediaPollSec, "poll-interval", 2, "poll interval in seconds")

	mediaStatusCmd.Flags().StringVar(&mediaToken, "token", "", "API token (or BLUE_TOKEN env)")

	mediaCmd.AddCommand(mediaGenerateCmd)
	mediaCmd.AddCommand(mediaStatusCmd)
	rootCmd.AddCommand(mediaCmd)
}

func resolveMediaToken() string {
	if mediaToken != "" {
		return mediaToken
	}
	return os.Getenv("BLUE_TOKEN")
}

func runMediaGenerate(_ *cobra.Command, args []string) {
	prompt := args[0]
	token := resolveMediaToken()

	params := map[string]string{
		"prompt":   prompt,
		"category": mediaCategory,
	}
	if token != "" {
		params["token"] = token
	}
	if mediaModel != "" {
		params["model"] = mediaModel
	}
	if mediaSize != "" {
		params["size"] = mediaSize
	}

	resp, err := ipcRoundTrip(&sockipc.Request{Cmd: "media.generate", Params: params})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if resp.Status != "ok" {
		fmt.Fprintf(os.Stderr, "Error: %s\n", resp.Error)
		os.Exit(1)
	}

	taskID := resp.Data["task_id"]
	if jsonOutput {
		printJSON(map[string]string{"task_id": taskID, "status": "submitted"})
	} else {
		fmt.Printf("Task submitted: %s\n", taskID)
	}

	if mediaPoll {
		pollTask(taskID)
	}
}

func runMediaStatus(_ *cobra.Command, args []string) {
	taskID := args[0]
	token := resolveMediaToken()

	params := map[string]string{"task_id": taskID}
	if token != "" {
		params["token"] = token
	}

	resp, err := ipcRoundTrip(&sockipc.Request{Cmd: "media.status", Params: params})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if resp.Status != "ok" {
		fmt.Fprintf(os.Stderr, "Error: %s\n", resp.Error)
		os.Exit(1)
	}

	if jsonOutput {
		printJSON(resp.Data)
	} else {
		fmt.Printf("Task:     %s\n", resp.Data["task_id"])
		fmt.Printf("Status:   %s\n", resp.Data["task_status"])
		fmt.Printf("Progress: %s\n", resp.Data["progress"])
		if e := resp.Data["error"]; e != "" {
			fmt.Printf("Error:    %s\n", e)
		}
	}
}

func pollTask(taskID string) {
	interval := time.Duration(mediaPollSec) * time.Second
	for {
		time.Sleep(interval)

		params := map[string]string{"task_id": taskID}
		if t := resolveMediaToken(); t != "" {
			params["token"] = t
		}

		resp, err := ipcRoundTrip(&sockipc.Request{Cmd: "media.status", Params: params})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Poll error: %v\n", err)
			continue
		}

		taskStatus := resp.Data["task_status"]
		progress := resp.Data["progress"]

		if !jsonOutput {
			fmt.Printf("\r  %s  %s", taskStatus, progress)
		}

		switch taskStatus {
		case "succeeded", "failed", "cancelled":
			if !jsonOutput {
				fmt.Println()
			}
			if jsonOutput {
				printJSON(resp.Data)
			}
			if taskStatus != "succeeded" {
				os.Exit(1)
			}
			return
		}
	}
}
