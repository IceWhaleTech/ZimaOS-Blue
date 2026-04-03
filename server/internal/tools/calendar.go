package tools

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/remindertime"
	z "github.com/IceWhaleTech/zorm"
	"github.com/google/uuid"
)

var (
	calendarEnglishClockRE = regexp.MustCompile(`(?i)\b(\d{1,2})(?::(\d{2}))?\s*(am|pm)?\b`)
	calendarChineseClockRE = regexp.MustCompile(`([零一二三四五六七八九十两0-9]{1,3})点(?:([零一二三四五六七八九十两0-9]{1,2})分?)?`)
)

// CalendarEvent is a benchmark-first local calendar event.
type CalendarEvent struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Location     string    `json:"location,omitempty"`
	Notes        string    `json:"notes,omitempty"`
	CalendarName string    `json:"calendar_name,omitempty"`
	Status       string    `json:"status,omitempty"`
	StartAt      time.Time `json:"start_at"`
	EndAt        time.Time `json:"end_at"`
	AllDay       bool      `json:"all_day"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CalendarQueryOptions captures list/search filters.
type CalendarQueryOptions struct {
	Query string
	Start *time.Time
	End   *time.Time
	Limit int
}

// CalendarStore is the persistence interface used by CalendarTool.
type CalendarStore interface {
	List(ctx context.Context, ownerID string, opts CalendarQueryOptions) ([]CalendarEvent, error)
	Get(ctx context.Context, ownerID, id string) (*CalendarEvent, error)
	Create(ctx context.Context, ownerID string, event CalendarEvent) (*CalendarEvent, error)
	Today(ctx context.Context, ownerID string, day time.Time) ([]CalendarEvent, error)
}

// LocalCalendarService stores events in SQLite and seeds an agenda dataset per user.
type LocalCalendarService struct {
	db     *sql.DB
	readDB *sql.DB
	now    func() time.Time
}

// NewLocalCalendarService creates the event table.
func NewLocalCalendarService(db *sql.DB) (*LocalCalendarService, error) {
	return NewLocalCalendarServiceWithReadDB(db, db)
}

// NewLocalCalendarServiceWithReadDB creates the event table with separate
// write and read database handles.
func NewLocalCalendarServiceWithReadDB(writeDB, readDB *sql.DB) (*LocalCalendarService, error) {
	if writeDB == nil {
		return nil, errors.New("calendar database is required")
	}
	if readDB == nil {
		readDB = writeDB
	}
	if _, err := writeDB.Exec(`CREATE TABLE IF NOT EXISTS tool_calendar_events (
		id             TEXT PRIMARY KEY,
		owner_id       TEXT NOT NULL,
		title          TEXT NOT NULL,
		location       TEXT NOT NULL DEFAULT '',
		notes          TEXT NOT NULL DEFAULT '',
		calendar_name  TEXT NOT NULL DEFAULT 'Default',
		status         TEXT NOT NULL DEFAULT 'confirmed',
		start_at       TEXT NOT NULL,
		end_at         TEXT NOT NULL,
		all_day        INTEGER NOT NULL DEFAULT 0,
		created_at     TEXT NOT NULL,
		updated_at     TEXT NOT NULL
	)`); err != nil {
		return nil, err
	}
	if _, err := writeDB.Exec(`CREATE INDEX IF NOT EXISTS idx_tool_calendar_owner_start ON tool_calendar_events(owner_id, start_at)`); err != nil {
		return nil, err
	}
	return &LocalCalendarService{
		db:     writeDB,
		readDB: readDB,
		now:    func() time.Time { return time.Now() },
	}, nil
}

func (s *LocalCalendarService) reader() *sql.DB {
	if s != nil && s.readDB != nil {
		return s.readDB
	}
	if s == nil {
		return nil
	}
	return s.db
}

func (s *LocalCalendarService) table(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.db, "tool_calendar_events")
}

func (s *LocalCalendarService) readTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.reader(), "tool_calendar_events")
}

type calendarEventRow struct {
	ID           string `json:"id" zorm:"id"`
	Title        string `json:"title" zorm:"title"`
	Location     string `json:"location" zorm:"location"`
	Notes        string `json:"notes" zorm:"notes"`
	CalendarName string `json:"calendar_name" zorm:"calendar_name"`
	Status       string `json:"status" zorm:"status"`
	StartAt      string `json:"start_at" zorm:"start_at"`
	EndAt        string `json:"end_at" zorm:"end_at"`
	AllDay       int    `json:"all_day" zorm:"all_day"`
	CreatedAt    string `json:"created_at" zorm:"created_at"`
	UpdatedAt    string `json:"updated_at" zorm:"updated_at"`
}

// SetNowFunc overrides the clock for deterministic tests.
func (s *LocalCalendarService) SetNowFunc(fn func() time.Time) {
	if s == nil || fn == nil {
		return
	}
	s.now = fn
}

// SeedFixtures replaces an owner's events with the provided fixture set.
func (s *LocalCalendarService) SeedFixtures(ctx context.Context, ownerID string, fixtures []CalendarEvent) error {
	if s == nil {
		return errors.New("calendar service not configured")
	}
	ownerID = normalizeProductivityOwnerID(ownerID)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	eventTable := z.TableContext(ctx, tx, "tool_calendar_events")
	if _, err := eventTable.Delete(z.Where(z.Eq("owner_id", ownerID))); err != nil {
		return err
	}
	for _, event := range fixtures {
		if err := insertCalendarEvent(ctx, tx, ownerID, event); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *LocalCalendarService) ensureSeeded(ctx context.Context, ownerID string) error {
	ownerID = normalizeProductivityOwnerID(ownerID)
	var count int64
	if _, err := s.readTable(ctx).Select(&count,
		z.Fields("count(1)"),
		z.Where(z.Eq("owner_id", ownerID)),
	); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, event := range defaultCalendarFixtures(s.now()) {
		if err := insertCalendarEvent(ctx, tx, ownerID, event); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func insertCalendarEvent(ctx context.Context, tx *sql.Tx, ownerID string, event CalendarEvent) error {
	if strings.TrimSpace(event.ID) == "" {
		event.ID = uuid.NewString()
	}
	if strings.TrimSpace(event.CalendarName) == "" {
		event.CalendarName = "Default"
	}
	if strings.TrimSpace(event.Status) == "" {
		event.Status = "confirmed"
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = event.StartAt
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	if event.UpdatedAt.IsZero() {
		event.UpdatedAt = event.CreatedAt
	}
	_, err := z.TableContext(ctx, tx, "tool_calendar_events").Insert(z.V{
		"id":            event.ID,
		"owner_id":      ownerID,
		"title":         strings.TrimSpace(event.Title),
		"location":      strings.TrimSpace(event.Location),
		"notes":         strings.TrimSpace(event.Notes),
		"calendar_name": strings.TrimSpace(event.CalendarName),
		"status":        strings.TrimSpace(event.Status),
		"start_at":      event.StartAt.UTC().Format(time.RFC3339),
		"end_at":        event.EndAt.UTC().Format(time.RFC3339),
		"all_day":       boolToInt(event.AllDay),
		"created_at":    event.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at":    event.UpdatedAt.UTC().Format(time.RFC3339),
	})
	return err
}

// List returns events for the user after applying filters.
func (s *LocalCalendarService) List(ctx context.Context, ownerID string, opts CalendarQueryOptions) ([]CalendarEvent, error) {
	if s == nil {
		return nil, errors.New("calendar service not configured")
	}
	ownerID = normalizeProductivityOwnerID(ownerID)
	if err := s.ensureSeeded(ctx, ownerID); err != nil {
		return nil, err
	}
	var rows []calendarEventRow
	_, err := s.readTable(ctx).Select(&rows,
		z.Where(z.Eq("owner_id", ownerID)),
		z.OrderBy("start_at ASC", "title ASC"),
	)
	if err != nil {
		return nil, err
	}

	result := make([]CalendarEvent, 0, 8)
	for i := range rows {
		event := rowToCalendarEvent(rows[i])
		if !calendarMatchesQuery(event, opts) {
			continue
		}
		result = append(result, event)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].StartAt.Equal(result[j].StartAt) {
			return result[i].Title < result[j].Title
		}
		return result[i].StartAt.Before(result[j].StartAt)
	})
	limit := opts.Limit
	if limit <= 0 {
		limit = 10
	}
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

// Get fetches an event by ID.
func (s *LocalCalendarService) Get(ctx context.Context, ownerID, id string) (*CalendarEvent, error) {
	if s == nil {
		return nil, errors.New("calendar service not configured")
	}
	ownerID = normalizeProductivityOwnerID(ownerID)
	if err := s.ensureSeeded(ctx, ownerID); err != nil {
		return nil, err
	}
	var rows []calendarEventRow
	_, err := s.readTable(ctx).Select(&rows,
		z.Where(z.Eq("owner_id", ownerID), z.Eq("id", strings.TrimSpace(id))),
		z.Limit(1),
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	event := rowToCalendarEvent(rows[0])
	return &event, nil
}

// Create inserts a new event and returns the stored row.
func (s *LocalCalendarService) Create(ctx context.Context, ownerID string, event CalendarEvent) (*CalendarEvent, error) {
	if s == nil {
		return nil, errors.New("calendar service not configured")
	}
	ownerID = normalizeProductivityOwnerID(ownerID)
	if err := s.ensureSeeded(ctx, ownerID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(event.ID) == "" {
		event.ID = uuid.NewString()
	}
	now := s.now()
	if event.CreatedAt.IsZero() {
		event.CreatedAt = now
	}
	if event.UpdatedAt.IsZero() {
		event.UpdatedAt = now
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := insertCalendarEvent(ctx, tx, ownerID, event); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.Get(ctx, ownerID, event.ID)
}

// Today returns events overlapping the supplied day.
func (s *LocalCalendarService) Today(ctx context.Context, ownerID string, day time.Time) ([]CalendarEvent, error) {
	start, end := dayBounds(day)
	return s.List(ctx, ownerID, CalendarQueryOptions{Start: &start, End: &end, Limit: 20})
}

type calendarScanner interface {
	Scan(dest ...interface{}) error
}

func rowToCalendarEvent(row calendarEventRow) CalendarEvent {
	event := CalendarEvent{
		ID:           row.ID,
		Title:        row.Title,
		Location:     row.Location,
		Notes:        row.Notes,
		CalendarName: row.CalendarName,
		Status:       row.Status,
		AllDay:       row.AllDay != 0,
	}
	event.StartAt, _ = time.Parse(time.RFC3339, row.StartAt)
	event.EndAt, _ = time.Parse(time.RFC3339, row.EndAt)
	event.CreatedAt, _ = time.Parse(time.RFC3339, row.CreatedAt)
	event.UpdatedAt, _ = time.Parse(time.RFC3339, row.UpdatedAt)
	return event
}

func scanCalendarEvent(scanner calendarScanner) (CalendarEvent, error) {
	var event CalendarEvent
	var startAt string
	var endAt string
	var allDayInt int
	var createdAt string
	var updatedAt string
	err := scanner.Scan(
		&event.ID,
		&event.Title,
		&event.Location,
		&event.Notes,
		&event.CalendarName,
		&event.Status,
		&startAt,
		&endAt,
		&allDayInt,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return CalendarEvent{}, err
	}
	event.StartAt, _ = time.Parse(time.RFC3339, startAt)
	event.EndAt, _ = time.Parse(time.RFC3339, endAt)
	event.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	event.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	event.AllDay = allDayInt != 0
	return event, nil
}

func calendarMatchesQuery(event CalendarEvent, opts CalendarQueryOptions) bool {
	if opts.Start != nil && event.EndAt.Before(*opts.Start) {
		return false
	}
	if opts.End != nil && event.StartAt.After(*opts.End) {
		return false
	}
	if query := strings.ToLower(strings.TrimSpace(opts.Query)); query != "" {
		haystack := strings.ToLower(strings.Join([]string{
			event.Title,
			event.Location,
			event.Notes,
			event.CalendarName,
		}, " "))
		if !strings.Contains(haystack, query) {
			return false
		}
	}
	return true
}

func defaultCalendarFixtures(now time.Time) []CalendarEvent {
	loc := now.Location()
	year, month, day := now.Date()
	todayMorning := time.Date(year, month, day, 9, 30, 0, 0, loc)
	todayAfternoon := time.Date(year, month, day, 15, 0, 0, 0, loc)
	tomorrowEvening := time.Date(year, month, day+1, 19, 0, 0, 0, loc)
	nextWeek := time.Date(year, month, day+5, 11, 0, 0, 0, loc)
	return []CalendarEvent{
		{
			Title:        "Team standup",
			Location:     "Focus room A",
			Notes:        "Share blockers, priorities, and release status.",
			CalendarName: "Work",
			Status:       "confirmed",
			StartAt:      todayMorning,
			EndAt:        todayMorning.Add(30 * time.Minute),
			AllDay:       false,
			CreatedAt:    todayMorning.Add(-24 * time.Hour),
			UpdatedAt:    todayMorning.Add(-24 * time.Hour),
		},
		{
			Title:        "Quarterly planning review",
			Location:     "Horizon room",
			Notes:        "Updated from the latest calendar email. Bring the roadmap deck.",
			CalendarName: "Work",
			Status:       "confirmed",
			StartAt:      todayAfternoon,
			EndAt:        todayAfternoon.Add(1 * time.Hour),
			AllDay:       false,
			CreatedAt:    todayAfternoon.Add(-48 * time.Hour),
			UpdatedAt:    todayAfternoon.Add(-2 * time.Hour),
		},
		{
			Title:        "Dinner with family",
			Location:     "Lantern House",
			Notes:        "Book a table for four if needed.",
			CalendarName: "Personal",
			Status:       "confirmed",
			StartAt:      tomorrowEvening,
			EndAt:        tomorrowEvening.Add(90 * time.Minute),
			AllDay:       false,
			CreatedAt:    now.Add(-72 * time.Hour),
			UpdatedAt:    now.Add(-72 * time.Hour),
		},
		{
			Title:        "Dentist appointment",
			Location:     "Smile Clinic",
			Notes:        "Bring insurance card.",
			CalendarName: "Personal",
			Status:       "confirmed",
			StartAt:      nextWeek,
			EndAt:        nextWeek.Add(45 * time.Minute),
			AllDay:       false,
			CreatedAt:    now.Add(-7 * 24 * time.Hour),
			UpdatedAt:    now.Add(-7 * 24 * time.Hour),
		},
	}
}

// CalendarTool provides local event creation, lookup, and daily summaries.
type CalendarTool struct {
	service     CalendarStore
	email       EmailService
	reminderSvc PushServiceInterface
	cronSvc     CronService
	now         func() time.Time
}

// NewCalendarTool creates a native calendar tool.
func NewCalendarTool(service CalendarStore) *CalendarTool {
	return &CalendarTool{
		service: service,
		now:     func() time.Time { return time.Now() },
	}
}

// SetEmailService wires inbox context into daily_summary.
func (t *CalendarTool) SetEmailService(service EmailService) {
	if t != nil {
		t.email = service
	}
}

// SetReminderService wires reminder context into daily_summary.
func (t *CalendarTool) SetReminderService(service PushServiceInterface) {
	if t != nil {
		t.reminderSvc = service
	}
}

// SetCronService wires scheduled job context into daily_summary.
func (t *CalendarTool) SetCronService(service CronService) {
	if t != nil {
		t.cronSvc = service
	}
}

// SetNowFunc overrides the clock, mainly for tests.
func (t *CalendarTool) SetNowFunc(fn func() time.Time) {
	if t != nil && fn != nil {
		t.now = fn
	}
}

// Definition returns the tool schema.
func (t *CalendarTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "calendar",
		Description: "Create, inspect, search, and summarize benchmark-first calendar events. Also produces a daily summary that combines agenda, reminders, scheduled jobs, and priority emails.",
		Icon:        "calendar",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"create", "list", "get", "search", "today", "daily_summary"},
					"description": "Calendar action. Defaults to list, get, or search based on provided arguments.",
				},
				"id": map[string]interface{}{
					"type":        "string",
					"description": "Event ID for get.",
				},
				"title": map[string]interface{}{
					"type":        "string",
					"description": "Event title for create.",
				},
				"time": map[string]interface{}{
					"type":        "string",
					"description": "Natural language or RFC3339 start time for create (e.g. tomorrow 9am).",
				},
				"start": map[string]interface{}{
					"type":        "string",
					"description": "Alias for time/start_at.",
				},
				"end": map[string]interface{}{
					"type":        "string",
					"description": "Optional end time for create.",
				},
				"duration": map[string]interface{}{
					"type":        "string",
					"description": "Optional duration for create when end is omitted (default 1h).",
				},
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query for titles, locations, and notes.",
				},
				"date": map[string]interface{}{
					"type":        "string",
					"description": "Optional date window anchor for list/today/daily_summary.",
				},
				"from": map[string]interface{}{
					"type":        "string",
					"description": "Window start for list/search.",
				},
				"to": map[string]interface{}{
					"type":        "string",
					"description": "Window end for list/search.",
				},
				"location": map[string]interface{}{
					"type":        "string",
					"description": "Event location for create.",
				},
				"notes": map[string]interface{}{
					"type":        "string",
					"description": "Event notes for create.",
				},
				"calendar": map[string]interface{}{
					"type":        "string",
					"description": "Calendar name for create (default: Default).",
				},
				"all_day": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether the created event is an all-day event.",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum events to return (default 10).",
				},
			},
		},
	}
}

// Execute dispatches the selected calendar action.
func (t *CalendarTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil || t.service == nil {
		return nil, errors.New("calendar service not available")
	}
	action := normalizeCalendarAction(firstCompatString(args, "action", "op", "operation", "command"), args)
	ownerID := normalizeProductivityOwnerID(GetUserID(ctx))
	lang := GetLang(ctx)

	switch action {
	case "create":
		title := strings.TrimSpace(firstCompatString(args, "title", "name"))
		if title == "" {
			return nil, errors.New("title is required")
		}
		startAt, endAt, allDay, err := calendarTimesFromArgs(args, t.now())
		if err != nil {
			return nil, err
		}
		event, err := t.service.Create(ctx, ownerID, CalendarEvent{
			Title:        title,
			Location:     strings.TrimSpace(firstCompatString(args, "location")),
			Notes:        strings.TrimSpace(firstCompatString(args, "notes", "description")),
			CalendarName: strings.TrimSpace(firstCompatString(args, "calendar", "calendar_name", "calendarName")),
			Status:       "confirmed",
			StartAt:      startAt,
			EndAt:        endAt,
			AllDay:       allDay,
		})
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"status":  "success",
			"message": calendarLocalized(lang, fmt.Sprintf("Created calendar event: %s", title), fmt.Sprintf("已创建日历事件：%s", title)),
			"event":   calendarEventForResult(*event),
		}, nil
	case "list", "search":
		opts, err := calendarQueryOptionsFromArgs(args, t.now())
		if err != nil {
			return nil, err
		}
		events, err := t.service.List(ctx, ownerID, opts)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"status":  "success",
			"message": calendarLocalized(lang, fmt.Sprintf("Calendar results: %d event(s).", len(events)), fmt.Sprintf("日历结果：%d 个事件。", len(events))),
			"count":   len(events),
			"events":  calendarEventsForResult(events),
		}, nil
	case "get":
		id := firstCompatString(args, "id", "event_id", "eventId")
		if id == "" {
			return nil, errors.New("id is required")
		}
		event, err := t.service.Get(ctx, ownerID, id)
		if err != nil {
			return nil, err
		}
		if event == nil {
			return nil, errors.New("calendar event not found")
		}
		return map[string]interface{}{
			"status":  "success",
			"message": calendarLocalized(lang, fmt.Sprintf("Opened event: %s", event.Title), fmt.Sprintf("已打开事件：%s", event.Title)),
			"event":   calendarEventForResult(*event),
		}, nil
	case "today":
		anchor, err := calendarAnchorDate(args, t.now())
		if err != nil {
			return nil, err
		}
		events, err := t.service.Today(ctx, ownerID, anchor)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"status":  "success",
			"message": calendarLocalized(lang, fmt.Sprintf("Today's agenda: %d event(s).", len(events)), fmt.Sprintf("今日议程：%d 个事件。", len(events))),
			"count":   len(events),
			"events":  calendarEventsForResult(events),
		}, nil
	case "daily_summary":
		return t.dailySummary(ctx, ownerID, args, lang)
	default:
		return nil, fmt.Errorf("unsupported calendar action %q", action)
	}
}

func (t *CalendarTool) dailySummary(ctx context.Context, ownerID string, args map[string]interface{}, lang string) (interface{}, error) {
	anchor, err := calendarAnchorDate(args, t.now())
	if err != nil {
		return nil, err
	}
	events, err := t.service.Today(ctx, ownerID, anchor)
	if err != nil {
		return nil, err
	}
	reminders := t.collectDailyReminders(ctx, ownerID, anchor)
	jobs := t.collectDailyJobs(ctx, ownerID, anchor)
	emails := t.collectPriorityEmails(ctx, ownerID)
	dateLabel := anchor.Format("2006-01-02")
	return map[string]interface{}{
		"status":           "success",
		"message":          calendarLocalized(lang, fmt.Sprintf("Daily summary for %s: %d event(s), %d reminder(s), %d scheduled job(s), %d priority email(s).", dateLabel, len(events), len(reminders), len(jobs), len(emails)), fmt.Sprintf("%s 的每日摘要：%d 个事件，%d 个提醒，%d 个计划任务，%d 封重点邮件。", dateLabel, len(events), len(reminders), len(jobs), len(emails))),
		"summary":          dailySummaryText(lang, anchor, events, reminders, jobs, emails),
		"date":             dateLabel,
		"events":           calendarEventsForResult(events),
		"reminders":        reminders,
		"scheduled_jobs":   jobs,
		"priority_emails":  emailMessagesForResult(emails, false),
		"focus_item_count": len(events) + len(reminders) + len(jobs) + len(emails),
	}, nil
}

func (t *CalendarTool) collectDailyReminders(ctx context.Context, ownerID string, anchor time.Time) []map[string]interface{} {
	if t == nil || t.reminderSvc == nil {
		return nil
	}
	list, err := t.reminderSvc.List(ctx, ownerID)
	if err != nil {
		return nil
	}
	start, end := dayBounds(anchor)
	result := make([]map[string]interface{}, 0, len(list))
	for _, reminder := range list {
		if reminder.FireAt.Before(start) || reminder.FireAt.After(end) {
			continue
		}
		result = append(result, map[string]interface{}{
			"id":        reminder.ID,
			"message":   reminder.Message,
			"fire_at":   reminder.FireAt.UTC().Format(time.RFC3339),
			"status":    reminder.Status,
			"recurring": reminder.Recurring,
		})
	}
	if len(result) > 5 {
		result = result[:5]
	}
	return result
}

func (t *CalendarTool) collectDailyJobs(ctx context.Context, ownerID string, anchor time.Time) []map[string]interface{} {
	if t == nil || t.cronSvc == nil {
		return nil
	}
	jobs, err := t.cronSvc.ListJobs(ctx)
	if err != nil {
		return nil
	}
	start, end := dayBounds(anchor)
	result := make([]map[string]interface{}, 0, len(jobs))
	for _, job := range jobs {
		if !job.Enabled || job.NextRunAt == nil {
			continue
		}
		if next := job.NextRunAt.UTC(); next.Before(start.UTC()) || next.After(end.UTC()) {
			continue
		}
		if userID, _ := job.Payload["user_id"].(string); strings.TrimSpace(userID) != "" && userID != ownerID {
			continue
		}
		result = append(result, map[string]interface{}{
			"id":          job.ID,
			"name":        job.Name,
			"schedule":    job.Schedule,
			"next_run_at": job.NextRunAt.UTC().Format(time.RFC3339),
			"handler":     job.Handler,
		})
	}
	if len(result) > 5 {
		result = result[:5]
	}
	return result
}

func (t *CalendarTool) collectPriorityEmails(ctx context.Context, ownerID string) []EmailMessage {
	if t == nil || t.email == nil {
		return nil
	}
	unread := true
	messages, err := t.email.List(ctx, ownerID, EmailQueryOptions{
		Priority: "high",
		Unread:   &unread,
		Limit:    5,
	})
	if err != nil {
		return nil
	}
	return messages
}

func RegisterCalendarTool(registry *Registry, service CalendarStore) *CalendarTool {
	if registry == nil || service == nil {
		return nil
	}
	tool := NewCalendarTool(service)
	registry.Register(tool)
	return tool
}

func GetCalendarTool(registry *Registry) *CalendarTool {
	if registry == nil {
		return nil
	}
	tool := registry.Get("calendar")
	if tool == nil {
		return nil
	}
	calendarTool, _ := tool.(*CalendarTool)
	return calendarTool
}

func normalizeCalendarAction(raw string, args map[string]interface{}) string {
	action := strings.ToLower(strings.TrimSpace(raw))
	switch action {
	case "add", "new", "create_event", "book":
		return "create"
	case "open", "show", "detail":
		return "get"
	case "agenda", "upcoming":
		return "list"
	case "summary", "digest", "daily-summary", "daily summary":
		return "daily_summary"
	}
	switch action {
	case "", "create", "list", "get", "search", "today", "daily_summary":
	default:
		return action
	}
	if action != "" {
		return action
	}
	if strings.TrimSpace(firstCompatString(args, "title", "name")) != "" && strings.TrimSpace(firstCompatString(args, "time", "start", "start_at", "startAt", "when")) != "" {
		return "create"
	}
	if strings.TrimSpace(firstCompatString(args, "id", "event_id", "eventId")) != "" {
		return "get"
	}
	if strings.TrimSpace(firstCompatString(args, "query", "q", "search")) != "" {
		return "search"
	}
	return "list"
}

func calendarQueryOptionsFromArgs(args map[string]interface{}, now time.Time) (CalendarQueryOptions, error) {
	opts := CalendarQueryOptions{
		Query: strings.TrimSpace(firstCompatString(args, "query", "q", "search")),
		Limit: compatInt(args, "limit", "max_results", "maxResults"),
	}
	if fromStr := strings.TrimSpace(firstCompatString(args, "from", "start", "date")); fromStr != "" {
		from, err := parseCalendarTimeInput(fromStr, now, nil)
		if err != nil {
			return CalendarQueryOptions{}, err
		}
		opts.Start = &from
	}
	if toStr := strings.TrimSpace(firstCompatString(args, "to", "end")); toStr != "" {
		to, err := parseCalendarTimeInput(toStr, now, opts.Start)
		if err != nil {
			return CalendarQueryOptions{}, err
		}
		opts.End = &to
	}
	return opts, nil
}

func calendarTimesFromArgs(args map[string]interface{}, now time.Time) (time.Time, time.Time, bool, error) {
	allDay := compatArgBool(args, "all_day", "allDay")
	startRaw := strings.TrimSpace(firstCompatString(args, "time", "start", "start_at", "startAt", "when", "date"))
	if startRaw == "" {
		return time.Time{}, time.Time{}, false, errors.New("time/start is required")
	}
	startAt, err := parseCalendarTimeInput(startRaw, now, nil)
	if err != nil {
		return time.Time{}, time.Time{}, false, err
	}
	var endAt time.Time
	if endRaw := strings.TrimSpace(firstCompatString(args, "end", "end_at", "endAt")); endRaw != "" {
		endAt, err = parseCalendarTimeInput(endRaw, now, &startAt)
		if err != nil {
			return time.Time{}, time.Time{}, false, err
		}
	} else if durationRaw := strings.TrimSpace(firstCompatString(args, "duration")); durationRaw != "" {
		duration, durationErr := remindertime.ParseDuration(durationRaw)
		if durationErr != nil {
			return time.Time{}, time.Time{}, false, durationErr
		}
		endAt = startAt.Add(duration)
	} else if allDay {
		endAt = startAt.Add(24 * time.Hour)
	} else {
		endAt = startAt.Add(1 * time.Hour)
	}
	if !endAt.After(startAt) {
		return time.Time{}, time.Time{}, false, errors.New("end must be after start")
	}
	return startAt, endAt, allDay, nil
}

func calendarAnchorDate(args map[string]interface{}, fallback time.Time) (time.Time, error) {
	if raw := strings.TrimSpace(firstCompatString(args, "date")); raw != "" {
		parsed, err := parseCalendarTimeInput(raw, fallback, nil)
		if err != nil {
			return time.Time{}, err
		}
		return parsed, nil
	}
	return fallback, nil
}

func calendarEventForResult(event CalendarEvent) map[string]interface{} {
	return map[string]interface{}{
		"id":            event.ID,
		"title":         event.Title,
		"location":      event.Location,
		"notes":         event.Notes,
		"calendar_name": event.CalendarName,
		"status":        event.Status,
		"start_at":      event.StartAt.UTC().Format(time.RFC3339),
		"end_at":        event.EndAt.UTC().Format(time.RFC3339),
		"all_day":       event.AllDay,
	}
}

func calendarEventsForResult(events []CalendarEvent) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(events))
	for _, event := range events {
		result = append(result, calendarEventForResult(event))
	}
	return result
}

func dailySummaryText(lang string, anchor time.Time, events []CalendarEvent, reminders []map[string]interface{}, jobs []map[string]interface{}, emails []EmailMessage) string {
	parts := make([]string, 0, 4)
	if len(events) > 0 {
		parts = append(parts, calendarLocalized(lang, "Today's agenda: ", "今日议程：")+strings.Join(formatEventHighlights(events, anchor.Location()), "; "))
	}
	if len(reminders) > 0 {
		parts = append(parts, calendarLocalized(lang, "Reminders: ", "提醒：")+strings.Join(formatReminderHighlights(reminders), "; "))
	}
	if len(jobs) > 0 {
		parts = append(parts, calendarLocalized(lang, "Scheduled jobs: ", "计划任务：")+strings.Join(formatJobHighlights(jobs), "; "))
	}
	if len(emails) > 0 {
		parts = append(parts, calendarLocalized(lang, "Priority inbox: ", "重点邮件：")+strings.Join(formatEmailHighlights(emails), "; "))
	}
	if len(parts) == 0 {
		return calendarLocalized(lang, "Nothing urgent is scheduled today.", "今天没有紧急安排。")
	}
	return strings.Join(parts, "\n")
}

func formatEventHighlights(events []CalendarEvent, loc *time.Location) []string {
	result := make([]string, 0, len(events))
	for _, event := range events {
		label := event.Title
		if event.AllDay {
			label = "All day — " + label
		} else {
			label = event.StartAt.In(loc).Format("15:04") + " — " + label
		}
		result = append(result, label)
	}
	return result
}

func formatReminderHighlights(reminders []map[string]interface{}) []string {
	result := make([]string, 0, len(reminders))
	for _, reminder := range reminders {
		text := strings.TrimSpace(fmt.Sprintf("%v", reminder["message"]))
		if text != "" {
			result = append(result, text)
		}
	}
	return result
}

func formatJobHighlights(jobs []map[string]interface{}) []string {
	result := make([]string, 0, len(jobs))
	for _, job := range jobs {
		name := strings.TrimSpace(fmt.Sprintf("%v", job["name"]))
		if name == "" {
			continue
		}
		result = append(result, name)
	}
	return result
}

func formatEmailHighlights(emails []EmailMessage) []string {
	result := make([]string, 0, len(emails))
	for _, msg := range emails {
		result = append(result, formatEmailHighlight(msg))
	}
	return result
}

func dayBounds(anchor time.Time) (time.Time, time.Time) {
	year, month, day := anchor.Date()
	loc := anchor.Location()
	start := time.Date(year, month, day, 0, 0, 0, 0, loc)
	end := start.Add(24*time.Hour - time.Nanosecond)
	return start, end
}

func calendarLocalized(lang, en, zh string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(lang)), "zh") {
		return zh
	}
	return en
}

func parseCalendarTimeInput(raw string, now time.Time, anchor *time.Time) (time.Time, error) {
	if shouldPreferNaturalCalendarTime(raw, anchor) {
		if parsed, ok := parseNaturalCalendarTime(raw, now, anchor); ok {
			return parsed, nil
		}
	}
	if parsed, err := remindertime.Parse(raw); err == nil {
		return parsed, nil
	}
	if parsed, ok := parseNaturalCalendarTime(raw, now, anchor); ok {
		return parsed, nil
	}
	return time.Time{}, fmt.Errorf("invalid time format: %s", raw)
}

func shouldPreferNaturalCalendarTime(raw string, anchor *time.Time) bool {
	if anchor != nil && !anchor.IsZero() {
		return true
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	return strings.Contains(lower, "today") ||
		strings.Contains(lower, "tomorrow") ||
		strings.Contains(lower, "next week") ||
		strings.Contains(lower, "morning") ||
		strings.Contains(lower, "afternoon") ||
		strings.Contains(lower, "evening") ||
		strings.Contains(lower, "tonight") ||
		strings.Contains(lower, "noon") ||
		strings.Contains(trimmed, "今天") ||
		strings.Contains(trimmed, "明天") ||
		strings.Contains(trimmed, "下周") ||
		strings.Contains(trimmed, "上午") ||
		strings.Contains(trimmed, "下午") ||
		strings.Contains(trimmed, "晚上") ||
		strings.Contains(trimmed, "今晚") ||
		strings.Contains(trimmed, "早上") ||
		strings.Contains(trimmed, "中午")
}

func parseNaturalCalendarTime(raw string, now time.Time, anchor *time.Time) (time.Time, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, false
	}
	lower := strings.ToLower(trimmed)
	base := now
	if anchor != nil && !anchor.IsZero() {
		base = *anchor
	}
	year, month, day := base.Date()
	loc := base.Location()
	targetDay := time.Date(year, month, day, 0, 0, 0, 0, loc)

	explicitDay := false
	switch {
	case strings.Contains(lower, "tomorrow") || strings.Contains(trimmed, "明天"):
		targetDay = targetDay.Add(24 * time.Hour)
		explicitDay = true
	case strings.Contains(lower, "today") || strings.Contains(trimmed, "今天") || strings.Contains(trimmed, "今日"):
		explicitDay = true
	case strings.Contains(lower, "next week") || strings.Contains(trimmed, "下周"):
		targetDay = targetDay.Add(7 * 24 * time.Hour)
		explicitDay = true
	}

	hour, minute, hasClock := extractCalendarClock(trimmed)
	if !hasClock {
		switch {
		case strings.Contains(lower, "tonight") || strings.Contains(lower, "evening") || strings.Contains(trimmed, "今晚") || strings.Contains(trimmed, "晚上"):
			hour = 19
			hasClock = true
		case strings.Contains(lower, "afternoon") || strings.Contains(trimmed, "下午"):
			hour = 15
			hasClock = true
		case strings.Contains(lower, "morning") || strings.Contains(trimmed, "上午") || strings.Contains(trimmed, "早上"):
			hour = 9
			hasClock = true
		case strings.Contains(lower, "noon") || strings.Contains(trimmed, "中午"):
			hour = 12
			hasClock = true
		}
	}
	if !hasClock {
		return time.Time{}, false
	}
	if !explicitDay && anchor == nil && !containsAnyCalendarTimeCue(lower, trimmed) {
		return time.Time{}, false
	}
	return time.Date(targetDay.Year(), targetDay.Month(), targetDay.Day(), hour, minute, 0, 0, loc), true
}

func containsAnyCalendarTimeCue(lower, raw string) bool {
	return strings.Contains(lower, "am") || strings.Contains(lower, "pm") ||
		strings.Contains(lower, "morning") || strings.Contains(lower, "afternoon") ||
		strings.Contains(lower, "evening") || strings.Contains(lower, "tonight") ||
		strings.Contains(lower, "today") || strings.Contains(lower, "tomorrow") ||
		strings.Contains(lower, "next week") ||
		strings.Contains(raw, "上午") || strings.Contains(raw, "下午") ||
		strings.Contains(raw, "晚上") || strings.Contains(raw, "今晚") ||
		strings.Contains(raw, "早上") || strings.Contains(raw, "中午") ||
		strings.Contains(raw, "今天") || strings.Contains(raw, "明天") || strings.Contains(raw, "下周")
}

func extractCalendarClock(raw string) (int, int, bool) {
	lower := strings.ToLower(strings.TrimSpace(raw))
	if matches := calendarEnglishClockRE.FindStringSubmatch(lower); len(matches) == 4 {
		hour, err := strconv.Atoi(matches[1])
		if err != nil {
			return 0, 0, false
		}
		minute := 0
		if matches[2] != "" {
			minute, err = strconv.Atoi(matches[2])
			if err != nil {
				return 0, 0, false
			}
		}
		switch strings.ToLower(strings.TrimSpace(matches[3])) {
		case "pm":
			if hour < 12 {
				hour += 12
			}
		case "am":
			if hour == 12 {
				hour = 0
			}
		default:
			hour = applyCalendarPeriodCues(hour, raw)
		}
		if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
			return 0, 0, false
		}
		return hour, minute, true
	}
	if matches := calendarChineseClockRE.FindStringSubmatch(strings.TrimSpace(raw)); len(matches) == 3 {
		hour, ok := parseCalendarNumberish(matches[1])
		if !ok {
			return 0, 0, false
		}
		minute := 0
		if matches[2] != "" {
			minute, ok = parseCalendarNumberish(matches[2])
			if !ok {
				return 0, 0, false
			}
		}
		hour = applyCalendarPeriodCues(hour, raw)
		if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
			return 0, 0, false
		}
		return hour, minute, true
	}
	return 0, 0, false
}

func applyCalendarPeriodCues(hour int, raw string) int {
	lower := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case strings.Contains(lower, "pm") || strings.Contains(lower, "afternoon") || strings.Contains(lower, "evening") || strings.Contains(lower, "tonight") ||
		strings.Contains(raw, "下午") || strings.Contains(raw, "晚上") || strings.Contains(raw, "今晚") || strings.Contains(raw, "傍晚"):
		if hour < 12 {
			return hour + 12
		}
	case strings.Contains(lower, "am") || strings.Contains(lower, "morning") || strings.Contains(raw, "上午") || strings.Contains(raw, "早上"):
		if hour == 12 {
			return 0
		}
	case strings.Contains(lower, "noon") || strings.Contains(raw, "中午"):
		if hour < 11 {
			return hour + 12
		}
	}
	return hour
}

func parseCalendarNumberish(raw string) (int, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, false
	}
	if value, err := strconv.Atoi(trimmed); err == nil {
		return value, true
	}
	digits := map[rune]int{
		'零': 0,
		'一': 1,
		'二': 2,
		'两': 2,
		'三': 3,
		'四': 4,
		'五': 5,
		'六': 6,
		'七': 7,
		'八': 8,
		'九': 9,
	}
	if trimmed == "十" {
		return 10, true
	}
	runes := []rune(trimmed)
	if strings.HasPrefix(trimmed, "十") && len(runes) == 2 {
		ones, ok := digits[runes[1]]
		return 10 + ones, ok
	}
	if strings.HasSuffix(trimmed, "十") && len(runes) == 2 {
		tens, ok := digits[runes[0]]
		return tens * 10, ok
	}
	if parts := strings.Split(trimmed, "十"); len(parts) == 2 && parts[0] != "" && parts[1] != "" {
		tensRunes := []rune(parts[0])
		onesRunes := []rune(parts[1])
		if len(tensRunes) == 1 && len(onesRunes) == 1 {
			tens, okTens := digits[tensRunes[0]]
			ones, okOnes := digits[onesRunes[0]]
			if okTens && okOnes {
				return tens*10 + ones, true
			}
		}
	}
	if len(runes) == 1 {
		value, ok := digits[runes[0]]
		return value, ok
	}
	return 0, false
}
