package ide

import "testing"

func TestParseCodexCredentials_LegacyTopLevel(t *testing.T) {
	data := []byte(`{
		"access_token": "access-legacy",
		"refresh_token": "refresh-legacy",
		"account_id": "user_legacy",
		"expires_at": 1740000000000
	}`)

	token, err := parseCodexCredentials(data)
	if err != nil {
		t.Fatalf("parseCodexCredentials returned error: %v", err)
	}
	if token.AccessToken != "access-legacy" {
		t.Fatalf("access token mismatch: got %q", token.AccessToken)
	}
	if token.RefreshToken != "refresh-legacy" {
		t.Fatalf("refresh token mismatch: got %q", token.RefreshToken)
	}
	if token.Email != "user_legacy" {
		t.Fatalf("account id mismatch: got %q", token.Email)
	}
}

func TestParseCodexCredentials_NestedTokens(t *testing.T) {
	data := []byte(`{
		"tokens": {
			"access_token": "access-nested",
			"refresh_token": "refresh-nested",
			"account_id": "user_nested"
		},
		"last_refresh": "1740000000000"
	}`)

	token, err := parseCodexCredentials(data)
	if err != nil {
		t.Fatalf("parseCodexCredentials returned error: %v", err)
	}
	if token.AccessToken != "access-nested" {
		t.Fatalf("access token mismatch: got %q", token.AccessToken)
	}
	if token.RefreshToken != "refresh-nested" {
		t.Fatalf("refresh token mismatch: got %q", token.RefreshToken)
	}
	if token.Email != "user_nested" {
		t.Fatalf("account id mismatch: got %q", token.Email)
	}
}
