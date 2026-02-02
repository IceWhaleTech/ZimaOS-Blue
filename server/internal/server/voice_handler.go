package server

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/tts"
)

// SynthesizeRequest represents a TTS synthesis request
type SynthesizeRequest struct {
	Text     string  `json:"text"`
	Provider string  `json:"provider,omitempty"` // espeak-ng, edge-tts, sherpa-onnx
	Voice    string  `json:"voice,omitempty"`
	Rate     float32 `json:"rate,omitempty"`
	Pitch    float32 `json:"pitch,omitempty"`
	Volume   float32 `json:"volume,omitempty"`
}

// HandleSynthesize handles POST /api/v1/voice/synthesize
func HandleSynthesize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SynthesizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Text == "" {
		http.Error(w, "Text is required", http.StatusBadRequest)
		return
	}

	// Default to eSpeak-NG if no provider specified
	if req.Provider == "" {
		req.Provider = "espeak-ng"
	}

	// Create provider based on request
	var audio io.ReadCloser
	var err error

	switch req.Provider {
	case "espeak-ng":
		provider := tts.NewEspeakNGProvider("")
		provider.MarkLanguageAvailable("en", true)
		audio, err = provider.Synthesize(r.Context(), &tts.EspeakNGRequest{
			Text:     req.Text,
			Language: "en",
			Rate:     150,
			Pitch:    50,
			Volume:   100,
		})

	case "edge-tts":
		// TODO: Implement Edge-TTS
		http.Error(w, "Edge-TTS not yet implemented", http.StatusNotImplemented)
		return

	case "sherpa-onnx":
		// TODO: Implement Sherpa-ONNX
		http.Error(w, "Sherpa-ONNX not yet implemented", http.StatusNotImplemented)
		return

	default:
		http.Error(w, "Unknown provider: "+req.Provider, http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, "Synthesis failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	defer audio.Close()

	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("Content-Disposition", "inline; filename=speech.wav")

	if _, err := io.Copy(w, audio); err != nil {
		http.Error(w, "Failed to write audio", http.StatusInternalServerError)
		return
	}
}
