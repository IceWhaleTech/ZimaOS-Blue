// Package layers is a minimal stub replacing github.com/google/gopacket/layers.
// Provides type definitions used by cloudflared's ICMP proxy (never executed).
package layers

import (
	"net"

	"github.com/google/gopacket"
)

// IPProtocol identifies the protocol encapsulated in an IP packet.
type IPProtocol uint8

const (
	IPProtocolICMPv4 IPProtocol = 1
	IPProtocolICMPv6 IPProtocol = 58
)

// LayerType constants.
var (
	LayerTypeIPv4   gopacket.LayerType = 20
	LayerTypeIPv6   gopacket.LayerType = 21
	LayerTypeICMPv4 gopacket.LayerType = 30
	LayerTypeICMPv6 gopacket.LayerType = 31
)

// IPv4 represents an IPv4 packet header.
type IPv4 struct {
	BaseLayer
	Version  uint8
	SrcIP    net.IP
	DstIP    net.IP
	Protocol IPProtocol
	TTL      uint8
}
