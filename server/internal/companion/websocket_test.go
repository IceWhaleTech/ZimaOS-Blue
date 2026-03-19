package companion

import (
	"context"
	"testing"
)

type stubStreamer struct{}

func (s *stubStreamer) Emit(event *SessionEvent) {}

func (s *stubStreamer) Subscribe(sessionID string) (<-chan *SessionEvent, func()) {
	ch := make(chan *SessionEvent)
	return ch, func() { close(ch) }
}

func (s *stubStreamer) Start(ctx context.Context) error { return nil }

func (s *stubStreamer) Stop() error { return nil }

func TestLazyWebSocketHandlerDefersStreamerResolutionUntilNeeded(t *testing.T) {
	cfg := DefaultConfig()
	streamer := &stubStreamer{}
	resolveCalls := 0

	h := NewLazyWebSocketHandler(func() Streamer {
		resolveCalls++
		return streamer
	}, cfg)

	if resolveCalls != 0 {
		t.Fatalf("resolveCalls after construction = %d, want 0", resolveCalls)
	}

	if got := h.ensureStreamer(); got != streamer {
		t.Fatalf("ensureStreamer() = %T, want %T", got, streamer)
	}
	if resolveCalls != 1 {
		t.Fatalf("resolveCalls after first ensure = %d, want 1", resolveCalls)
	}

	if got := h.ensureStreamer(); got != streamer {
		t.Fatalf("ensureStreamer() second call = %T, want %T", got, streamer)
	}
	if resolveCalls != 1 {
		t.Fatalf("resolveCalls after second ensure = %d, want 1", resolveCalls)
	}
}

func TestLazyWebSocketHandlerRetriesWhenInitReturnsNil(t *testing.T) {
	cfg := DefaultConfig()
	streamer := &stubStreamer{}
	resolveCalls := 0

	h := NewLazyWebSocketHandler(func() Streamer {
		resolveCalls++
		if resolveCalls == 1 {
			return nil
		}
		return streamer
	}, cfg)

	if got := h.ensureStreamer(); got != nil {
		t.Fatalf("ensureStreamer() first call = %T, want nil", got)
	}
	if resolveCalls != 1 {
		t.Fatalf("resolveCalls after first ensure = %d, want 1", resolveCalls)
	}

	if got := h.ensureStreamer(); got != streamer {
		t.Fatalf("ensureStreamer() second call = %T, want %T", got, streamer)
	}
	if resolveCalls != 2 {
		t.Fatalf("resolveCalls after second ensure = %d, want 2", resolveCalls)
	}
}
