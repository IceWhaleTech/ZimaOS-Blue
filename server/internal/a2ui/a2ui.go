// Package a2ui provides Agent-to-UI (A2UI) capabilities for dynamic UI generation.
// A2UI allows AI agents to generate interactive UI components that can be rendered
// by the frontend, enabling rich interactions beyond simple text responses.
package a2ui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ComponentType represents the type of UI component.
type ComponentType string

const (
	// ComponentTypeText represents a text display component.
	ComponentTypeText ComponentType = "text"
	// ComponentTypeButton represents a clickable button.
	ComponentTypeButton ComponentType = "button"
	// ComponentTypeInput represents a text input field.
	ComponentTypeInput ComponentType = "input"
	// ComponentTypeSelect represents a dropdown select.
	ComponentTypeSelect ComponentType = "select"
	// ComponentTypeCheckbox represents a checkbox.
	ComponentTypeCheckbox ComponentType = "checkbox"
	// ComponentTypeRadio represents radio buttons.
	ComponentTypeRadio ComponentType = "radio"
	// ComponentTypeSlider represents a slider input.
	ComponentTypeSlider ComponentType = "slider"
	// ComponentTypeImage represents an image display.
	ComponentTypeImage ComponentType = "image"
	// ComponentTypeCard represents a card container.
	ComponentTypeCard ComponentType = "card"
	// ComponentTypeList represents a list of items.
	ComponentTypeList ComponentType = "list"
	// ComponentTypeTable represents a data table.
	ComponentTypeTable ComponentType = "table"
	// ComponentTypeChart represents a chart/graph.
	ComponentTypeChart ComponentType = "chart"
	// ComponentTypeForm represents a form container.
	ComponentTypeForm ComponentType = "form"
	// ComponentTypeProgress represents a progress indicator.
	ComponentTypeProgress ComponentType = "progress"
	// ComponentTypeAlert represents an alert/notification.
	ComponentTypeAlert ComponentType = "alert"
	// ComponentTypeCode represents a code block.
	ComponentTypeCode ComponentType = "code"
	// ComponentTypeMarkdown represents markdown content.
	ComponentTypeMarkdown ComponentType = "markdown"
	// ComponentTypeContainer represents a generic container.
	ComponentTypeContainer ComponentType = "container"
	// ComponentTypeGrid represents a grid layout.
	ComponentTypeGrid ComponentType = "grid"
	// ComponentTypeTabs represents tabbed content.
	ComponentTypeTabs ComponentType = "tabs"
	// ComponentTypeAccordion represents collapsible sections.
	ComponentTypeAccordion ComponentType = "accordion"
)

// Component represents a UI component that can be rendered by the frontend.
type Component struct {
	// ID is the unique identifier for this component.
	ID string `json:"id"`
	// Type is the component type.
	Type ComponentType `json:"type"`
	// Props contains component-specific properties.
	Props map[string]interface{} `json:"props,omitempty"`
	// Children contains nested components.
	Children []Component `json:"children,omitempty"`
	// Actions contains interactive actions for this component.
	Actions []Action `json:"actions,omitempty"`
	// Style contains CSS-like styling properties.
	Style map[string]interface{} `json:"style,omitempty"`
	// Validation contains validation rules for input components.
	Validation *Validation `json:"validation,omitempty"`
}

type componentList []Component

func (c *componentList) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		*c = nil
		return nil
	}

	switch trimmed[0] {
	case '{':
		var component Component
		if err := json.Unmarshal(trimmed, &component); err != nil {
			return err
		}
		*c = componentList{component}
		return nil
	case '[':
		var components []Component
		if err := json.Unmarshal(trimmed, &components); err != nil {
			return err
		}
		*c = componentList(components)
		return nil
	default:
		return fmt.Errorf("components must be an object or array")
	}
}

// Action represents an interactive action that can be triggered by a component.
type Action struct {
	// ID is the unique identifier for this action.
	ID string `json:"id"`
	// Type is the action type (click, submit, change, etc.).
	Type string `json:"type"`
	// Label is the display label for the action.
	Label string `json:"label,omitempty"`
	// Handler is the handler name to invoke when action is triggered.
	Handler string `json:"handler"`
	// Params contains parameters to pass to the handler.
	Params map[string]interface{} `json:"params,omitempty"`
	// Confirm contains confirmation dialog settings.
	Confirm *ConfirmDialog `json:"confirm,omitempty"`
}

// ConfirmDialog represents a confirmation dialog before action execution.
type ConfirmDialog struct {
	// Title is the dialog title.
	Title string `json:"title"`
	// Message is the confirmation message.
	Message string `json:"message"`
	// ConfirmText is the confirm button text.
	ConfirmText string `json:"confirm_text"`
	// CancelText is the cancel button text.
	CancelText string `json:"cancel_text"`
}

// Validation represents validation rules for input components.
type Validation struct {
	// Required indicates if the field is required.
	Required bool `json:"required,omitempty"`
	// MinLength is the minimum string length.
	MinLength int `json:"min_length,omitempty"`
	// MaxLength is the maximum string length.
	MaxLength int `json:"max_length,omitempty"`
	// Min is the minimum numeric value.
	Min *float64 `json:"min,omitempty"`
	// Max is the maximum numeric value.
	Max *float64 `json:"max,omitempty"`
	// Pattern is a regex pattern for validation.
	Pattern string `json:"pattern,omitempty"`
	// Message is the validation error message.
	Message string `json:"message,omitempty"`
}

// Canvas represents a collection of UI components to be rendered.
type Canvas struct {
	// ID is the unique identifier for this canvas.
	ID string `json:"id"`
	// Title is the canvas title.
	Title string `json:"title,omitempty"`
	// Description is the canvas description.
	Description string `json:"description,omitempty"`
	// Components contains the root-level components.
	Components []Component `json:"components"`
	// Layout specifies the layout type (vertical, horizontal, grid).
	Layout string `json:"layout,omitempty"`
	// Metadata contains additional canvas metadata.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	// CreatedAt is when the canvas was created.
	CreatedAt time.Time `json:"created_at"`
	// ExpiresAt is when the canvas expires (optional).
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

func (c *Canvas) UnmarshalJSON(data []byte) error {
	type canvasPayload struct {
		ID          string                 `json:"id"`
		Title       string                 `json:"title,omitempty"`
		Description string                 `json:"description,omitempty"`
		Components  componentList          `json:"components"`
		Layout      string                 `json:"layout,omitempty"`
		Metadata    map[string]interface{} `json:"metadata,omitempty"`
		CreatedAt   time.Time              `json:"created_at"`
		ExpiresAt   *time.Time             `json:"expires_at,omitempty"`
	}

	var payload canvasPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}

	*c = Canvas{
		ID:          payload.ID,
		Title:       payload.Title,
		Description: payload.Description,
		Components:  []Component(payload.Components),
		Layout:      payload.Layout,
		Metadata:    payload.Metadata,
		CreatedAt:   payload.CreatedAt,
		ExpiresAt:   payload.ExpiresAt,
	}
	return nil
}

// ActionResult represents the result of an action execution.
type ActionResult struct {
	// Success indicates if the action was successful.
	Success bool `json:"success"`
	// Data contains the result data.
	Data interface{} `json:"data,omitempty"`
	// Error contains the error message if failed.
	Error string `json:"error,omitempty"`
	// UpdatedComponents contains components to update after action.
	UpdatedComponents []Component `json:"updated_components,omitempty"`
	// NewCanvas contains a new canvas to render (replaces current).
	NewCanvas *Canvas `json:"new_canvas,omitempty"`
}

// ActionHandler is a function that handles an action.
type ActionHandler func(ctx context.Context, action Action, formData map[string]interface{}) (*ActionResult, error)

// Manager manages A2UI canvases and action handlers.
type Manager struct {
	logger   *zap.Logger
	mu       sync.RWMutex
	canvases map[string]*Canvas
	handlers map[string]ActionHandler
}

// NewManager creates a new A2UI manager.
func NewManager(logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Manager{
		logger:   logger.With(zap.String("component", "a2ui")),
		canvases: make(map[string]*Canvas),
		handlers: make(map[string]ActionHandler),
	}
}

// RegisterHandler registers an action handler.
func (m *Manager) RegisterHandler(name string, handler ActionHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[name] = handler
	m.logger.Debug("registered action handler", zap.String("name", name))
}

// UnregisterHandler removes an action handler.
func (m *Manager) UnregisterHandler(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.handlers, name)
}

// CreateCanvas creates and stores a new canvas.
func (m *Manager) CreateCanvas(canvas *Canvas) error {
	if canvas.ID == "" {
		return fmt.Errorf("canvas ID is required")
	}

	canvas.CreatedAt = time.Now()

	m.mu.Lock()
	m.canvases[canvas.ID] = canvas
	m.mu.Unlock()

	m.logger.Debug("created canvas",
		zap.String("id", canvas.ID),
		zap.Int("components", len(canvas.Components)))

	return nil
}

// GetCanvas retrieves a canvas by ID.
func (m *Manager) GetCanvas(id string) (*Canvas, error) {
	m.mu.RLock()
	canvas, ok := m.canvases[id]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("canvas not found: %s", id)
	}

	// Check expiration
	if canvas.ExpiresAt != nil && time.Now().After(*canvas.ExpiresAt) {
		m.DeleteCanvas(id)
		return nil, fmt.Errorf("canvas expired: %s", id)
	}

	return canvas, nil
}

// UpdateCanvas updates an existing canvas.
func (m *Manager) UpdateCanvas(canvas *Canvas) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.canvases[canvas.ID]; !ok {
		return fmt.Errorf("canvas not found: %s", canvas.ID)
	}

	m.canvases[canvas.ID] = canvas
	return nil
}

// DeleteCanvas removes a canvas.
func (m *Manager) DeleteCanvas(id string) {
	m.mu.Lock()
	delete(m.canvases, id)
	m.mu.Unlock()
}

// ExecuteAction executes an action and returns the result.
func (m *Manager) ExecuteAction(ctx context.Context, canvasID string, actionID string, formData map[string]interface{}) (*ActionResult, error) {
	// Get canvas
	canvas, err := m.GetCanvas(canvasID)
	if err != nil {
		return nil, err
	}

	// Find action in canvas
	action := m.findAction(canvas.Components, actionID)
	if action == nil {
		return nil, fmt.Errorf("action not found: %s", actionID)
	}

	// Get handler
	m.mu.RLock()
	handler, ok := m.handlers[action.Handler]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("handler not found: %s", action.Handler)
	}

	// Execute handler
	result, err := handler(ctx, *action, formData)
	if err != nil {
		m.logger.Error("action handler failed",
			zap.String("canvas_id", canvasID),
			zap.String("action_id", actionID),
			zap.Error(err))
		return &ActionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return result, nil
}

// findAction recursively finds an action in components.
func (m *Manager) findAction(components []Component, actionID string) *Action {
	for _, comp := range components {
		for i := range comp.Actions {
			if comp.Actions[i].ID == actionID {
				return &comp.Actions[i]
			}
		}
		if action := m.findAction(comp.Children, actionID); action != nil {
			return action
		}
	}
	return nil
}

// CleanupExpired removes expired canvases.
func (m *Manager) CleanupExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	count := 0

	for id, canvas := range m.canvases {
		if canvas.ExpiresAt != nil && now.After(*canvas.ExpiresAt) {
			delete(m.canvases, id)
			count++
		}
	}

	if count > 0 {
		m.logger.Debug("cleaned up expired canvases", zap.Int("count", count))
	}

	return count
}

// ListCanvases returns all active canvas IDs.
func (m *Manager) ListCanvases() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.canvases))
	for id := range m.canvases {
		ids = append(ids, id)
	}
	return ids
}

// Builder provides a fluent interface for building UI components.
type Builder struct {
	component Component
}

// NewBuilder creates a new component builder.
func NewBuilder(componentType ComponentType) *Builder {
	return &Builder{
		component: Component{
			Type:  componentType,
			Props: make(map[string]interface{}),
			Style: make(map[string]interface{}),
		},
	}
}

// ID sets the component ID.
func (b *Builder) ID(id string) *Builder {
	b.component.ID = id
	return b
}

// Prop sets a component property.
func (b *Builder) Prop(key string, value interface{}) *Builder {
	b.component.Props[key] = value
	return b
}

// Props sets multiple properties.
func (b *Builder) Props(props map[string]interface{}) *Builder {
	for k, v := range props {
		b.component.Props[k] = v
	}
	return b
}

// Style sets a style property.
func (b *Builder) Style(key string, value interface{}) *Builder {
	b.component.Style[key] = value
	return b
}

// Styles sets multiple style properties.
func (b *Builder) Styles(styles map[string]interface{}) *Builder {
	for k, v := range styles {
		b.component.Style[k] = v
	}
	return b
}

// Child adds a child component.
func (b *Builder) Child(child Component) *Builder {
	b.component.Children = append(b.component.Children, child)
	return b
}

// Children adds multiple child components.
func (b *Builder) Children(children ...Component) *Builder {
	b.component.Children = append(b.component.Children, children...)
	return b
}

// Action adds an action to the component.
func (b *Builder) Action(action Action) *Builder {
	b.component.Actions = append(b.component.Actions, action)
	return b
}

// Validation sets validation rules.
func (b *Builder) Validation(validation *Validation) *Builder {
	b.component.Validation = validation
	return b
}

// Build returns the built component.
func (b *Builder) Build() Component {
	return b.component
}

// Helper functions for common components

// Text creates a text component.
func Text(id, content string) Component {
	return NewBuilder(ComponentTypeText).
		ID(id).
		Prop("content", content).
		Build()
}

// Button creates a button component.
func Button(id, label, handler string) Component {
	return NewBuilder(ComponentTypeButton).
		ID(id).
		Prop("label", label).
		Action(Action{
			ID:      id + "_click",
			Type:    "click",
			Handler: handler,
		}).
		Build()
}

// Input creates an input component.
func Input(id, label, placeholder string) Component {
	return NewBuilder(ComponentTypeInput).
		ID(id).
		Prop("label", label).
		Prop("placeholder", placeholder).
		Build()
}

// Select creates a select component.
func Select(id, label string, options []map[string]interface{}) Component {
	return NewBuilder(ComponentTypeSelect).
		ID(id).
		Prop("label", label).
		Prop("options", options).
		Build()
}

// Card creates a card component.
func Card(id, title string, children ...Component) Component {
	return NewBuilder(ComponentTypeCard).
		ID(id).
		Prop("title", title).
		Children(children...).
		Build()
}

// Alert creates an alert component.
func Alert(id, alertType, message string) Component {
	return NewBuilder(ComponentTypeAlert).
		ID(id).
		Prop("type", alertType).
		Prop("message", message).
		Build()
}

// Progress creates a progress component.
func Progress(id string, value, max int) Component {
	return NewBuilder(ComponentTypeProgress).
		ID(id).
		Prop("value", value).
		Prop("max", max).
		Build()
}

// Code creates a code block component.
func Code(id, language, code string) Component {
	return NewBuilder(ComponentTypeCode).
		ID(id).
		Prop("language", language).
		Prop("code", code).
		Build()
}

// Markdown creates a markdown component.
func Markdown(id, content string) Component {
	return NewBuilder(ComponentTypeMarkdown).
		ID(id).
		Prop("content", content).
		Build()
}

// Image creates an image component.
func Image(id, src, alt string) Component {
	return NewBuilder(ComponentTypeImage).
		ID(id).
		Prop("src", src).
		Prop("alt", alt).
		Build()
}

// Table creates a table component.
func Table(id string, columns []string, rows [][]interface{}) Component {
	return NewBuilder(ComponentTypeTable).
		ID(id).
		Prop("columns", columns).
		Prop("rows", rows).
		Build()
}

// List creates a list component.
func List(id string, items []string) Component {
	return NewBuilder(ComponentTypeList).
		ID(id).
		Prop("items", items).
		Build()
}

// Form creates a form component.
func Form(id, submitHandler string, children ...Component) Component {
	return NewBuilder(ComponentTypeForm).
		ID(id).
		Children(children...).
		Action(Action{
			ID:      id + "_submit",
			Type:    "submit",
			Handler: submitHandler,
		}).
		Build()
}

// Grid creates a grid layout component.
func Grid(id string, columns int, children ...Component) Component {
	return NewBuilder(ComponentTypeGrid).
		ID(id).
		Prop("columns", columns).
		Children(children...).
		Build()
}

// ToJSON converts a canvas to JSON.
func (c *Canvas) ToJSON() ([]byte, error) {
	return json.Marshal(c)
}

// FromJSON parses a canvas from JSON.
func FromJSON(data []byte) (*Canvas, error) {
	var canvas Canvas
	if err := json.Unmarshal(data, &canvas); err != nil {
		return nil, err
	}
	return &canvas, nil
}
