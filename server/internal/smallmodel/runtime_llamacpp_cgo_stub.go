//go:build !cgo

package smallmodel

import "fmt"

func ensureLlamaCppCGOBackend() error {
	return fmt.Errorf("llama.cpp cgo backend requires CGO_ENABLED=1")
}
