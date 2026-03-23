package main

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmarket"
)

func TestBlueBinarySkillMarketDiscoverSSEEndToEnd(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/skills":
			switch r.URL.Query().Get("page") {
			case "1":
				_, _ = w.Write([]byte(`{
					"code": 0,
					"message": "success",
					"data": {
						"total": 1,
						"skills": [
							{
								"category": "productivity",
								"description": "Binary end-to-end marketplace fixture",
								"downloads": 21,
								"homepage": "https://example.com/binary-e2e-skill",
								"installs": 5,
								"name": "Binary E2E Skill",
								"ownerName": "fixture",
								"score": 144,
								"slug": "binary-e2e-skill",
								"stars": 8,
								"tags": ["binary", "e2e"],
								"updated_at": 1742169600000,
								"version": "1.0.0"
							}
						]
					}
				}`))
			default:
				_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"total":1,"skills":[]}}`))
			}
		case "/search/code":
			_, _ = w.Write([]byte(`{"items":[]}`))
		case "/api/v1/skills":
			_, _ = w.Write([]byte(`{"items":[]}`))
		case "/":
			_, _ = w.Write([]byte(`<html><body>empty catalog</body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	homeDir := t.TempDir()
	configPath := filepath.Join(homeDir, "config.yaml")
	dbPath := filepath.Join(homeDir, ".zimaos-blue", "data", "blue.db")
	sockPath := filepath.Join(os.TempDir(), fmt.Sprintf("blue-e2e-%d.sock", time.Now().UnixNano()))
	_ = os.Remove(sockPath)
	defer os.Remove(sockPath)
	port := freeLocalPort(t)

	configYAML := fmt.Sprintf(`server:
  host: "127.0.0.1"
  port: %d
  port_auto_fallback: false

log:
  level: "warn"
  format: "console"
  output: "stdout"

proxy:
  enabled: false

update:
  enabled: false

companion:
  enabled: false

skill_market:
  enabled: true
  seed_urls: []
  curated_config_path: "%s"
  curated_config_urls: []
  tencent_skillhub_api_base_url: "%s"
  skillhub_base_url: "%s"
  skillstack_base_url: "%s"
  skillsmp_base_url: "%s"
  llmskills_base_url: "%s"
`, port, filepath.Join(homeDir, "missing-curations.yaml"), upstream.URL, upstream.URL, upstream.URL, upstream.URL, upstream.URL)

	if err := os.WriteFile(configPath, []byte(configYAML), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	binPath := buildBlueBinary(t)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	var logs bytes.Buffer
	cmd := exec.CommandContext(ctx, binPath, "--config", configPath)
	cmd.Dir = filepath.Dir(binPath)
	cmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"BLUE_IPC_SOCKET="+sockPath,
	)
	cmd.Stdout = &logs
	cmd.Stderr = &logs
	if err := cmd.Start(); err != nil {
		t.Fatalf("start blue binary: %v", err)
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
		if t.Failed() {
			t.Logf("blue logs:\n%s", logs.String())
		}
	}()

	waitForConditionOrFail(t, 30*time.Second, 200*time.Millisecond, logs.String, func() bool {
		resp, err := http.Get(baseURL + "/api/v1/system/mode")
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return false
		}
		var payload struct {
			Mode string `json:"mode"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return false
		}
		return payload.Mode == "preview"
	}, "timed out waiting for blue preview mode")

	token := fetchPreviewToken(t, baseURL)
	primeMarketplaceService(t, baseURL, token)
	waitForDiscoverIdle(t, baseURL, token)
	configureSingleDiscoverSource(t, dbPath, upstream.URL)

	sseReq, err := http.NewRequest(http.MethodGet, baseURL+"/api/v1/events", nil)
	if err != nil {
		t.Fatalf("new sse request: %v", err)
	}
	sseReq.Header.Set("Authorization", "Bearer "+token)
	sseClient := &http.Client{Timeout: 0}
	sseResp, err := sseClient.Do(sseReq)
	if err != nil {
		t.Fatalf("connect sse: %v", err)
	}
	defer sseResp.Body.Close()
	if sseResp.StatusCode != http.StatusOK {
		t.Fatalf("sse status = %d, want 200", sseResp.StatusCode)
	}

	eventCh := make(chan skillmarket.DiscoverProgressEvent, 16)
	errCh := make(chan error, 1)
	go captureBinaryDiscoverProgressEvents(sseResp.Body, eventCh, errCh)

	time.Sleep(200 * time.Millisecond)

	refreshReq, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/skills/discover/refresh", nil)
	if err != nil {
		t.Fatalf("new refresh request: %v", err)
	}
	refreshReq.Header.Set("Authorization", "Bearer "+token)
	refreshResp, err := http.DefaultClient.Do(refreshReq)
	if err != nil {
		t.Fatalf("POST /skills/discover/refresh error = %v", err)
	}
	defer refreshResp.Body.Close()
	if refreshResp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(refreshResp.Body)
		t.Fatalf("refresh status = %d, want 202, body=%s", refreshResp.StatusCode, string(body))
	}

	var refreshPayload struct {
		Accepted bool `json:"accepted"`
		Running  bool `json:"running"`
	}
	if err := json.NewDecoder(refreshResp.Body).Decode(&refreshPayload); err != nil {
		t.Fatalf("decode refresh payload: %v", err)
	}
	if !refreshPayload.Accepted || !refreshPayload.Running {
		t.Fatalf("unexpected refresh payload: %+v", refreshPayload)
	}

	seenPhases := make(map[string]skillmarket.DiscoverProgressEvent)
	waitDeadline := time.After(10 * time.Second)
	for {
		if _, ok := seenPhases["completed"]; ok {
			break
		}
		select {
		case evt := <-eventCh:
			if evt.Phase != "" {
				seenPhases[evt.Phase] = evt
			}
		case err := <-errCh:
			if err != nil {
				t.Fatalf("sse stream error: %v", err)
			}
		case <-waitDeadline:
			t.Fatalf("timed out waiting for discover progress events, phases=%v", mapKeys(seenPhases))
		}
	}

	for _, phase := range []string{"started", "batch", "source_complete", "completed"} {
		if _, ok := seenPhases[phase]; !ok {
			t.Fatalf("missing %q phase, phases=%v", phase, mapKeys(seenPhases))
		}
	}
	if batch := seenPhases["batch"]; batch.BatchInserted != 1 || batch.BatchUpdated != 0 || batch.BatchFailed != 0 {
		t.Fatalf("unexpected batch payload: %+v", batch)
	}
	if completed := seenPhases["completed"]; completed.Running || completed.Result == nil || completed.Result.Discovered != 1 {
		t.Fatalf("unexpected completed payload: %+v", completed)
	}

	statusReq, err := http.NewRequest(http.MethodGet, baseURL+"/api/v1/skills/discover/status", nil)
	if err != nil {
		t.Fatalf("new status request: %v", err)
	}
	statusReq.Header.Set("Authorization", "Bearer "+token)
	statusResp, err := http.DefaultClient.Do(statusReq)
	if err != nil {
		t.Fatalf("GET /skills/discover/status error = %v", err)
	}
	defer statusResp.Body.Close()
	if statusResp.StatusCode != http.StatusOK {
		t.Fatalf("status code = %d, want 200", statusResp.StatusCode)
	}
	var statusPayload struct {
		Running bool                        `json:"running"`
		Result  *skillmarket.DiscoverResult `json:"result"`
	}
	if err := json.NewDecoder(statusResp.Body).Decode(&statusPayload); err != nil {
		t.Fatalf("decode status payload: %v", err)
	}
	if statusPayload.Running || statusPayload.Result == nil || statusPayload.Result.Discovered != 1 || statusPayload.Result.SourcesProcessed != 1 {
		t.Fatalf("unexpected status payload: %+v", statusPayload)
	}

	searchReq, err := http.NewRequest(http.MethodGet, baseURL+"/api/v1/skills/search?page=1&page_size=20&sources=skillhub", nil)
	if err != nil {
		t.Fatalf("new search request: %v", err)
	}
	searchReq.Header.Set("Authorization", "Bearer "+token)
	searchResp, err := http.DefaultClient.Do(searchReq)
	if err != nil {
		t.Fatalf("GET /skills/search error = %v", err)
	}
	defer searchResp.Body.Close()
	if searchResp.StatusCode != http.StatusOK {
		t.Fatalf("search status = %d, want 200", searchResp.StatusCode)
	}
	var searchPayload struct {
		Skills []struct {
			Skill struct {
				ID          string `json:"id"`
				SourceName  string `json:"source_name"`
				Installable bool   `json:"installable"`
				InstallType string `json:"install_type"`
			} `json:"skill"`
		} `json:"skills"`
		Total int `json:"total"`
	}
	if err := json.NewDecoder(searchResp.Body).Decode(&searchPayload); err != nil {
		t.Fatalf("decode search payload: %v", err)
	}
	if searchPayload.Total != 1 || len(searchPayload.Skills) != 1 {
		t.Fatalf("unexpected search payload: %+v", searchPayload)
	}
	got := searchPayload.Skills[0].Skill
	if got.ID != "binary-e2e-skill" {
		t.Fatalf("skill id = %q, want binary-e2e-skill", got.ID)
	}
	if got.SourceName != "Tencent SkillHub" {
		t.Fatalf("source name = %q, want Tencent SkillHub", got.SourceName)
	}
	if !got.Installable || got.InstallType != skillmarket.InstallTypeSourceArchive {
		t.Fatalf("unexpected installability payload: %+v", got)
	}
}

func buildBlueBinary(t *testing.T) string {
	t.Helper()

	workdir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	binPath := filepath.Join(t.TempDir(), "blue-e2e")
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = workdir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build blue binary: %v\n%s", err, string(output))
	}
	return binPath
}

func freeLocalPort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("unexpected listener addr %T", listener.Addr())
	}
	return addr.Port
}

func configureSingleDiscoverSource(t *testing.T, dbPath, upstreamURL string) {
	t.Helper()

	waitForConditionOrFail(t, 20*time.Second, 100*time.Millisecond, func() string { return dbPath }, func() bool {
		db, err := sql.Open("sqlite3", dbPath)
		if err != nil {
			return false
		}
		defer db.Close()

		var count int
		if err := db.QueryRow(`SELECT count(*) FROM skill_sources`).Scan(&count); err != nil {
			return false
		}

		if _, err := db.Exec(`UPDATE skill_sources SET enabled = 0`); err != nil {
			return false
		}
		if _, err := db.Exec(`DELETE FROM skill_security_reports`); err != nil {
			return false
		}
		if _, err := db.Exec(`DELETE FROM skill_versions`); err != nil {
			return false
		}
		if _, err := db.Exec(`DELETE FROM skills`); err != nil {
			return false
		}
		if _, err := db.Exec(`DELETE FROM crawl_runs`); err != nil {
			return false
		}
		_, err = db.Exec(`
			INSERT INTO skill_sources (
				id,
				type,
				base_url,
				display_name,
				source_group,
				auth_mode,
				enabled,
				rate_limit_per_minute,
				priority,
				created_at,
				updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
			ON CONFLICT(id) DO UPDATE SET
				type=excluded.type,
				base_url=excluded.base_url,
				display_name=excluded.display_name,
				source_group=excluded.source_group,
				auth_mode=excluded.auth_mode,
				enabled=excluded.enabled,
				rate_limit_per_minute=excluded.rate_limit_per_minute,
				priority=excluded.priority,
				updated_at=CURRENT_TIMESTAMP
		`, "tencent-skillhub", "lightmake_api", upstreamURL, "Tencent SkillHub", "skillhub", "none", 1, 120, 5)
		return err == nil
	}, "timed out preparing single skill market source")
}

func fetchPreviewToken(t *testing.T, baseURL string) string {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/preview/token", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("new preview token request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /preview/token error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("preview token status = %d, want 200, body=%s", resp.StatusCode, string(body))
	}
	var payload struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode preview token payload: %v", err)
	}
	if strings.TrimSpace(payload.Token) == "" {
		t.Fatal("preview token is empty")
	}
	return payload.Token
}

func primeMarketplaceService(t *testing.T, baseURL, token string) {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, baseURL+"/api/v1/skills/discover/status", nil)
	if err != nil {
		t.Fatalf("new discover status request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	waitForConditionOrFail(t, 20*time.Second, 100*time.Millisecond, func() string { return baseURL }, func() bool {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, "timed out waiting for marketplace lazy initialization")
}

func waitForDiscoverIdle(t *testing.T, baseURL, token string) {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, baseURL+"/api/v1/skills/discover/status", nil)
	if err != nil {
		t.Fatalf("new discover status request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	waitForConditionOrFail(t, 45*time.Second, 250*time.Millisecond, func() string { return baseURL }, func() bool {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return false
		}
		var payload struct {
			Running bool `json:"running"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return false
		}
		return !payload.Running
	}, "timed out waiting for startup discover to finish")
}

func captureBinaryDiscoverProgressEvents(body io.Reader, events chan<- skillmarket.DiscoverProgressEvent, errCh chan<- error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)

	currentEvent := ""
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event: "))
		case strings.HasPrefix(line, "data: "):
			if currentEvent != "skill.market.discover.progress" {
				currentEvent = ""
				continue
			}
			var evt skillmarket.DiscoverProgressEvent
			if err := json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data: "))), &evt); err != nil {
				errCh <- err
				return
			}
			events <- evt
			currentEvent = ""
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		errCh <- err
	}
}

func waitForConditionOrFail(
	t *testing.T,
	timeout time.Duration,
	interval time.Duration,
	debug func() string,
	cond func() bool,
	message string,
) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(interval)
	}
	t.Fatalf("%s\n%s", message, debug())
}

func mapKeys[T any](items map[string]T) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	return keys
}
