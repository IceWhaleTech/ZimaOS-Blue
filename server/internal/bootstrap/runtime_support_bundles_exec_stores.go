package bootstrap

import (
	"database/sql"
	"log/slog"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func newRuntimeExecDirStore(writeDB, readDB *sql.DB) *tools.DirAllowlistStore {
	if writeDB == nil {
		return nil
	}

	var (
		store *tools.DirAllowlistStore
		err   error
	)
	if readDB != nil {
		store, err = tools.NewDirAllowlistStoreWithReadDB(writeDB, readDB)
	} else {
		store, err = tools.NewDirAllowlistStore(writeDB)
	}
	if err != nil {
		slog.Warn("failed to create exec dir allowlist store", "error", err)
		return nil
	}
	return store
}

func newRuntimeExecAuditStore(writeDB, readDB *sql.DB) *tools.ExecAuditStore {
	if writeDB == nil {
		return nil
	}

	var (
		store *tools.ExecAuditStore
		err   error
	)
	if readDB != nil {
		store, err = tools.NewExecAuditStoreWithReadDB(writeDB, readDB)
	} else {
		store, err = tools.NewExecAuditStore(writeDB)
	}
	if err != nil {
		slog.Warn("failed to create exec audit store", "error", err)
		return nil
	}
	return store
}
