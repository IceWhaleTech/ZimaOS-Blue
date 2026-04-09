package bootstrap

import (
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

// NewRuntimeExecConfig returns the shared exec runtime defaults used by Blue
// entrypoints. CLI shells such as `blue` and embedded surfaces such as future
// bluelib exec entrypoints should source host-exec defaults from here so the
// behavior stays aligned with the runtime bundle.
func NewRuntimeExecConfig(dataDir string, allowedDirs []string) tools.ExecConfig {
	cfg := tools.DefaultExecConfig()
	cfg.DataDir = strings.TrimSpace(dataDir)
	cfg.AllowedDirs = append([]string(nil), allowedDirs...)
	return cfg
}
