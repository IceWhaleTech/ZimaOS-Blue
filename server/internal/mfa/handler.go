package mfa

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// Handler handles HTTP requests for MFA operations.
type Handler struct {
	totp     *TOTP
	recovery *Recovery
	// userRepo is used to get/update user MFA settings
	// This would be injected from the user package
}

// NewHandler creates a new MFA handler.
func NewHandler(totp *TOTP, recovery *Recovery) *Handler {
	if totp == nil {
		totp = NewTOTP(nil)
	}
	if recovery == nil {
		recovery = NewRecovery(nil)
	}
	return &Handler{
		totp:     totp,
		recovery: recovery,
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
}

// SetupRequest represents a request to start MFA setup.
type SetupRequest struct {
	IncludeQRCode bool `json:"include_qr_code"`
}

// SetupResponseDTO represents the response for MFA setup.
type SetupResponseDTO struct {
	Secret string `json:"secret"`
	URI    string `json:"uri"`
	QRCode string `json:"qr_code,omitempty"`
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

	var req SetupRequest
	if err := c.Bind(&req); err != nil {
		req.IncludeQRCode = true // Default to including QR code
	}

	setup, err := h.totp.Setup(username, req.IncludeQRCode)
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
		QRCode: setup.QRCode,
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
	Enabled        bool `json:"enabled"`
	RecoveryCount  int  `json:"recovery_codes_remaining"`
	SetupRequired  bool `json:"setup_required,omitempty"`
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
