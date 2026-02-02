package network

import (
	"testing"
)

func TestAddressDetector_GetAddresses(t *testing.T) {
	detector := NewAddressDetector(23456)

	addresses, err := detector.GetAddresses()
	if err != nil {
		t.Logf("Warning: GetAddresses returned error: %v", err)
	}

	// Local address should always be set
	if addresses.Local == "" {
		t.Error("Local address should not be empty")
	}

	expectedLocal := "http://localhost:23456"
	if addresses.Local != expectedLocal {
		t.Errorf("Expected local address %s, got %s", expectedLocal, addresses.Local)
	}

	// Port should be set
	if addresses.Port != 23456 {
		t.Errorf("Expected port 23456, got %d", addresses.Port)
	}

	// Preferred should be set
	if addresses.Preferred == "" {
		t.Error("Preferred address should not be empty")
	}

	t.Logf("Local: %s", addresses.Local)
	t.Logf("Hostname: %s", addresses.Hostname)
	t.Logf("Preferred: %s", addresses.Preferred)
	t.Logf("LAN interfaces: %d", len(addresses.LAN))
	for _, iface := range addresses.LAN {
		t.Logf("  - %s (%s): %s", iface.Name, iface.Type, iface.Address)
	}
}

func TestAddressDetector_GetPreferredAddress(t *testing.T) {
	detector := NewAddressDetector(3000)

	preferred, err := detector.GetPreferredAddress()
	if err != nil {
		t.Logf("Warning: GetPreferredAddress returned error: %v", err)
	}

	if preferred == "" {
		t.Error("Preferred address should not be empty")
	}

	t.Logf("Preferred address: %s", preferred)
}

func TestDetectInterfaceType(t *testing.T) {
	tests := []struct {
		name     string
		expected InterfaceType
	}{
		// macOS
		{"en0", InterfaceTypeWiFi},
		{"en1", InterfaceTypeEthernet},

		// Linux
		{"wlan0", InterfaceTypeWiFi},
		{"wlp3s0", InterfaceTypeWiFi},
		{"eth0", InterfaceTypeEthernet},
		{"enp0s3", InterfaceTypeEthernet},
		{"ens33", InterfaceTypeEthernet},

		// Virtual
		{"docker0", InterfaceTypeVirtual},
		{"veth123", InterfaceTypeVirtual},
		{"br-abc123", InterfaceTypeVirtual},
		{"virbr0", InterfaceTypeVirtual},

		// Loopback
		{"lo", InterfaceTypeLoopback},
		{"lo0", InterfaceTypeLoopback},

		// Unknown
		{"unknown123", InterfaceTypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectInterfaceType(tt.name)
			if result != tt.expected {
				t.Errorf("detectInterfaceType(%s) = %s, want %s", tt.name, result, tt.expected)
			}
		})
	}
}

func TestInterfaceTypePriority(t *testing.T) {
	// WiFi should have highest priority (lowest number)
	if interfaceTypePriority(InterfaceTypeWiFi) >= interfaceTypePriority(InterfaceTypeEthernet) {
		t.Error("WiFi should have higher priority than Ethernet")
	}

	// Ethernet should have higher priority than Unknown
	if interfaceTypePriority(InterfaceTypeEthernet) >= interfaceTypePriority(InterfaceTypeUnknown) {
		t.Error("Ethernet should have higher priority than Unknown")
	}

	// Virtual should have lower priority than Unknown
	if interfaceTypePriority(InterfaceTypeUnknown) >= interfaceTypePriority(InterfaceTypeVirtual) {
		t.Error("Unknown should have higher priority than Virtual")
	}

	// Loopback should have lowest priority
	if interfaceTypePriority(InterfaceTypeVirtual) >= interfaceTypePriority(InterfaceTypeLoopback) {
		t.Error("Virtual should have higher priority than Loopback")
	}
}

func TestAddressDetector_DynamicPort(t *testing.T) {
	// Test that AddressDetector uses dynamic port from security.GetServerPort()
	// when created with port 0

	// Create detector with port 0 (simulating early initialization)
	detector := NewAddressDetector(0)

	// Set the dynamic port (simulating server startup)
	SetDynamicPort(23456)

	// Get addresses - should use the dynamic port, not 0
	addresses, err := detector.GetAddresses()
	if err != nil {
		t.Logf("Warning: GetAddresses returned error: %v", err)
	}

	// Verify port is not 0
	if addresses.Port == 0 {
		t.Error("Port should not be 0 after SetDynamicPort was called")
	}

	// Verify port is the dynamic port
	if addresses.Port != 23456 {
		t.Errorf("Expected port 23456, got %d", addresses.Port)
	}

	// Verify local address uses dynamic port
	expectedLocal := "http://localhost:23456"
	if addresses.Local != expectedLocal {
		t.Errorf("Expected local address %s, got %s", expectedLocal, addresses.Local)
	}

	// Verify LAN addresses use dynamic port (not :0)
	for _, iface := range addresses.LAN {
		if iface.Address == "" {
			continue
		}
		// Check that address doesn't contain :0
		if len(iface.Address) > 2 && iface.Address[len(iface.Address)-2:] == ":0" {
			t.Errorf("LAN address should not end with :0, got %s", iface.Address)
		}
	}

	t.Logf("Local: %s", addresses.Local)
	t.Logf("Port: %d", addresses.Port)
	for _, iface := range addresses.LAN {
		t.Logf("  - %s: %s", iface.Name, iface.Address)
	}

	// Reset for other tests
	SetDynamicPort(0)
}
