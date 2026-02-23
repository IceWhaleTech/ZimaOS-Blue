// Package humanizer transforms LLM Markdown output into formatted text
// suitable for various IM channels, voice/TTS output, and web display.
//
// Architecture: Markdown → IR (intermediate representation) → platform-specific rendering.
// The IR preserves style spans (bold, italic, code, etc.) and link spans,
// enabling one parse pass with multiple render targets.
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
	Enabled      bool `yaml:"enabled"`
	IMEnabled    bool `yaml:"im_enabled"`
	VoiceEnabled bool `yaml:"voice_enabled"`
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
// Uses the IR pipeline: Parse → RenderPlain/RenderVoice.
// Safe for concurrent use.
func Humanize(text string, mode Mode) string {
	if text == "" {
		return ""
	}

	// Pre-process: strip non-standard elements that the IR parser doesn't handle
	text = stripTypelessCards(text, mode)
	text = stripFunctionCalls(text, mode)
	text = stripHTMLTags(text)
	if mode == ModeVoice {
		text = stripMathBlocks(text)
	}

	ir := Parse(text, ParseOptions{
		HeadingStyle: "none",
		TableMode:    "off",
	})

	switch mode {
	case ModeVoice:
		return RenderVoice(ir)
	default:
		return RenderPlain(ir)
	}
}

// HumanizeForChannel transforms Markdown for a specific channel type.
// Returns the formatted text and the recommended format string for the channel.
func HumanizeForChannel(text string, channelType string) (content string, format string) {
	if text == "" {
		return "", ""
	}

	// Pre-process non-standard elements
	text = stripTypelessCards(text, ModeIM)
	text = stripFunctionCalls(text, ModeIM)
	text = stripHTMLTags(text)

	ir := Parse(text, ParseOptions{
		HeadingStyle:     "bold",
		BlockquotePrefix: "",
		TableMode:        "bullets",
	})

	switch channelType {
	case "discord":
		return RenderDiscord(ir), ""
	case "telegram":
		return RenderTelegram(ir), "html"
	case "slack":
		return RenderSlack(ir), ""
	case "matrix":
		return RenderMatrix(ir), "html"
	default:
		return RenderPlain(ir), ""
	}
}
