package providerpool

import "strings"

// SupportsChatCompletionsModelID returns whether the model identifier looks
// suitable for chat-completions style requests. Unknown model IDs default to
// true so custom relays still work, but clearly non-chat families are rejected.
func SupportsChatCompletionsModelID(modelID string) bool {
	return !isLikelyNonTextGenerationModelID(modelID)
}

// SupportsChatCompletions reports whether a discovered model should be used for
// chat-completions style routing.
func SupportsChatCompletions(model *Model) bool {
	if model == nil || !model.Enabled {
		return false
	}
	if !SupportsChatCompletionsModelID(preferredModelID(model)) {
		return false
	}

	caps := model.Capabilities
	if caps.ImageGeneration || caps.VideoGeneration || caps.AudioGeneration {
		return false
	}

	if caps.Chat || caps.Completion || caps.FunctionCall || caps.Streaming || caps.SystemPrompt || caps.JSON || caps.Thinking || caps.Vision {
		return true
	}

	// Treat unknown relay models as chat-capable unless they match a clear
	// non-chat heuristic above.
	return true
}

func applyDiscoveredModelTypeHeuristics(model *Model) {
	if model == nil {
		return
	}
	modelID := preferredModelID(model)
	if !isLikelyNonTextGenerationModelID(modelID) {
		return
	}

	model.Capabilities.Chat = false
	model.Capabilities.Completion = false
	model.Capabilities.Vision = false
	model.Capabilities.FunctionCall = false
	model.Capabilities.Streaming = false
	model.Capabilities.Thinking = false
	model.Capabilities.JSON = false
	model.Capabilities.SystemPrompt = false

	switch {
	case isLikelyImageGenerationModelID(modelID):
		model.Capabilities.ImageGeneration = true
	case isLikelyAudioGenerationModelID(modelID):
		model.Capabilities.AudioGeneration = true
	}
}

func isLikelyNonTextGenerationModelID(modelID string) bool {
	id := strings.ToLower(strings.TrimSpace(modelID))
	if id == "" {
		return false
	}

	switch {
	case strings.Contains(id, "embedding"),
		strings.HasPrefix(id, "bge-"),
		strings.Contains(id, "semantic_similarity"),
		strings.Contains(id, "rerank"),
		strings.Contains(id, "moderation"),
		strings.HasPrefix(id, "whisper"),
		strings.HasPrefix(id, "tts-"),
		strings.Contains(id, "transcribe"),
		strings.Contains(id, "transcription"),
		strings.Contains(id, "dall-e"),
		strings.Contains(id, "gpt-image"),
		strings.HasPrefix(id, "mj_"),
		strings.HasPrefix(id, "mj-"),
		strings.Contains(id, "imagen"),
		strings.Contains(id, "swap_face"),
		strings.HasPrefix(id, "text-ada-"),
		strings.HasPrefix(id, "text-babbage-"),
		strings.HasPrefix(id, "text-curie-"),
		strings.HasPrefix(id, "text-davinci-"),
		id == "davinci-002",
		id == "babbage-002":
		return true
	default:
		return false
	}
}

func isLikelyImageGenerationModelID(modelID string) bool {
	id := strings.ToLower(strings.TrimSpace(modelID))
	return strings.Contains(id, "dall-e") ||
		strings.Contains(id, "gpt-image") ||
		strings.HasPrefix(id, "mj_") ||
		strings.HasPrefix(id, "mj-") ||
		strings.Contains(id, "imagen") ||
		strings.Contains(id, "swap_face")
}

func isLikelyAudioGenerationModelID(modelID string) bool {
	id := strings.ToLower(strings.TrimSpace(modelID))
	return strings.HasPrefix(id, "tts-")
}
