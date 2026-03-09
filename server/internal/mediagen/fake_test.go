package mediagen

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestManager_DisableUsesConfigIDForRegisteredProvider(t *testing.T) {
	store := &memoryConfigStore{saved: map[string]*configOnDisk{
		"gemini-image": {ID: "gemini-image", Enabled: true, APIKey: "test-key"},
	}}
	m := NewManager(nil, store, "")
	m.InitConfigs()

	if !m.HasActiveProviders() {
		t.Fatal("expected provider to be active after init")
	}
	if err := m.Disable("gemini-image"); err != nil {
		t.Fatalf("Disable returned error: %v", err)
	}
	if m.HasActiveProviders() {
		t.Fatal("expected no active providers after disable")
	}
}

func TestManager_ModelConflictsRespectConfigPriority(t *testing.T) {
	store := &memoryConfigStore{saved: map[string]*configOnDisk{
		"dashscope-image": {ID: "dashscope-image", Enabled: true, APIKey: "dash-key"},
		"mulerouter":      {ID: "mulerouter", Enabled: true, APIKey: "mule-key"},
	}}
	m := NewManager(nil, store, "")
	m.InitConfigs()

	provider, err := m.findProvider("qwen-image-max")
	if err != nil {
		t.Fatalf("findProvider returned error: %v", err)
	}
	if provider.Name() != "dashscope" {
		t.Fatalf("provider = %q, want %q", provider.Name(), "dashscope")
	}
}

func TestFakeMediaProviderGenerateCachesLocalURLs(t *testing.T) {
	t.Setenv(fakeMediaProviderEnv, "1")
	tmp := t.TempDir()
	storage := NewMediaStorage(tmp, "/api/media/generated")
	if err := storage.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	m := NewManager(storage, nil, "")
	m.InitConfigs()

	cfg := m.GetConfig(fakeMediaProviderID)
	if cfg == nil {
		t.Fatalf("expected %s config to exist", fakeMediaProviderID)
	}
	if !cfg.HasAPIKey {
		t.Fatal("expected fake provider to report configured credential state")
	}
	provider := createProvider(cfg)
	if _, ok := provider.(*FakeMediaProvider); !ok {
		t.Fatalf("createProvider() type = %T, want *FakeMediaProvider", provider)
	}
	if err := m.Enable(fakeMediaProviderID); err != nil {
		t.Fatalf("Enable returned error: %v", err)
	}

	task, err := m.Generate(context.Background(), &MediaRequest{
		Type:   MediaTypeImage,
		Model:  fakeMediaModelID,
		Prompt: "draw a cat",
		N:      1,
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	result, err := m.WaitForTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("WaitForTask returned error: %v", err)
	}
	if result.Status != TaskStatusSucceeded {
		t.Fatalf("status = %q, want %q", result.Status, TaskStatusSucceeded)
	}
	if result.Response == nil || len(result.Response.Data) != 1 {
		t.Fatalf("response = %#v, want single result", result.Response)
	}
	got := result.Response.Data[0]
	if !strings.HasPrefix(got.URL, "/api/media/generated/images/") {
		t.Fatalf("url = %q, want local generated path", got.URL)
	}
	if !strings.HasPrefix(got.ThumbnailURL, "/api/media/generated/thumbnails/") {
		t.Fatalf("thumbnail_url = %q, want local thumbnail path", got.ThumbnailURL)
	}
	if got.B64JSON != "" {
		t.Fatal("expected base64 payload to be cleared after caching")
	}
}

func TestTestMediaProviderFakeIsHealthy(t *testing.T) {
	t.Setenv(fakeMediaProviderEnv, "1")
	cfg := &MediaProviderConfig{ID: fakeMediaProviderID}
	result := TestMediaProvider(cfg)
	if !result.Healthy {
		t.Fatalf("healthy = %v, want true (err=%q)", result.Healthy, result.Error)
	}
	if result.Error != "" {
		t.Fatalf("error = %q, want empty", result.Error)
	}
}

type memoryConfigStore struct {
	saved map[string]*configOnDisk
}

func (s *memoryConfigStore) Load() (map[string]*configOnDisk, error) {
	if s.saved == nil {
		return nil, nil
	}
	result := make(map[string]*configOnDisk, len(s.saved))
	for k, v := range s.saved {
		copy := *v
		result[k] = &copy
	}
	return result, nil
}

func (s *memoryConfigStore) Save(configs map[string]*MediaProviderConfig) error {
	s.saved = make(map[string]*configOnDisk, len(configs))
	for id, cfg := range configs {
		s.saved[id] = &configOnDisk{
			ID:      cfg.ID,
			Enabled: cfg.Enabled,
			BaseURL: cfg.BaseURL,
			APIKey:  cfg.APIKey,
		}
	}
	return nil
}

func (s *memoryConfigStore) Dir() string { return "" }
