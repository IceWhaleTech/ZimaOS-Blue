package embedded

import "embed"

//go:embed packs/**
var PacksFS embed.FS
