package logger

import "testing"

func TestNewRingBuffer_AllocatesEntriesLazilyAndPreservesOrder(t *testing.T) {
	rb := NewRingBuffer(8)
	if len(rb.entries) != 0 {
		t.Fatalf("expected ring buffer entries to start unallocated, got len=%d", len(rb.entries))
	}

	for i := 0; i < 8; i++ {
		rb.Add(LogEntry{Message: string(rune('a' + i))})
	}
	if rb.Count() != 8 {
		t.Fatalf("Count() = %d, want 8", rb.Count())
	}
	if len(rb.entries) != 8 {
		t.Fatalf("len(entries) = %d, want grown-to-max 8", len(rb.entries))
	}

	all := rb.GetAll()
	if len(all) != 8 {
		t.Fatalf("len(GetAll()) = %d, want 8", len(all))
	}
	for i, entry := range all {
		want := string(rune('a' + i))
		if entry.Message != want {
			t.Fatalf("GetAll()[%d].Message = %q, want %q", i, entry.Message, want)
		}
	}

	rb.Add(LogEntry{Message: "i"})
	all = rb.GetAll()
	want := []string{"b", "c", "d", "e", "f", "g", "h", "i"}
	for i, entry := range all {
		if entry.Message != want[i] {
			t.Fatalf("after wrap GetAll()[%d].Message = %q, want %q", i, entry.Message, want[i])
		}
	}
}
