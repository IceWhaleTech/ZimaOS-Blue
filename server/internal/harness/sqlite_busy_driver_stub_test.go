//go:build !cgo

package harness

import (
	"errors"
	"testing"
)

func TestIsSQLiteBusyDriverErrorIsDisabledWithoutCGO(t *testing.T) {
	if isSQLiteBusyDriverError(errors.New("database is locked")) {
		t.Fatal("expected non-cgo busy driver matcher to stay disabled")
	}
}
