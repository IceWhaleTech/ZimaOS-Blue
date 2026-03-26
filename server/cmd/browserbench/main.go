package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type scenario struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Category  string `json:"category"`
	URL       string `json:"url"`
	Operation string `json:"operation"`
	TimeoutMS int    `json:"timeout_ms"`
	Notes     string `json:"notes,omitempty"`
}

type engineMetrics struct {
	Engine        string    `json:"engine"`
	Status        string    `json:"status"`
	ColdMS        float64   `json:"cold_ms,omitempty"`
	WarmRunsMS    []float64 `json:"warm_runs_ms,omitempty"`
	WarmP50MS     float64   `json:"warm_p50_ms,omitempty"`
	ColdError     string    `json:"cold_error,omitempty"`
	WarmErrors    []string  `json:"warm_errors,omitempty"`
	SuccessfulRun int       `json:"successful_runs,omitempty"`
}

type scenarioResult struct {
	Scenario scenario        `json:"scenario"`
	Engines  []engineMetrics `json:"engines"`
}

type benchReport struct {
	GeneratedAt       string           `json:"generated_at"`
	Hostname          string           `json:"hostname,omitempty"`
	GOOS              string           `json:"goos"`
	GOARCH            string           `json:"goarch"`
	WarmRuns          int              `json:"warm_runs"`
	LightpandaRuntime string           `json:"lightpanda_runtime"`
	BinaryReadyPath   string           `json:"lightpanda_binary_path,omitempty"`
	DockerImage       string           `json:"lightpanda_docker_image,omitempty"`
	Notes             []string         `json:"notes,omitempty"`
	Results           []scenarioResult `json:"results"`
}

type backendFactory func(ctx context.Context) (tools.BrowserBackend, func(), error)

func percentile50(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	return sorted[len(sorted)/2]
}

func classifyBenchmarkError(err error) string {
	if err == nil {
		return "ok"
	}
	if errors.Is(err, browser.ErrLightpandaUnsupportedCapability) || strings.Contains(strings.ToLower(err.Error()), "unsupported_capability") {
		return "unsupported"
	}
	return "error"
}

func benchFlow(ctx context.Context, backend tools.BrowserBackend, op string, rawURL string, targetID string) (string, error) {
	if err := backend.Start(ctx); err != nil {
		return targetID, err
	}
	switch strings.TrimSpace(op) {
	case "a11y_tree":
		nav, err := backend.Navigate(ctx, rawURL, targetID)
		if err != nil {
			return targetID, err
		}
		targetID = nav.TargetID
		_, err = backend.AccessibilityTree(ctx, targetID, 10)
		return targetID, err
	case "screenshot":
		if strings.TrimSpace(targetID) == "" {
			_, err := backend.Screenshot(ctx, rawURL)
			return "", err
		}
		_, err := backend.ScreenshotTab(ctx, targetID)
		return targetID, err
	default:
		return targetID, fmt.Errorf("unsupported operation %q", op)
	}
}

func measureScenario(ctx context.Context, factory backendFactory, sc scenario, warmRuns int) engineMetrics {
	metrics := engineMetrics{}

	coldBackend, coldCleanup, err := factory(ctx)
	if err != nil {
		metrics.Status = classifyBenchmarkError(err)
		metrics.ColdError = err.Error()
		return metrics
	}
	started := time.Now()
	_, err = benchFlow(contextWithTimeout(ctx, sc.TimeoutMS), coldBackend, sc.Operation, sc.URL, "")
	metrics.ColdMS = durationMS(time.Since(started))
	if err != nil {
		metrics.Status = classifyBenchmarkError(err)
		metrics.ColdError = err.Error()
		coldCleanup()
		return metrics
	}
	coldCleanup()

	warmBackend, warmCleanup, err := factory(ctx)
	if err != nil {
		metrics.Status = classifyBenchmarkError(err)
		metrics.WarmErrors = []string{err.Error()}
		return metrics
	}
	defer warmCleanup()

	targetID := ""
	targetID, err = benchFlow(contextWithTimeout(ctx, sc.TimeoutMS), warmBackend, sc.Operation, sc.URL, targetID)
	if err != nil {
		metrics.Status = classifyBenchmarkError(err)
		metrics.WarmErrors = []string{err.Error()}
		return metrics
	}

	warmValues := make([]float64, 0, warmRuns)
	warmErrors := make([]string, 0, warmRuns)
	for i := 0; i < warmRuns; i++ {
		started = time.Now()
		nextTargetID, runErr := benchFlow(contextWithTimeout(ctx, sc.TimeoutMS), warmBackend, sc.Operation, sc.URL, targetID)
		if runErr != nil {
			warmErrors = append(warmErrors, runErr.Error())
			continue
		}
		targetID = nextTargetID
		warmValues = append(warmValues, durationMS(time.Since(started)))
	}
	if targetID != "" {
		_ = warmBackend.CloseTab(contextWithTimeout(ctx, sc.TimeoutMS), targetID)
	}

	metrics.WarmRunsMS = warmValues
	metrics.WarmP50MS = percentile50(warmValues)
	metrics.WarmErrors = warmErrors
	metrics.SuccessfulRun = len(warmValues)
	if len(warmValues) == 0 && len(warmErrors) > 0 {
		metrics.Status = classifyBenchmarkError(errors.New(warmErrors[0]))
	} else {
		metrics.Status = "ok"
	}
	return metrics
}

func contextWithTimeout(parent context.Context, timeoutMS int) context.Context {
	timeout := 30 * time.Second
	if timeoutMS > 0 {
		timeout = time.Duration(timeoutMS) * time.Millisecond
	}
	ctx, _ := context.WithTimeout(parent, timeout)
	return ctx
}

func durationMS(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}

func syntheticServer() *httptest.Server {
	denseHTML := func() string {
		var builder strings.Builder
		builder.WriteString(`<!doctype html><html><head><title>Dense DOM</title></head><body><main><h1>Dense DOM</h1>`)
		for i := 0; i < 240; i++ {
			builder.WriteString(fmt.Sprintf(`<section><h2>Section %d</h2><p>Item %d content for benchmark.</p><a href="/dense?item=%d">Read more</a></section>`, i, i, i))
		}
		builder.WriteString(`</main></body></html>`)
		return builder.String()
	}

	jsShellHTML := `<!doctype html><html><head><title>JS Shell</title></head><body><div id="root"></div><script>
document.getElementById('root').innerHTML = '<main><h1>Hydrated App</h1><button>Continue</button><p>Client-rendered content</p></main>';
</script><script>window.__bench = true;</script><script>window.__bench_ready = true;</script></body></html>`

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/article":
			_, _ = w.Write([]byte(`<!doctype html><html><head><title>Article</title></head><body><main><article><h1>Readable article</h1><p>Blue benchmark article content.</p><p>This page is intentionally simple and mostly text.</p></article></main></body></html>`))
		case "/dense":
			_, _ = w.Write([]byte(denseHTML()))
		case "/js-shell":
			_, _ = w.Write([]byte(jsShellHTML))
		default:
			http.NotFound(w, r)
		}
	}))
}

func defaultScenarios(baseURL string) []scenario {
	return []scenario{
		{
			ID:        "synthetic_static_article",
			Label:     "Synthetic static article / readable tree",
			Category:  "synthetic",
			URL:       baseURL + "/article",
			Operation: "a11y_tree",
			TimeoutMS: 15000,
			Notes:     "Measures navigate + accessibility tree on a simple readable page.",
		},
		{
			ID:        "synthetic_dense_dom",
			Label:     "Synthetic dense DOM / readable tree",
			Category:  "synthetic",
			URL:       baseURL + "/dense",
			Operation: "a11y_tree",
			TimeoutMS: 20000,
			Notes:     "Measures navigate + accessibility tree on a large local DOM.",
		},
		{
			ID:        "synthetic_js_shell",
			Label:     "Synthetic JS shell / hydrated app",
			Category:  "synthetic",
			URL:       baseURL + "/js-shell",
			Operation: "a11y_tree",
			TimeoutMS: 20000,
			Notes:     "Shim would fail fast here; lp binary and Chromium execute the client-side render.",
		},
		{
			ID:        "synthetic_screenshot_baseline",
			Label:     "Synthetic screenshot baseline",
			Category:  "synthetic",
			URL:       baseURL + "/article",
			Operation: "screenshot",
			TimeoutMS: 20000,
			Notes:     "Current Blue browser-lite contract keeps screenshots Chromium-only.",
		},
		{
			ID:        "site_example",
			Label:     "https://example.com",
			Category:  "real",
			URL:       "https://example.com",
			Operation: "a11y_tree",
			TimeoutMS: 45000,
		},
		{
			ID:        "site_mdn_html",
			Label:     "https://developer.mozilla.org/en-US/docs/Web/HTML",
			Category:  "real",
			URL:       "https://developer.mozilla.org/en-US/docs/Web/HTML",
			Operation: "a11y_tree",
			TimeoutMS: 60000,
		},
		{
			ID:        "site_apple",
			Label:     "https://www.apple.com/",
			Category:  "real",
			URL:       "https://www.apple.com/",
			Operation: "a11y_tree",
			TimeoutMS: 60000,
		},
		{
			ID:        "site_github",
			Label:     "https://github.com/",
			Category:  "real",
			URL:       "https://github.com/",
			Operation: "a11y_tree",
			TimeoutMS: 60000,
		},
		{
			ID:        "site_figma",
			Label:     "https://www.figma.com/",
			Category:  "real",
			URL:       "https://www.figma.com/",
			Operation: "a11y_tree",
			TimeoutMS: 60000,
		},
	}
}

func binaryFactory(cfg *browser.Config) backendFactory {
	return func(context.Context) (tools.BrowserBackend, func(), error) {
		runtime := browser.NewLightpandaBinaryRuntime(cfg)
		backend := tools.NewLightpandaBinaryBrowserBackend(runtime)
		cleanup := func() {
			_ = runtime.Stop(context.Background())
		}
		return backend, cleanup, nil
	}
}

func chromiumFactory(cfg *browser.Config) backendFactory {
	return func(context.Context) (tools.BrowserBackend, func(), error) {
		service, err := browser.NewService(cfg.CloneForDriver("managed"))
		if err != nil {
			return nil, nil, err
		}
		backend := tools.NewRodBrowserBackend(service)
		cleanup := func() {
			_ = service.Close()
		}
		return backend, cleanup, nil
	}
}

func warmChromiumRuntime(ctx context.Context, cfg *browser.Config) error {
	service, err := browser.NewService(cfg.CloneForDriver("managed"))
	if err != nil {
		return err
	}
	defer service.Close()
	return service.Start(contextWithTimeout(ctx, 20000))
}

func main() {
	outputPath := flag.String("output", "", "Optional JSON output path")
	warmRuns := flag.Int("warm-runs", 3, "Warm iterations per scenario")
	lightpandaRuntime := flag.String("lightpanda-runtime", "auto", "Lightpanda runtime mode: auto, binary, or docker")
	lightpandaDockerImage := flag.String("lightpanda-docker-image", "lightpanda/browser:nightly", "Docker image used when lightpanda-runtime=docker or auto falls back to docker")
	flag.Parse()

	ctx := context.Background()
	cfg := browser.DefaultConfig()
	cfg.PoolSize = 1
	cfg.Headless = true
	cfg.Lightpanda.Enabled = true

	lightpandaSelection, err := resolveLightpandaFactory(ctx, cfg, *lightpandaRuntime, *lightpandaDockerImage)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to prepare lightpanda runtime: %v\n", err)
		os.Exit(1)
	}
	if err := warmChromiumRuntime(ctx, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "failed to warm chromium runtime: %v\n", err)
		os.Exit(1)
	}

	srv := syntheticServer()
	defer srv.Close()

	report := benchReport{
		GeneratedAt:       time.Now().UTC().Format(time.RFC3339),
		GOOS:              runtime.GOOS,
		GOARCH:            runtime.GOARCH,
		WarmRuns:          *warmRuns,
		LightpandaRuntime: lightpandaSelection.Runtime,
		BinaryReadyPath:   lightpandaSelection.BinaryReadyPath,
		DockerImage:       lightpandaSelection.DockerImage,
		Notes: []string{
			"Cold timings exclude one-time browser/binary download and measure engine startup + navigate + operation.",
			"Warm timings reuse an already-started engine and report p50 over measured iterations after one uncounted priming run.",
			"lightpanda_binary is measured through Blue's current browser-lite backend contract, so screenshot remains unsupported by design.",
		},
	}
	report.Notes = append(report.Notes, lightpandaSelection.Notes...)
	if host, hostErr := os.Hostname(); hostErr == nil {
		report.Hostname = host
	}

	scenarios := defaultScenarios(srv.URL)
	binaryNew := lightpandaSelection.Factory
	chromiumNew := chromiumFactory(cfg)

	for _, sc := range scenarios {
		fmt.Printf("Running %s\n", sc.Label)
		binaryMetrics := measureScenario(ctx, binaryNew, sc, *warmRuns)
		binaryMetrics.Engine = string(browser.SessionEngineDetailLightpandaBinary)
		chromiumMetrics := measureScenario(ctx, chromiumNew, sc, *warmRuns)
		chromiumMetrics.Engine = string(browser.SessionEngineDetailChromiumManaged)
		report.Results = append(report.Results, scenarioResult{
			Scenario: sc,
			Engines:  []engineMetrics{binaryMetrics, chromiumMetrics},
		})
	}

	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal report: %v\n", err)
		os.Exit(1)
	}

	if strings.TrimSpace(*outputPath) != "" {
		resolved := *outputPath
		if !filepath.IsAbs(resolved) {
			if cwd, cwdErr := os.Getwd(); cwdErr == nil {
				resolved = filepath.Join(cwd, resolved)
			}
		}
		if err := os.MkdirAll(filepath.Dir(resolved), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "failed to create output directory: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(resolved, payload, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "failed to write report: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Wrote %s\n", resolved)
	}

	fmt.Println(string(payload))
}
