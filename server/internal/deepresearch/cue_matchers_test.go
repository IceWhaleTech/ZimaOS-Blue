package deepresearch

import "testing"

func TestQueryIntentMatchers(t *testing.T) {
	t.Run("knowledge base style", func(t *testing.T) {
		if !queryWantsKnowledgeBaseStyle("please build a knowledge base for this topic") {
			t.Fatal("expected knowledge-base query to be detected")
		}
		if queryWantsKnowledgeBaseStyle("just give me a short answer") {
			t.Fatal("expected short-answer query not to request knowledge-base style")
		}
	})

	t.Run("latest", func(t *testing.T) {
		if !queryWantsLatest("What are the latest updates on this project?") {
			t.Fatal("expected latest query to be detected")
		}
		if !queryWantsLatest("帮我看一下这个产品今年的动态") {
			t.Fatal("expected Chinese freshness query to be detected")
		}
		if queryWantsLatest("Explain the architecture of this project") {
			t.Fatal("expected static architecture query not to be marked as latest")
		}
	})

	t.Run("claim validation", func(t *testing.T) {
		if !looksLikeClaimValidation("Did the company officially confirm the rollout?") {
			t.Fatal("expected English claim-validation query to be detected")
		}
		if !looksLikeClaimValidation("这件事是真的假的，帮我核验一下") {
			t.Fatal("expected Chinese claim-validation query to be detected")
		}
		if looksLikeClaimValidation("Please summarize the product roadmap") {
			t.Fatal("expected summary query not to be marked as claim validation")
		}
	})
}

func TestHasOfficialLikeEvidence_UsesCompiledCueMatchers(t *testing.T) {
	if !hasOfficialLikeEvidence([]Evidence{{
		Title:   "Official documentation",
		URL:     "https://example.com/docs",
		Snippet: "official guide",
		Domain:  "example.com",
	}}) {
		t.Fatal("expected official/documentation cues to count as official-like evidence")
	}

	if !hasOfficialLikeEvidence([]Evidence{{
		Title:  "Agency notice",
		URL:    "https://agency.gov/notice",
		Domain: "agency.gov",
	}}) {
		t.Fatal("expected .gov domain to count as official-like evidence")
	}

	if hasOfficialLikeEvidence([]Evidence{{
		Title:   "Community blog recap",
		URL:     "https://blog.example.com/post",
		Snippet: "opinionated summary",
		Domain:  "blog.example.com",
	}}) {
		t.Fatal("expected generic blog evidence not to count as official-like evidence")
	}
}

func TestClassifyClaimPolarity_UsesCompiledCueMatchers(t *testing.T) {
	if got := classifyClaimPolarity("Official update confirms rollout"); got != claimPolarityPositive {
		t.Fatalf("positive polarity = %v, want %v", got, claimPolarityPositive)
	}
	if got := classifyClaimPolarity("Rumor says rollout is not happening"); got != claimPolarityNegative {
		t.Fatalf("negative polarity = %v, want %v", got, claimPolarityNegative)
	}
	if got := classifyClaimPolarity("General commentary without clear verification"); got != claimPolarityNeutral {
		t.Fatalf("neutral polarity = %v, want %v", got, claimPolarityNeutral)
	}
}
