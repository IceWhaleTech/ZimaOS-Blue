package browser

import "testing"

func TestExpandTrustedSitePresets(t *testing.T) {
	expanded := ExpandTrustedSitePresets([]string{SitePresetBrowserCommon})
	if len(expanded) == 0 {
		t.Fatal("ExpandTrustedSitePresets() returned no entries")
	}
	found := false
	for _, site := range expanded {
		if site == "https://www.zhihu.com" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected zhihu origin in preset expansion, got %v", expanded)
	}
}

func TestExpandRelayFocusedPresetIsTight(t *testing.T) {
	expanded := ExpandRelayPreferredSitePresets([]string{SitePresetRelayFocused})
	if len(expanded) == 0 {
		t.Fatal("ExpandRelayPreferredSitePresets() returned no entries for focused preset")
	}
	foundZhihu := false
	foundReuters := false
	for _, site := range expanded {
		if site == "zhihu.com" {
			foundZhihu = true
		}
		if site == "reuters.com" {
			foundReuters = true
		}
	}
	if !foundZhihu {
		t.Fatalf("expected zhihu.com in focused relay preset, got %v", expanded)
	}
	if foundReuters {
		t.Fatalf("did not expect reuters.com in focused relay preset, got %v", expanded)
	}
}

func TestMatchSitePattern(t *testing.T) {
	if !MatchSitePattern("https://www.zhihu.com/question/1", "zhihu.com") {
		t.Fatal("expected host pattern to match subdomain")
	}
	if MatchSitePattern("https://www.zhihu.com/question/1", "https://zhihu.com") {
		t.Fatal("did not expect exact origin pattern to match different origin")
	}
	if !MatchSitePattern("https://finance.yahoo.com/quote/AAPL", "finance.yahoo.com") {
		t.Fatal("expected nested host pattern to match")
	}
}

func TestConfigExpandedRelayPreferredSites(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RelayPreferredSites = []string{"zhihu.com"}
	cfg.RelayPreferredSitePresets = []string{SitePresetBrowserCommon}
	expanded := cfg.ExpandedRelayPreferredSites()
	if len(expanded) < 2 {
		t.Fatalf("ExpandedRelayPreferredSites() = %v, want merged entries", expanded)
	}
	if got := cfg.RelayPreferredFallback(); got != "managed" {
		t.Fatalf("RelayPreferredFallback() = %q, want managed", got)
	}
}
