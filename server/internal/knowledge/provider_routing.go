package knowledge

import (
	"context"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func applyKnowledgeProviderRouting(ctx context.Context, providerID string) context.Context {
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		return ctx
	}
	ctx = proxy.WithPinnedProvider(ctx, providerID)
	ctx = tools.WithProviderID(ctx, providerID)
	return ctx
}
