// Package webhook provides webhook management for external integrations.
package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// WebhookType represents the type of webhook.
type WebhookType string

const (
	// TypeWake triggers the agent with the webhook payload.
	TypeWake WebhookType = "wake"
	// TypeAgent runs an isolated agent with the payload.
	TypeAgent WebhookType = "agent"
	// TypeCustom uses a custom handler.
	TypeCustom WebhookType = "custom"
)

// Webhook represents a registered webhook.
type Webhook struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Type        WebhookType            `json:"type"`
	Secret      string                 `json:"secret,omitempty"`
	Enabled     bool                   `json:"enabled"`
	Description string                 `json:"description,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	LastUsedAt  *time.Time             `json:"last_used_at,omitempty"`
	UseCount    int64                  `json:"use_count"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// WebhookEvent represents an incoming webhook event.
type WebhookEvent struct {
	ID          string                 `json:"id"`
	WebhookID   string                 `json:"webhook_id"`
	Payload     json.RawMessage        `json:"payload"`
	Headers     map[string]string      `json:"headers"`
	ReceivedAt  time.Time              `json:"received_at"`
	ProcessedAt *time.Time             `json:"processed_at,omitempty"`
	Status      string                 `json:"status"`
	Error       string                 `json:"error,omitempty"`
	Response    interface{}            `json:"response,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// WebhookHandler is a function that handles webhook events.
type WebhookHandler func(ctx context.Context, event *WebhookEvent) (interface{}, error)

// Config contains webhook service configuration.
type Config struct {
	// Enabled indicates if webhooks are enabled.
	Enabled bool `mapstructure:"enabled"`
	// SecretLength is the length of generated secrets.
	SecretLength int `mapstructure:"secret_length"`
	// MaxPayloadSize is the maximum payload size in bytes.
	MaxPayloadSize int64 `mapstructure:"max_payload_size"`
	// EventRetentionHours is how long to keep events.
	EventRetentionHours int `mapstructure:"event_retention_hours"`
	// MaxEventsPerWebhook is the maximum events to keep per webhook.
	MaxEventsPerWebhook int `mapstructure:"max_events_per_webhook"`
}

// DefaultConfig returns the default webhook configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:             true,
		SecretLength:        32,
		MaxPayloadSize:      1024 * 1024, // 1MB
		EventRetentionHours: 24,
		MaxEventsPerWebhook: 100,
	}
}

// Service manages webhooks.
type Service struct {
	config   Config
	logger   *zap.Logger
	webhooks map[string]*Webhook
	events   map[string][]*WebhookEvent
	handlers map[WebhookType]WebhookHandler
	mu       sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
}

// NewService creates a new webhook service.
func NewService(cfg Config, logger *zap.Logger) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	return &Service{
		config:   cfg,
		logger:   logger.With(zap.String("component", "webhook")),
		webhooks: make(map[string]*Webhook),
		events:   make(map[string][]*WebhookEvent),
		handlers: make(map[WebhookType]WebhookHandler),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// RegisterHandler registers a handler for a webhook type.
func (s *Service) RegisterHandler(webhookType WebhookType, handler WebhookHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[webhookType] = handler
}

// Create creates a new webhook.
func (s *Service) Create(name string, webhookType WebhookType, description string) (*Webhook, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate ID and secret
	id, err := generateID(16)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ID: %w", err)
	}

	secret, err := generateSecret(s.config.SecretLength)
	if err != nil {
		return nil, fmt.Errorf("failed to generate secret: %w", err)
	}

	webhook := &Webhook{
		ID:          id,
		Name:        name,
		Type:        webhookType,
		Secret:      secret,
		Enabled:     true,
		Description: description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Metadata:    make(map[string]interface{}),
	}

	s.webhooks[id] = webhook
	s.events[id] = make([]*WebhookEvent, 0)

	s.logger.Info("webhook created",
		zap.String("id", id),
		zap.String("name", name),
		zap.String("type", string(webhookType)))

	return webhook, nil
}

// Get returns a webhook by ID.
func (s *Service) Get(id string) (*Webhook, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	webhook, exists := s.webhooks[id]
	if exists {
		copy := *webhook
		return &copy, true
	}
	return nil, false
}

// List returns all webhooks.
func (s *Service) List() []*Webhook {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Webhook, 0, len(s.webhooks))
	for _, webhook := range s.webhooks {
		copy := *webhook
		result = append(result, &copy)
	}
	return result
}

// Update updates a webhook.
func (s *Service) Update(id string, name, description string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	webhook, exists := s.webhooks[id]
	if !exists {
		return fmt.Errorf("webhook %s not found", id)
	}

	webhook.Name = name
	webhook.Description = description
	webhook.Enabled = enabled
	webhook.UpdatedAt = time.Now()

	return nil
}

// Delete deletes a webhook.
func (s *Service) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.webhooks[id]; !exists {
		return fmt.Errorf("webhook %s not found", id)
	}

	delete(s.webhooks, id)
	delete(s.events, id)

	s.logger.Info("webhook deleted", zap.String("id", id))
	return nil
}

// RegenerateSecret regenerates the secret for a webhook.
func (s *Service) RegenerateSecret(id string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	webhook, exists := s.webhooks[id]
	if !exists {
		return "", fmt.Errorf("webhook %s not found", id)
	}

	secret, err := generateSecret(s.config.SecretLength)
	if err != nil {
		return "", fmt.Errorf("failed to generate secret: %w", err)
	}

	webhook.Secret = secret
	webhook.UpdatedAt = time.Now()

	return secret, nil
}

// HandleRequest handles an incoming webhook request.
func (s *Service) HandleRequest(w http.ResponseWriter, r *http.Request, webhookID string) {
	s.mu.RLock()
	webhook, exists := s.webhooks[webhookID]
	if !exists {
		s.mu.RUnlock()
		http.Error(w, "webhook not found", http.StatusNotFound)
		return
	}

	if !webhook.Enabled {
		s.mu.RUnlock()
		http.Error(w, "webhook is disabled", http.StatusForbidden)
		return
	}

	handler, hasHandler := s.handlers[webhook.Type]
	s.mu.RUnlock()

	// Read body
	body, err := io.ReadAll(io.LimitReader(r.Body, s.config.MaxPayloadSize))
	if err != nil {
		s.logger.Error("failed to read webhook body", zap.Error(err))
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	// Verify signature if secret is set
	if webhook.Secret != "" {
		signature := r.Header.Get("X-Webhook-Signature")
		if signature == "" {
			signature = r.Header.Get("X-Hub-Signature-256")
		}
		if !s.verifySignature(body, signature, webhook.Secret) {
			s.logger.Warn("webhook signature verification failed",
				zap.String("webhook_id", webhookID))
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	}

	// Create event
	eventID, _ := generateID(16)
	headers := make(map[string]string)
	for key := range r.Header {
		headers[key] = r.Header.Get(key)
	}

	event := &WebhookEvent{
		ID:         eventID,
		WebhookID:  webhookID,
		Payload:    body,
		Headers:    headers,
		ReceivedAt: time.Now(),
		Status:     "received",
		Metadata:   make(map[string]interface{}),
	}

	// Store event
	s.mu.Lock()
	webhook.UseCount++
	now := time.Now()
	webhook.LastUsedAt = &now
	s.events[webhookID] = append(s.events[webhookID], event)

	// Trim old events
	if len(s.events[webhookID]) > s.config.MaxEventsPerWebhook {
		s.events[webhookID] = s.events[webhookID][len(s.events[webhookID])-s.config.MaxEventsPerWebhook:]
	}
	s.mu.Unlock()

	// Process event
	if hasHandler {
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		response, err := handler(ctx, event)

		s.mu.Lock()
		processedAt := time.Now()
		event.ProcessedAt = &processedAt
		if err != nil {
			event.Status = "failed"
			event.Error = err.Error()
			s.logger.Error("webhook handler failed",
				zap.String("webhook_id", webhookID),
				zap.String("event_id", eventID),
				zap.Error(err))
		} else {
			event.Status = "processed"
			event.Response = response
		}
		s.mu.Unlock()

		if err != nil {
			http.Error(w, "processing failed", http.StatusInternalServerError)
			return
		}

		// Return response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "ok",
			"event_id": eventID,
			"response": response,
		})
	} else {
		s.mu.Lock()
		event.Status = "no_handler"
		s.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "ok",
			"event_id": eventID,
		})
	}
}

// GetEvents returns events for a webhook.
func (s *Service) GetEvents(webhookID string, limit int) ([]*WebhookEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	events, exists := s.events[webhookID]
	if !exists {
		return nil, fmt.Errorf("webhook %s not found", webhookID)
	}

	if limit <= 0 || limit > len(events) {
		limit = len(events)
	}

	// Return most recent events
	result := make([]*WebhookEvent, limit)
	for i := 0; i < limit; i++ {
		idx := len(events) - limit + i
		copy := *events[idx]
		result[i] = &copy
	}

	return result, nil
}

// verifySignature verifies the HMAC signature.
func (s *Service) verifySignature(payload []byte, signature, secret string) bool {
	if signature == "" {
		return false
	}

	// Remove "sha256=" prefix if present
	if len(signature) > 7 && signature[:7] == "sha256=" {
		signature = signature[7:]
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(signature))
}

// Stop stops the webhook service.
func (s *Service) Stop() {
	s.cancel()
}

// generateID generates a random ID.
func generateID(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// generateSecret generates a random secret.
func generateSecret(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// Handler returns an HTTP handler for the webhook service.
func (s *Service) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract webhook ID from path
		// Expected path: /hooks/{id}
		path := r.URL.Path
		if len(path) < 8 || path[:7] != "/hooks/" {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}
		webhookID := path[7:]

		s.HandleRequest(w, r, webhookID)
	}
}
