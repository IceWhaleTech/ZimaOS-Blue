package nostr

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
)

func TestChannel_Send_AttachmentFallbackIncludedInEncryptedContent(t *testing.T) {
	sender := newTestNostrChannel(t, strings.Repeat("1", 64))
	recipient := newTestNostrChannel(t, strings.Repeat("2", 64))

	var evt nostrEvent
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var payload []json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if len(payload) != 2 {
			t.Fatalf("unexpected payload length %d", len(payload))
		}
		if err := json.Unmarshal(payload[1], &evt); err != nil {
			t.Fatalf("decode event: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	sender.relayURLs = []string{server.URL}

	err := sender.Send(context.Background(), channel.OutgoingMessage{
		ChatID:      recipient.pubKeyHex,
		Content:     "caption",
		Attachments: []channel.Attachment{{Type: channel.MessageTypeFile, URL: "https://example.com/file.pdf"}},
	})
	if err != nil {
		t.Fatalf("Send error = %v", err)
	}
	plaintext, err := recipient.decryptNIP04(evt.Content, sender.pubKeyHex)
	if err != nil {
		t.Fatalf("decryptNIP04 error = %v", err)
	}
	if plaintext != "caption\nhttps://example.com/file.pdf" {
		t.Fatalf("plaintext = %q, want caption+url", plaintext)
	}
}

func TestChannel_Send_NoSendableContentErrors(t *testing.T) {
	sender := newTestNostrChannel(t, strings.Repeat("1", 64))
	sender.relayURLs = []string{"http://127.0.0.1:1"}
	if err := sender.Send(context.Background(), channel.OutgoingMessage{ChatID: strings.Repeat("2", 64), Attachments: []channel.Attachment{{Type: channel.MessageTypeFile}}}); err == nil {
		t.Fatal("expected no sendable content error")
	}
}

func newTestNostrChannel(t *testing.T, privateKeyHex string) *Channel {
	t.Helper()
	ch := New(Config{}, zap.NewNop())
	privKeyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		t.Fatalf("decode private key: %v", err)
	}
	curve := secp256k1Curve()
	privKey := new(ecdsa.PrivateKey)
	privKey.Curve = curve
	privKey.D = new(big.Int).SetBytes(privKeyBytes)
	privKey.PublicKey.X, privKey.PublicKey.Y = curve.ScalarBaseMult(privKeyBytes)
	ch.privKey = privKey
	pubKeyBytes := privKey.PublicKey.X.Bytes()
	if len(pubKeyBytes) < 32 {
		padded := make([]byte, 32)
		copy(padded[32-len(pubKeyBytes):], pubKeyBytes)
		pubKeyBytes = padded
	}
	ch.pubKeyHex = hex.EncodeToString(pubKeyBytes)
	return ch
}
