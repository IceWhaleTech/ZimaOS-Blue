package oidc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewDiscoveryHandler(t *testing.T) {
	issuer := "https://example.com"
	handler := NewDiscoveryHandler(issuer)

	if handler == nil {
		t.Fatal("NewDiscoveryHandler() returned nil")
	}
	if handler.issuer != issuer {
		t.Errorf("NewDiscoveryHandler() issuer = %v, want %v", handler.issuer, issuer)
	}
}

func TestDiscoveryHandler_GetDiscoveryDocument(t *testing.T) {
	issuer := "https://example.com"
	handler := NewDiscoveryHandler(issuer)

	doc := handler.GetDiscoveryDocument()

	if doc.Issuer != issuer {
		t.Errorf("GetDiscoveryDocument() Issuer = %v, want %v", doc.Issuer, issuer)
	}
	if doc.AuthorizationEndpoint != issuer+"/oauth/authorize" {
		t.Errorf("GetDiscoveryDocument() AuthorizationEndpoint = %v, want %v", doc.AuthorizationEndpoint, issuer+"/oauth/authorize")
	}
	if doc.TokenEndpoint != issuer+"/oauth/token" {
		t.Errorf("GetDiscoveryDocument() TokenEndpoint = %v, want %v", doc.TokenEndpoint, issuer+"/oauth/token")
	}
	if doc.UserInfoEndpoint != issuer+"/oauth/userinfo" {
		t.Errorf("GetDiscoveryDocument() UserInfoEndpoint = %v, want %v", doc.UserInfoEndpoint, issuer+"/oauth/userinfo")
	}
	if doc.JwksURI != issuer+"/.well-known/jwks.json" {
		t.Errorf("GetDiscoveryDocument() JwksURI = %v, want %v", doc.JwksURI, issuer+"/.well-known/jwks.json")
	}
	if doc.RevocationEndpoint != issuer+"/oauth/revoke" {
		t.Errorf("GetDiscoveryDocument() RevocationEndpoint = %v, want %v", doc.RevocationEndpoint, issuer+"/oauth/revoke")
	}
	if doc.IntrospectionEndpoint != issuer+"/oauth/introspect" {
		t.Errorf("GetDiscoveryDocument() IntrospectionEndpoint = %v, want %v", doc.IntrospectionEndpoint, issuer+"/oauth/introspect")
	}
}

func TestDiscoveryHandler_GetDiscoveryDocument_Scopes(t *testing.T) {
	handler := NewDiscoveryHandler("https://example.com")
	doc := handler.GetDiscoveryDocument()

	expectedScopes := SupportedScopes()
	if len(doc.ScopesSupported) != len(expectedScopes) {
		t.Errorf("GetDiscoveryDocument() ScopesSupported length = %d, want %d", len(doc.ScopesSupported), len(expectedScopes))
	}

	scopeMap := make(map[string]bool)
	for _, s := range doc.ScopesSupported {
		scopeMap[s] = true
	}

	for _, expected := range expectedScopes {
		if !scopeMap[expected] {
			t.Errorf("GetDiscoveryDocument() missing scope %v", expected)
		}
	}
}

func TestDiscoveryHandler_GetDiscoveryDocument_GrantTypes(t *testing.T) {
	handler := NewDiscoveryHandler("https://example.com")
	doc := handler.GetDiscoveryDocument()

	expectedGrants := SupportedGrantTypes()
	if len(doc.GrantTypesSupported) != len(expectedGrants) {
		t.Errorf("GetDiscoveryDocument() GrantTypesSupported length = %d, want %d", len(doc.GrantTypesSupported), len(expectedGrants))
	}
}

func TestDiscoveryHandler_GetDiscoveryDocument_SigningAlgorithms(t *testing.T) {
	handler := NewDiscoveryHandler("https://example.com")
	doc := handler.GetDiscoveryDocument()

	if len(doc.IDTokenSigningAlgValuesSupported) == 0 {
		t.Error("GetDiscoveryDocument() IDTokenSigningAlgValuesSupported is empty")
	}

	hasRS256 := false
	for _, alg := range doc.IDTokenSigningAlgValuesSupported {
		if alg == "RS256" {
			hasRS256 = true
			break
		}
	}
	if !hasRS256 {
		t.Error("GetDiscoveryDocument() IDTokenSigningAlgValuesSupported missing RS256")
	}
}

func TestDiscoveryHandler_GetDiscoveryDocument_PKCEMethods(t *testing.T) {
	handler := NewDiscoveryHandler("https://example.com")
	doc := handler.GetDiscoveryDocument()

	if len(doc.CodeChallengeMethodsSupported) == 0 {
		t.Error("GetDiscoveryDocument() CodeChallengeMethodsSupported is empty")
	}

	hasS256 := false
	for _, method := range doc.CodeChallengeMethodsSupported {
		if method == "S256" {
			hasS256 = true
			break
		}
	}
	if !hasS256 {
		t.Error("GetDiscoveryDocument() CodeChallengeMethodsSupported missing S256")
	}
}

func TestDiscoveryHandler_ServeHTTP(t *testing.T) {
	handler := NewDiscoveryHandler("https://example.com")

	req := httptest.NewRequest(http.MethodGet, "/.well-known/openid-configuration", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ServeHTTP() status = %d, want %d", w.Code, http.StatusOK)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("ServeHTTP() Content-Type = %v, want application/json", contentType)
	}

	cacheControl := w.Header().Get("Cache-Control")
	if cacheControl != "public, max-age=3600" {
		t.Errorf("ServeHTTP() Cache-Control = %v, want public, max-age=3600", cacheControl)
	}

	var doc DiscoveryDocument
	if err := json.NewDecoder(w.Body).Decode(&doc); err != nil {
		t.Fatalf("ServeHTTP() failed to decode response: %v", err)
	}

	if doc.Issuer != "https://example.com" {
		t.Errorf("ServeHTTP() response Issuer = %v, want https://example.com", doc.Issuer)
	}
}

func TestDiscoveryHandler_ServeHTTP_MethodNotAllowed(t *testing.T) {
	handler := NewDiscoveryHandler("https://example.com")

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		req := httptest.NewRequest(method, "/.well-known/openid-configuration", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("ServeHTTP(%s) status = %d, want %d", method, w.Code, http.StatusMethodNotAllowed)
		}
	}
}

func TestDiscoveryDocument_JSON(t *testing.T) {
	doc := &DiscoveryDocument{
		Issuer:                "https://example.com",
		AuthorizationEndpoint: "https://example.com/oauth/authorize",
		TokenEndpoint:         "https://example.com/oauth/token",
		UserInfoEndpoint:      "https://example.com/oauth/userinfo",
		JwksURI:               "https://example.com/.well-known/jwks.json",
		ScopesSupported:       []string{"openid", "profile", "email"},
		ResponseTypesSupported: []string{"code"},
		GrantTypesSupported:   []string{"authorization_code", "refresh_token"},
		SubjectTypesSupported: []string{"public"},
		IDTokenSigningAlgValuesSupported: []string{"RS256"},
	}

	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded DiscoveryDocument
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.Issuer != doc.Issuer {
		t.Errorf("JSON roundtrip Issuer = %v, want %v", decoded.Issuer, doc.Issuer)
	}
}
