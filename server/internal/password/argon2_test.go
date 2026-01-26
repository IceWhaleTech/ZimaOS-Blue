package password

import (
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Memory != 64*1024 {
		t.Errorf("expected Memory to be 64MB, got %d KB", config.Memory)
	}
	if config.Iterations != 3 {
		t.Errorf("expected Iterations to be 3, got %d", config.Iterations)
	}
	if config.Parallelism != 4 {
		t.Errorf("expected Parallelism to be 4, got %d", config.Parallelism)
	}
	if config.SaltLength != 16 {
		t.Errorf("expected SaltLength to be 16, got %d", config.SaltLength)
	}
	if config.KeyLength != 32 {
		t.Errorf("expected KeyLength to be 32, got %d", config.KeyLength)
	}
}

func TestHasher_Hash(t *testing.T) {
	hasher := NewHasher(nil)
	password := "testPassword123!"

	hash, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}

	// Check format
	if !strings.HasPrefix(hash, "$argon2id$v=") {
		t.Errorf("Hash() format invalid, got %s", hash)
	}

	parts := strings.Split(hash, "$")
	if len(parts) != 6 {
		t.Errorf("Hash() expected 6 parts, got %d", len(parts))
	}
}

func TestHasher_Hash_UniqueHashes(t *testing.T) {
	hasher := NewHasher(nil)
	password := "testPassword123!"

	hash1, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}

	hash2, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}

	// Same password should produce different hashes (different salts)
	if hash1 == hash2 {
		t.Error("Hash() should produce unique hashes for same password")
	}
}

func TestHasher_Verify(t *testing.T) {
	hasher := NewHasher(nil)
	password := "testPassword123!"

	hash, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}

	// Correct password should verify
	err = hasher.Verify(password, hash)
	if err != nil {
		t.Errorf("Verify() with correct password error = %v", err)
	}

	// Wrong password should fail
	err = hasher.Verify("wrongPassword", hash)
	if err != ErrMismatchedPassword {
		t.Errorf("Verify() with wrong password expected ErrMismatchedPassword, got %v", err)
	}
}

func TestHasher_Verify_InvalidHash(t *testing.T) {
	hasher := NewHasher(nil)

	tests := []struct {
		name string
		hash string
		want error
	}{
		{
			name: "empty hash",
			hash: "",
			want: ErrInvalidHash,
		},
		{
			name: "wrong algorithm",
			hash: "$argon2i$v=19$m=65536,t=3,p=4$c2FsdA$aGFzaA",
			want: ErrInvalidHash,
		},
		{
			name: "missing parts",
			hash: "$argon2id$v=19$m=65536,t=3,p=4",
			want: ErrInvalidHash,
		},
		{
			name: "invalid base64 salt",
			hash: "$argon2id$v=19$m=65536,t=3,p=4$!!!invalid!!!$aGFzaA",
			want: ErrInvalidHash,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := hasher.Verify("password", tt.hash)
			if err != tt.want {
				t.Errorf("Verify() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestHasher_NeedsRehash(t *testing.T) {
	// Create hasher with default config
	hasher := NewHasher(nil)
	password := "testPassword123!"

	hash, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}

	// Same config should not need rehash
	if hasher.NeedsRehash(hash) {
		t.Error("NeedsRehash() should return false for same config")
	}

	// Different config should need rehash
	newHasher := NewHasher(&Config{
		Memory:      32 * 1024, // Different memory
		Iterations:  3,
		Parallelism: 4,
		SaltLength:  16,
		KeyLength:   32,
	})

	if !newHasher.NeedsRehash(hash) {
		t.Error("NeedsRehash() should return true for different config")
	}
}

func TestHasher_NeedsRehash_InvalidHash(t *testing.T) {
	hasher := NewHasher(nil)

	// Invalid hash should need rehash
	if !hasher.NeedsRehash("invalid") {
		t.Error("NeedsRehash() should return true for invalid hash")
	}
}

func TestNewHasher_NilConfig(t *testing.T) {
	hasher := NewHasher(nil)
	if hasher.config == nil {
		t.Error("NewHasher(nil) should use default config")
	}
}

func TestNewHasher_CustomConfig(t *testing.T) {
	config := &Config{
		Memory:      32 * 1024,
		Iterations:  2,
		Parallelism: 2,
		SaltLength:  32,
		KeyLength:   64,
	}

	hasher := NewHasher(config)
	password := "testPassword123!"

	hash, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}

	// Verify the hash contains the custom parameters
	if !strings.Contains(hash, "m=32768") {
		t.Error("Hash() should use custom memory parameter")
	}
	if !strings.Contains(hash, "t=2") {
		t.Error("Hash() should use custom iterations parameter")
	}
	if !strings.Contains(hash, "p=2") {
		t.Error("Hash() should use custom parallelism parameter")
	}

	// Verify should still work
	err = hasher.Verify(password, hash)
	if err != nil {
		t.Errorf("Verify() with custom config error = %v", err)
	}
}

func BenchmarkHasher_Hash(b *testing.B) {
	hasher := NewHasher(nil)
	password := "testPassword123!"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = hasher.Hash(password)
	}
}

func BenchmarkHasher_Verify(b *testing.B) {
	hasher := NewHasher(nil)
	password := "testPassword123!"
	hash, _ := hasher.Hash(password)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = hasher.Verify(password, hash)
	}
}
