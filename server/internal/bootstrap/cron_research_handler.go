package bootstrap

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cron"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	"go.uber.org/zap"
)

const researchAndNotifyHandlerName = "research_and_notify"

func registerDeepResearchCronHandler(cronSvc *cron.Service, researchSvc *deepresearch.Service, logger *zap.Logger) {
	if cronSvc == nil || researchSvc == nil {
		return
	}
	handler := newDeepResearchCronHandler(researchSvc, logger)
	cronSvc.RegisterHandler(researchAndNotifyHandlerName, handler)
	// Backward-compatible alias.
	cronSvc.RegisterHandler("research-and-notify", handler)
}

func newDeepResearchCronHandler(service *deepresearch.Service, logger *zap.Logger) cron.JobHandler {
	return func(ctx context.Context, job *cron.Job) (interface{}, error) {
		if service == nil {
			return nil, fmt.Errorf("deep research service not available")
		}

		payload := map[string]interface{}{}
		if job != nil && job.Payload != nil {
			payload = job.Payload
		}

		query := payloadString(payload, "query", "research_query", "researchQuery", "objective", "topic", "prompt")
		if query == "" {
			return nil, fmt.Errorf("query not specified in payload")
		}

		req := deepresearch.CreateJobRequest{
			UserID:         payloadString(payload, "user_id", "userId", "owner_id", "ownerId", "notify_user_id", "notifyUserId"),
			TenantID:       payloadString(payload, "tenant_id", "tenantId"),
			ConversationID: payloadString(payload, "conversation_id", "conversationId", "session_id", "sessionId"),
			Query:          query,
			Mode:           deepresearch.Mode(payloadString(payload, "mode")),
			RouteMode:      deepresearch.RouteMode(payloadString(payload, "route_mode", "routeMode")),
			Lang:           payloadString(payload, "lang", "language", "locale"),
			TimeWindows:    payloadStringSlice(payload, "time_windows", "timeWindows"),
			ReportStyle:    payloadString(payload, "report_style", "reportStyle"),
		}
		if strictEntity, ok := payloadBool(payload, "strict_entity", "strictEntity"); ok {
			req.StrictEntity = &strictEntity
		}

		researchJob, err := service.CreateJob(ctx, req)
		if err != nil {
			return nil, err
		}

		if logger != nil {
			cronJobID := ""
			if job != nil {
				cronJobID = strings.TrimSpace(job.ID)
			}
			logger.Info("cron research job created",
				zap.String("cron_job_id", cronJobID),
				zap.String("research_job_id", researchJob.ID),
				zap.String("query", researchJob.Query))
		}

		return map[string]interface{}{
			"type":                 "deep-research-job",
			"status":               "accepted",
			"research_job_id":      researchJob.ID,
			"job_id":               researchJob.ID,
			"query":                researchJob.Query,
			"mode":                 researchJob.Mode,
			"requested_route_mode": researchJob.RequestedRouteMode,
			"effective_route_mode": researchJob.EffectiveRouteMode,
			"route_reason":         researchJob.RouteReason,
		}, nil
	}
}

func payloadString(payload map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if payload == nil {
			continue
		}
		if value, ok := payload[key]; ok {
			switch typed := value.(type) {
			case string:
				if trimmed := strings.TrimSpace(typed); trimmed != "" {
					return trimmed
				}
			case []byte:
				if trimmed := strings.TrimSpace(string(typed)); trimmed != "" {
					return trimmed
				}
			default:
				trimmed := strings.TrimSpace(fmt.Sprintf("%v", typed))
				if trimmed != "" && trimmed != "<nil>" {
					return trimmed
				}
			}
		}
	}
	return ""
}

func payloadBool(payload map[string]interface{}, keys ...string) (bool, bool) {
	for _, key := range keys {
		if payload == nil {
			continue
		}
		raw, ok := payload[key]
		if !ok {
			continue
		}
		switch typed := raw.(type) {
		case bool:
			return typed, true
		case string:
			parsed, err := strconv.ParseBool(strings.TrimSpace(typed))
			if err == nil {
				return parsed, true
			}
		case float64:
			return typed != 0, true
		case float32:
			return typed != 0, true
		case int:
			return typed != 0, true
		case int64:
			return typed != 0, true
		}
	}
	return false, false
}

func payloadStringSlice(payload map[string]interface{}, keys ...string) []string {
	for _, key := range keys {
		if payload == nil {
			continue
		}
		raw, ok := payload[key]
		if !ok || raw == nil {
			continue
		}
		switch typed := raw.(type) {
		case []string:
			return cloneNonEmptyStrings(typed)
		case []interface{}:
			items := make([]string, 0, len(typed))
			for _, item := range typed {
				s := strings.TrimSpace(fmt.Sprintf("%v", item))
				if s != "" && s != "<nil>" {
					items = append(items, s)
				}
			}
			if len(items) > 0 {
				return items
			}
		case string:
			if strings.TrimSpace(typed) == "" {
				return nil
			}
			parts := strings.Split(typed, ",")
			items := make([]string, 0, len(parts))
			for _, part := range parts {
				if s := strings.TrimSpace(part); s != "" {
					items = append(items, s)
				}
			}
			if len(items) > 0 {
				return items
			}
		}
	}
	return nil
}

func cloneNonEmptyStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s := strings.TrimSpace(item); s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
