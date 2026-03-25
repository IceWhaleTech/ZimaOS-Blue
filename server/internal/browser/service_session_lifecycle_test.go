package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func findTestBrowserBinary() string {
	for _, candidate := range []string{"google-chrome", "chromium", "chromium-browser", "chrome"} {
		if path, err := exec.LookPath(candidate); err == nil && path != "" {
			return path
		}
	}

	for _, candidate := range []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
		"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return ""
}

func TestRodServiceSessionScreenshotLifecycleE2E(t *testing.T) {
	browserPath := findTestBrowserBinary()
	if browserPath == "" {
		t.Skip("no Chrome/Chromium binary found for browser lifecycle test")
	}

	pageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/first":
			_, _ = fmt.Fprint(
				w,
				`<!doctype html><html><head><title>First frame</title><style>body{margin:0;background:#0f766e;color:#ecfeff;font:700 56px/1.2 sans-serif;display:grid;place-items:center;height:100vh;}</style></head><body>FIRST FRAME</body></html>`,
			)
		case "/second":
			_, _ = fmt.Fprint(
				w,
				`<!doctype html><html><head><title>Second frame</title><style>body{margin:0;background:#7c3aed;color:#f5f3ff;font:700 56px/1.2 sans-serif;display:grid;place-items:center;height:100vh;}</style></head><body>SECOND FRAME</body></html>`,
			)
		default:
			http.NotFound(w, r)
		}
	}))
	defer pageServer.Close()

	cfg := DefaultConfig()
	cfg.BrowserPath = browserPath
	cfg.PoolSize = 1
	cfg.Headless = true

	service, err := NewService(cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	require.NoError(t, service.Start(ctx))
	defer func() {
		_ = service.Stop(context.Background())
	}()

	tab, err := service.OpenTab(ctx, pageServer.URL+"/first")
	require.NoError(t, err)
	require.NotEmpty(t, tab.TargetID)

	handler := NewHandler(service)
	echoServer := echo.New()
	handler.RegisterRoutes(echoServer.Group("/browser"))

	firstShotRec := performBrowserJSONRequest(
		echoServer,
		http.MethodPost,
		"/browser/sessions/"+tab.TargetID+"/screenshot",
		"",
	)
	require.Equal(t, http.StatusOK, firstShotRec.Code, firstShotRec.Body.String())

	var firstShot SessionScreenshotResponse
	require.NoError(t, json.Unmarshal(firstShotRec.Body.Bytes(), &firstShot))
	require.NotEmpty(t, firstShot.Screenshot)
	require.Len(t, firstShot.History, 1)
	assert.Equal(t, "viewport", firstShot.History[0].Scope)
	assert.Contains(t, firstShot.History[0].URL, "/first")
	assert.Equal(t, "First frame", firstShot.History[0].Title)

	navigateRec := performBrowserJSONRequest(
		echoServer,
		http.MethodPost,
		"/browser/sessions/"+tab.TargetID+"/navigate",
		fmt.Sprintf(`{"url":%q}`, pageServer.URL+"/second"),
	)
	require.Equal(t, http.StatusOK, navigateRec.Code, navigateRec.Body.String())

	secondShotRec := performBrowserJSONRequest(
		echoServer,
		http.MethodPost,
		"/browser/sessions/"+tab.TargetID+"/screenshot",
		"",
	)
	require.Equal(t, http.StatusOK, secondShotRec.Code, secondShotRec.Body.String())

	var secondShot SessionScreenshotResponse
	require.NoError(t, json.Unmarshal(secondShotRec.Body.Bytes(), &secondShot))
	require.NotEmpty(t, secondShot.Screenshot)
	require.NotEqual(t, firstShot.Screenshot, secondShot.Screenshot)
	require.Len(t, secondShot.History, 2)
	assert.Equal(t, secondShot.Screenshot, secondShot.History[0].Data)
	assert.Equal(t, "viewport", secondShot.History[0].Scope)
	assert.Contains(t, secondShot.History[0].URL, "/second")
	assert.Equal(t, "Second frame", secondShot.History[0].Title)
	assert.Equal(t, firstShot.Screenshot, secondShot.History[1].Data)
	assert.Contains(t, secondShot.History[1].URL, "/first")

	sessionsRec := performBrowserJSONRequest(echoServer, http.MethodGet, "/browser/sessions", "")
	require.Equal(t, http.StatusOK, sessionsRec.Code, sessionsRec.Body.String())

	var sessions []SessionInfo
	require.NoError(t, json.Unmarshal(sessionsRec.Body.Bytes(), &sessions))
	require.Len(t, sessions, 1)
	assert.Equal(t, tab.TargetID, sessions[0].ID)

	retainedHistory := service.SessionScreenshotHistory(tab.TargetID)
	require.Len(t, retainedHistory, 2)
	assert.Equal(t, secondShot.Screenshot, retainedHistory[0].Data)

	closeRec := performBrowserJSONRequest(
		echoServer,
		http.MethodDelete,
		"/browser/sessions/"+tab.TargetID,
		"",
	)
	require.Equal(t, http.StatusNoContent, closeRec.Code, closeRec.Body.String())
	assert.Nil(t, service.SessionScreenshotHistory(tab.TargetID))

	afterCloseRec := performBrowserJSONRequest(
		echoServer,
		http.MethodPost,
		"/browser/sessions/"+tab.TargetID+"/screenshot",
		"",
	)
	require.Equal(t, http.StatusOK, afterCloseRec.Code, afterCloseRec.Body.String())

	var afterClose SessionScreenshotResponse
	require.NoError(t, json.Unmarshal(afterCloseRec.Body.Bytes(), &afterClose))
	assert.Empty(t, afterClose.Screenshot)
	assert.Empty(t, afterClose.History)
	assert.Contains(t, afterClose.Error, ErrTabNotFound.Error())
}

func TestRodServiceDetachedScreenshotAppearsInMonitorSessionsE2E(t *testing.T) {
	browserPath := findTestBrowserBinary()
	if browserPath == "" {
		t.Skip("no Chrome/Chromium binary found for browser lifecycle test")
	}

	pageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/first":
			_, _ = fmt.Fprint(
				w,
				`<!doctype html><html><head><title>Detached First</title><style>body{margin:0;background:#14532d;color:#ecfccb;font:700 56px/1.2 sans-serif;display:grid;place-items:center;height:100vh;}</style></head><body>DETACHED FIRST</body></html>`,
			)
		case "/second":
			_, _ = fmt.Fprint(
				w,
				`<!doctype html><html><head><title>Detached Second</title><style>body{margin:0;background:#7f1d1d;color:#fee2e2;font:700 56px/1.2 sans-serif;display:grid;place-items:center;height:100vh;}</style></head><body>DETACHED SECOND</body></html>`,
			)
		default:
			http.NotFound(w, r)
		}
	}))
	defer pageServer.Close()

	cfg := DefaultConfig()
	cfg.BrowserPath = browserPath
	cfg.PoolSize = 1
	cfg.Headless = true

	service, err := NewService(cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	require.NoError(t, service.Start(ctx))
	defer func() {
		_ = service.Stop(context.Background())
	}()

	handler := NewHandler(service)
	echoServer := echo.New()
	handler.RegisterRoutes(echoServer.Group("/browser"))

	firstResp, err := service.Screenshot(ctx, &ScreenshotRequest{URL: pageServer.URL + "/first"})
	require.NoError(t, err)
	require.NotEmpty(t, firstResp.Data)

	sessionsRec := performBrowserJSONRequest(echoServer, http.MethodGet, "/browser/sessions", "")
	require.Equal(t, http.StatusOK, sessionsRec.Code, sessionsRec.Body.String())

	var sessions []SessionInfo
	require.NoError(t, json.Unmarshal(sessionsRec.Body.Bytes(), &sessions))
	require.Len(t, sessions, 1)
	assert.Equal(t, detachedMonitorTargetID, sessions[0].ID)
	assert.Equal(t, "Detached First", sessions[0].PageTitle)
	assert.Contains(t, sessions[0].CurrentURL, "/first")

	firstShotRec := performBrowserJSONRequest(
		echoServer,
		http.MethodPost,
		"/browser/sessions/"+sessions[0].ID+"/screenshot",
		"",
	)
	require.Equal(t, http.StatusOK, firstShotRec.Code, firstShotRec.Body.String())

	var firstShot SessionScreenshotResponse
	require.NoError(t, json.Unmarshal(firstShotRec.Body.Bytes(), &firstShot))
	require.Equal(t, firstResp.Data, firstShot.Screenshot)
	require.Len(t, firstShot.History, 1)
	assert.Equal(t, "Detached First", firstShot.History[0].Title)
	assert.Contains(t, firstShot.History[0].URL, "/first")

	secondResp, err := service.Screenshot(ctx, &ScreenshotRequest{URL: pageServer.URL + "/second"})
	require.NoError(t, err)
	require.NotEmpty(t, secondResp.Data)
	require.NotEqual(t, firstResp.Data, secondResp.Data)

	secondShotRec := performBrowserJSONRequest(
		echoServer,
		http.MethodPost,
		"/browser/sessions/"+detachedMonitorTargetID+"/screenshot",
		"",
	)
	require.Equal(t, http.StatusOK, secondShotRec.Code, secondShotRec.Body.String())

	var secondShot SessionScreenshotResponse
	require.NoError(t, json.Unmarshal(secondShotRec.Body.Bytes(), &secondShot))
	require.Equal(t, secondResp.Data, secondShot.Screenshot)
	require.Len(t, secondShot.History, 2)
	assert.Equal(t, "Detached Second", secondShot.History[0].Title)
	assert.Contains(t, secondShot.History[0].URL, "/second")
	assert.Equal(t, firstResp.Data, secondShot.History[1].Data)

	closeRec := performBrowserJSONRequest(
		echoServer,
		http.MethodDelete,
		"/browser/sessions/"+detachedMonitorTargetID,
		"",
	)
	require.Equal(t, http.StatusNoContent, closeRec.Code, closeRec.Body.String())
	assert.Nil(t, service.SessionScreenshotHistory(detachedMonitorTargetID))
}

func TestRodServiceAutomatePublishesFinalMonitorFrameE2E(t *testing.T) {
	browserPath := findTestBrowserBinary()
	if browserPath == "" {
		t.Skip("no Chrome/Chromium binary found for browser lifecycle test")
	}

	pageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(
			w,
			`<!doctype html><html><head><title>Automate Preview</title><style>body{margin:0;background:#1e3a8a;color:#dbeafe;font:700 56px/1.2 sans-serif;display:grid;place-items:center;height:100vh;}</style></head><body>AUTOMATE PREVIEW</body></html>`,
		)
	}))
	defer pageServer.Close()

	cfg := DefaultConfig()
	cfg.BrowserPath = browserPath
	cfg.PoolSize = 1
	cfg.Headless = true

	service, err := NewService(cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	require.NoError(t, service.Start(ctx))
	defer func() {
		_ = service.Stop(context.Background())
	}()

	result, err := service.Automate(ctx, &AutomateRequest{
		URL: pageServer.URL,
		Steps: []AutomationStep{
			{Action: ActionWait, WaitFor: 50},
		},
	})
	require.NoError(t, err)
	require.True(t, result.Success)
	assert.Contains(t, result.FinalURL, pageServer.URL)
	assert.Equal(t, "Automate Preview", result.FinalTitle)

	history := service.SessionScreenshotHistory(detachedMonitorTargetID)
	require.Len(t, history, 1)
	assert.Equal(t, "automate", history[0].Scope)
	assert.Equal(t, "Automate Preview", history[0].Title)
	assert.Contains(t, history[0].URL, pageServer.URL)
	assert.NotEmpty(t, history[0].Data)
}
