package audit

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// MiddlewareConfig holds configuration for the audit middleware.
type MiddlewareConfig struct {
	// Logger is the audit logger to use.
	Logger Logger
	// SkipPaths are paths to skip auditing.
	SkipPaths []string
	// LogRequestBody enables logging of request bodies.
	LogRequestBody bool
	// LogResponseBody enables logging of response bodies.
	LogResponseBody bool
	// MaxBodySize is the maximum body size to log.
	MaxBodySize int
	// GetUserID extracts the user ID from the request context.
	GetUserID func(c echo.Context) *uuid.UUID
	// GetUsername extracts the username from the request context.
	GetUsername func(c echo.Context) string
}

// DefaultMiddlewareConfig returns the default middleware configuration.
func DefaultMiddlewareConfig() *MiddlewareConfig {
	return &MiddlewareConfig{
		SkipPaths:       []string{"/health", "/metrics", "/.well-known"},
		LogRequestBody:  false,
		LogResponseBody: false,
		MaxBodySize:     4096,
		GetUserID:       func(c echo.Context) *uuid.UUID { return nil },
		GetUsername:     func(c echo.Context) string { return "" },
	}
}

// Middleware returns an Echo middleware for audit logging.
func Middleware(config *MiddlewareConfig) echo.MiddlewareFunc {
	if config == nil {
		config = DefaultMiddlewareConfig()
	}

	skipPathsMap := make(map[string]bool)
	for _, path := range config.SkipPaths {
		skipPathsMap[path] = true
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			path := req.URL.Path

			// Check if path should be skipped
			for skipPath := range skipPathsMap {
				if strings.HasPrefix(path, skipPath) {
					return next(c)
				}
			}

			// Capture request body if enabled
			var requestBody []byte
			if config.LogRequestBody && req.Body != nil {
				requestBody, _ = io.ReadAll(io.LimitReader(req.Body, int64(config.MaxBodySize)))
				req.Body = io.NopCloser(bytes.NewBuffer(requestBody))
			}

			// Create response writer wrapper to capture response
			resBody := new(bytes.Buffer)
			mw := &responseWriter{
				ResponseWriter: c.Response().Writer,
				body:           resBody,
				logBody:        config.LogResponseBody,
				maxSize:        config.MaxBodySize,
			}
			c.Response().Writer = mw

			// Process request
			start := timeutil.NowTime()
			err := next(c)
			duration := timeutil.SinceTime(start)

			// Determine status
			status := StatusSuccess
			if err != nil || c.Response().Status >= 400 {
				status = StatusFailure
			}

			// Determine action based on method
			action := methodToAction(req.Method)

			// Create audit entry
			entry := NewEntry(action, status).
				WithRequest(
					c.RealIP(),
					req.UserAgent(),
					c.Response().Header().Get(echo.HeaderXRequestID),
				).
				WithResource(
					"http",
					path,
				)

			// Add user info if available
			if userID := config.GetUserID(c); userID != nil {
				entry.WithUser(*userID, config.GetUsername(c))
			}

			// Add details
			details := map[string]interface{}{
				"method":      req.Method,
				"path":        path,
				"query":       req.URL.RawQuery,
				"status_code": c.Response().Status,
				"duration_ms": duration.Milliseconds(),
			}

			if config.LogRequestBody && len(requestBody) > 0 {
				details["request_body"] = string(requestBody)
			}

			if config.LogResponseBody && resBody.Len() > 0 {
				details["response_body"] = resBody.String()
			}

			entry.WithDetails(details)

			// Log asynchronously
			go func() {
				_ = config.Logger.Log(c.Request().Context(), entry)
			}()

			return err
		}
	}
}

// responseWriter wraps http.ResponseWriter to capture response body.
type responseWriter struct {
	http.ResponseWriter
	body    *bytes.Buffer
	logBody bool
	maxSize int
}

func (w *responseWriter) Write(b []byte) (int, error) {
	if w.logBody && w.body.Len() < w.maxSize {
		remaining := w.maxSize - w.body.Len()
		if len(b) > remaining {
			w.body.Write(b[:remaining])
		} else {
			w.body.Write(b)
		}
	}
	return w.ResponseWriter.Write(b)
}

// methodToAction converts HTTP method to audit action.
func methodToAction(method string) Action {
	switch method {
	case http.MethodGet:
		return ActionAPIAccess
	case http.MethodPost:
		return ActionUserCreate
	case http.MethodPut, http.MethodPatch:
		return ActionUserUpdate
	case http.MethodDelete:
		return ActionUserDelete
	default:
		return ActionAPIAccess
	}
}

// AuditAction is a helper to create audit entries for specific actions.
type AuditAction struct {
	logger Logger
}

// NewAuditAction creates a new AuditAction helper.
func NewAuditAction(logger Logger) *AuditAction {
	return &AuditAction{logger: logger}
}

// LogLogin logs a login event.
func (a *AuditAction) LogLogin(c echo.Context, userID uuid.UUID, username string, success bool) {
	action := ActionLogin
	status := StatusSuccess
	if !success {
		action = ActionLoginFailed
		status = StatusFailure
	}

	entry := NewEntry(action, status).
		WithUser(userID, username).
		WithRequest(
			c.RealIP(),
			c.Request().UserAgent(),
			c.Response().Header().Get(echo.HeaderXRequestID),
		)

	_ = a.logger.Log(c.Request().Context(), entry)
}

// LogLogout logs a logout event.
func (a *AuditAction) LogLogout(c echo.Context, userID uuid.UUID, username string) {
	entry := NewEntry(ActionLogout, StatusSuccess).
		WithUser(userID, username).
		WithRequest(
			c.RealIP(),
			c.Request().UserAgent(),
			c.Response().Header().Get(echo.HeaderXRequestID),
		)

	_ = a.logger.Log(c.Request().Context(), entry)
}

// LogPasswordChange logs a password change event.
func (a *AuditAction) LogPasswordChange(c echo.Context, userID uuid.UUID, username string, success bool) {
	status := StatusSuccess
	if !success {
		status = StatusFailure
	}

	entry := NewEntry(ActionPasswordChange, status).
		WithUser(userID, username).
		WithRequest(
			c.RealIP(),
			c.Request().UserAgent(),
			c.Response().Header().Get(echo.HeaderXRequestID),
		)

	_ = a.logger.Log(c.Request().Context(), entry)
}

// LogMFASetup logs an MFA setup event.
func (a *AuditAction) LogMFASetup(c echo.Context, userID uuid.UUID, username string, success bool) {
	status := StatusSuccess
	if !success {
		status = StatusFailure
	}

	entry := NewEntry(ActionMFASetup, status).
		WithUser(userID, username).
		WithRequest(
			c.RealIP(),
			c.Request().UserAgent(),
			c.Response().Header().Get(echo.HeaderXRequestID),
		)

	_ = a.logger.Log(c.Request().Context(), entry)
}

// LogMFADisable logs an MFA disable event.
func (a *AuditAction) LogMFADisable(c echo.Context, userID uuid.UUID, username string) {
	entry := NewEntry(ActionMFADisable, StatusSuccess).
		WithUser(userID, username).
		WithRequest(
			c.RealIP(),
			c.Request().UserAgent(),
			c.Response().Header().Get(echo.HeaderXRequestID),
		)

	_ = a.logger.Log(c.Request().Context(), entry)
}

// LogUserCreate logs a user creation event.
func (a *AuditAction) LogUserCreate(c echo.Context, actorID uuid.UUID, actorName string, targetID uuid.UUID, targetName string) {
	entry := NewEntry(ActionUserCreate, StatusSuccess).
		WithUser(actorID, actorName).
		WithResource("user", targetID.String()).
		WithRequest(
			c.RealIP(),
			c.Request().UserAgent(),
			c.Response().Header().Get(echo.HeaderXRequestID),
		).
		WithDetails(map[string]interface{}{
			"target_username": targetName,
		})

	_ = a.logger.Log(c.Request().Context(), entry)
}

// LogUserUpdate logs a user update event.
func (a *AuditAction) LogUserUpdate(c echo.Context, actorID uuid.UUID, actorName string, targetID uuid.UUID, oldValue, newValue interface{}) {
	entry := NewEntry(ActionUserUpdate, StatusSuccess).
		WithUser(actorID, actorName).
		WithResource("user", targetID.String()).
		WithRequest(
			c.RealIP(),
			c.Request().UserAgent(),
			c.Response().Header().Get(echo.HeaderXRequestID),
		).
		WithChange(oldValue, newValue)

	_ = a.logger.Log(c.Request().Context(), entry)
}

// LogUserDelete logs a user deletion event.
func (a *AuditAction) LogUserDelete(c echo.Context, actorID uuid.UUID, actorName string, targetID uuid.UUID, targetName string) {
	entry := NewEntry(ActionUserDelete, StatusSuccess).
		WithUser(actorID, actorName).
		WithResource("user", targetID.String()).
		WithRequest(
			c.RealIP(),
			c.Request().UserAgent(),
			c.Response().Header().Get(echo.HeaderXRequestID),
		).
		WithDetails(map[string]interface{}{
			"target_username": targetName,
		})

	_ = a.logger.Log(c.Request().Context(), entry)
}

// LogUserLock logs a user lock event.
func (a *AuditAction) LogUserLock(c echo.Context, actorID uuid.UUID, actorName string, targetID uuid.UUID, targetName string) {
	entry := NewEntry(ActionUserLock, StatusSuccess).
		WithUser(actorID, actorName).
		WithResource("user", targetID.String()).
		WithRequest(
			c.RealIP(),
			c.Request().UserAgent(),
			c.Response().Header().Get(echo.HeaderXRequestID),
		).
		WithDetails(map[string]interface{}{
			"target_username": targetName,
		})

	_ = a.logger.Log(c.Request().Context(), entry)
}

// LogUserUnlock logs a user unlock event.
func (a *AuditAction) LogUserUnlock(c echo.Context, actorID uuid.UUID, actorName string, targetID uuid.UUID, targetName string) {
	entry := NewEntry(ActionUserUnlock, StatusSuccess).
		WithUser(actorID, actorName).
		WithResource("user", targetID.String()).
		WithRequest(
			c.RealIP(),
			c.Request().UserAgent(),
			c.Response().Header().Get(echo.HeaderXRequestID),
		).
		WithDetails(map[string]interface{}{
			"target_username": targetName,
		})

	_ = a.logger.Log(c.Request().Context(), entry)
}

// LogConfigChange logs a configuration change event.
func (a *AuditAction) LogConfigChange(c echo.Context, userID uuid.UUID, username string, configKey string, oldValue, newValue interface{}) {
	entry := NewEntry(ActionConfigChange, StatusSuccess).
		WithUser(userID, username).
		WithResource("config", configKey).
		WithRequest(
			c.RealIP(),
			c.Request().UserAgent(),
			c.Response().Header().Get(echo.HeaderXRequestID),
		).
		WithChange(oldValue, newValue)

	_ = a.logger.Log(c.Request().Context(), entry)
}
