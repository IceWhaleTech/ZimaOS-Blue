package pruner

import (
	"testing"
)

func TestByteToUnicode(t *testing.T) {
	// Verify all 256 bytes are mapped
	seen := make(map[rune]bool)
	for i := 0; i < 256; i++ {
		r := byteToUnicode[i]
		if r == 0 {
			t.Errorf("byte %d not mapped", i)
		}
		if seen[r] {
			t.Errorf("byte %d maps to duplicate rune %d", i, r)
		}
		seen[r] = true
	}
	// Printable ASCII should map to themselves
	if byteToUnicode['A'] != 'A' {
		t.Errorf("expected 'A' to map to itself, got %c", byteToUnicode['A'])
	}
}

func TestSplitOnWhitespace(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"hello world", []string{"hello", " world"}},
		{"a\nb", []string{"a", "\nb"}},
		{"  x", []string{" ", " x"}},
		{"", nil},
	}
	for _, tt := range tests {
		got := splitOnWhitespace(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitOnWhitespace(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitOnWhitespace(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestBuildPrunerInput(t *testing.T) {
	// This test verifies the structure without real vocab/merges files.
	// With a real tokenizer, we'd check exact token IDs.
	// For now, just verify the function doesn't panic and produces correct shapes.
	tok := &BPETokenizer{
		vocab:    map[string]int{"a": 1, "b": 2, " ": 3},
		ivocab:   map[int]string{1: "a", 2: "b", 3: " "},
		mergeMap: map[mergePair]int{},
		padID:    0,
	}

	ids, mask, codeStart, codeEnd := tok.BuildPrunerInput("test", "ab", 128)
	if len(ids) != 128 {
		t.Errorf("expected 128 input_ids, got %d", len(ids))
	}
	if len(mask) != 128 {
		t.Errorf("expected 128 attention_mask, got %d", len(mask))
	}
	if codeStart >= codeEnd {
		t.Logf("codeStart=%d codeEnd=%d (may be 0 if tokens not in vocab)", codeStart, codeEnd)
	}
}
