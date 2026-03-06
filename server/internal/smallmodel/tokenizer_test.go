package smallmodel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadTokenizer_ParsesArrayMerges(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tokenizer.json")
	payload := `{
  "model": {
    "type": "BPE",
    "vocab": {
      "a": 0,
      "b": 1,
      "ab": 2
    },
    "merges": [
      ["a", "b"]
    ]
  },
  "added_tokens": [
    {"id": 3, "content": "<|endoftext|>", "special": true}
  ],
  "pre_tokenizer": {
    "type": "Sequence",
    "pretokenizers": [
      {"type": "Split", "pattern": {"Regex": "(?i:'s|'t|'re|'ve|'m|'ll|'d)|[^\\r\\n\\p{L}\\p{N}]?[\\p{L}\\p{M}]+|\\p{N}| ?[^\\s\\p{L}\\p{M}\\p{N}]+[\\r\\n]*|\\s*\\r?\\n+|\\s+(?!\\S)|\\s+"}}
    ]
  }
}`
	if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
		t.Fatalf("write tokenizer.json: %v", err)
	}

	tok, err := loadTokenizer(path)
	if err != nil {
		t.Fatalf("loadTokenizer failed: %v", err)
	}

	ids := tok.encode("ab")
	if len(ids) != 1 || ids[0] != 2 {
		t.Fatalf("encode(ab) = %v, want [2]", ids)
	}
	if got := tok.decode(ids); got != "ab" {
		t.Fatalf("decode([2]) = %q, want ab", got)
	}
	if id, ok := tok.specialID("<|endoftext|>"); !ok || id != 3 {
		t.Fatalf("specialID(<|endoftext|>) = (%d,%v), want (3,true)", id, ok)
	}
}

func TestLoadTokenizer_RejectsNonBPE(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tokenizer.json")
	payload := `{
  "model": {
    "type": "WordPiece",
    "vocab": {},
    "merges": []
  }
}`
	if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
		t.Fatalf("write tokenizer.json: %v", err)
	}

	_, err := loadTokenizer(path)
	if err == nil || !strings.Contains(err.Error(), "unsupported tokenizer model type") {
		t.Fatalf("loadTokenizer() error = %v, want unsupported tokenizer model type", err)
	}
}
