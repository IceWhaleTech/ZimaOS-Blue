package bootstrap

import (
	"database/sql"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func newRuntimeCapabilityImplementation(
	writeDB *sql.DB,
	readDB *sql.DB,
	cfg *config.Config,
	logger *zap.Logger,
	workspaceSource runtimeWorkspaceManagerSource,
	webSearchConfig tools.WebSearchConfig,
) runtimeCapabilityContract {
	contract := newRuntimeCapabilityServices(webSearchConfig)
	workspaceSource = normalizeRuntimeWorkspaceManagerSource(workspaceSource)
	bindRuntimeCapabilityProposalStore(&contract, writeDB, readDB, logger)
	bindRuntimeCapabilityWorkspaceManager(&contract, workspaceSource)
	bindRuntimeCapabilityHarness(&contract, writeDB, readDB, cfg, logger)
	return contract
}
