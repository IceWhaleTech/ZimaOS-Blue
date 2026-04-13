//go:build cgo

package harness

import sqlite3 "github.com/mattn/go-sqlite3"

func newSQLiteBusyErrorForTest() error {
	return sqlite3.Error{Code: sqlite3.ErrBusy}
}
