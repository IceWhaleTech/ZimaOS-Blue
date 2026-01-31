// +build !windows

package ngrok

// AddFirewallException is a no-op on non-Windows platforms.
func AddFirewallException(ngrokPath string) error {
	return nil
}

// RemoveFirewallException is a no-op on non-Windows platforms.
func RemoveFirewallException() error {
	return nil
}

// CheckFirewallException always returns true on non-Windows platforms.
func CheckFirewallException() bool {
	return true
}
