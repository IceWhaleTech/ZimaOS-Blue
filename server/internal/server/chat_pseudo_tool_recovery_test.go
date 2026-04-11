package server

import "testing"

func TestPseudoXMLToolTagCandidates_UsesNativeDocumentToolsByDefault(t *testing.T) {
	candidates := pseudoXMLToolTagCandidates(nil)
	set := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		set[candidate] = struct{}{}
	}

	for _, required := range []string{"docx", "xlsx", "pptx"} {
		if _, ok := set[required]; !ok {
			t.Fatalf("expected %q in default pseudo tool candidates, got=%v", required, candidates)
		}
	}
	if _, ok := set["office"]; ok {
		t.Fatalf("did not expect legacy office pseudo tool candidate, got=%v", candidates)
	}
}
