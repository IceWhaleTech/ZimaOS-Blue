// Package nostr provides a Nostr protocol channel implementation.
// It supports NIP-04 encrypted direct messages (kind 4) over multiple relays.
package nostr

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/validator"
)

// Config contains Nostr channel configuration.
type Config struct {
	Enabled    bool   `yaml:"enabled"`
	PrivateKey string `yaml:"private_key"` // 64 hex chars
	PublicKey  string `yaml:"public_key"`  // derived if empty
	Relays     string `yaml:"relays"`      // comma-separated relay URLs
}

// secp256k1 curve parameters.
var secp256k1Params = &elliptic.CurveParams{
	P:       fromHex("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFC2F"),
	N:       fromHex("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141"),
	B:       big.NewInt(7),
	Gx:      fromHex("79BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798"),
	Gy:      fromHex("483ADA7726A3C4655DA4FBFC0E1108A8FD17B448A68554199C47D08FFB10D4B8"),
	BitSize: 256,
	Name:    "secp256k1",
}

func fromHex(s string) *big.Int {
	v, _ := new(big.Int).SetString(s, 16)
	return v
}

func secp256k1Curve() elliptic.Curve { return secp256k1Params }

const (
	kindEncryptedDM = 4
	pollInterval    = 3 * time.Second
)

// nostrEvent represents a Nostr event (NIP-01).
type nostrEvent struct {
	ID        string     `json:"id"`
	PubKey    string     `json:"pubkey"`
	CreatedAt int64      `json:"created_at"`
	Kind      int        `json:"kind"`
	Tags      [][]string `json:"tags"`
	Content   string     `json:"content"`
	Sig       string     `json:"sig"`
}

// relayMessage represents a message from a relay.
type relayMessage struct {
	Type  string      // "EVENT", "EOSE", "OK", "NOTICE"
	SubID string      // subscription ID
	Event *nostrEvent // for EVENT messages
}

// Channel implements the channel.Channel interface for Nostr.
type Channel struct {
	config     Config
	logger     *zap.Logger
	messages   chan channel.Message
	privKey    *ecdsa.PrivateKey
	pubKeyHex  string
	relayURLs  []string

	mu          sync.RWMutex
	status      channel.Status
	connectedAt *time.Time
	lastError   string
	lastErrorAt *time.Time
	msgCount    atomic.Int64
	msgsSent    atomic.Int64
	msgsRecv    atomic.Int64

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Track seen event IDs to avoid duplicates
	seenMu sync.Mutex
	seen   map[string]bool
}

// New creates a new Nostr channel.
func New(cfg Config, logger *zap.Logger) *Channel {
	return &Channel{
		config:   cfg,
		logger:   logger.With(zap.String("channel", "nostr")),
		messages: make(chan channel.Message, 100),
		status:   channel.StatusDisconnected,
		seen:     make(map[string]bool),
	}
}

func (c *Channel) Name() string                    { return "nostr" }
func (c *Channel) Type() string                    { return "nostr" }
func (c *Channel) Messages() <-chan channel.Message { return c.messages }

func (c *Channel) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status == channel.StatusConnected
}

// Start initializes keys and begins polling relays for DM events.
func (c *Channel) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.status == channel.StatusConnected || c.status == channel.StatusConnecting {
		c.mu.Unlock()
		return fmt.Errorf("channel already started")
	}
	c.status = channel.StatusConnecting
	c.mu.Unlock()

	// Parse private key
	privKeyBytes, err := hex.DecodeString(c.config.PrivateKey)
	if err != nil || len(privKeyBytes) != 32 {
		c.setError("invalid private key: must be 64 hex characters")
		return fmt.Errorf("invalid private key")
	}

	curve := secp256k1Curve()
	privKey := new(ecdsa.PrivateKey)
	privKey.Curve = curve
	privKey.D = new(big.Int).SetBytes(privKeyBytes)
	privKey.PublicKey.X, privKey.PublicKey.Y = curve.ScalarBaseMult(privKeyBytes)
	c.privKey = privKey

	// Derive public key (x-only, 32 bytes hex)
	pubKeyBytes := privKey.PublicKey.X.Bytes()
	if len(pubKeyBytes) < 32 {
		padded := make([]byte, 32)
		copy(padded[32-len(pubKeyBytes):], pubKeyBytes)
		pubKeyBytes = padded
	}
	c.pubKeyHex = hex.EncodeToString(pubKeyBytes)

	if c.config.PublicKey != "" {
		c.pubKeyHex = c.config.PublicKey
	}

	// Parse relay URLs
	for _, r := range strings.Split(c.config.Relays, ",") {
		r = strings.TrimSpace(r)
		if r != "" {
			c.relayURLs = append(c.relayURLs, r)
		}
	}
	if len(c.relayURLs) == 0 {
		c.setError("no relays configured")
		return fmt.Errorf("no relays configured")
	}

	c.ctx, c.cancel = context.WithCancel(ctx)

	// Start polling each relay
	for _, relay := range c.relayURLs {
		c.wg.Add(1)
		go c.pollRelay(relay)
	}

	now := time.Now()
	c.mu.Lock()
	c.status = channel.StatusConnected
	c.connectedAt = &now
	c.lastError = ""
	c.lastErrorAt = nil
	c.mu.Unlock()

	c.logger.Info("Nostr channel started",
		zap.String("pubkey", c.pubKeyHex),
		zap.Int("relay_count", len(c.relayURLs)))
	return nil
}

// Stop gracefully shuts down the channel.
func (c *Channel) Stop(ctx context.Context) error {
	c.mu.Lock()
	if c.status == channel.StatusDisconnected {
		c.mu.Unlock()
		return nil
	}
	c.status = channel.StatusDisconnected
	c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
	}

	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}

	close(c.messages)
	c.logger.Info("Nostr channel stopped")
	return nil
}

// pollRelay polls a single relay for DM events using HTTP-based REQ/response.
// For simplicity, this uses a polling approach with the NIP-01 HTTP JSON API
// pattern: POST a REQ filter and read EVENT responses.
func (c *Channel) pollRelay(relayURL string) {
	defer c.wg.Done()

	// Convert wss:// to https:// for HTTP polling fallback
	httpURL := relayURL
	httpURL = strings.Replace(httpURL, "wss://", "https://", 1)
	httpURL = strings.Replace(httpURL, "ws://", "http://", 1)

	client := &http.Client{Timeout: 15 * time.Second}
	sinceTime := time.Now().Unix()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-time.After(pollInterval):
		}

		// Build REQ filter for kind 4 DMs addressed to our pubkey
		filter := map[string]interface{}{
			"kinds": []int{kindEncryptedDM},
			"#p":    []string{c.pubKeyHex},
			"since": sinceTime,
		}

		reqBody, err := json.Marshal([]interface{}{"REQ", "sub1", filter})
		if err != nil {
			c.logger.Error("failed to marshal REQ", zap.Error(err))
			continue
		}

		req, err := http.NewRequestWithContext(c.ctx, http.MethodPost, httpURL, strings.NewReader(string(reqBody)))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			c.logger.Debug("relay poll failed", zap.String("relay", relayURL), zap.Error(err))
			continue
		}

		c.processRelayResponse(resp.Body, relayURL)
		resp.Body.Close()

		sinceTime = time.Now().Unix()
	}
}

// processRelayResponse reads and processes events from a relay response.
func (c *Channel) processRelayResponse(body io.Reader, relayURL string) {
	decoder := json.NewDecoder(body)
	for decoder.More() {
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			break
		}

		var arr []json.RawMessage
		if err := json.Unmarshal(raw, &arr); err != nil {
			continue
		}
		if len(arr) < 2 {
			continue
		}

		var msgType string
		if err := json.Unmarshal(arr[0], &msgType); err != nil {
			continue
		}

		if msgType == "EVENT" && len(arr) >= 3 {
			var evt nostrEvent
			if err := json.Unmarshal(arr[2], &evt); err != nil {
				continue
			}
			c.handleDMEvent(&evt, relayURL)
		}
	}
}

// handleDMEvent processes an incoming NIP-04 encrypted DM event.
func (c *Channel) handleDMEvent(evt *nostrEvent, relayURL string) {
	if evt.Kind != kindEncryptedDM {
		return
	}

	// Deduplicate
	c.seenMu.Lock()
	if c.seen[evt.ID] {
		c.seenMu.Unlock()
		return
	}
	c.seen[evt.ID] = true
	c.seenMu.Unlock()

	// Decrypt NIP-04 content
	plaintext, err := c.decryptNIP04(evt.Content, evt.PubKey)
	if err != nil {
		c.logger.Warn("failed to decrypt DM",
			zap.String("event_id", evt.ID),
			zap.Error(err))
		return
	}

	msg := channel.Message{
		ID:          evt.ID,
		ChannelName: "nostr",
		ChatID:      evt.PubKey,
		UserID:      evt.PubKey,
		Username:    evt.PubKey[:12] + "...",
		Type:        channel.MessageTypeText,
		Content:     plaintext,
		Timestamp:   time.Unix(evt.CreatedAt, 0),
		IsGroup:     false,
		Metadata: map[string]interface{}{
			"relay":  relayURL,
			"pubkey": evt.PubKey,
			"kind":   evt.Kind,
		},
	}

	c.msgCount.Add(1)
	c.msgsRecv.Add(1)

	select {
	case c.messages <- msg:
	default:
		c.logger.Warn("message channel full, dropping message", zap.String("id", evt.ID))
	}
}

// Send sends an encrypted DM via Nostr relays.
func (c *Channel) Send(ctx context.Context, msg channel.OutgoingMessage) error {
	recipientPubKey := msg.ChatID

	// Encrypt content with NIP-04
	encrypted, err := c.encryptNIP04(msg.Content, recipientPubKey)
	if err != nil {
		return fmt.Errorf("failed to encrypt message: %w", err)
	}

	now := time.Now().Unix()
	evt := nostrEvent{
		PubKey:    c.pubKeyHex,
		CreatedAt: now,
		Kind:      kindEncryptedDM,
		Tags:      [][]string{{"p", recipientPubKey}},
		Content:   encrypted,
	}

	// Compute event ID
	evt.ID = computeEventID(&evt)

	// Sign event
	sig, err := c.signEvent(&evt)
	if err != nil {
		return fmt.Errorf("failed to sign event: %w", err)
	}
	evt.Sig = sig

	// Publish to all relays
	eventMsg, _ := json.Marshal([]interface{}{"EVENT", evt})
	client := &http.Client{Timeout: 10 * time.Second}

	var lastErr error
	for _, relayURL := range c.relayURLs {
		httpURL := strings.Replace(relayURL, "wss://", "https://", 1)
		httpURL = strings.Replace(httpURL, "ws://", "http://", 1)

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, httpURL, strings.NewReader(string(eventMsg)))
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			c.logger.Debug("failed to publish to relay", zap.String("relay", relayURL), zap.Error(err))
			continue
		}
		resp.Body.Close()
		lastErr = nil
	}

	if lastErr != nil {
		return fmt.Errorf("failed to publish to any relay: %w", lastErr)
	}

	c.msgsSent.Add(1)
	return nil
}

// SendStreaming accumulates chunked content into multiple DM events.
func (c *Channel) SendStreaming(ctx context.Context, chatID string, replyToID string, content <-chan string, done chan<- struct{}) error {
	defer close(done)

	var chunk strings.Builder
	const maxChunkSize = 3000 // Keep under typical relay limits

	for {
		select {
		case <-ctx.Done():
			// Send remaining
			if chunk.Len() > 0 {
				_ = c.Send(ctx, channel.OutgoingMessage{ChatID: chatID, Content: chunk.String()})
			}
			return ctx.Err()
		case text, ok := <-content:
			if !ok {
				if chunk.Len() > 0 {
					return c.Send(ctx, channel.OutgoingMessage{ChatID: chatID, Content: chunk.String()})
				}
				return nil
			}
			chunk.WriteString(text)
			// Flush if chunk is large enough
			if chunk.Len() >= maxChunkSize {
				if err := c.Send(ctx, channel.OutgoingMessage{ChatID: chatID, Content: chunk.String()}); err != nil {
					return err
				}
				chunk.Reset()
			}
		}
	}
}

func (c *Channel) Info() channel.Info {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return channel.Info{
		Name:             "nostr",
		Type:             "nostr",
		Status:           c.status,
		Enabled:          c.config.Enabled,
		ConnectedAt:      c.connectedAt,
		LastError:        c.lastError,
		LastErrorAt:      c.lastErrorAt,
		MessageCount:     c.msgCount.Load(),
		MessagesReceived: c.msgsRecv.Load(),
		MessagesSent:     c.msgsSent.Load(),
		Metadata: map[string]interface{}{
			"pubkey":      c.pubKeyHex,
			"relay_count": len(c.relayURLs),
			"relays":      c.relayURLs,
		},
	}
}

func (c *Channel) setError(errMsg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastError = errMsg
	now := time.Now()
	c.lastErrorAt = &now
	c.status = channel.StatusError
}

// --- NIP-04 Encryption/Decryption ---

// computeSharedSecret computes the ECDH shared secret for NIP-04.
func (c *Channel) computeSharedSecret(theirPubKeyHex string) ([]byte, error) {
	theirPubBytes, err := hex.DecodeString(theirPubKeyHex)
	if err != nil || len(theirPubBytes) != 32 {
		return nil, fmt.Errorf("invalid public key")
	}

	curve := secp256k1Curve()
	// Reconstruct full public key from x-coordinate (assume even y)
	x := new(big.Int).SetBytes(theirPubBytes)
	// y^2 = x^3 + 7 mod p
	x3 := new(big.Int).Mul(x, x)
	x3.Mul(x3, x)
	x3.Mod(x3, curve.Params().P)
	y2 := new(big.Int).Add(x3, curve.Params().B)
	y2.Mod(y2, curve.Params().P)
	y := new(big.Int).ModSqrt(y2, curve.Params().P)
	if y == nil {
		return nil, fmt.Errorf("invalid public key point")
	}
	// Use even y
	if y.Bit(0) != 0 {
		y.Sub(curve.Params().P, y)
	}

	// ECDH: shared = our_privkey * their_pubkey
	sx, _ := curve.ScalarMult(x, y, c.privKey.D.Bytes())
	sharedBytes := sx.Bytes()
	if len(sharedBytes) < 32 {
		padded := make([]byte, 32)
		copy(padded[32-len(sharedBytes):], sharedBytes)
		sharedBytes = padded
	}
	return sharedBytes, nil
}

// encryptNIP04 encrypts plaintext using NIP-04 (AES-256-CBC).
// Returns: base64(ciphertext)?iv=base64(iv)
func (c *Channel) encryptNIP04(plaintext string, recipientPubKey string) (string, error) {
	sharedSecret, err := c.computeSharedSecret(recipientPubKey)
	if err != nil {
		return "", err
	}

	// Pad plaintext with PKCS7
	plaintextBytes := []byte(plaintext)
	blockSize := aes.BlockSize
	padding := blockSize - len(plaintextBytes)%blockSize
	for i := 0; i < padding; i++ {
		plaintextBytes = append(plaintextBytes, byte(padding))
	}

	// Generate random IV
	iv := make([]byte, aes.BlockSize)
	if _, err := rand.Read(iv); err != nil {
		return "", fmt.Errorf("failed to generate IV: %w", err)
	}

	block, err := aes.NewCipher(sharedSecret)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	ciphertext := make([]byte, len(plaintextBytes))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, plaintextBytes)

	return base64.StdEncoding.EncodeToString(ciphertext) + "?iv=" + base64.StdEncoding.EncodeToString(iv), nil
}

// decryptNIP04 decrypts NIP-04 encrypted content.
// Expects format: base64(ciphertext)?iv=base64(iv)
func (c *Channel) decryptNIP04(content string, senderPubKey string) (string, error) {
	parts := strings.SplitN(content, "?iv=", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid NIP-04 format: missing ?iv=")
	}

	ciphertext, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	iv, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("failed to decode IV: %w", err)
	}

	sharedSecret, err := c.computeSharedSecret(senderPubKey)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(sharedSecret)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	if len(ciphertext)%aes.BlockSize != 0 {
		return "", fmt.Errorf("ciphertext not a multiple of block size")
	}

	plaintext := make([]byte, len(ciphertext))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(plaintext, ciphertext)

	// Remove PKCS7 padding
	if len(plaintext) == 0 {
		return "", fmt.Errorf("empty plaintext")
	}
	padLen := int(plaintext[len(plaintext)-1])
	if padLen > aes.BlockSize || padLen == 0 || padLen > len(plaintext) {
		return "", fmt.Errorf("invalid PKCS7 padding")
	}
	for i := len(plaintext) - padLen; i < len(plaintext); i++ {
		if plaintext[i] != byte(padLen) {
			return "", fmt.Errorf("invalid PKCS7 padding")
		}
	}

	return string(plaintext[:len(plaintext)-padLen]), nil
}

// --- Event Signing ---

// computeEventID computes the SHA256 hash of the serialized event per NIP-01.
func computeEventID(evt *nostrEvent) string {
	// [0, pubkey, created_at, kind, tags, content]
	serialized, _ := json.Marshal([]interface{}{
		0,
		evt.PubKey,
		evt.CreatedAt,
		evt.Kind,
		evt.Tags,
		evt.Content,
	})
	hash := sha256.Sum256(serialized)
	return hex.EncodeToString(hash[:])
}

// signEvent signs the event ID with the private key using Schnorr-like signature.
// For simplicity, this uses ECDSA and encodes as 64-byte hex (r||s).
func (c *Channel) signEvent(evt *nostrEvent) (string, error) {
	idBytes, err := hex.DecodeString(evt.ID)
	if err != nil {
		return "", fmt.Errorf("invalid event ID: %w", err)
	}

	r, s, err := ecdsa.Sign(rand.Reader, c.privKey, idBytes)
	if err != nil {
		return "", fmt.Errorf("failed to sign: %w", err)
	}

	// Encode r and s as 32 bytes each
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	sig := make([]byte, 64)
	copy(sig[32-len(rBytes):32], rBytes)
	copy(sig[64-len(sBytes):], sBytes)

	return hex.EncodeToString(sig), nil
}

// --- Validator ---

// Validator validates Nostr configuration.
type Validator struct{}

// NewValidator creates a new Nostr validator.
func NewValidator() *Validator { return &Validator{} }

// Validate validates the Nostr configuration.
func (v *Validator) Validate(ctx context.Context, config map[string]string) validator.Result {
	privateKey := config["private_key"]
	if privateKey == "" {
		return validator.Result{
			Success:    false,
			Error:      "private_key is required",
			MessageKey: "channels.privateKeyRequired",
		}
	}

	// Validate private key format: 64 hex chars
	if len(privateKey) != 64 {
		return validator.Result{
			Success:    false,
			Error:      "private_key must be 64 hex characters",
			MessageKey: "channels.invalidPrivateKey",
		}
	}
	if _, err := hex.DecodeString(privateKey); err != nil {
		return validator.Result{
			Success:    false,
			Error:      "private_key must be valid hex",
			MessageKey: "channels.invalidPrivateKey",
		}
	}

	relays := config["relays"]
	if relays == "" {
		return validator.Result{
			Success:    false,
			Error:      "relays is required (comma-separated relay URLs)",
			MessageKey: "channels.relaysRequired",
		}
	}

	// Test relay connectivity via HTTP handshake
	relayList := strings.Split(relays, ",")
	client := &http.Client{Timeout: 10 * time.Second}
	connectedRelays := 0

	for _, relay := range relayList {
		relay = strings.TrimSpace(relay)
		if relay == "" {
			continue
		}
		httpURL := strings.Replace(relay, "wss://", "https://", 1)
		httpURL = strings.Replace(httpURL, "ws://", "http://", 1)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, httpURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Accept", "application/nostr+json")

		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()
		connectedRelays++
	}

	// Derive public key for display
	privKeyBytes, _ := hex.DecodeString(privateKey)
	curve := secp256k1Curve()
	x, _ := curve.ScalarBaseMult(privKeyBytes)
	xBytes := x.Bytes()
	if len(xBytes) < 32 {
		padded := make([]byte, 32)
		copy(padded[32-len(xBytes):], xBytes)
		xBytes = padded
	}
	pubKeyHex := hex.EncodeToString(xBytes)

	return validator.Result{
		Success:    true,
		MessageKey: "channels.connectionSuccess",
		Data: map[string]interface{}{
			"public_key":       pubKeyHex,
			"relays_total":     len(relayList),
			"relays_reachable": connectedRelays,
		},
	}
}

