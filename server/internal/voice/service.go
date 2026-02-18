package voice

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
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
	// TTS cache
	ttsCache   map[string]*ttsCacheEntry
	ttsCacheMu sync.RWMutex
	ttsSF      singleflight.Group
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

// maxChunkLen is the threshold (in runes) above which text is split into chunks.
const maxChunkLen = 200

// synthesizeChunk synthesizes a single text chunk via singleflight (dedup identical requests).
func (s *service) synthesizeChunk(ctx context.Context, text string, format tts.AudioFormat, speed, pitch, volume float32) ([]byte, string, error) {
	provider := string(s.ttsService.GetDefaultProvider())
	key := fmt.Sprintf("%s:%s:%s:%.2f:%.2f:%.2f", provider, text, format, speed, pitch, volume)

	// Check cache first
	s.ttsCacheMu.RLock()
	if entry, exists := s.ttsCache[key]; exists {
		s.ttsCacheMu.RUnlock()
		return entry.audio, entry.contentType, nil
	}
	s.ttsCacheMu.RUnlock()

	// Singleflight dedup: identical chunks only synthesize once
	type sfResult struct {
		audio       []byte
		contentType string
	}
	v, err, _ := s.ttsSF.Do(key, func() (interface{}, error) {
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
		// Cache
		s.ttsCacheMu.Lock()
		s.ttsCache[key] = &ttsCacheEntry{audio: data, contentType: result.ContentType, createdAt: time.Now()}
		s.ttsCacheMu.Unlock()
		return &sfResult{audio: data, contentType: result.ContentType}, nil
	})
	if err != nil {
		return nil, "", err
	}
	r := v.(*sfResult)
	return r.audio, r.contentType, nil
}

// Synthesize synthesizes text to speech.
// Long text is split into sentence chunks and synthesized concurrently via singleflight.
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

	speed, pitch, volume := s.ttsService.GetConfig()
	format := tts.AudioFormat(req.Format)
	if format == "" {
		format = tts.FormatMP3
	}

	chunks := splitTextForTTS(cleanText, maxChunkLen)

	// Single chunk: no concurrency needed
	if len(chunks) == 1 {
		return s.synthesizeChunk(ctx, chunks[0], format, speed, pitch, volume)
	}

	// Multiple chunks: synthesize concurrently via singleflight
	type indexedResult struct {
		idx         int
		audio       []byte
		contentType string
		err         error
	}
	ch := make(chan indexedResult, len(chunks))
	for i, chunk := range chunks {
		go func(idx int, text string) {
			audio, ct, err := s.synthesizeChunk(ctx, text, format, speed, pitch, volume)
			ch <- indexedResult{idx: idx, audio: audio, contentType: ct, err: err}
		}(i, chunk)
	}

	// Collect in order
	ordered := make([]indexedResult, len(chunks))
	for range chunks {
		r := <-ch
		if r.err != nil {
			return nil, "", fmt.Errorf("synthesis failed: %w", r.err)
		}
		ordered[r.idx] = r
	}

	var buf bytes.Buffer
	var contentType string
	for _, r := range ordered {
		buf.Write(r.audio)
		if contentType == "" {
			contentType = r.contentType
		}
	}
	return buf.Bytes(), contentType, nil
}

// splitTextForTTS splits text into sentence-level chunks.
func splitTextForTTS(text string, maxRunes int) []string {
	if utf8.RuneCountInString(text) <= maxRunes {
		return []string{text}
	}

	var chunks []string
	var current strings.Builder
	currentLen := 0

	for _, r := range text {
		current.WriteRune(r)
		currentLen++

		isSentenceEnd := r == '.' || r == '!' || r == '?' ||
			r == '。' || r == '！' || r == '？' || r == '；' ||
			r == '\n'

		if isSentenceEnd && currentLen > 0 {
			if s := strings.TrimSpace(current.String()); s != "" {
				chunks = append(chunks, s)
			}
			current.Reset()
			currentLen = 0
		} else if currentLen >= maxRunes {
			s := current.String()
			cutIdx := strings.LastIndexAny(s, ",;，、 ")
			if cutIdx > len(s)/2 {
				chunks = append(chunks, strings.TrimSpace(s[:cutIdx+1]))
				remainder := s[cutIdx+1:]
				current.Reset()
				current.WriteString(remainder)
				currentLen = utf8.RuneCountInString(remainder)
			} else {
				chunks = append(chunks, strings.TrimSpace(s))
				current.Reset()
				currentLen = 0
			}
		}
	}
	if s := strings.TrimSpace(current.String()); s != "" {
		chunks = append(chunks, s)
	}
	return chunks
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
