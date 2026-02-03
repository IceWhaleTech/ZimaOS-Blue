package server

import (
	"bytes"
	"encoding/json"
	"sync"
)

// FastJSONEncoder provides optimized JSON encoding with buffer pooling
type FastJSONEncoder struct {
	bufferPool *sync.Pool
}

// NewFastJSONEncoder creates a new fast JSON encoder
func NewFastJSONEncoder() *FastJSONEncoder {
	return &FastJSONEncoder{
		bufferPool: &sync.Pool{
			New: func() interface{} {
				return new(bytes.Buffer)
			},
		},
	}
}

// Encode encodes a value to JSON using a pooled buffer
func (fje *FastJSONEncoder) Encode(v interface{}) ([]byte, error) {
	buf := fje.bufferPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		fje.bufferPool.Put(buf)
	}()

	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false) // Faster encoding
	if err := encoder.Encode(v); err != nil {
		return nil, err
	}

	// Remove trailing newline added by Encoder
	result := buf.Bytes()
	if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}

	// Copy to avoid buffer reuse issues
	return bytes.Clone(result), nil
}

// EncodeToBuffer encodes directly to a buffer (caller manages buffer lifecycle)
func (fje *FastJSONEncoder) EncodeToBuffer(buf *bytes.Buffer, v interface{}) error {
	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(v)
}

// GetBuffer gets a buffer from the pool
func (fje *FastJSONEncoder) GetBuffer() *bytes.Buffer {
	return fje.bufferPool.Get().(*bytes.Buffer)
}

// PutBuffer returns a buffer to the pool
func (fje *FastJSONEncoder) PutBuffer(buf *bytes.Buffer) {
	buf.Reset()
	fje.bufferPool.Put(buf)
}
