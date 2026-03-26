package browser

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	lightpandaBinaryHost           = "127.0.0.1"
	lightpandaBinaryStartupTimeout = 20 * time.Second
)

// LightpandaBinaryRuntime manages a local upstream Lightpanda process and the
// relay-mode Rod service Blue uses to attach to it.
type LightpandaBinaryRuntime struct {
	config        *Config
	binaryManager *LightpandaBinaryManager

	mu       sync.Mutex
	cond     *sync.Cond
	starting bool
	service  *RodService
	process  *exec.Cmd
	waitDone <-chan error
}

// NewLightpandaBinaryRuntime creates a new runtime manager for the upstream
// Lightpanda binary.
func NewLightpandaBinaryRuntime(config *Config) *LightpandaBinaryRuntime {
	cfg := config.Clone()
	runtime := &LightpandaBinaryRuntime{
		config:        cfg,
		binaryManager: NewLightpandaBinaryManager(cfg),
	}
	runtime.cond = sync.NewCond(&runtime.mu)
	return runtime
}

// PeekService returns the current runtime service without creating or starting
// a new Lightpanda process.
func (r *LightpandaBinaryRuntime) PeekService() *RodService {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.service
}

// Start returns a relay-mode Rod service bound to a live upstream Lightpanda
// process. The returned service is not started until callers invoke svc.Start.
func (r *LightpandaBinaryRuntime) Start(ctx context.Context) (*RodService, error) {
	if r == nil {
		return nil, ErrBrowserNotAvailable
	}
	if ctx == nil {
		ctx = context.Background()
	}

	r.mu.Lock()
	for r.starting {
		r.cond.Wait()
	}
	if r.service != nil {
		svc := r.service
		r.mu.Unlock()
		return svc, nil
	}
	r.starting = true
	r.mu.Unlock()

	svc, cmd, waitDone, err := r.startFresh(ctx)

	r.mu.Lock()
	defer r.mu.Unlock()
	r.starting = false
	if err == nil {
		r.service = svc
		r.process = cmd
		r.waitDone = waitDone
	}
	r.cond.Broadcast()
	if err != nil {
		return nil, err
	}
	return r.service, nil
}

// Stop stops the current runtime and clears the cached service.
func (r *LightpandaBinaryRuntime) Stop(ctx context.Context) error {
	if r == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	r.mu.Lock()
	for r.starting {
		r.cond.Wait()
	}
	svc := r.service
	cmd := r.process
	waitDone := r.waitDone
	r.service = nil
	r.process = nil
	r.waitDone = nil
	r.mu.Unlock()

	if svc != nil {
		_ = svc.Close()
	}
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	if waitDone != nil {
		select {
		case <-waitDone:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (r *LightpandaBinaryRuntime) startFresh(ctx context.Context) (*RodService, *exec.Cmd, <-chan error, error) {
	binaryPath, err := r.binaryPath(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	port, err := reserveLoopbackPort(lightpandaBinaryHost)
	if err != nil {
		return nil, nil, nil, err
	}
	cdpURL := fmt.Sprintf("http://%s:%d", lightpandaBinaryHost, port)
	args := buildLightpandaServeArgs(lightpandaBinaryHost, port, r.config.Lightpanda.Args)

	cmd := exec.Command(binaryPath, args...)
	var logs synchronizedBuffer
	cmd.Stdout = &logs
	cmd.Stderr = &logs
	if err := cmd.Start(); err != nil {
		return nil, nil, nil, fmt.Errorf("start lightpanda binary: %w", err)
	}

	waitDone := make(chan error, 1)
	go r.awaitProcess(cmd, waitDone)

	startupCtx := ctx
	if _, ok := startupCtx.Deadline(); !ok {
		var cancel context.CancelFunc
		startupCtx, cancel = context.WithTimeout(startupCtx, lightpandaBinaryStartupTimeout)
		defer cancel()
	}
	if err := waitForLightpandaCDP(startupCtx, cdpURL, waitDone); err != nil {
		_ = cmd.Process.Kill()
		return nil, nil, nil, fmt.Errorf("wait for lightpanda cdp: %w%s", err, formatLightpandaRuntimeLogs(logs.String()))
	}

	serviceCfg := r.config.CloneForDriver("relay")
	serviceCfg.CDPURL = cdpURL
	serviceCfg.RelayEnabled = false
	service, err := NewService(serviceCfg)
	if err != nil {
		_ = cmd.Process.Kill()
		return nil, nil, nil, err
	}
	return service, cmd, waitDone, nil
}

func (r *LightpandaBinaryRuntime) awaitProcess(cmd *exec.Cmd, done chan<- error) {
	err := cmd.Wait()

	r.mu.Lock()
	sameProcess := r.process == cmd
	svc := r.service
	if sameProcess {
		r.service = nil
		r.process = nil
		r.waitDone = nil
	}
	r.mu.Unlock()

	if sameProcess && svc != nil {
		_ = svc.Close()
	}

	done <- err
	close(done)
}

func (r *LightpandaBinaryRuntime) binaryPath(ctx context.Context) (string, error) {
	if r == nil || r.binaryManager == nil {
		return "", ErrBrowserNotAvailable
	}
	if readyPath, ok, err := r.binaryManager.ReadyPath(); err != nil {
		return "", err
	} else if ok {
		return readyPath, nil
	}
	if r.config == nil || !r.config.LightpandaAutoDownload() {
		return "", fmt.Errorf("%w: lightpanda binary is not ready and auto_download is disabled", ErrBrowserNotAvailable)
	}

	downloadCtx := ctx
	if downloadCtx == nil {
		downloadCtx = context.Background()
	}
	if _, ok := downloadCtx.Deadline(); !ok {
		var cancel context.CancelFunc
		downloadCtx, cancel = context.WithTimeout(downloadCtx, r.config.LightpandaDownloadTimeout())
		defer cancel()
	}
	return r.binaryManager.Ensure(downloadCtx)
}

func buildLightpandaServeArgs(host string, port int, extra []string) []string {
	args := []string{
		"serve",
		"--host", strings.TrimSpace(host),
		"--port", strconv.Itoa(port),
	}
	if len(extra) == 0 {
		return args
	}
	return append(args, append([]string(nil), extra...)...)
}

func reserveLoopbackPort(host string) (int, error) {
	listener, err := net.Listen("tcp", net.JoinHostPort(strings.TrimSpace(host), "0"))
	if err != nil {
		return 0, err
	}
	defer listener.Close()

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok || addr == nil || addr.Port <= 0 {
		return 0, fmt.Errorf("allocate lightpanda port: invalid listener address")
	}
	return addr.Port, nil
}

func waitForLightpandaCDP(ctx context.Context, cdpURL string, exited <-chan error) error {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	var lastErr error
	for {
		probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err := ProbeCDPURL(probeCtx, cdpURL)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err

		select {
		case err, ok := <-exited:
			if ok && err != nil {
				return fmt.Errorf("lightpanda exited before ready: %w", err)
			}
			return fmt.Errorf("lightpanda exited before ready")
		case <-ctx.Done():
			if lastErr != nil {
				return fmt.Errorf("%w: %v", ctx.Err(), lastErr)
			}
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func formatLightpandaRuntimeLogs(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) > 600 {
		trimmed = strings.TrimSpace(trimmed[len(trimmed)-600:])
	}
	return ": " + trimmed
}

type synchronizedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *synchronizedBuffer) Write(p []byte) (int, error) {
	if b == nil {
		return len(p), nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *synchronizedBuffer) String() string {
	if b == nil {
		return ""
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.TrimSpace(b.buf.String())
}

var _ io.Writer = (*synchronizedBuffer)(nil)
