package heartbeat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/companion"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// ChatFunc is a function that sends a chat request and returns a response.
// This allows heartbeat to use any backend (proxy, direct provider, etc).
type ChatFunc func(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error)

var activeHoursPattern = regexp.MustCompile(`^([01]\d|2[0-3]|24):([0-5]\d)$`)

// RunResult represents the outcome of a single heartbeat execution.
type RunResult struct {
	Status     string // ran, skipped, failed
	Reason     string
	DurationMs int64
}

// RunDeps holds dependencies for runOnce.
type RunDeps struct {
	Config   *Config
	ChatFn   ChatFunc
	Channels *channel.Manager
	Streamer *companion.EventStreamer
	Dedup    *DedupCache
	Logger   *zap.Logger
}

// RunOnce executes a single heartbeat check.
func RunOnce(ctx context.Context, deps RunDeps) RunResult {
	cfg := deps.Config
	start := timeutil.NowTime()

	emit := func(status, reason string) {
		if !cfg.Visibility.UseIndicator {
			return
		}
		EmitEvent(deps.Streamer, HeartbeatEvent{
			Timestamp:     start,
			Status:        status,
			Reason:        reason,
			DurationMs:    timeutil.SinceTime(start).Milliseconds(),
			IndicatorType: ResolveIndicator(status),
		})
	}

	if !cfg.Enabled {
		return RunResult{Status: "skipped", Reason: "disabled"}
	}

	if !isWithinActiveHours(cfg.ActiveHours, timeutil.NowTime()) {
		emit("skipped", "quiet-hours")
		return RunResult{Status: "skipped", Reason: "quiet-hours"}
	}

	// Read HEARTBEAT.md
	workDir := cfg.WorkspaceDir
	if workDir == "" {
		workDir = "."
	}
	hbPath := filepath.Join(workDir, HeartbeatFilename)
	content, err := os.ReadFile(hbPath)
	if err == nil && IsEffectivelyEmpty(string(content)) {
		emit("skipped", "empty-heartbeat-file")
		return RunResult{Status: "skipped", Reason: "empty-heartbeat-file"}
	}
	// If file doesn't exist, proceed — the LLM prompt says "if it exists".

	// Call LLM via proxy
	if deps.ChatFn == nil {
		emit("failed", "no-chat-func")
		deps.Logger.Error("heartbeat: chat function not available")
		return RunResult{Status: "failed", Reason: "no-chat-func"}
	}

	prompt := cfg.Prompt
	if prompt == "" {
		prompt = DefaultPrompt
	}

	resp, err := deps.ChatFn(ctx, llm.ChatRequest{
		Model: cfg.LLMModel,
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: prompt},
		},
		MaxTokens: 1024,
	})
	if err != nil {
		reason := fmt.Sprintf("llm-error: %v", err)
		emit("failed", reason)
		deps.Logger.Error("heartbeat: LLM call failed", zap.Error(err))
		return RunResult{Status: "failed", Reason: reason, DurationMs: timeutil.SinceTime(start).Milliseconds()}
	}

	replyText := strings.TrimSpace(resp.Message.Content)
	if replyText == "" {
		emit("ok-empty", "")
		return RunResult{Status: "ran", DurationMs: timeutil.SinceTime(start).Milliseconds()}
	}

	// Strip HEARTBEAT_OK token
	stripped := StripHeartbeatToken(replyText, cfg.AckMaxChars)
	if stripped.ShouldSkip {
		emit("ok-token", "")
		return RunResult{Status: "ran", DurationMs: timeutil.SinceTime(start).Milliseconds()}
	}

	alertText := stripped.Text

	// Dedup check
	if deps.Dedup != nil && deps.Dedup.IsDuplicate(alertText) {
		emit("skipped", "duplicate")
		return RunResult{Status: "ran", DurationMs: timeutil.SinceTime(start).Milliseconds()}
	}

	// Visibility check
	if !cfg.Visibility.ShowAlerts {
		emit("skipped", "alerts-disabled")
		return RunResult{Status: "ran", DurationMs: timeutil.SinceTime(start).Milliseconds()}
	}

	// Deliver through channel
	if cfg.DeliveryChannel != "" && deps.Channels != nil {
		outMsg := channel.OutgoingMessage{
			ChatID:  cfg.DeliveryChatID,
			Content: alertText,
		}
		if sendErr := deps.Channels.Send(ctx, cfg.DeliveryChannel, outMsg); sendErr != nil {
			deps.Logger.Warn("heartbeat: failed to deliver alert",
				zap.String("channel", cfg.DeliveryChannel),
				zap.Error(sendErr))
		}
	}

	// Record for dedup
	if deps.Dedup != nil {
		deps.Dedup.Record(alertText)
	}

	preview := alertText
	if len(preview) > 200 {
		preview = preview[:200]
	}
	EmitEvent(deps.Streamer, HeartbeatEvent{
		Timestamp:     start,
		Status:        "sent",
		Channel:       cfg.DeliveryChannel,
		Preview:       preview,
		DurationMs:    timeutil.SinceTime(start).Milliseconds(),
		IndicatorType: IndicatorAlert,
	})

	return RunResult{Status: "ran", DurationMs: timeutil.SinceTime(start).Milliseconds()}
}

// isWithinActiveHours checks if the current time falls within the configured window.
func isWithinActiveHours(ah *ActiveHours, now time.Time) bool {
	if ah == nil {
		return true
	}
	startMin := parseHHMM(ah.Start, false)
	endMin := parseHHMM(ah.End, true)
	if startMin < 0 || endMin < 0 || startMin == endMin {
		return true
	}

	loc := time.Local
	if ah.Timezone != "" && ah.Timezone != "Local" {
		if parsed, err := time.LoadLocation(ah.Timezone); err == nil {
			loc = parsed
		}
	}

	t := now.In(loc)
	currentMin := t.Hour()*60 + t.Minute()

	if endMin > startMin {
		return currentMin >= startMin && currentMin < endMin
	}
	// Wraps midnight
	return currentMin >= startMin || currentMin < endMin
}

// parseHHMM parses "HH:MM" into minutes since midnight. Returns -1 on invalid input.
func parseHHMM(s string, allow24 bool) int {
	if !activeHoursPattern.MatchString(s) {
		return -1
	}
	parts := strings.Split(s, ":")
	hour, _ := strconv.Atoi(parts[0])
	minute, _ := strconv.Atoi(parts[1])
	if hour == 24 {
		if !allow24 || minute != 0 {
			return -1
		}
		return 24 * 60
	}
	return hour*60 + minute
}
