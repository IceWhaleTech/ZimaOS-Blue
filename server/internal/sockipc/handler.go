package sockipc

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

// MediaGenerator creates a media generation task. Returns task ID or error.
type MediaGenerator func(ctx context.Context, category, model, prompt string, params map[string]string) (taskID string, err error)

// MediaStatusQuerier returns task status fields. Returns nil map if not found.
type MediaStatusQuerier func(ctx context.Context, taskID string) (map[string]string, error)

// RegisterMediaHandlers wires up the standard media IPC commands.
func RegisterMediaHandlers(srv *Server, generate MediaGenerator, queryStatus MediaStatusQuerier, log *zap.Logger) {
	// ping — health check
	srv.Handle("ping", func(_ context.Context, _ *Request) *Response {
		return OkResponse(nil)
	})

	// media.generate — requires prompt
	srv.Handle("media.generate", func(ctx context.Context, req *Request) *Response {
		prompt := req.Params["prompt"]
		if prompt == "" {
			return ErrResponse("missing prompt")
		}

		category := req.Params["category"]
		if category == "" {
			category = "t2i"
		}

		// Collect extra params
		params := make(map[string]string)
		for _, key := range []string{"size", "resolution", "duration", "n", "quality", "style", "negative_prompt", "message_id", "session_id", "source"} {
			if v := req.Params[key]; v != "" {
				params[key] = v
			}
		}

		taskID, err := generate(ctx, category, req.Params["model"], prompt, params)
		if err != nil {
			log.Warn("sockipc media.generate failed", zap.Error(err))
			return ErrResponse(fmt.Sprintf("generate failed: %s", err))
		}

		return OkResponse(map[string]string{"task_id": taskID})
	})

	// media.status — requires task_id
	srv.Handle("media.status", func(ctx context.Context, req *Request) *Response {
		taskID := req.Params["task_id"]
		if taskID == "" {
			return ErrResponse("missing task_id")
		}

		fields, err := queryStatus(ctx, taskID)
		if err != nil {
			return ErrResponse(err.Error())
		}
		if fields == nil {
			return ErrResponse("task not found")
		}

		return OkResponse(fields)
	})
}
