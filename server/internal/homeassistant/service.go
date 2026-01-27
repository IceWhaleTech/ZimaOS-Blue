package homeassistant

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// CommandResult represents the result of a natural language command.
type CommandResult struct {
	Success     bool                   `json:"success"`
	Message     string                 `json:"message"`
	EntityID    string                 `json:"entity_id,omitempty"`
	Action      string                 `json:"action,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Suggestions []string               `json:"suggestions,omitempty"`
}

// HAService provides high-level Home Assistant operations.
type HAService struct {
	client        Client
	config        *Config
	entityCache   map[string]*Entity
	cacheMu       sync.RWMutex
	cacheExpiry   time.Time
	cacheTTL      time.Duration
	stateCallback func(entityID string, oldState, newState *EntityState)
	callbackMu    sync.RWMutex
}

// NewHAService creates a new Home Assistant service.
func NewHAService() *HAService {
	return &HAService{
		entityCache: make(map[string]*Entity),
		cacheTTL:    5 * time.Minute,
	}
}

// Connect connects to Home Assistant.
func (s *HAService) Connect(ctx context.Context, config *Config) error {
	s.config = config
	s.client = NewClient(config)

	if err := s.client.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	// Subscribe to state changes
	if err := s.client.SubscribeStateChanges(ctx, func(entityID string, oldState, newState *EntityState) {
		// Update cache
		s.cacheMu.Lock()
		if entity, ok := s.entityCache[entityID]; ok && newState != nil {
			entity.State = newState
		}
		s.cacheMu.Unlock()

		// Forward to subscriber
		s.callbackMu.RLock()
		callback := s.stateCallback
		s.callbackMu.RUnlock()

		if callback != nil {
			callback(entityID, oldState, newState)
		}
	}); err != nil {
		// Non-fatal, just log
		fmt.Printf("Warning: failed to subscribe to state changes: %v\n", err)
	}

	return nil
}

// Disconnect disconnects from Home Assistant.
func (s *HAService) Disconnect() error {
	if s.client != nil {
		return s.client.Disconnect()
	}
	return nil
}

// IsConnected returns whether the service is connected.
func (s *HAService) IsConnected() bool {
	return s.client != nil && s.client.IsConnected()
}

// GetAllEntities returns all entities.
func (s *HAService) GetAllEntities(ctx context.Context) ([]*Entity, error) {
	// Check cache
	s.cacheMu.RLock()
	if time.Now().Before(s.cacheExpiry) && len(s.entityCache) > 0 {
		entities := make([]*Entity, 0, len(s.entityCache))
		for _, e := range s.entityCache {
			entities = append(entities, e)
		}
		s.cacheMu.RUnlock()
		return entities, nil
	}
	s.cacheMu.RUnlock()

	// Fetch from HA
	entities, err := s.client.GetEntities(ctx)
	if err != nil {
		return nil, err
	}

	// Update cache
	s.cacheMu.Lock()
	s.entityCache = make(map[string]*Entity)
	for _, e := range entities {
		s.entityCache[e.EntityID] = e
	}
	s.cacheExpiry = time.Now().Add(s.cacheTTL)
	s.cacheMu.Unlock()

	return entities, nil
}

// GetEntitiesByDomain returns entities filtered by domain.
func (s *HAService) GetEntitiesByDomain(ctx context.Context, domain string) ([]*Entity, error) {
	entities, err := s.GetAllEntities(ctx)
	if err != nil {
		return nil, err
	}

	var filtered []*Entity
	prefix := domain + "."
	for _, e := range entities {
		if strings.HasPrefix(e.EntityID, prefix) {
			filtered = append(filtered, e)
		}
	}

	return filtered, nil
}

// GetEntity returns a single entity.
func (s *HAService) GetEntity(ctx context.Context, entityID string) (*Entity, error) {
	// Check cache first
	s.cacheMu.RLock()
	if entity, ok := s.entityCache[entityID]; ok && time.Now().Before(s.cacheExpiry) {
		s.cacheMu.RUnlock()
		return entity, nil
	}
	s.cacheMu.RUnlock()

	// Fetch from HA
	return s.client.GetEntity(ctx, entityID)
}

// ControlEntity controls an entity with the specified action.
func (s *HAService) ControlEntity(ctx context.Context, entityID string, action string, params map[string]interface{}) error {
	domain := strings.Split(entityID, ".")[0]

	// Map common actions to services
	svc := action
	switch action {
	case "on", "turn_on":
		svc = "turn_on"
	case "off", "turn_off":
		svc = "turn_off"
	case "toggle":
		svc = "toggle"
	}

	// Build service data
	serviceData := make(map[string]interface{})
	for k, v := range params {
		serviceData[k] = v
	}

	return s.client.CallService(ctx, &ServiceCallRequest{
		Domain:      domain,
		Service:     svc,
		Target:      &ServiceTarget{EntityID: entityID},
		ServiceData: serviceData,
	})
}

// GetScenes returns all scenes.
func (s *HAService) GetScenes(ctx context.Context) ([]*Scene, error) {
	return s.client.GetScenes(ctx)
}

// ActivateScene activates a scene.
func (s *HAService) ActivateScene(ctx context.Context, sceneID string) error {
	return s.client.ActivateScene(ctx, sceneID)
}

// GetAutomations returns all automations.
func (s *HAService) GetAutomations(ctx context.Context) ([]*Automation, error) {
	return s.client.GetAutomations(ctx)
}

// TriggerAutomation triggers an automation.
func (s *HAService) TriggerAutomation(ctx context.Context, automationID string) error {
	return s.client.TriggerAutomation(ctx, automationID)
}

// ToggleAutomation enables or disables an automation.
func (s *HAService) ToggleAutomation(ctx context.Context, automationID string, enable bool) error {
	return s.client.ToggleAutomation(ctx, automationID, enable)
}

// ProcessCommand processes a natural language command.
func (s *HAService) ProcessCommand(ctx context.Context, command string) (*CommandResult, error) {
	command = strings.ToLower(strings.TrimSpace(command))

	// Parse the command
	result := &CommandResult{
		Success: false,
	}

	// Pattern matching for common commands
	patterns := []struct {
		regex   *regexp.Regexp
		handler func(matches []string) (*CommandResult, error)
	}{
		// Turn on/off patterns
		{
			regex: regexp.MustCompile(`(?i)turn\s+(on|off)\s+(?:the\s+)?(.+)`),
			handler: func(matches []string) (*CommandResult, error) {
				action := matches[1]
				target := matches[2]
				return s.handleOnOffCommand(ctx, action, target)
			},
		},
		// Set brightness
		{
			regex: regexp.MustCompile(`(?i)set\s+(?:the\s+)?(.+?)\s+(?:brightness\s+)?to\s+(\d+)(?:\s*%)?`),
			handler: func(matches []string) (*CommandResult, error) {
				target := matches[1]
				brightness := matches[2]
				return s.handleBrightnessCommand(ctx, target, brightness)
			},
		},
		// Set temperature
		{
			regex: regexp.MustCompile(`(?i)set\s+(?:the\s+)?(?:temperature|thermostat)\s+to\s+(\d+)`),
			handler: func(matches []string) (*CommandResult, error) {
				temp := matches[1]
				return s.handleTemperatureCommand(ctx, temp)
			},
		},
		// Activate scene
		{
			regex: regexp.MustCompile(`(?i)(?:activate|turn on|set)\s+(?:the\s+)?(.+?)\s+scene`),
			handler: func(matches []string) (*CommandResult, error) {
				sceneName := matches[1]
				return s.handleSceneCommand(ctx, sceneName)
			},
		},
		// Lock/unlock
		{
			regex: regexp.MustCompile(`(?i)(lock|unlock)\s+(?:the\s+)?(.+)`),
			handler: func(matches []string) (*CommandResult, error) {
				action := matches[1]
				target := matches[2]
				return s.handleLockCommand(ctx, action, target)
			},
		},
		// Open/close
		{
			regex: regexp.MustCompile(`(?i)(open|close)\s+(?:the\s+)?(.+)`),
			handler: func(matches []string) (*CommandResult, error) {
				action := matches[1]
				target := matches[2]
				return s.handleCoverCommand(ctx, action, target)
			},
		},
	}

	// Try each pattern
	for _, p := range patterns {
		if matches := p.regex.FindStringSubmatch(command); matches != nil {
			return p.handler(matches)
		}
	}

	// No pattern matched
	result.Message = "I didn't understand that command. Try something like 'turn on the living room light' or 'set temperature to 72'."
	result.Suggestions = []string{
		"Turn on/off [device name]",
		"Set [light] brightness to [0-100]%",
		"Set temperature to [degrees]",
		"Activate [scene name] scene",
		"Lock/unlock [door name]",
	}

	return result, nil
}

// handleOnOffCommand handles turn on/off commands.
func (s *HAService) handleOnOffCommand(ctx context.Context, action, target string) (*CommandResult, error) {
	entity, err := s.findEntityByName(ctx, target)
	if err != nil {
		return &CommandResult{
			Success: false,
			Message: fmt.Sprintf("Could not find device '%s'", target),
		}, nil
	}

	svc := "turn_on"
	if strings.ToLower(action) == "off" {
		svc = "turn_off"
	}

	if err := s.ControlEntity(ctx, entity.EntityID, svc, nil); err != nil {
		return &CommandResult{
			Success: false,
			Message: fmt.Sprintf("Failed to %s %s: %v", action, target, err),
		}, nil
	}

	return &CommandResult{
		Success:  true,
		Message:  fmt.Sprintf("Turned %s %s", action, entity.Name),
		EntityID: entity.EntityID,
		Action:   svc,
	}, nil
}

// handleBrightnessCommand handles brightness commands.
func (s *HAService) handleBrightnessCommand(ctx context.Context, target, brightness string) (*CommandResult, error) {
	entity, err := s.findEntityByName(ctx, target)
	if err != nil {
		return &CommandResult{
			Success: false,
			Message: fmt.Sprintf("Could not find light '%s'", target),
		}, nil
	}

	// Convert percentage to 0-255 range
	var brightnessInt int
	fmt.Sscanf(brightness, "%d", &brightnessInt)
	brightnessValue := int(float64(brightnessInt) / 100.0 * 255.0)

	params := map[string]interface{}{
		"brightness": brightnessValue,
	}

	if err := s.ControlEntity(ctx, entity.EntityID, "turn_on", params); err != nil {
		return &CommandResult{
			Success: false,
			Message: fmt.Sprintf("Failed to set brightness: %v", err),
		}, nil
	}

	return &CommandResult{
		Success:    true,
		Message:    fmt.Sprintf("Set %s brightness to %s%%", entity.Name, brightness),
		EntityID:   entity.EntityID,
		Action:     "turn_on",
		Parameters: params,
	}, nil
}

// handleTemperatureCommand handles temperature commands.
func (s *HAService) handleTemperatureCommand(ctx context.Context, temp string) (*CommandResult, error) {
	// Find climate entities
	entities, err := s.GetEntitiesByDomain(ctx, "climate")
	if err != nil || len(entities) == 0 {
		return &CommandResult{
			Success: false,
			Message: "No thermostat found",
		}, nil
	}

	// Use the first climate entity
	entity := entities[0]

	var tempValue float64
	fmt.Sscanf(temp, "%f", &tempValue)

	params := map[string]interface{}{
		"temperature": tempValue,
	}

	if err := s.client.CallService(ctx, &ServiceCallRequest{
		Domain:  "climate",
		Service: "set_temperature",
		Target:  &ServiceTarget{EntityID: entity.EntityID},
		ServiceData: map[string]interface{}{
			"temperature": tempValue,
		},
	}); err != nil {
		return &CommandResult{
			Success: false,
			Message: fmt.Sprintf("Failed to set temperature: %v", err),
		}, nil
	}

	return &CommandResult{
		Success:    true,
		Message:    fmt.Sprintf("Set temperature to %s°", temp),
		EntityID:   entity.EntityID,
		Action:     "set_temperature",
		Parameters: params,
	}, nil
}

// handleSceneCommand handles scene activation commands.
func (s *HAService) handleSceneCommand(ctx context.Context, sceneName string) (*CommandResult, error) {
	scenes, err := s.GetScenes(ctx)
	if err != nil {
		return &CommandResult{
			Success: false,
			Message: "Failed to get scenes",
		}, nil
	}

	// Find matching scene
	sceneName = strings.ToLower(sceneName)
	for _, scene := range scenes {
		if strings.Contains(strings.ToLower(scene.Name), sceneName) {
			if err := s.ActivateScene(ctx, scene.EntityID); err != nil {
				return &CommandResult{
					Success: false,
					Message: fmt.Sprintf("Failed to activate scene: %v", err),
				}, nil
			}

			return &CommandResult{
				Success:  true,
				Message:  fmt.Sprintf("Activated %s scene", scene.Name),
				EntityID: scene.EntityID,
				Action:   "activate",
			}, nil
		}
	}

	return &CommandResult{
		Success: false,
		Message: fmt.Sprintf("Could not find scene '%s'", sceneName),
	}, nil
}

// handleLockCommand handles lock/unlock commands.
func (s *HAService) handleLockCommand(ctx context.Context, action, target string) (*CommandResult, error) {
	entity, err := s.findEntityByName(ctx, target)
	if err != nil {
		// Try finding in lock domain
		entities, err := s.GetEntitiesByDomain(ctx, "lock")
		if err != nil || len(entities) == 0 {
			return &CommandResult{
				Success: false,
				Message: fmt.Sprintf("Could not find lock '%s'", target),
			}, nil
		}

		// Find by name
		target = strings.ToLower(target)
		for _, e := range entities {
			if strings.Contains(strings.ToLower(e.Name), target) {
				entity = e
				break
			}
		}

		if entity == nil {
			return &CommandResult{
				Success: false,
				Message: fmt.Sprintf("Could not find lock '%s'", target),
			}, nil
		}
	}

	if err := s.client.CallService(ctx, &ServiceCallRequest{
		Domain:  "lock",
		Service: action,
		Target:  &ServiceTarget{EntityID: entity.EntityID},
	}); err != nil {
		return &CommandResult{
			Success: false,
			Message: fmt.Sprintf("Failed to %s: %v", action, err),
		}, nil
	}

	return &CommandResult{
		Success:  true,
		Message:  fmt.Sprintf("%sed %s", strings.Title(action), entity.Name),
		EntityID: entity.EntityID,
		Action:   action,
	}, nil
}

// handleCoverCommand handles open/close commands for covers.
func (s *HAService) handleCoverCommand(ctx context.Context, action, target string) (*CommandResult, error) {
	entity, err := s.findEntityByName(ctx, target)
	if err != nil {
		return &CommandResult{
			Success: false,
			Message: fmt.Sprintf("Could not find '%s'", target),
		}, nil
	}

	svc := "open_cover"
	if strings.ToLower(action) == "close" {
		svc = "close_cover"
	}

	if err := s.client.CallService(ctx, &ServiceCallRequest{
		Domain:  "cover",
		Service: svc,
		Target:  &ServiceTarget{EntityID: entity.EntityID},
	}); err != nil {
		return &CommandResult{
			Success: false,
			Message: fmt.Sprintf("Failed to %s: %v", action, err),
		}, nil
	}

	return &CommandResult{
		Success:  true,
		Message:  fmt.Sprintf("%sed %s", strings.Title(action), entity.Name),
		EntityID: entity.EntityID,
		Action:   svc,
	}, nil
}

// findEntityByName finds an entity by its friendly name or entity ID.
func (s *HAService) findEntityByName(ctx context.Context, name string) (*Entity, error) {
	entities, err := s.GetAllEntities(ctx)
	if err != nil {
		return nil, err
	}

	name = strings.ToLower(name)

	// First try exact match on entity ID
	for _, e := range entities {
		if strings.ToLower(e.EntityID) == name {
			return e, nil
		}
	}

	// Then try friendly name match
	for _, e := range entities {
		friendlyName := strings.ToLower(e.Name)
		if friendlyName == name || strings.Contains(friendlyName, name) {
			return e, nil
		}
	}

	return nil, fmt.Errorf("entity not found: %s", name)
}

// SubscribeStateChanges subscribes to state change events.
func (s *HAService) SubscribeStateChanges(callback func(entityID string, oldState, newState *EntityState)) {
	s.callbackMu.Lock()
	s.stateCallback = callback
	s.callbackMu.Unlock()
}

// UnsubscribeStateChanges unsubscribes from state change events.
func (s *HAService) UnsubscribeStateChanges() {
	s.callbackMu.Lock()
	s.stateCallback = nil
	s.callbackMu.Unlock()
}
