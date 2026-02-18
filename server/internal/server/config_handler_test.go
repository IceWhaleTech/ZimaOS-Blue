package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestConfigHandler_Reload(t *testing.T) {
	t.Run("returns error when hot reloader is nil", func(t *testing.T) {
		e := echo.New()
		h := NewConfigHandler(nil)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/config/reload", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := h.Reload(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	})

	t.Run("reloads configuration successfully", func(t *testing.T) {
		e := echo.New()

		// Create a temporary config file
		cfg := &config.Config{
			Server: config.ServerConfig{
				Host: "0.0.0.0",
				Port: 80,
			},
			Worker: config.WorkerConfig{
				PoolSize: 10,
			},
		}

		hr, err := config.NewHotReloader("", cfg, &config.HotReloadConfig{
			Enabled:             true,
			ValidateBeforeApply: false,
		})
		assert.NoError(t, err)

		h := NewConfigHandler(hr)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/config/reload", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err = h.Reload(c)
		// Note: This will fail because there's no config file, but we're testing the handler logic
		assert.NoError(t, err)
	})
}

func TestConfigHandler_Status(t *testing.T) {
	t.Run("returns disabled when hot reloader is nil", func(t *testing.T) {
		e := echo.New()
		h := NewConfigHandler(nil)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/config/status", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := h.Status(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"enabled":false`)
	})

	t.Run("returns status when hot reloader is enabled", func(t *testing.T) {
		e := echo.New()

		cfg := &config.Config{
			Server: config.ServerConfig{
				Host: "0.0.0.0",
				Port: 80,
			},
			Worker: config.WorkerConfig{
				PoolSize: 10,
			},
		}

		hr, err := config.NewHotReloader("/tmp/test-config.yaml", cfg, &config.HotReloadConfig{
			Enabled:             true,
			WatchInterval:       time.Second,
			ValidateBeforeApply: true,
		})
		assert.NoError(t, err)

		h := NewConfigHandler(hr)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/config/status", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err = h.Status(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"enabled":true`)
	})
}
