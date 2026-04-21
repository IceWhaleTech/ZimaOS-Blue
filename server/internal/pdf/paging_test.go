package pdf

import (
	"sort"
	"testing"
)

func TestResolveSelectedPages_DefaultSamplingIncludesTailPages(t *testing.T) {
	const pageCount = 155

	got, warnings, err := resolveSelectedPages(pageCount, nil, 0)
	if err != nil {
		t.Fatalf("resolveSelectedPages returned error: %v", err)
	}
	if len(got) != defaultMaxPages {
		t.Fatalf("len(pages) = %d, want %d", len(got), defaultMaxPages)
	}
	if !sort.IntsAreSorted(got) {
		t.Fatalf("pages not sorted: %#v", got)
	}
	if got[0] != 1 {
		t.Fatalf("pages[0] = %d, want 1", got[0])
	}
	if got[len(got)-1] != pageCount {
		t.Fatalf("pages[last] = %d, want %d", got[len(got)-1], pageCount)
	}
	if len(warnings) == 0 {
		t.Fatal("expected warnings when default selection samples a subset of pages")
	}
}

func TestExtractTOCPageTargets_FindsKeywordPageNumbers(t *testing.T) {
	text := "Table of Contents\n" +
		"1 Introduction .................................. 3\n" +
		"2 Approach and Methods ........................... 15\n" +
		"3 Safety & Alignment ............................. 42\n" +
		"4 Limitations .................................... 57\n" +
		"5 Conclusion ..................................... 60\n"

	got := extractTOCPageTargets(text, 155)
	if len(got) == 0 {
		t.Fatalf("extractTOCPageTargets returned empty targets")
	}
	for _, want := range []int{3, 15, 42, 57, 60} {
		found := false
		for _, page := range got {
			if page == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing toc target page %d in %#v", want, got)
		}
	}
}

func TestResolveDefaultSelectedPages_IncludesTOCTargetsAndTail(t *testing.T) {
	const pageCount = 155
	limit := defaultMaxPages
	tocTargets := []int{42, 57, 60}

	got, warnings := resolveDefaultSelectedPages(pageCount, limit, tocTargets)
	if len(got) != limit {
		t.Fatalf("len(pages) = %d, want %d", len(got), limit)
	}
	if !sort.IntsAreSorted(got) {
		t.Fatalf("pages not sorted: %#v", got)
	}
	if got[len(got)-1] != pageCount {
		t.Fatalf("pages[last] = %d, want %d", got[len(got)-1], pageCount)
	}
	for _, want := range tocTargets {
		found := false
		for _, page := range got {
			if page == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing toc target page %d in %#v", want, got)
		}
	}
	if len(warnings) == 0 {
		t.Fatal("expected warnings when default selection uses toc targets")
	}
}

func TestClampMaxChars_DefaultIsLargeEnoughForLongPDFSummaries(t *testing.T) {
	if got := clampMaxChars(0); got < 100000 {
		t.Fatalf("clampMaxChars(0) = %d, want at least 100000", got)
	}
}
