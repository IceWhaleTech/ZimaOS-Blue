package bootstrap

import (
	"database/sql"
	"reflect"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selfreflect"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func normalizeRuntimeWorkspaceManagerSource(source runtimeWorkspaceManagerSource) runtimeWorkspaceManagerSource {
	if source == nil {
		return nil
	}
	value := reflect.ValueOf(source)
	if value.Kind() == reflect.Pointer && value.IsNil() {
		return nil
	}
	return source
}

func harnessRuntimeController(bundle *HarnessRuntimeBundle) *harness.Controller {
	if bundle == nil {
		return nil
	}
	return bundle.Controller
}

func harnessRuntimeObserver(bundle *HarnessRuntimeBundle) tools.RuntimeEventObserver {
	if bundle == nil {
		return nil
	}
	return bundle.RuntimeObserver
}

func newHarnessRuntimeBundle(db *sql.DB, cfg *config.Config, reflectService *selfreflect.Service) (*HarnessRuntimeBundle, error) {
	return newHarnessRuntimeBundleWithReadDB(db, db, cfg, reflectService)
}

func newHarnessRuntimeBundleWithReadDB(writeDB, readDB *sql.DB, cfg *config.Config, reflectService *selfreflect.Service) (*HarnessRuntimeBundle, error) {
	if writeDB == nil || cfg == nil || !cfg.Harness.Enabled {
		return nil, nil
	}

	store, err := harness.NewSQLiteStoreWithReadDB(writeDB, readDB)
	if err != nil {
		return nil, err
	}

	controller := harness.NewController(store, harness.NewPolicyResolver(cfg.Harness, &cfg.Agents))
	if reflectService != nil {
		controller.SetReflector(reflectService)
	}
	runTracer := harness.NewRunTraceCollector(controller)
	if runTracer != nil {
		controller.SetRunTraceProvider(runTracer)
		controller.UseExecutionMiddleware(runTracer.Middleware())
	}
	controller.UseExecutionMiddleware(harness.NewSkillCandidateMiddleware())

	return &HarnessRuntimeBundle{
		Controller:       controller,
		GroupDispatcher:  harness.NewGroupDispatcher(controller),
		RunTracer:        runTracer,
		RuntimeObserver:  harness.NewRuntimeObserver(controller),
		SubagentExecutor: harness.NewSubagentExecutor(controller, &cfg.Agents),
		WriteGuard:       harness.NewWritePathGuard(controller),
		ExecGuard:        harness.NewExecPathGuard(controller),
	}, nil
}
