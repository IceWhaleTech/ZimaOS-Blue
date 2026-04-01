package bootstrap

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	memorypkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/metrics"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/security"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workflow"
)

type stubRouteRegistrar struct {
	calls    int
	register func(*echo.Group)
}

func (s *stubRouteRegistrar) RegisterRoutes(g *echo.Group) {
	s.calls++
	if s.register != nil {
		s.register(g)
	}
}

type stubSecurityRouteRegistrar struct {
	stubRouteRegistrar
	scannerConfig *security.ScannerConfig
	store         kvstore.Store
	dataDir       string
}

type stubWorkflowRouteRegistrar struct {
	calls           int
	middlewares     []echo.MiddlewareFunc
	serviceInitHook func(*workflow.WorkflowService)
	register        func(*echo.Echo)
}

type stubCompanionGroupRouteRegistrar struct {
	groupCalls  int
	compatCalls int
	group       func(*echo.Group)
	compat      func(*echo.Group)
}

type stubMetricsRecorderTarget struct {
	recorder serverpkg.MetricsRecorder
}

type stubMemoryRouteRegistrar struct {
	stubRouteRegistrar
	onLayeredReady func(*memorypkg.LayeredMemoryService)
}

type stubChannelConfigRouteRegistrar struct {
	stubRouteRegistrar
	manager *channel.Manager
	factory *serverpkg.ChannelFactory
}

type stubExternalAuthRouteRegistrar struct {
	publicCalls    int
	protectedCalls int
	public         func(*echo.Group)
	protected      func(*echo.Group)
}

func (s *stubWorkflowRouteRegistrar) RegisterRoutes(e *echo.Echo) {
	s.calls++
	if s.register != nil {
		s.register(e)
	}
}

func (s *stubWorkflowRouteRegistrar) SetServiceInitHook(fn func(*workflow.WorkflowService)) {
	s.serviceInitHook = fn
}

func (s *stubWorkflowRouteRegistrar) SetRouteMiddlewares(middlewares ...echo.MiddlewareFunc) {
	s.middlewares = append([]echo.MiddlewareFunc(nil), middlewares...)
}

func (s *stubCompanionGroupRouteRegistrar) RegisterGroupRoutes(g *echo.Group) {
	s.groupCalls++
	if s.group != nil {
		s.group(g)
	}
}

func (s *stubCompanionGroupRouteRegistrar) RegisterCompatGroupRoutes(g *echo.Group) {
	s.compatCalls++
	if s.compat != nil {
		s.compat(g)
	}
}

func (s *stubMetricsRecorderTarget) SetMetricsRecorder(recorder serverpkg.MetricsRecorder) {
	s.recorder = recorder
}

func (s *stubMemoryRouteRegistrar) SetOnLayeredReady(fn func(*memorypkg.LayeredMemoryService)) {
	s.onLayeredReady = fn
}

func (s *stubChannelConfigRouteRegistrar) SetManager(manager *channel.Manager) {
	s.manager = manager
}

func (s *stubChannelConfigRouteRegistrar) SetFactory(factory *serverpkg.ChannelFactory) {
	s.factory = factory
}

func (s *stubExternalAuthRouteRegistrar) RegisterRoutes(g *echo.Group) {
	s.publicCalls++
	if s.public != nil {
		s.public(g)
	}
}

func (s *stubExternalAuthRouteRegistrar) RegisterProtectedRoutes(g *echo.Group) {
	s.protectedCalls++
	if s.protected != nil {
		s.protected(g)
	}
}

func (s *stubSecurityRouteRegistrar) SetScannerConfig(cfg *security.ScannerConfig) {
	s.scannerConfig = cfg
}

func (s *stubSecurityRouteRegistrar) SetKVStore(store kvstore.Store) {
	s.store = store
}

func (s *stubSecurityRouteRegistrar) SetDataDir(dataDir string) {
	s.dataDir = dataDir
}

func assertFeatureDisabledResponse(t *testing.T, e *echo.Echo, method, path, feature string) {
	t.Helper()

	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var body struct {
		Enabled bool   `json:"enabled"`
		Feature string `json:"feature"`
		Status  string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Enabled {
		t.Fatalf("enabled=%v, want false", body.Enabled)
	}
	if body.Feature != feature {
		t.Fatalf("feature=%q, want %q", body.Feature, feature)
	}
	if body.Status != "not_initialized" {
		t.Fatalf("status=%q, want not_initialized", body.Status)
	}
}

func TestRegisterBackupRoutes_UsesHandlerAndFallback(t *testing.T) {
	t.Run("handler", func(t *testing.T) {
		e := echo.New()
		v1 := e.Group("/api/v1")
		handler := &stubRouteRegistrar{
			register: func(g *echo.Group) {
				g.GET("/backup/live", func(c echo.Context) error {
					return c.NoContent(http.StatusNoContent)
				})
			},
		}

		registerBackupRoutes(v1, handler, nil, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/backup/live", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("status=%d, want 204, body=%s", rec.Code, rec.Body.String())
		}
		if handler.calls != 1 {
			t.Fatalf("RegisterRoutes calls=%d, want 1", handler.calls)
		}
	})

	t.Run("fallback", func(t *testing.T) {
		e := echo.New()
		v1 := e.Group("/api/v1")

		registerBackupRoutes(v1, nil, nil, nil)

		assertFeatureDisabledResponse(t, e, http.MethodGet, "/api/v1/backup/history", "backup")
	})
}

func TestRegisterSecurityRoutes_ConfiguresHandlerAndFallback(t *testing.T) {
	t.Run("handler", func(t *testing.T) {
		e := echo.New()
		protected := e.Group("/api/v1")
		store := kvstore.NewMemoryStore()
		handler := &stubSecurityRouteRegistrar{}
		handler.register = func(g *echo.Group) {
			g.GET("/live", func(c echo.Context) error {
				return c.String(http.StatusOK, "ok")
			})
		}

		cfg := &config.Config{}
		cfg.Server.Host = "127.0.0.1"
		cfg.Security.Sandbox.Enabled = true
		cfg.Security.Sandbox.NetworkEnabled = true
		cfg.Security.Sandbox.DefaultTimeout = 30 * time.Second
		cfg.Security.Sandbox.MemoryLimit = "512MB"
		cfg.Security.Sandbox.CPULimit = 1.5
		cfg.Security.JWT.Secret = "0123456789abcdef"
		cfg.Security.JWT.Expiration = time.Hour

		registerSecurityRoutes(protected, nil, handler, cfg, store, "/tmp/blue-data", true)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/security/live", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d, want 200, body=%s", rec.Code, rec.Body.String())
		}
		if handler.calls != 1 {
			t.Fatalf("RegisterRoutes calls=%d, want 1", handler.calls)
		}
		if handler.scannerConfig == nil {
			t.Fatal("expected scanner config to be wired")
		}
		if !handler.scannerConfig.SandboxEnabled {
			t.Fatal("expected sandbox-enabled scanner config")
		}
		if handler.store != store {
			t.Fatal("expected kv store to be wired")
		}
		if handler.dataDir != "/tmp/blue-data" {
			t.Fatalf("dataDir=%q, want /tmp/blue-data", handler.dataDir)
		}
	})

	t.Run("fallback", func(t *testing.T) {
		e := echo.New()
		protected := e.Group("/api/v1")

		registerSecurityRoutes(protected, nil, nil, nil, nil, "", false)

		assertFeatureDisabledResponse(t, e, http.MethodGet, "/api/v1/security/firewall", "security")
	})
}

func TestRegisterCronRoutes_UsesHandlerAndFallback(t *testing.T) {
	t.Run("handler", func(t *testing.T) {
		e := echo.New()
		apiProtected := e.Group("/api")
		handler := &stubRouteRegistrar{
			register: func(g *echo.Group) {
				g.GET("/cron/live", func(c echo.Context) error {
					return c.NoContent(http.StatusNoContent)
				})
			},
		}

		registerCronRoutes(apiProtected, handler, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/cron/live", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("status=%d, want 204, body=%s", rec.Code, rec.Body.String())
		}
		if handler.calls != 1 {
			t.Fatalf("RegisterRoutes calls=%d, want 1", handler.calls)
		}
	})

	t.Run("fallback", func(t *testing.T) {
		e := echo.New()
		apiProtected := e.Group("/api")

		registerCronRoutes(apiProtected, nil, nil)

		assertFeatureDisabledResponse(t, e, http.MethodGet, "/api/cron/jobs", "cron")
	})
}

func TestRegisterBrowserAutomationRoutes_UsesHandlerAndFallback(t *testing.T) {
	t.Run("handler", func(t *testing.T) {
		e := echo.New()
		apiProtected := e.Group("/api")
		handler := &stubRouteRegistrar{
			register: func(g *echo.Group) {
				g.GET("/live", func(c echo.Context) error {
					return c.String(http.StatusOK, "ok")
				})
			},
		}

		registerBrowserAutomationRoutes(apiProtected, handler, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/browser/live", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d, want 200, body=%s", rec.Code, rec.Body.String())
		}
		if handler.calls != 1 {
			t.Fatalf("RegisterRoutes calls=%d, want 1", handler.calls)
		}
	})

	t.Run("fallback", func(t *testing.T) {
		e := echo.New()
		apiProtected := e.Group("/api")

		registerBrowserAutomationRoutes(apiProtected, nil, nil)

		assertFeatureDisabledResponse(t, e, http.MethodGet, "/api/browser/tabs", "browser")
	})
}

func TestRegisterWorkflowRoutes_ConfiguresHandlerAndFallback(t *testing.T) {
	t.Run("handler", func(t *testing.T) {
		e := echo.New()
		v1 := e.Group("/api/v1")
		authMiddleware := func(next echo.HandlerFunc) echo.HandlerFunc { return next }
		handler := &stubWorkflowRouteRegistrar{
			register: func(e *echo.Echo) {
				e.GET("/api/v1/workflows/live", func(c echo.Context) error {
					return c.NoContent(http.StatusNoContent)
				})
			},
		}

		registerWorkflowRoutes(e, v1, handler, authMiddleware, nil, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/live", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("status=%d, want 204, body=%s", rec.Code, rec.Body.String())
		}
		if handler.calls != 1 {
			t.Fatalf("RegisterRoutes calls=%d, want 1", handler.calls)
		}
		if len(handler.middlewares) != 1 {
			t.Fatalf("middlewares len=%d, want 1", len(handler.middlewares))
		}
		if handler.serviceInitHook == nil {
			t.Fatal("expected service init hook to be wired")
		}
		handler.serviceInitHook(nil)
	})

	t.Run("fallback", func(t *testing.T) {
		e := echo.New()
		v1 := e.Group("/api/v1")

		registerWorkflowRoutes(e, v1, nil, nil, nil, nil)

		assertFeatureDisabledResponse(t, e, http.MethodGet, "/api/v1/workflows/stats", "workflow")
	})
}

func TestRegisterVoiceRoutes_UsesHandlersAndFallback(t *testing.T) {
	t.Run("handlers", func(t *testing.T) {
		e := echo.New()
		v1 := e.Group("/api/v1")
		protected := e.Group("/api/v1")
		voiceHandler := &stubRouteRegistrar{
			register: func(g *echo.Group) {
				g.GET("/live", func(c echo.Context) error {
					return c.String(http.StatusOK, "voice")
				})
			},
		}
		voiceWSHandler := &stubRouteRegistrar{
			register: func(g *echo.Group) {
				g.GET("/ws-live", func(c echo.Context) error {
					return c.String(http.StatusOK, "ws")
				})
			},
		}

		registerVoiceRoutes(v1, protected, voiceHandler, voiceWSHandler, nil, nil)

		reqVoice := httptest.NewRequest(http.MethodGet, "/api/v1/voice/live", nil)
		recVoice := httptest.NewRecorder()
		e.ServeHTTP(recVoice, reqVoice)
		if recVoice.Code != http.StatusOK {
			t.Fatalf("voice status=%d, want 200, body=%s", recVoice.Code, recVoice.Body.String())
		}

		reqWS := httptest.NewRequest(http.MethodGet, "/api/v1/voice/ws-live", nil)
		recWS := httptest.NewRecorder()
		e.ServeHTTP(recWS, reqWS)
		if recWS.Code != http.StatusOK {
			t.Fatalf("ws status=%d, want 200, body=%s", recWS.Code, recWS.Body.String())
		}

		if voiceHandler.calls != 1 {
			t.Fatalf("voice RegisterRoutes calls=%d, want 1", voiceHandler.calls)
		}
		if voiceWSHandler.calls != 1 {
			t.Fatalf("voice ws RegisterRoutes calls=%d, want 1", voiceWSHandler.calls)
		}
	})

	t.Run("fallback_ignores_ws_without_voice_handler", func(t *testing.T) {
		e := echo.New()
		v1 := e.Group("/api/v1")
		protected := e.Group("/api/v1")
		voiceWSHandler := &stubRouteRegistrar{
			register: func(g *echo.Group) {
				g.GET("/ws-live", func(c echo.Context) error {
					return c.String(http.StatusOK, "ws")
				})
			},
		}

		registerVoiceRoutes(v1, protected, nil, voiceWSHandler, nil, nil)

		assertFeatureDisabledResponse(t, e, http.MethodGet, "/api/v1/voice/ws-live", "voice")
		if voiceWSHandler.calls != 0 {
			t.Fatalf("voice ws RegisterRoutes calls=%d, want 0", voiceWSHandler.calls)
		}
	})
}

func TestRegisterSpeechRoutes_UsesHandlerAndFallback(t *testing.T) {
	t.Run("handler", func(t *testing.T) {
		e := echo.New()
		v1 := e.Group("/api/v1")
		handler := &stubRouteRegistrar{
			register: func(g *echo.Group) {
				g.GET("/live", func(c echo.Context) error {
					return c.String(http.StatusOK, "speech")
				})
			},
		}

		registerSpeechRoutes(v1, handler, nil, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/speech/live", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d, want 200, body=%s", rec.Code, rec.Body.String())
		}
		if handler.calls != 1 {
			t.Fatalf("RegisterRoutes calls=%d, want 1", handler.calls)
		}
	})

	t.Run("fallback", func(t *testing.T) {
		e := echo.New()
		v1 := e.Group("/api/v1")

		registerSpeechRoutes(v1, nil, nil, nil)

		assertFeatureDisabledResponse(t, e, http.MethodGet, "/api/v1/speech/models", "speech")
	})
}

func TestRegisterCompanionRoutes_UsesHandlersAndFallback(t *testing.T) {
	t.Run("handlers", func(t *testing.T) {
		e := echo.New()
		v1 := e.Group("/api/v1")
		companionV1 := e.Group("/api/v1")
		companionAPI := e.Group("/api")
		handler := &stubCompanionGroupRouteRegistrar{
			group: func(g *echo.Group) {
				g.GET("/companion/live", func(c echo.Context) error {
					return c.String(http.StatusOK, "companion")
				})
			},
			compat: func(g *echo.Group) {
				g.GET("/companion/compat-live", func(c echo.Context) error {
					return c.String(http.StatusOK, "compat")
				})
			},
		}
		wsHandler := &stubCompanionGroupRouteRegistrar{
			group: func(g *echo.Group) {
				g.GET("/companion/ws-live", func(c echo.Context) error {
					return c.String(http.StatusOK, "ws")
				})
			},
			compat: func(g *echo.Group) {
				g.GET("/companion/ws-compat-live", func(c echo.Context) error {
					return c.String(http.StatusOK, "ws-compat")
				})
			},
		}

		registration := registerRouteRuntimeCompanionRoutes(v1, companionV1, companionAPI, handler, wsHandler)

		assertRouteStatus := func(path string) {
			t.Helper()
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("path=%s status=%d, want 200, body=%s", path, rec.Code, rec.Body.String())
			}
		}

		assertRouteStatus("/api/v1/companion/live")
		assertRouteStatus("/api/companion/compat-live")
		assertRouteStatus("/api/v1/companion/ws-live")
		assertRouteStatus("/api/companion/ws-compat-live")

		if !registration.handlerRegistered {
			t.Fatal("expected handlerRegistered=true")
		}
		if !registration.wsRegistered {
			t.Fatal("expected wsRegistered=true")
		}
		if handler.groupCalls != 1 || handler.compatCalls != 1 {
			t.Fatalf("handler calls=(%d,%d), want (1,1)", handler.groupCalls, handler.compatCalls)
		}
		if wsHandler.groupCalls != 1 || wsHandler.compatCalls != 1 {
			t.Fatalf("ws calls=(%d,%d), want (1,1)", wsHandler.groupCalls, wsHandler.compatCalls)
		}
	})

	t.Run("fallback", func(t *testing.T) {
		e := echo.New()
		v1 := e.Group("/api/v1")

		registration := registerRouteRuntimeCompanionRoutes(v1, nil, nil, nil, nil)

		if registration.handlerRegistered || registration.wsRegistered {
			t.Fatalf("unexpected registration flags: %+v", registration)
		}
		assertFeatureDisabledResponse(t, e, http.MethodGet, "/api/v1/companion/stream", "companion")
	})
}

func TestRegisterFormfillerRoutes_UsesHandlerAndFallback(t *testing.T) {
	t.Run("handler", func(t *testing.T) {
		e := echo.New()
		v1 := e.Group("/api/v1")
		handler := &stubRouteRegistrar{
			register: func(g *echo.Group) {
				g.GET("/live", func(c echo.Context) error {
					return c.String(http.StatusOK, "formfiller")
				})
			},
		}

		registerFormfillerRoutes(v1, handler, nil, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/formfiller/live", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d, want 200, body=%s", rec.Code, rec.Body.String())
		}
		if handler.calls != 1 {
			t.Fatalf("RegisterRoutes calls=%d, want 1", handler.calls)
		}
	})

	t.Run("fallback", func(t *testing.T) {
		e := echo.New()
		v1 := e.Group("/api/v1")

		registerFormfillerRoutes(v1, nil, nil, nil)

		assertFeatureDisabledResponse(t, e, http.MethodGet, "/api/v1/formfiller/patterns", "formfiller")
	})
}

func TestRegisterBillingRoutes_UsesHandlerAndFallback(t *testing.T) {
	t.Run("handler", func(t *testing.T) {
		e := echo.New()
		protected := e.Group("/api/v1")
		handler := &stubRouteRegistrar{
			register: func(g *echo.Group) {
				g.GET("/summary", func(c echo.Context) error {
					return c.String(http.StatusOK, "billing")
				})
			},
		}

		registerBillingRoutes(protected, handler)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/billing/summary", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d, want 200, body=%s", rec.Code, rec.Body.String())
		}
		if handler.calls != 1 {
			t.Fatalf("RegisterRoutes calls=%d, want 1", handler.calls)
		}
	})

	t.Run("fallback", func(t *testing.T) {
		e := echo.New()
		protected := e.Group("/api/v1")

		registerBillingRoutes(protected, nil)

		assertFeatureDisabledResponse(t, e, http.MethodGet, "/api/v1/billing/export", "billing")
	})
}

func TestRegisterMetricsRoutes_UsesHandlersAndFallback(t *testing.T) {
	t.Run("handlers", func(t *testing.T) {
		e := echo.New()
		v1 := e.Group("/api/v1")
		summaryHandler := &stubRouteRegistrar{
			register: func(g *echo.Group) {
				g.GET("/system/metrics", func(c echo.Context) error {
					return c.String(http.StatusOK, "summary")
				})
			},
		}
		detailedHandler := &stubRouteRegistrar{
			register: func(g *echo.Group) {
				g.GET("/calls", func(c echo.Context) error {
					return c.String(http.StatusOK, "detailed")
				})
			},
		}
		writer := metrics.NewMetricsWriter(nil, &metrics.WriterConfig{EnableSystemMetrics: false})
		target := &stubMetricsRecorderTarget{}

		registerMetricsRoutes(v1, summaryHandler, detailedHandler, nil, nil, writer, target)

		reqSummary := httptest.NewRequest(http.MethodGet, "/api/v1/system/metrics", nil)
		recSummary := httptest.NewRecorder()
		e.ServeHTTP(recSummary, reqSummary)
		if recSummary.Code != http.StatusOK {
			t.Fatalf("summary status=%d, want 200, body=%s", recSummary.Code, recSummary.Body.String())
		}

		reqDetailed := httptest.NewRequest(http.MethodGet, "/api/v1/metrics/calls", nil)
		recDetailed := httptest.NewRecorder()
		e.ServeHTTP(recDetailed, reqDetailed)
		if recDetailed.Code != http.StatusOK {
			t.Fatalf("detailed status=%d, want 200, body=%s", recDetailed.Code, recDetailed.Body.String())
		}

		if summaryHandler.calls != 1 {
			t.Fatalf("summary RegisterRoutes calls=%d, want 1", summaryHandler.calls)
		}
		if detailedHandler.calls != 1 {
			t.Fatalf("detailed RegisterRoutes calls=%d, want 1", detailedHandler.calls)
		}
		if target.recorder != writer {
			t.Fatal("expected metrics writer to be wired to metrics target")
		}
	})

	t.Run("fallback", func(t *testing.T) {
		e := echo.New()
		v1 := e.Group("/api/v1")

		registerMetricsRoutes(v1, nil, nil, nil, nil, nil, nil)

		assertFeatureDisabledResponse(t, e, http.MethodGet, "/api/v1/metrics/all", "metrics")
	})
}

func TestRegisterMemoryRoutes_UsesHandlerAndFallback(t *testing.T) {
	t.Run("handler", func(t *testing.T) {
		e := echo.New()
		v1 := e.Group("/api/v1")
		handler := &stubMemoryRouteRegistrar{
			stubRouteRegistrar: stubRouteRegistrar{
				register: func(g *echo.Group) {
					g.GET("/memory/live", func(c echo.Context) error {
						return c.String(http.StatusOK, "memory")
					})
				},
			},
		}

		registerMemoryRoutes(v1, handler, nil, nil, nil, nil, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/memory/live", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d, want 200, body=%s", rec.Code, rec.Body.String())
		}
		if handler.calls != 1 {
			t.Fatalf("RegisterRoutes calls=%d, want 1", handler.calls)
		}
		if handler.onLayeredReady == nil {
			t.Fatal("expected layered memory hook to be wired")
		}
	})

	t.Run("fallback", func(t *testing.T) {
		e := echo.New()
		v1 := e.Group("/api/v1")

		registerMemoryRoutes(v1, nil, nil, nil, nil, nil, nil)

		assertFeatureDisabledResponse(t, e, http.MethodGet, "/api/v1/memory/backend", "memory")
	})
}

func TestRegisterProviderUnavailableRoutes_RegistersFallbacks(t *testing.T) {
	e := echo.New()
	protected := e.Group("/api/v1")

	registerProviderUnavailableRoutes(protected, nil)

	assertFeatureDisabledResponse(t, e, http.MethodGet, "/api/v1/providers", "providers")
	assertFeatureDisabledResponse(t, e, http.MethodGet, "/api/v1/models", "providers")
	assertFeatureDisabledResponse(t, e, http.MethodGet, "/api/v1/ide/scan", "providers")
	assertFeatureDisabledResponse(t, e, http.MethodGet, "/api/v1/pricing", "providers")
	assertFeatureDisabledResponse(t, e, http.MethodGet, "/api/v1/proxy/failover/config", "proxy_failover")
}

func TestRegisterProxyCacheRoutes_RegistersFallback(t *testing.T) {
	e := echo.New()
	v1 := e.Group("/api/v1")

	registerProxyCacheRoutes(v1, nil, nil)

	assertFeatureDisabledResponse(t, e, http.MethodGet, "/api/v1/proxy/cache/stats", "proxy_cache")
}

func TestRegisterChannelConfigRoutes_UsesHandlerAndFallback(t *testing.T) {
	t.Run("handler", func(t *testing.T) {
		e := echo.New()
		api := e.Group("/api")
		manager := &channel.Manager{}
		factory := &serverpkg.ChannelFactory{}
		handler := &stubChannelConfigRouteRegistrar{
			stubRouteRegistrar: stubRouteRegistrar{
				register: func(g *echo.Group) {
					g.GET("/channels/live", func(c echo.Context) error {
						return c.String(http.StatusOK, "channels")
					})
				},
			},
		}

		registerChannelConfigRoutes(api, handler, nil, nil, manager, factory)

		req := httptest.NewRequest(http.MethodGet, "/api/channels/live", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d, want 200, body=%s", rec.Code, rec.Body.String())
		}
		if handler.calls != 1 {
			t.Fatalf("RegisterRoutes calls=%d, want 1", handler.calls)
		}
		if handler.manager != manager {
			t.Fatal("expected manager to be wired")
		}
		if handler.factory != factory {
			t.Fatal("expected factory to be wired")
		}
	})

	t.Run("fallback", func(t *testing.T) {
		e := echo.New()
		api := e.Group("/api")

		registerChannelConfigRoutes(api, nil, nil, nil, nil, nil)

		assertFeatureDisabledResponse(t, e, http.MethodGet, "/api/channels", "channels")
	})
}

func TestRegisterUserScopedRoutes_UsesFactoryAndSkipsOnError(t *testing.T) {
	t.Run("handler", func(t *testing.T) {
		e := echo.New()
		myGroup := e.Group("/api/v1/my")
		handler := &stubRouteRegistrar{
			register: func(g *echo.Group) {
				g.GET("/live", func(c echo.Context) error {
					return c.String(http.StatusOK, "user-scoped")
				})
			},
		}

		registerRouteRuntimeUserScopedRoutes(
			myGroup,
			"/providers",
			nil,
			"unused failure",
			"unused success",
			func() (routeRegistrar, error) {
				return handler, nil
			},
		)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/my/providers/live", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d, want 200, body=%s", rec.Code, rec.Body.String())
		}
		if handler.calls != 1 {
			t.Fatalf("RegisterRoutes calls=%d, want 1", handler.calls)
		}
	})

	t.Run("factory error", func(t *testing.T) {
		e := echo.New()
		myGroup := e.Group("/api/v1/my")

		registerRouteRuntimeUserScopedRoutes(
			myGroup,
			"/skills",
			nil,
			"unused failure",
			"unused success",
			func() (routeRegistrar, error) {
				return nil, errors.New("boom")
			},
		)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/my/skills/live", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d, want 404, body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestRegisterMyUsageRoute_UsesHandlerAndFallback(t *testing.T) {
	t.Run("handler", func(t *testing.T) {
		e := echo.New()
		myGroup := e.Group("/api/v1/my")

		registerRouteRuntimeMyUsageRoute(myGroup, func(c echo.Context) error {
			return c.String(http.StatusOK, "usage")
		}, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/my/usage", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d, want 200, body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("fallback", func(t *testing.T) {
		e := echo.New()
		myGroup := e.Group("/api/v1/my")

		registerRouteRuntimeMyUsageRoute(myGroup, nil, nil)

		assertFeatureDisabledResponse(t, e, http.MethodGet, "/api/v1/my/usage", "metrics")
	})
}

func TestRegisterExternalAuthRoutes_UsesHandlerAndSkipsWhenNil(t *testing.T) {
	t.Run("handler", func(t *testing.T) {
		e := echo.New()
		v1 := e.Group("/api/v1")
		protected := e.Group("/api/v1")
		handler := &stubExternalAuthRouteRegistrar{
			public: func(g *echo.Group) {
				g.GET("/public-live", func(c echo.Context) error {
					return c.String(http.StatusOK, "public")
				})
			},
			protected: func(g *echo.Group) {
				g.GET("/protected-live", func(c echo.Context) error {
					return c.String(http.StatusOK, "protected")
				})
			},
		}

		registerExternalAuthRoutes(v1, protected, handler)

		for _, path := range []string{"/api/v1/auth/public-live", "/api/v1/auth/protected-live"} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("path=%s status=%d, want 200, body=%s", path, rec.Code, rec.Body.String())
			}
		}
		if handler.publicCalls != 1 || handler.protectedCalls != 1 {
			t.Fatalf("calls=(%d,%d), want (1,1)", handler.publicCalls, handler.protectedCalls)
		}
	})

	t.Run("nil", func(t *testing.T) {
		e := echo.New()
		v1 := e.Group("/api/v1")
		protected := e.Group("/api/v1")

		registerExternalAuthRoutes(v1, protected, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/public-live", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d, want 404, body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestRegisterAutoreplyRoutes_UsesHandlerAndSkipsWhenNil(t *testing.T) {
	t.Run("handler", func(t *testing.T) {
		e := echo.New()
		group := e.Group("/api/v1")
		handler := &stubRouteRegistrar{
			register: func(g *echo.Group) {
				g.GET("/autoreply/live", func(c echo.Context) error {
					return c.String(http.StatusOK, "autoreply")
				})
			},
		}

		registerAutoreplyRoutes(group, handler)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/autoreply/live", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d, want 200, body=%s", rec.Code, rec.Body.String())
		}
		if handler.calls != 1 {
			t.Fatalf("RegisterRoutes calls=%d, want 1", handler.calls)
		}
	})

	t.Run("nil", func(t *testing.T) {
		e := echo.New()
		group := e.Group("/api/v1")

		registerAutoreplyRoutes(group, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/autoreply/live", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d, want 404, body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestRuntimeRouteHelpersGo_DelegatesSecurityAutomationAndPlatformLanes(t *testing.T) {
	coreContent, err := os.ReadFile(filepath.Join("runtime_route_helpers_core.go"))
	if err != nil {
		t.Fatalf("read runtime_route_helpers_core.go: %v", err)
	}
	coreSource := string(coreContent)

	securityContent, err := os.ReadFile(filepath.Join("runtime_route_helpers_security.go"))
	if err != nil {
		t.Fatalf("read runtime_route_helpers_security.go: %v", err)
	}
	securitySource := string(securityContent)

	automationContent, err := os.ReadFile(filepath.Join("runtime_route_helpers_automation.go"))
	if err != nil {
		t.Fatalf("read runtime_route_helpers_automation.go: %v", err)
	}
	automationSource := string(automationContent)

	platformContent, err := os.ReadFile(filepath.Join("runtime_route_helpers_platform.go"))
	if err != nil {
		t.Fatalf("read runtime_route_helpers_platform.go: %v", err)
	}
	platformSource := string(platformContent)

	if lines := strings.Count(coreSource, "\n") + 1; lines > 50 {
		t.Fatalf("expected runtime_route_helpers_core.go to stay below 50 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(securitySource, "\n") + 1; lines > 115 {
		t.Fatalf("expected runtime_route_helpers_security.go to stay below 115 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(automationSource, "\n") + 1; lines > 120 {
		t.Fatalf("expected runtime_route_helpers_automation.go to stay below 120 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(platformSource, "\n") + 1; lines > 130 {
		t.Fatalf("expected runtime_route_helpers_platform.go to stay below 130 lines after extraction, got %d", lines)
	}

	requiredCore := []string{
		"func featureDisabled(",
		"func filterRouteMiddlewares(",
		"func routeRuntimeHasValue(",
	}
	for _, token := range requiredCore {
		if !strings.Contains(coreSource, token) {
			t.Fatalf("expected runtime_route_helpers_core.go to contain token %q", token)
		}
	}

	requiredSecurity := []string{
		"func registerSandboxRoutes(",
		"func registerBackupRoutes(",
		"func registerSecurityRoutes(",
	}
	for _, token := range requiredSecurity {
		if !strings.Contains(securitySource, token) {
			t.Fatalf("expected runtime_route_helpers_security.go to contain token %q", token)
		}
	}

	requiredAutomation := []string{
		"func registerCronRoutes(",
		"func registerBrowserAutomationRoutes(",
		"func registerWorkflowRoutes(",
		"func registerVoiceRoutes(",
		"func registerSpeechRoutes(",
	}
	for _, token := range requiredAutomation {
		if !strings.Contains(automationSource, token) {
			t.Fatalf("expected runtime_route_helpers_automation.go to contain token %q", token)
		}
	}

	requiredPlatform := []string{
		"func registerBillingRoutes(",
		"func registerMetricsRoutes(",
		"func registerProviderUnavailableRoutes(",
		"func registerProxyCacheRoutes(",
		"func registerChannelConfigRoutes(",
	}
	for _, token := range requiredPlatform {
		if !strings.Contains(platformSource, token) {
			t.Fatalf("expected runtime_route_helpers_platform.go to contain token %q", token)
		}
	}

	forbiddenCore := []string{
		"func registerSandboxRoutes(",
		"func registerCronRoutes(",
		"func registerBillingRoutes(",
	}
	for _, token := range forbiddenCore {
		if strings.Contains(coreSource, token) {
			t.Fatalf("expected runtime_route_helpers_core.go to delegate token %q", token)
		}
	}

	forbiddenSecurity := []string{
		"func featureDisabled(",
		"func registerCronRoutes(",
		"func registerBillingRoutes(",
	}
	for _, token := range forbiddenSecurity {
		if strings.Contains(securitySource, token) {
			t.Fatalf("expected runtime_route_helpers_security.go to delegate token %q", token)
		}
	}

	forbiddenAutomation := []string{
		"func featureDisabled(",
		"func registerSandboxRoutes(",
		"func registerBillingRoutes(",
	}
	for _, token := range forbiddenAutomation {
		if strings.Contains(automationSource, token) {
			t.Fatalf("expected runtime_route_helpers_automation.go to delegate token %q", token)
		}
	}

	forbiddenPlatform := []string{
		"func featureDisabled(",
		"func registerSandboxRoutes(",
		"func registerCronRoutes(",
	}
	for _, token := range forbiddenPlatform {
		if strings.Contains(platformSource, token) {
			t.Fatalf("expected runtime_route_helpers_platform.go to delegate token %q", token)
		}
	}
}
