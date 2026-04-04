// Package embedded provides go:embed access to provider_catalog.json.
// Regenerate with `make provider-catalog`.
package embedded

import "embed"

//go:embed provider_catalog.json
var ProviderCatalogFS embed.FS
