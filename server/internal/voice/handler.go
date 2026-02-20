package voice

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/speech"
)

// Handler handles voice HTTP requests.
type Handler struct {
	service Service
}

// NewHandler creates a new voice handler.
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// Service returns the voice service.
func (h *Handler) Service() Service {
	return h.service
}

// RegisterRoutes registers the voice routes.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	g.POST("/transcribe", h.Transcribe)
	g.POST("/synthesize", h.Synthesize)
	g.GET("/synthesize/stream", h.SynthesizeStream)
	g.POST("/synthesize/stop", h.StopSpeaking)
	g.GET("/voices", h.ListVoices)
	g.POST("/sessions", h.CreateSession)
	g.GET("/sessions/:id", h.GetSession)
	g.DELETE("/sessions/:id", h.CloseSession)
}

// TranscribeRequest represents the transcribe API request.
type transcribeAPIRequest struct {
	Audio    string `json:"audio" form:"audio"` // Base64 encoded audio
	Format   string `json:"format" form:"format"`
	Language string `json:"language" form:"language"`
}

// Transcribe handles audio transcription requests.
func (h *Handler) Transcribe(c echo.Context) error {
	// Check content type for multipart form
	contentType := c.Request().Header.Get("Content-Type")

	var audioData []byte
	var format, language string

	if contentType == "application/json" {
		// JSON request with base64 audio
		var req transcribeAPIRequest
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}

		if req.Audio == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "audio is required")
		}

		// Decode base64 audio
		decoded, err := base64.StdEncoding.DecodeString(req.Audio)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid base64 audio")
		}
		audioData = decoded
		format = req.Format
		language = req.Language
	} else {
		// Multipart form with file upload
		file, err := c.FormFile("audio")
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "audio file is required")
		}

		src, err := file.Open()
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to open file")
		}
		defer src.Close()

		audioData, err = io.ReadAll(src)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to read file")
		}

		format = c.FormValue("format")
		language = c.FormValue("language")
	}

	// Default format
	if format == "" {
		format = "wav"
	}

	// Transcribe
	result, err := h.service.Transcribe(c.Request().Context(), &TranscribeRequest{
		Format:   format,
		Language: language,
	}, audioData)
	if err != nil {
		var onDeviceErr *speech.OnDeviceUnavailableError
		if errors.As(err, &onDeviceErr) {
			return c.JSON(http.StatusUnprocessableEntity, map[string]string{
				"error":      err.Error(),
				"error_code": "on_device_unavailable",
				"locale":     onDeviceErr.Locale,
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, result)
}

// SynthesizeRequest represents the synthesize API request.
type synthesizeAPIRequest struct {
	Text   string `json:"text"`
	Format string `json:"format"`
}

// Synthesize handles text-to-speech requests.
func (h *Handler) Synthesize(c echo.Context) error {
	var req synthesizeAPIRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if req.Text == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "text is required")
	}

	// Check if client is on loopback — try local playback first
	if isLoopback(c.RealIP()) {
		supported, err := h.service.SpeakLocally(c.Request().Context(), req.Text)
		if supported {
			if err != nil {
				if err == context.Canceled {
					slog.Info("[tts] local playback stopped by user")
					return c.JSON(http.StatusOK, map[string]interface{}{
						"played_locally": true,
						"stopped":        true,
					})
				}
				slog.Error("[tts] local playback failed", "error", err)
				return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
			}
			return c.JSON(http.StatusOK, map[string]interface{}{
				"played_locally": true,
			})
		}
	}

	// Default format
	if req.Format == "" {
		req.Format = "wav"
	}

	// Synthesize - voice/speed/provider are determined internally
	slog.Info("[tts] synthesize request", "text_len", len(req.Text), "format", req.Format)
	audioData, contentType, err := h.service.Synthesize(c.Request().Context(), &SynthesizeRequest{
		Text:   req.Text,
		Format: req.Format,
	})
	if err != nil {
		slog.Error("[tts] synthesize failed", "error", err, "text_len", len(req.Text))
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	slog.Info("[tts] synthesize ok", "audio_bytes", len(audioData), "content_type", contentType)

	// Check if client wants base64 response
	if c.Request().Header.Get("Accept") == "application/json" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"audio":        base64.StdEncoding.EncodeToString(audioData),
			"content_type": contentType,
			"format":       req.Format,
		})
	}

	// Return audio directly
	return c.Blob(http.StatusOK, contentType, audioData)
}

// SynthesizeStream handles streaming text-to-speech requests via SSE.
// Uses true chunk-by-chunk streaming: each sentence's audio is sent as soon as it's ready.
func (h *Handler) SynthesizeStream(c echo.Context) error {
	text := c.QueryParam("text")
	if text == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "text is required")
	}

	format := c.QueryParam("format")
	if format == "" {
		format = "wav"
	}

	// Set SSE headers
	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().WriteHeader(http.StatusOK)

	// Loopback: play locally, send played_locally event instead of audio
	if isLoopback(c.RealIP()) {
		supported, err := h.service.SpeakLocally(c.Request().Context(), text)
		if supported {
			if err != nil && err != context.Canceled {
				c.Response().Write([]byte("event: error\ndata: " + err.Error() + "\n\n"))
				c.Response().Flush()
				return nil
			}
			c.Response().Write([]byte("event: audio\ndata: {\"played_locally\":true}\n\n"))
			c.Response().Flush()
			c.Response().Write([]byte("event: done\ndata: {}\n\n"))
			c.Response().Flush()
			return nil
		}
	}

	// True streaming: each chunk arrives as a separate SSE event
	err := h.service.SynthesizeStream(c.Request().Context(), &SynthesizeRequest{
		Text:   text,
		Format: format,
	}, func(audio []byte, contentType string) error {
		audioBase64 := base64.StdEncoding.EncodeToString(audio)
		_, writeErr := c.Response().Write([]byte("event: audio\ndata: {\"audio\":\"" + audioBase64 + "\",\"content_type\":\"" + contentType + "\"}\n\n"))
		if writeErr != nil {
			return writeErr
		}
		c.Response().Flush()
		return nil
	})

	if err != nil {
		slog.Error("[tts-stream] streaming synthesis failed", "error", err)
		c.Response().Write([]byte("event: error\ndata: " + err.Error() + "\n\n"))
		c.Response().Flush()
		return nil
	}

	// Send done event
	c.Response().Write([]byte("event: done\ndata: {}\n\n"))
	c.Response().Flush()

	return nil
}

// ListVoices returns available TTS voices.
func (h *Handler) ListVoices(c echo.Context) error {
	// This would need access to the TTS service
	// For now, return a placeholder
	voices := []map[string]string{
		{"id": "alloy", "name": "Alloy", "language": "en", "gender": "neutral"},
		{"id": "echo", "name": "Echo", "language": "en", "gender": "male"},
		{"id": "fable", "name": "Fable", "language": "en", "gender": "neutral"},
		{"id": "onyx", "name": "Onyx", "language": "en", "gender": "male"},
		{"id": "nova", "name": "Nova", "language": "en", "gender": "female"},
		{"id": "shimmer", "name": "Shimmer", "language": "en", "gender": "female"},
	}
	return c.JSON(http.StatusOK, voices)
}

// CreateSessionRequest represents the create session API request.
type createSessionRequest struct {
	Language            string `json:"language"`
	Voice               string `json:"voice"`
	WakeWord            string `json:"wake_word"`
	WakeWordEnabled     bool   `json:"wake_word_enabled"`
	ContinuousListening bool   `json:"continuous_listening"`
	AutoPlayResponse    bool   `json:"auto_play_response"`
}

// CreateSession creates a new voice session.
func (h *Handler) CreateSession(c echo.Context) error {
	// Get user ID from context
	userID := c.Get("user_id")
	if userID == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}

	var req createSessionRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	// Default values
	if req.Language == "" {
		req.Language = "en"
	}
	if req.Voice == "" {
		req.Voice = "alloy"
	}

	session, err := h.service.CreateSession(c.Request().Context(), userID.(string), &VoiceConfig{
		Language:            req.Language,
		Voice:               req.Voice,
		WakeWord:            req.WakeWord,
		WakeWordEnabled:     req.WakeWordEnabled,
		ContinuousListening: req.ContinuousListening,
		AutoPlayResponse:    req.AutoPlayResponse,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusCreated, session)
}

// GetSession gets a voice session.
func (h *Handler) GetSession(c echo.Context) error {
	sessionID := c.Param("id")
	if sessionID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "session ID is required")
	}

	session, err := h.service.GetSession(c.Request().Context(), sessionID)
	if err != nil {
		if err == ErrSessionNotFound {
			return echo.NewHTTPError(http.StatusNotFound, err.Error())
		}
		if err == ErrSessionExpired {
			return echo.NewHTTPError(http.StatusGone, err.Error())
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, session)
}

// CloseSession closes a voice session.
func (h *Handler) CloseSession(c echo.Context) error {
	sessionID := c.Param("id")
	if sessionID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "session ID is required")
	}

	if err := h.service.CloseSession(c.Request().Context(), sessionID); err != nil {
		if err == ErrSessionNotFound {
			return echo.NewHTTPError(http.StatusNotFound, err.Error())
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "closed"})
}

// isLoopback checks if an IP string is a loopback address.
func isLoopback(ip string) bool {
	parsed := net.ParseIP(ip)
	return parsed != nil && parsed.IsLoopback()
}

// StopSpeaking stops any currently running local speech.
func (h *Handler) StopSpeaking(c echo.Context) error {
	h.service.StopSpeaking()
	return c.JSON(http.StatusOK, map[string]string{"status": "stopped"})
}

