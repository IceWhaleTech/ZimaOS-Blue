package selector

import "testing"

func TestAnalyzeQuery_UsesCompiledStaticCueMatchers(t *testing.T) {
	t.Run("url and live web", func(t *testing.T) {
		signals := AnalyzeQuery("Open https://example.com/docs and check the latest news.")
		if !signals.URLPresent {
			t.Fatal("expected URLPresent for raw URL query")
		}
		if !signals.LiveWeb {
			t.Fatal("expected LiveWeb for URL/latest query")
		}
	})

	t.Run("workspace file task", func(t *testing.T) {
		signals := AnalyzeQuery("Read report.md from my workspace and summarize it.")
		if !signals.LocalWorkspace {
			t.Fatal("expected LocalWorkspace for workspace file task")
		}
	})

	t.Run("workspace summarize query stays local", func(t *testing.T) {
		signals := AnalyzeQuery("Review files in the workspace and summarize the report.")
		if !signals.LocalWorkspace {
			t.Fatalf("expected LocalWorkspace for workspace summarize query, got %+v", signals)
		}
		if signals.LiveWeb {
			t.Fatalf("did not expect LiveWeb for workspace summarize query, got %+v", signals)
		}
		if signals.Productivity {
			t.Fatalf("did not expect Productivity for workspace summarize query, got %+v", signals)
		}
	})

	t.Run("email cli query stays productivity", func(t *testing.T) {
		signals := AnalyzeQuery("Search my IMAP inbox for unread mail from Alice and reply from the terminal.")
		if !signals.Productivity {
			t.Fatalf("expected Productivity for email CLI query, got %+v", signals)
		}
		if signals.LiveWeb {
			t.Fatalf("did not expect LiveWeb for email CLI query, got %+v", signals)
		}
		if signals.LocalWorkspace {
			t.Fatalf("did not expect LocalWorkspace for email CLI query, got %+v", signals)
		}
	})

	t.Run("howto meta", func(t *testing.T) {
		signals := AnalyzeQuery("How to use this tool mode?")
		if !signals.QuestionPrefix || !signals.HowToQuestion || !signals.MetaIntent {
			t.Fatalf("expected how-to meta query, got %+v", signals)
		}
	})

	t.Run("plain reply", func(t *testing.T) {
		signals := AnalyzeQuery("Just reply with hello.")
		if !signals.PlainReply {
			t.Fatal("expected plain reply query")
		}
		if !signals.BlocksAutomaticSelection() {
			t.Fatal("expected plain reply to block automatic selection")
		}
	})

	t.Run("smalltalk suppressed by operational cue", func(t *testing.T) {
		if !AnalyzeQuery("hello there").Smalltalk {
			t.Fatal("expected greeting to be treated as smalltalk")
		}
		if AnalyzeQuery("hello, open https://example.com").Smalltalk {
			t.Fatal("expected operational URL query not to be treated as smalltalk")
		}
	})

	t.Run("negation", func(t *testing.T) {
		if !AnalyzeQuery("Don't search the web for this.").Negated {
			t.Fatal("expected negated operational query")
		}
	})
}

func TestContainsTerm_HandlesBoundaryAndPunctuationTerms(t *testing.T) {
	if !ContainsTerm("switch mode now", "mode") {
		t.Fatal("expected standalone ASCII term to match")
	}
	if ContainsTerm("the model changed", "mode") {
		t.Fatal("expected ASCII boundary to block partial word match")
	}
	if !ContainsTerm("visit https://example.com", "https://") {
		t.Fatal("expected punctuation term to match inside URL")
	}
	if !ContainsTerm("review report.md", ".md") {
		t.Fatal("expected file extension term to match inside filename")
	}
}

func TestMatchProfile_UsesCompiledProfileMatchers(t *testing.T) {
	signals := AnalyzeQuery("Search the web for latest sources and citations.")
	profile := SelectorProfile{
		Name:             "web_search",
		ExactAliases:     []string{"web_search"},
		Actions:          []string{"search", "look up", "find"},
		Objects:          []string{"web", "sources", "citations"},
		ContextCues:      []string{"latest", "news"},
		PreferredDomains: []string{DomainLiveWeb},
	}

	match := MatchProfile(signals, profile)
	if !match.Eligible {
		t.Fatalf("expected compiled profile to be eligible, got %+v", match)
	}
	if len(match.ActionHits) == 0 || len(match.ObjectHits) == 0 {
		t.Fatalf("expected action/object hits, got %+v", match)
	}
	if len(match.DomainHits) == 0 || match.DomainHits[0] != DomainLiveWeb {
		t.Fatalf("expected live_web domain hit, got %+v", match.DomainHits)
	}
}
