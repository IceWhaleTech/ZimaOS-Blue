package ngrok

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image/png"

	"github.com/skip2/go-qrcode"
)

// GenerateQRCode generates a QR code for the given URL and returns it as a base64 data URL.
func GenerateQRCode(url string, size int) (string, error) {
	if url == "" {
		return "", errors.New("URL cannot be empty")
	}

	// Validate size
	if size < 50 && size != 0 {
		return "", errors.New("size must be at least 50 pixels")
	}
	if size > 1000 {
		return "", errors.New("size must be at most 1000 pixels")
	}

	// Default size
	if size == 0 {
		size = 200
	}

	// Generate QR code
	qr, err := qrcode.New(url, qrcode.Medium)
	if err != nil {
		return "", err
	}

	// Generate PNG
	pngData, err := qr.PNG(size)
	if err != nil {
		return "", err
	}

	// Encode as base64 data URL
	base64Data := base64.StdEncoding.EncodeToString(pngData)
	return "data:image/png;base64," + base64Data, nil
}

// GenerateQRCodeBytes generates a QR code and returns the raw PNG bytes.
func GenerateQRCodeBytes(url string, size int) ([]byte, error) {
	if url == "" {
		return nil, errors.New("URL cannot be empty")
	}

	// Validate size
	if size < 50 && size != 0 {
		return nil, errors.New("size must be at least 50 pixels")
	}
	if size > 1000 {
		return nil, errors.New("size must be at most 1000 pixels")
	}

	// Default size
	if size == 0 {
		size = 200
	}

	// Generate QR code
	qr, err := qrcode.New(url, qrcode.Medium)
	if err != nil {
		return nil, err
	}

	// Generate PNG
	return qr.PNG(size)
}

// GenerateQRCodeImage generates a QR code and returns it as a PNG image.
func GenerateQRCodeImage(url string, size int) (*bytes.Buffer, error) {
	if url == "" {
		return nil, errors.New("URL cannot be empty")
	}

	// Validate size
	if size < 50 && size != 0 {
		return nil, errors.New("size must be at least 50 pixels")
	}
	if size > 1000 {
		return nil, errors.New("size must be at most 1000 pixels")
	}

	// Default size
	if size == 0 {
		size = 200
	}

	// Generate QR code
	qr, err := qrcode.New(url, qrcode.Medium)
	if err != nil {
		return nil, err
	}

	// Generate image
	img := qr.Image(size)

	// Encode as PNG
	buf := new(bytes.Buffer)
	if err := png.Encode(buf, img); err != nil {
		return nil, err
	}

	return buf, nil
}
