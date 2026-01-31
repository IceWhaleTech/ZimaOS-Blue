// +build windows

package ngrok

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

const (
	// Firewall rule constants
	NET_FW_PROFILE2_ALL    = 0x7FFFFFFF
	NET_FW_ACTION_ALLOW    = 1
	NET_FW_RULE_DIR_IN     = 1
	NET_FW_IP_PROTOCOL_TCP = 6
)

// AddFirewallException adds a Windows Firewall exception for Echo executable using COM API.
// This requires administrator privileges.
func AddFirewallException(ngrokPath string) error {
	// Get Echo executable path instead of ngrok path
	echoPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get Echo executable path: %w", err)
	}

	// Get absolute path
	absPath, err := filepath.Abs(echoPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Initialize COM
	err = ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
	if err != nil {
		return fmt.Errorf("failed to initialize COM: %w", err)
	}
	defer ole.CoUninitialize()

	// Create INetFwPolicy2 object
	unknown, err := oleutil.CreateObject("HNetCfg.FwPolicy2")
	if err != nil {
		return fmt.Errorf("failed to create FwPolicy2 object: %w", err)
	}
	defer unknown.Release()

	policy, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return fmt.Errorf("failed to query IDispatch interface: %w", err)
	}
	defer policy.Release()

	// Get Rules collection
	rulesRaw, err := oleutil.GetProperty(policy, "Rules")
	if err != nil {
		return fmt.Errorf("failed to get Rules property: %w", err)
	}
	rules := rulesRaw.ToIDispatch()
	defer rules.Release()

	// Rule name - changed to reflect Echo itself
	ruleName := "ZimaOS-Echo-Remote-Access"

	// Check if rule already exists
	if CheckFirewallException() {
		return nil // Rule already exists
	}

	// Create new rule
	ruleUnknown, err := oleutil.CreateObject("HNetCfg.FWRule")
	if err != nil {
		return fmt.Errorf("failed to create FWRule object: %w", err)
	}
	defer ruleUnknown.Release()

	rule, err := ruleUnknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return fmt.Errorf("failed to query IDispatch interface for rule: %w", err)
	}
	defer rule.Release()

	// Set rule properties
	_, err = oleutil.PutProperty(rule, "Name", ruleName)
	if err != nil {
		return fmt.Errorf("failed to set rule name: %w", err)
	}

	_, err = oleutil.PutProperty(rule, "Description", "Allow ZimaOS-Echo remote access via ngrok")
	if err != nil {
		return fmt.Errorf("failed to set rule description: %w", err)
	}

	_, err = oleutil.PutProperty(rule, "ApplicationName", absPath)
	if err != nil {
		return fmt.Errorf("failed to set application name: %w", err)
	}

	_, err = oleutil.PutProperty(rule, "Protocol", NET_FW_IP_PROTOCOL_TCP)
	if err != nil {
		return fmt.Errorf("failed to set protocol: %w", err)
	}

	_, err = oleutil.PutProperty(rule, "Direction", NET_FW_RULE_DIR_IN)
	if err != nil {
		return fmt.Errorf("failed to set direction: %w", err)
	}

	_, err = oleutil.PutProperty(rule, "Action", NET_FW_ACTION_ALLOW)
	if err != nil {
		return fmt.Errorf("failed to set action: %w", err)
	}

	_, err = oleutil.PutProperty(rule, "Enabled", true)
	if err != nil {
		return fmt.Errorf("failed to enable rule: %w", err)
	}

	_, err = oleutil.PutProperty(rule, "Profiles", NET_FW_PROFILE2_ALL)
	if err != nil {
		return fmt.Errorf("failed to set profiles: %w", err)
	}

	// Add rule to collection
	_, err = oleutil.CallMethod(rules, "Add", rule)
	if err != nil {
		return fmt.Errorf("failed to add rule (requires admin): %w", err)
	}

	return nil
}

// RemoveFirewallException removes the Windows Firewall exception for Echo using COM API.
func RemoveFirewallException() error {
	// Initialize COM
	err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
	if err != nil {
		return fmt.Errorf("failed to initialize COM: %w", err)
	}
	defer ole.CoUninitialize()

	// Create INetFwPolicy2 object
	unknown, err := oleutil.CreateObject("HNetCfg.FwPolicy2")
	if err != nil {
		return fmt.Errorf("failed to create FwPolicy2 object: %w", err)
	}
	defer unknown.Release()

	policy, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return fmt.Errorf("failed to query IDispatch interface: %w", err)
	}
	defer policy.Release()

	// Get Rules collection
	rulesRaw, err := oleutil.GetProperty(policy, "Rules")
	if err != nil {
		return fmt.Errorf("failed to get Rules property: %w", err)
	}
	rules := rulesRaw.ToIDispatch()
	defer rules.Release()

	// Rule name
	ruleName := "ZimaOS-Echo-Remote-Access"

	// Remove rule
	_, err = oleutil.CallMethod(rules, "Remove", ruleName)
	if err != nil {
		return fmt.Errorf("failed to remove rule: %w", err)
	}

	return nil
}

// CheckFirewallException checks if the firewall exception exists using COM API.
func CheckFirewallException() bool {
	// Initialize COM
	err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
	if err != nil {
		return false
	}
	defer ole.CoUninitialize()

	// Create INetFwPolicy2 object
	unknown, err := oleutil.CreateObject("HNetCfg.FwPolicy2")
	if err != nil {
		return false
	}
	defer unknown.Release()

	policy, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return false
	}
	defer policy.Release()

	// Get Rules collection
	rulesRaw, err := oleutil.GetProperty(policy, "Rules")
	if err != nil {
		return false
	}
	rules := rulesRaw.ToIDispatch()
	defer rules.Release()

	// Rule name
	ruleName := "ZimaOS-Echo-Remote-Access"

	// Try to get the rule by name
	ruleRaw, err := oleutil.CallMethod(rules, "Item", ruleName)
	if err != nil {
		return false
	}

	if ruleRaw.VT == ole.VT_DISPATCH && ruleRaw.Val != 0 {
		rule := ruleRaw.ToIDispatch()
		defer rule.Release()
		return true
	}

	return false
}
