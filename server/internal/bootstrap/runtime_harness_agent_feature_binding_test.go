package bootstrap

import (
	"context"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agent"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	harnessdrivers "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness/drivers"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
)

type stubBootstrapHarnessDriver struct {
	kind          harness.RunKind
	validateCount int
}

func (d *stubBootstrapHarnessDriver) Kind() harness.RunKind {
	return d.kind
}

func (d *stubBootstrapHarnessDriver) Validate(spec harness.RunSpec) error {
	d.validateCount++
	return nil
}

func (d *stubBootstrapHarnessDriver) Start(_ context.Context, _ *harness.Run, _ harness.RunEnv) error {
	return nil
}

func (d *stubBootstrapHarnessDriver) Cancel(_ context.Context, _ *harness.Run) error {
	return nil
}

func TestAgentRuntimeBindingRegisterPreservesSelectorDryRunMux(t *testing.T) {
	controller := harness.NewController(nil, nil)

	oldDefault := &stubBootstrapHarnessDriver{kind: harness.RunKindAgentTask}
	selectorOverride := &stubBootstrapHarnessDriver{kind: harness.RunKindAgentTask}
	mux := harnessdrivers.NewDriverMux(harness.RunKindAgentTask, oldDefault)
	mux.Register("selector_dry_run", selectorOverride)
	controller.RegisterDriver(mux)

	replacement := harnessdrivers.NewAgentDriver(harness.RunKindAgentTask, &agent.Runner{}, &agent.Store{}, controller)
	agentBinding := agentRuntimeBinding{agentDriver: replacement}
	agentBinding.register(controller)

	registered := controller.GetRegisteredDriver(harness.RunKindAgentTask)
	preservedMux, ok := registered.(*harnessdrivers.DriverMux)
	if !ok {
		t.Fatalf("registered agent task driver = %T, want *DriverMux", registered)
	}

	if err := preservedMux.Validate(harness.RunSpec{
		Kind:     harness.RunKindAgentTask,
		Goal:     "selector dry run",
		Metadata: map[string]interface{}{"driver": "selector_dry_run"},
	}); err != nil {
		t.Fatalf("selector mux Validate failed: %v", err)
	}
	if selectorOverride.validateCount != 1 {
		t.Fatalf("selector override validate count = %d, want 1", selectorOverride.validateCount)
	}

	if err := preservedMux.Validate(harness.RunSpec{
		Kind: harness.RunKindAgentTask,
		Goal: "plain agent task",
	}); err != nil {
		t.Fatalf("default mux Validate failed: %v", err)
	}
	if replacement.Validate(harness.RunSpec{Kind: harness.RunKindAgentTask, Goal: "plain agent task"}) != nil {
		t.Fatal("expected replacement driver to validate after mux default swap")
	}
	if oldDefault.validateCount != 0 {
		t.Fatalf("old default validate count = %d, want 0", oldDefault.validateCount)
	}
}

func TestBindRuntimeSelectorDryRunBeforeAgentRegistrationStillPreservesOverride(t *testing.T) {
	controller := harness.NewController(nil, nil)
	bundle := &HarnessRuntimeBundle{Controller: controller}
	settings := serverpkg.NewSettingsHandler(kvstore.NewMemoryStore())

	bindRuntimeSelectorDryRun(settings, bundle)

	replacement := harnessdrivers.NewAgentDriver(harness.RunKindAgentTask, &agent.Runner{}, &agent.Store{}, controller)
	agentBinding := agentRuntimeBinding{agentDriver: replacement}
	agentBinding.register(controller)

	registered := controller.GetRegisteredDriver(harness.RunKindAgentTask)
	preservedMux, ok := registered.(*harnessdrivers.DriverMux)
	if !ok {
		t.Fatalf("registered driver = %T, want *DriverMux", registered)
	}

	if err := preservedMux.Validate(harness.RunSpec{
		Kind: harness.RunKindAgentTask,
		Metadata: map[string]interface{}{
			"driver": "selector_dry_run",
			"query":  "Search the latest OpenAI Responses API documentation.",
		},
	}); err != nil {
		t.Fatalf("selector dry-run Validate failed after delayed agent registration: %v", err)
	}

	if err := preservedMux.Validate(harness.RunSpec{
		Kind: harness.RunKindAgentTask,
		Goal: "plain agent task",
	}); err != nil {
		t.Fatalf("default agent Validate failed after delayed registration: %v", err)
	}
}
