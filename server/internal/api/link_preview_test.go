package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestLinkPreviewRejectsPrivateTargets(t *testing.T) {
	handler := NewLinkPreviewHandler()
	called := false
	handler.client = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			called = true
			return nil, nil
		}),
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/link-preview?url="+url.QueryEscape("http://127.0.0.1/internal"), nil)
	rec := httptest.NewRecorder()

	if err := handler.GetLinkPreview(e.NewContext(req, rec)); err != nil {
		t.Fatalf("GetLinkPreview returned error: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if called {
		t.Fatal("expected handler to reject private target before making an outbound request")
	}
	if !strings.Contains(rec.Body.String(), "not allowed") {
		t.Fatalf("body = %s, want target-not-allowed error", rec.Body.String())
	}
}

func TestLinkPreviewFetchesMetadata(t *testing.T) {
	handler := NewLinkPreviewHandler()
	handler.client = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			body := `<html><head><title>Hello</title><meta name="description" content="World"><meta property="og:image" content="/hero.png"></head><body></body></html>`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
				Request:    req,
			}, nil
		}),
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/link-preview?url="+url.QueryEscape("https://example.com/post"), nil)
	rec := httptest.NewRecorder()

	if err := handler.GetLinkPreview(e.NewContext(req, rec)); err != nil {
		t.Fatalf("GetLinkPreview returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload LinkPreviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Title != "Hello" {
		t.Fatalf("title = %q, want %q", payload.Title, "Hello")
	}
	if payload.Description != "World" {
		t.Fatalf("description = %q, want %q", payload.Description, "World")
	}
	if payload.Image != "https://example.com/hero.png" {
		t.Fatalf("image = %q, want %q", payload.Image, "https://example.com/hero.png")
	}
}

func TestLinkPreviewClientRejectsPrivateRedirects(t *testing.T) {
	handler := NewLinkPreviewHandler()

	redirectReq := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/admin", nil)
	viaReq := httptest.NewRequest(http.MethodGet, "https://example.com/start", nil)

	if err := handler.client.CheckRedirect(redirectReq, []*http.Request{viaReq}); err == nil {
		t.Fatal("expected private redirect target to be blocked")
	}
}
