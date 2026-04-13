//go:build !cgo

package harness

func isSQLiteBusyDriverError(error) bool {
	return false
}
