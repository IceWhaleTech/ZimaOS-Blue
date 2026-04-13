package providerpool

import (
	"net/url"
	"strings"
	"time"
)

func normalizeTrialBaseURL(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", false
	}
	if !strings.EqualFold(parsed.Scheme, "https") || strings.TrimSpace(parsed.Host) == "" {
		return "", false
	}

	return parsed.String(), true
}

func normalizeTrialProviderSecurity(provider *Provider) bool {
	if provider == nil {
		return false
	}
	if provider.ID != TrialProviderID && provider.Type != ProviderTypeTrial {
		return false
	}

	changed := false
	resetParsedURL := false

	if provider.SkipTLSVerify {
		provider.SkipTLSVerify = false
		changed = true
	}
	if provider.DetectedEndpoint != "" {
		provider.DetectedEndpoint = ""
		changed = true
		resetParsedURL = true
	}
	if provider.DetectedFormat != "" {
		provider.DetectedFormat = ""
		changed = true
	}
	if !provider.DetectedAt.IsZero() {
		provider.DetectedAt = time.Time{}
		changed = true
	}
	if len(provider.AlternateBaseURLs) > 0 {
		provider.AlternateBaseURLs = nil
		changed = true
	}

	if canonical := GetBuiltinProvider(TrialProviderID); canonical != nil {
		if provider.Type != ProviderTypeTrial {
			provider.Type = ProviderTypeTrial
			changed = true
		}
		if provider.Location != canonical.Location {
			provider.Location = canonical.Location
			changed = true
		}
		if provider.BaseURL != canonical.BaseURL {
			provider.BaseURL = canonical.BaseURL
			changed = true
			resetParsedURL = true
		}
		if provider.APIFormat != canonical.APIFormat {
			provider.APIFormat = canonical.APIFormat
			changed = true
		}
		if provider.APIFormatMode != APIFormatModePinned {
			provider.APIFormatMode = APIFormatModePinned
			changed = true
		}
	}

	if resetParsedURL {
		provider.ResetParsedURL()
	}

	return changed
}
