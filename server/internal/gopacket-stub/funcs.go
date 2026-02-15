package gopacket

import "fmt"

// Payload is a layer containing raw payload data.
type Payload []byte

func (p Payload) LayerType() LayerType                                        { return 0 }
func (p Payload) CanDecode() LayerClass                                       { return nil }
func (p Payload) NextLayerType() LayerType                                    { return 0 }
func (p Payload) DecodeFromBytes(data []byte, df DecodeFeedback) error        { return nil }
func (p Payload) SerializeTo(b SerializeBuffer, opts SerializeOptions) error   { return nil }
func (p Payload) LayerContents() []byte                                       { return []byte(p) }
func (p Payload) LayerPayload() []byte                                        { return nil }

// DecodingLayerContainer maps layer types to decoding layers.
type DecodingLayerContainer interface{}

// DecodingLayerSparse creates a sparse decoding layer container.
func DecodingLayerSparse(cap interface{}) DecodingLayerContainer { return nil }

// DecodingLayerArray creates an array-based decoding layer container.
func DecodingLayerArray(dl DecodingLayer) DecodingLayerContainer { return nil }

// serializeBuffer implements SerializeBuffer.
type serializeBuffer struct {
	data []byte
}

func (s *serializeBuffer) Bytes() []byte                    { return s.data }
func (s *serializeBuffer) PrependBytes(num int) ([]byte, error) {
	b := make([]byte, num)
	s.data = append(b, s.data...)
	return b, nil
}
func (s *serializeBuffer) AppendBytes(num int) ([]byte, error) {
	b := make([]byte, num)
	s.data = append(s.data, b...)
	return b, nil
}
func (s *serializeBuffer) Clear() error { s.data = s.data[:0]; return nil }

// NewSerializeBuffer creates a new SerializeBuffer.
func NewSerializeBuffer() SerializeBuffer {
	return &serializeBuffer{}
}

// SerializeLayers serializes layers into a buffer.
func SerializeLayers(buf SerializeBuffer, opts SerializeOptions, layers ...SerializableLayer) error {
	return fmt.Errorf("gopacket stub: SerializeLayers not implemented")
}

// DecodingLayerParser decodes packet layers.
type DecodingLayerParser struct {
	IgnoreUnsupported bool
}

// NewDecodingLayerParser creates a new parser.
func NewDecodingLayerParser(first LayerType, decoders ...DecodingLayer) *DecodingLayerParser {
	return &DecodingLayerParser{}
}

func (p *DecodingLayerParser) SetDecodingLayerContainer(dlc DecodingLayerContainer) {}
func (p *DecodingLayerParser) AddDecodingLayer(d DecodingLayer)                     {}
func (p *DecodingLayerParser) DecodeLayers(data []byte, decoded *[]LayerType) error {
	return fmt.Errorf("gopacket stub: DecodeLayers not implemented")
}

// Ensure fmt is used.
var _ = fmt.Errorf
