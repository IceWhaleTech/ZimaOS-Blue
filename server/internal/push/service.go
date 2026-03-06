package push

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/inject"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"go.uber.org/zap"
)

// EventPublisher pushes real-time events to connected clients.
type EventPublisher interface {
	Publish(userID string, eventType string, data any)
}

// CronService is the subset of cron.Service needed by push notifications.
type CronService interface {
	RegisterHandler(name string, handler func(ctx context.Context, payload map[string]interface{}) (interface{}, error))
	CreateJob(name, description, schedule, handler string, payload map[string]interface{}) (string, error)
	DeleteJob(id string) error
}

// WebPushSender sends Web Push notifications to subscribed browsers.
type WebPushSender interface {
	SendToUser(ctx context.Context, userID, title, body string) error
}

// Service coordinates push notification persistence, cron scheduling, and delivery.
type Service struct {
	store      *Store
	logger     *zap.Logger
	mu         sync.RWMutex
	cron       CronService
	injector   inject.MessageInjector
	publisher  EventPublisher
	notifier   Notifier
	webpush    WebPushSender
	localeFunc func() string
}

// NewService creates a new push notification service.
func NewService(store *Store, logger *zap.Logger) *Service {
	return &Service{
		store:  store,
		logger: logger.Named("push"),
	}
}

// SetCron wires the cron service and registers the "push" handler.
func (s *Service) SetCron(c CronService) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cron = c
	if c != nil {
		c.RegisterHandler("push", s.handlePushFired)
	}
}

// SetMessageInjector wires the message injector.
func (s *Service) SetMessageInjector(inj inject.MessageInjector) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.injector = inj
}

// SetEventPublisher wires the SSE event publisher.
func (s *Service) SetEventPublisher(pub EventPublisher) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.publisher = pub
}

// SetNotifier wires the native OS notifier.
func (s *Service) SetNotifier(n Notifier) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.notifier = n
}

// SetWebPushSender wires the Web Push sender.
func (s *Service) SetWebPushSender(wp WebPushSender) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.webpush = wp
}

// SetLocaleFunc sets a callback that returns the current locale (e.g. "en-US").
func (s *Service) SetLocaleFunc(fn func() string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.localeFunc = fn
}

// Add creates a new push notification with cron scheduling.
func (s *Service) Add(ctx context.Context, ownerID, message string, fireAt time.Time, recurring, sessionID string) (*PushNotification, error) {
	idBytes := make([]byte, 8)
	rand.Read(idBytes)
	id := fmt.Sprintf("push_%x", idBytes)

	r := &PushNotification{
		ID:        id,
		OwnerID:   ownerID,
		Message:   message,
		FireAt:    fireAt,
		Recurring: recurring,
		SessionID: sessionID,
		Status:    StatusPending,
		CreatedAt: timeutil.NowTime(),
	}

	if err := s.store.Create(ctx, r); err != nil {
		return nil, fmt.Errorf("create push notification: %w", err)
	}

	// Schedule cron job
	if err := s.scheduleCronJob(ctx, r); err != nil {
		s.logger.Warn("failed to schedule cron job, notification saved but won't fire until restart",
			zap.String("id", id), zap.Error(err))
	}

	return r, nil
}

// List returns all push notifications for a user.
func (s *Service) List(ctx context.Context, ownerID string) ([]*PushNotification, error) {
	return s.store.ListByOwner(ctx, ownerID)
}

// Delete removes a push notification and its cron job.
func (s *Service) Delete(ctx context.Context, ownerID, id string) error {
	r, err := s.store.Get(ctx, id)
	if err != nil {
		return err
	}
	if r == nil {
		return fmt.Errorf("push notification %s not found", id)
	}
	if r.OwnerID != ownerID {
		return fmt.Errorf("push notification %s not found", id)
	}

	// Delete cron job if exists
	if r.CronJobID != "" {
		s.mu.RLock()
		c := s.cron
		s.mu.RUnlock()
		if c != nil {
			if err := c.DeleteJob(r.CronJobID); err != nil {
				s.logger.Warn("failed to delete cron job", zap.String("cron_job_id", r.CronJobID), zap.Error(err))
			}
		}
	}

	return s.store.Delete(ctx, id, ownerID)
}

// Clear removes all push notifications for a user.
func (s *Service) Clear(ctx context.Context, ownerID string) (int64, error) {
	// List to get cron job IDs first
	notifications, err := s.store.ListByOwner(ctx, ownerID, StatusPending)
	if err != nil {
		return 0, err
	}

	s.mu.RLock()
	c := s.cron
	s.mu.RUnlock()

	for _, r := range notifications {
		if r.CronJobID != "" && c != nil {
			c.DeleteJob(r.CronJobID)
		}
	}

	return s.store.DeleteByOwner(ctx, ownerID)
}

// RestorePending re-registers cron jobs for all pending notifications after restart.
func (s *Service) RestorePending(ctx context.Context) error {
	pending, err := s.store.ListPending(ctx)
	if err != nil {
		return fmt.Errorf("list pending push notifications: %w", err)
	}

	s.logger.Info("restoring pending push notifications", zap.Int("count", len(pending)))

	now := timeutil.NowTime()
	for _, r := range pending {
		if r.Recurring == "" && r.FireAt.Before(now) {
			// Past-due one-shot: fire immediately
			s.logger.Info("firing past-due push notification", zap.String("id", r.ID))
			s.firePush(ctx, r)
			continue
		}
		if err := s.scheduleCronJob(ctx, r); err != nil {
			s.logger.Warn("failed to restore cron job for push notification",
				zap.String("id", r.ID), zap.Error(err))
		}
	}

	return nil
}

// scheduleCronJob creates a cron job for a push notification.
func (s *Service) scheduleCronJob(ctx context.Context, r *PushNotification) error {
	s.mu.RLock()
	c := s.cron
	s.mu.RUnlock()

	if c == nil {
		return fmt.Errorf("cron service not available")
	}

	schedule := timeToCron(r.FireAt, r.Recurring)
	payload := map[string]interface{}{
		"push_id": r.ID,
	}

	jobID, err := c.CreateJob(
		fmt.Sprintf("push:%s", r.ID),
		r.Message,
		schedule,
		"push",
		payload,
	)
	if err != nil {
		return err
	}

	return s.store.UpdateCronJobID(ctx, r.ID, jobID)
}

// handlePushFired is the cron handler called when a push notification fires.
func (s *Service) handlePushFired(ctx context.Context, payload map[string]interface{}) (interface{}, error) {
	pushID, ok := payload["push_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing push_id in payload")
	}

	r, err := s.store.Get(ctx, pushID)
	if err != nil {
		return nil, fmt.Errorf("get push notification: %w", err)
	}
	if r == nil || r.Status != StatusPending {
		return nil, nil // already fired or deleted
	}

	s.firePush(ctx, r)
	return map[string]string{"push_id": r.ID, "status": "fired"}, nil
}

// firePush executes the push notification: inject message, publish event, update status.
func (s *Service) firePush(ctx context.Context, r *PushNotification) {
	s.logger.Info("firing push notification", zap.String("id", r.ID), zap.String("message", r.Message))

	s.mu.RLock()
	inj := s.injector
	pub := s.publisher
	notif := s.notifier
	wp := s.webpush
	c := s.cron
	localeFn := s.localeFunc
	s.mu.RUnlock()

	locale := "en-US"
	if localeFn != nil {
		if l := localeFn(); l != "" {
			locale = l
		}
	}

	// Inject message into conversation as a typeless alert card
	var conversationID string
	if inj != nil {
		content := fmt.Sprintf("```typeless\n{\"type\":\"alert\",\"icon\":\"⏰\",\"message\":%q,\"variant\":\"info\",\"title_key\":\"push.reminder\"}\n```", r.Message)
		var err error
		conversationID, err = inj.InjectMessage(ctx, r.OwnerID, r.SessionID, content)
		if err != nil {
			s.logger.Error("failed to inject push message", zap.String("id", r.ID), zap.Error(err))
		}
	}

	// Publish SSE events
	if pub != nil {
		pushData := map[string]any{
			"id":      r.ID,
			"message": r.Message,
			"locale":  locale,
		}
		if conversationID != "" {
			pushData["conversation_id"] = conversationID
		}
		pub.Publish(r.OwnerID, "push", pushData)
		if conversationID != "" {
			pub.Publish(r.OwnerID, "conversation_updated", map[string]any{
				"id": conversationID,
			})
		}
	}

	// Native OS notification (best-effort)
	if notif != nil {
		if err := notif.Notify(ctx, "Blue", r.Message); err != nil {
			s.logger.Warn("native notification failed", zap.String("id", r.ID), zap.Error(err))
		}
	}

	// Web Push notification (best-effort, for closed browser tabs)
	if wp != nil {
		if err := wp.SendToUser(ctx, r.OwnerID, "🔔 Reminder", r.Message); err != nil {
			s.logger.Warn("web push failed", zap.String("id", r.ID), zap.Error(err))
		}
	}

	// Update status
	now := timeutil.NowTime()
	if r.Recurring == "" {
		// One-shot: mark as fired, delete cron job
		s.store.UpdateStatus(ctx, r.ID, StatusFired, &now)
		if r.CronJobID != "" && c != nil {
			c.DeleteJob(r.CronJobID)
		}
	} else {
		// Recurring: update fire_at to next occurrence
		next := nextOccurrence(r.FireAt, r.Recurring)
		r.FireAt = next
		s.store.UpdateStatus(ctx, r.ID, StatusPending, nil)
		// Cron job continues running on its schedule
	}
}

// timeToCron converts a fire time and recurring pattern to a 6-field cron expression.
func timeToCron(fireAt time.Time, recurring string) string {
	sec := fireAt.Second()
	min := fireAt.Minute()
	hour := fireAt.Hour()
	day := fireAt.Day()
	month := int(fireAt.Month())
	dow := int(fireAt.Weekday())

	switch recurring {
	case "daily":
		return fmt.Sprintf("%d %d %d * * *", sec, min, hour)
	case "weekly":
		return fmt.Sprintf("%d %d %d * * %d", sec, min, hour, dow)
	case "monthly":
		return fmt.Sprintf("%d %d %d %d * *", sec, min, hour, day)
	default:
		// One-shot: exact time
		return fmt.Sprintf("%d %d %d %d %d *", sec, min, hour, day, month)
	}
}

// nextOccurrence computes the next fire time for a recurring notification.
func nextOccurrence(current time.Time, recurring string) time.Time {
	switch recurring {
	case "daily":
		return current.AddDate(0, 0, 1)
	case "weekly":
		return current.AddDate(0, 0, 7)
	case "monthly":
		return current.AddDate(0, 1, 0)
	default:
		return current
	}
}
