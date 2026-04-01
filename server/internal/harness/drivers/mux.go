package drivers

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
)

// DriverMux routes one run kind to different concrete drivers based on the
// metadata "driver" field, while preserving a default fallback driver.
type DriverMux struct {
	kind          harness.RunKind
	defaultDriver harness.Driver

	mu        sync.RWMutex
	overrides map[string]harness.Driver
}

func NewDriverMux(kind harness.RunKind, defaultDriver harness.Driver) *DriverMux {
	return &DriverMux{
		kind:          kind,
		defaultDriver: defaultDriver,
		overrides:     make(map[string]harness.Driver),
	}
}

func (m *DriverMux) Kind() harness.RunKind {
	if m == nil || m.kind == "" {
		return harness.RunKindAgentTask
	}
	return m.kind
}

func (m *DriverMux) Register(name string, driver harness.Driver) {
	if m == nil || driver == nil {
		return
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.overrides[name] = driver
}

func (m *DriverMux) SetDefaultDriver(driver harness.Driver) {
	if m == nil || driver == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.defaultDriver = driver
}

func (m *DriverMux) Validate(spec harness.RunSpec) error {
	driver, err := m.driverForMetadata(spec.Metadata)
	if err != nil {
		return err
	}
	slog.Info("Harness driver mux validate",
		"mux", fmt.Sprintf("%p", m),
		"kind", m.Kind(),
		"selected_driver_type", fmt.Sprintf("%T", driver),
		"metadata_driver", strings.TrimSpace(fmt.Sprint(spec.Metadata["driver"])),
	)
	return driver.Validate(spec)
}

func (m *DriverMux) Start(ctx context.Context, run *harness.Run, env harness.RunEnv) error {
	driver, err := m.driverForRun(run)
	if err != nil {
		return err
	}
	metadataDriver := ""
	runID := ""
	if run != nil {
		metadataDriver = strings.TrimSpace(fmt.Sprint(run.Metadata["driver"]))
		runID = strings.TrimSpace(run.ID)
	}
	slog.Info("Harness driver mux start",
		"mux", fmt.Sprintf("%p", m),
		"kind", m.Kind(),
		"run_id", runID,
		"selected_driver_type", fmt.Sprintf("%T", driver),
		"metadata_driver", metadataDriver,
	)
	return driver.Start(ctx, run, env)
}

func (m *DriverMux) Cancel(ctx context.Context, run *harness.Run) error {
	driver, err := m.driverForRun(run)
	if err != nil {
		return err
	}
	return driver.Cancel(ctx, run)
}

func (m *DriverMux) Sync(ctx context.Context, run *harness.Run) (*harness.Run, error) {
	driver, err := m.driverForRun(run)
	if err != nil {
		return run, err
	}
	provider, ok := driver.(harness.SnapshotDriver)
	if !ok {
		return run, nil
	}
	return provider.Sync(ctx, run)
}

func (m *DriverMux) ListRuntimeEvidence(ctx context.Context, run *harness.Run) ([]harness.RuntimeEvidenceEntry, error) {
	driver, err := m.driverForRun(run)
	if err != nil {
		return nil, err
	}
	provider, ok := driver.(harness.RuntimeEvidenceProvider)
	if !ok {
		return nil, nil
	}
	return provider.ListRuntimeEvidence(ctx, run)
}

func (m *DriverMux) driverForRun(run *harness.Run) (harness.Driver, error) {
	if run == nil {
		return m.driverForMetadata(nil)
	}
	return m.driverForMetadata(run.Metadata)
}

func (m *DriverMux) driverForMetadata(meta map[string]interface{}) (harness.Driver, error) {
	if m == nil {
		return nil, fmt.Errorf("driver mux is not configured")
	}
	name := strings.TrimSpace(fmt.Sprint(meta["driver"]))

	m.mu.RLock()
	defer m.mu.RUnlock()
	if name != "" {
		if override := m.overrides[name]; override != nil {
			return override, nil
		}
	}
	if m.defaultDriver != nil {
		return m.defaultDriver, nil
	}
	return nil, fmt.Errorf("driver mux for kind %q has no default driver", m.Kind())
}
