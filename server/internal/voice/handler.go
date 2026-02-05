package voice

import (
	"encoding/base64"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
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
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
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

	// Default format
	if req.Format == "" {
		req.Format = "mp3"
	}

	// Synthesize - voice/speed/provider are determined internally
	audioData, contentType, err := h.service.Synthesize(c.Request().Context(), &SynthesizeRequest{
		Text:   req.Text,
		Format: req.Format,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

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
// Client sends sentences as query params, server streams back audio chunks.
func (h *Handler) SynthesizeStream(c echo.Context) error {
	text := c.QueryParam("text")
	if text == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "text is required")
	}

	format := c.QueryParam("format")
	if format == "" {
		format = "mp3"
	}

	// Set SSE headers
	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().WriteHeader(http.StatusOK)

	// Synthesize audio
	audioData, contentType, err := h.service.Synthesize(c.Request().Context(), &SynthesizeRequest{
		Text:   text,
		Format: format,
	})
	if err != nil {
		c.Response().Write([]byte("event: error\ndata: " + err.Error() + "\n\n"))
		c.Response().Flush()
		return nil
	}

	// Send audio as base64 in SSE event
	audioBase64 := base64.StdEncoding.EncodeToString(audioData)
	c.Response().Write([]byte("event: audio\ndata: {\"audio\":\"" + audioBase64 + "\",\"content_type\":\"" + contentType + "\"}\n\n"))
	c.Response().Flush()

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
