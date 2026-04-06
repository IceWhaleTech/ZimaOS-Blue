package bootstrap

import (
	"context"
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cron"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/knowledge"
	"go.uber.org/zap"
)

const knowledgeLintHandlerName = "knowledge_lint"

const (
	knowledgeNightlyLintJobName        = "knowledge-nightly-lint"
	knowledgeNightlyLintJobDescription = "nightly knowledge lint"
	knowledgeNightlyLintSchedule       = "0 0 3 * * *"
)

func registerKnowledgeCronSupport(options runtimeTaskSurfaceOptions, service *knowledge.Service) bool {
	if options.cronHandler == nil || service == nil {
		return false
	}
	options.cronHandler.SetServiceInitHook(func(cronSvc *cron.Service) {
		registerKnowledgeCronHandler(cronSvc, service, options.logger)
	})
	return true
}

func registerKnowledgeCronHandler(cronSvc *cron.Service, service *knowledge.Service, logger *zap.Logger) {
	if cronSvc == nil || service == nil {
		return
	}
	handler := newKnowledgeCronHandler(service, logger)
	cronSvc.RegisterHandler(knowledgeLintHandlerName, handler)
	cronSvc.RegisterHandler("knowledge-lint", handler)
}

func ensureNightlyKnowledgeLintJob(cronSvc *cron.Service, logger *zap.Logger) error {
	if cronSvc == nil {
		return nil
	}
	for _, job := range cronSvc.List() {
		if job == nil {
			continue
		}
		if job.Schedule != knowledgeNightlyLintSchedule {
			continue
		}
		handler := strings.TrimSpace(job.Handler)
		if handler != knowledgeLintHandlerName && handler != "knowledge-lint" {
			continue
		}
		return nil
	}

	job, err := cronSvc.Create(
		knowledgeNightlyLintJobName,
		knowledgeNightlyLintJobDescription,
		knowledgeNightlyLintSchedule,
		knowledgeLintHandlerName,
		nil,
	)
	if err != nil {
		return err
	}
	if job != nil {
		if job.Metadata == nil {
			job.Metadata = make(map[string]interface{})
		}
		job.Metadata["managed_by"] = "knowledge"
		job.Metadata["maintenance_kind"] = "nightly_lint"
	}
	if logger != nil {
		cronJobID := ""
		if job != nil {
			cronJobID = job.ID
		}
		logger.Info("knowledge nightly lint cron job ensured",
			zap.String("cron_job_id", cronJobID),
			zap.String("schedule", knowledgeNightlyLintSchedule),
		)
	}
	return nil
}

func newKnowledgeCronHandler(service *knowledge.Service, logger *zap.Logger) cron.JobHandler {
	return func(ctx context.Context, job *cron.Job) (interface{}, error) {
		if service == nil {
			return nil, fmt.Errorf("knowledge service not available")
		}
		if !service.HasCompiledKnowledge() {
			return map[string]interface{}{
				"type":   "knowledge-lint",
				"status": "skipped",
				"reason": "compiled knowledge not ready",
			}, nil
		}

		payload := map[string]interface{}{}
		if job != nil && job.Payload != nil {
			payload = job.Payload
		}
		targetPaths := payloadStringSlice(payload, "target_paths", "targetPaths")
		report, err := service.Lint(ctx, knowledge.LintRequest{TargetPaths: targetPaths})
		if err != nil {
			return nil, err
		}
		if logger != nil {
			cronJobID := ""
			if job != nil {
				cronJobID = strings.TrimSpace(job.ID)
			}
			logger.Info("knowledge cron lint completed",
				zap.String("cron_job_id", cronJobID),
				zap.Int("issues", len(report.Issues)),
			)
		}
		return map[string]interface{}{
			"type":        "knowledge-lint",
			"status":      "completed",
			"issue_count": len(report.Issues),
			"fixed_paths": report.FixedPaths,
		}, nil
	}
}
