package main

import (
	"os"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/logger"
	blueServer "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
)

var (
	runServerEntry        = runServer
	setStartupURLHandler  = blueServer.SetStartupURLHandler
	openStartupBrowserURL = openBrowserURL
)

func shouldAutoOpenStartupBrowser() bool {
	if jsonOutput {
		return false
	}
	return strings.TrimSpace(os.Getenv("BLUE_GATEWAY_SUPERVISOR")) == ""
}

func runForegroundServer() {
	if shouldAutoOpenStartupBrowser() {
		setStartupURLHandler(func(rawURL string) {
			if err := openStartupBrowserURL(rawURL); err != nil {
				logger.Warn().Err(err).Str("url", rawURL).Msg("Failed to open startup page in browser")
			}
		})
	} else {
		setStartupURLHandler(nil)
	}

	runServerEntry()
}
