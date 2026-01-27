// Package homeassistant provides Home Assistant integration.
// This package implements a client for the Home Assistant REST and WebSocket APIs
// to enable smart home control from ZimaOS-Echo.
package homeassistant

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrNotConnected is returned when not connected to Home Assistant.
	ErrNotConnected = errors.New("not connected to Home Assistant")
	// ErrEntityNotFound is returned when an entity is not found.
	ErrEntityNotFound = errors.New("entity not found")
	// ErrServiceNotFound is returned when a service is not found.
	ErrServiceNotFound = errors.New("service not found")
	// ErrInvalidToken is returned when the access token is invalid.
	ErrInvalidToken = errors.New("invalid access token")
	// ErrConnectionFailed is returned when connection fails.
	ErrConnectionFailed = errors.New("connection failed")
	// ErrTimeout is returned when an operation times out.
	ErrTimeout = errors.New("operation timeout")
)

// EntityState represents the state of a Home Assistant entity.
type EntityState struct {
	// EntityID is the unique identifier (e.g., "light.living_room").
	EntityID string `json:"entity_id"`
	// State is the current state value.
	State string `json:"state"`
	// Attributes contains additional entity attributes.
	Attributes map[string]interface{} `json:"attributes"`
	// LastChanged is when the state last changed.
	LastChanged time.Time `json:"last_changed"`
	// LastUpdated is when the state was last updated.
	LastUpdated time.Time `json:"last_updated"`
	// Context contains context information.
	Context *StateContext `json:"context,omitempty"`
}

// StateContext contains context information for a state change.
type StateContext struct {
	ID       string `json:"id"`
	ParentID string `json:"parent_id,omitempty"`
	UserID   string `json:"user_id,omitempty"`
}

// Entity represents a Home Assistant entity with metadata.
type Entity struct {
	// EntityID is the unique identifier.
	EntityID string `json:"entity_id"`
	// Name is the friendly name.
	Name string `json:"name"`
	// Domain is the entity domain (e.g., "light", "switch").
	Domain string `json:"domain"`
	// DeviceClass is the device class (e.g., "temperature").
	DeviceClass string `json:"device_class,omitempty"`
	// UnitOfMeasurement is the unit for sensor values.
	UnitOfMeasurement string `json:"unit_of_measurement,omitempty"`
	// Icon is the icon name.
	Icon string `json:"icon,omitempty"`
	// State is the current state.
	State *EntityState `json:"state,omitempty"`
	// Alias is a custom alias set by the user.
	Alias string `json:"alias,omitempty"`
	// Hidden indicates if the entity is hidden.
	Hidden bool `json:"hidden,omitempty"`
}

// HAServiceInfo represents a Home Assistant service definition.
type HAServiceInfo struct {
	// Domain is the service domain.
	Domain string `json:"domain"`
	// Service is the service name.
	Service string `json:"service"`
	// Name is the friendly name.
	Name string `json:"name,omitempty"`
	// Description is the service description.
	Description string `json:"description,omitempty"`
	// Fields contains the service fields/parameters.
	Fields map[string]ServiceField `json:"fields,omitempty"`
}

// ServiceField represents a service field/parameter.
type ServiceField struct {
	// Name is the field name.
	Name string `json:"name,omitempty"`
	// Description is the field description.
	Description string `json:"description,omitempty"`
	// Required indicates if the field is required.
	Required bool `json:"required,omitempty"`
	// Example is an example value.
	Example interface{} `json:"example,omitempty"`
	// Selector contains the field selector.
	Selector map[string]interface{} `json:"selector,omitempty"`
}

// Scene represents a Home Assistant scene.
type Scene struct {
	// EntityID is the scene entity ID.
	EntityID string `json:"entity_id"`
	// Name is the scene name.
	Name string `json:"name"`
	// Icon is the scene icon.
	Icon string `json:"icon,omitempty"`
}

// Automation represents a Home Assistant automation.
type Automation struct {
	// EntityID is the automation entity ID.
	EntityID string `json:"entity_id"`
	// Name is the automation name.
	Name string `json:"name"`
	// State is the automation state (on/off).
	State string `json:"state"`
	// LastTriggered is when the automation was last triggered.
	LastTriggered time.Time `json:"last_triggered,omitempty"`
}

// Event represents a Home Assistant event.
type Event struct {
	// EventType is the event type.
	EventType string `json:"event_type"`
	// Data contains the event data.
	Data map[string]interface{} `json:"data,omitempty"`
	// Origin is the event origin.
	Origin string `json:"origin,omitempty"`
	// TimeFired is when the event was fired.
	TimeFired time.Time `json:"time_fired"`
	// Context contains context information.
	Context *StateContext `json:"context,omitempty"`
}

// ServiceCallRequest represents a request to call a service.
type ServiceCallRequest struct {
	// Domain is the service domain.
	Domain string `json:"domain"`
	// Service is the service name.
	Service string `json:"service"`
	// Target contains the target entities.
	Target *ServiceTarget `json:"target,omitempty"`
	// ServiceData contains additional service data.
	ServiceData map[string]interface{} `json:"service_data,omitempty"`
}

// ServiceTarget represents the target for a service call.
type ServiceTarget struct {
	// EntityID is a single entity ID or list of entity IDs.
	EntityID interface{} `json:"entity_id,omitempty"`
	// DeviceID is a single device ID or list of device IDs.
	DeviceID interface{} `json:"device_id,omitempty"`
	// AreaID is a single area ID or list of area IDs.
	AreaID interface{} `json:"area_id,omitempty"`
}

// Config represents the Home Assistant configuration.
type Config struct {
	// URL is the Home Assistant URL.
	URL string `json:"url" yaml:"url"`
	// Token is the long-lived access token.
	Token string `json:"token" yaml:"token"`
	// VerifySSL indicates whether to verify SSL certificates.
	VerifySSL bool `json:"verify_ssl" yaml:"verify_ssl"`
	// PollInterval is the interval for polling entity states.
	PollInterval time.Duration `json:"poll_interval" yaml:"poll_interval"`
	// EntityFilter contains entity filter patterns.
	EntityFilter *EntityFilter `json:"entity_filter,omitempty" yaml:"entity_filter,omitempty"`
}

// EntityFilter contains patterns for filtering entities.
type EntityFilter struct {
	// IncludeDomains is a list of domains to include.
	IncludeDomains []string `json:"include_domains,omitempty" yaml:"include_domains,omitempty"`
	// ExcludeDomains is a list of domains to exclude.
	ExcludeDomains []string `json:"exclude_domains,omitempty" yaml:"exclude_domains,omitempty"`
	// IncludeEntities is a list of entity IDs to include.
	IncludeEntities []string `json:"include_entities,omitempty" yaml:"include_entities,omitempty"`
	// ExcludeEntities is a list of entity IDs to exclude.
	ExcludeEntities []string `json:"exclude_entities,omitempty" yaml:"exclude_entities,omitempty"`
}

// EventCallback is called when an event is received.
type EventCallback func(event *Event)

// StateChangeCallback is called when an entity state changes.
type StateChangeCallback func(entityID string, oldState, newState *EntityState)

// Client defines the Home Assistant client interface.
type Client interface {
	// Connect connects to Home Assistant.
	Connect(ctx context.Context) error
	// Disconnect disconnects from Home Assistant.
	Disconnect() error
	// IsConnected returns whether the client is connected.
	IsConnected() bool

	// GetConfig gets the Home Assistant configuration.
	GetConfig(ctx context.Context) (map[string]interface{}, error)

	// Entity operations
	GetEntities(ctx context.Context) ([]*Entity, error)
	GetEntity(ctx context.Context, entityID string) (*Entity, error)
	GetEntityState(ctx context.Context, entityID string) (*EntityState, error)
	GetEntitiesByDomain(ctx context.Context, domain string) ([]*Entity, error)

	// Service operations
	GetServices(ctx context.Context) ([]*HAServiceInfo, error)
	CallService(ctx context.Context, req *ServiceCallRequest) error

	// Scene operations
	GetScenes(ctx context.Context) ([]*Scene, error)
	ActivateScene(ctx context.Context, sceneID string) error

	// Automation operations
	GetAutomations(ctx context.Context) ([]*Automation, error)
	TriggerAutomation(ctx context.Context, automationID string) error
	ToggleAutomation(ctx context.Context, automationID string, enable bool) error

	// Event operations
	SubscribeEvents(ctx context.Context, eventType string, callback EventCallback) error
	SubscribeStateChanges(ctx context.Context, callback StateChangeCallback) error
	FireEvent(ctx context.Context, eventType string, data map[string]interface{}) error
}

// Service defines the high-level Home Assistant service interface used by handlers.
type Service interface {
	// Connection management
	Connect(ctx context.Context, config *Config) error
	Disconnect() error
	IsConnected() bool

	// Entity operations
	GetAllEntities(ctx context.Context) ([]*Entity, error)
	GetEntitiesByDomain(ctx context.Context, domain string) ([]*Entity, error)
	GetEntity(ctx context.Context, entityID string) (*Entity, error)
	ControlEntity(ctx context.Context, entityID string, action string, params map[string]interface{}) error

	// Scene operations
	GetScenes(ctx context.Context) ([]*Scene, error)
	ActivateScene(ctx context.Context, sceneID string) error

	// Automation operations
	GetAutomations(ctx context.Context) ([]*Automation, error)
	TriggerAutomation(ctx context.Context, automationID string) error
	ToggleAutomation(ctx context.Context, automationID string, enable bool) error

	// Natural language control
	ProcessCommand(ctx context.Context, command string) (*CommandResult, error)
}
