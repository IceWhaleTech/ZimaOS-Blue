package server

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	_ "github.com/mattn/go-sqlite3"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmarket"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
)

func TestSkillMarketDiscoverSSEEndToEnd(t *testing.T) {
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
								"description": "End-to-end marketplace fixture",
								"downloads": 12,
								"homepage": "https://example.com/e2e-skill",
								"installs": 3,
								"name": "E2E Skill",
								"ownerName": "fixture",
								"score": 120,
								"slug": "e2e-skill",
								"stars": 6,
								"tags": ["e2e", "refresh"],
								"updated_at": 1742169600000,
								"version": "1.0.0"
							}
						]
					}
				}`))
			default:
				_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"total":1,"skills":[]}}`))
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	tempDir := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tempDir, "skillmarket-e2e.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	activeDir := filepath.Join(tempDir, "active")
	cfg := skillmarket.DefaultConfig(tempDir, activeDir)
	cfg.CacheRoot = filepath.Join(tempDir, "cache")
	cfg.CuratedConfigPath = filepath.Join(tempDir, "missing-curations.yaml")
	cfg.CuratedConfigURLs = nil
	cfg.DiscoveryPageURLs = nil
	cfg.TencentSkillHubAPIBaseURL = upstream.URL
	cfg.GitHubAPIBaseURL = upstream.URL
	cfg.ClawHubBaseURL = upstream.URL
	cfg.SkillHubBaseURL = upstream.URL
	cfg.LLMSkillsBaseURL = upstream.URL

	broker := sse.NewBroker()
	defer broker.Close()

	market, err := skillmarket.NewService(db, skillmarket.Options{
		Config:       cfg,
		Registry:     skill.NewRegistry(),
		LocalScanner: skillstore.NewLocalSkillScanner(activeDir),
		HTTPClient:   upstream.Client(),
		Scanner:      skillmarket.NewScanner(nil),
		DiscoverEventBroadcaster: func(eventType string, data any) {
			broker.Broadcast(eventType, data)
		},
	})
	if err != nil {
		t.Fatalf("new market: %v", err)
	}
	defer market.Close()

	if _, err := db.Exec(`UPDATE skill_sources SET enabled = 0`); err != nil {
		t.Fatalf("disable default sources: %v", err)
	}
	if err := market.Store().UpsertSource(context.Background(), skillmarket.Source{
		ID:          "tencent-skillhub",
		Type:        "lightmake_api",
		BaseURL:     upstream.URL,
		DisplayName: "Tencent SkillHub",
		SourceGroup: "skillhub",
		Enabled:     true,
		Priority:    5,
	}); err != nil {
		t.Fatalf("UpsertSource(tencent-skillhub) error = %v", err)
	}

	skillHandler := NewSkillHandler(skill.NewRegistry())
	skillHandler.SetMarketplace(market)

	jwtSvc := auth.NewJWTService(&auth.JWTConfig{
		Secret:            "12345678901234567890123456789012",
		Expiration:        time.Hour,
		RefreshExpiration: 24 * time.Hour,
		Issuer:            "skillmarket-e2e",
	})
	token, err := jwtSvc.GenerateAccessToken(&auth.UserClaims{
		UserID:   "e2e-user",
		Username: "e2e",
		Role:     "admin",
	})
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}

	e := echo.New()
	authMW := auth.NewAuthMiddleware(jwtSvc, nil)
	api := e.Group("/api/v1", authMW.Authenticate())
	sse.NewHandler(broker).RegisterRoutes(api)
	skillHandler.RegisterRoutes(api)

	server := httptest.NewServer(e)
	defer server.Close()

	sseReq, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/events", nil)
	if err != nil {
		t.Fatalf("new sse request: %v", err)
	}
	sseReq.Header.Set("Authorization", "Bearer "+token)
	sseResp, err := server.Client().Do(sseReq)
	if err != nil {
		t.Fatalf("connect sse: %v", err)
	}
	defer sseResp.Body.Close()
	if sseResp.StatusCode != http.StatusOK {
		t.Fatalf("sse status = %d, want 200", sseResp.StatusCode)
	}

	if !waitForSkillMarketCondition(2*time.Second, 20*time.Millisecond, func() bool {
		return broker.ClientCount("e2e-user") == 1
	}) {
		t.Fatal("timed out waiting for sse client subscription")
	}

	eventCh := make(chan skillmarket.DiscoverProgressEvent, 16)
	errCh := make(chan error, 1)
	go captureDiscoverProgressEvents(sseResp.Body, eventCh, errCh)

	refreshReq, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/skills/discover/refresh", nil)
	if err != nil {
		t.Fatalf("new refresh request: %v", err)
	}
	refreshReq.Header.Set("Authorization", "Bearer "+token)
	refreshResp, err := server.Client().Do(refreshReq)
	if err != nil {
		t.Fatalf("POST /skills/discover/refresh error = %v", err)
	}
	defer refreshResp.Body.Close()
	if refreshResp.StatusCode != http.StatusAccepted {
		t.Fatalf("refresh status = %d, want %d", refreshResp.StatusCode, http.StatusAccepted)
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
	waitDeadline := time.After(5 * time.Second)
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
			t.Fatalf("timed out waiting for discover progress events, phases=%v", mapsKeys(seenPhases))
		}
	}

	for _, phase := range []string{"started", "batch", "source_complete", "completed"} {
		if _, ok := seenPhases[phase]; !ok {
			t.Fatalf("missing %q phase, phases=%v", phase, mapsKeys(seenPhases))
		}
	}
	if batch := seenPhases["batch"]; batch.BatchInserted != 1 || batch.BatchUpdated != 0 || batch.BatchFailed != 0 {
		t.Fatalf("unexpected batch payload: %+v", batch)
	}
	if completed := seenPhases["completed"]; completed.Running || completed.Result == nil || completed.Result.Discovered != 1 {
		t.Fatalf("unexpected completed payload: %+v", completed)
	}

	statusReq, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/skills/discover/status", nil)
	if err != nil {
		t.Fatalf("new status request: %v", err)
	}
	statusReq.Header.Set("Authorization", "Bearer "+token)
	statusResp, err := server.Client().Do(statusReq)
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
	if statusPayload.Running || statusPayload.Result == nil || statusPayload.Result.Discovered != 1 {
		t.Fatalf("unexpected status payload: %+v", statusPayload)
	}

	searchReq, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/skills/search?page=1&page_size=20&sources=skillhub", nil)
	if err != nil {
		t.Fatalf("new search request: %v", err)
	}
	searchReq.Header.Set("Authorization", "Bearer "+token)
	searchResp, err := server.Client().Do(searchReq)
	if err != nil {
		t.Fatalf("GET /skills/search error = %v", err)
	}
	defer searchResp.Body.Close()
	if searchResp.StatusCode != http.StatusOK {
		t.Fatalf("search status = %d, want 200", searchResp.StatusCode)
	}
	var searchPayload struct {
		Skills []struct {
			Skill RemoteSkill `json:"skill"`
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
	if got.ID != "e2e-skill" {
		t.Fatalf("skill id = %q, want e2e-skill", got.ID)
	}
	if got.SourceName != "Tencent SkillHub" {
		t.Fatalf("source name = %q, want Tencent SkillHub", got.SourceName)
	}
	if !got.Installable || got.InstallType != skillmarket.InstallTypeSourceArchive {
		t.Fatalf("unexpected installability payload: %+v", got)
	}
}

func captureDiscoverProgressEvents(body io.Reader, events chan<- skillmarket.DiscoverProgressEvent, errCh chan<- error) {
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

func waitForSkillMarketCondition(timeout, interval time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(interval)
	}
	return cond()
}

func mapsKeys[T any](items map[string]T) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	return keys
}
