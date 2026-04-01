package bootstrap

import (
	"database/sql"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selfreflect"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func newRuntimeCapabilityServices(webSearchConfig tools.WebSearchConfig) runtimeCapabilityContract {
	return runtimeCapabilityContract{
		Research: deepresearch.NewService(nil, deepresearch.NewToolWebSearcherWithConfig(webSearchConfig)),
		Reflect:  selfreflect.NewService(nil, nil),
	}
}

func bindRuntimeCapabilityProposalStore(contract *runtimeCapabilityContract, writeDB, readDB *sql.DB, logger *zap.Logger) {
	if contract == nil || writeDB == nil || contract.Reflect == nil {
		return
	}
	proposalStore, err := selfreflect.NewSQLiteProposalStoreWithReadDB(writeDB, readDB)
	if err != nil {
		if logger != nil {
			logger.Warn("Failed to initialize self-reflect proposal store", zap.Error(err))
		}
		return
	}
	contract.Reflect.SetProposalStore(proposalStore)
}

func bindRuntimeCapabilityWorkspaceManager(contract *runtimeCapabilityContract, workspaceSource runtimeWorkspaceManagerSource) {
	if contract == nil || workspaceSource == nil || contract.Reflect == nil {
		return
	}
	if mgr := workspaceSource.Manager(); mgr != nil {
		contract.Reflect.SetWorkspaceManager(mgr)
	}
}

func bindRuntimeCapabilityHarness(contract *runtimeCapabilityContract, writeDB, readDB *sql.DB, cfg *config.Config, logger *zap.Logger) {
	if contract == nil {
		return
	}
	var (
		bundle *HarnessRuntimeBundle
		err    error
	)
	if readDB != nil {
		bundle, err = newHarnessRuntimeBundleWithReadDB(writeDB, readDB, cfg, contract.Reflect)
	} else {
		bundle, err = newHarnessRuntimeBundle(writeDB, cfg, contract.Reflect)
	}
	if err != nil {
		if logger != nil {
			logger.Warn("Failed to initialize harness runtime bundle", zap.Error(err))
		}
		return
	}
	contract.Harness = bundle
}
