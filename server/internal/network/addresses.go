package network

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"sort"
	"strings"
)

// InterfaceType represents the type of network interface.
type InterfaceType string

const (
	InterfaceTypeWiFi     InterfaceType = "wifi"
	InterfaceTypeEthernet InterfaceType = "ethernet"
	InterfaceTypeLoopback InterfaceType = "loopback"
	InterfaceTypeVirtual  InterfaceType = "virtual"
	InterfaceTypeUnknown  InterfaceType = "unknown"
)

// NetworkInterface represents a network interface with its address.
type NetworkInterface struct {
	Name    string        `json:"interface"`
	Address string        `json:"address"`
	Type    InterfaceType `json:"type"`
	IsUp    bool          `json:"is_up"`
	IsIPv6  bool          `json:"is_ipv6"`
}

// NetworkAddresses contains all available network addresses.
type NetworkAddresses struct {
	Local     string             `json:"local"`
	LAN       []NetworkInterface `json:"lan"`
	Hostname  string             `json:"hostname,omitempty"`
	Port      int                `json:"port"`
	Preferred string             `json:"preferred"`
}

// AddressDetector detects network addresses.
type AddressDetector struct {
	port int
}

// NewAddressDetector creates a new address detector.
func NewAddressDetector(port int) *AddressDetector {
	return &AddressDetector{port: port}
}

// GetAddresses returns all available network addresses.
func (d *AddressDetector) GetAddresses() (*NetworkAddresses, error) {
	addresses := &NetworkAddresses{
		Local: fmt.Sprintf("http://localhost:%d", d.port),
		Port:  d.port,
		LAN:   make([]NetworkInterface, 0),
	}

	// Get hostname
	hostname, err := os.Hostname()
	if err == nil && hostname != "" {
		// Add .local suffix for mDNS/Bonjour
		if !strings.Contains(hostname, ".") {
			addresses.Hostname = fmt.Sprintf("http://%s.local:%d", hostname, d.port)
		} else {
			addresses.Hostname = fmt.Sprintf("http://%s:%d", hostname, d.port)
		}
	}

	// Get network interfaces
	interfaces, err := net.Interfaces()
	if err != nil {
		return addresses, err
	}

	for _, iface := range interfaces {
		// Skip down interfaces
		if iface.Flags&net.FlagUp == 0 {
			continue
		}

		// Skip loopback
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil {
				continue
			}

			// Skip loopback addresses
			if ip.IsLoopback() {
				continue
			}

			// Skip link-local addresses
			if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
				continue
			}

			isIPv6 := ip.To4() == nil

			// For now, prefer IPv4 addresses
			if isIPv6 {
				continue
			}

			netInterface := NetworkInterface{
				Name:    iface.Name,
				Address: fmt.Sprintf("http://%s:%d", ip.String(), d.port),
				Type:    detectInterfaceType(iface.Name),
				IsUp:    true,
				IsIPv6:  isIPv6,
			}

			addresses.LAN = append(addresses.LAN, netInterface)
		}
	}

	// Sort interfaces: WiFi first, then Ethernet, then others
	sort.Slice(addresses.LAN, func(i, j int) bool {
		return interfaceTypePriority(addresses.LAN[i].Type) < interfaceTypePriority(addresses.LAN[j].Type)
	})

	// Set preferred address
	if len(addresses.LAN) > 0 {
		addresses.Preferred = addresses.LAN[0].Address
	} else {
		addresses.Preferred = addresses.Local
	}

	return addresses, nil
}

// GetPreferredAddress returns the best address for sharing.
func (d *AddressDetector) GetPreferredAddress() (string, error) {
	addresses, err := d.GetAddresses()
	if err != nil {
		return "", err
	}
	return addresses.Preferred, nil
}

// detectInterfaceType tries to determine the interface type from its name.
func detectInterfaceType(name string) InterfaceType {
	nameLower := strings.ToLower(name)

	// Check for virtual interfaces first (they can have misleading names)
	if strings.Contains(nameLower, "docker") || strings.HasPrefix(nameLower, "veth") || strings.HasPrefix(nameLower, "br-") || strings.Contains(nameLower, "virbr") {
		return InterfaceTypeVirtual
	}

	// Check for loopback
	if nameLower == "lo" || nameLower == "lo0" || strings.Contains(nameLower, "loopback") {
		return InterfaceTypeLoopback
	}

	// macOS
	if runtime.GOOS == "darwin" {
		if strings.HasPrefix(nameLower, "en0") {
			return InterfaceTypeWiFi
		}
		if strings.HasPrefix(nameLower, "en") {
			return InterfaceTypeEthernet
		}
	}

	// Linux
	if runtime.GOOS == "linux" {
		if strings.HasPrefix(nameLower, "wl") || strings.Contains(nameLower, "wifi") || strings.Contains(nameLower, "wlan") {
			return InterfaceTypeWiFi
		}
		if strings.HasPrefix(nameLower, "eth") || strings.HasPrefix(nameLower, "enp") || strings.HasPrefix(nameLower, "ens") {
			return InterfaceTypeEthernet
		}
	}

	// Windows
	if runtime.GOOS == "windows" {
		if strings.Contains(nameLower, "wi-fi") || strings.Contains(nameLower, "wifi") || strings.Contains(nameLower, "wireless") {
			return InterfaceTypeWiFi
		}
		if strings.Contains(nameLower, "ethernet") || strings.Contains(nameLower, "local area") {
			return InterfaceTypeEthernet
		}
	}

	// Generic detection (cross-platform)
	if strings.HasPrefix(nameLower, "wl") || strings.Contains(nameLower, "wifi") || strings.Contains(nameLower, "wlan") || strings.Contains(nameLower, "wireless") {
		return InterfaceTypeWiFi
	}
	if strings.HasPrefix(nameLower, "eth") || strings.HasPrefix(nameLower, "enp") || strings.HasPrefix(nameLower, "ens") {
		return InterfaceTypeEthernet
	}

	return InterfaceTypeUnknown
}

// interfaceTypePriority returns the priority of an interface type (lower is better).
func interfaceTypePriority(t InterfaceType) int {
	switch t {
	case InterfaceTypeWiFi:
		return 0
	case InterfaceTypeEthernet:
		return 1
	case InterfaceTypeUnknown:
		return 2
	case InterfaceTypeVirtual:
		return 3
	case InterfaceTypeLoopback:
		return 4
	default:
		return 5
	}
}
