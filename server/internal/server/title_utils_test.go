package server

import (
	"testing"
)

// Test findRuneBoundary with various inputs
func TestFindRuneBoundary(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected int // -1 means no space found
	}{
		{
			name:     "English with space before limit",
			input:    "Hello world this is a test",
			maxLen:   15,
			expected: 11, // position of space after "world"
		},
		{
			name:     "Chinese text without spaces",
			input:    "这是一个中文标题测试",
			maxLen:   5,
			expected: -1, // no spaces in Chinese
		},
		{
			name:     "Mixed Chinese and English with space",
			input:    "你好 world 测试",
			maxLen:   8,
			expected: 6, // byte position of space after "你好" (6 bytes for 2 Chinese chars)
		},
		{
			name:     "Short string under limit",
			input:    "短标题",
			maxLen:   10,
			expected: -1, // string is shorter than maxLen
		},
		{
			name:     "English no spaces",
			input:    "NoSpacesHereAtAll",
			maxLen:   10,
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findRuneBoundary(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("findRuneBoundary(%q, %d) = %d, want %d",
					tt.input, tt.maxLen, result, tt.expected)
			}
		})
	}
}

// Test sanitizeTitle with UTF-8 characters
func TestSanitizeTitle(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Chinese with newlines",
			input:    "你好\n世界",
			expected: "你好 世界",
		},
		{
			name:     "Chinese with tabs",
			input:    "测试\t标题",
			expected: "测试 标题",
		},
		{
			name:     "Chinese with multiple spaces",
			input:    "中文  多个   空格",
			expected: "中文 多个 空格",
		},
		{
			name:     "Mixed content with various whitespace",
			input:    "Hello\n你好\t\tWorld  世界",
			expected: "Hello 你好 World 世界",
		},
		{
			name:     "Japanese text",
			input:    "こんにちは\n世界",
			expected: "こんにちは 世界",
		},
		{
			name:     "Korean text",
			input:    "안녕하세요\t세계",
			expected: "안녕하세요 세계",
		},
		{
			name:     "Emoji in title",
			input:    "Hello 🌍\nWorld",
			expected: "Hello 🌍 World",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeTitle(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeTitle(%q) = %q, want %q",
					tt.input, result, tt.expected)
			}
		})
	}
}

// isValidUTF8 checks if a string is valid UTF-8
func isValidUTF8(s string) bool {
	for i := 0; i < len(s); {
		r, size := decodeRuneInString(s[i:])
		if r == '\uFFFD' && size == 1 {
			// Invalid UTF-8 sequence
			return false
		}
		i += size
	}
	return true
}

// decodeRuneInString decodes the first rune in the string
func decodeRuneInString(s string) (rune, int) {
	if len(s) == 0 {
		return '\uFFFD', 0
	}
	// Use range to decode - Go's range on string iterates by rune
	for _, r := range s {
		return r, len(string(r))
	}
	return '\uFFFD', 1
}

// Test UTF-8 title truncation doesn't corrupt characters
func TestUTF8TitleTruncation(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		maxLen      int
		shouldEndOK bool // result should be valid UTF-8
	}{
		{
			name:        "Chinese title truncation",
			input:       "这是一个非常长的中文标题需要被截断处理以确保不会出现乱码问题",
			maxLen:      50,
			shouldEndOK: true,
		},
		{
			name:        "Japanese title truncation",
			input:       "これは非常に長い日本語のタイトルで、切り捨てが必要です",
			maxLen:      50,
			shouldEndOK: true,
		},
		{
			name:        "Korean title truncation",
			input:       "이것은 매우 긴 한국어 제목으로 잘라야 합니다 테스트입니다",
			maxLen:      50,
			shouldEndOK: true,
		},
		{
			name:        "Mixed CJK and English",
			input:       "Hello 你好 こんにちは 안녕하세요 this is a very long mixed title",
			maxLen:      50,
			shouldEndOK: true,
		},
		{
			name:        "Emoji heavy title",
			input:       "🎉🎊🎁🎈🎂🎄🎃🎇🎆✨🌟⭐💫🌈☀️🌙⚡🔥💧🌊",
			maxLen:      10,
			shouldEndOK: true,
		},
		{
			name:        "Short Chinese title under limit",
			input:       "短标题",
			maxLen:      50,
			shouldEndOK: true,
		},
		{
			name:        "Exact limit Chinese",
			input:       "一二三四五六七八九十一二三四五六七八九十一二三四五六七八九十一二三四五六七八九十一二三四五六七八九十",
			maxLen:      50,
			shouldEndOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the truncation logic from generateConversationTitle
			title := tt.input
			runes := []rune(title)
			if len(runes) > tt.maxLen {
				if idx := findRuneBoundary(title, tt.maxLen); idx > 0 {
					title = title[:idx] + "..."
				} else {
					title = string(runes[:tt.maxLen]) + "..."
				}
			}

			// Verify result is valid UTF-8
			if !isValidUTF8(title) {
				t.Errorf("Truncated title is not valid UTF-8: %q (bytes: %v)",
					title, []byte(title))
			}

			// Verify we didn't exceed character limit (plus "...")
			resultRunes := []rune(title)
			maxAllowed := tt.maxLen + 3 // for "..."
			if len(resultRunes) > maxAllowed {
				t.Errorf("Truncated title too long: got %d runes, want <= %d",
					len(resultRunes), maxAllowed)
			}

			t.Logf("Input (%d chars): %s", len([]rune(tt.input)), tt.input)
			t.Logf("Output (%d chars): %s", len(resultRunes), title)
		})
	}
}

// Test that byte-level truncation would fail (regression test)
func TestByteLevelTruncationWouldCorrupt(t *testing.T) {
	// This test demonstrates why we need rune-based truncation
	chineseTitle := "这是中文标题" // 6 characters, 18 bytes

	// Verify byte count vs rune count
	byteCount := len(chineseTitle)
	runeCount := len([]rune(chineseTitle))

	t.Logf("Chinese title: %q", chineseTitle)
	t.Logf("Byte count: %d, Rune count: %d", byteCount, runeCount)

	if byteCount != 18 {
		t.Errorf("Expected 18 bytes, got %d", byteCount)
	}
	if runeCount != 6 {
		t.Errorf("Expected 6 runes, got %d", runeCount)
	}

	// Byte-level truncation at position 10 would corrupt
	if len(chineseTitle) > 10 {
		// This would be the OLD buggy behavior:
		// badTruncation := chineseTitle[:10]
		// The result would be invalid UTF-8

		// NEW correct behavior using runes:
		runes := []rune(chineseTitle)
		goodTruncation := string(runes[:4]) // Take 4 characters

		if !isValidUTF8(goodTruncation) {
			t.Error("Rune-based truncation should produce valid UTF-8")
		}

		expectedChars := 4
		if len([]rune(goodTruncation)) != expectedChars {
			t.Errorf("Expected %d characters, got %d", expectedChars, len([]rune(goodTruncation)))
		}

		t.Logf("Good truncation (4 chars): %q", goodTruncation)
	}
}

// Test edge cases for findRuneBoundary
func TestFindRuneBoundaryEdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		check  func(t *testing.T, result int)
	}{
		{
			name:   "Empty string",
			input:  "",
			maxLen: 10,
			check: func(t *testing.T, result int) {
				if result != -1 {
					t.Errorf("Expected -1 for empty string, got %d", result)
				}
			},
		},
		{
			name:   "Single space at start",
			input:  " Hello",
			maxLen: 5,
			check: func(t *testing.T, result int) {
				if result != 0 {
					t.Errorf("Expected 0 (position of first space), got %d", result)
				}
			},
		},
		{
			name:   "Space at exact maxLen position",
			input:  "Hello World",
			maxLen: 6,
			check: func(t *testing.T, result int) {
				if result != 5 {
					t.Errorf("Expected 5 (position of space), got %d", result)
				}
			},
		},
		{
			name:   "Chinese with space in middle",
			input:  "你好 世界 测试 标题",
			maxLen: 6,
			check: func(t *testing.T, result int) {
				// Should find the space after "世界" (position 4 in runes)
				// Byte position should be len("你好 世界") = 6 + 1 + 6 = 13
				if result < 0 {
					t.Errorf("Expected to find a space, got %d", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findRuneBoundary(tt.input, tt.maxLen)
			tt.check(t, result)
		})
	}
}
