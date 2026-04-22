//go:build darwin

package voicewake

import (
	"errors"
	"testing"

	"github.com/ebitengine/purego/objc"
)

func TestWithVoiceWakeStartPipelineCleanup_ReleasesPartialResourcesOnError(t *testing.T) {
	var released []startPipelineResult
	restore := setVoiceWakeReleasePipelineForTest(func(result startPipelineResult) {
		released = append(released, result)
	})
	defer restore()

	wantErr := errors.New("boom")
	_, err := withVoiceWakeStartPipelineCleanup(func(result *startPipelineResult) error {
		result.recognizer = objc.ID(1)
		result.request = objc.ID(2)
		result.engine = objc.ID(3)
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if len(released) != 1 {
		t.Fatalf("released calls = %d, want 1", len(released))
	}
	if released[0].recognizer != objc.ID(1) || released[0].request != objc.ID(2) || released[0].engine != objc.ID(3) {
		t.Fatalf("released result = %#v, want recognizer=1 request=2 engine=3", released[0])
	}
}

func TestWithVoiceWakeStartPipelineCleanup_DoesNotReleaseOnSuccess(t *testing.T) {
	released := false
	restore := setVoiceWakeReleasePipelineForTest(func(result startPipelineResult) {
		released = true
	})
	defer restore()

	result, err := withVoiceWakeStartPipelineCleanup(func(result *startPipelineResult) error {
		result.recognizer = objc.ID(7)
		result.task = objc.ID(8)
		return nil
	})
	if err != nil {
		t.Fatalf("withVoiceWakeStartPipelineCleanup returned error: %v", err)
	}
	if released {
		t.Fatal("expected no cleanup on success")
	}
	if result.recognizer != objc.ID(7) || result.task != objc.ID(8) {
		t.Fatalf("result = %#v, want recognizer=7 task=8", result)
	}
}
