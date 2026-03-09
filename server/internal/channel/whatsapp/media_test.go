package whatsapp

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

func TestChannel_DownloadMedia_DecryptsEncryptedPayload(t *testing.T) {
	plaintext := []byte("hello from whatsapp media")
	mediaKey := make([]byte, 32)
	if _, err := rand.Read(mediaKey); err != nil {
		t.Fatalf("rand.Read(mediaKey): %v", err)
	}
	ciphertext, encSHA, plainSHA := buildEncryptedWhatsAppMedia(t, plaintext, mediaKey, "image")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(ciphertext)
	}))
	defer server.Close()

	ch := New(DefaultConfig(), zap.NewNop())
	ch.mu.Lock()
	ch.isLoggedIn = true
	ch.mu.Unlock()

	data, err := ch.DownloadMedia(context.Background(), server.URL, mediaKey, encSHA, plainSHA, uint64(len(plaintext)), "image")
	if err != nil {
		t.Fatalf("DownloadMedia error = %v", err)
	}
	if string(data) != string(plaintext) {
		t.Fatalf("DownloadMedia payload = %q, want %q", string(data), string(plaintext))
	}
}

func TestChannel_DownloadMedia_ReturnsPlainPayloadWithoutMediaKey(t *testing.T) {
	plaintext := []byte("plain payload")
	plainSHA := sha256.Sum256(plaintext)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(plaintext)
	}))
	defer server.Close()

	ch := New(DefaultConfig(), zap.NewNop())
	ch.mu.Lock()
	ch.isLoggedIn = true
	ch.mu.Unlock()

	data, err := ch.DownloadMedia(context.Background(), server.URL, nil, nil, plainSHA[:], uint64(len(plaintext)), "document")
	if err != nil {
		t.Fatalf("DownloadMedia error = %v", err)
	}
	if string(data) != string(plaintext) {
		t.Fatalf("DownloadMedia payload = %q, want %q", string(data), string(plaintext))
	}
}

func TestChannel_DownloadMedia_FailsOnChecksumMismatch(t *testing.T) {
	plaintext := []byte("mismatch payload")
	mediaKey := make([]byte, 32)
	if _, err := rand.Read(mediaKey); err != nil {
		t.Fatalf("rand.Read(mediaKey): %v", err)
	}
	ciphertext, encSHA, _ := buildEncryptedWhatsAppMedia(t, plaintext, mediaKey, "audio")
	badPlainSHA := sha256.Sum256([]byte("different"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(ciphertext)
	}))
	defer server.Close()

	ch := New(DefaultConfig(), zap.NewNop())
	ch.mu.Lock()
	ch.isLoggedIn = true
	ch.mu.Unlock()

	_, err := ch.DownloadMedia(context.Background(), server.URL, mediaKey, encSHA, badPlainSHA[:], uint64(len(plaintext)), "audio")
	if err == nil {
		t.Fatal("expected checksum mismatch error")
	}
}

func buildEncryptedWhatsAppMedia(t *testing.T, plaintext, mediaKey []byte, mediaType string) ([]byte, []byte, []byte) {
	t.Helper()
	iv, cipherKey, macKey, err := deriveWhatsAppMediaKeys(mediaKey, mediaType)
	if err != nil {
		t.Fatalf("deriveWhatsAppMediaKeys error = %v", err)
	}
	padded := pkcs7Pad(plaintext, aes.BlockSize)
	block, err := aes.NewCipher(cipherKey)
	if err != nil {
		t.Fatalf("aes.NewCipher error = %v", err)
	}
	ciphertext := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ciphertext, padded)
	h := hmac.New(sha256.New, macKey)
	h.Write(iv)
	h.Write(ciphertext)
	mac := h.Sum(nil)[:whatsappMediaMACLength]
	payload := append(append([]byte{}, ciphertext...), mac...)
	encSHA := sha256.Sum256(payload)
	plainSHA := sha256.Sum256(plaintext)
	return payload, encSHA[:], plainSHA[:]
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padLen := blockSize - (len(data) % blockSize)
	if padLen == 0 {
		padLen = blockSize
	}
	return append(append([]byte{}, data...), bytes.Repeat([]byte{byte(padLen)}, padLen)...)
}

func TestFetchWhatsAppMedia_PropagatesHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusForbidden)
	}))
	defer server.Close()

	_, err := fetchWhatsAppMedia(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected HTTP error")
	}
}

func TestDecryptWhatsAppMedia_RejectsBadMAC(t *testing.T) {
	plaintext := []byte("hello")
	mediaKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, mediaKey); err != nil {
		t.Fatalf("ReadFull(rand.Reader): %v", err)
	}
	payload, _, _ := buildEncryptedWhatsAppMedia(t, plaintext, mediaKey, "video")
	payload[len(payload)-1] ^= 0xFF

	_, err := decryptWhatsAppMedia(payload, mediaKey, "video")
	if err == nil {
		t.Fatal("expected MAC mismatch error")
	}
}
