package sockipc

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"
)

// CronBackend defines the cron/scheduler operations exposed via IPC.
type CronBackend interface {
	Create(ctx context.Context, name, description, schedule, command string) (CronResult, error)
	List(ctx context.Context) ([]CronResult, error)
	Delete(ctx context.Context, id string) error
	Trigger(ctx context.Context, id string) error
	Enable(ctx context.Context, id string) error
	Disable(ctx context.Context, id string) error
}

// CronResult is the data returned by the cron backend.
type CronResult struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Schedule    string `json:"schedule"`
	Handler     string `json:"handler"`
	Enabled     bool   `json:"enabled"`
	Status      string `json:"status"`
	RunCount    int64  `json:"run_count"`
	FailCount   int64  `json:"fail_count"`
}

// RegisterCronHandlers wires up cron/scheduler IPC commands.
// Auth is handled by Unix socket file permissions (0600).
func RegisterCronHandlers(srv *Server, backend CronBackend, log *zap.Logger) {
	// cron.create — create a new cron job
	srv.Handle("cron.create", func(ctx context.Context, req *Request) *Response {

		name := req.Params["name"]
		schedule := req.Params["schedule"]
		if schedule == "" {
			return ErrResponse("missing schedule")
		}
		command := req.Params["command"]
		if command == "" {
			return ErrResponse("missing command")
		}
		description := req.Params["description"]

		result, err := backend.Create(ctx, name, description, schedule, command)
		if err != nil {
			log.Warn("sockipc cron.create failed", zap.Error(err))
			return ErrResponse("create failed: " + err.Error())
		}

		data, _ := json.Marshal(result)
		return OkResponse(map[string]string{"job": string(data)})
	})

	// cron.list — list all cron jobs
	srv.Handle("cron.list", func(ctx context.Context, req *Request) *Response {
		list, err := backend.List(ctx)
		if err != nil {
			return ErrResponse("list failed: " + err.Error())
		}

		data, _ := json.Marshal(list)
		return OkResponse(map[string]string{"jobs": string(data), "count": fmt.Sprintf("%d", len(list))})
	})

	// cron.delete — delete a cron job by ID
	srv.Handle("cron.delete", func(ctx context.Context, req *Request) *Response {
		id := req.Params["id"]
		if id == "" {
			return ErrResponse("missing id")
		}

		if err := backend.Delete(ctx, id); err != nil {
			return ErrResponse("delete failed: " + err.Error())
		}

		return OkResponse(map[string]string{"deleted": id})
	})

	// cron.trigger — manually trigger a cron job
	srv.Handle("cron.trigger", func(ctx context.Context, req *Request) *Response {
		id := req.Params["id"]
		if id == "" {
			return ErrResponse("missing id")
		}

		if err := backend.Trigger(ctx, id); err != nil {
			return ErrResponse("trigger failed: " + err.Error())
		}

		return OkResponse(map[string]string{"triggered": id})
	})

	// cron.enable — enable a paused cron job
	srv.Handle("cron.enable", func(ctx context.Context, req *Request) *Response {
		id := req.Params["id"]
		if id == "" {
			return ErrResponse("missing id")
		}

		if err := backend.Enable(ctx, id); err != nil {
			return ErrResponse("enable failed: " + err.Error())
		}

		return OkResponse(map[string]string{"enabled": id})
	})

	// cron.disable — disable a cron job
	srv.Handle("cron.disable", func(ctx context.Context, req *Request) *Response {
		id := req.Params["id"]
		if id == "" {
			return ErrResponse("missing id")
		}

		if err := backend.Disable(ctx, id); err != nil {
			return ErrResponse("disable failed: " + err.Error())
		}

		return OkResponse(map[string]string{"disabled": id})
	})
}
