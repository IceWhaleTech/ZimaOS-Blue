//go:build darwin

package main

import (
	"errors"
	"testing"

	"github.com/ebitengine/purego/objc"
)

func TestWithDarwinOwnedObjectRelease_ReleasesOnSuccess(t *testing.T) {
	var released []objc.ID
	gotErr := withDarwinOwnedObjectRelease(
		objc.ID(7),
		func(id objc.ID) { released = append(released, id) },
		func(id objc.ID) error {
			if id != objc.ID(7) {
				t.Fatalf("id = %v, want 7", id)
			}
			return nil
		},
	)
	if gotErr != nil {
		t.Fatalf("withDarwinOwnedObjectRelease returned error: %v", gotErr)
	}
	if len(released) != 1 || released[0] != objc.ID(7) {
		t.Fatalf("released = %#v, want [7]", released)
	}
}

func TestWithDarwinOwnedObjectRelease_ReleasesOnError(t *testing.T) {
	var released []objc.ID
	wantErr := errors.New("boom")
	gotErr := withDarwinOwnedObjectRelease(
		objc.ID(9),
		func(id objc.ID) { released = append(released, id) },
		func(id objc.ID) error {
			if id != objc.ID(9) {
				t.Fatalf("id = %v, want 9", id)
			}
			return wantErr
		},
	)
	if !errors.Is(gotErr, wantErr) {
		t.Fatalf("error = %v, want %v", gotErr, wantErr)
	}
	if len(released) != 1 || released[0] != objc.ID(9) {
		t.Fatalf("released = %#v, want [9]", released)
	}
}

func TestWithDarwinOwnedObjectRelease_SkipsZeroObject(t *testing.T) {
	released := false
	gotErr := withDarwinOwnedObjectRelease(
		0,
		func(id objc.ID) { released = true },
		func(id objc.ID) error {
			if id != 0 {
				t.Fatalf("id = %v, want 0", id)
			}
			return nil
		},
	)
	if gotErr != nil {
		t.Fatalf("withDarwinOwnedObjectRelease returned error: %v", gotErr)
	}
	if released {
		t.Fatal("expected zero object not to be released")
	}
}
