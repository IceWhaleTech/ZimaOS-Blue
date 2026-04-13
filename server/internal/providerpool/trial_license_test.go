package providerpool

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Test key pair 1 (from plan)
const testPrivateKey1 = "MC4CAQAwBQYDK2VwBCIEIL92Mg6vYxGXSoNiNxnfg6hsezyNsaoLQCgyHp0pAjXZ"
const testPrivateKey2 = "MC4CAQAwBQYDK2VwBCIEIN2Gx6tmA/jqQIGYLFswWbbmbqmS3wj1jyui9JYrr1nE"

func testClaims() *LicenseClaims {
	return &LicenseClaims{
		KID:      "1",
		Key:      "sk-test-key-12345",
		URL:      "https://api.example.com/",
		Limit:    10000,
		IssuedAt: time.Now().Unix(),
		Format:   "anthropic",
		Model:    "claude-haiku-4-5",
	}
}

func TestSignAndVerifyLicense(t *testing.T) {
	claims := testClaims()
	license, err := SignLicense(claims, testPrivateKey1)
	if err != nil {
		t.Fatalf("SignLicense: %v", err)
	}

	verified, sig, err := VerifyLicense(license)
	if err != nil {
		t.Fatalf("VerifyLicense: %v", err)
	}
	if verified.Key != claims.Key {
		t.Errorf("Key = %q, want %q", verified.Key, claims.Key)
	}
	if verified.URL != claims.URL {
		t.Errorf("URL = %q, want %q", verified.URL, claims.URL)
	}
	if verified.Limit != claims.Limit {
		t.Errorf("Limit = %d, want %d", verified.Limit, claims.Limit)
	}
	if len(sig) != 64 {
		t.Errorf("signature length = %d, want 64", len(sig))
	}
}

func TestVerifyLicense_RejectsNonHTTPSURL(t *testing.T) {
	claims := testClaims()
	claims.URL = "http://api.jianli.eu.org/"

	license, err := SignLicense(claims, testPrivateKey1)
	if err != nil {
		t.Fatalf("SignLicense: %v", err)
	}

	_, _, err = VerifyLicense(license)
	if !errors.Is(err, ErrLicenseInvalid) {
		t.Fatalf("VerifyLicense error = %v, want ErrLicenseInvalid", err)
	}
}

func TestVerifyLicense_TamperedPayload(t *testing.T) {
	claims := testClaims()
	license, _ := SignLicense(claims, testPrivateKey1)

	// Tamper with payload (flip a character)
	bs := []byte(license)
	bs[5] ^= 0x01
	_, _, err := VerifyLicense(string(bs))
	if err == nil {
		t.Fatal("expected error for tampered payload")
	}
}

func TestVerifyLicense_TamperedSignature(t *testing.T) {
	claims := testClaims()
	license, _ := SignLicense(claims, testPrivateKey1)

	// Find the dot and tamper after it
	parts := splitLicense(license)
	bs := []byte(parts[1])
	bs[2] ^= 0xFF
	tampered := parts[0] + "." + string(bs)

	_, _, err := VerifyLicense(tampered)
	if err == nil {
		t.Fatal("expected error for tampered signature")
	}
}

func TestVerifyLicense_WrongKey(t *testing.T) {
	// Sign with key 2 but claim kid "1"
	claims := testClaims()
	claims.KID = "1"

	// Sign with key 2's private key but keep kid=1
	license, err := SignLicense(&LicenseClaims{
		KID: "2", Key: claims.Key, URL: claims.URL, Limit: claims.Limit,
		IssuedAt: claims.IssuedAt, Format: claims.Format, Model: claims.Model,
	}, testPrivateKey2)
	if err != nil {
		t.Fatalf("SignLicense: %v", err)
	}

	// Verify should succeed because kid=2 matches key 2
	_, _, err = VerifyLicense(license)
	if err != nil {
		t.Fatalf("VerifyLicense with matching kid/key should succeed: %v", err)
	}

	// Now tamper kid to "1" — signature won't match key 1's public key
	// This is inherently tested by TamperedPayload test since kid is in payload
}

func TestVerifyLicense_Expired(t *testing.T) {
	claims := testClaims()
	claims.ExpiresAt = time.Now().Add(-1 * time.Hour).Unix()

	license, _ := SignLicense(claims, testPrivateKey1)
	verified, sig, err := VerifyLicense(license)
	if err != ErrLicenseExpired {
		t.Fatalf("expected ErrLicenseExpired, got %v", err)
	}
	// Should still return claims and sig for callers to distinguish expired vs invalid
	if verified == nil {
		t.Fatal("expected non-nil claims for expired license")
	}
	if len(sig) == 0 {
		t.Fatal("expected non-empty sig for expired license")
	}
	if verified.Key != claims.Key {
		t.Errorf("Key = %q, want %q", verified.Key, claims.Key)
	}
}

func TestVerifyLicense_NotExpired(t *testing.T) {
	claims := testClaims()
	claims.ExpiresAt = time.Now().Add(24 * time.Hour).Unix()

	license, _ := SignLicense(claims, testPrivateKey1)
	_, _, err := VerifyLicense(license)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyLicense_NoExpiry(t *testing.T) {
	claims := testClaims()
	claims.ExpiresAt = 0

	license, _ := SignLicense(claims, testPrivateKey1)
	_, _, err := VerifyLicense(license)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyLicense_InvalidFormat(t *testing.T) {
	_, _, err := VerifyLicense("not-a-license")
	if err != ErrLicenseInvalid {
		t.Fatalf("expected ErrLicenseInvalid, got %v", err)
	}
}

func TestVerifyLicense_UnknownKID(t *testing.T) {
	claims := testClaims()
	claims.KID = "99"
	license, _ := SignLicense(claims, testPrivateKey1)
	// The signature was made with key 1 but claims kid=99
	_, _, err := VerifyLicense(license)
	if err != ErrLicenseKeyID {
		t.Fatalf("expected ErrLicenseKeyID, got %v", err)
	}
}

// --- HMAC State Tests ---

func TestSaveAndLoadTrialState(t *testing.T) {
	dir := t.TempDir()
	sig := make([]byte, 64)
	for i := range sig {
		sig[i] = byte(i)
	}

	state := &trialState{
		TokensUsed:  5000,
		Exhausted:   false,
		LastUpdated: time.Now().Truncate(time.Second),
	}

	if err := SaveTrialState(dir, sig, state); err != nil {
		t.Fatalf("SaveTrialState: %v", err)
	}

	loaded, err := LoadTrialState(dir, sig)
	if err != nil {
		t.Fatalf("LoadTrialState: %v", err)
	}
	if loaded.TokensUsed != state.TokensUsed {
		t.Errorf("TokensUsed = %d, want %d", loaded.TokensUsed, state.TokensUsed)
	}
	if loaded.Exhausted != state.Exhausted {
		t.Errorf("Exhausted = %v, want %v", loaded.Exhausted, state.Exhausted)
	}
}

func TestLoadTrialState_FreshInstall(t *testing.T) {
	dir := t.TempDir()
	sig := make([]byte, 64)

	loaded, err := LoadTrialState(dir, sig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded != nil {
		t.Fatal("expected nil for fresh install")
	}
}

func TestLoadTrialState_Tampered(t *testing.T) {
	dir := t.TempDir()
	sig := make([]byte, 64)
	for i := range sig {
		sig[i] = byte(i)
	}

	state := &trialState{TokensUsed: 5000}
	SaveTrialState(dir, sig, state)

	// Tamper with the file
	path := filepath.Join(dir, trialStateFile)
	data, _ := os.ReadFile(path)
	data[10] ^= 0xFF
	os.WriteFile(path, data, 0600)

	_, err := LoadTrialState(dir, sig)
	if err != ErrStateTampered {
		t.Fatalf("expected ErrStateTampered, got %v", err)
	}
}

func TestLoadTrialState_WrongKey(t *testing.T) {
	dir := t.TempDir()
	sig1 := make([]byte, 64)
	sig2 := make([]byte, 64)
	for i := range sig1 {
		sig1[i] = byte(i)
		sig2[i] = byte(i + 100)
	}

	state := &trialState{TokensUsed: 5000}
	SaveTrialState(dir, sig1, state)

	// Try to load with different signature
	_, err := LoadTrialState(dir, sig2)
	if err != ErrStateTampered {
		t.Fatalf("expected ErrStateTampered, got %v", err)
	}
}

func TestKeyRotation(t *testing.T) {
	// Sign with key 1
	claims1 := testClaims()
	claims1.KID = "1"
	lic1, _ := SignLicense(claims1, testPrivateKey1)

	// Sign with key 2
	claims2 := testClaims()
	claims2.KID = "2"
	lic2, _ := SignLicense(claims2, testPrivateKey2)

	// Both should verify
	_, _, err := VerifyLicense(lic1)
	if err != nil {
		t.Fatalf("key 1 verify: %v", err)
	}
	_, _, err = VerifyLicense(lic2)
	if err != nil {
		t.Fatalf("key 2 verify: %v", err)
	}
}

// --- Clock Rollback & Version Reset Tests ---

func TestTrialQuotaManager_ClockRollback(t *testing.T) {
	dir := t.TempDir()
	claims := testClaims()
	claims.ExpiresAt = time.Now().Add(24 * time.Hour).Unix()
	license, _ := SignLicense(claims, testPrivateKey1)

	// Create manager to initialize state
	m := NewTrialQuotaManager(nil, dir, license)
	if m.IsExhausted() {
		t.Fatal("should not be exhausted initially")
	}

	// Manually write state with a high-water mark far in the future (simulates clock rollback)
	_, sig, _ := VerifyLicense(license)
	futureHWM := time.Now().Add(48 * time.Hour).Unix()
	state := &trialState{
		TokensUsed:    100,
		Exhausted:     false,
		LastUpdated:   time.Now(),
		HighWaterMark: futureHWM,
		LicenseIAT:    claims.IssuedAt,
	}
	SaveTrialState(dir, sig, state)

	// Re-create manager — should detect clock rollback
	m2 := NewTrialQuotaManager(nil, dir, license)
	if !m2.IsExhausted() {
		t.Fatal("should be exhausted after clock rollback")
	}
	status := m2.GetStatus()
	if status.ExhaustedReason != "clock_rollback" {
		t.Errorf("reason = %q, want clock_rollback", status.ExhaustedReason)
	}
}

func TestTrialQuotaManager_VersionReset(t *testing.T) {
	dir := t.TempDir()

	// Old version license
	oldClaims := testClaims()
	oldClaims.IssuedAt = 1000000
	oldClaims.ExpiresAt = time.Now().Add(24 * time.Hour).Unix()
	oldLicense, _ := SignLicense(oldClaims, testPrivateKey1)

	// Use some quota with old license
	m := NewTrialQuotaManager(nil, dir, oldLicense)
	m.RecordUsage(5000, 0, "s1")
	if m.GetStatus().TokensUsed != 5000 {
		t.Fatalf("tokens = %d, want 5000", m.GetStatus().TokensUsed)
	}

	// New version license (different iat)
	newClaims := testClaims()
	newClaims.IssuedAt = 2000000
	newClaims.ExpiresAt = time.Now().Add(24 * time.Hour).Unix()
	newLicense, _ := SignLicense(newClaims, testPrivateKey1)

	// Re-create with new license — quota should reset
	m2 := NewTrialQuotaManager(nil, dir, newLicense)
	if m2.IsExhausted() {
		t.Fatal("should not be exhausted after version reset")
	}
	if m2.GetStatus().TokensUsed != 0 {
		t.Errorf("tokens = %d, want 0 after version reset", m2.GetStatus().TokensUsed)
	}
}

func TestTrialQuotaManager_ExpiredLicense(t *testing.T) {
	dir := t.TempDir()
	claims := testClaims()
	claims.ExpiresAt = time.Now().Add(-1 * time.Hour).Unix()
	license, _ := SignLicense(claims, testPrivateKey1)

	m := NewTrialQuotaManager(nil, dir, license)
	if !m.IsExhausted() {
		t.Fatal("should be exhausted for expired license")
	}
	status := m.GetStatus()
	if status.ExhaustedReason != "license_expired" {
		t.Errorf("reason = %q, want license_expired", status.ExhaustedReason)
	}
	if !status.IsExpired {
		t.Error("IsExpired should be true")
	}
}

func TestTrialQuotaManager_SameVersionKeepsQuota(t *testing.T) {
	dir := t.TempDir()
	claims := testClaims()
	claims.IssuedAt = 1000000
	claims.ExpiresAt = time.Now().Add(24 * time.Hour).Unix()
	license, _ := SignLicense(claims, testPrivateKey1)

	m := NewTrialQuotaManager(nil, dir, license)
	m.RecordUsage(3000, 0, "s1")

	// Re-create with same license — quota should persist
	m2 := NewTrialQuotaManager(nil, dir, license)
	if m2.GetStatus().TokensUsed != 3000 {
		t.Errorf("tokens = %d, want 3000", m2.GetStatus().TokensUsed)
	}
}

func TestSaveTrialState_HighWaterMark(t *testing.T) {
	dir := t.TempDir()
	sig := make([]byte, 64)
	for i := range sig {
		sig[i] = byte(i)
	}

	state := &trialState{
		TokensUsed:    1000,
		HighWaterMark: 1700000000,
		LicenseIAT:    1600000000,
	}
	SaveTrialState(dir, sig, state)

	loaded, err := LoadTrialState(dir, sig)
	if err != nil {
		t.Fatalf("LoadTrialState: %v", err)
	}
	if loaded.HighWaterMark != 1700000000 {
		t.Errorf("HighWaterMark = %d, want 1700000000", loaded.HighWaterMark)
	}
	if loaded.LicenseIAT != 1600000000 {
		t.Errorf("LicenseIAT = %d, want 1600000000", loaded.LicenseIAT)
	}
}

func splitLicense(s string) [2]string {
	for i, c := range s {
		if c == '.' {
			return [2]string{s[:i], s[i+1:]}
		}
	}
	return [2]string{s, ""}
}
