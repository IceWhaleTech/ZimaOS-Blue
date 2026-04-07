package bootstrap

import (
	"context"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
)

const smallmodelProviderID = "smallmodel"

func normalizeLowerTrimmedProviderID(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizedPinnedProviderID(ctx context.Context) string {
	return normalizeLowerTrimmedProviderID(proxy.GetPinnedProvider(ctx))
}
