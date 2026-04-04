// Package embedded provides go:embed access to provider_catalog.json.
// Build-time copy: Makefile should sync docs/provider_catalog.json → here.
package embedded

import "embed"

//go:embed provider_catalog.json
var ProviderCatalogFS embed.FS
