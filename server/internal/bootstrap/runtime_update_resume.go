package bootstrap

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cron"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/update"
)

// readLocaleFromKV reads the locale from kvstore settings without creating a full SettingsHandler.
func readLocaleFromKV(kv kvstore.Store) string {
	if kv == nil {
		return ""
	}
	var s struct {
		Locale string `json:"locale"`
	}
	if kv.GetJSON(context.Background(), "config:settings", &s) != nil {
		return ""
	}
	return s.Locale
}

func updateResumeContextString(ctx map[string]interface{}, key string) string {
	if len(ctx) == 0 || key == "" {
		return ""
	}
	raw, ok := ctx[key]
	if !ok || raw == nil {
		return ""
	}
	if v, ok := raw.(string); ok {
		return strings.TrimSpace(v)
	}
	return strings.TrimSpace(fmt.Sprint(raw))
}

func normalizeUpdateResumeKind(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "none", "noop":
		return ""
	case "once-cron", "cron-once", "once_cron", "cron_once":
		return "once-cron"
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}

func buildUpdateResumeRecoverer(cronHandler *cron.Handler, logger *zap.Logger) update.ResumeRecoverer {
	return func(_ context.Context, input update.ResumeRecoverInput) error {
		kind := normalizeUpdateResumeKind(updateResumeContextString(input.Context, "kind"))
		if kind == "" {
			return nil
		}
		switch kind {
		case "once-cron":
			if cronHandler == nil {
				return fmt.Errorf("resume kind %q requires cron handler", kind)
			}
			jobID := updateResumeContextString(input.Context, "job_id")
			if jobID == "" {
				return fmt.Errorf("resume kind %q requires context.job_id", kind)
			}
			svc := cronHandler.GetService()
			if svc == nil {
				return fmt.Errorf("cron service unavailable")
			}
			if err := svc.Trigger(jobID); err != nil {
				return fmt.Errorf("trigger cron job %s: %w", jobID, err)
			}
			if logger != nil {
				logger.Info("OTA resume context triggered cron job",
					zap.String("task_id", input.TaskID),
					zap.String("job_id", jobID),
					zap.String("kind", kind))
			}
			return nil
		default:
			return fmt.Errorf("unsupported resume context kind %q", kind)
		}
	}
}
