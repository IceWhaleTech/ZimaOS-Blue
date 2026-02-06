package voice

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"regexp"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/stt"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/tts"
)

// emojiRegex matches emoji characters
var emojiRegex = regexp.MustCompile(`[\x{1F600}-\x{1F64F}]|[\x{1F300}-\x{1F5FF}]|[\x{1F680}-\x{1F6FF}]|[\x{1F1E0}-\x{1F1FF}]|[\x{2600}-\x{26FF}]|[\x{2700}-\x{27BF}]|[\x{FE00}-\x{FE0F}]|[\x{1F900}-\x{1F9FF}]|[\x{1FA00}-\x{1FA6F}]|[\x{1FA70}-\x{1FAFF}]|[\x{231A}-\x{231B}]|[\x{23E9}-\x{23F3}]|[\x{23F8}-\x{23FA}]|[\x{25AA}-\x{25AB}]|[\x{25B6}]|[\x{25C0}]|[\x{25FB}-\x{25FE}]|[\x{2614}-\x{2615}]|[\x{2648}-\x{2653}]|[\x{267F}]|[\x{2693}]|[\x{26A1}]|[\x{26AA}-\x{26AB}]|[\x{26BD}-\x{26BE}]|[\x{26C4}-\x{26C5}]|[\x{26CE}]|[\x{26D4}]|[\x{26EA}]|[\x{26F2}-\x{26F3}]|[\x{26F5}]|[\x{26FA}]|[\x{26FD}]|[\x{2702}]|[\x{2705}]|[\x{2708}-\x{270D}]|[\x{270F}]|[\x{2712}]|[\x{2714}]|[\x{2716}]|[\x{271D}]|[\x{2721}]|[\x{2728}]|[\x{2733}-\x{2734}]|[\x{2744}]|[\x{2747}]|[\x{274C}]|[\x{274E}]|[\x{2753}-\x{2755}]|[\x{2757}]|[\x{2763}-\x{2764}]|[\x{2795}-\x{2797}]|[\x{27A1}]|[\x{27B0}]|[\x{27BF}]|[\x{2934}-\x{2935}]|[\x{2B05}-\x{2B07}]|[\x{2B1B}-\x{2B1C}]|[\x{2B50}]|[\x{2B55}]|[\x{3030}]|[\x{303D}]|[\x{3297}]|[\x{3299}]`)

// stripEmojis removes emoji characters from text
func stripEmojis(text string) string {
	return emojiRegex.ReplaceAllString(text, "")
}

// service implements the Service interface.
type service struct {
	sttService stt.Service
	ttsService tts.Service
	sessions   map[string]*Session
	mu         sync.RWMutex
	// TTS cache
	ttsCache map[string]*ttsCacheEntry
	ttsCacheMu sync.RWMutex
}

// ttsCacheEntry represents a cached TTS result
type ttsCacheEntry struct {
	audio       []byte
	contentType string
	createdAt   time.Time
}

// ServiceConfig holds the configuration for the voice service.
type ServiceConfig struct {
	STTService stt.Service
	TTSService tts.Service
}

// NewService creates a new voice service.
func NewService(cfg *ServiceConfig) Service {
	return &service{
		sttService: cfg.STTService,
		ttsService: cfg.TTSService,
		sessions:   make(map[string]*Session),
		ttsCache:   make(map[string]*ttsCacheEntry),
	}
}

// CreateSession creates a new voice session.
func (s *service) CreateSession(ctx context.Context, userID string, config *VoiceConfig) (*Session, error) {
	session := &Session{
		ID:           uuid.New().String(),
		UserID:       userID,
		State:        StateIdle,
		Language:     config.Language,
		Voice:        config.Voice,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
	}

	s.mu.Lock()
	s.sessions[session.ID] = session
	s.mu.Unlock()

	return session, nil
}

// GetSession gets a session by ID.
func (s *service) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	s.mu.RLock()
	session, ok := s.sessions[sessionID]
	s.mu.RUnlock()

	if !ok {
		return nil, ErrSessionNotFound
	}

	// Check if session has expired (30 minutes of inactivity)
	if time.Since(session.LastActivity) > 30*time.Minute {
		s.mu.Lock()
		delete(s.sessions, sessionID)
		s.mu.Unlock()
		return nil, ErrSessionExpired
	}

	return session, nil
}

// UpdateSessionState updates the session state.
func (s *service) UpdateSessionState(ctx context.Context, sessionID string, state SessionState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return ErrSessionNotFound
	}

	session.State = state
	session.LastActivity = time.Now()
	return nil
}

// CloseSession closes a session.
func (s *service) CloseSession(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.sessions[sessionID]; !ok {
		return ErrSessionNotFound
	}

	delete(s.sessions, sessionID)
	return nil
}

// Transcribe transcribes audio to text.
func (s *service) Transcribe(ctx context.Context, req *TranscribeRequest, audio []byte) (*TranscribeResponse, error) {
	if len(audio) == 0 {
		return nil, ErrInvalidAudio
	}

	if s.sttService == nil {
		return nil, fmt.Errorf("STT service not initialized")
	}

	// Determine audio format
	format := stt.AudioFormat(req.Format)
	if format == "" {
		format = stt.FormatWAV
	}

	// Create STT request
	sttReq := &stt.TranscribeRequest{
		Audio:    bytes.NewReader(audio),
		Format:   format,
		Language: req.Language,
	}

	// Transcribe
	result, err := s.sttService.Transcribe(ctx, sttReq)
	if err != nil {
		return nil, fmt.Errorf("transcription failed: %w", err)
	}

	return &TranscribeResponse{
		Text:       result.Text,
		Language:   result.Language,
		Duration:   result.Duration,
		Confidence: result.Confidence,
	}, nil
}

// Synthesize synthesizes text to speech.
// Provider, voice, and speed are determined internally based on text content.
func (s *service) Synthesize(ctx context.Context, req *SynthesizeRequest) ([]byte, string, error) {
	if req.Text == "" {
		return nil, "", fmt.Errorf("text is required")
	}

	if s.ttsService == nil {
		return nil, "", fmt.Errorf("TTS service not configured")
	}

	// Strip emojis from text before synthesis
	cleanText := stripEmojis(req.Text)
	if cleanText == "" {
		return nil, "", fmt.Errorf("text contains only emojis")
	}

	// Get TTS config (speed, pitch, volume)
	speed, pitch, volume := s.ttsService.GetConfig()

	// Determine format
	format := tts.AudioFormat(req.Format)
	if format == "" {
		format = tts.FormatMP3
	}

	// Generate cache key including config settings (use clean text)
	cacheKey := fmt.Sprintf("%s:%s:%.2f:%.2f:%.2f", cleanText, format, speed, pitch, volume)

	// Check cache
	s.ttsCacheMu.RLock()
	if entry, exists := s.ttsCache[cacheKey]; exists {
		s.ttsCacheMu.RUnlock()
		return entry.audio, entry.contentType, nil
	}
	s.ttsCacheMu.RUnlock()

	// Create TTS request with config settings (use clean text)
	ttsReq := &tts.SynthesizeRequest{
		Text:   cleanText,
		Format: format,
		Speed:  speed,
		Pitch:  pitch,
		Volume: volume,
	}

	// Use default provider (Edge TTS with language detection)
	result, err := s.ttsService.Synthesize(ctx, ttsReq)
	if err != nil {
		return nil, "", fmt.Errorf("synthesis failed: %w", err)
	}
	defer result.Audio.Close()

	// Read audio data
	audioData, err := io.ReadAll(result.Audio)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read audio: %w", err)
	}

	// Cache the result
	s.ttsCacheMu.Lock()
	s.ttsCache[cacheKey] = &ttsCacheEntry{
		audio:       audioData,
		contentType: result.ContentType,
		createdAt:   time.Now(),
	}
	s.ttsCacheMu.Unlock()

	return audioData, result.ContentType, nil
}

// ProcessVoiceInput processes voice input and returns a response.
// This is a placeholder that should be integrated with the chat/LLM service.
func (s *service) ProcessVoiceInput(ctx context.Context, sessionID string, text string) (string, error) {
	// Update session state
	if err := s.UpdateSessionState(ctx, sessionID, StateProcessing); err != nil {
		return "", err
	}

	// TODO: Integrate with chat service to get LLM response
	// For now, return a placeholder response
	response := fmt.Sprintf("I heard you say: %s", text)

	return response, nil
}

// CleanupExpiredSessions removes expired sessions.
func (s *service) CleanupExpiredSessions() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for id, session := range s.sessions {
		if now.Sub(session.LastActivity) > 30*time.Minute {
			delete(s.sessions, id)
		}
	}
}
