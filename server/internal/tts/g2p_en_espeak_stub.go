//go:build !espeak || windows

package tts

// espeakFallback stub — eSpeak not compiled in. Returns empty string.
func espeakFallback(_ string) string { return "" }
