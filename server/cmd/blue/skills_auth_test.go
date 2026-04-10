package main

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// TestDoSkillsRequest_SendsAuthorizationHeader 验证 skills 请求携带 Authorization header
func TestDoSkillsRequest_SendsAuthorizationHeader(t *testing.T) {
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"skill_id":"test-skill","version":"1.0.0","path":"/tmp/test","warnings":[],"security":{}}`))
	}))
	defer server.Close()

	// 设置认证 token
	t.Setenv("BLUE_HARNESS_BEARER_TOKEN", "test-token-123")

	resp, err := doSkillsRequest(http.MethodPost, server.URL+"/install", map[string]interface{}{"id": "test"})
	if err != nil {
		t.Fatalf("doSkillsRequest failed: %v", err)
	}
	defer resp.Body.Close()

	if receivedAuth == "" {
		t.Errorf("Authorization header not sent to server")
	}
	wantAuth := "Bearer test-token-123"
	if receivedAuth != wantAuth {
		t.Errorf("Authorization header = %q, want %q", receivedAuth, wantAuth)
	}
}

// TestDoSkillsRequest_WorksWithoutAuth 验证没有认证时也能工作
func TestDoSkillsRequest_WorksWithoutAuth(t *testing.T) {
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"skill_id":"test-skill","version":"1.0.0"}`))
	}))
	defer server.Close()

	// 确保没有设置 token
	t.Setenv("BLUE_HARNESS_BEARER_TOKEN", "")

	resp, err := doSkillsRequest(http.MethodGet, server.URL+"/list", nil)
	if err != nil {
		t.Fatalf("doSkillsRequest failed: %v", err)
	}
	defer resp.Body.Close()

	if receivedAuth != "" {
		t.Errorf("Authorization header should be empty when no token set, got %q", receivedAuth)
	}
}

func TestDecodeInto_UsesMessageFieldForHTTPErrors(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusUnauthorized,
		Body:       io.NopCloser(strings.NewReader(`{"message":"authentication required"}`)),
	}

	err := decodeInto(resp, &struct{}{})
	if err == nil {
		t.Fatal("decodeInto() error = nil, want message error")
	}
	if got, want := err.Error(), "authentication required"; got != want {
		t.Fatalf("decodeInto() error = %q, want %q", got, want)
	}
}

type skillsExitPanic struct {
	code int
}

func TestPrintSkillsError_IncludesDetailsWhenStdoutNotTerminal(t *testing.T) {
	oldStdout := os.Stdout
	oldJSONOutput := jsonOutput
	oldNoColor := noColor
	oldVerbose := verbose
	oldSkillsExit := skillsExit
	defer func() {
		os.Stdout = oldStdout
		jsonOutput = oldJSONOutput
		noColor = oldNoColor
		verbose = oldVerbose
		skillsExit = oldSkillsExit
	}()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	defer r.Close()

	os.Stdout = w
	jsonOutput = false
	noColor = true
	verbose = false
	skillsExit = func(code int) { panic(skillsExitPanic{code: code}) }

	var buf bytes.Buffer
	func() {
		defer func() {
			_ = w.Close()
			_, _ = buf.ReadFrom(r)
			if rec := recover(); rec != nil {
				exit, ok := rec.(skillsExitPanic)
				if !ok {
					panic(rec)
				}
				if exit.code != 1 {
					t.Fatalf("skillsExit code = %d, want 1", exit.code)
				}
			} else {
				t.Fatal("printSkillsError() did not exit")
			}
		}()
		printSkillsError("Failed to parse install response", errors.New("skill not found: humanizer"))
	}()

	stdout := buf.String()
	if !strings.Contains(stdout, "Error: Failed to parse install response") {
		t.Fatalf("stdout = %q, want primary error message", stdout)
	}
	if !strings.Contains(stdout, "Details: skill not found: humanizer") {
		t.Fatalf("stdout = %q, want error details", stdout)
	}
}
