package security

import "testing"

func TestSystemEnvironmentUsesStableDevelopmentMessage(t *testing.T) {
	scanner := NewSecurityScanner(nil, &ScannerConfig{
		Environment: "development",
	})

	items := scanner.checkSystemSecurity()
	for _, item := range items {
		if item.ID != "system_environment" {
			continue
		}
		if item.Details != "Running in development mode. Ensure production settings before deployment." {
			t.Fatalf("Details = %q", item.Details)
		}
		return
	}

	t.Fatal("system_environment item not found")
}

func TestJWTTokenExpirationUsesStableNotSetMessage(t *testing.T) {
	scanner := NewSecurityScanner(nil, &ScannerConfig{
		JWTSecretLength: 32,
		JWTExpirySecs:   0,
	})

	items := scanner.checkJWTSecurity()
	for _, item := range items {
		if item.ID != "auth_token_expiration" {
			continue
		}
		if item.Details != "Token expiration is not set. Check security.jwt.expiration in the loaded security configuration." {
			t.Fatalf("Details = %q", item.Details)
		}
		return
	}

	t.Fatal("auth_token_expiration item not found")
}

func TestJWTTokenExpirationUsesStableTooLongMessage(t *testing.T) {
	scanner := NewSecurityScanner(nil, &ScannerConfig{
		JWTSecretLength: 32,
		JWTExpirySecs:   9 * 3600,
	})

	items := scanner.checkJWTSecurity()
	for _, item := range items {
		if item.ID != "auth_token_expiration" {
			continue
		}
		if item.Details != "Token expiration exceeds 8 hours. This increases risk of token theft." {
			t.Fatalf("Details = %q", item.Details)
		}
		return
	}

	t.Fatal("auth_token_expiration item not found")
}
