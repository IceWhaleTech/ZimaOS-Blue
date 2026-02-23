package proxy

import "unsafe"

// FNV-1a constants for fast, non-cryptographic hashing.
const (
	fnvOffset64 uint64 = 14695981039346656037
	fnvPrime64  uint64 = 1099511628211
)

// unsafeString converts a byte slice to string without copying.
// The caller MUST ensure the bytes are not modified while the string is in use.
func unsafeString(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}

// getFloat extracts a float64 from a map, returning 0 if the key is missing or not a float.
func getFloat(m map[string]interface{}, key string) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return 0
}
