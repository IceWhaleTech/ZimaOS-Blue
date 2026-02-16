package update

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOTAChecker_FetchOTA(t *testing.T) {
	resp := OTAResponse{
		Version:           "0.0.4",
		DownloadURL:       "https://example.com/blue.tar.gz",
		ReleaseNoteURL:    "https://example.com/notes.md",
		ClientDownloadURL: "https://example.com/blue.dmg",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("os") == "" || q.Get("ver") == "" {
			t.Error("missing required query params")
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	dir := t.TempDir()
	ota := NewOTAChecker("0.0.3", dir, "en_US")

	// Override endpoints for test — use fetchOTA indirectly via checkIfDue
	// Instead, test doRequest directly
	got, err := ota.doRequest(context.Background(), srv.URL+"/blue?os=darwin-arm64&ver=0.0.3&lang=en_US&mid=test", "")
	if err != nil {
		t.Fatalf("doRequest failed: %v", err)
	}
	if got.Version != "0.0.4" {
		t.Errorf("version = %q, want 0.0.4", got.Version)
	}
	if got.ClientDownloadURL != resp.ClientDownloadURL {
		t.Errorf("client_download_url = %q, want %q", got.ClientDownloadURL, resp.ClientDownloadURL)
	}
}

func TestOTAChecker_HostHeader(t *testing.T) {
	var gotHost string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost = r.Host
		json.NewEncoder(w).Encode(OTAResponse{Version: "1.0.0"})
	}))
	defer srv.Close()

	ota := NewOTAChecker("0.0.1", t.TempDir(), "")
	_, err := ota.doRequest(context.Background(), srv.URL+"/blue", "custom.host.com")
	if err != nil {
		t.Fatalf("doRequest failed: %v", err)
	}
	if gotHost != "custom.host.com" {
		t.Errorf("Host header = %q, want custom.host.com", gotHost)
	}
}

func TestOTAChecker_ThrottleCheck(t *testing.T) {
	dir := t.TempDir()
	tsPath := filepath.Join(dir, timestampFile)

	// Write a recent timestamp — should skip check
	_ = os.WriteFile(tsPath, []byte(time.Now().UTC().Format(time.RFC3339)), 0644)

	ota := NewOTAChecker("0.0.1", dir, "")
	// checkIfDue should not make any HTTP calls (no server to call)
	ota.checkIfDue(context.Background())

	if ota.GetLatest() != nil {
		t.Error("expected nil latest after throttled check")
	}
}

func TestOTAChecker_UpdateAvailable(t *testing.T) {
	ota := NewOTAChecker("0.0.3", t.TempDir(), "")
	if ota.UpdateAvailable() {
		t.Error("should not have update before check")
	}

	ota.mu.Lock()
	ota.latest = &OTAResponse{Version: "0.0.4"}
	ota.mu.Unlock()

	if !ota.UpdateAvailable() {
		t.Error("should have update when version differs")
	}

	ota.mu.Lock()
	ota.latest = &OTAResponse{Version: "0.0.3"}
	ota.mu.Unlock()

	if ota.UpdateAvailable() {
		t.Error("should not have update when version matches")
	}
}

func TestOTAChecker_CachePersistence(t *testing.T) {
	dir := t.TempDir()

	// Write cached result
	resp := OTAResponse{Version: "1.2.3", DownloadURL: "https://example.com/dl"}
	data, _ := json.Marshal(resp)
	_ = os.WriteFile(filepath.Join(dir, otaResultFile), data, 0644)

	ota := NewOTAChecker("1.0.0", dir, "")
	ota.loadCached()

	got := ota.GetLatest()
	if got == nil {
		t.Fatal("expected cached result")
	}
	if got.Version != "1.2.3" {
		t.Errorf("cached version = %q, want 1.2.3", got.Version)
	}
}

func TestGetMachineID(t *testing.T) {
	id := getMachineID()
	if id == "" {
		t.Error("machine ID should not be empty")
	}
	if len(id) != 16 {
		t.Errorf("machine ID length = %d, want 16", len(id))
	}
	// Should be deterministic
	if id2 := getMachineID(); id != id2 {
		t.Errorf("machine ID not deterministic: %q != %q", id, id2)
	}
}
