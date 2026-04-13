//go:build cgo

package harness

import (
	"errors"

	sqlite3 "github.com/mattn/go-sqlite3"
)

func isSQLiteBusyDriverError(err error) bool {
	var sqliteErr sqlite3.Error
	return errors.As(err, &sqliteErr) &&
		(sqliteErr.Code == sqlite3.ErrBusy || sqliteErr.Code == sqlite3.ErrLocked)
}
