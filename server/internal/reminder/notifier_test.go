package reminder

import (
	"context"
	"testing"

	"go.uber.org/zap"
)

type mockNotifier struct {
	calls []struct{ title, body string }
	err   error
}

func (m *mockNotifier) Notify(_ context.Context, title, body string) error {
	m.calls = append(m.calls, struct{ title, body string }{title, body})
	return m.err
}

func TestNewNotifierReturnsNonNil(t *testing.T) {
	n := NewNotifier(zap.NewNop())
	if n == nil {
		t.Fatal("NewNotifier returned nil")
	}
}

func TestMockNotifierRecordsCalls(t *testing.T) {
	m := &mockNotifier{}
	if err := m.Notify(context.Background(), "Title", "Body"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(m.calls))
	}
	if m.calls[0].title != "Title" || m.calls[0].body != "Body" {
		t.Fatalf("unexpected call: %+v", m.calls[0])
	}
}
