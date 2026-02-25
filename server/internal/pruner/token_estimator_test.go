package pruner

import "testing"

func TestCompactMarkdown_BlankLines(t *testing.T) {
	input := "# Title\n\n\n\n\nSome text\n\n\n\nMore text"
	got := CompactMarkdown(input)
	want := "# Title\n\n\nSome text\n\n\nMore text"
	if got != want {
		t.Errorf("blank lines:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestCompactMarkdown_HTMLComments(t *testing.T) {
	input := "# Title\n<!-- this is a comment -->\nContent"
	got := CompactMarkdown(input)
	want := "# Title\n\nContent"
	if got != want {
		t.Errorf("html comments:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestCompactMarkdown_MultilineHTMLComment(t *testing.T) {
	input := "Before\n<!--\nmulti\nline\n-->\nAfter"
	got := CompactMarkdown(input)
	want := "Before\n\nAfter"
	if got != want {
		t.Errorf("multiline html comment:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestCompactMarkdown_EmptySection(t *testing.T) {
	input := "# Title\n\n## Facts\n\n## Preferences\n\nSome prefs here"
	got := CompactMarkdown(input)
	want := "# Title\n\n## Preferences\n\nSome prefs here"
	if got != want {
		t.Errorf("empty section:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestCompactMarkdown_EmptySectionAtEOF(t *testing.T) {
	input := "# Title\n\n## Facts\n\n"
	got := CompactMarkdown(input)
	want := ""
	if got != want {
		t.Errorf("empty section at EOF:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestCompactMarkdown_TrailingWhitespace(t *testing.T) {
	input := "# Title   \nSome text  \t\n"
	got := CompactMarkdown(input)
	want := "# Title\nSome text"
	if got != want {
		t.Errorf("trailing whitespace:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestCompactMarkdown_Empty(t *testing.T) {
	if got := CompactMarkdown(""); got != "" {
		t.Errorf("empty input: got %q", got)
	}
}

func TestCompactMarkdown_PreservesContent(t *testing.T) {
	// Real-world template: should keep non-empty sections
	input := `# Blue - ZimaOS AI Assistant

## Identity
You are Blue, the AI assistant for ZimaOS.

## Core Values
- Be genuinely helpful
- Have opinions
`
	got := CompactMarkdown(input)
	want := "# Blue - ZimaOS AI Assistant\n\n## Identity\nYou are Blue, the AI assistant for ZimaOS.\n\n## Core Values\n- Be genuinely helpful\n- Have opinions"
	if got != want {
		t.Errorf("preserve content:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestCompactMarkdown_MemoryTemplate(t *testing.T) {
	// The memory template has multiple empty sections
	input := `# Long-Term Memory

*Blue maintains this file automatically.*

## Facts

## Preferences

## Lessons Learned
`
	got := CompactMarkdown(input)
	// Italic hint, empty sections, and empty fields are all stripped.
	// Only "# Long-Term Memory" heading would remain — but onlyHeadings
	// detects this and returns "" so the file is skipped entirely.
	want := ""
	if got != want {
		t.Errorf("memory template:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestCompactMarkdown_UserTemplate(t *testing.T) {
	// USER.md template: italic hint + all empty fields → should compact to ""
	input := `# About You

*Blue will learn about you over time. You can also edit it directly.*

- **Name:**
- **What to call you:**
- **Timezone:**
- **Language preference:**
- **Notes:**
`
	got := CompactMarkdown(input)
	want := ""
	if got != want {
		t.Errorf("user template:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestEstimateTokens_ASCII(t *testing.T) {
	got := EstimateTokens("hello world test")
	// 16 chars / 4 = 4
	if got != 4 {
		t.Errorf("ASCII: got %d, want 4", got)
	}
}

func TestEstimateTokens_Empty(t *testing.T) {
	if got := EstimateTokens(""); got != 0 {
		t.Errorf("empty: got %d, want 0", got)
	}
}
