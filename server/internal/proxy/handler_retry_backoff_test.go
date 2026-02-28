package proxy

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTransientUpstreamRetryDelay(t *testing.T) {
	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{attempt: 0, want: 500 * time.Millisecond},
		{attempt: 1, want: 1500 * time.Millisecond},
		{attempt: 2, want: 3 * time.Second},
	}

	for _, tc := range tests {
		if got := transientUpstreamRetryDelay(tc.attempt); got != tc.want {
			t.Fatalf("transientUpstreamRetryDelay(%d) = %s, want %s", tc.attempt, got, tc.want)
		}
	}
}

func TestIsContextCanceledError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "context canceled", err: context.Canceled, want: true},
		{name: "deadline exceeded", err: context.DeadlineExceeded, want: true},
		{name: "wrapped canceled", err: errors.New("Post \"https://example.com\": context canceled"), want: true},
		{name: "wrapped deadline", err: errors.New("upstream timeout: deadline exceeded"), want: true},
		{name: "other error", err: errors.New("connection refused"), want: false},
	}

	for _, tc := range tests {
		if got := isContextCanceledError(tc.err); got != tc.want {
			t.Fatalf("%s: isContextCanceledError(%v) = %v, want %v", tc.name, tc.err, got, tc.want)
		}
	}
}

func TestIsResponsesContinuationRejectedError(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   bool
	}{
		{name: "no body", status: 500, body: "", want: false},
		{name: "non continuation", status: 500, body: `{"error":"backend overloaded"}`, want: false},
		{name: "continuation 400", status: 400, body: `{"error":"previous_response_id not found"}`, want: true},
		{name: "continuation 404", status: 404, body: `{"error":"response id does not exist"}`, want: true},
		{name: "continuation wrapped 5xx", status: 502, body: `{"error":"upstream error for previous response"}`, want: true},
	}

	for _, tc := range tests {
		if got := isResponsesContinuationRejectedError(tc.status, []byte(tc.body)); got != tc.want {
			t.Fatalf("%s: isResponsesContinuationRejectedError(%d, %q) = %v, want %v", tc.name, tc.status, tc.body, got, tc.want)
		}
	}
}

func TestShouldRetryWithoutTools(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "no response", err: errors.New("provider prov_x returned no response"), want: false},
		{name: "upstream 502", err: errors.New("upstream 502: bad gateway"), want: false},
		{name: "provider 502", err: errors.New("provider returned 502: bad gateway"), want: false},
		{name: "explicit no tool support", err: errors.New("provider prov_x does not support tool calls"), want: true},
		{name: "tool field rejected 400", err: errors.New("provider returned 400: unknown field tool_choice"), want: true},
		{name: "function calling rejected 422", err: errors.New("provider returned 422: function calling not supported"), want: true},
		{name: "generic 400 without tool signal", err: errors.New("upstream 400: prompt too long"), want: false},
		{name: "generic 422 without tool signal", err: errors.New("provider returned 422: invalid parameter"), want: false},
	}

	for _, tc := range tests {
		if got := shouldRetryWithoutTools(tc.err); got != tc.want {
			t.Fatalf("%s: shouldRetryWithoutTools(%v) = %v, want %v", tc.name, tc.err, got, tc.want)
		}
	}
}
