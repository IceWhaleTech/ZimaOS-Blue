package tools

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
)

// TTSVoiceInfo is the normalized voice descriptor exposed by the tts tool.
type TTSVoiceInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Language    string `json:"language,omitempty"`
	Gender      string `json:"gender,omitempty"`
	Description string `json:"description,omitempty"`
	Provider    string `json:"provider,omitempty"`
	Quality     string `json:"quality,omitempty"`
}

// TTSSynthesizeRequest is the normalized synthesis request used by the tts tool.
type TTSSynthesizeRequest struct {
	Text     string
	Format   string
	Voice    string
	Provider string
	Speed    *float32
	Pitch    *float32
	Volume   *float32
}

// TTSAudioResult is the normalized synthesis response used by the tts tool.
type TTSAudioResult struct {
	Audio           []byte
	Format          string
	ContentType     string
	DurationSeconds float64
	Provider        string
	Voice           string
}

// TTSBackend exposes the speech capabilities used by the tts tool.
type TTSBackend interface {
	Status(ctx context.Context) interface{}
	Config(ctx context.Context) map[string]interface{}
	ListVoices(ctx context.Context) ([]TTSVoiceInfo, error)
	Synthesize(ctx context.Context, req TTSSynthesizeRequest) (*TTSAudioResult, error)
	SpeakLocally(ctx context.Context, text string) (bool, error)
	StopSpeaking(ctx context.Context) error
}

// TTSTool provides a native compatibility surface for legacy text-to-speech usage.
type TTSTool struct {
	backend TTSBackend
}

// NewTTSTool creates a new native tts tool.
func NewTTSTool(backend TTSBackend) *TTSTool {
	return &TTSTool{backend: backend}
}

// Definition returns the tool definition.
func (t *TTSTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "tts",
		Description: "Generate speech audio, play speech locally when supported, inspect TTS status/config, list voices, and stop active local playback.",
		Icon:        "volume-2",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"synthesize", "speak", "say", "generate", "voices", "list", "status", "get", "config", "stop", "cancel"},
					"description": "Action to perform. Defaults to synthesize when text is provided, otherwise status.",
				},
				"text":           map[string]interface{}{"type": "string", "description": "Text to synthesize or speak."},
				"input":          map[string]interface{}{"type": "string", "description": "Alias for text."},
				"content":        map[string]interface{}{"type": "string", "description": "Alias for text."},
				"message":        map[string]interface{}{"type": "string", "description": "Alias for text."},
				"prompt":         map[string]interface{}{"type": "string", "description": "Alias for text."},
				"format":         map[string]interface{}{"type": "string", "description": "Output audio format: wav, mp3, opus, aac, flac, or pcm."},
				"audio_format":   map[string]interface{}{"type": "string", "description": "Alias for format."},
				"voice":          map[string]interface{}{"type": "string", "description": "Optional voice ID to use for synthesis."},
				"provider":       map[string]interface{}{"type": "string", "description": "Optional TTS provider override, e.g. edge-tts, espeak-ng, macos-native."},
				"speed":          map[string]interface{}{"type": "number", "description": "Optional speech speed override."},
				"pitch":          map[string]interface{}{"type": "number", "description": "Optional speech pitch override."},
				"volume":         map[string]interface{}{"type": "number", "description": "Optional speech volume override."},
				"play_local":     map[string]interface{}{"type": "boolean", "description": "Prefer local playback when supported. Defaults to true for speak/say."},
				"include_base64": map[string]interface{}{"type": "boolean", "description": "Include base64 audio in the response. Disabled by default to keep payloads small."},
				"limit":          map[string]interface{}{"type": "integer", "description": "Optional limit for voices/list output."},
			},
			"additionalProperties": true,
		},
	}
}

// Execute dispatches the requested tts action.
func (t *TTSTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil || t.backend == nil {
		return nil, errors.New("tts service not available")
	}
	switch ttsAction(args) {
	case "status":
		return t.executeStatus(ctx)
	case "config":
		return t.executeConfig(ctx)
	case "voices":
		return t.executeVoices(ctx, args)
	case "stop":
		return t.executeStop(ctx)
	case "speak":
		return t.executeSynthesize(ctx, args, true)
	case "synthesize":
		return t.executeSynthesize(ctx, args, false)
	default:
		return nil, errors.New("unsupported tts action")
	}
}

func (t *TTSTool) executeStatus(ctx context.Context) (interface{}, error) {
	config := t.backend.Config(ctx)
	if config == nil {
		config = map[string]interface{}{}
	}
	return map[string]interface{}{
		"status":            t.backend.Status(ctx),
		"config":            config,
		"supported_actions": []string{"synthesize", "speak", "voices", "status", "config", "stop"},
	}, nil
}

func (t *TTSTool) executeConfig(ctx context.Context) (interface{}, error) {
	config := t.backend.Config(ctx)
	if config == nil {
		config = map[string]interface{}{}
	}
	config["supported_actions"] = []string{"synthesize", "speak", "voices", "status", "config", "stop"}
	return config, nil
}

func (t *TTSTool) executeVoices(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	voices, err := t.backend.ListVoices(ctx)
	if err != nil {
		return nil, err
	}
	count := len(voices)
	if limit := compatInt(args, "limit", "max_results", "n"); limit > 0 && limit < len(voices) {
		voices = voices[:limit]
	}
	return map[string]interface{}{
		"voices": voices,
		"count":  len(voices),
		"total":  count,
	}, nil
}

func (t *TTSTool) executeStop(ctx context.Context) (interface{}, error) {
	if err := t.backend.StopSpeaking(ctx); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"success": true,
		"stopped": true,
	}, nil
}

func (t *TTSTool) executeSynthesize(ctx context.Context, args map[string]interface{}, preferLocal bool) (interface{}, error) {
	text := firstCompatString(args, "text", "input", "content", "message", "prompt")
	if text == "" {
		return nil, errors.New("text/input is required")
	}

	req, err := normalizeTTSSynthesizeRequest(args)
	if err != nil {
		return nil, err
	}
	if req.Text == "" {
		req.Text = text
	}

	playLocal, playLocalSet := compatBoolInArgs(args, "play_local", "playLocal")
	useLocal := preferLocal
	if playLocalSet {
		useLocal = playLocal
	}
	if useLocal && canLocalSpeak(req) {
		supported, err := t.backend.SpeakLocally(ctx, req.Text)
		if err != nil {
			return nil, err
		}
		if supported {
			return map[string]interface{}{
				"success":        true,
				"played_locally": true,
				"text":           clipTTSText(req.Text, 160),
			}, nil
		}
	}

	result, err := t.backend.Synthesize(ctx, req)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, errors.New("tts synthesis returned no result")
	}
	format := strings.ToLower(strings.TrimSpace(result.Format))
	if format == "" {
		format = req.Format
	}
	if format == "" {
		format = "wav"
	}
	path, fileURL, err := writeTTSAudioFile(result.Audio, format, result.ContentType)
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"success":          true,
		"played_locally":   false,
		"text":             clipTTSText(req.Text, 160),
		"format":           format,
		"content_type":     result.ContentType,
		"size_bytes":       len(result.Audio),
		"duration_seconds": result.DurationSeconds,
		"path":             path,
		"file_url":         fileURL,
	}
	if result.Provider != "" {
		payload["provider"] = result.Provider
	}
	if result.Voice != "" {
		payload["voice"] = result.Voice
	} else if req.Voice != "" {
		payload["voice"] = req.Voice
	}
	if includeBase64, ok := compatBoolInArgs(args, "include_base64", "includeBase64"); ok && includeBase64 {
		payload["audio_base64"] = base64.StdEncoding.EncodeToString(result.Audio)
	}
	return payload, nil
}

func ttsAction(args map[string]interface{}) string {
	action := strings.ToLower(strings.TrimSpace(firstCompatString(args, "action", "op", "operation", "command")))
	if action == "" {
		if firstCompatString(args, "text", "input", "content", "message", "prompt") != "" {
			if playLocal, ok := compatBoolInArgs(args, "play_local", "playLocal"); ok && playLocal {
				return "speak"
			}
			return "synthesize"
		}
		return "status"
	}
	switch action {
	case "say", "speak":
		return "speak"
	case "synthesize", "generate":
		return "synthesize"
	case "voices", "list":
		return "voices"
	case "status", "get":
		return "status"
	case "config":
		return "config"
	case "stop", "cancel":
		return "stop"
	default:
		return action
	}
}

func normalizeTTSSynthesizeRequest(args map[string]interface{}) (TTSSynthesizeRequest, error) {
	format, err := normalizeTTSAudioFormat(firstCompatString(args, "format", "audio_format", "audioFormat"))
	if err != nil {
		return TTSSynthesizeRequest{}, err
	}
	return TTSSynthesizeRequest{
		Text:     firstCompatString(args, "text", "input", "content", "message", "prompt"),
		Format:   format,
		Voice:    firstCompatString(args, "voice"),
		Provider: normalizeTTSProvider(firstCompatString(args, "provider")),
		Speed:    compatFloat32Ptr(args, "speed"),
		Pitch:    compatFloat32Ptr(args, "pitch"),
		Volume:   compatFloat32Ptr(args, "volume"),
	}, nil
}

func normalizeTTSAudioFormat(value string) (string, error) {
	format := strings.ToLower(strings.TrimSpace(value))
	if format == "" {
		return "wav", nil
	}
	switch format {
	case "wav", "mp3", "opus", "aac", "flac", "pcm":
		return format, nil
	default:
		return "", fmt.Errorf("unsupported audio format %q", value)
	}
}

func normalizeTTSProvider(value string) string {
	provider := strings.ToLower(strings.TrimSpace(value))
	switch provider {
	case "", "edge-tts", "openai", "elevenlabs", "piper", "kokoro", "sherpa", "espeak-ng", "macos-native", "windows-native":
		return provider
	case "edge":
		return "edge-tts"
	case "espeak":
		return "espeak-ng"
	case "macos", "macos_native":
		return "macos-native"
	case "windows", "windows_native":
		return "windows-native"
	default:
		return provider
	}
}

func compatFloat32Ptr(args map[string]interface{}, keys ...string) *float32 {
	value, ok := firstCompatValueDeep(args, keys...)
	if !ok || value == nil {
		return nil
	}
	switch typed := value.(type) {
	case float32:
		v := typed
		return &v
	case float64:
		v := float32(typed)
		return &v
	case int:
		v := float32(typed)
		return &v
	case int64:
		v := float32(typed)
		return &v
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return nil
		}
		var parsed float64
		if _, err := fmt.Sscanf(trimmed, "%f", &parsed); err == nil {
			v := float32(parsed)
			return &v
		}
	}
	return nil
}

func compatBoolInArgs(args map[string]interface{}, keys ...string) (bool, bool) {
	value, ok := firstCompatValueDeep(args, keys...)
	if !ok {
		return false, false
	}
	return asCompatBool(value)
}

func canLocalSpeak(req TTSSynthesizeRequest) bool {
	return req.Provider == "" && req.Voice == "" && req.Speed == nil && req.Pitch == nil && req.Volume == nil
}

func clipTTSText(text string, maxRunes int) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || maxRunes <= 0 {
		return trimmed
	}
	runes := []rune(trimmed)
	if len(runes) <= maxRunes {
		return trimmed
	}
	return string(runes[:maxRunes]) + "…"
}

func writeTTSAudioFile(audio []byte, format, contentType string) (string, string, error) {
	ext := ttsAudioExt(format, contentType)
	pattern := "zimaos-blue-tts-*"
	if ext != "" {
		pattern += "." + ext
	}
	file, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", "", fmt.Errorf("create temp audio file: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(audio); err != nil {
		return "", "", fmt.Errorf("write temp audio file: %w", err)
	}
	path := file.Name()
	fileURL := (&url.URL{Scheme: "file", Path: path}).String()
	return path, fileURL, nil
}

func ttsAudioExt(format, contentType string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "wav":
		return "wav"
	case "mp3":
		return "mp3"
	case "opus":
		return "opus"
	case "aac":
		return "aac"
	case "flac":
		return "flac"
	case "pcm":
		return "pcm"
	}
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "audio/wav", "audio/x-wav":
		return "wav"
	case "audio/mpeg", "audio/mp3":
		return "mp3"
	case "audio/opus":
		return "opus"
	case "audio/aac":
		return "aac"
	case "audio/flac":
		return "flac"
	case "audio/pcm", "audio/l16":
		return "pcm"
	default:
		return "bin"
	}
}

// RegisterTTSTool registers the native tts compatibility tool.
func RegisterTTSTool(registry *Registry, backend TTSBackend) {
	if registry == nil || backend == nil {
		return
	}
	registry.Register(NewTTSTool(backend))
}
