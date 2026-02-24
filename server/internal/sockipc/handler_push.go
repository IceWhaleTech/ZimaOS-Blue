package sockipc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// PushBackend defines the push notification operations exposed via IPC.
type PushBackend interface {
	Add(ctx context.Context, ownerID, message string, fireAt time.Time, recurring, sessionID string) (PushResult, error)
	List(ctx context.Context, ownerID string) ([]PushResult, error)
	Delete(ctx context.Context, ownerID, id string) error
	Clear(ctx context.Context, ownerID string) (int64, error)
}

// PushResult is the data returned by the push backend.
type PushResult struct {
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	FireAt    time.Time `json:"fire_at"`
	Recurring string    `json:"recurring,omitempty"`
	SessionID string    `json:"session_id,omitempty"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// RegisterPushHandlers wires up push notification IPC commands.
// Auth is handled by Unix socket file permissions (0600).
// User identification uses session_id param (defaults to "default").
func RegisterPushHandlers(srv *Server, backend PushBackend, log *zap.Logger) {
	ownerOf := func(req *Request) string {
		if id := req.Params["session_id"]; id != "" {
			return id
		}
		return "default"
	}

	// push.add — schedule a push notification
	srv.Handle("push.add", func(ctx context.Context, req *Request) *Response {
		userID := ownerOf(req)

		message := req.Params["message"]
		if message == "" {
			return ErrResponse("missing message")
		}
		timeStr := req.Params["time"]
		if timeStr == "" {
			return ErrResponse("missing time")
		}

		fireAt, err := parseTime(timeStr)
		if err != nil {
			return ErrResponse("invalid time: " + err.Error())
		}

		recurring := req.Params["recurring"]
		sessionID := req.Params["session_id"]

		result, err := backend.Add(ctx, userID, message, fireAt, recurring, sessionID)
		if err != nil {
			log.Warn("sockipc push.add failed", zap.Error(err))
			return ErrResponse("add failed: " + err.Error())
		}

		data, _ := json.Marshal(result)
		return OkResponse(map[string]string{"notification": string(data)})
	})

	// push.list — list all pending notifications for the user
	srv.Handle("push.list", func(ctx context.Context, req *Request) *Response {
		userID := ownerOf(req)

		list, err := backend.List(ctx, userID)
		if err != nil {
			return ErrResponse("list failed: " + err.Error())
		}

		data, _ := json.Marshal(list)
		return OkResponse(map[string]string{"notifications": string(data), "count": fmt.Sprintf("%d", len(list))})
	})

	// push.delete — delete a notification by ID
	srv.Handle("push.delete", func(ctx context.Context, req *Request) *Response {
		userID := ownerOf(req)

		id := req.Params["id"]
		if id == "" {
			return ErrResponse("missing id")
		}

		if err := backend.Delete(ctx, userID, id); err != nil {
			return ErrResponse("delete failed: " + err.Error())
		}

		return OkResponse(map[string]string{"deleted": id})
	})

	// push.clear — delete all notifications for the user
	srv.Handle("push.clear", func(ctx context.Context, req *Request) *Response {
		userID := ownerOf(req)

		count, err := backend.Clear(ctx, userID)
		if err != nil {
			return ErrResponse("clear failed: " + err.Error())
		}

		return OkResponse(map[string]string{"cleared": fmt.Sprintf("%d", count)})
	})
}

// parseTime parses a time string as duration, RFC3339, or common format.
func parseTime(s string) (time.Time, error) {
	if d, err := time.ParseDuration(s); err == nil {
		return time.Now().Add(d), nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04", s, time.Local); err == nil {
		return t, nil
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("use duration (1h30m), RFC3339, or YYYY-MM-DD HH:MM")
}
