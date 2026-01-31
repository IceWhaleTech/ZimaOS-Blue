package ngrok

import (
	"strings"
	"testing"
)

func TestGenerateQRCode(t *testing.T) {
	url := "https://abc123.ngrok-free.app"

	qrData, err := GenerateQRCode(url, 200)
	if err != nil {
		t.Fatalf("GenerateQRCode() error: %v", err)
	}

	// Should return base64 PNG data
	if !strings.HasPrefix(qrData, "data:image/png;base64,") {
		t.Error("QR code should be base64 PNG data URL")
	}

	// Should have actual data after prefix
	base64Part := strings.TrimPrefix(qrData, "data:image/png;base64,")
	if len(base64Part) < 100 {
		t.Error("QR code data seems too short")
	}
}

func TestGenerateQRCode_EmptyURL(t *testing.T) {
	_, err := GenerateQRCode("", 200)
	if err == nil {
		t.Error("GenerateQRCode() should return error for empty URL")
	}
}

func TestGenerateQRCode_InvalidSize(t *testing.T) {
	url := "https://abc123.ngrok-free.app"

	// Size too small
	_, err := GenerateQRCode(url, 10)
	if err == nil {
		t.Error("GenerateQRCode() should return error for size < 50")
	}

	// Size too large
	_, err = GenerateQRCode(url, 2000)
	if err == nil {
		t.Error("GenerateQRCode() should return error for size > 1000")
	}
}

func TestGenerateQRCode_DefaultSize(t *testing.T) {
	url := "https://abc123.ngrok-free.app"

	// Size 0 should use default
	qrData, err := GenerateQRCode(url, 0)
	if err != nil {
		t.Fatalf("GenerateQRCode() error: %v", err)
	}

	if !strings.HasPrefix(qrData, "data:image/png;base64,") {
		t.Error("QR code should be base64 PNG data URL")
	}
}

func TestGenerateQRCodeBytes(t *testing.T) {
	url := "https://abc123.ngrok-free.app"

	pngData, err := GenerateQRCodeBytes(url, 200)
	if err != nil {
		t.Fatalf("GenerateQRCodeBytes() error: %v", err)
	}

	// Check PNG magic bytes
	if len(pngData) < 8 {
		t.Fatal("PNG data too short")
	}

	// PNG files start with these bytes
	pngMagic := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	for i, b := range pngMagic {
		if pngData[i] != b {
			t.Errorf("PNG magic byte %d: got %x, want %x", i, pngData[i], b)
		}
	}
}
