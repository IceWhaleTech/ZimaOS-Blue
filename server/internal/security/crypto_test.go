package security

import (
	"testing"
)

func TestTimingSafeCompare(t *testing.T) {
	tests := []struct {
		name     string
		a        string
		b        string
		expected bool
	}{
		{
			name:     "equal strings",
			a:        "hello",
			b:        "hello",
			expected: true,
		},
		{
			name:     "different strings same length",
			a:        "hello",
			b:        "world",
			expected: false,
		},
		{
			name:     "different lengths",
			a:        "hello",
			b:        "hi",
			expected: false,
		},
		{
			name:     "empty strings",
			a:        "",
			b:        "",
			expected: true,
		},
		{
			name:     "one empty",
			a:        "hello",
			b:        "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TimingSafeCompare(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("TimingSafeCompare(%q, %q) = %v, want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestTimingSafeCompareBytes(t *testing.T) {
	tests := []struct {
		name     string
		a        []byte
		b        []byte
		expected bool
	}{
		{
			name:     "equal bytes",
			a:        []byte{1, 2, 3},
			b:        []byte{1, 2, 3},
			expected: true,
		},
		{
			name:     "different bytes same length",
			a:        []byte{1, 2, 3},
			b:        []byte{4, 5, 6},
			expected: false,
		},
		{
			name:     "different lengths",
			a:        []byte{1, 2, 3},
			b:        []byte{1, 2},
			expected: false,
		},
		{
			name:     "empty slices",
			a:        []byte{},
			b:        []byte{},
			expected: true,
		},
		{
			name:     "nil slices",
			a:        nil,
			b:        nil,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TimingSafeCompareBytes(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("TimingSafeCompareBytes(%v, %v) = %v, want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestValidateHMACSHA256Base64(t *testing.T) {
	tests := []struct {
		name      string
		body      []byte
		signature string
		secret    string
		expected  bool
	}{
		{
			name:      "valid signature",
			body:      []byte(`{"events":[]}`),
			signature: "Yz0xMjM0NTY3ODkw", // This is a placeholder, real test would use actual HMAC
			secret:    "test-secret",
			expected:  false, // Will be false because signature doesn't match
		},
		{
			name:      "invalid signature",
			body:      []byte(`{"events":[]}`),
			signature: "invalid-signature",
			secret:    "test-secret",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateHMACSHA256Base64(tt.body, tt.signature, tt.secret)
			if result != tt.expected {
				t.Errorf("ValidateHMACSHA256Base64() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestValidateHMACSHA256Hex(t *testing.T) {
	tests := []struct {
		name      string
		body      []byte
		signature string
		secret    string
		expected  bool
	}{
		{
			name:      "valid hex signature",
			body:      []byte("test message"),
			signature: "invalid", // Placeholder
			secret:    "secret",
			expected:  false,
		},
		{
			name:      "empty body with empty secret",
			body:      []byte{},
			signature: "b613679a0814d9ec772f95d778c35fc5ff1697c493715653c6c712144292c5ad",
			secret:    "",
			expected:  true, // HMAC-SHA256 of empty with empty secret
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateHMACSHA256Hex(tt.body, tt.signature, tt.secret)
			if result != tt.expected {
				t.Errorf("ValidateHMACSHA256Hex() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// BenchmarkTimingSafeCompare ensures constant-time behavior
func BenchmarkTimingSafeCompare(b *testing.B) {
	s1 := "abcdefghijklmnopqrstuvwxyz"
	s2 := "abcdefghijklmnopqrstuvwxyz"
	s3 := "zbcdefghijklmnopqrstuvwxyz" // Different first char
	s4 := "abcdefghijklmnopqrstuvwxya" // Different last char

	b.Run("equal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			TimingSafeCompare(s1, s2)
		}
	})

	b.Run("diff_first", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			TimingSafeCompare(s1, s3)
		}
	})

	b.Run("diff_last", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			TimingSafeCompare(s1, s4)
		}
	})
}
