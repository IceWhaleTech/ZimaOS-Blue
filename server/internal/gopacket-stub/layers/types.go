package layers

import (
	"net"

	"github.com/google/gopacket"
)

// BaseLayer provides default implementations for layer methods.
type BaseLayer struct {
	Contents []byte
	Payload  []byte
}

func (b *BaseLayer) LayerContents() []byte { return b.Contents }
func (b *BaseLayer) LayerPayload() []byte  { return b.Payload }

// IPv6 represents an IPv6 packet header.
type IPv6 struct {
	BaseLayer
	Version    uint8
	SrcIP      net.IP
	DstIP      net.IP
	NextHeader IPProtocol
	HopLimit   uint8
}

// ICMPv4 represents an ICMPv4 message.
type ICMPv4 struct {
	BaseLayer
	TypeCode ICMPv4TypeCode
	Checksum uint16
	Id       uint16
	Seq      uint16
}

// ICMPv4TypeCode combines type and code for ICMPv4.
type ICMPv4TypeCode uint16

func (t ICMPv4TypeCode) Type() uint8 { return uint8(t >> 8) }
func (t ICMPv4TypeCode) Code() uint8 { return uint8(t) }

// ICMPv6 represents an ICMPv6 message.
type ICMPv6 struct {
	BaseLayer
	TypeCode ICMPv6TypeCode
	Checksum uint16
}

// ICMPv6TypeCode combines type and code for ICMPv6.
type ICMPv6TypeCode uint16

func (t ICMPv6TypeCode) Type() uint8 { return uint8(t >> 8) }
func (t ICMPv6TypeCode) Code() uint8 { return uint8(t) }

// Implement gopacket interfaces for all layer types.

func (l *IPv4) LayerType() gopacket.LayerType  { return LayerTypeIPv4 }
func (l *IPv4) CanDecode() gopacket.LayerClass { return nil }
func (l *IPv4) NextLayerType() gopacket.LayerType { return 0 }
func (l *IPv4) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error { return nil }
func (l *IPv4) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	return nil
}

func (l *IPv6) LayerType() gopacket.LayerType  { return LayerTypeIPv6 }
func (l *IPv6) CanDecode() gopacket.LayerClass { return nil }
func (l *IPv6) NextLayerType() gopacket.LayerType { return 0 }
func (l *IPv6) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error { return nil }
func (l *IPv6) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	return nil
}

func (l *ICMPv4) LayerType() gopacket.LayerType  { return LayerTypeICMPv4 }
func (l *ICMPv4) CanDecode() gopacket.LayerClass { return nil }
func (l *ICMPv4) NextLayerType() gopacket.LayerType { return 0 }
func (l *ICMPv4) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error { return nil }
func (l *ICMPv4) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	return nil
}

func (l *ICMPv6) LayerType() gopacket.LayerType  { return LayerTypeICMPv6 }
func (l *ICMPv6) CanDecode() gopacket.LayerClass { return nil }
func (l *ICMPv6) NextLayerType() gopacket.LayerType { return 0 }
func (l *ICMPv6) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error { return nil }
func (l *ICMPv6) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	return nil
}

// Ensure net is used.
var _ net.IP
