package reminder

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// MessageInjector injects reminder messages into conversations.
type MessageInjector interface {
	InjectReminderMessage(ctx context.Context, ownerID, sessionID, content string) (conversationID string, err error)
}

// EventPublisher pushes real-time events to connected clients.
type EventPublisher interface {
	Publish(userID string, eventType string, data any)
}

// CronService is the subset of cron.Service needed by reminders.
type CronService interface {
	RegisterHandler(name string, handler func(ctx context.Context, payload map[string]interface{}) (interface{}, error))
	CreateJob(name, description, schedule, handler string, payload map[string]interface{}) (string, error)
	DeleteJob(id string) error
}

// Service coordinates reminder persistence, cron scheduling, and notification delivery.
type Service struct {
	store     *Store
	logger    *zap.Logger
	mu        sync.RWMutex
	cron      CronService
	injector  MessageInjector
	publisher EventPublisher
}

// NewService creates a new reminder service.
func NewService(store *Store, logger *zap.Logger) *Service {
	return &Service{
		store:  store,
		logger: logger.Named("reminder"),
	}
}

// SetCron wires the cron service and registers the "reminder" handler.
func (s *Service) SetCron(c CronService) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cron = c
	if c != nil {
		c.RegisterHandler("reminder", s.handleReminderFired)
	}
}

// SetMessageInjector wires the message injector.
func (s *Service) SetMessageInjector(inj MessageInjector) {
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

// Add creates a new reminder with cron scheduling.
func (s *Service) Add(ctx context.Context, ownerID, message string, fireAt time.Time, recurring, sessionID string) (*Reminder, error) {
	idBytes := make([]byte, 8)
	rand.Read(idBytes)
	id := fmt.Sprintf("rem_%x", idBytes)

	r := &Reminder{
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
		return nil, fmt.Errorf("create reminder: %w", err)
	}

	// Schedule cron job
	if err := s.scheduleCronJob(ctx, r); err != nil {
		s.logger.Warn("failed to schedule cron job, reminder saved but won't fire until restart",
			zap.String("id", id), zap.Error(err))
	}

	return r, nil
}

// List returns all reminders for a user.
func (s *Service) List(ctx context.Context, ownerID string) ([]*Reminder, error) {
	return s.store.ListByOwner(ctx, ownerID)
}

// Delete removes a reminder and its cron job.
func (s *Service) Delete(ctx context.Context, ownerID, id string) error {
	r, err := s.store.Get(ctx, id)
	if err != nil {
		return err
	}
	if r == nil {
		return fmt.Errorf("reminder %s not found", id)
	}
	if r.OwnerID != ownerID {
		return fmt.Errorf("reminder %s not found", id)
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

// Clear removes all reminders for a user.
func (s *Service) Clear(ctx context.Context, ownerID string) (int64, error) {
	// List to get cron job IDs first
	reminders, err := s.store.ListByOwner(ctx, ownerID, StatusPending)
	if err != nil {
		return 0, err
	}

	s.mu.RLock()
	c := s.cron
	s.mu.RUnlock()

	for _, r := range reminders {
		if r.CronJobID != "" && c != nil {
			c.DeleteJob(r.CronJobID)
		}
	}

	return s.store.DeleteByOwner(ctx, ownerID)
}

// RestorePendingReminders re-registers cron jobs for all pending reminders after restart.
func (s *Service) RestorePendingReminders(ctx context.Context) error {
	pending, err := s.store.ListPending(ctx)
	if err != nil {
		return fmt.Errorf("list pending reminders: %w", err)
	}

	s.logger.Info("restoring pending reminders", zap.Int("count", len(pending)))

	now := timeutil.NowTime()
	for _, r := range pending {
		if r.Recurring == "" && r.FireAt.Before(now) {
			// Past-due one-shot: fire immediately
			s.logger.Info("firing past-due reminder", zap.String("id", r.ID))
			s.fireReminder(ctx, r)
			continue
		}
		if err := s.scheduleCronJob(ctx, r); err != nil {
			s.logger.Warn("failed to restore cron job for reminder",
				zap.String("id", r.ID), zap.Error(err))
		}
	}

	return nil
}

// scheduleCronJob creates a cron job for a reminder.
func (s *Service) scheduleCronJob(ctx context.Context, r *Reminder) error {
	s.mu.RLock()
	c := s.cron
	s.mu.RUnlock()

	if c == nil {
		return fmt.Errorf("cron service not available")
	}

	schedule := timeToCron(r.FireAt, r.Recurring)
	payload := map[string]interface{}{
		"reminder_id": r.ID,
	}

	jobID, err := c.CreateJob(
		fmt.Sprintf("reminder:%s", r.ID),
		r.Message,
		schedule,
		"reminder",
		payload,
	)
	if err != nil {
		return err
	}

	return s.store.UpdateCronJobID(ctx, r.ID, jobID)
}

// handleReminderFired is the cron handler called when a reminder fires.
func (s *Service) handleReminderFired(ctx context.Context, payload map[string]interface{}) (interface{}, error) {
	reminderID, ok := payload["reminder_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing reminder_id in payload")
	}

	r, err := s.store.Get(ctx, reminderID)
	if err != nil {
		return nil, fmt.Errorf("get reminder: %w", err)
	}
	if r == nil || r.Status != StatusPending {
		return nil, nil // already fired or deleted
	}

	s.fireReminder(ctx, r)
	return map[string]string{"reminder_id": r.ID, "status": "fired"}, nil
}

// fireReminder executes the reminder: inject message, publish event, update status.
func (s *Service) fireReminder(ctx context.Context, r *Reminder) {
	s.logger.Info("firing reminder", zap.String("id", r.ID), zap.String("message", r.Message))

	s.mu.RLock()
	inj := s.injector
	pub := s.publisher
	c := s.cron
	s.mu.RUnlock()

	// Inject message into conversation
	var conversationID string
	if inj != nil {
		content := fmt.Sprintf("🔔 Reminder: %s", r.Message)
		var err error
		conversationID, err = inj.InjectReminderMessage(ctx, r.OwnerID, r.SessionID, content)
		if err != nil {
			s.logger.Error("failed to inject reminder message", zap.String("id", r.ID), zap.Error(err))
		}
	}

	// Publish SSE events
	if pub != nil {
		pub.Publish(r.OwnerID, "reminder", map[string]any{
			"id":      r.ID,
			"message": r.Message,
		})
		if conversationID != "" {
			pub.Publish(r.OwnerID, "conversation_updated", map[string]any{
				"id": conversationID,
			})
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

// nextOccurrence computes the next fire time for a recurring reminder.
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
