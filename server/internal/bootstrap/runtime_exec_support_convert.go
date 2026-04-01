package bootstrap

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/labstack/echo/v4"

	convertsvc "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/convert"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func newRuntimeConvertSupportWithReadDB(
	writeDB *sql.DB,
	readDB *sql.DB,
	dataDir string,
	memoryStore *memory.Store,
	registry *tools.Registry,
	approvals *tools.ApprovalManager,
	dirStore *tools.DirAllowlistStore,
	allowedPaths []string,
) (*convertsvc.Service, *convertsvc.Handler, error) {
	service, err := convertsvc.NewServiceWithReadDB(writeDB, readDB, dataDir)
	if err != nil {
		return nil, nil, err
	}
	handler := convertsvc.NewHandler(service, newRuntimeConvertConversationAuthorizer(memoryStore))
	if registry != nil {
		tools.RegisterConvertTool(registry, service, approvals, dirStore, allowedPaths)
	}
	return service, handler, nil
}

func newRuntimeConvertConversationAuthorizer(memoryStore *memory.Store) func(ctx context.Context, userID, conversationID string) error {
	return func(ctx context.Context, userID, conversationID string) error {
		conv, err := memoryStore.GetConversation(ctx, conversationID, userID)
		if err != nil {
			if err == memory.ErrNotFound {
				return echo.NewHTTPError(http.StatusNotFound, "conversation not found")
			}
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get conversation")
		}
		_ = conv
		return nil
	}
}
