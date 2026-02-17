// Package humanizer transforms LLM Markdown output into natural text
// suitable for IM channels and voice/TTS output.
package humanizer

// Mode determines the level of text transformation.
type Mode int

const (
	// ModeIM applies light cleanup for IM channels (Telegram, Discord, etc.).
	// Preserves structure but strips Markdown syntax.
	ModeIM Mode = iota
	// ModeVoice applies aggressive cleanup for TTS engines.
	// Removes all formatting, code blocks, and emojis.
	ModeVoice
)

// Config holds humanizer configuration.
type Config struct {
	Enabled      bool `yaml:"enabled" yaml:"enabled"`
	IMEnabled    bool `yaml:"im_enabled" yaml:"im_enabled"`
	VoiceEnabled bool `yaml:"voice_enabled" yaml:"voice_enabled"`
}

// DefaultConfig returns the default configuration with all modes enabled.
func DefaultConfig() Config {
	return Config{
		Enabled:      true,
		IMEnabled:    true,
		VoiceEnabled: true,
	}
}

// Humanize transforms Markdown-formatted LLM output into natural text.
// Rules are applied in a deterministic order. Safe for concurrent use.
func Humanize(text string, mode Mode) string {
	if text == "" {
		return ""
	}

	// Order matters: typeless cards first (before code fences strip their markers),
	// then code fences, then block-level, then inline, then cleanup.
	text = stripTypelessCards(text, mode)
	text = stripCodeFences(text, mode)
	text = stripImages(text, mode)
	text = stripLinks(text, mode)
	text = stripHeaders(text)
	text = stripHorizontalRules(text)
	text = stripBlockquotes(text)
	text = stripBold(text)
	text = stripItalic(text)
	text = stripStrikethrough(text)
	text = stripInlineCode(text)
	text = stripHTMLTags(text)
	text = normalizeBullets(text, mode)

	if mode == ModeVoice {
		text = stripEmojis(text)
	}

	text = normalizeWhitespace(text)

	return text
}
