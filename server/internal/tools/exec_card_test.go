package tools

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestExtractCardPayload(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantOK  bool
		wantTyp string
	}{
		{
			name:    "valid card",
			line:    `__CARD__{"type":"analyze-progress","step":"data","status":"running"}__END__`,
			wantOK:  true,
			wantTyp: "analyze-progress",
		},
		{
			name:   "no prefix",
			line:   `{"type":"analyze-progress"}__END__`,
			wantOK: false,
		},
		{
			name:   "no suffix",
			line:   `__CARD__{"type":"analyze-progress"}`,
			wantOK: false,
		},
		{
			name:   "invalid json",
			line:   `__CARD__not-json__END__`,
			wantOK: false,
		},
		{
			name:   "normal output line",
			line:   `hello world`,
			wantOK: false,
		},
		{
			name:    "with whitespace",
			line:    `  __CARD__{"type":"alert","variant":"success"}__END__  `,
			wantOK:  true,
			wantTyp: "alert",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			card, ok := extractCardPayload(tt.line)
			if ok != tt.wantOK {
				t.Fatalf("extractCardPayload(%q) ok = %v, want %v", tt.line, ok, tt.wantOK)
			}
			if ok && card["type"] != tt.wantTyp {
				t.Fatalf("card type = %v, want %v", card["type"], tt.wantTyp)
			}
		})
	}
}

func TestReadIntoBufferWithCards(t *testing.T) {
	input := strings.Join([]string{
		"line 1",
		`__CARD__{"type":"progress","step":"a","status":"running"}__END__`,
		"line 2",
		`__CARD__{"type":"progress","step":"a","status":"done"}__END__`,
		"line 3",
	}, "\n")

	var emitted []map[string]interface{}
	ctx := WithCardEmitter(context.Background(), func(card map[string]interface{}) {
		emitted = append(emitted, card)
	})

	buf := NewOutputBuffer(0)
	readIntoBufferWithCards(ctx, bytes.NewReader([]byte(input)), buf)

	// Should have emitted 2 cards.
	if len(emitted) != 2 {
		t.Fatalf("emitted %d cards, want 2", len(emitted))
	}
	if emitted[0]["status"] != "running" {
		t.Fatalf("card 0 status = %v, want running", emitted[0]["status"])
	}
	if emitted[1]["status"] != "done" {
		t.Fatalf("card 1 status = %v, want done", emitted[1]["status"])
	}

	// Buffer should contain only non-card lines.
	out := buf.String()
	if strings.Contains(out, "__CARD__") {
		t.Fatalf("buffer contains card line: %s", out)
	}
	if !strings.Contains(out, "line 1") || !strings.Contains(out, "line 2") || !strings.Contains(out, "line 3") {
		t.Fatalf("buffer missing normal lines: %s", out)
	}
}
