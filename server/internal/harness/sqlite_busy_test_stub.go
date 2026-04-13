//go:build !cgo

package harness

import "errors"

func newSQLiteBusyErrorForTest() error {
	return errors.New("database is locked")
}
