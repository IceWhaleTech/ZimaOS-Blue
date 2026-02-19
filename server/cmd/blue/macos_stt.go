//go:build darwin

package main

import (
	"log/slog"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/speech"
)

// macosRequestSTTAuthorization requests speech recognition authorization
// on thread 0 before the server starts. Must be called from main().
func macosRequestSTTAuthorization() {
	status, err := speech.RequestSTTAuthorization()
	if err != nil {
		slog.Warn("[main] macOS STT authorization failed", "error", err)
		return
	}
	slog.Info("[main] macOS STT authorization complete", "status", status)
}

// macosRunMainRunLoop pumps the Cocoa main run loop on thread 0 forever.
// The server must be started on a goroutine before calling this.
// This function never returns.
func macosRunMainRunLoop() {
	speech.RunMainRunLoop()
}

func macosStopMainRunLoop() {
	speech.StopMainRunLoop()
}

func isDarwin() bool { return true }
