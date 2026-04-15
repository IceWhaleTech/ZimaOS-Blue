//go:build darwin

package a11y

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestLocateTextInImage_ParsesVisionJSON(t *testing.T) {
	prev := darwinCLIFallback
	var name string
	var args []string
	darwinCLIFallback = darwinSystemCLI{
		run: func(_ context.Context, command string, commandArgs ...string) (string, error) {
			name = command
			args = append([]string(nil), commandArgs...)
			return `[{"text":"Orca","bounds":{"x":0.1,"y":0.2,"width":0.3,"height":0.4},"confidence":0.91}]`, nil
		},
	}
	defer func() { darwinCLIFallback = prev }()

	lines, err := LocateTextInImage(context.Background(), "/tmp/orca-search.png")
	if err != nil {
		t.Fatalf("LocateTextInImage() error = %v", err)
	}
	if name != "osascript" {
		t.Fatalf("command = %q, want osascript", name)
	}
	joined := strings.Join(args, " ")
	for _, needle := range []string{
		`-l JavaScript`,
		`ObjC.import('Vision')`,
		`VNRecognizeTextRequest`,
		`-- /tmp/orca-search.png`,
	} {
		if !strings.Contains(joined, needle) {
			t.Fatalf("args missing %q in %q", needle, joined)
		}
	}
	if len(lines) != 1 {
		t.Fatalf("lines len = %d, want 1", len(lines))
	}
	if lines[0].Text != "Orca" {
		t.Fatalf("lines[0].Text = %q, want Orca", lines[0].Text)
	}
	if lines[0].Bounds.X != 0.1 || lines[0].Bounds.Height != 0.4 {
		t.Fatalf("lines[0].Bounds = %#v, want parsed bounds", lines[0].Bounds)
	}
	if lines[0].Confidence != 0.91 {
		t.Fatalf("lines[0].Confidence = %v, want 0.91", lines[0].Confidence)
	}
}

func TestLocateTextInImage_ReturnsCLIContextOnFailure(t *testing.T) {
	prev := darwinCLIFallback
	darwinCLIFallback = darwinSystemCLI{
		run: func(_ context.Context, command string, commandArgs ...string) (string, error) {
			return "vision failed", errors.New("exit status 1")
		},
	}
	defer func() { darwinCLIFallback = prev }()

	_, err := LocateTextInImage(context.Background(), "/tmp/orca-search.png")
	if err == nil {
		t.Fatal("LocateTextInImage() error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "vision failed") {
		t.Fatalf("error = %v, want osascript output", err)
	}
}

func TestLocateTextInPNG_ParsesVisionJSON(t *testing.T) {
	prev := darwinCLIFallback
	var name string
	var args []string
	var stdin []byte
	darwinCLIFallback = darwinSystemCLI{
		runWithInput: func(_ context.Context, input []byte, command string, commandArgs ...string) (string, error) {
			name = command
			stdin = append([]byte(nil), input...)
			args = append([]string(nil), commandArgs...)
			return `[{"text":"Orca","bounds":{"x":0.2,"y":0.3,"width":0.25,"height":0.1},"confidence":0.87}]`, nil
		},
	}
	defer func() { darwinCLIFallback = prev }()

	imagePNG := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x01, 0x02, 0x03}
	lines, err := LocateTextInPNG(context.Background(), imagePNG)
	if err != nil {
		t.Fatalf("LocateTextInPNG() error = %v", err)
	}
	if name != "osascript" {
		t.Fatalf("command = %q, want osascript", name)
	}
	joined := strings.Join(args, " ")
	for _, needle := range []string{
		`-l JavaScript`,
		`ObjC.import('Vision')`,
		`initWithDataOptions`,
		`fileHandleWithStandardInput`,
	} {
		if !strings.Contains(joined, needle) {
			t.Fatalf("args missing %q in %q", needle, joined)
		}
	}
	if !bytes.Equal(stdin, imagePNG) {
		t.Fatalf("stdin = %q, want forwarded PNG bytes", string(stdin))
	}
	if len(lines) != 1 {
		t.Fatalf("lines len = %d, want 1", len(lines))
	}
	if lines[0].Text != "Orca" {
		t.Fatalf("lines[0].Text = %q, want Orca", lines[0].Text)
	}
	if lines[0].Bounds.X != 0.2 || lines[0].Bounds.Height != 0.1 {
		t.Fatalf("lines[0].Bounds = %#v, want parsed bounds", lines[0].Bounds)
	}
	if lines[0].Confidence != 0.87 {
		t.Fatalf("lines[0].Confidence = %v, want 0.87", lines[0].Confidence)
	}
}
