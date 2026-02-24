package mediagen

import (
	"fmt"
	"net"
	"strings"
)

// SimpleURLResolver resolves local API paths to externally-accessible URLs.
// It tries the tunnel URL first, then falls back to a LAN IP.
type SimpleURLResolver struct {
	TunnelGetURL func() string // returns tunnel URL or ""
	ServerPort   int
}

// ResolveExternalURL converts a local path like "/api/media/generated/images/xxx.png"
// to a full external URL using the tunnel or LAN IP.
func (r *SimpleURLResolver) ResolveExternalURL(localPath string) string {
	if r.TunnelGetURL != nil {
		if u := r.TunnelGetURL(); u != "" {
			return strings.TrimRight(u, "/") + localPath
		}
	}
	ip := detectLANIP()
	if ip == "" {
		return localPath // can't resolve, return as-is
	}
	port := r.ServerPort
	if port == 0 {
		port = 8080
	}
	return fmt.Sprintf("http://%s:%d%s", ip, port, localPath)
}

// detectLANIP returns the first non-loopback IPv4 address.
func detectLANIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok || ipNet.IP.IsLoopback() {
			continue
		}
		if ip4 := ipNet.IP.To4(); ip4 != nil {
			return ip4.String()
		}
	}
	return ""
}
