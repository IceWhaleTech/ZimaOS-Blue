package homeassistant

import (
	"context"
	"testing"
	"time"
)

// mockClient is a mock implementation of the Client interface for testing.
type mockClient struct {
	connected   bool
	entities    []*Entity
	scenes      []*Scene
	automations []*Automation
	callErr     error
}

func (m *mockClient) Connect(ctx context.Context) error {
	m.connected = true
	return nil
}

func (m *mockClient) Disconnect() error {
	m.connected = false
	return nil
}

func (m *mockClient) IsConnected() bool {
	return m.connected
}

func (m *mockClient) GetConfig(ctx context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{"version": "2024.1.0"}, nil
}

func (m *mockClient) GetEntities(ctx context.Context) ([]*Entity, error) {
	return m.entities, nil
}

func (m *mockClient) GetEntity(ctx context.Context, entityID string) (*Entity, error) {
	for _, e := range m.entities {
		if e.EntityID == entityID {
			return e, nil
		}
	}
	return nil, ErrEntityNotFound
}

func (m *mockClient) GetEntityState(ctx context.Context, entityID string) (*EntityState, error) {
	for _, e := range m.entities {
		if e.EntityID == entityID && e.State != nil {
			return e.State, nil
		}
	}
	return nil, ErrEntityNotFound
}

func (m *mockClient) GetEntitiesByDomain(ctx context.Context, domain string) ([]*Entity, error) {
	var result []*Entity
	for _, e := range m.entities {
		if e.Domain == domain {
			result = append(result, e)
		}
	}
	return result, nil
}

func (m *mockClient) GetServices(ctx context.Context) ([]*HAServiceInfo, error) {
	return nil, nil
}

func (m *mockClient) CallService(ctx context.Context, req *ServiceCallRequest) error {
	return m.callErr
}

func (m *mockClient) GetScenes(ctx context.Context) ([]*Scene, error) {
	return m.scenes, nil
}

func (m *mockClient) ActivateScene(ctx context.Context, sceneID string) error {
	return m.callErr
}

func (m *mockClient) GetAutomations(ctx context.Context) ([]*Automation, error) {
	return m.automations, nil
}

func (m *mockClient) TriggerAutomation(ctx context.Context, automationID string) error {
	return m.callErr
}

func (m *mockClient) ToggleAutomation(ctx context.Context, automationID string, enable bool) error {
	return m.callErr
}

func (m *mockClient) SubscribeEvents(ctx context.Context, eventType string, callback EventCallback) error {
	return nil
}

func (m *mockClient) SubscribeStateChanges(ctx context.Context, callback StateChangeCallback) error {
	return nil
}

func (m *mockClient) FireEvent(ctx context.Context, eventType string, data map[string]interface{}) error {
	return nil
}

func TestHAService_EntityFilter(t *testing.T) {
	service := NewHAService()
	service.client = &mockClient{
		connected: true,
		entities: []*Entity{
			{EntityID: "light.living_room", Name: "Living Room Light", Domain: "light"},
			{EntityID: "light.bedroom", Name: "Bedroom Light", Domain: "light"},
			{EntityID: "switch.fan", Name: "Fan", Domain: "switch"},
			{EntityID: "sensor.temperature", Name: "Temperature", Domain: "sensor"},
			{EntityID: "climate.thermostat", Name: "Thermostat", Domain: "climate"},
		},
	}
	service.config = &Config{}

	ctx := context.Background()

	t.Run("no filter returns all entities", func(t *testing.T) {
		service.SetEntityFilter(nil)
		entities, err := service.GetAllEntities(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entities) != 5 {
			t.Errorf("expected 5 entities, got %d", len(entities))
		}
	})

	t.Run("include domains filter", func(t *testing.T) {
		service.SetEntityFilter(&EntityFilter{
			IncludeDomains: []string{"light"},
		})
		entities, err := service.GetAllEntities(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entities) != 2 {
			t.Errorf("expected 2 light entities, got %d", len(entities))
		}
		for _, e := range entities {
			if e.Domain != "light" {
				t.Errorf("expected domain 'light', got '%s'", e.Domain)
			}
		}
	})

	t.Run("exclude domains filter", func(t *testing.T) {
		service.SetEntityFilter(&EntityFilter{
			ExcludeDomains: []string{"sensor"},
		})
		entities, err := service.GetAllEntities(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entities) != 4 {
			t.Errorf("expected 4 entities (excluding sensor), got %d", len(entities))
		}
		for _, e := range entities {
			if e.Domain == "sensor" {
				t.Error("sensor domain should be excluded")
			}
		}
	})

	t.Run("include specific entities", func(t *testing.T) {
		service.SetEntityFilter(&EntityFilter{
			IncludeEntities: []string{"light.living_room", "switch.fan"},
		})
		entities, err := service.GetAllEntities(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entities) != 2 {
			t.Errorf("expected 2 entities, got %d", len(entities))
		}
	})

	t.Run("exclude specific entities", func(t *testing.T) {
		service.SetEntityFilter(&EntityFilter{
			ExcludeEntities: []string{"light.bedroom"},
		})
		entities, err := service.GetAllEntities(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entities) != 4 {
			t.Errorf("expected 4 entities, got %d", len(entities))
		}
		for _, e := range entities {
			if e.EntityID == "light.bedroom" {
				t.Error("light.bedroom should be excluded")
			}
		}
	})

	t.Run("exclude takes priority over include", func(t *testing.T) {
		service.SetEntityFilter(&EntityFilter{
			IncludeDomains:  []string{"light"},
			ExcludeEntities: []string{"light.bedroom"},
		})
		entities, err := service.GetAllEntities(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entities) != 1 {
			t.Errorf("expected 1 entity, got %d", len(entities))
		}
		if entities[0].EntityID != "light.living_room" {
			t.Errorf("expected light.living_room, got %s", entities[0].EntityID)
		}
	})
}

func TestHAService_PollInterval(t *testing.T) {
	service := NewHAService()

	t.Run("default poll interval", func(t *testing.T) {
		interval := service.GetPollInterval()
		if interval != 5*time.Minute {
			t.Errorf("expected default 5 minutes, got %v", interval)
		}
	})

	t.Run("set poll interval", func(t *testing.T) {
		service.SetPollInterval(30 * time.Second)
		interval := service.GetPollInterval()
		if interval != 30*time.Second {
			t.Errorf("expected 30 seconds, got %v", interval)
		}
	})
}

func TestHAService_ProcessCommand(t *testing.T) {
	service := NewHAService()
	service.client = &mockClient{
		connected: true,
		entities: []*Entity{
			{EntityID: "light.living_room", Name: "Living Room Light", Domain: "light"},
			{EntityID: "climate.thermostat", Name: "Thermostat", Domain: "climate"},
		},
		scenes: []*Scene{
			{EntityID: "scene.movie_night", Name: "Movie Night"},
		},
	}
	service.config = &Config{}

	ctx := context.Background()

	t.Run("turn on command", func(t *testing.T) {
		result, err := service.ProcessCommand(ctx, "turn on the living room light")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Success {
			t.Errorf("expected success, got failure: %s", result.Message)
		}
		if result.EntityID != "light.living_room" {
			t.Errorf("expected entity light.living_room, got %s", result.EntityID)
		}
	})

	t.Run("turn off command", func(t *testing.T) {
		result, err := service.ProcessCommand(ctx, "turn off living room")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Success {
			t.Errorf("expected success, got failure: %s", result.Message)
		}
	})

	t.Run("set temperature command", func(t *testing.T) {
		result, err := service.ProcessCommand(ctx, "set the thermostat to 72")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Success {
			t.Errorf("expected success, got failure: %s", result.Message)
		}
	})

	t.Run("activate scene command", func(t *testing.T) {
		result, err := service.ProcessCommand(ctx, "activate movie night scene")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Success {
			t.Errorf("expected success, got failure: %s", result.Message)
		}
	})

	t.Run("unknown command", func(t *testing.T) {
		result, err := service.ProcessCommand(ctx, "do something random")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Success {
			t.Error("expected failure for unknown command")
		}
		if len(result.Suggestions) == 0 {
			t.Error("expected suggestions for unknown command")
		}
	})

	t.Run("device not found", func(t *testing.T) {
		result, err := service.ProcessCommand(ctx, "turn on the kitchen light")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Success {
			t.Error("expected failure for non-existent device")
		}
	})
}

func TestHAService_ControlEntity(t *testing.T) {
	service := NewHAService()
	service.client = &mockClient{
		connected: true,
		entities: []*Entity{
			{EntityID: "light.test", Name: "Test Light", Domain: "light"},
		},
	}
	service.config = &Config{}

	ctx := context.Background()

	t.Run("turn on", func(t *testing.T) {
		err := service.ControlEntity(ctx, "light.test", "on", nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("turn off", func(t *testing.T) {
		err := service.ControlEntity(ctx, "light.test", "off", nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("toggle", func(t *testing.T) {
		err := service.ControlEntity(ctx, "light.test", "toggle", nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("with parameters", func(t *testing.T) {
		params := map[string]interface{}{
			"brightness": 128,
		}
		err := service.ControlEntity(ctx, "light.test", "turn_on", params)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestHAService_GetEntitiesByDomain(t *testing.T) {
	service := NewHAService()
	service.client = &mockClient{
		connected: true,
		entities: []*Entity{
			{EntityID: "light.one", Name: "Light One", Domain: "light"},
			{EntityID: "light.two", Name: "Light Two", Domain: "light"},
			{EntityID: "switch.one", Name: "Switch One", Domain: "switch"},
		},
	}
	service.config = &Config{}

	ctx := context.Background()

	t.Run("get lights", func(t *testing.T) {
		entities, err := service.GetEntitiesByDomain(ctx, "light")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entities) != 2 {
			t.Errorf("expected 2 lights, got %d", len(entities))
		}
	})

	t.Run("get switches", func(t *testing.T) {
		entities, err := service.GetEntitiesByDomain(ctx, "switch")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entities) != 1 {
			t.Errorf("expected 1 switch, got %d", len(entities))
		}
	})

	t.Run("get non-existent domain", func(t *testing.T) {
		entities, err := service.GetEntitiesByDomain(ctx, "camera")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entities) != 0 {
			t.Errorf("expected 0 cameras, got %d", len(entities))
		}
	})
}
