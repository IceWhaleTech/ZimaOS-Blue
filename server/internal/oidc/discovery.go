package oidc

import (
	"encoding/json"
	"net/http"
)

// DiscoveryDocument represents the OIDC discovery document.
type DiscoveryDocument struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	UserInfoEndpoint                  string   `json:"userinfo_endpoint"`
	JwksURI                           string   `json:"jwks_uri"`
	RevocationEndpoint                string   `json:"revocation_endpoint,omitempty"`
	IntrospectionEndpoint             string   `json:"introspection_endpoint,omitempty"`
	RegistrationEndpoint              string   `json:"registration_endpoint,omitempty"`
	ScopesSupported                   []string `json:"scopes_supported"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	ResponseModesSupported            []string `json:"response_modes_supported,omitempty"`
	GrantTypesSupported               []string `json:"grant_types_supported"`
	SubjectTypesSupported             []string `json:"subject_types_supported"`
	IDTokenSigningAlgValuesSupported  []string `json:"id_token_signing_alg_values_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
	ClaimsSupported                   []string `json:"claims_supported"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported,omitempty"`
}

// DiscoveryHandler handles the OIDC discovery endpoint.
type DiscoveryHandler struct {
	issuer string
}

// NewDiscoveryHandler creates a new discovery handler.
func NewDiscoveryHandler(issuer string) *DiscoveryHandler {
	return &DiscoveryHandler{issuer: issuer}
}

// GetDiscoveryDocument returns the OIDC discovery document.
func (h *DiscoveryHandler) GetDiscoveryDocument() *DiscoveryDocument {
	return &DiscoveryDocument{
		Issuer:                h.issuer,
		AuthorizationEndpoint: h.issuer + "/oauth/authorize",
		TokenEndpoint:         h.issuer + "/oauth/token",
		UserInfoEndpoint:      h.issuer + "/oauth/userinfo",
		JwksURI:               h.issuer + "/.well-known/jwks.json",
		RevocationEndpoint:    h.issuer + "/oauth/revoke",
		IntrospectionEndpoint: h.issuer + "/oauth/introspect",
		ScopesSupported:       SupportedScopes(),
		ResponseTypesSupported: []string{
			"code",
			"token",
			"id_token",
			"code token",
			"code id_token",
			"token id_token",
			"code token id_token",
		},
		ResponseModesSupported: []string{"query", "fragment"},
		GrantTypesSupported:    SupportedGrantTypes(),
		SubjectTypesSupported:  []string{"public"},
		IDTokenSigningAlgValuesSupported: []string{
			"RS256",
		},
		TokenEndpointAuthMethodsSupported: []string{
			"client_secret_basic",
			"client_secret_post",
			"none",
		},
		ClaimsSupported: []string{
			"iss",
			"sub",
			"aud",
			"exp",
			"iat",
			"auth_time",
			"nonce",
			"name",
			"email",
			"email_verified",
			"preferred_username",
		},
		CodeChallengeMethodsSupported: []string{
			"S256",
			"plain",
		},
	}
}

// ServeHTTP handles the discovery endpoint request.
func (h *DiscoveryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	doc := h.GetDiscoveryDocument()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")

	if err := json.NewEncoder(w).Encode(doc); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// JWKSHandler handles the JWKS endpoint.
type JWKSHandler struct {
	keyManager *KeyManager
}

// NewJWKSHandler creates a new JWKS handler.
func NewJWKSHandler(keyManager *KeyManager) *JWKSHandler {
	return &JWKSHandler{keyManager: keyManager}
}

// ServeHTTP handles the JWKS endpoint request.
func (h *JWKSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jwks := h.keyManager.GetJWKS()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")

	if err := json.NewEncoder(w).Encode(jwks); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}
