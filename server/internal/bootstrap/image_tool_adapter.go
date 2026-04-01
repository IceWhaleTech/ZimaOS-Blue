package bootstrap

import (
	"context"
	"fmt"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mediagen"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func newImageGenerateAdapter(manager *mediagen.Manager, workspaceRoots []string) tools.ImageGenerateFunc {
	if manager == nil {
		return nil
	}
	fallbackRoots := normalizeImageWorkspaceRoots(workspaceRoots)
	return func(ctx context.Context, req tools.ImageGenerateRequest) (*tools.ImageTaskResult, error) {
		lookupUserID := imageTaskLookupUserID(ctx)
		mediaReq, err := imageGenerateMediaRequest(req)
		if err != nil {
			return nil, err
		}
		task, err := manager.Generate(ctx, mediaReq)
		if err != nil {
			return nil, err
		}
		if !req.Wait {
			return imageTaskFromMediaTask(task, req.Category), nil
		}
		return waitForGeneratedImageResult(ctx, manager, task, req, lookupUserID, fallbackRoots)
	}
}

func waitForGeneratedImageResult(
	ctx context.Context,
	manager *mediagen.Manager,
	task *mediagen.MediaTask,
	req tools.ImageGenerateRequest,
	lookupUserID string,
	fallbackRoots []string,
) (*tools.ImageTaskResult, error) {
	waitTimeout := req.WaitTimeout
	if waitTimeout <= 0 {
		waitTimeout = 120 * time.Second
	}
	waitBase := context.Background()
	if lookupUserID != "" {
		waitBase = tools.WithUserID(waitBase, lookupUserID)
	}
	waitCtx, cancel := context.WithTimeout(waitBase, waitTimeout)
	defer cancel()

	waitedTask, waitErr := waitForImageTask(waitCtx, manager, task.ID, lookupUserID)
	if waitedTask != nil {
		task = waitedTask
	}
	result := imageTaskFromMediaTask(task, req.Category)
	if result != nil && req.OutputPath != "" {
		if result.Request == nil {
			result.Request = make(map[string]interface{})
		}
		result.Request["path"] = req.OutputPath
	}
	if waitErr != nil {
		result.Message = waitErr.Error()
		if result.Status == "" {
			result.Status = "processing"
		}
		return result, nil
	}
	saveCtx, scopedOutputPath := withImageOutputWorkspaceScope(ctx, req.OutputPath, fallbackRoots)
	if savedPath, saveErr := saveGeneratedImageOutput(saveCtx, manager, task, scopedOutputPath, lookupUserID); saveErr == nil && savedPath != "" {
		if result.Request == nil {
			result.Request = make(map[string]interface{})
		}
		result.Request["path"] = savedPath
		result.Message = joinImageMessages(result.Message, fmt.Sprintf("saved generated image to %s", savedPath))
	} else if saveErr != nil {
		result.Message = joinImageMessages(result.Message, fmt.Sprintf("generated image but failed to save %q: %v", req.OutputPath, saveErr))
	}
	return result, nil
}
