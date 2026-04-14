package bootstrap

import "context"

type runtimeChannelTunnelSetup interface {
	EnsureTunnelURL(ctx context.Context) (string, error)
}
