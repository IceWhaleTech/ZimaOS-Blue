package bootstrap

import (
	"fmt"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxybridge"
)

var errRuntimeLLMUnavailable = fmt.Errorf("runtime llm backend not configured")

func newProxyBridgeProvider(bridge *proxybridge.Bridge, pool *providerpool.Pool) *proxyBridgeProvider {
	return &proxyBridgeProvider{
		bridge:       bridge,
		providerPool: pool,
	}
}
