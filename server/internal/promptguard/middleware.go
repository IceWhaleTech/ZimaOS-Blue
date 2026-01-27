// Package promptguard provides protection against prompt injection attacks.
package promptguard

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

// MiddlewareConfig holds configuration for the prompt guard middleware.
type MiddlewareConfig struct {
	// Detector is the prompt injection detector.
	Detector *Detector
	// OutputFilter is the output filter.
	OutputFilter *OutputFilter
	// Skipper defines a function to skip middleware.
	Skipper func(c echo.Context) bool
	// OnThreatDetected is called when a threat is detected.
	OnThreatDetected func(c echo.Context, result *DetectionResult)
	// BlockOnThreat determines if requests should be blocked on threat detection.
	BlockOnThreat bool
	// LogThreats enables logging of detected threats.
	LogThreats bool
	// InputFields specifies which JSON fields to check for injection.
	InputFields []string
	// FilterOutput enables output filtering.
	FilterOutput bool
}

// DefaultMiddlewareConfig returns the default middleware configuration.
func DefaultMiddlewareConfig() *MiddlewareConfig {
	return &MiddlewareConfig{
		Detector:      NewDetector(nil),
		OutputFilter:  NewOutputFilter(nil),
		BlockOnThreat: true,
		LogThreats:    true,
		InputFields:   []string{"content", "message", "prompt", "query", "text", "input"},
		FilterOutput:  true,
	}
}

// Middleware returns an Echo middleware for prompt injection protection.
func Middleware(config *MiddlewareConfig) echo.MiddlewareFunc {
	if config == nil {
		config = DefaultMiddlewareConfig()
	}

	if config.Detector == nil {
		config.Detector = NewDetector(nil)
	}

	if config.OutputFilter == nil {
		config.OutputFilter = NewOutputFilter(nil)
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Skip if skipper returns true
			if config.Skipper != nil && config.Skipper(c) {
				return next(c)
			}

			// Only check POST/PUT/PATCH requests with JSON body
			if !isJSONRequest(c) {
				return next(c)
			}

			// Read and check request body
			body, err := io.ReadAll(c.Request().Body)
			if err != nil {
				return next(c)
			}

			// Restore body for downstream handlers
			c.Request().Body = io.NopCloser(bytes.NewReader(body))

			// Extract and check input fields
			var jsonBody map[string]interface{}
			if err := json.Unmarshal(body, &jsonBody); err != nil {
				return next(c)
			}

			// Check each configured field
			var allDetections []Detection
			for _, field := range config.InputFields {
				if value, ok := extractField(jsonBody, field); ok {
					result := config.Detector.Detect(value)
					if result.IsThreat {
						allDetections = append(allDetections, result.Detections...)
					}
				}
			}

			// Handle threat detection
			if len(allDetections) > 0 {
				combinedResult := &DetectionResult{
					IsThreat:    true,
					Detections:  allDetections,
					ThreatLevel: calculateMaxThreatLevel(allDetections),
				}

				// Call threat handler if configured
				if config.OnThreatDetected != nil {
					config.OnThreatDetected(c, combinedResult)
				}

				// Log threat if enabled
				if config.LogThreats {
					logThreat(c, combinedResult)
				}

				// Block request if configured
				if config.BlockOnThreat && combinedResult.ThreatLevel >= config.Detector.config.BlockThreshold {
					return c.JSON(http.StatusBadRequest, map[string]interface{}{
						"error":        "potential prompt injection detected",
						"threat_level": combinedResult.ThreatLevel.String(),
						"score":        combinedResult.Score,
					})
				}

				// Store result in context for downstream handlers
				c.Set("prompt_guard_result", combinedResult)
			}

			// If output filtering is enabled, wrap the response
			if config.FilterOutput {
				return filterResponse(c, next, config.OutputFilter)
			}

			return next(c)
		}
	}
}

// isJSONRequest checks if the request has a JSON content type.
func isJSONRequest(c echo.Context) bool {
	contentType := c.Request().Header.Get("Content-Type")
	return strings.Contains(contentType, "application/json")
}

// extractField extracts a string value from a nested JSON structure.
func extractField(data map[string]interface{}, field string) (string, bool) {
	// Handle nested fields (e.g., "messages.content")
	parts := strings.Split(field, ".")

	var current interface{} = data
	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			current = v[part]
		case []interface{}:
			// For arrays, check all elements
			var results []string
			for _, item := range v {
				if m, ok := item.(map[string]interface{}); ok {
					if val, ok := extractField(m, part); ok {
						results = append(results, val)
					}
				}
			}
			if len(results) > 0 {
				return strings.Join(results, " "), true
			}
			return "", false
		default:
			return "", false
		}
	}

	if str, ok := current.(string); ok {
		return str, true
	}

	return "", false
}

// calculateMaxThreatLevel calculates the maximum threat level from detections.
func calculateMaxThreatLevel(detections []Detection) ThreatLevel {
	maxLevel := ThreatNone
	for _, d := range detections {
		if d.Severity > maxLevel {
			maxLevel = d.Severity
		}
	}
	return maxLevel
}

// logThreat logs a detected threat.
func logThreat(c echo.Context, result *DetectionResult) {
	// Use Echo's logger
	c.Logger().Warnf("Prompt injection detected: level=%s, detections=%d, ip=%s, path=%s",
		result.ThreatLevel.String(),
		len(result.Detections),
		c.RealIP(),
		c.Request().URL.Path,
	)
}

// responseWriter wraps http.ResponseWriter to capture the response.
type responseWriter struct {
	http.ResponseWriter
	body   *bytes.Buffer
	status int
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.body.Write(b)
	return rw.ResponseWriter.Write(b)
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

// filterResponse filters the response body.
func filterResponse(c echo.Context, next echo.HandlerFunc, filter *OutputFilter) error {
	// Create a buffer to capture the response
	resBody := new(bytes.Buffer)
	mw := io.MultiWriter(c.Response().Writer, resBody)

	// Create a custom response writer
	writer := &responseCapture{
		ResponseWriter: c.Response().Writer,
		body:           resBody,
		tee:            mw,
	}

	// Replace the response writer
	originalWriter := c.Response().Writer
	c.Response().Writer = writer

	// Call the next handler
	err := next(c)

	// Restore original writer
	c.Response().Writer = originalWriter

	// Filter the response if it's JSON
	if strings.Contains(c.Response().Header().Get("Content-Type"), "application/json") {
		var jsonResp map[string]interface{}
		if json.Unmarshal(resBody.Bytes(), &jsonResp) == nil {
			// Filter relevant fields
			filtered := false
			for key, value := range jsonResp {
				if str, ok := value.(string); ok {
					result := filter.Filter(str)
					if result.WasFiltered {
						jsonResp[key] = result.FilteredOutput
						filtered = true
					}
				}
			}

			if filtered {
				c.Set("output_filtered", true)
			}
		}
	}

	return err
}

// responseCapture captures the response body.
type responseCapture struct {
	http.ResponseWriter
	body *bytes.Buffer
	tee  io.Writer
}

func (rc *responseCapture) Write(b []byte) (int, error) {
	return rc.tee.Write(b)
}

// Guard provides a unified interface for prompt injection protection.
type Guard struct {
	detector     *Detector
	outputFilter *OutputFilter
}

// NewGuard creates a new prompt guard.
func NewGuard(detectorConfig *DetectorConfig, filterConfig *OutputFilterConfig) *Guard {
	return &Guard{
		detector:     NewDetector(detectorConfig),
		outputFilter: NewOutputFilter(filterConfig),
	}
}

// CheckInput checks input for prompt injection.
func (g *Guard) CheckInput(input string) *DetectionResult {
	return g.detector.Detect(input)
}

// FilterOutput filters output for sensitive content.
func (g *Guard) FilterOutput(output string) *OutputFilterResult {
	return g.outputFilter.Filter(output)
}

// ProcessChat processes a chat message through both input detection and output filtering.
func (g *Guard) ProcessChat(input string) (string, *DetectionResult, error) {
	// Check input
	result := g.detector.Detect(input)
	if result.IsThreat && result.ThreatLevel >= g.detector.config.BlockThreshold {
		return "", result, ErrPromptInjectionDetected
	}

	// Return sanitized input if threat was detected but below threshold
	if result.IsThreat {
		return result.SanitizedInput, result, nil
	}

	return input, result, nil
}

// GetDetector returns the underlying detector.
func (g *Guard) GetDetector() *Detector {
	return g.detector
}

// GetOutputFilter returns the underlying output filter.
func (g *Guard) GetOutputFilter() *OutputFilter {
	return g.outputFilter
}

// SetSystemPrompt sets the system prompt for leak detection.
func (g *Guard) SetSystemPrompt(prompt string) {
	// Extract key phrases from system prompt for leak detection
	phrases := extractKeyPhrases(prompt)
	for _, phrase := range phrases {
		g.outputFilter.AddSystemPromptPattern(phrase)
	}
}

// extractKeyPhrases extracts key phrases from text for leak detection.
func extractKeyPhrases(text string) []string {
	var phrases []string

	// Split into sentences
	sentences := strings.Split(text, ".")
	for _, sentence := range sentences {
		sentence = strings.TrimSpace(sentence)
		// Only include substantial phrases
		if len(sentence) >= 20 && len(sentence) <= 200 {
			phrases = append(phrases, sentence)
		}
	}

	return phrases
}
