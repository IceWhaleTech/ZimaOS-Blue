package voice

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"golang.org/x/sync/singleflight"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/humanizer"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/stt"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tts"
)

// service implements the Service interface.
type service struct {
	sttService stt.Service
	ttsService tts.Service
	sessions   map[string]*Session
	mu         sync.RWMutex
	// TTS disk cache — bounded LRU index, audio stored in /tmp
	ttsCacheDir string
	ttsIndex    []*ttsCacheEntry // LRU order: oldest first, newest last
	ttsIndexMu  sync.Mutex
	ttsSF       singleflight.Group
}

// ttsCacheMaxSize is the maximum number of TTS results cached on disk.
const ttsCacheMaxSize = 5

// ttsCacheEntry is an LRU index entry pointing to a file on disk.
type ttsCacheEntry struct {
	hash        string // SHA-256 hex of cache key
	contentType string
}

// ServiceConfig holds the configuration for the voice service.
type ServiceConfig struct {
	STTService stt.Service
	TTSService tts.Service
}

// NewService creates a new voice service.
func NewService(cfg *ServiceConfig) Service {
	cacheDir := filepath.Join(os.TempDir(), "zimaos-tts-cache")
	os.MkdirAll(cacheDir, 0o755)
	return &service{
		sttService:  cfg.STTService,
		ttsService:  cfg.TTSService,
		sessions:    make(map[string]*Session),
		ttsCacheDir: cacheDir,
	}
}

// SetSTTService replaces the STT service (e.g. to switch from whisper to macOS native).
func (s *service) SetSTTService(svc stt.Service) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sttService = svc
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

// maxSynthesizeLen is the maximum text length (in runes) accepted for synthesis.
// Frontend is responsible for splitting long text into sentences.
const maxSynthesizeLen = 2000

// ttsCacheHash returns a short SHA-256 hex hash for the cache key.
func ttsCacheHash(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:16]) // 32 hex chars, collision-safe enough
}

// synthesizeChunk synthesizes a single text chunk via singleflight (dedup identical requests).
func (s *service) synthesizeChunk(ctx context.Context, text string, format tts.AudioFormat, speed, pitch, volume float32) ([]byte, string, error) {
	provider := string(s.ttsService.GetDefaultProvider())
	key := fmt.Sprintf("%s:%s:%s:%.2f:%.2f:%.2f", provider, text, format, speed, pitch, volume)
	hash := ttsCacheHash(key)

	// Check disk cache (LRU index lookup + file read)
	s.ttsIndexMu.Lock()
	for i, entry := range s.ttsIndex {
		if entry.hash == hash {
			// Promote to most-recent (move to end)
			s.ttsIndex = append(append(s.ttsIndex[:i], s.ttsIndex[i+1:]...), entry)
			s.ttsIndexMu.Unlock()
			data, err := os.ReadFile(filepath.Join(s.ttsCacheDir, hash))
			if err == nil {
				return data, entry.contentType, nil
			}
			// File missing — fall through to re-synthesize
			break
		}
	}
	s.ttsIndexMu.Unlock()

	// Singleflight dedup: identical chunks only synthesize once
	type sfResult struct {
		audio       []byte
		contentType string
	}
	v, err, _ := s.ttsSF.Do(hash, func() (interface{}, error) {
		result, err := s.ttsService.Synthesize(ctx, &tts.SynthesizeRequest{
			Text: text, Format: format, Speed: speed, Pitch: pitch, Volume: volume,
		})
		if err != nil {
			return nil, err
		}
		defer result.Audio.Close()
		data, err := io.ReadAll(result.Audio)
		if err != nil {
			return nil, err
		}
		// Write audio to disk
		_ = os.WriteFile(filepath.Join(s.ttsCacheDir, hash), data, 0o644)

		// Update LRU index, evict oldest if over limit
		entry := &ttsCacheEntry{hash: hash, contentType: result.ContentType}
		s.ttsIndexMu.Lock()
		s.ttsIndex = append(s.ttsIndex, entry)
		for len(s.ttsIndex) > ttsCacheMaxSize {
			evicted := s.ttsIndex[0]
			s.ttsIndex = s.ttsIndex[1:]
			os.Remove(filepath.Join(s.ttsCacheDir, evicted.hash))
		}
		s.ttsIndexMu.Unlock()
		return &sfResult{audio: data, contentType: result.ContentType}, nil
	})
	if err != nil {
		return nil, "", err
	}
	r := v.(*sfResult)
	return r.audio, r.contentType, nil
}

// Synthesize synthesizes text to speech.
// Frontend splits long text into sentences; backend synthesizes each call directly.
func (s *service) Synthesize(ctx context.Context, req *SynthesizeRequest) ([]byte, string, error) {
	if req.Text == "" {
		return nil, "", fmt.Errorf("text is required")
	}
	if s.ttsService == nil {
		return nil, "", fmt.Errorf("TTS service not configured")
	}

	cleanText := humanizer.Humanize(req.Text, humanizer.ModeVoice)
	if cleanText == "" {
		return nil, "", fmt.Errorf("text contains only formatting or emojis")
	}

	if utf8.RuneCountInString(cleanText) > maxSynthesizeLen {
		cleanText = string([]rune(cleanText)[:maxSynthesizeLen])
	}

	speed, pitch, volume := s.ttsService.GetConfig()
	format := tts.AudioFormat(req.Format)
	if format == "" {
		format = tts.FormatWAV
	}

	return s.synthesizeChunk(ctx, cleanText, format, speed, pitch, volume)
}

// SynthesizeStream synthesizes text with chunk-by-chunk streaming.
// Each audio chunk is delivered via callback as soon as it's ready.
func (s *service) SynthesizeStream(ctx context.Context, req *SynthesizeRequest, callback func(audio []byte, contentType string) error) error {
	if req.Text == "" {
		return fmt.Errorf("text is required")
	}
	if s.ttsService == nil {
		return fmt.Errorf("TTS service not configured")
	}

	cleanText := humanizer.Humanize(req.Text, humanizer.ModeVoice)
	if cleanText == "" {
		return fmt.Errorf("text contains only formatting or emojis")
	}
	if utf8.RuneCountInString(cleanText) > maxSynthesizeLen {
		cleanText = string([]rune(cleanText)[:maxSynthesizeLen])
	}

	speed, pitch, volume := s.ttsService.GetConfig()
	format := tts.AudioFormat(req.Format)
	if format == "" {
		format = tts.FormatWAV
	}

	return s.ttsService.SynthesizeStream(ctx, &tts.SynthesizeRequest{
		Text: cleanText, Format: format, Speed: speed, Pitch: pitch, Volume: volume,
	}, func(chunk []byte) error {
		return callback(chunk, "audio/wav")
	})
}

// SpeakLocally plays text through local audio output if the provider supports it.
// When speed is non-default, returns false so the caller falls through to Synthesize
// (which returns audio data to the frontend for custom-rate playback).
func (s *service) SpeakLocally(ctx context.Context, text string) (bool, error) {
	if s.ttsService == nil {
		return false, nil
	}

	speed, _, _ := s.ttsService.GetConfig()

	// Non-default speed: skip local playback, let frontend handle audio with rate control
	if speed != 1.0 {
		return false, nil
	}

	provider := s.ttsService.GetProvider(s.ttsService.GetDefaultProvider())
	if provider == nil {
		return false, nil
	}
	speaker, ok := provider.(tts.LocalSpeaker)
	if !ok {
		return false, nil
	}
	cleanText := humanizer.Humanize(text, humanizer.ModeVoice)
	if cleanText == "" {
		return false, fmt.Errorf("text contains only formatting or emojis")
	}
	return true, speaker.SpeakLocally(ctx, cleanText, speed)
}

// StopSpeaking stops any currently running local speech.
func (s *service) StopSpeaking() {
	if s.ttsService == nil {
		return
	}
	provider := s.ttsService.GetProvider(s.ttsService.GetDefaultProvider())
	if provider == nil {
		return
	}
	if speaker, ok := provider.(tts.LocalSpeaker); ok {
		speaker.StopSpeaking()
	}
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
