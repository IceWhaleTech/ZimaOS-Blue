//go:build !espeak

package tts

// espeakFallback stub — eSpeak not compiled in. Returns empty string.
func espeakFallback(_ string) string { return "" }
