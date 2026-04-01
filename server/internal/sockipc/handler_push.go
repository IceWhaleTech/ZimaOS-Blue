package sockipc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/remindertime"
	"go.uber.org/zap"
)

// PushBackend defines reminder operations exposed via IPC.
type PushBackend interface {
	Add(ctx context.Context, ownerID, message string, fireAt time.Time, recurring, sessionID string, untilAt *time.Time) (PushResult, error)
	List(ctx context.Context, ownerID string) ([]PushResult, error)
	Delete(ctx context.Context, ownerID, id string) error
	Clear(ctx context.Context, ownerID string) (int64, error)
}

// PushResult is the data returned by the push backend.
type PushResult struct {
	ID        string     `json:"id"`
	Message   string     `json:"message"`
	FireAt    time.Time  `json:"fire_at"`
	Recurring string     `json:"recurring,omitempty"`
	UntilAt   *time.Time `json:"until_at,omitempty"`
	SessionID string     `json:"session_id,omitempty"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
}

// RegisterPushHandlers wires up reminder IPC commands.
// Auth is handled by Unix socket file permissions (0600).
// User identification uses session_id param (defaults to "default").
func RegisterPushHandlers(srv *Server, backend PushBackend, log *zap.Logger) {
	ownerOf := func(req *Request) string {
		if id := req.Params["session_id"]; id != "" {
			return id
		}
		return "default"
	}

	// reminder.add — schedule a reminder.
	handleAdd := func(ctx context.Context, req *Request) *Response {
		userID := ownerOf(req)

		message := req.Params["message"]
		if message == "" {
			return ErrResponse("missing message")
		}
		timeStr := req.Params["time"]
		everyStr := req.Params["every"]
		if timeStr == "" && everyStr == "" {
			if inferredTime, ok := remindertime.InferTimeStringFromMessage(message); ok {
				timeStr = inferredTime
			}
		}
		if timeStr == "" && everyStr == "" {
			return ErrResponse("missing time or every")
		}

		recurring := req.Params["recurring"]
		if everyStr != "" && recurring != "" {
			return ErrResponse("every cannot be combined with recurring")
		}

		var fireAt time.Time
		var err error
		if timeStr != "" {
			fireAt, err = parseTime(timeStr)
			if err != nil {
				return ErrResponse("invalid time: " + err.Error())
			}
		}
		if everyStr != "" {
			interval, intervalErr := remindertime.ParseDuration(everyStr)
			if intervalErr != nil {
				return ErrResponse("invalid every: " + intervalErr.Error())
			}
			if interval < time.Minute {
				return ErrResponse("every must be at least 1 minute")
			}
			recurring = "interval:" + interval.String()
			if timeStr == "" {
				fireAt = time.Now().Add(interval)
			}
		}

		var untilAt *time.Time
		if untilStr := req.Params["until"]; untilStr != "" {
			parsedUntil, untilErr := parseTime(untilStr)
			if untilErr != nil {
				return ErrResponse("invalid until: " + untilErr.Error())
			}
			untilAt = &parsedUntil
		}

		sessionID := req.Params["session_id"]

		result, err := backend.Add(ctx, userID, message, fireAt, recurring, sessionID, untilAt)
		if err != nil {
			log.Warn("sockipc reminder.add failed", zap.Error(err))
			return ErrResponse("add failed: " + err.Error())
		}

		data, _ := json.Marshal(result)
		return OkResponse(map[string]string{"reminder": string(data)})
	}
	srv.Handle("reminder.add", handleAdd)

	// reminder.list — list pending reminders.
	handleList := func(ctx context.Context, req *Request) *Response {
		userID := ownerOf(req)

		list, err := backend.List(ctx, userID)
		if err != nil {
			return ErrResponse("list failed: " + err.Error())
		}

		data, _ := json.Marshal(list)
		return OkResponse(map[string]string{"reminders": string(data), "count": fmt.Sprintf("%d", len(list))})
	}
	srv.Handle("reminder.list", handleList)

	// reminder.delete — delete a reminder by ID.
	handleDelete := func(ctx context.Context, req *Request) *Response {
		userID := ownerOf(req)

		id := req.Params["id"]
		if id == "" {
			return ErrResponse("missing id")
		}

		if err := backend.Delete(ctx, userID, id); err != nil {
			return ErrResponse("delete failed: " + err.Error())
		}

		return OkResponse(map[string]string{"deleted": id})
	}
	srv.Handle("reminder.delete", handleDelete)

	// reminder.clear — delete all reminders.
	handleClear := func(ctx context.Context, req *Request) *Response {
		userID := ownerOf(req)

		count, err := backend.Clear(ctx, userID)
		if err != nil {
			return ErrResponse("clear failed: " + err.Error())
		}

		return OkResponse(map[string]string{"cleared": fmt.Sprintf("%d", count)})
	}
	srv.Handle("reminder.clear", handleClear)
}

// parseTime parses a time string as duration, RFC3339, or common format.
func parseTime(s string) (time.Time, error) {
	return remindertime.Parse(s)
}
