package browser

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, 3, config.PoolSize)
	assert.True(t, config.Headless)
	assert.False(t, config.RelayEnabled)
	assert.Equal(t, DefaultRelayHost, config.RelayHost)
	assert.Equal(t, DefaultRelayPort, config.RelayPort)
	assert.Equal(t, 60000, config.DefaultTimeout)
	assert.Equal(t, 300000, config.MaxTimeout)
	assert.Equal(t, 1920, config.DefaultViewportWidth)
	assert.Equal(t, 1080, config.DefaultViewportHeight)
	assert.Empty(t, config.AllowedDomains)
	assert.Empty(t, config.BlockedDomains)
}

func TestGetTimeout(t *testing.T) {
	config := &Config{
		DefaultTimeout: 60000,
		MaxTimeout:     300000,
	}

	tests := []struct {
		name      string
		requested int
		expected  time.Duration
	}{
		{
			name:      "zero uses default",
			requested: 0,
			expected:  60 * time.Second,
		},
		{
			name:      "negative uses default",
			requested: -1,
			expected:  60 * time.Second,
		},
		{
			name:      "within limits",
			requested: 60000,
			expected:  60 * time.Second,
		},
		{
			name:      "exceeds max uses max",
			requested: 400000,
			expected:  300 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetTimeout(tt.requested, config)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCleanupExpiredMonitorFramesUsesRetention(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SessionScreenshotRetention = 24 * time.Hour

	service, err := NewService(cfg)
	require.NoError(t, err)

	now := time.Now()
	service.tabs["tab-live"] = &tabInfo{targetID: "tab-live", active: true}
	service.tabs[detachedMonitorTargetID] = &tabInfo{
		targetID: detachedMonitorTargetID,
		detached: true,
	}
	service.screenshotHistory["tab-live"] = []SessionScreenshot{
		{
			Data:       "old-frame",
			CapturedAt: now.Add(-72 * time.Hour).Format(time.RFC3339),
			Title:      "Old frame",
		},
		{
			Data:       "fresh-frame",
			CapturedAt: now.Add(-2 * time.Hour).Format(time.RFC3339),
			Title:      "Fresh frame",
		},
	}
	service.screenshotHistory[detachedMonitorTargetID] = []SessionScreenshot{
		{
			Data:       "stale-detached",
			CapturedAt: now.Add(-72 * time.Hour).Format(time.RFC3339),
			Title:      "Detached frame",
		},
	}

	service.CleanupExpiredMonitorFrames()

	history := service.SessionScreenshotHistory("tab-live")
	require.Len(t, history, 1)
	assert.Equal(t, "fresh-frame", history[0].Data)
	assert.Equal(t, "Fresh frame", history[0].Title)
	assert.Nil(t, service.SessionScreenshotHistory(detachedMonitorTargetID))

	_, detachedExists := service.tabs[detachedMonitorTargetID]
	assert.False(t, detachedExists)
}

func TestSecurityChecker_CheckURL(t *testing.T) {
	tests := []struct {
		name           string
		allowedDomains []string
		blockedDomains []string
		url            string
		expectError    error
	}{
		{
			name:           "allow all when no restrictions",
			allowedDomains: []string{},
			blockedDomains: []string{},
			url:            "https://example.com",
			expectError:    nil,
		},
		{
			name:           "block blocked domain",
			allowedDomains: []string{},
			blockedDomains: []string{"blocked.com"},
			url:            "https://blocked.com/page",
			expectError:    ErrURLBlocked,
		},
		{
			name:           "block subdomain of blocked domain",
			allowedDomains: []string{},
			blockedDomains: []string{"blocked.com"},
			url:            "https://sub.blocked.com/page",
			expectError:    ErrURLBlocked,
		},
		{
			name:           "allow allowed domain",
			allowedDomains: []string{"allowed.com"},
			blockedDomains: []string{},
			url:            "https://allowed.com/page",
			expectError:    nil,
		},
		{
			name:           "allow subdomain of allowed domain",
			allowedDomains: []string{"allowed.com"},
			blockedDomains: []string{},
			url:            "https://sub.allowed.com/page",
			expectError:    nil,
		},
		{
			name:           "allow bare domain when allowlist set",
			allowedDomains: []string{"allowed.com"},
			blockedDomains: []string{},
			url:            "allowed.com/page",
			expectError:    nil,
		},
		{
			name:           "ignore empty allowlist entries",
			allowedDomains: []string{"", "allowed.com"},
			blockedDomains: []string{},
			url:            "https://allowed.com/page",
			expectError:    nil,
		},
		{
			name:           "reject non-allowed domain",
			allowedDomains: []string{"allowed.com"},
			blockedDomains: []string{},
			url:            "https://other.com/page",
			expectError:    ErrURLNotAllowed,
		},
		{
			name:           "blocked takes precedence over allowed",
			allowedDomains: []string{"example.com"},
			blockedDomains: []string{"example.com"},
			url:            "https://example.com/page",
			expectError:    ErrURLBlocked,
		},
		{
			name:           "wildcard allowed domain",
			allowedDomains: []string{"*.example.com"},
			blockedDomains: []string{},
			url:            "https://sub.example.com/page",
			expectError:    nil,
		},
		{
			name:           "invalid URL",
			allowedDomains: []string{},
			blockedDomains: []string{},
			url:            "://invalid",
			expectError:    ErrURLNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				AllowedDomains: tt.allowedDomains,
				BlockedDomains: tt.blockedDomains,
			}
			checker := NewSecurityChecker(config)

			err := checker.CheckURL(tt.url)
			assert.Equal(t, tt.expectError, err)
		})
	}
}

func TestSecurityChecker_NormalizeAndCheckURL(t *testing.T) {
	checker := NewSecurityChecker(&Config{})

	t.Run("bare domain is normalized to https", func(t *testing.T) {
		normalized, err := checker.NormalizeAndCheckURL("example.com/path")
		require.NoError(t, err)
		assert.Equal(t, "https://example.com/path", normalized)
	})

	t.Run("wrapped url and spaces are normalized", func(t *testing.T) {
		normalized, err := checker.NormalizeAndCheckURL(` <"https://example.com/a b"> `)
		require.NoError(t, err)
		assert.Equal(t, "https://example.com/a%20b", normalized)
	})
}

func TestSecurityChecker_NormalizeAndCheckURL_Table(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		want       string
		wantErr    error
		checkerCfg *Config
	}{
		{
			name:       "protocol-relative URL gets https scheme",
			input:      "//example.com/path",
			want:       "https://example.com/path",
			checkerCfg: &Config{},
		},
		{
			name:       "bare localhost with port is normalized",
			input:      "localhost:8080/health",
			want:       "https://localhost:8080/health",
			checkerCfg: &Config{},
		},
		{
			name:       "bare IPv4 with port is normalized",
			input:      "127.0.0.1:9222",
			want:       "https://127.0.0.1:9222",
			checkerCfg: &Config{},
		},
		{
			name:       "wrapped bare domain is normalized",
			input:      ` <"example.com/docs"> `,
			want:       "https://example.com/docs",
			checkerCfg: &Config{},
		},
		{
			name:       "sentence with https url is extracted and normalized",
			input:      "请打开 https://www.zimaos.com/docs，帮我看一下",
			want:       "https://www.zimaos.com/docs",
			checkerCfg: &Config{},
		},
		{
			name:       "sentence with bare domain token is extracted and normalized",
			input:      "请访问 example.com/path 并检查导航",
			want:       "https://example.com/path",
			checkerCfg: &Config{},
		},
		{
			name:       "invalid command-like text is rejected",
			input:      "go test ./...",
			wantErr:    ErrURLNotAllowed,
			checkerCfg: &Config{},
		},
		{
			name:  "allowlist works with bare domain input",
			input: "allowed.com/page",
			want:  "https://allowed.com/page",
			checkerCfg: &Config{
				AllowedDomains: []string{"allowed.com"},
			},
		},
		{
			name:    "blocklist still takes precedence after normalization",
			input:   "blocked.com/page",
			wantErr: ErrURLBlocked,
			checkerCfg: &Config{
				AllowedDomains: []string{"blocked.com"},
				BlockedDomains: []string{"blocked.com"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.checkerCfg
			if cfg == nil {
				cfg = &Config{}
			}
			checker := NewSecurityChecker(cfg)
			got, err := checker.NormalizeAndCheckURL(tt.input)
			if tt.wantErr != nil {
				assert.Equal(t, tt.wantErr, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLooksLikeHostCandidate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "domain", input: "example.com", want: true},
		{name: "localhost with port", input: "localhost:8080", want: true},
		{name: "IPv4", input: "127.0.0.1", want: true},
		{name: "IPv6 with port", input: "[::1]:9000", want: true},
		{name: "single token without dot", input: "example", want: false},
		{name: "contains spaces", input: "exa mple.com", want: false},
		{name: "path only", input: "/tmp/file", want: false},
		{name: "command-like text", input: "go test ./...", want: false},
		{name: "empty", input: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, looksLikeHostCandidate(tt.input))
		})
	}
}

func TestSecurityChecker_ValidateSelector(t *testing.T) {
	checker := NewSecurityChecker(&Config{})

	tests := []struct {
		name        string
		selector    string
		expectError bool
	}{
		{
			name:        "empty selector",
			selector:    "",
			expectError: false,
		},
		{
			name:        "valid CSS selector",
			selector:    "#id .class",
			expectError: false,
		},
		{
			name:        "javascript protocol",
			selector:    "javascript:alert(1)",
			expectError: true,
		},
		{
			name:        "data protocol",
			selector:    "data:text/html,<script>",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checker.ValidateSelector(tt.selector)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestScreenshotFormat_toProto(t *testing.T) {
	tests := []struct {
		format   ScreenshotFormat
		expected string
	}{
		{FormatPNG, "png"},
		{FormatJPEG, "jpeg"},
		{FormatWebP, "webp"},
		{"unknown", "png"}, // default to PNG
	}

	for _, tt := range tests {
		t.Run(string(tt.format), func(t *testing.T) {
			result := tt.format.toProto()
			assert.Equal(t, tt.expected, string(result))
		})
	}
}

func TestDefaultViewport(t *testing.T) {
	viewport := DefaultViewport()

	assert.Equal(t, 1920, viewport.Width)
	assert.Equal(t, 1080, viewport.Height)
}

func TestNewService(t *testing.T) {
	t.Run("with nil config uses defaults", func(t *testing.T) {
		service, err := NewService(nil)
		require.NoError(t, err)
		require.NotNil(t, service)
		assert.NotNil(t, service.config)
		assert.NotNil(t, service.pool)
		assert.NotNil(t, service.security)
	})

	t.Run("with custom config", func(t *testing.T) {
		config := &Config{
			PoolSize:       5,
			Headless:       false,
			DefaultTimeout: 60000,
		}
		service, err := NewService(config)
		require.NoError(t, err)
		require.NotNil(t, service)
		assert.Equal(t, 5, service.config.PoolSize)
		assert.False(t, service.config.Headless)
	})
}

func TestNewPool(t *testing.T) {
	t.Run("with nil config uses defaults", func(t *testing.T) {
		pool, err := NewPool(nil)
		require.NoError(t, err)
		require.NotNil(t, pool)
		assert.NotNil(t, pool.config)
	})

	t.Run("with custom config", func(t *testing.T) {
		config := &Config{
			PoolSize: 10,
		}
		pool, err := NewPool(config)
		require.NoError(t, err)
		require.NotNil(t, pool)
		assert.Equal(t, 10, pool.config.PoolSize)
	})
}

func TestMapError(t *testing.T) {
	tests := []struct {
		err            error
		expectedStatus int
	}{
		{ErrBrowserNotAvailable, 503},
		{ErrBrowserNotRunning, 503},
		{ErrPageNotFound, 404},
		{ErrTabNotFound, 404},
		{ErrElementNotFound, 404},
		{ErrURLNotAllowed, 403},
		{ErrURLBlocked, 403},
		{ErrTimeout, 504},
		{ErrResourceLimitExceeded, 429},
		{ErrInvalidSelector, 400},
		{ErrInvalidAction, 400},
	}

	for _, tt := range tests {
		t.Run(tt.err.Error(), func(t *testing.T) {
			httpErr := mapError(tt.err)
			assert.Equal(t, tt.expectedStatus, httpErr.Code)
		})
	}
}

func TestActionTypes(t *testing.T) {
	// Verify action type constants
	assert.Equal(t, ActionType("click"), ActionClick)
	assert.Equal(t, ActionType("type"), ActionTypeText)
	assert.Equal(t, ActionType("select"), ActionSelect)
	assert.Equal(t, ActionType("wait"), ActionWait)
	assert.Equal(t, ActionType("wait_for"), ActionWaitFor)
	assert.Equal(t, ActionType("scroll"), ActionScroll)
	assert.Equal(t, ActionType("screenshot"), ActionScreenshot)
	assert.Equal(t, ActionType("navigate"), ActionNavigate)
	assert.Equal(t, ActionType("eval"), ActionEval)
	assert.Equal(t, ActionType("hover"), ActionHover)
	assert.Equal(t, ActionType("drag"), ActionDrag)
	assert.Equal(t, ActionType("press"), ActionPress)
	assert.Equal(t, ActionType("fill"), ActionFill)
}

func TestPDFFormats(t *testing.T) {
	assert.Equal(t, PDFFormat("A4"), PDFFormatA4)
	assert.Equal(t, PDFFormat("Letter"), PDFFormatLetter)
	assert.Equal(t, PDFFormat("Legal"), PDFFormatLegal)
}

func TestScreenshotFormats(t *testing.T) {
	assert.Equal(t, ScreenshotFormat("png"), FormatPNG)
	assert.Equal(t, ScreenshotFormat("jpeg"), FormatJPEG)
	assert.Equal(t, ScreenshotFormat("webp"), FormatWebP)
}

func TestIsConnectionClosed(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		expect bool
	}{
		{"nil", nil, false},
		{"normal error", fmt.Errorf("timeout"), false},
		{"closed connection", fmt.Errorf("write tcp 127.0.0.1:1234->127.0.0.1:5678: use of closed network connection"), true},
		{"connection reset", fmt.Errorf("connection reset by peer"), true},
		{"broken pipe", fmt.Errorf("write: broken pipe"), true},
		{"websocket close", fmt.Errorf("websocket: close 1006"), true},
		{"EOF", fmt.Errorf("EOF"), true},
		{"wrapped", fmt.Errorf("navigate failed: %w", fmt.Errorf("use of closed network connection")), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, isConnectionClosed(tt.err))
		})
	}
}
