//go:build !windows

package service

import (
	"os"
	"testing"
)

func TestIsTerminalFile(t *testing.T) {
	if isTerminalFile(nil) {
		t.Fatalf("nil file should not be treated as a terminal")
	}

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	defer devNull.Close()

	if isTerminalFile(devNull) {
		t.Fatalf("%s should not be treated as an interactive terminal", os.DevNull)
	}
}
