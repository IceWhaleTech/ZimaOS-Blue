package ngrok

import (
	"context"
	"testing"
	"time"
)

func TestTunnelStatus_Initial(t *testing.T) {
	status := TunnelStatus{}

	if status.Active {
		t.Error("Initial Active should be false")
	}

	if status.URL != "" {
		t.Error("Initial URL should be empty")
	}
}

func TestTunnelManager_NewTunnelManager(t *testing.T) {
	tm := NewTunnelManager()

	if tm == nil {
		t.Fatal("NewTunnelManager() returned nil")
	}
}

func TestTunnelManager_IsRunning_Initial(t *testing.T) {
	tm := NewTunnelManager()

	if tm.IsRunning() {
		t.Error("IsRunning() should be false initially")
	}
}

func TestTunnelManager_GetStatus_NotRunning(t *testing.T) {
	tm := NewTunnelManager()

	status := tm.GetStatus()

	if status.Active {
		t.Error("Status.Active should be false when not running")
	}

	if status.URL != "" {
		t.Error("Status.URL should be empty when not running")
	}
}

func TestTunnelManager_Start_NgrokNotInstalled(t *testing.T) {
	tm := NewTunnelManager()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := tm.Start(ctx, 23456, "")
	if err == nil {
		t.Error("Start() should return error when ngrok not installed")
	}

	if err != ErrNgrokNotInstalled {
		t.Errorf("Start() error = %v, want ErrNgrokNotInstalled", err)
	}
}

func TestTunnelManager_Stop_NotRunning(t *testing.T) {
	tm := NewTunnelManager()

	// Stop when not running should not error
	err := tm.Stop()
	if err != nil {
		t.Errorf("Stop() when not running should not error, got: %v", err)
	}
}

func TestParseNgrokURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		hasError bool
	}{
		{
			name:     "valid JSON log with URL",
			input:    `{"lvl":"info","msg":"started tunnel","url":"https://abc123.ngrok-free.app"}`,
			expected: "https://abc123.ngrok-free.app",
			hasError: false,
		},
		{
			name:     "JSON log without URL",
			input:    `{"lvl":"info","msg":"some other message"}`,
			expected: "",
			hasError: true,
		},
		{
			name:     "invalid JSON",
			input:    `not json`,
			expected: "",
			hasError: true,
		},
		{
			name:     "empty input",
			input:    "",
			expected: "",
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, err := parseNgrokURL(tt.input)

			if tt.hasError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if url != tt.expected {
					t.Errorf("URL = %s, want %s", url, tt.expected)
				}
			}
		})
	}
}

func TestCalculateExpiresAt(t *testing.T) {
	startTime := time.Now()
	expiresAt := calculateExpiresAt(startTime)

	// Should be approximately 8 hours from start
	expectedDuration := 8 * time.Hour
	actualDuration := expiresAt.Sub(startTime)

	// Allow 1 second tolerance
	if actualDuration < expectedDuration-time.Second || actualDuration > expectedDuration+time.Second {
		t.Errorf("ExpiresAt duration = %v, want ~%v", actualDuration, expectedDuration)
	}
}

func TestCalculateRemainingTime(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		minRemain time.Duration
		maxRemain time.Duration
	}{
		{
			name:      "8 hours remaining",
			expiresAt: time.Now().Add(8 * time.Hour),
			minRemain: 7*time.Hour + 59*time.Minute,
			maxRemain: 8*time.Hour + 1*time.Minute,
		},
		{
			name:      "1 hour remaining",
			expiresAt: time.Now().Add(1 * time.Hour),
			minRemain: 59 * time.Minute,
			maxRemain: 61 * time.Minute,
		},
		{
			name:      "expired",
			expiresAt: time.Now().Add(-1 * time.Hour),
			minRemain: -2 * time.Hour,
			maxRemain: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			remaining := calculateRemainingTime(tt.expiresAt)

			if remaining < tt.minRemain || remaining > tt.maxRemain {
				t.Errorf("Remaining = %v, want between %v and %v", remaining, tt.minRemain, tt.maxRemain)
			}
		})
	}
}

func TestFormatRemainingTime(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{8 * time.Hour, "8h 0m"},
		{7*time.Hour + 30*time.Minute, "7h 30m"},
		{1 * time.Hour, "1h 0m"},
		{30 * time.Minute, "30m"},
		{5 * time.Minute, "5m"},
		{0, "0m"},
		{-1 * time.Hour, "expired"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatRemainingTime(tt.duration)
			if result != tt.expected {
				t.Errorf("formatRemainingTime(%v) = %s, want %s", tt.duration, result, tt.expected)
			}
		})
	}
}
