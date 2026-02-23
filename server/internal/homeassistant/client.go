package homeassistant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// client implements the Client interface.
type client struct {
	config     *Config
	httpClient *http.Client
	wsConn     *websocket.Conn
	connected  bool
	msgID      int
	mu         sync.RWMutex

	// Callbacks
	eventCallbacks       map[string][]EventCallback
	stateChangeCallbacks []StateChangeCallback
	callbackMu           sync.RWMutex

	// State cache
	entities   map[string]*Entity
	entitiesMu sync.RWMutex
}

// NewClient creates a new Home Assistant client.
func NewClient(config *Config) Client {
	if config.PollInterval == 0 {
		config.PollInterval = 30 * time.Second
	}

	return &client{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		eventCallbacks: make(map[string][]EventCallback),
		entities:       make(map[string]*Entity),
	}
}

// Connect connects to Home Assistant.
func (c *client) Connect(ctx context.Context) error {
	// Test connection with API call
	_, err := c.GetConfig(ctx)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrConnectionFailed, err)
	}

	// Connect WebSocket for real-time updates
	if err := c.connectWebSocket(ctx); err != nil {
		// WebSocket is optional, log but don't fail
		fmt.Printf("WebSocket connection failed (optional): %v\n", err)
	}

	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()

	// Initial entity fetch
	go c.refreshEntities(ctx)

	return nil
}

// connectWebSocket establishes a WebSocket connection.
func (c *client) connectWebSocket(ctx context.Context) error {
	wsURL := strings.Replace(c.config.URL, "http", "ws", 1) + "/api/websocket"

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.wsConn = conn
	c.mu.Unlock()

	// Authenticate
	if err := c.authenticateWebSocket(); err != nil {
		conn.Close()
		return err
	}

	// Start message handler
	go c.handleWebSocketMessages()

	return nil
}

// authenticateWebSocket authenticates the WebSocket connection.
func (c *client) authenticateWebSocket() error {
	// Read auth_required message
	var authRequired struct {
		Type string `json:"type"`
	}
	if err := c.wsConn.ReadJSON(&authRequired); err != nil {
		return err
	}
	if authRequired.Type != "auth_required" {
		return fmt.Errorf("unexpected message type: %s", authRequired.Type)
	}

	// Send auth message
	authMsg := map[string]string{
		"type":         "auth",
		"access_token": c.config.Token,
	}
	if err := c.wsConn.WriteJSON(authMsg); err != nil {
		return err
	}

	// Read auth result
	var authResult struct {
		Type    string `json:"type"`
		Message string `json:"message,omitempty"`
	}
	if err := c.wsConn.ReadJSON(&authResult); err != nil {
		return err
	}
	if authResult.Type != "auth_ok" {
		return fmt.Errorf("%w: %s", ErrInvalidToken, authResult.Message)
	}

	return nil
}

// handleWebSocketMessages handles incoming WebSocket messages.
func (c *client) handleWebSocketMessages() {
	for {
		c.mu.RLock()
		conn := c.wsConn
		c.mu.RUnlock()

		if conn == nil {
			return
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			c.mu.Lock()
			c.wsConn = nil
			c.mu.Unlock()
			return
		}

		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		msgType, _ := msg["type"].(string)
		switch msgType {
		case "event":
			c.handleEventMessage(msg)
		case "result":
			// Handle command results if needed
		}
	}
}

// handleEventMessage handles event messages from WebSocket.
func (c *client) handleEventMessage(msg map[string]interface{}) {
	eventData, ok := msg["event"].(map[string]interface{})
	if !ok {
		return
	}

	eventType, _ := eventData["event_type"].(string)

	// Handle state_changed events
	if eventType == "state_changed" {
		c.handleStateChanged(eventData)
	}

	// Call registered callbacks
	c.callbackMu.RLock()
	callbacks := c.eventCallbacks[eventType]
	c.callbackMu.RUnlock()

	if len(callbacks) > 0 {
		event := &Event{
			EventType: eventType,
			TimeFired: timeutil.NowTime(),
		}
		if data, ok := eventData["data"].(map[string]interface{}); ok {
			event.Data = data
		}

		for _, cb := range callbacks {
			go cb(event)
		}
	}
}

// handleStateChanged handles state_changed events.
func (c *client) handleStateChanged(eventData map[string]interface{}) {
	data, ok := eventData["data"].(map[string]interface{})
	if !ok {
		return
	}

	entityID, _ := data["entity_id"].(string)
	if entityID == "" {
		return
	}

	var oldState, newState *EntityState
	if old, ok := data["old_state"].(map[string]interface{}); ok {
		oldState = parseEntityState(old)
	}
	if new, ok := data["new_state"].(map[string]interface{}); ok {
		newState = parseEntityState(new)
	}

	// Update cache
	c.entitiesMu.Lock()
	if entity, exists := c.entities[entityID]; exists && newState != nil {
		entity.State = newState
	}
	c.entitiesMu.Unlock()

	// Call callbacks
	c.callbackMu.RLock()
	callbacks := c.stateChangeCallbacks
	c.callbackMu.RUnlock()

	for _, cb := range callbacks {
		go cb(entityID, oldState, newState)
	}
}

// parseEntityState parses a state from a map.
func parseEntityState(data map[string]interface{}) *EntityState {
	state := &EntityState{
		EntityID:   data["entity_id"].(string),
		State:      data["state"].(string),
		Attributes: make(map[string]interface{}),
	}

	if attrs, ok := data["attributes"].(map[string]interface{}); ok {
		state.Attributes = attrs
	}

	if lastChanged, ok := data["last_changed"].(string); ok {
		state.LastChanged, _ = time.Parse(time.RFC3339, lastChanged)
	}
	if lastUpdated, ok := data["last_updated"].(string); ok {
		state.LastUpdated, _ = time.Parse(time.RFC3339, lastUpdated)
	}

	return state
}

// Disconnect disconnects from Home Assistant.
func (c *client) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.connected = false
	if c.wsConn != nil {
		c.wsConn.Close()
		c.wsConn = nil
	}
	return nil
}

// IsConnected returns whether the client is connected.
func (c *client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// doRequest performs an HTTP request to the Home Assistant API.
func (c *client) doRequest(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	url := c.config.URL + path

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.config.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrInvalidToken
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// GetConfig gets the Home Assistant configuration.
func (c *client) GetConfig(ctx context.Context) (map[string]interface{}, error) {
	body, err := c.doRequest(ctx, "GET", "/api/config", nil)
	if err != nil {
		return nil, err
	}

	var config map[string]interface{}
	if err := json.Unmarshal(body, &config); err != nil {
		return nil, err
	}

	return config, nil
}

// GetEntities gets all entities.
func (c *client) GetEntities(ctx context.Context) ([]*Entity, error) {
	body, err := c.doRequest(ctx, "GET", "/api/states", nil)
	if err != nil {
		return nil, err
	}

	var states []EntityState
	if err := json.Unmarshal(body, &states); err != nil {
		return nil, err
	}

	entities := make([]*Entity, 0, len(states))
	for _, state := range states {
		if c.shouldIncludeEntity(state.EntityID) {
			entity := c.stateToEntity(&state)
			entities = append(entities, entity)

			// Update cache
			c.entitiesMu.Lock()
			c.entities[entity.EntityID] = entity
			c.entitiesMu.Unlock()
		}
	}

	return entities, nil
}

// stateToEntity converts an EntityState to an Entity.
func (c *client) stateToEntity(state *EntityState) *Entity {
	parts := strings.SplitN(state.EntityID, ".", 2)
	domain := ""
	if len(parts) > 0 {
		domain = parts[0]
	}

	name := state.EntityID
	if friendlyName, ok := state.Attributes["friendly_name"].(string); ok {
		name = friendlyName
	}

	entity := &Entity{
		EntityID: state.EntityID,
		Name:     name,
		Domain:   domain,
		State:    state,
	}

	if deviceClass, ok := state.Attributes["device_class"].(string); ok {
		entity.DeviceClass = deviceClass
	}
	if unit, ok := state.Attributes["unit_of_measurement"].(string); ok {
		entity.UnitOfMeasurement = unit
	}
	if icon, ok := state.Attributes["icon"].(string); ok {
		entity.Icon = icon
	}

	return entity
}

// shouldIncludeEntity checks if an entity should be included based on filters.
func (c *client) shouldIncludeEntity(entityID string) bool {
	if c.config.EntityFilter == nil {
		return true
	}

	parts := strings.SplitN(entityID, ".", 2)
	domain := ""
	if len(parts) > 0 {
		domain = parts[0]
	}

	filter := c.config.EntityFilter

	// Check exclude entities
	for _, e := range filter.ExcludeEntities {
		if e == entityID {
			return false
		}
	}

	// Check exclude domains
	for _, d := range filter.ExcludeDomains {
		if d == domain {
			return false
		}
	}

	// If include lists are specified, entity must be in them
	if len(filter.IncludeEntities) > 0 || len(filter.IncludeDomains) > 0 {
		for _, e := range filter.IncludeEntities {
			if e == entityID {
				return true
			}
		}
		for _, d := range filter.IncludeDomains {
			if d == domain {
				return true
			}
		}
		return false
	}

	return true
}

// GetEntity gets a single entity.
func (c *client) GetEntity(ctx context.Context, entityID string) (*Entity, error) {
	state, err := c.GetEntityState(ctx, entityID)
	if err != nil {
		return nil, err
	}
	return c.stateToEntity(state), nil
}

// GetEntityState gets the state of an entity.
func (c *client) GetEntityState(ctx context.Context, entityID string) (*EntityState, error) {
	body, err := c.doRequest(ctx, "GET", "/api/states/"+entityID, nil)
	if err != nil {
		return nil, err
	}

	var state EntityState
	if err := json.Unmarshal(body, &state); err != nil {
		return nil, err
	}

	if state.EntityID == "" {
		return nil, ErrEntityNotFound
	}

	return &state, nil
}

// GetEntitiesByDomain gets entities by domain.
func (c *client) GetEntitiesByDomain(ctx context.Context, domain string) ([]*Entity, error) {
	entities, err := c.GetEntities(ctx)
	if err != nil {
		return nil, err
	}

	filtered := make([]*Entity, 0)
	for _, e := range entities {
		if e.Domain == domain {
			filtered = append(filtered, e)
		}
	}

	return filtered, nil
}

// GetServices gets all available services.
func (c *client) GetServices(ctx context.Context) ([]*HAServiceInfo, error) {
	body, err := c.doRequest(ctx, "GET", "/api/services", nil)
	if err != nil {
		return nil, err
	}

	var servicesMap []map[string]interface{}
	if err := json.Unmarshal(body, &servicesMap); err != nil {
		return nil, err
	}

	var services []*HAServiceInfo
	for _, domainServices := range servicesMap {
		domain, _ := domainServices["domain"].(string)
		if servicesList, ok := domainServices["services"].(map[string]interface{}); ok {
			for serviceName, serviceData := range servicesList {
				service := &HAServiceInfo{
					Domain:  domain,
					Service: serviceName,
				}
				if data, ok := serviceData.(map[string]interface{}); ok {
					if name, ok := data["name"].(string); ok {
						service.Name = name
					}
					if desc, ok := data["description"].(string); ok {
						service.Description = desc
					}
				}
				services = append(services, service)
			}
		}
	}

	return services, nil
}

// CallService calls a Home Assistant service.
func (c *client) CallService(ctx context.Context, req *ServiceCallRequest) error {
	path := fmt.Sprintf("/api/services/%s/%s", req.Domain, req.Service)

	data := make(map[string]interface{})
	if req.Target != nil {
		if req.Target.EntityID != nil {
			data["entity_id"] = req.Target.EntityID
		}
		if req.Target.DeviceID != nil {
			data["device_id"] = req.Target.DeviceID
		}
		if req.Target.AreaID != nil {
			data["area_id"] = req.Target.AreaID
		}
	}
	for k, v := range req.ServiceData {
		data[k] = v
	}

	_, err := c.doRequest(ctx, "POST", path, data)
	return err
}

// GetScenes gets all scenes.
func (c *client) GetScenes(ctx context.Context) ([]*Scene, error) {
	entities, err := c.GetEntitiesByDomain(ctx, "scene")
	if err != nil {
		return nil, err
	}

	scenes := make([]*Scene, 0, len(entities))
	for _, e := range entities {
		scene := &Scene{
			EntityID: e.EntityID,
			Name:     e.Name,
			Icon:     e.Icon,
		}
		scenes = append(scenes, scene)
	}

	return scenes, nil
}

// ActivateScene activates a scene.
func (c *client) ActivateScene(ctx context.Context, sceneID string) error {
	return c.CallService(ctx, &ServiceCallRequest{
		Domain:  "scene",
		Service: "turn_on",
		Target:  &ServiceTarget{EntityID: sceneID},
	})
}

// GetAutomations gets all automations.
func (c *client) GetAutomations(ctx context.Context) ([]*Automation, error) {
	entities, err := c.GetEntitiesByDomain(ctx, "automation")
	if err != nil {
		return nil, err
	}

	automations := make([]*Automation, 0, len(entities))
	for _, e := range entities {
		automation := &Automation{
			EntityID: e.EntityID,
			Name:     e.Name,
			State:    e.State.State,
		}
		if lastTriggered, ok := e.State.Attributes["last_triggered"].(string); ok {
			automation.LastTriggered, _ = time.Parse(time.RFC3339, lastTriggered)
		}
		automations = append(automations, automation)
	}

	return automations, nil
}

// TriggerAutomation triggers an automation.
func (c *client) TriggerAutomation(ctx context.Context, automationID string) error {
	return c.CallService(ctx, &ServiceCallRequest{
		Domain:  "automation",
		Service: "trigger",
		Target:  &ServiceTarget{EntityID: automationID},
	})
}

// ToggleAutomation enables or disables an automation.
func (c *client) ToggleAutomation(ctx context.Context, automationID string, enable bool) error {
	service := "turn_off"
	if enable {
		service = "turn_on"
	}
	return c.CallService(ctx, &ServiceCallRequest{
		Domain:  "automation",
		Service: service,
		Target:  &ServiceTarget{EntityID: automationID},
	})
}

// SubscribeEvents subscribes to events of a specific type.
func (c *client) SubscribeEvents(ctx context.Context, eventType string, callback EventCallback) error {
	c.callbackMu.Lock()
	c.eventCallbacks[eventType] = append(c.eventCallbacks[eventType], callback)
	c.callbackMu.Unlock()

	// Subscribe via WebSocket if connected
	c.mu.RLock()
	conn := c.wsConn
	c.mu.RUnlock()

	if conn != nil {
		c.mu.Lock()
		c.msgID++
		msgID := c.msgID
		c.mu.Unlock()

		msg := map[string]interface{}{
			"id":         msgID,
			"type":       "subscribe_events",
			"event_type": eventType,
		}
		return conn.WriteJSON(msg)
	}

	return nil
}

// SubscribeStateChanges subscribes to state change events.
func (c *client) SubscribeStateChanges(ctx context.Context, callback StateChangeCallback) error {
	c.callbackMu.Lock()
	c.stateChangeCallbacks = append(c.stateChangeCallbacks, callback)
	c.callbackMu.Unlock()

	return c.SubscribeEvents(ctx, "state_changed", func(event *Event) {
		// Handled in handleStateChanged
	})
}

// FireEvent fires a custom event.
func (c *client) FireEvent(ctx context.Context, eventType string, data map[string]interface{}) error {
	path := "/api/events/" + eventType
	_, err := c.doRequest(ctx, "POST", path, data)
	return err
}

// refreshEntities periodically refreshes the entity cache.
func (c *client) refreshEntities(ctx context.Context) {
	ticker := time.NewTicker(c.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.GetEntities(ctx)
		}
	}
}
