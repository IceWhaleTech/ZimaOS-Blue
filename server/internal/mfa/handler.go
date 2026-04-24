package mfa

import (
	"encoding/base64"
	"net/http"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// Handler handles HTTP requests for MFA operations.
type Handler struct {
	totp              *TOTP
	recovery          *Recovery
	webauthn          *WebAuthn
	credentialManager *CredentialManager
	// Session storage for WebAuthn ceremonies (in production, use Redis or similar)
	registrationSessions sync.Map // map[string]*RegistrationSession
}

// NewHandler creates a new MFA handler.
func NewHandler(totp *TOTP, recovery *Recovery) *Handler {
	if totp == nil {
		totp = NewTOTP(nil)
	}
	if recovery == nil {
		recovery = NewRecovery(nil)
	}

	// Initialize WebAuthn with default config
	webauthn, _ := NewWebAuthn(nil)

	// Initialize credential manager with in-memory store
	credStore := NewInMemoryCredentialStore()
	credManager := NewCredentialManager(credStore, nil)

	return &Handler{
		totp:              totp,
		recovery:          recovery,
		webauthn:          webauthn,
		credentialManager: credManager,
	}
}

// NewHandlerWithWebAuthn creates a new MFA handler with custom WebAuthn configuration.
func NewHandlerWithWebAuthn(totp *TOTP, recovery *Recovery, webauthnConfig *WebAuthnConfig, credStore CredentialStore) *Handler {
	if totp == nil {
		totp = NewTOTP(nil)
	}
	if recovery == nil {
		recovery = NewRecovery(nil)
	}

	webauthn, _ := NewWebAuthn(webauthnConfig)

	if credStore == nil {
		credStore = NewInMemoryCredentialStore()
	}
	credManager := NewCredentialManager(credStore, nil)

	return &Handler{
		totp:              totp,
		recovery:          recovery,
		webauthn:          webauthn,
		credentialManager: credManager,
	}
}

// RegisterRoutes registers the MFA routes.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	mfa := g.Group("/auth/mfa")
	mfa.POST("/setup", h.Setup)
	mfa.POST("/verify", h.Verify)
	mfa.POST("/disable", h.Disable)
	mfa.GET("/status", h.Status)
	mfa.GET("/recovery", h.GetRecoveryCodes)
	mfa.POST("/recovery/regenerate", h.RegenerateRecoveryCodes)

	// WebAuthn routes
	webauthn := g.Group("/auth/webauthn")
	webauthn.GET("/status", h.WebAuthnStatus)
	webauthn.POST("/register/begin", h.WebAuthnRegisterBegin)
	webauthn.POST("/register/finish", h.WebAuthnRegisterFinish)
	webauthn.DELETE("/credentials/:id", h.WebAuthnDeleteCredential)
}

// SetupResponseDTO represents the response for MFA setup.
type SetupResponseDTO struct {
	Secret string `json:"secret"`
	URI    string `json:"uri"`
}

// Setup handles MFA setup initiation.
func (h *Handler) Setup(c echo.Context) error {
	userID := getUserIDFromContext(c)
	if userID == uuid.Nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	username := getUsernameFromContext(c)
	if username == "" {
		username = userID.String()
	}

	setup, err := h.totp.Setup(username)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate MFA setup")
	}

	// Store the secret temporarily in session/context for verification
	// In a real implementation, this would be stored in a temporary store
	// and associated with the user's session
	c.Set("mfa_setup_secret", setup.Secret)

	return c.JSON(http.StatusOK, &SetupResponseDTO{
		Secret: setup.Secret,
		URI:    setup.URI,
	})
}

// VerifyRequest represents a request to verify MFA setup.
type VerifyRequest struct {
	Code   string `json:"code" validate:"required"`
	Secret string `json:"secret" validate:"required"`
}

// VerifyResponse represents the response for MFA verification.
type VerifyResponse struct {
	Enabled       bool     `json:"enabled"`
	RecoveryCodes []string `json:"recovery_codes,omitempty"`
}

// Verify handles MFA setup verification.
func (h *Handler) Verify(c echo.Context) error {
	userID := getUserIDFromContext(c)
	if userID == uuid.Nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	var req VerifyRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	// Validate the TOTP code
	if !h.totp.ValidateWithSkew(req.Code, req.Secret) {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid verification code")
	}

	// Generate recovery codes
	recoveryCodes, err := h.recovery.Generate()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate recovery codes")
	}

	// In a real implementation, we would:
	// 1. Store the encrypted MFA secret in the user record
	// 2. Store the hashed recovery codes
	// 3. Set MFA enabled flag to true

	// Format recovery codes for display
	formattedCodes := h.recovery.FormatCodesForDisplay(recoveryCodes.Codes)

	return c.JSON(http.StatusOK, &VerifyResponse{
		Enabled:       true,
		RecoveryCodes: formattedCodes,
	})
}

// DisableRequest represents a request to disable MFA.
type DisableRequest struct {
	Code     string `json:"code"`
	Password string `json:"password" validate:"required"`
}

// Disable handles MFA disabling.
func (h *Handler) Disable(c echo.Context) error {
	userID := getUserIDFromContext(c)
	if userID == uuid.Nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	var req DisableRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	// In a real implementation, we would:
	// 1. Verify the password
	// 2. Optionally verify a TOTP code
	// 3. Clear the MFA secret and recovery codes
	// 4. Set MFA enabled flag to false

	return c.JSON(http.StatusOK, map[string]bool{"enabled": false})
}

// StatusResponse represents the MFA status response.
type StatusResponse struct {
	Enabled       bool `json:"enabled"`
	RecoveryCount int  `json:"recovery_codes_remaining"`
	SetupRequired bool `json:"setup_required,omitempty"`
}

// Status handles getting MFA status.
func (h *Handler) Status(c echo.Context) error {
	userID := getUserIDFromContext(c)
	if userID == uuid.Nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	// In a real implementation, we would fetch the user's MFA status from the database
	// For now, return a placeholder response

	return c.JSON(http.StatusOK, &StatusResponse{
		Enabled:       false,
		RecoveryCount: 0,
		SetupRequired: false,
	})
}

// RecoveryCodesResponse represents the recovery codes response.
type RecoveryCodesResponse struct {
	Codes     []string `json:"codes"`
	Remaining int      `json:"remaining"`
}

// GetRecoveryCodes handles getting recovery codes.
func (h *Handler) GetRecoveryCodes(c echo.Context) error {
	userID := getUserIDFromContext(c)
	if userID == uuid.Nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	// In a real implementation, we would:
	// 1. Check if MFA is enabled
	// 2. Return masked recovery codes (e.g., "XXXX-1234")
	// 3. Return the count of remaining codes

	return c.JSON(http.StatusOK, &RecoveryCodesResponse{
		Codes:     []string{},
		Remaining: 0,
	})
}

// RegenerateRecoveryCodes handles regenerating recovery codes.
func (h *Handler) RegenerateRecoveryCodes(c echo.Context) error {
	userID := getUserIDFromContext(c)
	if userID == uuid.Nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	// In a real implementation, we would:
	// 1. Verify the user's password or TOTP code
	// 2. Generate new recovery codes
	// 3. Store the hashed codes
	// 4. Return the plaintext codes (only time they're shown)

	recoveryCodes, err := h.recovery.Generate()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate recovery codes")
	}

	formattedCodes := h.recovery.FormatCodesForDisplay(recoveryCodes.Codes)

	return c.JSON(http.StatusOK, &RecoveryCodesResponse{
		Codes:     formattedCodes,
		Remaining: len(formattedCodes),
	})
}

// ValidateMFARequest represents a request to validate MFA during login.
type ValidateMFARequest struct {
	MFAToken     string `json:"mfa_token" validate:"required"`
	Code         string `json:"code"`
	RecoveryCode string `json:"recovery_code"`
}

// ValidateMFA validates MFA code during login.
// This would be called after initial password authentication.
func (h *Handler) ValidateMFA(c echo.Context) error {
	var req ValidateMFARequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	// Either code or recovery_code must be provided
	if req.Code == "" && req.RecoveryCode == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "code or recovery_code required")
	}

	// In a real implementation, we would:
	// 1. Validate the MFA token (from initial login)
	// 2. Get the user's MFA secret
	// 3. Validate the TOTP code or recovery code
	// 4. If recovery code, mark it as used
	// 5. Complete the login and return tokens

	return c.JSON(http.StatusOK, map[string]string{
		"status": "validated",
	})
}

// Helper functions

func getUserIDFromContext(c echo.Context) uuid.UUID {
	if id, ok := c.Get("user_id").(uuid.UUID); ok {
		return id
	}
	if idStr, ok := c.Get("user_id").(string); ok {
		if id, err := uuid.Parse(idStr); err == nil {
			return id
		}
	}
	return uuid.Nil
}

func getUsernameFromContext(c echo.Context) string {
	if username, ok := c.Get("username").(string); ok {
		return username
	}
	return ""
}

// WebAuthn Handler Methods

// WebAuthnStatusResponse represents the WebAuthn status response.
type WebAuthnStatusResponse struct {
	Enabled     bool                    `json:"enabled"`
	Credentials []WebAuthnCredentialDTO `json:"credentials"`
}

// WebAuthnCredentialDTO represents a WebAuthn credential for API responses.
type WebAuthnCredentialDTO struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	CreatedAt  string `json:"created_at"`
	LastUsedAt string `json:"last_used_at"`
}

// WebAuthnStatus returns the WebAuthn status for the current user.
func (h *Handler) WebAuthnStatus(c echo.Context) error {
	userID := getUserIDFromContext(c)
	if userID == uuid.Nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	// Check if WebAuthn is configured
	if h.webauthn == nil {
		return c.JSON(http.StatusOK, &WebAuthnStatusResponse{
			Enabled:     false,
			Credentials: []WebAuthnCredentialDTO{},
		})
	}

	// Get credentials from the credential manager
	creds, err := h.credentialManager.ListCredentials(c.Request().Context(), userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get credentials")
	}

	// Convert to DTOs
	credDTOs := make([]WebAuthnCredentialDTO, len(creds))
	for i, cred := range creds {
		credDTOs[i] = WebAuthnCredentialDTO{
			ID:         cred.ID,
			Name:       cred.Name,
			CreatedAt:  cred.CreatedAt.Format(time.RFC3339),
			LastUsedAt: cred.LastUsedAt.Format(time.RFC3339),
		}
	}

	return c.JSON(http.StatusOK, &WebAuthnStatusResponse{
		Enabled:     len(credDTOs) > 0,
		Credentials: credDTOs,
	})
}

// WebAuthnRegisterBeginRequest represents a request to begin WebAuthn registration.
type WebAuthnRegisterBeginRequest struct {
	Name string `json:"name" validate:"required"`
}

// WebAuthnRegisterBegin starts the WebAuthn registration ceremony.
func (h *Handler) WebAuthnRegisterBegin(c echo.Context) error {
	userID := getUserIDFromContext(c)
	if userID == uuid.Nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	if h.webauthn == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "WebAuthn not configured")
	}

	var req WebAuthnRegisterBeginRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.Name == "" {
		req.Name = "Security Key"
	}

	// Get username from context
	username := getUsernameFromContext(c)
	if username == "" {
		username = userID.String()
	}

	// Get existing credentials for the user
	existingCreds, err := h.credentialManager.GetRawCredentials(c.Request().Context(), userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get existing credentials")
	}

	// Create WebAuthn user
	webauthnUser := &WebAuthnUser{
		ID:          userID,
		Name:        username,
		DisplayName: username,
		Credentials: existingCreds,
	}

	// Begin registration ceremony
	options, session, err := h.webauthn.BeginRegistration(webauthnUser)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to begin registration: "+err.Error())
	}

	// Store session with credential name
	sessionKey := base64.URLEncoding.EncodeToString([]byte(session.Challenge))
	h.registrationSessions.Store(sessionKey, &registrationSessionWithName{
		Session: session,
		Name:    req.Name,
	})

	// Clean up old sessions after timeout
	go func() {
		time.Sleep(time.Duration(h.webauthn.config.Timeout) * time.Millisecond)
		h.registrationSessions.Delete(sessionKey)
	}()

	return c.JSON(http.StatusOK, options)
}

// registrationSessionWithName wraps RegistrationSession with credential name.
type registrationSessionWithName struct {
	Session *RegistrationSession
	Name    string
}

// WebAuthnRegisterFinish completes the WebAuthn registration ceremony.
func (h *Handler) WebAuthnRegisterFinish(c echo.Context) error {
	userID := getUserIDFromContext(c)
	if userID == uuid.Nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	if h.webauthn == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "WebAuthn not configured")
	}

	// Parse the credential creation response from request body
	parsedResponse, err := protocol.ParseCredentialCreationResponseBody(c.Request().Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to parse credential response: "+err.Error())
	}

	// Find the session by challenge (challenge is already a string in CollectedClientData)
	sessionKey := base64.URLEncoding.EncodeToString([]byte(parsedResponse.Response.CollectedClientData.Challenge))
	sessionData, ok := h.registrationSessions.Load(sessionKey)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "registration session not found or expired")
	}

	sessionWithName := sessionData.(*registrationSessionWithName)
	session := sessionWithName.Session

	// Verify the session belongs to this user
	if session.UserID != userID {
		return echo.NewHTTPError(http.StatusBadRequest, "session user mismatch")
	}

	// Check if session is expired
	if timeutil.NowTime().After(session.ExpiresAt) {
		h.registrationSessions.Delete(sessionKey)
		return echo.NewHTTPError(http.StatusBadRequest, "registration session expired")
	}

	// Get username from context
	username := getUsernameFromContext(c)
	if username == "" {
		username = userID.String()
	}

	// Get existing credentials for the user
	existingCreds, err := h.credentialManager.GetRawCredentials(c.Request().Context(), userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get existing credentials")
	}

	// Create WebAuthn user
	webauthnUser := &WebAuthnUser{
		ID:          userID,
		Name:        username,
		DisplayName: username,
		Credentials: existingCreds,
	}

	// Finish registration
	credential, err := h.webauthn.FinishRegistration(webauthnUser, session, parsedResponse)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to verify registration: "+err.Error())
	}

	// Set the credential name
	credential.Name = sessionWithName.Name

	// Store the credential
	if err := h.credentialManager.AddCredential(c.Request().Context(), userID, credential); err != nil {
		if err == ErrMaxCredentialsReached {
			return echo.NewHTTPError(http.StatusBadRequest, "maximum number of security keys reached")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to store credential")
	}

	// Clean up session
	h.registrationSessions.Delete(sessionKey)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"credential": WebAuthnCredentialDTO{
			ID:         base64.URLEncoding.EncodeToString(credential.ID),
			Name:       credential.Name,
			CreatedAt:  credential.CreatedAt.Format(time.RFC3339),
			LastUsedAt: credential.LastUsedAt.Format(time.RFC3339),
		},
	})
}

// WebAuthnDeleteCredential deletes a WebAuthn credential.
func (h *Handler) WebAuthnDeleteCredential(c echo.Context) error {
	userID := getUserIDFromContext(c)
	if userID == uuid.Nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	credentialID := c.Param("id")
	if credentialID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "credential ID required")
	}

	// Delete the credential using the credential manager
	if err := h.credentialManager.DeleteCredential(c.Request().Context(), userID, credentialID); err != nil {
		if err == ErrCredentialNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "credential not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete credential")
	}

	return c.JSON(http.StatusOK, map[string]bool{"success": true})
}
