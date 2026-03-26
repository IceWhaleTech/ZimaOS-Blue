package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

const dockerLightpandaStartupTimeout = 45 * time.Second

type lightpandaRuntimeSelection struct {
	Factory         backendFactory
	Runtime         string
	BinaryReadyPath string
	DockerImage     string
	Notes           []string
}

func resolveLightpandaFactory(ctx context.Context, cfg *browser.Config, mode string, dockerImage string) (lightpandaRuntimeSelection, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "auto"
	}
	switch mode {
	case "auto":
		service := browser.NewLightpandaService(cfg)
		if service.BinaryAvailable() {
			binaryPath, err := service.WarmBinary(ctx)
			if err == nil && strings.TrimSpace(binaryPath) != "" {
				return lightpandaRuntimeSelection{
					Factory:         binaryFactory(cfg),
					Runtime:         "native_binary",
					BinaryReadyPath: binaryPath,
					Notes: []string{
						"lightpanda_binary used a locally ready upstream Lightpanda native binary.",
					},
				}, nil
			}
		}
		return lightpandaRuntimeSelection{
			Factory:     dockerFactory(cfg, dockerImage),
			Runtime:     "docker_nightly",
			DockerImage: strings.TrimSpace(dockerImage),
			Notes: []string{
				fmt.Sprintf("lightpanda_binary fell back to the official Docker image %s because no ready native binary was available locally.", strings.TrimSpace(dockerImage)),
			},
		}, nil
	case "binary":
		service := browser.NewLightpandaService(cfg)
		binaryPath, err := service.WarmBinary(ctx)
		if err != nil {
			return lightpandaRuntimeSelection{}, err
		}
		if strings.TrimSpace(binaryPath) == "" {
			return lightpandaRuntimeSelection{}, errors.New("lightpanda native binary is not ready")
		}
		return lightpandaRuntimeSelection{
			Factory:         binaryFactory(cfg),
			Runtime:         "native_binary",
			BinaryReadyPath: binaryPath,
			Notes: []string{
				"lightpanda_binary used a locally ready upstream Lightpanda native binary.",
			},
		}, nil
	case "docker":
		dockerImage = strings.TrimSpace(dockerImage)
		if dockerImage == "" {
			return lightpandaRuntimeSelection{}, errors.New("lightpanda docker image is required when lightpanda-runtime=docker")
		}
		return lightpandaRuntimeSelection{
			Factory:     dockerFactory(cfg, dockerImage),
			Runtime:     "docker_nightly",
			DockerImage: dockerImage,
			Notes: []string{
				fmt.Sprintf("lightpanda_binary used the official Docker image %s and attached through Blue's relay-mode browser-lite backend.", dockerImage),
			},
		}, nil
	default:
		return lightpandaRuntimeSelection{}, fmt.Errorf("unsupported lightpanda runtime mode %q", mode)
	}
}

func dockerFactory(cfg *browser.Config, image string) backendFactory {
	return func(context.Context) (tools.BrowserBackend, func(), error) {
		runtime := newDockerLightpandaRuntime(cfg, image)
		backend := tools.NewLightpandaBinaryBrowserBackend(runtime)
		cleanup := func() {
			_ = runtime.Stop(context.Background())
		}
		return backend, cleanup, nil
	}
}

type dockerLightpandaRuntime struct {
	config *browser.Config
	image  string

	mu            sync.Mutex
	service       *browser.RodService
	containerName string
}

func newDockerLightpandaRuntime(cfg *browser.Config, image string) *dockerLightpandaRuntime {
	return &dockerLightpandaRuntime{
		config: cfg.Clone(),
		image:  strings.TrimSpace(image),
	}
}

func (r *dockerLightpandaRuntime) PeekService() *browser.RodService {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.service
}

func (r *dockerLightpandaRuntime) Start(ctx context.Context) (*browser.RodService, error) {
	if r == nil {
		return nil, browser.ErrBrowserNotAvailable
	}
	if ctx == nil {
		ctx = context.Background()
	}

	r.mu.Lock()
	if r.service != nil {
		svc := r.service
		r.mu.Unlock()
		return svc, nil
	}
	r.mu.Unlock()

	port, err := reserveLoopbackPort("127.0.0.1")
	if err != nil {
		return nil, err
	}
	containerName := fmt.Sprintf("browserbench-lightpanda-%d", time.Now().UnixNano())
	if err := runDockerContainer(ctx, containerName, r.image, port); err != nil {
		return nil, err
	}

	cdpURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	startupCtx := ctx
	if _, ok := startupCtx.Deadline(); !ok {
		var cancel context.CancelFunc
		startupCtx, cancel = context.WithTimeout(startupCtx, dockerLightpandaStartupTimeout)
		defer cancel()
	}
	if err := waitForCDP(startupCtx, cdpURL); err != nil {
		_ = stopDockerContainer(context.Background(), containerName)
		return nil, fmt.Errorf("wait for docker lightpanda cdp: %w%s", err, readDockerLogs(containerName))
	}

	serviceCfg := r.config.CloneForDriver("relay")
	serviceCfg.CDPURL = cdpURL
	serviceCfg.RelayEnabled = false
	service, err := browser.NewService(serviceCfg)
	if err != nil {
		_ = stopDockerContainer(context.Background(), containerName)
		return nil, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.service != nil {
		_ = service.Close()
		_ = stopDockerContainer(context.Background(), containerName)
		return r.service, nil
	}
	r.service = service
	r.containerName = containerName
	return r.service, nil
}

func (r *dockerLightpandaRuntime) Stop(ctx context.Context) error {
	if r == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	r.mu.Lock()
	service := r.service
	containerName := r.containerName
	r.service = nil
	r.containerName = ""
	r.mu.Unlock()

	var firstErr error
	if service != nil {
		if err := service.Close(); err != nil {
			firstErr = err
		}
	}
	if strings.TrimSpace(containerName) != "" {
		if err := stopDockerContainer(ctx, containerName); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func reserveLoopbackPort(host string) (int, error) {
	listener, err := net.Listen("tcp", net.JoinHostPort(strings.TrimSpace(host), "0"))
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok || addr == nil || addr.Port <= 0 {
		return 0, errors.New("allocate docker lightpanda port: invalid listener address")
	}
	return addr.Port, nil
}

func runDockerContainer(ctx context.Context, containerName, image string, port int) error {
	args := []string{
		"run",
		"--rm",
		"-d",
		"--name", containerName,
		"-p", fmt.Sprintf("127.0.0.1:%d:9222", port),
		image,
	}
	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker run %s: %w%s", image, err, formatCommandOutput(output))
	}
	return nil
}

func stopDockerContainer(ctx context.Context, containerName string) error {
	stopCtx := ctx
	if stopCtx == nil {
		stopCtx = context.Background()
	}
	if _, ok := stopCtx.Deadline(); !ok {
		var cancel context.CancelFunc
		stopCtx, cancel = context.WithTimeout(stopCtx, 15*time.Second)
		defer cancel()
	}
	cmd := exec.CommandContext(stopCtx, "docker", "rm", "-f", containerName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.ToLower(strings.TrimSpace(string(output)))
		if strings.Contains(trimmed, "no such container") {
			return nil
		}
		return fmt.Errorf("docker rm -f %s: %w%s", containerName, err, formatCommandOutput(output))
	}
	return nil
}

func readDockerLogs(containerName string) string {
	if strings.TrimSpace(containerName) == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "logs", containerName)
	output, err := cmd.CombinedOutput()
	if err != nil || len(bytes.TrimSpace(output)) == 0 {
		return ""
	}
	return "\ncontainer logs:\n" + strings.TrimSpace(string(output))
}

func waitForCDP(ctx context.Context, cdpURL string) error {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	var lastErr error
	for {
		probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err := browser.ProbeCDPURL(probeCtx, cdpURL)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err

		select {
		case <-ctx.Done():
			if lastErr != nil {
				return fmt.Errorf("%w (last error: %v)", ctx.Err(), lastErr)
			}
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func formatCommandOutput(output []byte) string {
	if len(bytes.TrimSpace(output)) == 0 {
		return ""
	}
	return ": " + strings.TrimSpace(string(output))
}
