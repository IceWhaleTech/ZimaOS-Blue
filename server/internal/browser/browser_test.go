package browser

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, 3, config.PoolSize)
	assert.True(t, config.Headless)
	assert.Equal(t, 30000, config.DefaultTimeout)
	assert.Equal(t, 120000, config.MaxTimeout)
	assert.Equal(t, 1920, config.DefaultViewportWidth)
	assert.Equal(t, 1080, config.DefaultViewportHeight)
	assert.Empty(t, config.AllowedDomains)
	assert.Empty(t, config.BlockedDomains)
}

func TestGetTimeout(t *testing.T) {
	config := &Config{
		DefaultTimeout: 30000,
		MaxTimeout:     120000,
	}

	tests := []struct {
		name      string
		requested int
		expected  time.Duration
	}{
		{
			name:      "zero uses default",
			requested: 0,
			expected:  30 * time.Second,
		},
		{
			name:      "negative uses default",
			requested: -1,
			expected:  30 * time.Second,
		},
		{
			name:      "within limits",
			requested: 60000,
			expected:  60 * time.Second,
		},
		{
			name:      "exceeds max uses max",
			requested: 200000,
			expected:  120 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetTimeout(tt.requested, config)
			assert.Equal(t, tt.expected, result)
		})
	}
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
