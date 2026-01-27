package voice

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/stt"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/tts"
)

// service implements the Service interface.
type service struct {
	sttService stt.Service
	ttsService tts.Service
	sessions   map[string]*Session
	mu         sync.RWMutex
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
func (s *service) Synthesize(ctx context.Context, req *SynthesizeRequest) ([]byte, string, error) {
	if req.Text == "" {
		return nil, "", fmt.Errorf("text is required")
	}

	// Determine format
	format := tts.AudioFormat(req.Format)
	if format == "" {
		format = tts.FormatMP3
	}

	// Create TTS request
	ttsReq := &tts.SynthesizeRequest{
		Text:   req.Text,
		Voice:  req.Voice,
		Format: format,
		Speed:  req.Speed,
	}

	// Synthesize
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
