package providerpool

import "testing"

func TestLookupLegacyProviderDefaults(t *testing.T) {
	tests := []struct {
		id       string
		ok       bool
		name     string
		baseURL  string
		priority int
		typ      ProviderType
	}{
		{
			id:       "anthropic",
			ok:       true,
			name:     "Anthropic",
			baseURL:  "https://api.anthropic.com",
			priority: 100,
			typ:      ProviderTypeBuiltin,
		},
		{
			id:       "custom",
			ok:       true,
			name:     "Custom Provider",
			baseURL:  "",
			priority: 40,
			typ:      ProviderTypeCustom,
		},
		{
			id:  "unknown-provider",
			ok:  false,
			typ: ProviderTypeCustom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			defaults, ok := lookupLegacyProviderDefaults(tt.id)
			if ok != tt.ok {
				t.Fatalf("lookupLegacyProviderDefaults(%q) ok = %v, want %v", tt.id, ok, tt.ok)
			}
			if defaults.DisplayName != tt.name {
				t.Fatalf("lookupLegacyProviderDefaults(%q).DisplayName = %q, want %q", tt.id, defaults.DisplayName, tt.name)
			}
			if defaults.BaseURL != tt.baseURL {
				t.Fatalf("lookupLegacyProviderDefaults(%q).BaseURL = %q, want %q", tt.id, defaults.BaseURL, tt.baseURL)
			}
			if defaults.Priority != tt.priority {
				t.Fatalf("lookupLegacyProviderDefaults(%q).Priority = %d, want %d", tt.id, defaults.Priority, tt.priority)
			}
			if defaults.Type != tt.typ {
				t.Fatalf("lookupLegacyProviderDefaults(%q).Type = %q, want %q", tt.id, defaults.Type, tt.typ)
			}
		})
	}
}
