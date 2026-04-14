package bootstrap

import (
	"database/sql"
	"testing"

	dbutil "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
)

func TestBindRouteRegistrationStateDBPrefersRuntimeDBConn(t *testing.T) {
	runtimeWrite, runtimeRead := &sql.DB{}, &sql.DB{}
	primaryWrite, primaryRead := &sql.DB{}, &sql.DB{}
	state := &routeRegistrationState{
		services: &Services{
			RuntimeDBConn: &dbutil.SQLiteConn{Writer: runtimeWrite, Reader: runtimeRead},
			DBConn:        &dbutil.SQLiteConn{Writer: primaryWrite, Reader: primaryRead},
		},
	}

	bindRouteRegistrationStateDB(state)

	if state.runtimeWriteDB != runtimeWrite || state.runtimeReadDB != runtimeRead {
		t.Fatalf("expected runtime db handles to come from RuntimeDBConn")
	}
}

func TestBindRouteRegistrationStateDBFallsBackToPrimaryDBConn(t *testing.T) {
	primaryWrite, primaryRead := &sql.DB{}, &sql.DB{}
	state := &routeRegistrationState{
		services: &Services{
			DBConn: &dbutil.SQLiteConn{Writer: primaryWrite, Reader: primaryRead},
		},
	}

	bindRouteRegistrationStateDB(state)

	if state.runtimeWriteDB != primaryWrite || state.runtimeReadDB != primaryRead {
		t.Fatalf("expected runtime db handles to fall back to DBConn")
	}
}
