package proxy

import (
	"context"
	"net/http/httptest"
	"testing"
)

func TestApplyRequestLocaleHeader_FromContext(t *testing.T) {
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	ctx := WithLocale(context.Background(), "zh-CN")

	applyRequestLocaleHeader(req, ctx)

	if got := req.Header.Get("Accept-Language"); got != "zh-CN" {
		t.Fatalf("Accept-Language = %q, want %q", got, "zh-CN")
	}
}

func TestApplyRequestLocaleHeader_DoesNotOverrideExisting(t *testing.T) {
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	req.Header.Set("Accept-Language", "fr-FR")
	ctx := WithLocale(context.Background(), "zh-CN")

	applyRequestLocaleHeader(req, ctx)

	if got := req.Header.Get("Accept-Language"); got != "fr-FR" {
		t.Fatalf("Accept-Language = %q, want %q", got, "fr-FR")
	}
}

func TestApplyRequestLocaleHeader_NoLocale(t *testing.T) {
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)

	applyRequestLocaleHeader(req, context.Background())

	if got := req.Header.Get("Accept-Language"); got != "" {
		t.Fatalf("Accept-Language = %q, want empty", got)
	}
}
