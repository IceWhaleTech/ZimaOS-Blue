package windows

import "io"

// Voice represents a Windows TTS voice
type Voice struct {
	ID       string
	Name     string
	Language string
	Gender   string
}

// SynthesizeRequest represents a TTS synthesis request
type SynthesizeRequest struct {
	Text   string
	Voice  string
	Speed  float32
	Pitch  float32
	Volume float32
}

// SynthesizeResponse represents a TTS synthesis response
type SynthesizeResponse struct {
	Audio       []byte
	ContentType string
	SampleRate  int
}

// RecognizeRequest represents an ASR recognition request
type RecognizeRequest struct {
	Audio      io.Reader
	Language   string
	SampleRate int
}

// RecognizeResponse represents an ASR recognition response
type RecognizeResponse struct {
	Text       string
	Confidence float32
	Language   string
}
