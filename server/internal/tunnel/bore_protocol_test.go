package tunnel

import (
	"encoding/json"
	"fmt"
	"net"
	"testing"

	"github.com/google/uuid"
)

func TestClientMessageJSONRoundtrip(t *testing.T) {
	port := uint16(0)
	msg := clientMessage{Hello: &port}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out clientMessage
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Hello == nil || *out.Hello != 0 {
		t.Errorf("expected Hello(0), got %+v", out)
	}

	id := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	msg2 := clientMessage{Accept: &id}
	data2, err := json.Marshal(msg2)
	if err != nil {
		t.Fatalf("marshal Accept: %v", err)
	}
	var out2 clientMessage
	if err := json.Unmarshal(data2, &out2); err != nil {
		t.Fatalf("unmarshal Accept: %v", err)
	}
	if out2.Accept == nil || *out2.Accept != id {
		t.Errorf("expected Accept(%v), got %+v", id, out2)
	}
}

func TestServerMessageJSONRoundtrip(t *testing.T) {
	port := uint16(35429)
	msg := serverMessage{Hello: &port}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out serverMessage
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Hello == nil || *out.Hello != 35429 {
		t.Errorf("expected Hello(35429), got %+v", out)
	}

	id := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	msg2 := serverMessage{Connection: &id}
	data2, err := json.Marshal(msg2)
	if err != nil {
		t.Fatalf("marshal Connection: %v", err)
	}
	var out2 serverMessage
	if err := json.Unmarshal(data2, &out2); err != nil {
		t.Fatalf("unmarshal Connection: %v", err)
	}
	if out2.Connection == nil || *out2.Connection != id {
		t.Errorf("expected Connection(%v), got %+v", id, out2)
	}

	errMsg := "something went wrong"
	msg3 := serverMessage{Error: &errMsg}
	data3, err := json.Marshal(msg3)
	if err != nil {
		t.Fatalf("marshal Error: %v", err)
	}
	var out3 serverMessage
	if err := json.Unmarshal(data3, &out3); err != nil {
		t.Fatalf("unmarshal Error: %v", err)
	}
	if out3.Error == nil || *out3.Error != errMsg {
		t.Errorf("expected Error(%q), got %+v", errMsg, out3)
	}
}

func TestBoreConnReadWriteMessage(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	bc := newBoreConn(client)

	// Server side: write Hello(12345) with null delimiter
	go func() {
		port := uint16(12345)
		data, _ := json.Marshal(serverMessage{Hello: &port})
		_, _ = server.Write(append(data, 0))
	}()

	msg, err := bc.readMessage()
	if err != nil {
		t.Fatalf("readMessage: %v", err)
	}
	if msg.Hello == nil || *msg.Hello != 12345 {
		t.Errorf("expected Hello(12345), got %+v", msg)
	}
}

func TestBoreConnWriteMessage(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	bc := newBoreConn(client)

	// Read on server side in goroutine so client Write can complete (pipe is synchronous).
	done := make(chan struct{})
	var out clientMessage
	var readErr error
	go func() {
		buf := make([]byte, 256)
		n, err := server.Read(buf)
		if err != nil {
			readErr = err
			close(done)
			return
		}
		if n < 2 || buf[n-1] != 0 {
			readErr = fmt.Errorf("expected null-terminated frame, got %d bytes", n)
			close(done)
			return
		}
		readErr = json.Unmarshal(buf[:n-1], &out)
		close(done)
	}()

	portZero := uint16(0)
	err := bc.writeMessage(clientMessage{Hello: &portZero})
	if err != nil {
		t.Fatalf("writeMessage: %v", err)
	}
	<-done
	if readErr != nil {
		t.Fatalf("server read: %v", readErr)
	}
	if out.Hello == nil || *out.Hello != 0 {
		t.Errorf("expected Hello(0), got %+v", out)
	}
}
