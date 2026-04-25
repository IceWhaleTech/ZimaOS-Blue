package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxybridge"
)

func TestLazyRodBrowserAdapterDoesNotCacheResolvedService(t *testing.T) {
	svcs := []*browser.RodService{{}, {}}
	resolveCalls := 0
	adapter := NewLazyRodBrowserAdapter(func() *browser.RodService {
		defer func() { resolveCalls++ }()
		return svcs[resolveCalls]
	})

	first, err := adapter.get()
	if err != nil {
		t.Fatalf("get() first error = %v", err)
	}
	if first != svcs[0] {
		t.Fatalf("get() first = %p, want %p", first, svcs[0])
	}

	second, err := adapter.get()
	if err != nil {
		t.Fatalf("get() second error = %v", err)
	}
	if second != svcs[1] {
		t.Fatalf("get() second = %p, want %p", second, svcs[1])
	}
}

func TestLeaseAwareRodBrowserAdapterReleasesService(t *testing.T) {
	svc := &browser.RodService{}
	releases := 0
	adapter := NewLeaseAwareRodBrowserAdapter(func() (*browser.RodService, func(), error) {
		return svc, func() { releases++ }, nil
	})

	lease, err := adapter.acquire()
	if err != nil {
		t.Fatalf("acquire() error = %v", err)
	}
	if lease.svc != svc {
		t.Fatalf("acquire() service = %p, want %p", lease.svc, svc)
	}
	if releases != 0 {
		t.Fatalf("release count before close = %d, want 0", releases)
	}

	lease.close()
	if releases != 1 {
		t.Fatalf("release count after close = %d, want 1", releases)
	}
}

func TestProxyBridgeLLMAdapterAttachesScreenshotPathAsVisionPart(t *testing.T) {
	screenshotPath := writeCUATestPNG(t, 4, 3)
	var captured map[string]interface{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-1","choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	})
	adapter := NewProxyBridgeLLMAdapter(proxybridge.NewBridge(handler))

	text, err := adapter.Chat(context.Background(), "Screenshot: "+screenshotPath, 100)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if text != "ok" {
		t.Fatalf("Chat() = %q, want ok", text)
	}
	messages, _ := captured["messages"].([]interface{})
	if len(messages) != 1 {
		t.Fatalf("captured messages = %#v", captured["messages"])
	}
	message, _ := messages[0].(map[string]interface{})
	parts, ok := message["content"].([]interface{})
	if !ok || len(parts) != 2 {
		t.Fatalf("message content = %#v, want text+image parts", message["content"])
	}
	if first, _ := parts[0].(map[string]interface{}); first["type"] != "text" || !strings.Contains(first["text"].(string), screenshotPath) {
		t.Fatalf("first content part = %#v, want text with screenshot path", parts[0])
	}
	second, _ := parts[1].(map[string]interface{})
	imageURL, _ := second["image_url"].(map[string]interface{})
	if second["type"] != "image_url" || !strings.HasPrefix(imageURL["url"].(string), "data:image/png;base64,") {
		t.Fatalf("second content part = %#v, want png image_url", parts[1])
	}
}
