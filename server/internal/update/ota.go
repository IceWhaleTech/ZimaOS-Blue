package update

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	OnlineURL            = "https://ota.zimaos.com"
	OnlineFallbackURL    = "https://ota2.zimaos.com"
	OnlineFallbackIP     = "http://139.224.8.35"
	OnlineFallbackHeader = "ota2.zimaos.com"

	otaTimeout       = 5 * time.Second
	checkInterval    = 24 * time.Hour
	timestampFile    = ".last_update_check"
	otaResultFile    = "ota_latest.json"
)

// OTAResponse is the JSON response from the OTA server (BlueReleaseInfo).
type OTAResponse struct {
	Version        string   `json:"version"`
	Packages       []string `json:"packages"`
	ReleaseNoteURL string   `json:"release_note_url"`
	Delay          int      `json:"delay,omitempty"` // cooldown seconds before allowing update
}

// OTAChecker performs periodic update checks against the OTA server.
type OTAChecker struct {
	mu             sync.RWMutex
	currentVersion string
	dataDir        string
	lang           string
	latest         *OTAResponse
}

// NewOTAChecker creates a new OTA checker.
func NewOTAChecker(version, dataDir, lang string) *OTAChecker {
	if lang == "" {
		lang = "en_US"
	}
	return &OTAChecker{
		currentVersion: version,
		dataDir:        dataDir,
		lang:           lang,
	}
}

// Run starts the OTA check loop. Intended to be called via lifecycle manager.
func (o *OTAChecker) Run(ctx context.Context) {
	// Load cached result
	o.loadCached()

	// Initial check (with 24h throttle)
	o.checkIfDue(ctx)

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			o.checkIfDue(ctx)
		}
	}
}

// GetLatest returns the latest OTA response, if any.
func (o *OTAChecker) GetLatest() *OTAResponse {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.latest
}

// UpdateAvailable returns true if a newer version is available.
func (o *OTAChecker) UpdateAvailable() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.latest != nil && o.latest.Version != "" && IsNewerVersionString(o.currentVersion, o.latest.Version)
}

func (o *OTAChecker) checkIfDue(ctx context.Context) {
	tsPath := filepath.Join(o.dataDir, timestampFile)
	if data, err := os.ReadFile(tsPath); err == nil {
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data))); err == nil {
			if time.Since(t) < checkInterval {
				return
			}
		}
	}

	resp, err := o.fetchOTADefault(ctx)
	if err != nil {
		log.Printf("[ota] check failed: %v", err)
		return
	}

	o.mu.Lock()
	o.latest = resp
	o.mu.Unlock()

	// Persist timestamp and result
	_ = os.WriteFile(tsPath, []byte(time.Now().UTC().Format(time.RFC3339)), 0644)
	if data, err := json.Marshal(resp); err == nil {
		_ = os.WriteFile(filepath.Join(o.dataDir, otaResultFile), data, 0644)
	}

	if resp.Version != o.currentVersion {
		log.Printf("[ota] update available: %s -> %s", o.currentVersion, resp.Version)
	}
}

func (o *OTAChecker) loadCached() {
	path := filepath.Join(o.dataDir, otaResultFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var resp OTAResponse
	if json.Unmarshal(data, &resp) == nil && isBlueOTAResponse(&resp) {
		o.mu.Lock()
		o.latest = &resp
		o.mu.Unlock()
	}
}

// FetchForDesktop performs an OTA check with the -desktop OS suffix.
func (o *OTAChecker) FetchForDesktop(ctx context.Context) (*OTAResponse, error) {
	return o.fetchOTA(ctx, true)
}

func (o *OTAChecker) fetchOTADefault(ctx context.Context) (*OTAResponse, error) {
	return o.fetchOTA(ctx, false)
}

func (o *OTAChecker) fetchOTA(ctx context.Context, desktop bool) (*OTAResponse, error) {
	osParam := runtime.GOOS + "-" + runtime.GOARCH
	if desktop {
		osParam += "-desktop"
	}

	query := fmt.Sprintf("/blue?os=%s&ver=%s&lang=%s&mid=%s",
		osParam, o.currentVersion, o.lang, getMachineID())

	log.Printf("[ota] fetching OTA: %s", query)

	type endpoint struct {
		url        string
		hostHeader string
	}
	endpoints := []endpoint{
		{url: OnlineURL + query},
		{url: OnlineFallbackURL + query},
		{url: OnlineFallbackIP + query, hostHeader: OnlineFallbackHeader},
	}

	var lastErr error
	for _, ep := range endpoints {
		resp, err := o.doRequest(ctx, ep.url, ep.hostHeader)
		if err != nil {
			lastErr = err
			continue
		}
		return resp, nil
	}
	return nil, fmt.Errorf("all OTA endpoints failed: %w", lastErr)
}

func (o *OTAChecker) doRequest(ctx context.Context, url, hostHeader string) (*OTAResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, otaTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if hostHeader != "" {
		req.Host = hostHeader
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	var result OTAResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse OTA response: %w", err)
	}

	// Validate response belongs to ZimaOS-Blue, not ZimaOS.
	// Fallback OTA servers may return wrong product data.
	if !isBlueOTAResponse(&result) {
		return nil, fmt.Errorf("OTA response is not for ZimaOS-Blue (got version %s from %s)", result.Version, url)
	}

	return &result, nil
}

// isBlueOTAResponse checks that the OTA response is for ZimaOS-Blue.
// ZimaOS versions are 1.x+, Blue versions are 0.x. Also check URLs for "Blue"/"blue".
func isBlueOTAResponse(r *OTAResponse) bool {
	if r == nil || r.Version == "" {
		return false
	}
	// ZimaOS mainline versions start at 1.x; Blue is 0.x
	if !strings.HasPrefix(r.Version, "0.") {
		return false
	}
	// Double-check: release note URL should reference Blue, not ZimaOS mainline
	if r.ReleaseNoteURL != "" && !strings.Contains(strings.ToLower(r.ReleaseNoteURL), "blue") {
		return false
	}
	return true
}

func getMachineID() string {
	var parts []string
	if hostname, err := os.Hostname(); err == nil {
		parts = append(parts, hostname)
	}
	if home, err := os.UserHomeDir(); err == nil {
		parts = append(parts, home)
	}
	if len(parts) == 0 {
		return "default"
	}
	h := sha256.Sum256([]byte(strings.Join(parts, ":")))
	return fmt.Sprintf("%x", h[:8])
}
