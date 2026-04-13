//go:build !darwin && !windows

package a11y

func DefaultHostBackend(string) Backend {
	return nil
}
