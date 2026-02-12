package builtin

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
)

// CalendarEvent represents a calendar event
type CalendarEvent struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Location    string    `json:"location,omitempty"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
	AllDay      bool      `json:"all_day"`
	Recurring   string    `json:"recurring,omitempty"` // daily, weekly, monthly, yearly
	Reminders   []int     `json:"reminders,omitempty"` // minutes before event
	Created     time.Time `json:"created"`
	Updated     time.Time `json:"updated"`
}

// CalendarConfig holds calendar configuration
type CalendarConfig struct {
	CalDAVURL      string `json:"caldav_url"`
	CalDAVUser     string `json:"caldav_user"`
	CalDAVPassword string `json:"caldav_password"`
}

// Calendar is a built-in calendar skill
type Calendar struct {
	manifest *skill.Manifest
	config   *CalendarConfig
	events   map[string]*CalendarEvent
	mu       sync.RWMutex
	counter  int
}

// NewCalendar creates a new calendar skill
func NewCalendar(config *CalendarConfig) *Calendar {
	return &Calendar{
		manifest: &skill.Manifest{
			ID:          "calendar",
			Name:        "Calendar",
			Version:     "1.0.0",
			Description: "Calendar management with CalDAV support",
			Category:    "communication",
			Icon:        "calendar",
			Tags:        []string{"calendar", "events", "schedule", "caldav"},
			Inputs: []skill.Parameter{
				{
					Name:        "action",
					Type:        "string",
					Description: "Action: create, read, update, delete, list, today, upcoming",
					Required:    true,
				},
				{
					Name:        "id",
					Type:        "string",
					Description: "Event ID",
					Required:    false,
				},
				{
					Name:        "title",
					Type:        "string",
					Description: "Event title",
					Required:    false,
				},
				{
					Name:        "description",
					Type:        "string",
					Description: "Event description",
					Required:    false,
				},
				{
					Name:        "location",
					Type:        "string",
					Description: "Event location",
					Required:    false,
				},
				{
					Name:        "start",
					Type:        "string",
					Description: "Start time (RFC3339 format)",
					Required:    false,
				},
				{
					Name:        "end",
					Type:        "string",
					Description: "End time (RFC3339 format)",
					Required:    false,
				},
				{
					Name:        "all_day",
					Type:        "boolean",
					Description: "All-day event",
					Required:    false,
				},
				{
					Name:        "recurring",
					Type:        "string",
					Description: "Recurring: daily, weekly, monthly, yearly",
					Required:    false,
				},
				{
					Name:        "days",
					Type:        "number",
					Description: "Number of days to look ahead (for upcoming)",
					Required:    false,
					Default:     7,
				},
			},
			Outputs: []skill.Parameter{
				{
					Name:        "event",
					Type:        "object",
					Description: "Calendar event",
				},
				{
					Name:        "events",
					Type:        "array",
					Description: "List of events",
				},
			},
			Permissions: []string{"calendar.read", "calendar.write"},
		},
		config: config,
		events: make(map[string]*CalendarEvent),
	}
}

// Manifest returns the skill manifest
func (c *Calendar) Manifest() *skill.Manifest {
	return c.manifest
}

// Validate validates the input parameters
func (c *Calendar) Validate(input map[string]any) error {
	action, ok := input["action"]
	if !ok {
		return fmt.Errorf("action is required")
	}

	actionStr, ok := action.(string)
	if !ok {
		return fmt.Errorf("action must be a string")
	}

	validActions := map[string]bool{
		"create": true, "read": true, "update": true, "delete": true,
		"list": true, "today": true, "upcoming": true,
	}
	if !validActions[actionStr] {
		return fmt.Errorf("invalid action: %s", actionStr)
	}

	switch actionStr {
	case "create":
		if _, ok := input["title"]; !ok {
			return fmt.Errorf("title is required for create action")
		}
		if _, ok := input["start"]; !ok {
			return fmt.Errorf("start is required for create action")
		}
	case "read", "delete":
		if _, ok := input["id"]; !ok {
			return fmt.Errorf("id is required for %s action", actionStr)
		}
	case "update":
		if _, ok := input["id"]; !ok {
			return fmt.Errorf("id is required for update action")
		}
	}

	return nil
}

// Execute executes the calendar skill
func (c *Calendar) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	action := input["action"].(string)

	switch action {
	case "create":
		return c.createEvent(input)
	case "read":
		return c.readEvent(input)
	case "update":
		return c.updateEvent(input)
	case "delete":
		return c.deleteEvent(input)
	case "list":
		return c.listEvents(input)
	case "today":
		return c.todayEvents()
	case "upcoming":
		return c.upcomingEvents(input)
	}

	return skill.NewErrorResult(fmt.Errorf("unknown action: %s", action)), nil
}

func (c *Calendar) createEvent(input map[string]any) (*skill.Result, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.counter++
	id := fmt.Sprintf("event-%d", c.counter)
	now := time.Now()

	startStr := input["start"].(string)
	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		return skill.NewErrorResult(fmt.Errorf("invalid start time: %v", err)), nil
	}

	event := &CalendarEvent{
		ID:      id,
		Title:   input["title"].(string),
		Start:   start,
		Created: now,
		Updated: now,
	}

	// Parse end time
	if endStr, ok := input["end"].(string); ok {
		end, err := time.Parse(time.RFC3339, endStr)
		if err != nil {
			return skill.NewErrorResult(fmt.Errorf("invalid end time: %v", err)), nil
		}
		event.End = end
	} else {
		// Default to 1 hour duration
		event.End = start.Add(time.Hour)
	}

	if desc, ok := input["description"].(string); ok {
		event.Description = desc
	}
	if loc, ok := input["location"].(string); ok {
		event.Location = loc
	}
	if allDay, ok := input["all_day"].(bool); ok {
		event.AllDay = allDay
	}
	if recurring, ok := input["recurring"].(string); ok {
		event.Recurring = recurring
	}

	c.events[id] = event

	return skill.NewResult(map[string]any{
		"event":   event,
		"message": fmt.Sprintf("Event '%s' created", event.Title),
	}), nil
}

func (c *Calendar) readEvent(input map[string]any) (*skill.Result, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	id := input["id"].(string)

	if event, ok := c.events[id]; ok {
		return skill.NewResult(map[string]any{
			"event": event,
		}), nil
	}

	return skill.NewResult(map[string]any{
		"event":   nil,
		"message": fmt.Sprintf("Event '%s' not found", id),
	}), nil
}

func (c *Calendar) updateEvent(input map[string]any) (*skill.Result, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := input["id"].(string)

	event, ok := c.events[id]
	if !ok {
		return skill.NewResult(map[string]any{
			"updated": false,
			"message": fmt.Sprintf("Event '%s' not found", id),
		}), nil
	}

	if title, ok := input["title"].(string); ok {
		event.Title = title
	}
	if desc, ok := input["description"].(string); ok {
		event.Description = desc
	}
	if loc, ok := input["location"].(string); ok {
		event.Location = loc
	}
	if startStr, ok := input["start"].(string); ok {
		start, err := time.Parse(time.RFC3339, startStr)
		if err == nil {
			event.Start = start
		}
	}
	if endStr, ok := input["end"].(string); ok {
		end, err := time.Parse(time.RFC3339, endStr)
		if err == nil {
			event.End = end
		}
	}
	if allDay, ok := input["all_day"].(bool); ok {
		event.AllDay = allDay
	}
	if recurring, ok := input["recurring"].(string); ok {
		event.Recurring = recurring
	}
	event.Updated = time.Now()

	return skill.NewResult(map[string]any{
		"updated": true,
		"event":   event,
		"message": fmt.Sprintf("Event '%s' updated", id),
	}), nil
}

func (c *Calendar) deleteEvent(input map[string]any) (*skill.Result, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := input["id"].(string)

	if event, ok := c.events[id]; ok {
		delete(c.events, id)
		return skill.NewResult(map[string]any{
			"deleted": true,
			"event":   event,
			"message": fmt.Sprintf("Event '%s' deleted", id),
		}), nil
	}

	return skill.NewResult(map[string]any{
		"deleted": false,
		"message": fmt.Sprintf("Event '%s' not found", id),
	}), nil
}

func (c *Calendar) listEvents(input map[string]any) (*skill.Result, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	events := make([]*CalendarEvent, 0, len(c.events))
	for _, event := range c.events {
		events = append(events, event)
	}

	// Sort by start time
	sort.Slice(events, func(i, j int) bool {
		return events[i].Start.Before(events[j].Start)
	})

	return skill.NewResult(map[string]any{
		"events": events,
		"count":  len(events),
	}), nil
}

func (c *Calendar) todayEvents() (*skill.Result, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrow := today.Add(24 * time.Hour)

	var events []*CalendarEvent
	for _, event := range c.events {
		if (event.Start.After(today) || event.Start.Equal(today)) && event.Start.Before(tomorrow) {
			events = append(events, event)
		}
	}

	// Sort by start time
	sort.Slice(events, func(i, j int) bool {
		return events[i].Start.Before(events[j].Start)
	})

	return skill.NewResult(map[string]any{
		"events":  events,
		"count":   len(events),
		"date":    today.Format("2006-01-02"),
		"message": fmt.Sprintf("Found %d events for today", len(events)),
	}), nil
}

func (c *Calendar) upcomingEvents(input map[string]any) (*skill.Result, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	days := 7
	if d, ok := input["days"].(float64); ok {
		days = int(d)
	}

	now := time.Now()
	endDate := now.Add(time.Duration(days) * 24 * time.Hour)

	var events []*CalendarEvent
	for _, event := range c.events {
		if event.Start.After(now) && event.Start.Before(endDate) {
			events = append(events, event)
		}
	}

	// Sort by start time
	sort.Slice(events, func(i, j int) bool {
		return events[i].Start.Before(events[j].Start)
	})

	return skill.NewResult(map[string]any{
		"events":  events,
		"count":   len(events),
		"days":    days,
		"message": fmt.Sprintf("Found %d events in the next %d days", len(events), days),
	}), nil
}
