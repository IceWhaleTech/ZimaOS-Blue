package wasm

import (
	"context"
	"io"

	"github.com/danlock/gogosseract/internal/gen"
	"github.com/danlock/pkg/errors"
	embind "github.com/jerbob92/wazero-emscripten-embind"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

type CompileConfig struct {
	// Stderr and Stdout enable redirection of any logs. If left nil they point at os.Stderr and os.Stdout. Turn off by setting them to io.Discard
	Stderr, Stdout io.Writer
	// Modified for ZimaOS-Blue: the Tesseract runtime is supplied externally
	// so the main binary no longer embeds the WASM blob.
	WASMBytes []byte
}

func CompileTesseract(ctx context.Context, waRT wazero.Runtime, embEng embind.Engine, cfg CompileConfig) (api.Module, error) {
	if len(cfg.WASMBytes) == 0 {
		return nil, errors.Errorf("CompileConfig.WASMBytes is required")
	}
	tessCompiled, err := waRT.CompileModule(ctx, cfg.WASMBytes)
	if err != nil {
		return nil, errors.Errorf("waRT.CompileModule %w", err)
	}

	if err := BuildImports(ctx, waRT, embEng, tessCompiled); err != nil {
		return nil, errors.Wrap(err)
	}
	tessMod, err := waRT.InstantiateModule(ctx, tessCompiled, wazero.NewModuleConfig().
		WithStderr(cfg.Stderr).
		WithStdout(cfg.Stdout).
		WithStartFunctions("_initialize"))
	if err != nil {
		return nil, errors.Errorf("waRT.InstantiateModule %w", err)
	}

	return tessMod, errors.Wrap(gen.Attach(embEng))
}
