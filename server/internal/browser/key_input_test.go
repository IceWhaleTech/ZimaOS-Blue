package browser

import (
	"testing"
	"time"

	"github.com/go-rod/rod/lib/input"
)

type fakeBrowserKeyTarget struct {
	calls []string
}

func (f *fakeBrowserKeyTarget) Press(key input.Key) error {
	f.calls = append(f.calls, "press:"+key.Info().Code)
	return nil
}

func (f *fakeBrowserKeyTarget) Release(key input.Key) error {
	f.calls = append(f.calls, "release:"+key.Info().Code)
	return nil
}

func (f *fakeBrowserKeyTarget) Type(keys ...input.Key) error {
	for _, key := range keys {
		f.calls = append(f.calls, "type:"+key.Info().Code)
	}
	return nil
}

func (f *fakeBrowserKeyTarget) InsertText(text string) error {
	f.calls = append(f.calls, "text:"+text)
	return nil
}

func TestExecuteBrowserKeyInputTreatsModifierListAsChord(t *testing.T) {
	target := &fakeBrowserKeyTarget{}
	var sleeps []time.Duration
	prevSleep := browserKeySleep
	browserKeySleep = func(duration time.Duration) {
		sleeps = append(sleeps, duration)
	}
	defer func() {
		browserKeySleep = prevSleep
	}()

	if err := executeBrowserKeyInput(target, []string{"ctrl", "c"}, 25); err != nil {
		t.Fatalf("executeBrowserKeyInput() error = %v", err)
	}

	wantCalls := []string{
		"press:ControlLeft",
		"press:KeyC",
		"release:KeyC",
		"release:ControlLeft",
	}
	if len(target.calls) != len(wantCalls) {
		t.Fatalf("calls = %#v, want %#v", target.calls, wantCalls)
	}
	for idx, want := range wantCalls {
		if target.calls[idx] != want {
			t.Fatalf("calls[%d] = %q, want %q", idx, target.calls[idx], want)
		}
	}
	if len(sleeps) != 1 || sleeps[0] != 25*time.Millisecond {
		t.Fatalf("sleeps = %#v, want [25ms]", sleeps)
	}
}

func TestExecuteBrowserKeyInputSupportsChordStringsAsSequentialSteps(t *testing.T) {
	target := &fakeBrowserKeyTarget{}
	var sleeps []time.Duration
	prevSleep := browserKeySleep
	browserKeySleep = func(duration time.Duration) {
		sleeps = append(sleeps, duration)
	}
	defer func() {
		browserKeySleep = prevSleep
	}()

	if err := executeBrowserKeyInput(target, []string{"cmd+l", "shift+tab"}, 40); err != nil {
		t.Fatalf("executeBrowserKeyInput() error = %v", err)
	}

	wantCalls := []string{
		"press:MetaLeft",
		"press:KeyL",
		"release:KeyL",
		"release:MetaLeft",
		"press:ShiftLeft",
		"press:Tab",
		"release:Tab",
		"release:ShiftLeft",
	}
	if len(target.calls) != len(wantCalls) {
		t.Fatalf("calls = %#v, want %#v", target.calls, wantCalls)
	}
	for idx, want := range wantCalls {
		if target.calls[idx] != want {
			t.Fatalf("calls[%d] = %q, want %q", idx, target.calls[idx], want)
		}
	}
	if len(sleeps) != 2 || sleeps[0] != 40*time.Millisecond || sleeps[1] != 40*time.Millisecond {
		t.Fatalf("sleeps = %#v, want [40ms 40ms]", sleeps)
	}
}

func TestExecuteBrowserKeyInputInsertsPlainTextWhenNoNamedKeyExists(t *testing.T) {
	target := &fakeBrowserKeyTarget{}

	if err := executeBrowserKeyInput(target, []string{"hello world"}, 0); err != nil {
		t.Fatalf("executeBrowserKeyInput() error = %v", err)
	}

	if len(target.calls) != 1 || target.calls[0] != "text:hello world" {
		t.Fatalf("calls = %#v, want [text:hello world]", target.calls)
	}
}
