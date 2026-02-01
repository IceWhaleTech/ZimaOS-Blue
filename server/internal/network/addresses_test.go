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
