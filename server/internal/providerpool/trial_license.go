package providerpool

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// License format: base64(json_payload) + "." + base64(ed25519_signature)

var (
	ErrLicenseInvalid   = errors.New("invalid license")
	ErrLicenseExpired   = errors.New("license expired")
	ErrLicenseKeyID     = errors.New("unknown key ID")
	ErrLicenseSignature = errors.New("signature verification failed")
	ErrStateTampered    = errors.New("state file tampered")
)

// LicenseClaims contains the verified claims from a trial license.
type LicenseClaims struct {
	KID       string `json:"kid"`       // key ID
	Key       string `json:"key"`       // trial API key (plaintext in signed payload)
	URL       string `json:"url"`       // trial base URL
	Limit     int64  `json:"lim"`       // token limit
	IssuedAt  int64  `json:"iat"`       // issued-at unix timestamp
	ExpiresAt int64  `json:"exp"`       // expiry unix timestamp (0=no expiry)
	Format    string `json:"fmt"`       // API format (e.g. "anthropic")
	Model     string `json:"model"`     // default model ID
}

// Embedded public keys for Ed25519 verification (DER-encoded, base64).
// Private keys are stored in CI secrets only.
var embeddedPublicKeys = map[string]string{
	"1": "MCowBQYDK2VwAyEA23g1/UxI9poP/QyC0GMEU8mtOThm2uXelk2Ken5kiXI=",
	"2": "MCowBQYDK2VwAyEA6Psn+zQfF54p0qdrw9sOPUvSDSAGly73GF3W/dmABTs=",
	"3": "MCowBQYDK2VwAyEAyUsF88tfQc+7pDNweQKXRbFe94X4Dhicnum2LdjsJe0=",
	"4": "MCowBQYDK2VwAyEAMUFQlIReIXgrUcjo6F4iBbfd0CNdTv/f9GHZMegp8Tg=",
	"5": "MCowBQYDK2VwAyEAZTX0+DkLwxkUEdm/23nMz7FaM3x0/0xQYJoX3hsHCiM=",
}

const trialStateFile = ".trial_state"

// getPublicKey returns the Ed25519 public key for the given key ID.
func getPublicKey(kid string) (ed25519.PublicKey, error) {
	b64, ok := embeddedPublicKeys[kid]
	if !ok {
		return nil, ErrLicenseKeyID
	}
	der, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("decode public key: %w", err)
	}
	pub, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	edPub, ok := pub.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an Ed25519 public key")
	}
	return edPub, nil
}

// VerifyLicense verifies an Ed25519-signed license string and returns the claims.
// License format: base64url(json) + "." + base64url(signature)
func VerifyLicense(licenseStr string) (*LicenseClaims, []byte, error) {
	parts := strings.SplitN(licenseStr, ".", 2)
	if len(parts) != 2 {
		return nil, nil, ErrLicenseInvalid
	}

	payloadB64, sigB64 := parts[0], parts[1]

	payloadBytes, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return nil, nil, fmt.Errorf("decode payload: %w", err)
	}
	sigBytes, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return nil, nil, fmt.Errorf("decode signature: %w", err)
	}

	// Parse claims to get kid
	var claims LicenseClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, nil, fmt.Errorf("parse claims: %w", err)
	}

	// Look up public key
	pubKey, err := getPublicKey(claims.KID)
	if err != nil {
		return nil, nil, err
	}

	// Verify signature over the raw base64 payload (not decoded JSON)
	if !ed25519.Verify(pubKey, []byte(payloadB64), sigBytes) {
		return nil, nil, ErrLicenseSignature
	}

	// Check expiry — still return claims so callers can distinguish expired vs invalid
	if claims.ExpiresAt > 0 && time.Now().Unix() > claims.ExpiresAt {
		return &claims, sigBytes, ErrLicenseExpired
	}

	return &claims, sigBytes, nil
}

// SignLicense signs claims with an Ed25519 private key (PEM-encoded PKCS8).
// Used by the build tool and tests only.
func SignLicense(claims *LicenseClaims, privateKeyPEM string) (string, error) {
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		// Try raw base64 DER
		der, err := base64.StdEncoding.DecodeString(privateKeyPEM)
		if err != nil {
			return "", fmt.Errorf("decode private key: %w", err)
		}
		return signWithDER(claims, der)
	}
	return signWithDER(claims, block.Bytes)
}

func signWithDER(claims *LicenseClaims, der []byte) (string, error) {
	key, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return "", fmt.Errorf("parse private key: %w", err)
	}
	privKey, ok := key.(ed25519.PrivateKey)
	if !ok {
		return "", fmt.Errorf("not an Ed25519 private key")
	}

	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)
	sig := ed25519.Sign(privKey, []byte(payloadB64))
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)

	return payloadB64 + "." + sigB64, nil
}

// --- HMAC-protected state file ---

// trialState is the persisted quota state.
type trialState struct {
	TokensUsed      int64     `json:"tokens_used"`
	Exhausted       bool      `json:"exhausted"`
	ExhaustedReason string    `json:"exhausted_reason,omitempty"`
	LastUpdated     time.Time `json:"last_updated"`
	HighWaterMark   int64     `json:"hwm"`         // highest seen unix timestamp (anti-clock-rollback)
	LicenseIAT      int64     `json:"license_iat"` // license issued-at — detects version change
}

// hmacKey derives a 32-byte HMAC key from the license signature.
func hmacKey(licenseSig []byte) []byte {
	if len(licenseSig) >= 32 {
		return licenseSig[:32]
	}
	// Pad with SHA-256 if signature is shorter than 32 bytes (shouldn't happen for Ed25519)
	h := sha256.Sum256(licenseSig)
	return h[:]
}

// stateFilePath returns the path to the trial state file in dataDir.
func stateFilePath(dataDir string) string {
	return filepath.Join(dataDir, trialStateFile)
}

// SaveTrialState writes HMAC-protected state to disk.
// Format: base64(json) + "." + base64(hmac-sha256)
func SaveTrialState(dataDir string, licenseSig []byte, state *trialState) error {
	jsonData, err := json.Marshal(state)
	if err != nil {
		return err
	}
	payloadB64 := base64.RawURLEncoding.EncodeToString(jsonData)

	mac := hmac.New(sha256.New, hmacKey(licenseSig))
	mac.Write([]byte(payloadB64))
	macBytes := mac.Sum(nil)
	macB64 := base64.RawURLEncoding.EncodeToString(macBytes)

	content := payloadB64 + "." + macB64

	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return err
	}
	filePath := stateFilePath(dataDir)
	if err := os.WriteFile(filePath, []byte(content), 0600); err != nil {
		return err
	}
	// Set hidden + system attributes on Windows (no-op on other platforms)
	setWindowsHiddenSystem(filePath)
	return nil
}

// LoadTrialState reads and verifies the HMAC-protected state file.
// Returns (nil, nil) if file doesn't exist (fresh install).
// Returns (nil, ErrStateTampered) if HMAC verification fails.
func LoadTrialState(dataDir string, licenseSig []byte) (*trialState, error) {
	data, err := os.ReadFile(stateFilePath(dataDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // fresh install
		}
		return nil, err
	}

	parts := strings.SplitN(string(data), ".", 2)
	if len(parts) != 2 {
		return nil, ErrStateTampered
	}

	payloadB64, macB64 := parts[0], parts[1]

	// Verify HMAC
	mac := hmac.New(sha256.New, hmacKey(licenseSig))
	mac.Write([]byte(payloadB64))
	expectedMAC := mac.Sum(nil)

	actualMAC, err := base64.RawURLEncoding.DecodeString(macB64)
	if err != nil {
		return nil, ErrStateTampered
	}
	if !hmac.Equal(expectedMAC, actualMAC) {
		return nil, ErrStateTampered
	}

	// Decode payload
	jsonData, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return nil, ErrStateTampered
	}

	var state trialState
	if err := json.Unmarshal(jsonData, &state); err != nil {
		return nil, ErrStateTampered
	}

	return &state, nil
}

// DeleteTrialState removes the state file.
func DeleteTrialState(dataDir string) error {
	return os.Remove(stateFilePath(dataDir))
}

// --- License IAT marker (non-HMAC, detects version change across different HMAC keys) ---

const licenseIATFile = ".trial_iat"

// SaveLicenseIAT writes the license issued-at timestamp to a plain file.
func SaveLicenseIAT(dataDir string, iat int64) error {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return err
	}
	filePath := filepath.Join(dataDir, licenseIATFile)
	if err := os.WriteFile(filePath, []byte(fmt.Sprintf("%d", iat)), 0600); err != nil {
		return err
	}
	// Set hidden + system attributes on Windows (no-op on other platforms)
	setWindowsHiddenSystem(filePath)
	return nil
}

// LoadLicenseIAT reads the stored license IAT. Returns 0 if not found.
func LoadLicenseIAT(dataDir string) int64 {
	data, err := os.ReadFile(filepath.Join(dataDir, licenseIATFile))
	if err != nil {
		return 0
	}
	var iat int64
	fmt.Sscanf(string(data), "%d", &iat)
	return iat
}
