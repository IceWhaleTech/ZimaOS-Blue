package tools

import (
	"context"
	"testing"
)

func TestCanonicalizeUIReviewAction(t *testing.T) {
	tests := []struct {
		name      string
		action    string
		url       string
		image     string
		want      string
		wantError bool
	}{
		{name: "explicit review url", action: "review_url", url: "https://example.com", want: "review_url"},
		{name: "implicit from url", url: "https://example.com", want: "review_url"},
		{name: "implicit from image", image: "base64data", want: "review_image"},
		{name: "audit url alias", action: "audit", url: "https://example.com", want: "review_url"},
		{name: "audit image alias", action: "audit", image: "base64data", want: "review_image"},
		{name: "hyphenated screenshot alias", action: "audit-screenshot", image: "base64data", want: "review_image"},
		{name: "accessibility alias", action: "a11y", url: "https://example.com", want: "check_accessibility"},
		{name: "invalid action", action: "bad", url: "https://example.com", wantError: true},
		{name: "missing everything", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CanonicalizeUIReviewAction(tt.action, tt.url, tt.image)
			if (err != nil) != tt.wantError {
				t.Fatalf("CanonicalizeUIReviewAction() error = %v, wantError %v", err, tt.wantError)
			}
			if err == nil && got != tt.want {
				t.Fatalf("CanonicalizeUIReviewAction() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUIReviewerToolExecuteCanonicalizesAuditAlias(t *testing.T) {
	tool := NewUIReviewerTool()
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "audit",
		"url":    "https://example.com",
	})
	if err == nil {
		t.Fatal("expected browser service error")
	}
	if got := err.Error(); got != "browser service not available — cannot review URL" {
		t.Fatalf("Execute() error = %q", got)
	}
}
