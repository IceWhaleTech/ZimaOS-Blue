//go:build darwin

package main

import (
	"log/slog"
	"os"
	"strings"

	serviceutil "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/service"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/speech"
)

// macosRequestSTTAuthorization requests speech recognition authorization
// on thread 0 before the server starts. Must be called from main().
func macosRequestSTTAuthorization() {
	if skip, reason := shouldSkipMacOSSTTAuthorization(os.Getenv, serviceutil.IsInteractive()); skip {
		slog.Info("[main] skipping macOS STT authorization", "reason", reason)
		return
	}
	status, err := speech.RequestSTTAuthorization()
	if err != nil {
		slog.Warn("[main] macOS STT authorization failed", "error", err)
		return
	}
	slog.Info("[main] macOS STT authorization complete", "status", status)
}

func shouldSkipMacOSSTTAuthorization(getenv func(string) string, interactive bool) (bool, string) {
	if getenv != nil {
		if v := strings.TrimSpace(getenv("ZIMA_SKIP_STT_AUTH")); v == "1" || strings.EqualFold(v, "true") {
			return true, "env:ZIMA_SKIP_STT_AUTH"
		}
	}
	if !interactive {
		return true, "non-interactive"
	}
	return false, ""
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
