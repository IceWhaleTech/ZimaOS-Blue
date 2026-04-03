package server

import (
	"net"
	"strconv"
	"strings"
	"sync"
)

var (
	startupURLHandlerMu sync.Mutex
	startupURLHandler   func(string)
)

// SetStartupURLHandler configures a one-shot callback invoked when the HTTP
// server has determined its actual listening port. Passing nil clears it.
func SetStartupURLHandler(handler func(string)) {
	startupURLHandlerMu.Lock()
	defer startupURLHandlerMu.Unlock()
	startupURLHandler = handler
}

func takeStartupURLHandler() func(string) {
	startupURLHandlerMu.Lock()
	defer startupURLHandlerMu.Unlock()
	handler := startupURLHandler
	startupURLHandler = nil
	return handler
}

func handleStartupURL(host string, port int) {
	handler := takeStartupURLHandler()
	if handler == nil {
		return
	}
	handler(buildStartupURL(host, port))
}

func buildStartupURL(host string, port int) string {
	return "http://" + net.JoinHostPort(normalizeStartupURLHost(host), strconv.Itoa(port))
}

func normalizeStartupURLHost(raw string) string {
	host := strings.TrimSpace(raw)
	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")
	switch host {
	case "", "0.0.0.0", "::":
		return "localhost"
	default:
		return host
	}
}
