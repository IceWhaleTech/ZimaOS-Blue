package sockipc

import (
	"context"
	"strconv"

	"go.uber.org/zap"
)

// SkillManagerBackend defines the skill management operations exposed via IPC.
type SkillManagerBackend interface {
	Search(ctx context.Context, query, category string, page, pageSize int) (string, error)
	Install(ctx context.Context, id string) (string, error)
	InstallURL(ctx context.Context, url, name string) (string, error)
	Uninstall(ctx context.Context, id string) error
	Update(ctx context.Context, id string) (string, error)
	Enable(ctx context.Context, id string) error
	Disable(ctx context.Context, id string) error
	List(ctx context.Context) (string, error)
	Info(ctx context.Context, id string) (string, error)
}

// RegisterSkillManagerHandlers wires up skill management IPC commands.
// Auth is handled by Unix socket file permissions (0600).
func RegisterSkillManagerHandlers(srv *Server, backend SkillManagerBackend, log *zap.Logger) {
	// skill.search — search the skill store
	srv.Handle("skill.search", func(ctx context.Context, req *Request) *Response {
		query := req.Params["query"]
		if query == "" {
			return ErrResponse("missing query")
		}
		category := req.Params["category"]
		page := 1
		pageSize := 24
		if v, err := strconv.Atoi(req.Params["page"]); err == nil && v > 0 {
			page = v
		}
		if v, err := strconv.Atoi(req.Params["page_size"]); err == nil && v > 0 && v <= 100 {
			pageSize = v
		}

		result, err := backend.Search(ctx, query, category, page, pageSize)
		if err != nil {
			log.Warn("sockipc skill.search failed", zap.Error(err))
			return ErrResponse("search failed: " + err.Error())
		}
		return OkResponse(map[string]string{"result": result})
	})

	// skill.install — install a skill by ID from the store
	srv.Handle("skill.install", func(ctx context.Context, req *Request) *Response {
		id := req.Params["id"]
		if id == "" {
			return ErrResponse("missing id")
		}

		result, err := backend.Install(ctx, id)
		if err != nil {
			log.Warn("sockipc skill.install failed", zap.String("id", id), zap.Error(err))
			return ErrResponse("install failed: " + err.Error())
		}
		return OkResponse(map[string]string{"result": result})
	})

	// skill.install_url — install a skill from a URL
	srv.Handle("skill.install_url", func(ctx context.Context, req *Request) *Response {
		url := req.Params["url"]
		if url == "" {
			return ErrResponse("missing url")
		}
		name := req.Params["name"]

		result, err := backend.InstallURL(ctx, url, name)
		if err != nil {
			log.Warn("sockipc skill.install_url failed", zap.String("url", url), zap.Error(err))
			return ErrResponse("install failed: " + err.Error())
		}
		return OkResponse(map[string]string{"result": result})
	})

	// skill.uninstall — uninstall a skill by ID
	srv.Handle("skill.uninstall", func(ctx context.Context, req *Request) *Response {
		id := req.Params["id"]
		if id == "" {
			return ErrResponse("missing id")
		}

		if err := backend.Uninstall(ctx, id); err != nil {
			log.Warn("sockipc skill.uninstall failed", zap.String("id", id), zap.Error(err))
			return ErrResponse("uninstall failed: " + err.Error())
		}
		return OkResponse(map[string]string{"uninstalled": id})
	})

	// skill.update — update a skill (uninstall + reinstall)
	srv.Handle("skill.update", func(ctx context.Context, req *Request) *Response {
		id := req.Params["id"]
		if id == "" {
			return ErrResponse("missing id")
		}

		result, err := backend.Update(ctx, id)
		if err != nil {
			log.Warn("sockipc skill.update failed", zap.String("id", id), zap.Error(err))
			return ErrResponse("update failed: " + err.Error())
		}
		return OkResponse(map[string]string{"result": result})
	})

	// skill.enable — enable a skill
	srv.Handle("skill.enable", func(ctx context.Context, req *Request) *Response {
		id := req.Params["id"]
		if id == "" {
			return ErrResponse("missing id")
		}

		if err := backend.Enable(ctx, id); err != nil {
			return ErrResponse("enable failed: " + err.Error())
		}
		return OkResponse(map[string]string{"enabled": id})
	})

	// skill.disable — disable a skill
	srv.Handle("skill.disable", func(ctx context.Context, req *Request) *Response {
		id := req.Params["id"]
		if id == "" {
			return ErrResponse("missing id")
		}

		if err := backend.Disable(ctx, id); err != nil {
			return ErrResponse("disable failed: " + err.Error())
		}
		return OkResponse(map[string]string{"disabled": id})
	})

	// skill.list — list installed skills
	srv.Handle("skill.list", func(ctx context.Context, req *Request) *Response {
		result, err := backend.List(ctx)
		if err != nil {
			return ErrResponse("list failed: " + err.Error())
		}
		return OkResponse(map[string]string{"skills": result})
	})

	// skill.info — get skill detail
	srv.Handle("skill.info", func(ctx context.Context, req *Request) *Response {
		id := req.Params["id"]
		if id == "" {
			return ErrResponse("missing id")
		}

		result, err := backend.Info(ctx, id)
		if err != nil {
			return ErrResponse("info failed: " + err.Error())
		}
		return OkResponse(map[string]string{"skill": result})
	})
}
