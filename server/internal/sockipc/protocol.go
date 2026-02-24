// Package sockipc provides a Unix domain socket IPC layer using
// length-prefixed JSON messages. Designed for LLM skill calls and
// CLI commands that need a stable interface to the main process.
package sockipc

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

// Request is a JSON IPC request sent by the client.
type Request struct {
	Cmd    string            `json:"cmd"`
	Params map[string]string `json:"params,omitempty"`
}

// Response is a JSON IPC response returned by the server.
type Response struct {
	Status string            `json:"status"`
	Error  string            `json:"error,omitempty"`
	Data   map[string]string `json:"data,omitempty"`
}

// OkResponse creates a success response with optional data.
func OkResponse(data map[string]string) *Response {
	return &Response{Status: "ok", Data: data}
}

// ErrResponse creates an error response.
func ErrResponse(msg string) *Response {
	return &Response{Status: "error", Error: msg}
}

// maxMessageSize is the maximum allowed message size (16 MB).
const maxMessageSize = 16 << 20

// WriteJSON marshals v to JSON and writes it with a 4-byte big-endian length prefix.
func WriteJSON(w io.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("sockipc: marshal: %w", err)
	}
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(data)))
	if _, err := w.Write(hdr[:]); err != nil {
		return fmt.Errorf("sockipc: write header: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("sockipc: write body: %w", err)
	}
	return nil
}

// ReadJSON reads a length-prefixed JSON message and unmarshals it into T.
func ReadJSON[T any](r io.Reader) (*T, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err // io.EOF or io.ErrUnexpectedEOF
	}
	size := binary.BigEndian.Uint32(hdr[:])
	if size > maxMessageSize {
		return nil, fmt.Errorf("sockipc: message too large: %d bytes", size)
	}
	buf := make([]byte, size)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, fmt.Errorf("sockipc: read body: %w", err)
	}
	var v T
	if err := json.Unmarshal(buf, &v); err != nil {
		return nil, fmt.Errorf("sockipc: unmarshal: %w", err)
	}
	return &v, nil
}
