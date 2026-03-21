package whatsapp

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/network"
	"go.uber.org/zap"
	"golang.org/x/crypto/hkdf"
)

const whatsappMediaMACLength = 10

var whatsappMediaHTTPClient = network.NewPooledHTTPClient(2 * time.Minute)

// DownloadMedia downloads media from a received message.
func (c *Channel) DownloadMedia(ctx context.Context, mediaURL string, mediaKey []byte, fileEncSHA256 []byte, fileSHA256 []byte, fileLength uint64, mediaType string) ([]byte, error) {
	c.mu.RLock()
	if !c.isLoggedIn {
		c.mu.RUnlock()
		return nil, fmt.Errorf("WhatsApp not logged in")
	}
	c.mu.RUnlock()

	if strings.TrimSpace(mediaURL) == "" {
		return nil, fmt.Errorf("WhatsApp media URL is empty")
	}

	c.logger.Debug("downloading WhatsApp media",
		zap.String("url", mediaURL),
		zap.String("type", mediaType),
		zap.Uint64("size", fileLength))

	encryptedData, err := fetchWhatsAppMedia(ctx, mediaURL)
	if err != nil {
		return nil, fmt.Errorf("download WhatsApp media: %w", err)
	}

	if matchesSHA256(encryptedData, fileSHA256) {
		if err := validateMediaLength(encryptedData, fileLength); err != nil {
			return nil, err
		}
		return encryptedData, nil
	}

	if len(mediaKey) == 0 {
		if len(fileSHA256) > 0 {
			return nil, fmt.Errorf("WhatsApp media key missing and downloaded payload does not match plaintext checksum")
		}
		if len(fileEncSHA256) > 0 && !matchesSHA256(encryptedData, fileEncSHA256) {
			return nil, fmt.Errorf("WhatsApp encrypted media checksum mismatch: got %s want %s", sha256Hex(encryptedData), hex.EncodeToString(fileEncSHA256))
		}
		return encryptedData, nil
	}

	if len(fileEncSHA256) > 0 && !matchesSHA256(encryptedData, fileEncSHA256) {
		return nil, fmt.Errorf("WhatsApp encrypted media checksum mismatch: got %s want %s", sha256Hex(encryptedData), hex.EncodeToString(fileEncSHA256))
	}

	decrypted, err := decryptWhatsAppMedia(encryptedData, mediaKey, mediaType)
	if err != nil {
		return nil, fmt.Errorf("decrypt WhatsApp media: %w", err)
	}
	if len(fileSHA256) > 0 && !matchesSHA256(decrypted, fileSHA256) {
		return nil, fmt.Errorf("WhatsApp media checksum mismatch: got %s want %s", sha256Hex(decrypted), hex.EncodeToString(fileSHA256))
	}
	if err := validateMediaLength(decrypted, fileLength); err != nil {
		return nil, err
	}
	return decrypted, nil
}

func fetchWhatsAppMedia(ctx context.Context, mediaURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mediaURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	resp, err := whatsappMediaHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	return data, nil
}

func decryptWhatsAppMedia(encryptedData, mediaKey []byte, mediaType string) ([]byte, error) {
	if len(mediaKey) == 0 {
		return nil, fmt.Errorf("missing media key")
	}
	if len(encryptedData) <= whatsappMediaMACLength {
		return nil, fmt.Errorf("encrypted payload too short")
	}

	iv, cipherKey, macKey, err := deriveWhatsAppMediaKeys(mediaKey, mediaType)
	if err != nil {
		return nil, err
	}

	ciphertext := encryptedData[:len(encryptedData)-whatsappMediaMACLength]
	mac := encryptedData[len(encryptedData)-whatsappMediaMACLength:]
	if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("invalid ciphertext size %d", len(ciphertext))
	}

	h := hmac.New(sha256.New, macKey)
	h.Write(iv)
	h.Write(ciphertext)
	expectedMAC := h.Sum(nil)[:whatsappMediaMACLength]
	if !hmac.Equal(mac, expectedMAC) {
		return nil, fmt.Errorf("media MAC mismatch")
	}

	block, err := aes.NewCipher(cipherKey)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	plaintext := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plaintext, ciphertext)
	plaintext, err = pkcs7Unpad(plaintext, aes.BlockSize)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}

func deriveWhatsAppMediaKeys(mediaKey []byte, mediaType string) ([]byte, []byte, []byte, error) {
	info, err := whatsappMediaHKDFInfo(mediaType)
	if err != nil {
		return nil, nil, nil, err
	}
	reader := hkdf.New(sha256.New, mediaKey, nil, []byte(info))
	derived := make([]byte, 112)
	if _, err := io.ReadFull(reader, derived); err != nil {
		return nil, nil, nil, fmt.Errorf("derive media keys: %w", err)
	}
	return derived[:16], derived[16:48], derived[48:80], nil
}

func whatsappMediaHKDFInfo(mediaType string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(mediaType))
	switch {
	case strings.Contains(normalized, "image"), strings.Contains(normalized, "sticker"), strings.Contains(normalized, "thumbnail"):
		return "WhatsApp Image Keys", nil
	case strings.Contains(normalized, "video"):
		return "WhatsApp Video Keys", nil
	case strings.Contains(normalized, "audio"), strings.Contains(normalized, "voice"), strings.Contains(normalized, "ptt"):
		return "WhatsApp Audio Keys", nil
	case strings.Contains(normalized, "document"), strings.Contains(normalized, "file"):
		return "WhatsApp Document Keys", nil
	default:
		return "", fmt.Errorf("unsupported WhatsApp media type %q", mediaType)
	}
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, fmt.Errorf("invalid PKCS#7 payload size %d", len(data))
	}
	padLen := int(data[len(data)-1])
	if padLen == 0 || padLen > blockSize || padLen > len(data) {
		return nil, fmt.Errorf("invalid PKCS#7 padding")
	}
	padding := bytes.Repeat([]byte{byte(padLen)}, padLen)
	if !bytes.Equal(data[len(data)-padLen:], padding) {
		return nil, fmt.Errorf("invalid PKCS#7 padding")
	}
	return data[:len(data)-padLen], nil
}

func validateMediaLength(data []byte, fileLength uint64) error {
	if fileLength == 0 {
		return nil
	}
	if uint64(len(data)) != fileLength {
		return fmt.Errorf("WhatsApp media length mismatch: got %d want %d", len(data), fileLength)
	}
	return nil
}

func matchesSHA256(data, expected []byte) bool {
	if len(expected) == 0 {
		return false
	}
	sum := sha256.Sum256(data)
	return hmac.Equal(sum[:], expected)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
