//go:build cgo && !(darwin || linux || freebsd)

package smallmodel

import "fmt"

func ensureLlamaCppCGOBackend() error {
	return fmt.Errorf("llama.cpp cgo backend probe is not implemented for this platform")
}
