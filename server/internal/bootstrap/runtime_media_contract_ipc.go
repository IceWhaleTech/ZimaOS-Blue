package bootstrap

import (
	"context"
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mediagen"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sockipc"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"go.uber.org/zap"
)

func registerRouteRuntimeMediaIPC(
	ipcSrv *sockipc.Server,
	deps *RoutesDeps,
	services *Services,
	mediaManager *mediagen.Manager,
	workspaceDir string,
	logger *zap.Logger,
) {
	if ipcSrv == nil || deps == nil || services == nil || mediaManager == nil {
		return
	}
	mediaGen := func(ctx context.Context, category, model, prompt string, params map[string]string) (string, error) {
		req := &mediagen.MediaRequest{
			Prompt: prompt,
			Model:  model,
		}
		switch category {
		case "t2v", "i2v", "kf2v":
			req.Type = mediagen.MediaTypeVideo
		default:
			req.Type = mediagen.MediaTypeImage
		}
		if v, ok := params["size"]; ok {
			req.Size = v
		}
		messageID := params["message_id"]
		source := params["source"]
		if source == "" {
			source = "ipc"
		}
		task, err := mediaManager.CreateTask(ctx, req, messageID, category, source)
		if err != nil {
			return "", err
		}
		return task.ID, nil
	}
	statusQuery := func(ctx context.Context, taskID string) (map[string]string, error) {
		lookupUserID := strings.TrimSpace(tools.GetUserID(ctx))
		var (
			task *mediagen.MediaTask
			err  error
		)
		if lookupUserID != "" {
			task, err = mediaManager.GetTask(taskID, lookupUserID)
		} else {
			task, err = mediaManager.GetTask(taskID)
		}
		if err != nil {
			return nil, err
		}
		if task == nil {
			return nil, nil
		}
		return map[string]string{
			"task_id":     task.ID,
			"task_status": string(task.Status),
			"progress":    fmt.Sprintf("%.2f", task.Progress),
			"error":       task.Error,
		}, nil
	}
	sockipc.RegisterMediaHandlers(ipcSrv, mediaGen, statusQuery, logger)
	if deps.BrowserIPC != nil {
		sockipc.RegisterBrowserHandlers(ipcSrv, deps.BrowserIPC, logger)
	}
	if deps.UIReviewerIPC != nil {
		sockipc.RegisterUIReviewHandlers(ipcSrv, deps.UIReviewerIPC, logger)
	}
	if deps.PushIPC != nil {
		sockipc.RegisterPushHandlers(ipcSrv, deps.PushIPC, logger)
	}
	if deps.CronIPC != nil {
		sockipc.RegisterCronHandlers(ipcSrv, deps.CronIPC, logger)
	}
	if deps.RegisterIPCExtensions != nil {
		deps.RegisterIPCExtensions(ipcSrv)
	}
	sockipc.RegisterSkillFallback(ipcSrv, newRuntimeIPCSkillExecutor(services, workspaceDir), logger)
}
