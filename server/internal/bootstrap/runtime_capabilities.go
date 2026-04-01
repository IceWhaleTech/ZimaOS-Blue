package bootstrap

import (
	"database/sql"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func newRuntimeCapabilityContract(
	writeDB *sql.DB,
	readDB *sql.DB,
	cfg *config.Config,
	logger *zap.Logger,
	workspaceSource runtimeWorkspaceManagerSource,
	webSearchConfig tools.WebSearchConfig,
) runtimeCapabilityAdapter {
	return newRuntimeCapabilityAdapter(newRuntimeCapabilityImplementation(
		writeDB,
		readDB,
		cfg,
		logger,
		workspaceSource,
		webSearchConfig,
	))
}
