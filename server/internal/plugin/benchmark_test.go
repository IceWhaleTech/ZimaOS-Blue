package plugin

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
)

// BenchmarkPluginRegistry benchmarks plugin registry operations
func BenchmarkPluginRegistry(b *testing.B) {
	b.Run("RegisterNativePlugin", func(b *testing.B) {
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			registry := NewRegistry()
			plugin := &benchMockNativePlugin{
				id: fmt.Sprintf("plugin-%d", i),
				manifest: &Manifest{
					ID:      fmt.Sprintf("plugin-%d", i),
					Name:    fmt.Sprintf("Plugin %d", i),
					Version: "1.0.0",
				},
			}
			_ = registry.RegisterNativePlugin(plugin)
		}
	})

	b.Run("GetPlugin", func(b *testing.B) {
		registry := NewRegistry()
		// Pre-register plugins
		for i := 0; i < 100; i++ {
			plugin := &benchMockNativePlugin{
				id: fmt.Sprintf("plugin-%d", i),
				manifest: &Manifest{
					ID:      fmt.Sprintf("plugin-%d", i),
					Name:    fmt.Sprintf("Plugin %d", i),
					Version: "1.0.0",
				},
			}
			registry.RegisterNativePlugin(plugin)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = registry.GetPlugin(fmt.Sprintf("plugin-%d", i%100))
		}
	})

	b.Run("ListPlugins", func(b *testing.B) {
		registry := NewRegistry()
		// Pre-register plugins
		for i := 0; i < 100; i++ {
			plugin := &benchMockNativePlugin{
				id: fmt.Sprintf("plugin-%d", i),
				manifest: &Manifest{
					ID:      fmt.Sprintf("plugin-%d", i),
					Name:    fmt.Sprintf("Plugin %d", i),
					Version: "1.0.0",
				},
			}
			registry.RegisterNativePlugin(plugin)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = registry.ListPlugins()
		}
	})
}

// benchMockNativePlugin is a mock implementation for benchmarks
type benchMockNativePlugin struct {
	id       string
	manifest *Manifest
}

func (m *benchMockNativePlugin) ID() string           { return m.id }
func (m *benchMockNativePlugin) Manifest() *Manifest  { return m.manifest }
func (m *benchMockNativePlugin) IsNative() bool       { return true }
func (m *benchMockNativePlugin) Init(ctx context.Context, api PluginAPI) error { return nil }
func (m *benchMockNativePlugin) Start(ctx context.Context) error               { return nil }
func (m *benchMockNativePlugin) Stop(ctx context.Context) error                { return nil }

// BenchmarkDependencyResolver benchmarks dependency resolution
func BenchmarkDependencyResolver(b *testing.B) {
	b.Run("SmallGraph", func(b *testing.B) {
		// 10 plugins with linear dependencies
		plugins := make(map[string]*PluginInfo, 10)
		for i := 0; i < 10; i++ {
			manifest := &Manifest{
				ID:      fmt.Sprintf("plugin-%d", i),
				Version: "1.0.0",
			}
			if i > 0 {
				manifest.Dependencies = []Dependency{
					{ID: fmt.Sprintf("plugin-%d", i-1), Version: "^1.0.0"},
				}
			}
			plugins[manifest.ID] = &PluginInfo{
				Manifest: manifest,
				Status:   StatusLoaded,
			}
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			resolver := NewDependencyResolver(plugins)
			_ = resolver.Resolve()
		}
	})

	b.Run("MediumGraph", func(b *testing.B) {
		// 50 plugins with tree-like dependencies
		plugins := make(map[string]*PluginInfo, 50)
		for i := 0; i < 50; i++ {
			manifest := &Manifest{
				ID:      fmt.Sprintf("plugin-%d", i),
				Version: "1.0.0",
			}
			if i > 0 {
				// Each plugin depends on its "parent" in a binary tree
				parentIdx := (i - 1) / 2
				manifest.Dependencies = []Dependency{
					{ID: fmt.Sprintf("plugin-%d", parentIdx), Version: "^1.0.0"},
				}
			}
			plugins[manifest.ID] = &PluginInfo{
				Manifest: manifest,
				Status:   StatusLoaded,
			}
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			resolver := NewDependencyResolver(plugins)
			_ = resolver.Resolve()
		}
	})

	b.Run("LargeGraph", func(b *testing.B) {
		// 100 plugins with complex dependencies
		plugins := make(map[string]*PluginInfo, 100)
		for i := 0; i < 100; i++ {
			manifest := &Manifest{
				ID:      fmt.Sprintf("plugin-%d", i),
				Version: "1.0.0",
			}
			// Add multiple dependencies
			var deps []Dependency
			if i > 0 {
				deps = append(deps, Dependency{ID: fmt.Sprintf("plugin-%d", i-1), Version: "^1.0.0"})
			}
			if i > 10 {
				deps = append(deps, Dependency{ID: fmt.Sprintf("plugin-%d", i-10), Version: "^1.0.0"})
			}
			manifest.Dependencies = deps
			plugins[manifest.ID] = &PluginInfo{
				Manifest: manifest,
				Status:   StatusLoaded,
			}
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			resolver := NewDependencyResolver(plugins)
			_ = resolver.Resolve()
		}
	})
}

// BenchmarkIsolation benchmarks plugin isolation overhead
func BenchmarkIsolation(b *testing.B) {
	b.Run("SafeExecute", func(b *testing.B) {
		ctx := context.Background()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = safeExecute(ctx, 0, "test", "bench", func(ctx context.Context) error {
				// Simulate minimal work
				_ = i * 2
				return nil
			})
		}
	})

	b.Run("SafeExecuteWithResult", func(b *testing.B) {
		ctx := context.Background()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = safeExecuteWithResult(ctx, 0, "test", "bench", func(ctx context.Context) (int, error) {
				return i * 2, nil
			})
		}
	})

	b.Run("WithTimeout", func(b *testing.B) {
		config := DefaultIsolationConfig()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), config.ToolTimeout)
			done := make(chan struct{})
			go func() {
				_ = i * 2
				close(done)
			}()
			select {
			case <-done:
			case <-ctx.Done():
			}
			cancel()
		}
	})
}

// BenchmarkResourceMonitor benchmarks resource monitoring
func BenchmarkResourceMonitor(b *testing.B) {
	b.Run("AcquireRelease", func(b *testing.B) {
		limits := &ResourceLimits{
			MaxConcurrentOps: 1000,
		}
		monitor := NewResourceMonitor("bench-plugin", limits)
		ctx := context.Background()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = monitor.AcquireOperation(ctx)
			monitor.ReleaseOperation()
		}
	})

	b.Run("ConcurrentOperations", func(b *testing.B) {
		limits := &ResourceLimits{
			MaxConcurrentOps: 100,
		}
		monitor := NewResourceMonitor("bench-plugin", limits)
		ctx := context.Background()

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if monitor.AcquireOperation(ctx) == nil {
					monitor.ReleaseOperation()
				}
			}
		})
	})
}

// BenchmarkPluginScheduler benchmarks plugin scheduling
func BenchmarkPluginScheduler(b *testing.B) {
	b.Run("Schedule", func(b *testing.B) {
		scheduler := NewPluginScheduler("bench-plugin", 10)
		ctx := context.Background()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = scheduler.Schedule(ctx, func(ctx context.Context) error {
				return nil
			})
		}
	})

	b.Run("ConcurrentSchedule", func(b *testing.B) {
		scheduler := NewPluginScheduler("bench-plugin", 100)
		ctx := context.Background()

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = scheduler.Schedule(ctx, func(ctx context.Context) error {
					return nil
				})
			}
		})
	})
}

// BenchmarkPluginSandbox benchmarks sandbox operations
func BenchmarkPluginSandbox(b *testing.B) {
	b.Run("Go", func(b *testing.B) {
		limits := &ResourceLimits{
			MaxGoroutines: 10000,
		}
		sandbox := NewPluginSandbox("bench-plugin", limits)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var wg sync.WaitGroup
			wg.Add(1)
			_ = sandbox.Go(func() {
				wg.Done()
			})
			wg.Wait()
		}
	})
}

// BenchmarkVersionConstraint benchmarks version constraint checking
func BenchmarkVersionConstraint(b *testing.B) {
	resolver := NewDependencyResolver(nil)

	testCases := []struct {
		name       string
		constraint string
		version    string
	}{
		{"Exact", "1.0.0", "1.0.0"},
		{"Caret", "^1.0.0", "1.5.0"},
		{"Tilde", "~1.0.0", "1.0.5"},
		{"GreaterEqual", ">=1.0.0", "2.0.0"},
		{"Range", ">=1.0.0 <2.0.0", "1.5.0"},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = resolver.checkVersionConstraint(tc.version, tc.constraint)
			}
		})
	}
}

// PerformanceBaseline captures baseline performance metrics
type PerformanceBaseline struct {
	RegistryRegisterNs    int64
	RegistryGetNs         int64
	DependencyResolveNs   int64
	IsolationOverheadNs   int64
	SchedulerOverheadNs   int64
}

// GetPerformanceBaseline returns expected baseline performance
func GetPerformanceBaseline() *PerformanceBaseline {
	return &PerformanceBaseline{
		RegistryRegisterNs:    10000,   // 10µs
		RegistryGetNs:         500,     // 500ns
		DependencyResolveNs:   100000,  // 100µs for small graph
		IsolationOverheadNs:   5000,    // 5µs (includes goroutine overhead)
		SchedulerOverheadNs:   10000,   // 10µs
	}
}

// TestPerformanceRegression tests for performance regressions
func TestPerformanceRegression(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("disabled in CI: nanosecond-scale plugin performance thresholds are host-load sensitive")
	}

	baseline := GetPerformanceBaseline()
	tolerance := 3.0 // Allow 3x baseline for CI variability

	t.Run("RegistryRegister", func(t *testing.T) {
		iterations := 1000

		start := time.Now()
		for i := 0; i < iterations; i++ {
			registry := NewRegistry()
			plugin := &benchMockNativePlugin{
				id: fmt.Sprintf("plugin-%d", i),
				manifest: &Manifest{
					ID:      fmt.Sprintf("plugin-%d", i),
					Name:    fmt.Sprintf("Plugin %d", i),
					Version: "1.0.0",
				},
			}
			_ = registry.RegisterNativePlugin(plugin)
		}
		elapsed := time.Since(start)
		avgNs := elapsed.Nanoseconds() / int64(iterations)

		if avgNs > int64(float64(baseline.RegistryRegisterNs)*tolerance) {
			t.Errorf("Registry.Register performance regression: %dns (baseline: %dns, tolerance: %.1fx)",
				avgNs, baseline.RegistryRegisterNs, tolerance)
		}
		t.Logf("Registry.Register: %dns/op (baseline: %dns)", avgNs, baseline.RegistryRegisterNs)
	})

	t.Run("RegistryGet", func(t *testing.T) {
		registry := NewRegistry()
		// Pre-register plugins
		for i := 0; i < 100; i++ {
			plugin := &benchMockNativePlugin{
				id: fmt.Sprintf("plugin-%d", i),
				manifest: &Manifest{
					ID:      fmt.Sprintf("plugin-%d", i),
					Name:    fmt.Sprintf("Plugin %d", i),
					Version: "1.0.0",
				},
			}
			registry.RegisterNativePlugin(plugin)
		}

		iterations := 10000
		start := time.Now()
		for i := 0; i < iterations; i++ {
			_ = registry.GetPlugin(fmt.Sprintf("plugin-%d", i%100))
		}
		elapsed := time.Since(start)
		avgNs := elapsed.Nanoseconds() / int64(iterations)

		if avgNs > int64(float64(baseline.RegistryGetNs)*tolerance) {
			t.Errorf("Registry.Get performance regression: %dns (baseline: %dns, tolerance: %.1fx)",
				avgNs, baseline.RegistryGetNs, tolerance)
		}
		t.Logf("Registry.Get: %dns/op (baseline: %dns)", avgNs, baseline.RegistryGetNs)
	})

	t.Run("DependencyResolve", func(t *testing.T) {
		// 10 plugins with linear dependencies
		plugins := make(map[string]*PluginInfo, 10)
		for i := 0; i < 10; i++ {
			manifest := &Manifest{
				ID:      fmt.Sprintf("plugin-%d", i),
				Version: "1.0.0",
			}
			if i > 0 {
				manifest.Dependencies = []Dependency{
					{ID: fmt.Sprintf("plugin-%d", i-1), Version: "^1.0.0"},
				}
			}
			plugins[manifest.ID] = &PluginInfo{
				Manifest: manifest,
				Status:   StatusLoaded,
			}
		}

		iterations := 1000
		start := time.Now()
		for i := 0; i < iterations; i++ {
			resolver := NewDependencyResolver(plugins)
			_ = resolver.Resolve()
		}
		elapsed := time.Since(start)
		avgNs := elapsed.Nanoseconds() / int64(iterations)

		if avgNs > int64(float64(baseline.DependencyResolveNs)*tolerance) {
			t.Errorf("DependencyResolver.Resolve performance regression: %dns (baseline: %dns, tolerance: %.1fx)",
				avgNs, baseline.DependencyResolveNs, tolerance)
		}
		t.Logf("DependencyResolver.Resolve: %dns/op (baseline: %dns)", avgNs, baseline.DependencyResolveNs)
	})

	t.Run("IsolationOverhead", func(t *testing.T) {
		ctx := context.Background()
		iterations := 10000
		start := time.Now()
		for i := 0; i < iterations; i++ {
			_ = safeExecute(ctx, 0, "test", "bench", func(ctx context.Context) error {
				_ = i * 2
				return nil
			})
		}
		elapsed := time.Since(start)
		avgNs := elapsed.Nanoseconds() / int64(iterations)

		if avgNs > int64(float64(baseline.IsolationOverheadNs)*tolerance) {
			t.Errorf("Isolation overhead regression: %dns (baseline: %dns, tolerance: %.1fx)",
				avgNs, baseline.IsolationOverheadNs, tolerance)
		}
		t.Logf("Isolation overhead: %dns/op (baseline: %dns)", avgNs, baseline.IsolationOverheadNs)
	})

	t.Run("SchedulerOverhead", func(t *testing.T) {
		scheduler := NewPluginScheduler("test-plugin", 10)
		ctx := context.Background()

		iterations := 1000
		start := time.Now()
		for i := 0; i < iterations; i++ {
			_ = scheduler.Schedule(ctx, func(ctx context.Context) error {
				return nil
			})
		}
		elapsed := time.Since(start)
		avgNs := elapsed.Nanoseconds() / int64(iterations)

		if avgNs > int64(float64(baseline.SchedulerOverheadNs)*tolerance) {
			t.Errorf("Scheduler overhead regression: %dns (baseline: %dns, tolerance: %.1fx)",
				avgNs, baseline.SchedulerOverheadNs, tolerance)
		}
		t.Logf("Scheduler overhead: %dns/op (baseline: %dns)", avgNs, baseline.SchedulerOverheadNs)
	})
}
