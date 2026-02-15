// Package gopacket is a minimal stub replacing github.com/google/gopacket.
// The real gopacket/layers adds ~7.8MB to the binary but is only used by
// cloudflared's ICMP proxy code path, which is disabled (ICMPRouterServer=nil).
package gopacket

import "fmt"

// LayerType identifies the type of a layer.
type LayerType int

// String returns a string representation of the layer type.
func (l LayerType) String() string { return fmt.Sprintf("LayerType(%d)", int(l)) }

// DecodingLayer is implemented by layers that can decode themselves.
type DecodingLayer interface {
	CanDecode() LayerClass
	NextLayerType() LayerType
	DecodeFromBytes(data []byte, df DecodeFeedback) error
	LayerType() LayerType
}

// DecodeFeedback is used during decoding.
type DecodeFeedback interface {
	SetTruncated()
}

// LayerClass identifies a set of layer types.
type LayerClass interface {
	Contains(LayerType) bool
}

// SerializableLayer can serialize itself to a buffer.
type SerializableLayer interface {
	SerializeTo(b SerializeBuffer, opts SerializeOptions) error
	LayerType() LayerType
}

// SerializeBuffer is a buffer for serializing packet data.
type SerializeBuffer interface {
	Bytes() []byte
	PrependBytes(num int) ([]byte, error)
	AppendBytes(num int) ([]byte, error)
	Clear() error
}

// SerializeOptions controls serialization behavior.
type SerializeOptions struct {
	FixLengths       bool
	ComputeChecksums bool
}
