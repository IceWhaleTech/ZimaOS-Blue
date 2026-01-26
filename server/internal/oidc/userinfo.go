package oidc

import (
	"encoding/json"
	"net/http"
	"strings"
)

// UserInfoClaims represents the claims returned by the userinfo endpoint.
type UserInfoClaims struct {
	Sub               string `json:"sub"`
	Name              string `json:"name,omitempty"`
	PreferredUsername string `json:"preferred_username,omitempty"`
	Email             string `json:"email,omitempty"`
	EmailVerified     bool   `json:"email_verified,omitempty"`
}

// UserInfoProvider provides user information for the userinfo endpoint.
type UserInfoProvider interface {
	// GetUserInfo returns user information for the given user ID.
	GetUserInfo(userID string) (*UserInfoClaims, error)
}

// UserInfoHandler handles the userinfo endpoint.
type UserInfoHandler struct {
	tokenService *TokenService
	userProvider UserInfoProvider
}

// NewUserInfoHandler creates a new userinfo handler.
func NewUserInfoHandler(tokenService *TokenService, userProvider UserInfoProvider) *UserInfoHandler {
	return &UserInfoHandler{
		tokenService: tokenService,
		userProvider: userProvider,
	}
}

// ServeHTTP handles the userinfo endpoint request.
func (h *UserInfoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract bearer token
	token := extractBearerToken(r)
	if token == "" {
		w.Header().Set("WWW-Authenticate", "Bearer")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Validate token
	claims, err := h.tokenService.ValidateAccessToken(token)
	if err != nil {
		w.Header().Set("WWW-Authenticate", "Bearer error=\"invalid_token\"")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get user info
	userInfo, err := h.userProvider.GetUserInfo(claims.Subject)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Filter claims based on scope
	filteredClaims := filterClaimsByScope(userInfo, claims.Scope)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(filteredClaims); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// extractBearerToken extracts the bearer token from the request.
func extractBearerToken(r *http.Request) string {
	// Check Authorization header
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}

	// Check form body for POST requests
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err == nil {
			if token := r.FormValue("access_token"); token != "" {
				return token
			}
		}
	}

	// Check query parameter
	return r.URL.Query().Get("access_token")
}

// filterClaimsByScope filters user info claims based on the requested scopes.
func filterClaimsByScope(userInfo *UserInfoClaims, scopeString string) map[string]interface{} {
	scopes := strings.Split(scopeString, " ")
	scopeMap := make(map[string]bool)
	for _, s := range scopes {
		scopeMap[s] = true
	}

	result := map[string]interface{}{
		"sub": userInfo.Sub,
	}

	if scopeMap[ScopeProfile] {
		if userInfo.Name != "" {
			result["name"] = userInfo.Name
		}
		if userInfo.PreferredUsername != "" {
			result["preferred_username"] = userInfo.PreferredUsername
		}
	}

	if scopeMap[ScopeEmail] {
		if userInfo.Email != "" {
			result["email"] = userInfo.Email
			result["email_verified"] = userInfo.EmailVerified
		}
	}

	return result
}
