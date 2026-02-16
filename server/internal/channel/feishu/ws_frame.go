package feishu

// Lightweight protobuf frame codec for Feishu WebSocket protocol (pbbp2).
// Copied from larksuite/oapi-sdk-go/v3/ws/pbbp2.pb.go with gogo/protobuf dependency removed.
// Only Marshal/Unmarshal/Size methods are needed — no proto.Register* calls.

import (
	"fmt"
	"io"
	"math/bits"
)

type wsHeader struct {
	Key   string
	Value string
}

type wsFrame struct {
	SeqID           uint64
	LogID           uint64
	Service         int32
	Method          int32
	Headers         []wsHeader
	PayloadEncoding string
	PayloadType     string
	Payload         []byte
	LogIDNew        string
}

// wsHeaders helpers

type wsHeaders []wsHeader

func (h wsHeaders) getString(key string) string {
	for _, hdr := range h {
		if hdr.Key == key {
			return hdr.Value
		}
	}
	return ""
}

func (h wsHeaders) getInt(key string) int {
	for _, hdr := range h {
		if hdr.Key == key {
			v := 0
			for _, c := range hdr.Value {
				if c >= '0' && c <= '9' {
					v = v*10 + int(c-'0')
				}
			}
			return v
		}
	}
	return 0
}

func (h *wsHeaders) add(key, value string) {
	*h = append(*h, wsHeader{Key: key, Value: value})
}

// Marshal / Unmarshal — protobuf wire format, hand-rolled (no external deps)

func (m *wsHeader) marshal() ([]byte, error) {
	size := m.size()
	buf := make([]byte, size)
	n, err := m.marshalToSizedBuffer(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func (m *wsHeader) marshalToSizedBuffer(buf []byte) (int, error) {
	i := len(buf)
	i -= len(m.Value)
	copy(buf[i:], m.Value)
	i = encodeVarint(buf, i, uint64(len(m.Value)))
	i--
	buf[i] = 0x12
	i -= len(m.Key)
	copy(buf[i:], m.Key)
	i = encodeVarint(buf, i, uint64(len(m.Key)))
	i--
	buf[i] = 0xa
	return len(buf) - i, nil
}

func (m *wsHeader) size() int {
	n := 0
	l := len(m.Key)
	n += 1 + l + sov(uint64(l))
	l = len(m.Value)
	n += 1 + l + sov(uint64(l))
	return n
}

func (m *wsFrame) marshal() ([]byte, error) {
	size := m.size()
	buf := make([]byte, size)
	n, err := m.marshalToSizedBuffer(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func (m *wsFrame) marshalToSizedBuffer(buf []byte) (int, error) {
	i := len(buf)
	i -= len(m.LogIDNew)
	copy(buf[i:], m.LogIDNew)
	i = encodeVarint(buf, i, uint64(len(m.LogIDNew)))
	i--
	buf[i] = 0x4a
	if m.Payload != nil {
		i -= len(m.Payload)
		copy(buf[i:], m.Payload)
		i = encodeVarint(buf, i, uint64(len(m.Payload)))
		i--
		buf[i] = 0x42
	}
	i -= len(m.PayloadType)
	copy(buf[i:], m.PayloadType)
	i = encodeVarint(buf, i, uint64(len(m.PayloadType)))
	i--
	buf[i] = 0x3a
	i -= len(m.PayloadEncoding)
	copy(buf[i:], m.PayloadEncoding)
	i = encodeVarint(buf, i, uint64(len(m.PayloadEncoding)))
	i--
	buf[i] = 0x32
	if len(m.Headers) > 0 {
		for idx := len(m.Headers) - 1; idx >= 0; idx-- {
			size, err := m.Headers[idx].marshalToSizedBuffer(buf[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarint(buf, i, uint64(size))
			i--
			buf[i] = 0x2a
		}
	}
	i = encodeVarint(buf, i, uint64(m.Method))
	i--
	buf[i] = 0x20
	i = encodeVarint(buf, i, uint64(m.Service))
	i--
	buf[i] = 0x18
	i = encodeVarint(buf, i, uint64(m.LogID))
	i--
	buf[i] = 0x10
	i = encodeVarint(buf, i, uint64(m.SeqID))
	i--
	buf[i] = 0x8
	return len(buf) - i, nil
}

func (m *wsFrame) size() int {
	n := 0
	n += 1 + sov(m.SeqID)
	n += 1 + sov(m.LogID)
	n += 1 + sov(uint64(m.Service))
	n += 1 + sov(uint64(m.Method))
	for _, e := range m.Headers {
		l := e.size()
		n += 1 + l + sov(uint64(l))
	}
	l := len(m.PayloadEncoding)
	n += 1 + l + sov(uint64(l))
	l = len(m.PayloadType)
	n += 1 + l + sov(uint64(l))
	if m.Payload != nil {
		l = len(m.Payload)
		n += 1 + l + sov(uint64(l))
	}
	l = len(m.LogIDNew)
	n += 1 + l + sov(uint64(l))
	return n
}

func (m *wsFrame) unmarshal(data []byte) error {
	l := len(data)
	idx := 0
	for idx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return errOverflow
			}
			if idx >= l {
				return io.ErrUnexpectedEOF
			}
			b := data[idx]
			idx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		switch fieldNum {
		case 1: // SeqID
			if wireType != 0 {
				return fmt.Errorf("wrong wireType for SeqID")
			}
			m.SeqID, idx = decodeVarintU64(data, idx)
		case 2: // LogID
			if wireType != 0 {
				return fmt.Errorf("wrong wireType for LogID")
			}
			m.LogID, idx = decodeVarintU64(data, idx)
		case 3: // Service
			if wireType != 0 {
				return fmt.Errorf("wrong wireType for Service")
			}
			v, ni := decodeVarintU64(data, idx)
			m.Service = int32(v)
			idx = ni
		case 4: // Method
			if wireType != 0 {
				return fmt.Errorf("wrong wireType for Method")
			}
			v, ni := decodeVarintU64(data, idx)
			m.Method = int32(v)
			idx = ni
		case 5: // Headers
			if wireType != 2 {
				return fmt.Errorf("wrong wireType for Headers")
			}
			msgLen, ni := decodeVarintInt(data, idx)
			idx = ni
			if msgLen < 0 || idx+msgLen > l {
				return io.ErrUnexpectedEOF
			}
			var h wsHeader
			if err := h.unmarshal(data[idx : idx+msgLen]); err != nil {
				return err
			}
			m.Headers = append(m.Headers, h)
			idx += msgLen
		case 6: // PayloadEncoding
			if wireType != 2 {
				return fmt.Errorf("wrong wireType for PayloadEncoding")
			}
			sLen, ni := decodeVarintInt(data, idx)
			idx = ni
			if sLen < 0 || idx+sLen > l {
				return io.ErrUnexpectedEOF
			}
			m.PayloadEncoding = string(data[idx : idx+sLen])
			idx += sLen
		case 7: // PayloadType
			if wireType != 2 {
				return fmt.Errorf("wrong wireType for PayloadType")
			}
			sLen, ni := decodeVarintInt(data, idx)
			idx = ni
			if sLen < 0 || idx+sLen > l {
				return io.ErrUnexpectedEOF
			}
			m.PayloadType = string(data[idx : idx+sLen])
			idx += sLen
		case 8: // Payload
			if wireType != 2 {
				return fmt.Errorf("wrong wireType for Payload")
			}
			bLen, ni := decodeVarintInt(data, idx)
			idx = ni
			if bLen < 0 || idx+bLen > l {
				return io.ErrUnexpectedEOF
			}
			m.Payload = append(m.Payload[:0], data[idx:idx+bLen]...)
			idx += bLen
		case 9: // LogIDNew
			if wireType != 2 {
				return fmt.Errorf("wrong wireType for LogIDNew")
			}
			sLen, ni := decodeVarintInt(data, idx)
			idx = ni
			if sLen < 0 || idx+sLen > l {
				return io.ErrUnexpectedEOF
			}
			m.LogIDNew = string(data[idx : idx+sLen])
			idx += sLen
		default:
			// skip unknown field
			if err := skipField(data, &idx, wireType); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *wsHeader) unmarshal(data []byte) error {
	l := len(data)
	idx := 0
	for idx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if idx >= l {
				return io.ErrUnexpectedEOF
			}
			b := data[idx]
			idx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		switch fieldNum {
		case 1: // Key
			if wireType != 2 {
				return fmt.Errorf("wrong wireType for Key")
			}
			sLen, ni := decodeVarintInt(data, idx)
			idx = ni
			if sLen < 0 || idx+sLen > l {
				return io.ErrUnexpectedEOF
			}
			m.Key = string(data[idx : idx+sLen])
			idx += sLen
		case 2: // Value
			if wireType != 2 {
				return fmt.Errorf("wrong wireType for Value")
			}
			sLen, ni := decodeVarintInt(data, idx)
			idx = ni
			if sLen < 0 || idx+sLen > l {
				return io.ErrUnexpectedEOF
			}
			m.Value = string(data[idx : idx+sLen])
			idx += sLen
		default:
			if err := skipField(data, &idx, wireType); err != nil {
				return err
			}
		}
	}
	return nil
}

// protobuf encoding helpers

func encodeVarint(buf []byte, offset int, v uint64) int {
	offset -= sov(v)
	base := offset
	for v >= 1<<7 {
		buf[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	buf[offset] = uint8(v)
	return base
}

func sov(x uint64) int {
	return (bits.Len64(x|1) + 6) / 7
}

func decodeVarintU64(data []byte, idx int) (uint64, int) {
	var v uint64
	for shift := uint(0); shift < 64; shift += 7 {
		if idx >= len(data) {
			return v, idx
		}
		b := data[idx]
		idx++
		v |= uint64(b&0x7F) << shift
		if b < 0x80 {
			break
		}
	}
	return v, idx
}

func decodeVarintInt(data []byte, idx int) (int, int) {
	v, ni := decodeVarintU64(data, idx)
	return int(v), ni
}

func skipField(data []byte, idx *int, wireType int) error {
	switch wireType {
	case 0: // varint
		for *idx < len(data) {
			b := data[*idx]
			*idx++
			if b < 0x80 {
				return nil
			}
		}
		return io.ErrUnexpectedEOF
	case 1: // 64-bit
		*idx += 8
	case 2: // length-delimited
		length, ni := decodeVarintInt(data, *idx)
		*idx = ni + length
	case 5: // 32-bit
		*idx += 4
	default:
		return fmt.Errorf("unknown wireType %d", wireType)
	}
	if *idx > len(data) {
		return io.ErrUnexpectedEOF
	}
	return nil
}

var errOverflow = fmt.Errorf("proto: integer overflow")
