//go:build cgo

package harness

import (
	"testing"

	sqlite3 "github.com/mattn/go-sqlite3"
)

func TestIsSQLiteBusyDriverErrorRecognizesBusyAndLocked(t *testing.T) {
	if !isSQLiteBusyDriverError(sqlite3.Error{Code: sqlite3.ErrBusy}) {
		t.Fatal("expected sqlite busy error to be recognized")
	}
	if !isSQLiteBusyDriverError(sqlite3.Error{Code: sqlite3.ErrLocked}) {
		t.Fatal("expected sqlite locked error to be recognized")
	}
	if isSQLiteBusyDriverError(sqlite3.Error{Code: sqlite3.ErrAbort}) {
		t.Fatal("expected non-busy sqlite error to be rejected")
	}
}
