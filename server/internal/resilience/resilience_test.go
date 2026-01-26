package resilience

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPanicRecoveryMiddleware(t *testing.T) {
	t.Run("recovers from panic", func(t *testing.T) {
		m := NewPanicRecoveryMiddleware(PanicRecoveryConfig{})
		e := echo.New()
		e.Use(m.Middleware())

		e.GET("/panic", func(c echo.Context) error {
			panic("test panic")
		})

		req := httptest.NewRequest(http.MethodGet, "/panic", nil)
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.Equal(t, int64(1), m.PanicCount())
	})

	t.Run("records panic history", func(t *testing.T) {
		m := NewPanicRecoveryMiddleware(PanicRecoveryConfig{
			MaxPanicHistory: 5,
		})
		e := echo.New()
		e.Use(m.Middleware())

		e.GET("/panic", func(c echo.Context) error {
			panic("test panic")
		})

		// Trigger multiple panics
		for i := 0; i < 3; i++ {
			req := httptest.NewRequest(http.MethodGet, "/panic", nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
		}

		history := m.PanicHistory()
		assert.Len(t, history, 3)
		assert.Equal(t, int64(3), m.PanicCount())
	})

	t.Run("rotates panic history", func(t *testing.T) {
		m := NewPanicRecoveryMiddleware(PanicRecoveryConfig{
			MaxPanicHistory: 2,
		})
		e := echo.New()
		e.Use(m.Middleware())

		e.GET("/panic", func(c echo.Context) error {
			panic("test panic")
		})

		// Trigger more panics than history size
		for i := 0; i < 5; i++ {
			req := httptest.NewRequest(http.MethodGet, "/panic", nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
		}

		history := m.PanicHistory()
		assert.Len(t, history, 2)
		assert.Equal(t, int64(5), m.PanicCount())
	})

	t.Run("calls OnPanic callback", func(t *testing.T) {
		called := false
		m := NewPanicRecoveryMiddleware(PanicRecoveryConfig{
			OnPanic: func(c echo.Context, err interface{}, stack []byte) {
				called = true
				assert.Equal(t, "test panic", err)
				assert.NotEmpty(t, stack)
			},
		})
		e := echo.New()
		e.Use(m.Middleware())

		e.GET("/panic", func(c echo.Context) error {
			panic("test panic")
		})

		req := httptest.NewRequest(http.MethodGet, "/panic", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.True(t, called)
	})

	t.Run("normal requests pass through", func(t *testing.T) {
		m := NewPanicRecoveryMiddleware(PanicRecoveryConfig{})
		e := echo.New()
		e.Use(m.Middleware())

		e.GET("/ok", func(c echo.Context) error {
			return c.String(http.StatusOK, "ok")
		})

		req := httptest.NewRequest(http.MethodGet, "/ok", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, int64(0), m.PanicCount())
	})

	t.Run("reset clears history", func(t *testing.T) {
		m := NewPanicRecoveryMiddleware(PanicRecoveryConfig{})
		e := echo.New()
		e.Use(m.Middleware())

		e.GET("/panic", func(c echo.Context) error {
			panic("test panic")
		})

		req := httptest.NewRequest(http.MethodGet, "/panic", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, int64(1), m.PanicCount())

		m.Reset()

		assert.Equal(t, int64(0), m.PanicCount())
		assert.Empty(t, m.PanicHistory())
	})

	t.Run("stats returns correct data", func(t *testing.T) {
		m := NewPanicRecoveryMiddleware(PanicRecoveryConfig{})
		e := echo.New()
		e.Use(m.Middleware())

		e.GET("/panic", func(c echo.Context) error {
			panic("test panic")
		})

		req := httptest.NewRequest(http.MethodGet, "/panic", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		stats := m.Stats()
		assert.Equal(t, int64(1), stats["total_panics"])
		assert.Equal(t, 1, stats["history_count"])
		assert.NotNil(t, stats["last_panic"])
	})
}

func TestTimeoutMiddleware(t *testing.T) {
	t.Run("default config", func(t *testing.T) {
		m := NewTimeoutMiddleware(TimeoutConfig{})
		require.NotNil(t, m)
	})

	t.Run("skipper works", func(t *testing.T) {
		m := NewTimeoutMiddleware(TimeoutConfig{
			Skipper: func(c echo.Context) bool {
				return c.Path() == "/skip"
			},
		})
		e := echo.New()
		e.Use(m.Middleware())

		e.GET("/skip", func(c echo.Context) error {
			return c.String(http.StatusOK, "skipped")
		})

		req := httptest.NewRequest(http.MethodGet, "/skip", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("set path timeout", func(t *testing.T) {
		m := NewTimeoutMiddleware(TimeoutConfig{})
		m.SetPathTimeout("/custom", 5000000000) // 5 seconds
		m.RemovePathTimeout("/custom")
	})
}

func TestBulkhead(t *testing.T) {
	t.Run("allows requests within limit", func(t *testing.T) {
		b := NewBulkhead(BulkheadConfig{
			MaxConcurrent: 2,
			MaxWaiting:    10,
		})

		err := b.Execute(func() error {
			return nil
		})

		assert.NoError(t, err)
	})

	t.Run("rejects when full", func(t *testing.T) {
		b := NewBulkhead(BulkheadConfig{
			MaxConcurrent: 1,
			MaxWaiting:    0,
		})

		// Fill the bulkhead
		done := make(chan struct{})
		go func() {
			b.Execute(func() error {
				<-done
				return nil
			})
		}()

		// Wait for goroutine to acquire slot
		for len(b.semaphore) == 0 {
		}

		// This should be rejected
		err := b.Execute(func() error {
			return nil
		})

		close(done)
		assert.ErrorIs(t, err, ErrBulkheadFull)
	})

	t.Run("stats returns correct data", func(t *testing.T) {
		b := NewBulkhead(BulkheadConfig{
			Name:          "test",
			MaxConcurrent: 5,
			MaxWaiting:    10,
		})

		b.Execute(func() error { return nil })

		stats := b.Stats()
		assert.Equal(t, "test", stats["name"])
		assert.Equal(t, 5, stats["max_concurrent"])
		assert.Equal(t, 10, stats["max_waiting"])
		assert.Equal(t, int64(1), stats["total_completed"])
	})

	t.Run("middleware returns 503 when full", func(t *testing.T) {
		b := NewBulkhead(BulkheadConfig{
			MaxConcurrent: 1,
			MaxWaiting:    0,
		})

		e := echo.New()
		e.Use(b.Middleware())

		e.GET("/test", func(c echo.Context) error {
			return c.String(http.StatusOK, "ok")
		})

		// Fill the bulkhead
		done := make(chan struct{})
		go func() {
			b.Execute(func() error {
				<-done
				return nil
			})
		}()

		// Wait for goroutine to acquire slot
		for len(b.semaphore) == 0 {
		}

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		close(done)
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	})
}

func TestBulkheadRegistry(t *testing.T) {
	t.Run("creates and retrieves bulkheads", func(t *testing.T) {
		r := NewBulkheadRegistry()

		b1 := r.Get("test1", BulkheadConfig{MaxConcurrent: 5})
		b2 := r.Get("test1", BulkheadConfig{MaxConcurrent: 10})

		assert.Same(t, b1, b2)
	})

	t.Run("stats returns all bulkheads", func(t *testing.T) {
		r := NewBulkheadRegistry()

		r.Get("test1", BulkheadConfig{})
		r.Get("test2", BulkheadConfig{})

		stats := r.Stats()
		assert.Len(t, stats, 2)
	})
}

func TestErrorClassification(t *testing.T) {
	t.Run("error categories have correct HTTP status", func(t *testing.T) {
		assert.Equal(t, http.StatusBadRequest, CategoryValidation.HTTPStatus())
		assert.Equal(t, http.StatusUnauthorized, CategoryAuthentication.HTTPStatus())
		assert.Equal(t, http.StatusForbidden, CategoryAuthorization.HTTPStatus())
		assert.Equal(t, http.StatusNotFound, CategoryNotFound.HTTPStatus())
		assert.Equal(t, http.StatusConflict, CategoryConflict.HTTPStatus())
		assert.Equal(t, http.StatusTooManyRequests, CategoryRateLimit.HTTPStatus())
		assert.Equal(t, http.StatusGatewayTimeout, CategoryTimeout.HTTPStatus())
		assert.Equal(t, http.StatusServiceUnavailable, CategoryUnavailable.HTTPStatus())
		assert.Equal(t, http.StatusInternalServerError, CategoryInternal.HTTPStatus())
		assert.Equal(t, http.StatusBadGateway, CategoryExternal.HTTPStatus())
		assert.Equal(t, http.StatusBadGateway, CategoryLLM.HTTPStatus())
	})

	t.Run("app error constructors", func(t *testing.T) {
		err := ErrValidation("INVALID_INPUT", "Invalid input")
		assert.Equal(t, CategoryValidation, err.Category)
		assert.Equal(t, "INVALID_INPUT", err.Code)
		assert.Equal(t, "Invalid input", err.Message)

		err = ErrRateLimit("RATE_LIMITED", "Too many requests")
		assert.True(t, err.Retryable)
	})

	t.Run("app error with cause", func(t *testing.T) {
		cause := assert.AnError
		err := ErrInternal("INTERNAL", "Internal error").WithCause(cause)

		assert.ErrorIs(t, err, cause)
		assert.Contains(t, err.Error(), "caused by")
	})

	t.Run("app error with details", func(t *testing.T) {
		details := map[string]string{"field": "email"}
		err := ErrValidation("INVALID_EMAIL", "Invalid email").WithDetails(details)

		assert.Equal(t, details, err.Details)
	})

	t.Run("error classifier", func(t *testing.T) {
		c := NewErrorClassifier()

		// Test with AppError
		appErr := ErrValidation("TEST", "test")
		assert.Equal(t, CategoryValidation, c.Classify(appErr))

		// Test with circuit breaker error
		assert.Equal(t, CategoryUnavailable, c.Classify(ErrCircuitOpen))

		// Test with bulkhead error
		assert.Equal(t, CategoryUnavailable, c.Classify(ErrBulkheadFull))

		// Test with unknown error
		assert.Equal(t, CategoryUnknown, c.Classify(assert.AnError))
	})

	t.Run("error response conversion", func(t *testing.T) {
		err := ErrValidation("INVALID", "Invalid").WithDetails(map[string]string{"field": "name"})
		resp := err.ToResponse()

		assert.Equal(t, "validation", resp.Error.Category)
		assert.Equal(t, "INVALID", resp.Error.Code)
		assert.Equal(t, "Invalid", resp.Error.Message)
		assert.NotNil(t, resp.Error.Details)
	})
}

func TestDegradationManager(t *testing.T) {
	t.Run("default level is normal", func(t *testing.T) {
		m := NewDegradationManager(DegradationConfig{})
		assert.Equal(t, LevelNormal, m.Level())
	})

	t.Run("manual level setting", func(t *testing.T) {
		m := NewDegradationManager(DegradationConfig{})
		m.SetLevel(LevelPartial)
		assert.Equal(t, LevelPartial, m.Level())
	})

	t.Run("error rate calculation", func(t *testing.T) {
		m := NewDegradationManager(DegradationConfig{})

		m.RecordRequest(true)
		m.RecordRequest(true)
		m.RecordRequest(false)
		m.RecordRequest(false)

		assert.Equal(t, 0.5, m.ErrorRate())
	})

	t.Run("level change callback", func(t *testing.T) {
		called := false
		m := NewDegradationManager(DegradationConfig{
			OnLevelChange: func(from, to DegradationLevel) {
				called = true
				assert.Equal(t, LevelNormal, from)
				assert.Equal(t, LevelEmergency, to)
			},
		})

		m.SetLevel(LevelEmergency)

		// Wait for callback
		for !called {
		}
		assert.True(t, called)
	})

	t.Run("stats returns correct data", func(t *testing.T) {
		m := NewDegradationManager(DegradationConfig{})
		m.RecordRequest(true)
		m.RecordRequest(false)

		stats := m.Stats()
		assert.Equal(t, "normal", stats["level"])
		assert.Equal(t, int64(2), stats["total_requests"])
		assert.Equal(t, int64(1), stats["failed_requests"])
	})

	t.Run("feature enabled check", func(t *testing.T) {
		m := NewDegradationManager(DegradationConfig{})

		assert.True(t, m.IsFeatureEnabled("feature1", LevelNormal))
		assert.True(t, m.IsFeatureEnabled("feature2", LevelPartial))

		m.SetLevel(LevelPartial)
		assert.False(t, m.IsFeatureEnabled("feature1", LevelNormal))
		assert.True(t, m.IsFeatureEnabled("feature2", LevelPartial))
	})
}

func TestFallbackRegistry(t *testing.T) {
	t.Run("register and get fallback", func(t *testing.T) {
		r := NewFallbackRegistry()

		r.Register("/api/test", FallbackResponse{
			StatusCode: 200,
			Body:       map[string]string{"status": "fallback"},
		})

		resp, ok := r.Get("/api/test")
		assert.True(t, ok)
		assert.Equal(t, 200, resp.StatusCode)
	})

	t.Run("get non-existent fallback", func(t *testing.T) {
		r := NewFallbackRegistry()

		_, ok := r.Get("/api/nonexistent")
		assert.False(t, ok)
	})
}

func TestResponseCache(t *testing.T) {
	t.Run("set and get", func(t *testing.T) {
		c := NewResponseCache(5000000000) // 5 seconds

		c.Set("key1", "value1")

		val, ok := c.Get("key1")
		assert.True(t, ok)
		assert.Equal(t, "value1", val)
	})

	t.Run("get non-existent", func(t *testing.T) {
		c := NewResponseCache(5000000000)

		_, ok := c.Get("nonexistent")
		assert.False(t, ok)
	})

	t.Run("clear cache", func(t *testing.T) {
		c := NewResponseCache(5000000000)

		c.Set("key1", "value1")
		c.Clear()

		_, ok := c.Get("key1")
		assert.False(t, ok)
	})
}

func TestHealthBasedRouter(t *testing.T) {
	t.Run("register and get healthy backend", func(t *testing.T) {
		r := NewHealthBasedRouter()

		r.RegisterBackend("backend1", 10)
		r.RegisterBackend("backend2", 5)

		backend := r.GetHealthyBackend()
		assert.Equal(t, "backend1", backend)
	})

	t.Run("record failure marks unhealthy", func(t *testing.T) {
		r := NewHealthBasedRouter()

		r.RegisterBackend("backend1", 10)

		r.RecordResult("backend1", false)
		r.RecordResult("backend1", false)
		r.RecordResult("backend1", false)

		backend := r.GetHealthyBackend()
		assert.Empty(t, backend)
	})

	t.Run("record success marks healthy", func(t *testing.T) {
		r := NewHealthBasedRouter()

		r.RegisterBackend("backend1", 10)

		r.RecordResult("backend1", false)
		r.RecordResult("backend1", false)
		r.RecordResult("backend1", false)
		r.RecordResult("backend1", true)

		backend := r.GetHealthyBackend()
		assert.Equal(t, "backend1", backend)
	})

	t.Run("stats returns all backends", func(t *testing.T) {
		r := NewHealthBasedRouter()

		r.RegisterBackend("backend1", 10)
		r.RegisterBackend("backend2", 5)

		stats := r.Stats()
		assert.Len(t, stats, 2)
	})
}
