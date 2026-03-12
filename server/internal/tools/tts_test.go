package tools

import (
	"context"
	"os"
	"testing"
)

type mockTTSBackend struct {
	status         interface{}
	config         map[string]interface{}
	voices         []TTSVoiceInfo
	synthResult    *TTSAudioResult
	synthErr       error
	lastSynthReq   TTSSynthesizeRequest
	synthCalls     int
	speakSupported bool
	speakErr       error
	speakCalls     int
	stopCalled     bool
}

func (m *mockTTSBackend) Status(context.Context) interface{} {
	if m.status != nil {
		return m.status
	}
	return map[string]interface{}{"tts": map[string]interface{}{"ready": true}}
}

func (m *mockTTSBackend) Config(context.Context) map[string]interface{} {
	if m.config != nil {
		return m.config
	}
	return map[string]interface{}{"speed": 1.0, "pitch": 0.0, "volume": 100.0, "provider": "edge-tts"}
}

func (m *mockTTSBackend) ListVoices(context.Context) ([]TTSVoiceInfo, error) {
	return m.voices, nil
}

func (m *mockTTSBackend) Synthesize(_ context.Context, req TTSSynthesizeRequest) (*TTSAudioResult, error) {
	m.lastSynthReq = req
	m.synthCalls++
	if m.synthErr != nil {
		return nil, m.synthErr
	}
	if m.synthResult != nil {
		return m.synthResult, nil
	}
	return &TTSAudioResult{Audio: []byte("audio"), Format: "wav", ContentType: "audio/wav", DurationSeconds: 1.2, Provider: "edge-tts"}, nil
}

func (m *mockTTSBackend) SpeakLocally(_ context.Context, text string) (bool, error) {
	_ = text
	m.speakCalls++
	if m.speakErr != nil {
		return false, m.speakErr
	}
	return m.speakSupported, nil
}

func (m *mockTTSBackend) StopSpeaking(context.Context) error {
	m.stopCalled = true
	return nil
}

func TestTTSToolSynthesizeWritesTempFile(t *testing.T) {
	backend := &mockTTSBackend{
		synthResult: &TTSAudioResult{Audio: []byte("abc123"), Format: "mp3", ContentType: "audio/mpeg", DurationSeconds: 2.5, Provider: "edge-tts", Voice: "alloy"},
	}
	tool := NewTTSTool(backend)

	result, err := tool.Execute(context.Background(), map[string]interface{}{"text": "hello world", "provider": "edge", "include_base64": true})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("result type = %T, want map", result)
	}
	path, _ := payload["path"].(string)
	if path == "" {
		t.Fatal("expected synthesized audio path")
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected synthesized file to exist: %v", err)
	}
	if got := backend.lastSynthReq.Provider; got != "edge-tts" {
		t.Fatalf("provider = %q, want edge-tts", got)
	}
	if got := payload["format"]; got != "mp3" {
		t.Fatalf("format = %v, want mp3", got)
	}
	if got := payload["voice"]; got != "alloy" {
		t.Fatalf("voice = %v, want alloy", got)
	}
	if _, ok := payload["audio_base64"].(string); !ok {
		t.Fatalf("expected audio_base64 in payload, got %#v", payload["audio_base64"])
	}
	if got := payload["played_locally"]; got != false {
		t.Fatalf("played_locally = %v, want false", got)
	}
}

func TestTTSToolSynthesizeSupportsNestedCamelCaseArgs(t *testing.T) {
	backend := &mockTTSBackend{
		synthResult: &TTSAudioResult{Audio: []byte("abc123"), Format: "mp3", ContentType: "audio/mpeg", DurationSeconds: 2.5, Provider: "edge-tts", Voice: "alloy"},
	}
	tool := NewTTSTool(backend)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": map[string]interface{}{
			"text":          "hello world",
			"provider":      "edge",
			"audioFormat":   "mp3",
			"includeBase64": true,
			"speed":         1.25,
		},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("result type = %T, want map", result)
	}
	if got := backend.lastSynthReq.Provider; got != "edge-tts" {
		t.Fatalf("provider = %q, want edge-tts", got)
	}
	if got := backend.lastSynthReq.Format; got != "mp3" {
		t.Fatalf("format = %q, want mp3", got)
	}
	if backend.lastSynthReq.Speed == nil || *backend.lastSynthReq.Speed != float32(1.25) {
		t.Fatalf("speed = %#v, want 1.25", backend.lastSynthReq.Speed)
	}
	if _, ok := payload["audio_base64"].(string); !ok {
		t.Fatalf("expected audio_base64 in payload, got %#v", payload["audio_base64"])
	}
}

func TestTTSToolSpeakUsesLocalPlaybackWhenSupported(t *testing.T) {
	backend := &mockTTSBackend{speakSupported: true}
	tool := NewTTSTool(backend)

	result, err := tool.Execute(context.Background(), map[string]interface{}{"action": "speak", "text": "hello"})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("result type = %T, want map", result)
	}
	if got := payload["played_locally"]; got != true {
		t.Fatalf("played_locally = %v, want true", got)
	}
	if backend.synthCalls != 0 {
		t.Fatalf("synthCalls = %d, want 0", backend.synthCalls)
	}
}

func TestTTSToolSpeakFallsBackToSynthesis(t *testing.T) {
	backend := &mockTTSBackend{
		synthResult: &TTSAudioResult{Audio: []byte("wavdata"), Format: "wav", ContentType: "audio/wav", DurationSeconds: 1.0},
	}
	tool := NewTTSTool(backend)

	result, err := tool.Execute(context.Background(), map[string]interface{}{"action": "say", "text": "hello"})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("result type = %T, want map", result)
	}
	path, _ := payload["path"].(string)
	if path == "" {
		t.Fatal("expected synthesized audio path")
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	if got := payload["played_locally"]; got != false {
		t.Fatalf("played_locally = %v, want false", got)
	}
	if backend.synthCalls != 1 {
		t.Fatalf("synthCalls = %d, want 1", backend.synthCalls)
	}
}

func TestTTSToolVoicesStatusAndStop(t *testing.T) {
	backend := &mockTTSBackend{
		voices: []TTSVoiceInfo{{ID: "alloy", Name: "Alloy", Language: "en"}},
		status: map[string]interface{}{"tts": map[string]interface{}{"ready": true, "provider": "edge-tts"}},
		config: map[string]interface{}{"speed": 1.1, "pitch": 0.0, "volume": 90.0, "provider": "edge-tts"},
	}
	tool := NewTTSTool(backend)

	voicesResult, err := tool.Execute(context.Background(), map[string]interface{}{"action": "voices"})
	if err != nil {
		t.Fatalf("voices Execute returned error: %v", err)
	}
	voicesPayload, ok := voicesResult.(map[string]interface{})
	if !ok {
		t.Fatalf("voices result type = %T, want map", voicesResult)
	}
	if got := voicesPayload["count"]; got != 1 {
		t.Fatalf("count = %v, want 1", got)
	}

	statusResult, err := tool.Execute(context.Background(), map[string]interface{}{"action": "status"})
	if err != nil {
		t.Fatalf("status Execute returned error: %v", err)
	}
	statusPayload, ok := statusResult.(map[string]interface{})
	if !ok {
		t.Fatalf("status result type = %T, want map", statusResult)
	}
	if _, ok := statusPayload["status"].(map[string]interface{}); !ok {
		t.Fatalf("expected status payload map, got %#v", statusPayload["status"])
	}

	if _, err := tool.Execute(context.Background(), map[string]interface{}{"action": "stop"}); err != nil {
		t.Fatalf("stop Execute returned error: %v", err)
	}
	if !backend.stopCalled {
		t.Fatal("expected stop to be called")
	}
}

func TestRegisterTTSTool(t *testing.T) {
	registry := NewRegistry()
	backend := &mockTTSBackend{}
	RegisterTTSTool(registry, backend)
	if registry.Get("tts") == nil {
		t.Fatal("expected tts tool to be registered")
	}
}
