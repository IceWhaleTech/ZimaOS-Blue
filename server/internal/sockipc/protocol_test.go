package sockipc

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"
)

func TestRequestRoundTrip(t *testing.T) {
	req := &Request{
		Cmd:    "media.generate",
		Params: map[string]string{"prompt": "a cat on a rainbow", "model": "nano-banana-pro"},
	}

	var buf bytes.Buffer
	if err := WriteJSON(&buf, req); err != nil {
		t.Fatal(err)
	}

	got, err := ReadJSON[Request](&buf)
	if err != nil {
		t.Fatal(err)
	}
	if got.Cmd != req.Cmd {
		t.Errorf("Cmd = %q, want %q", got.Cmd, req.Cmd)
	}
	if got.Params["prompt"] != "a cat on a rainbow" {
		t.Errorf("prompt = %q", got.Params["prompt"])
	}
	if got.Params["model"] != "nano-banana-pro" {
		t.Errorf("model = %q", got.Params["model"])
	}
}

func TestResponseRoundTrip(t *testing.T) {
	resp := OkResponse(map[string]string{"task_id": "abc-123"})

	var buf bytes.Buffer
	if err := WriteJSON(&buf, resp); err != nil {
		t.Fatal(err)
	}

	got, err := ReadJSON[Response](&buf)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "ok" {
		t.Errorf("Status = %q", got.Status)
	}
	if got.Data["task_id"] != "abc-123" {
		t.Errorf("task_id = %q", got.Data["task_id"])
	}
}

func TestErrResponseRoundTrip(t *testing.T) {
	resp := ErrResponse("something went wrong")

	var buf bytes.Buffer
	WriteJSON(&buf, resp)

	got, err := ReadJSON[Response](&buf)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "error" {
		t.Errorf("Status = %q", got.Status)
	}
	if got.Error != "something went wrong" {
		t.Errorf("Error = %q", got.Error)
	}
}

func TestReadJSONEOF(t *testing.T) {
	_, err := ReadJSON[Request](bytes.NewReader(nil))
	if err != io.EOF {
		t.Errorf("err = %v, want io.EOF", err)
	}
}

func TestReadJSONTooLarge(t *testing.T) {
	var buf bytes.Buffer
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], maxMessageSize+1)
	buf.Write(hdr[:])

	_, err := ReadJSON[Request](&buf)
	if err == nil {
		t.Error("expected error for oversized message")
	}
}

func TestMultipleMessagesOnSameBuffer(t *testing.T) {
	var buf bytes.Buffer

	req1 := &Request{Cmd: "ping"}
	req2 := &Request{Cmd: "media.generate", Params: map[string]string{"prompt": "test"}}
	WriteJSON(&buf, req1)
	WriteJSON(&buf, req2)

	got1, err := ReadJSON[Request](&buf)
	if err != nil {
		t.Fatal(err)
	}
	if got1.Cmd != "ping" {
		t.Errorf("msg1 Cmd = %q", got1.Cmd)
	}

	got2, err := ReadJSON[Request](&buf)
	if err != nil {
		t.Fatal(err)
	}
	if got2.Cmd != "media.generate" {
		t.Errorf("msg2 Cmd = %q", got2.Cmd)
	}
	if got2.Params["prompt"] != "test" {
		t.Errorf("msg2 prompt = %q", got2.Params["prompt"])
	}
}

func TestOkResponseNilData(t *testing.T) {
	resp := OkResponse(nil)
	if resp.Status != "ok" {
		t.Errorf("Status = %q", resp.Status)
	}
	if resp.Data != nil {
		t.Errorf("Data = %v, want nil", resp.Data)
	}
}
