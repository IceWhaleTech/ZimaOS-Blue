package bootstrap

import (
	"database/sql"

	convertsvc "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/convert"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func newRuntimeConvertSupport(
	db *sql.DB,
	dataDir string,
	memoryStore *memory.Store,
	registry *tools.Registry,
	approvals *tools.ApprovalManager,
	dirStore *tools.DirAllowlistStore,
	allowedPaths []string,
) (*convertsvc.Service, *convertsvc.Handler, error) {
	return newRuntimeConvertSupportWithReadDB(db, db, dataDir, memoryStore, registry, approvals, dirStore, allowedPaths)
}
