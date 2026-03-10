package server

import "testing"

func TestTrimLeadingReplyNewlines(t *testing.T) {
	tests := []struct {
		name            string
		existingContent string
		delta           string
		want            string
	}{
		{
			name:  "trims leading newline run at reply start",
			delta: "\n\nHello, world",
			want:  "Hello, world",
		},
		{
			name:  "trims carriage return and newline mix",
			delta: "\r\n\r\nIndented",
			want:  "Indented",
		},
		{
			name:            "keeps later chunks unchanged once content exists",
			delta:           "\nSecond line",
			want:            "\nSecond line",
			existingContent: "Hello",
		},
		{
			name:  "allows repeated trimming while reply still empty",
			delta: "\n",
			want:  "",
		},
		{
			name:  "preserves leading spaces after newline trim",
			delta: "\n    code block",
			want:  "    code block",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := trimLeadingReplyNewlines(tt.existingContent, tt.delta); got != tt.want {
				t.Fatalf("trimLeadingReplyNewlines(%q, %q) = %q, want %q", tt.existingContent, tt.delta, got, tt.want)
			}
		})
	}

	if got := trimLeadingReplyNewlines("", trimLeadingReplyNewlines("", "\n")); got != "" {
		t.Fatalf("expected empty repeated trim result, got %q", got)
	}

	if got := trimLeadingReplyNewlines("", "\nHello"); got != "Hello" {
		t.Fatalf("expected follow-up chunk to trim to visible text, got %q", got)
	}
}
