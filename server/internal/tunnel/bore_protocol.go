// Package tunnel: bore protocol implementation (compatible with https://github.com/ekzhang/bore).
// Control port 7835, JSON messages delimited by null byte, max frame 256 bytes.

package tunnel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	boreControlPort = 7835
	boreMaxFrameLen = 256
	boreDialTimeout = 10 * time.Second
)

// clientMessage is sent from client to server (JSON with tag).
type clientMessage struct {
	Authenticate *string    `json:"Authenticate,omitempty"`
	Hello        *uint16    `json:"Hello,omitempty"`
	Accept       *uuid.UUID `json:"Accept,omitempty"`
}

// serverMessage is sent from server to client.
type serverMessage struct {
	Challenge  *uuid.UUID `json:"Challenge,omitempty"`
	Hello      *uint16    `json:"Hello,omitempty"`
	Heartbeat  *struct{}  `json:"Heartbeat,omitempty"`
	Connection *uuid.UUID `json:"Connection,omitempty"`
	Error      *string    `json:"Error,omitempty"`
}

// boreConn wraps a TCP connection with null-delimited JSON framing.
type boreConn struct {
	conn net.Conn
	dec  *json.Decoder
	enc  *json.Encoder
	mu   sync.Mutex
}

func newBoreConn(conn net.Conn) *boreConn {
	// We read until 0, then decode JSON from that slice. Writer: encode JSON then write 0.
	return &boreConn{conn: conn}
}

// readMessage reads one null-delimited JSON frame and parses ServerMessage.
func (c *boreConn) readMessage() (*serverMessage, error) {
	buf := make([]byte, 0, boreMaxFrameLen)
	for len(buf) < boreMaxFrameLen {
		b := make([]byte, 1)
		n, err := c.conn.Read(b)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			continue
		}
		if b[0] == 0 {
			break
		}
		buf = append(buf, b[0])
	}
	if len(buf) == 0 {
		return nil, io.EOF
	}
	var msg serverMessage
	if err := json.Unmarshal(buf, &msg); err != nil {
		return nil, fmt.Errorf("bore protocol: %w", err)
	}
	return &msg, nil
}

// writeMessage encodes msg as JSON and writes with trailing null.
func (c *boreConn) writeMessage(msg interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	if len(data) > boreMaxFrameLen-1 {
		return fmt.Errorf("bore frame too long")
	}
	_, err = c.conn.Write(append(data, 0))
	return err
}

func (c *boreConn) close() error {
	return c.conn.Close()
}

// boreClient implements the bore client protocol (connect to bore.pub, Hello, then handle Connection/Accept).
func boreClient(ctx context.Context, server string, localPort int, onURL func(string)) error {
	addr := fmt.Sprintf("%s:%d", server, boreControlPort)
	conn, err := net.DialTimeout("tcp", addr, boreDialTimeout)
	if err != nil {
		return fmt.Errorf("bore dial: %w", err)
	}
	defer conn.Close()

	// When ctx is cancelled, close the connection so blocking read returns.
	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	bc := newBoreConn(conn)

	// Send Hello(0) for random port
	portZero := uint16(0)
	if err := bc.writeMessage(clientMessage{Hello: &portZero}); err != nil {
		return err
	}

	// Read Hello(remotePort) or Error
	msg, err := bc.readMessage()
	if err != nil {
		return err
	}
	if msg.Error != nil {
		return fmt.Errorf("bore server: %s", *msg.Error)
	}
	if msg.Hello == nil {
		return fmt.Errorf("bore: expected Hello, got %+v", msg)
	}
	remotePort := *msg.Hello
	urlStr := fmt.Sprintf("http://%s:%d", server, remotePort)
	onURL(urlStr)

	// Loop: read ServerMessage; on Connection(uuid), accept and proxy
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		msg, err := bc.readMessage()
		if err != nil {
			return err
		}
		if msg.Connection != nil {
			go handleBoreConnection(server, *msg.Connection, localPort)
			continue
		}
		if msg.Error != nil {
			return fmt.Errorf("bore server: %s", *msg.Error)
		}
		// Heartbeat / Hello ignore
	}
}

func handleBoreConnection(server string, id uuid.UUID, localPort int) {
	addr := fmt.Sprintf("%s:%d", server, boreControlPort)
	conn, err := net.DialTimeout("tcp", addr, boreDialTimeout)
	if err != nil {
		return
	}
	defer conn.Close()

	bc := newBoreConn(conn)
	if err := bc.writeMessage(clientMessage{Accept: &id}); err != nil {
		return
	}
	// After Accept, the same TCP connection is used for raw tunnel data (no more JSON frames)
	localAddr := fmt.Sprintf("127.0.0.1:%d", localPort)
	local, err := net.DialTimeout("tcp", localAddr, 5*time.Second)
	if err != nil {
		return
	}
	defer local.Close()
	bidirectionalCopy(bc.conn, local)
}

func bidirectionalCopy(a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		io.Copy(a, b)
		a.Close()
	}()
	go func() {
		defer wg.Done()
		io.Copy(b, a)
		b.Close()
	}()
	wg.Wait()
}
