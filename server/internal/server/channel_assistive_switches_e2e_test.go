package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/i18n"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type mediaInterceptorMock struct {
	isMedia bool
	calls   int
}

func (m *mediaInterceptorMock) ClassifyAndGenerate(ctx context.Context, message string, hasImages bool, imageCount int, locale string, source string) (string, bool, error) {
	m.calls++
	if m.isMedia {
		return "task-1", true, nil
	}
	return "", false, nil
}

func TestChannelAssistiveSwitchesE2E_MediaIntentGate(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	defer store.Close()

	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	registry := llm.NewProviderRegistry()
	registry.Register(&requestCaptureProvider{})

	handler := NewChatHandler(store, registry, tools.NewRegistry())
	handler.SetSettingsHandler(settings)

	interceptor := &mediaInterceptorMock{isMedia: true}
	handler.SetMediaInterceptor(interceptor)

	patch := func(t *testing.T, body string) {
		t.Helper()
		e := echo.New()
		req := httptest.NewRequest(http.MethodPatch, "/api/settings", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		if err := settings.Patch(e.NewContext(req, rec)); err != nil {
			t.Fatalf("settings Patch() error = %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("settings Patch() status=%d, want 200", rec.Code)
		}
	}

	// Gate off => should not intercept.
	patch(t, `{"small_model_media_intent_enabled":false}`)
	out, err := handler.ProcessChannelMessage(context.Background(), channel.Message{
		ID:          "m1",
		ChannelName: "test",
		ChatID:      "c1",
		UserID:      "u1",
		Type:        channel.MessageTypeText,
		Content:     "generate an image of a cat",
		Metadata: map[string]interface{}{
			"language": "zh-CN",
		},
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("ProcessChannelMessage: %v", err)
	}
	if strings.Contains(out, i18n.T(i18n.ParseLanguage("zh-CN"), i18n.MsgMediaGenerating)) {
		t.Fatalf("expected no media interception when small_model_media_intent_enabled=false, got=%q", out)
	}

	// Gate on => should intercept and return the localized "generating" message.
	patch(t, `{"small_model_media_intent_enabled":true}`)
	out, err = handler.ProcessChannelMessage(context.Background(), channel.Message{
		ID:          "m2",
		ChannelName: "test",
		ChatID:      "c1",
		UserID:      "u1",
		Type:        channel.MessageTypeText,
		Content:     "generate an image of a cat",
		Metadata: map[string]interface{}{
			"language": "zh-CN",
		},
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("ProcessChannelMessage: %v", err)
	}
	want := i18n.T(i18n.ParseLanguage("zh-CN"), i18n.MsgMediaGenerating)
	if out != want {
		t.Fatalf("expected media generating message, got=%q want=%q", out, want)
	}
	if interceptor.calls < 1 {
		t.Fatalf("expected media interceptor to be called")
	}
}
