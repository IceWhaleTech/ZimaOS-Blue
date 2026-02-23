// Package tunnel provides a local replacement for trycloudflared.CreateCloudflareTunnel
// that fixes the child goroutine panic on context cancellation.
//
// The upstream library (github.com/wizzard0/trycloudflared) spawns a goroutine
// that calls supervisor.StartTunnelDaemon and panics if it returns an error.
// When the context is cancelled (e.g. tunnel stop), StartTunnelDaemon returns
// "context canceled" and the goroutine panics, crashing the process.
//
// This file is a copy of the upstream CreateCloudflareTunnel with the panic
// replaced by an error channel, so the caller can handle the error gracefully.
package tunnel

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"runtime"
	"time"

	"encoding/json"
	"io"
	"net/http"

	"github.com/cloudflare/cloudflared/config"
	"github.com/cloudflare/cloudflared/connection"
	"github.com/cloudflare/cloudflared/edgediscovery"
	"github.com/cloudflare/cloudflared/edgediscovery/allregions"
	"github.com/cloudflare/cloudflared/features"
	"github.com/cloudflare/cloudflared/ingress"
	"github.com/cloudflare/cloudflared/logger"
	"github.com/cloudflare/cloudflared/metrics"
	"github.com/cloudflare/cloudflared/orchestration"
	"github.com/cloudflare/cloudflared/signal"
	"github.com/cloudflare/cloudflared/supervisor"
	"github.com/cloudflare/cloudflared/tlsconfig"
	"github.com/cloudflare/cloudflared/tunnelrpc/pogs"
	"github.com/google/uuid"
	"github.com/urfave/cli/v2"
	"github.com/wizzard0/trycloudflared"
)

// requestTunnel creates a quick tunnel via the Cloudflare API.
// Copied from trycloudflared.createTunnel (unexported).
func requestTunnel(clientID uuid.UUID) (*connection.TunnelProperties, error) {
	client := http.Client{
		Transport: &http.Transport{
			TLSHandshakeTimeout:   30 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
		},
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.trycloudflare.com/tunnel", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build tunnel request: %w", err)
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Agent", "cloudflared/embedded-wizzard0-trycloudflared")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to request tunnel: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read creation response: %w", err)
	}

	var parsed struct {
		Success bool `json:"success"`
		Result  struct {
			ID         string `json:"id"`
			Hostname   string `json:"hostname"`
			AccountTag string `json:"account_tag"`
			Secret     []byte `json:"secret"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("failed to unmarshal credentials: %w", err)
	}

	tunnelID, err := uuid.Parse(parsed.Result.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse tunnel ID: %w", err)
	}

	return &connection.TunnelProperties{
		Credentials: connection.Credentials{
			AccountTag:   parsed.Result.AccountTag,
			TunnelSecret: parsed.Result.Secret,
			TunnelID:     tunnelID,
		},
		QuickTunnelUrl: parsed.Result.Hostname,
		Client: pogs.ClientInfo{
			ClientID: clientID[:],
			Features: []string{},
			Version:  trycloudflared.Version,
			Arch:     runtime.GOOS + "_" + runtime.GOARCH,
		},
	}, nil
}

// tunnelResult holds the result of createCloudflareTunnelSafe.
type tunnelResult struct {
	URL    string
	DaemonErrCh <-chan error // receives daemon exit error (nil on clean shutdown)
}

// createCloudflareTunnelSafe is a panic-free replacement for
// trycloudflared.CreateCloudflareTunnel. Instead of panicking when the
// tunnel daemon exits with an error, it sends the error on DaemonErrCh.
func createCloudflareTunnelSafe(ctx context.Context, port int) (*tunnelResult, error) {
	metrics.RegisterBuildInfo(trycloudflared.BuildType, trycloudflared.BuildTime, trycloudflared.Version)

	clientID, err := uuid.NewRandom()
	if err != nil {
		return nil, fmt.Errorf("can't generate connector UUID: %w", err)
	}

	logTransport := logger.Create(logger.CreateConfig("", false, "", ""))
	observer := connection.NewObserver(logTransport, logTransport)

	ing, err := ingress.ParseIngress(&config.Configuration{
		Ingress: []config.UnvalidatedIngressRule{
			{Service: fmt.Sprintf("http://localhost:%d", port)},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("can't parse ingress: %w", err)
	}

	orchestrator, err := orchestration.NewOrchestrator(
		ctx,
		&orchestration.Config{
			Ingress:            &ing,
			WarpRouting:        ingress.NewWarpRoutingConfig(&config.WarpRoutingConfig{}),
			ConfigurationFlags: map[string]string{},
			WriteTimeout:       0,
		},
		[]pogs.Tag{},
		[]ingress.Rule{},
		logTransport,
	)
	if err != nil {
		return nil, fmt.Errorf("can't create orchestrator: %w", err)
	}

	connectedSignal := signal.New(make(chan struct{}))
	reconnectCh := make(chan supervisor.ReconnectSignal, 4)

	protocolSelector, err := connection.NewProtocolSelector(
		connection.HTTP2.String(),
		"random value",
		false,
		false,
		edgediscovery.ProtocolPercentage,
		connection.ResolveTTL,
		logTransport,
	)
	if err != nil {
		return nil, fmt.Errorf("unable to create protocol selector: %w", err)
	}

	edgeTLSConfigs := make(map[connection.Protocol]*tls.Config, len(connection.ProtocolList))
	for _, p := range connection.ProtocolList {
		tlsSettings := p.TLSSettings()
		if tlsSettings == nil {
			return nil, fmt.Errorf("%s has unknown TLS settings", p)
		}
		edgeTLSConfig, err := tlsconfig.CreateTunnelConfig(
			cli.NewContext(cli.NewApp(), &flag.FlagSet{}, nil),
			tlsSettings.ServerName,
		)
		if err != nil {
			return nil, fmt.Errorf("unable to create TLS config to connect with edge: %w", err)
		}
		if len(tlsSettings.NextProtos) > 0 {
			edgeTLSConfig.NextProtos = tlsSettings.NextProtos
		}
		edgeTLSConfigs[p] = edgeTLSConfig
	}

	tunnel, err := requestTunnel(clientID)
	if err != nil {
		return nil, err
	}

	tunnelConfig := &supervisor.TunnelConfig{
		GracePeriod:                         30,
		ReplaceExisting:                     false,
		OSArch:                              runtime.GOOS + "_" + runtime.GOARCH,
		ClientID:                            clientID.String(),
		EdgeAddrs:                           []string{},
		Region:                              "",
		EdgeIPVersion:                       allregions.Auto,
		EdgeBindAddr:                        nil,
		HAConnections:                       2,
		IsAutoupdated:                       false,
		LBPool:                              "",
		Tags:                                []pogs.Tag{},
		Log:                                 logTransport,
		LogTransport:                        logTransport,
		Observer:                            observer,
		ReportedVersion:                     "embedded-go-test",
		Retries:                             5,
		RunFromTerminal:                     true,
		NamedTunnel:                         tunnel,
		ProtocolSelector:                    protocolSelector,
		EdgeTLSConfigs:                      edgeTLSConfigs,
		FeatureSelector:                     &features.FeatureSelector{},
		MaxEdgeAddrRetries:                  8,
		RPCTimeout:                          5 * time.Second,
		WriteStreamTimeout:                  0,
		DisableQUICPathMTUDiscovery:         false,
		QUICConnectionLevelFlowControlLimit: 30 * (1 << 20),
		QUICStreamLevelFlowControlLimit:     6 * (1 << 20),
		ICMPRouterServer:                    nil,
	}

	shutdown := make(chan struct{})
	daemonErrCh := make(chan error, 1)

	go func() {
		// Recover any panic from StartTunnelDaemon or its internals.
		defer func() {
			if r := recover(); r != nil {
				daemonErrCh <- fmt.Errorf("tunnel daemon panic: %v", r)
			}
		}()
		startErr := supervisor.StartTunnelDaemon(ctx, tunnelConfig, orchestrator, connectedSignal, reconnectCh, shutdown)
		daemonErrCh <- startErr // nil on clean shutdown
	}()

	return &tunnelResult{
		URL:         "https://" + tunnel.QuickTunnelUrl,
		DaemonErrCh: daemonErrCh,
	}, nil
}
