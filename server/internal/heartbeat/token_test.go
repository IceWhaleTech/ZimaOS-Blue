package heartbeat

import "testing"

func TestIsEffectivelyEmpty(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"empty string", "", true},
		{"whitespace only", "  \n\t\n  ", true},
		{"headers only", "# Title\n## Subtitle\n", true},
		{"empty list items", "- \n* \n- [ ]\n- [x]\n", true},
		{"code comments", "// this is a comment\n// another comment\n", true},
		{"html comment single line", "<!-- this is a comment -->", true},
		{"html comment multi line", "<!-- start\nsome text\n-->", true},
		{"yaml front matter", "---\ntitle: test\n---\n", true},
		{"emphasis only", "*Edit this file to configure heartbeat.*\n", true},
		{"underscore emphasis", "_This is a meta instruction_\n", true},
		{"mixed decorative", "# Heartbeat\n\n*Edit this file.*\n\n- \n", true},
		{"clawdbot template", "---\nsummary: test\n---\n\n# HEARTBEAT.md\n\n# Keep this file empty.\n\n# Add tasks below.\n", true},
		{"en template", "# Heartbeat Checklist\n\n*Edit this file to tell Blue what to check periodically.*\n\n## Checks\n- \n", true},

		// Non-empty cases
		{"has task", "# Heartbeat\n- Check disk usage", false},
		{"has text", "Check if server is running", false},
		{"list with content", "- [ ] Monitor CPU", false},
		{"real task after comments", "// comment\n- Check API health", false},
		{"text after front matter", "---\nkey: val\n---\nDo something", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsEffectivelyEmpty(tt.content)
			if got != tt.want {
				t.Errorf("IsEffectivelyEmpty(%q) = %v, want %v", tt.content, got, tt.want)
			}
		})
	}
}
