package metrics

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
)

// DevHandler serves the dev dashboard REST API.
type DevHandler struct {
	db      *sql.DB
	readDB  *sql.DB
	writer  *MetricsWriter
	enabled bool
}

// NewDevHandler creates a new dev dashboard handler.
func NewDevHandler(writer *MetricsWriter, enabled bool) *DevHandler {
	var db, readDB *sql.DB
	if writer != nil {
		db = writer.GetDB()
		readDB = writer.GetReadDB()
	}
	if readDB == nil {
		readDB = db
	}
	return &DevHandler{
		db:      db,
		readDB:  readDB,
		writer:  writer,
		enabled: enabled,
	}
}

// RegisterRoutes registers all dev dashboard routes on the given group.
func (h *DevHandler) RegisterRoutes(g *echo.Group) {
	if h == nil || !h.enabled {
		return
	}
	g.GET("/sessions", h.ListSessions)
	g.GET("/sessions/:id/turns", h.GetSessionTurns)
	g.GET("/turns/:turnId", h.GetTurnDetail)
	g.GET("/turns/:turnId/replay", h.GetTurnReplay)
	g.GET("/stats/aggregate", h.GetAggregateStats)
	g.GET("/stats/models", h.GetModelBreakdown)
	g.GET("/stats/tools", h.GetToolStats)
	g.GET("/stats/timeseries", h.GetTimeseries)
	g.GET("/audit/:conversationId", h.GetUnifiedAudit)
}

// SessionInfo represents a conversation summary for the session list.
type SessionInfo struct {
	ConversationID string  `json:"conversation_id"`
	UserID         string  `json:"user_id"`
	TurnCount      int     `json:"turn_count"`
	TotalCost      float64 `json:"total_cost"`
	TotalTokens    int64   `json:"total_tokens"`
	FirstTurnAt    string  `json:"first_turn_at"`
	LastTurnAt     string  `json:"last_turn_at"`
	Model          string  `json:"model"`
}

// ListSessions returns all sessions with aggregated stats.
func (h *DevHandler) ListSessions(c echo.Context) error {
	if h.readDB == nil {
		return c.JSON(http.StatusOK, []SessionInfo{})
	}
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	rows, err := h.readDB.Query(`
		SELECT conversation_id,
			COALESCE(MAX(user_id), '') as user_id,
			COUNT(*) as turn_count,
			COALESCE(SUM(cost_usd), 0) as total_cost,
			COALESCE(SUM(input_tokens + output_tokens), 0) as total_tokens,
			MIN(created_at) as first_turn,
			MAX(created_at) as last_turn,
			COALESCE(
				(SELECT model FROM turn_metrics t2
				 WHERE t2.conversation_id = t1.conversation_id
				 ORDER BY t2.created_at DESC LIMIT 1),
				''
			) as model
		FROM turn_metrics t1
		GROUP BY conversation_id
		ORDER BY MAX(created_at) DESC
		LIMIT ?`, limit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	defer rows.Close()

	var sessions []SessionInfo
	for rows.Next() {
		var s SessionInfo
		if err := rows.Scan(&s.ConversationID, &s.UserID, &s.TurnCount, &s.TotalCost, &s.TotalTokens, &s.FirstTurnAt, &s.LastTurnAt, &s.Model); err != nil {
			continue
		}
		sessions = append(sessions, s)
	}
	if sessions == nil {
		sessions = []SessionInfo{}
	}
	return c.JSON(http.StatusOK, sessions)
}

// GetSessionTurns returns the turn timeline for a session.
func (h *DevHandler) GetSessionTurns(c echo.Context) error {
	if h.readDB == nil {
		return c.JSON(http.StatusOK, []TurnMetrics{})
	}
	convID := c.Param("id")
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	rows, err := h.readDB.Query(`
		SELECT id, conversation_id, user_id, turn_id, model, status,
			latency_ms, llm_latency_ms, ttft_ms, input_tokens, output_tokens, cache_read, cache_write,
			cost_usd, tool_calls, tool_count, tool_latency_ms,
			llm_request, llm_response, error_type, created_at
		FROM turn_metrics
		WHERE conversation_id = ?
		ORDER BY created_at ASC
		LIMIT ?`, convID, limit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	defer rows.Close()

	var turns []TurnMetrics
	for rows.Next() {
		var t TurnMetrics
		var toolCallsJSON, createdAt string
		if err := rows.Scan(&t.ID, &t.ConversationID, &t.UserID, &t.TurnID, &t.Model, &t.Status,
			&t.LatencyMs, &t.LLMLatencyMs, &t.TTFTMs, &t.InputTokens, &t.OutputTokens, &t.CacheRead, &t.CacheWrite,
			&t.CostUSD, &toolCallsJSON, &t.ToolCount, &t.ToolLatencyMs,
			&t.LLMRequest, &t.LLMResponse, &t.ErrorType, &createdAt); err != nil {
			continue
		}
		t.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		turns = append(turns, t)
	}
	if turns == nil {
		turns = []TurnMetrics{}
	}
	return c.JSON(http.StatusOK, turns)
}

// GetTurnDetail returns full details for a single turn.
func (h *DevHandler) GetTurnDetail(c echo.Context) error {
	if h.readDB == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
	}
	turnID := c.Param("turnId")

	var t TurnMetrics
	var toolCallsJSON, createdAt string
	err := h.readDB.QueryRow(`
		SELECT id, conversation_id, user_id, turn_id, model, status,
			latency_ms, llm_latency_ms, ttft_ms, input_tokens, output_tokens, cache_read, cache_write,
			cost_usd, tool_calls, tool_count, tool_latency_ms,
			llm_request, llm_response, error_type, created_at
		FROM turn_metrics
		WHERE turn_id = ?`, turnID).Scan(
		&t.ID, &t.ConversationID, &t.UserID, &t.TurnID, &t.Model, &t.Status,
		&t.LatencyMs, &t.LLMLatencyMs, &t.TTFTMs, &t.InputTokens, &t.OutputTokens, &t.CacheRead, &t.CacheWrite,
		&t.CostUSD, &toolCallsJSON, &t.ToolCount, &t.ToolLatencyMs,
		&t.LLMRequest, &t.LLMResponse, &t.ErrorType, &createdAt)
	if err == sql.ErrNoRows {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "turn not found"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	t.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	return c.JSON(http.StatusOK, t)
}

// GetTurnReplay returns the LLM request payload for replay.
func (h *DevHandler) GetTurnReplay(c echo.Context) error {
	if h.readDB == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
	}
	turnID := c.Param("turnId")

	var llmRequest, llmResponse, model string
	err := h.readDB.QueryRow(`
		SELECT llm_request, llm_response, model FROM turn_metrics WHERE turn_id = ?`, turnID).
		Scan(&llmRequest, &llmResponse, &model)
	if err == sql.ErrNoRows {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "turn not found"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"turn_id":     turnID,
		"model":       model,
		"llm_request": json.RawMessage(llmRequest),
		"llm_response": json.RawMessage(llmResponse),
	})
}

// AggregateStats represents overall dashboard statistics.
type AggregateStats struct {
	TotalTurns     int     `json:"total_turns"`
	TotalCost      float64 `json:"total_cost"`
	TotalTokens    int64   `json:"total_tokens"`
	AvgLatencyMs   float64 `json:"avg_latency_ms"`
	P50LatencyMs   float64 `json:"p50_latency_ms"`
	P95LatencyMs   float64 `json:"p95_latency_ms"`
	ErrorRate      float64 `json:"error_rate"`
	CacheHitRate   float64 `json:"cache_hit_rate"`
	SessionCount   int     `json:"session_count"`
	TotalToolCalls int     `json:"total_tool_calls"`
}

// GetAggregateStats returns overall statistics.
func (h *DevHandler) GetAggregateStats(c echo.Context) error {
	if h.readDB == nil {
		return c.JSON(http.StatusOK, AggregateStats{})
	}

	var stats AggregateStats
	h.readDB.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(cost_usd), 0), COALESCE(SUM(input_tokens + output_tokens), 0),
			COALESCE(AVG(latency_ms), 0), COUNT(DISTINCT conversation_id),
			COALESCE(SUM(tool_count), 0)
		FROM turn_metrics`).Scan(
		&stats.TotalTurns, &stats.TotalCost, &stats.TotalTokens,
		&stats.AvgLatencyMs, &stats.SessionCount, &stats.TotalToolCalls)

	// Error rate
	var total, errors int
	h.readDB.QueryRow(`SELECT COUNT(*), SUM(CASE WHEN status != 'success' THEN 1 ELSE 0 END) FROM turn_metrics`).Scan(&total, &errors)
	if total > 0 {
		stats.ErrorRate = float64(errors) / float64(total)
	}

	// Cache hit rate
	var totalCache, cacheRead int
	h.readDB.QueryRow(`SELECT SUM(input_tokens + output_tokens), SUM(cache_read) FROM turn_metrics`).Scan(&totalCache, &cacheRead)
	if totalCache > 0 {
		stats.CacheHitRate = float64(cacheRead) / float64(totalCache)
	}

	// Latency percentiles
	latencies := make([]float64, 0, 100)
	rows, err := h.readDB.Query(`SELECT latency_ms FROM turn_metrics ORDER BY latency_ms`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var lat float64
			if rows.Scan(&lat) == nil {
				latencies = append(latencies, lat)
			}
		}
	}
	if len(latencies) > 0 {
		stats.P50LatencyMs = latencies[len(latencies)*50/100]
		stats.P95LatencyMs = latencies[len(latencies)*95/100]
	}

	return c.JSON(http.StatusOK, stats)
}

// ModelBreakdown represents per-model statistics.
type ModelBreakdown struct {
	Model      string  `json:"model"`
	TurnCount  int     `json:"turn_count"`
	TotalCost  float64 `json:"total_cost"`
	AvgLatency float64 `json:"avg_latency_ms"`
	TotalIn    int64   `json:"total_input_tokens"`
	TotalOut   int64   `json:"total_output_tokens"`
	ErrorCount int     `json:"error_count"`
}

// GetModelBreakdown returns per-model statistics.
func (h *DevHandler) GetModelBreakdown(c echo.Context) error {
	if h.readDB == nil {
		return c.JSON(http.StatusOK, []ModelBreakdown{})
	}

	rows, err := h.readDB.Query(`
		SELECT model, COUNT(*), COALESCE(SUM(cost_usd), 0), COALESCE(AVG(latency_ms), 0),
			COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0),
			SUM(CASE WHEN status != 'success' THEN 1 ELSE 0 END)
		FROM turn_metrics
		GROUP BY model
		ORDER BY COUNT(*) DESC`)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	defer rows.Close()

	var models []ModelBreakdown
	for rows.Next() {
		var m ModelBreakdown
		if err := rows.Scan(&m.Model, &m.TurnCount, &m.TotalCost, &m.AvgLatency, &m.TotalIn, &m.TotalOut, &m.ErrorCount); err != nil {
			continue
		}
		models = append(models, m)
	}
	if models == nil {
		models = []ModelBreakdown{}
	}
	return c.JSON(http.StatusOK, models)
}

// ToolStats represents per-tool usage statistics.
type ToolStats struct {
	ToolName   string  `json:"tool_name"`
	CallCount  int     `json:"call_count"`
	AvgLatency float64 `json:"avg_latency_ms"`
	Successes  int     `json:"successes"`
	Failures   int     `json:"failures"`
}

// GetToolStats returns per-tool usage statistics from turn_metrics tool_calls JSON.
func (h *DevHandler) GetToolStats(c echo.Context) error {
	if h.readDB == nil {
		return c.JSON(http.StatusOK, []ToolStats{})
	}

	// Parse tool_calls JSON from all turns and aggregate
	rows, err := h.readDB.Query(`SELECT tool_calls FROM turn_metrics WHERE tool_count > 0`)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	defer rows.Close()

	toolMap := make(map[string]*ToolStats)
	for rows.Next() {
		var toolCallsJSON string
		if err := rows.Scan(&toolCallsJSON); err != nil {
			continue
		}
		var calls []ToolCallSummary
		if err := json.Unmarshal([]byte(toolCallsJSON), &calls); err != nil {
			continue
		}
		for _, call := range calls {
			ts, ok := toolMap[call.Name]
			if !ok {
				ts = &ToolStats{ToolName: call.Name}
				toolMap[call.Name] = ts
			}
			ts.CallCount++
			ts.AvgLatency += call.LatencyMs
			if call.Success {
				ts.Successes++
			} else {
				ts.Failures++
			}
		}
	}

	var stats []ToolStats
	for _, ts := range toolMap {
		if ts.CallCount > 0 {
			ts.AvgLatency /= float64(ts.CallCount)
		}
		stats = append(stats, *ts)
	}
	if stats == nil {
		stats = []ToolStats{}
	}
	return c.JSON(http.StatusOK, stats)
}

// TimeseriesPoint represents a time-bucketed metric point.
type TimeseriesPoint struct {
	Bucket   string  `json:"bucket"`
	Turns    int     `json:"turns"`
	Cost     float64 `json:"cost"`
	AvgLat   float64 `json:"avg_latency_ms"`
	Tokens   int64   `json:"tokens"`
}

// GetTimeseries returns time-bucketed metrics (hourly).
func (h *DevHandler) GetTimeseries(c echo.Context) error {
	if h.readDB == nil {
		return c.JSON(http.StatusOK, []TimeseriesPoint{})
	}
	hours, _ := strconv.Atoi(c.QueryParam("hours"))
	if hours <= 0 || hours > 168 {
		hours = 24
	}

	since := time.Now().Add(-time.Duration(hours) * time.Hour).Format(time.RFC3339Nano)

	rows, err := h.readDB.Query(`
		SELECT strftime('%Y-%m-%dT%H:00:00Z', created_at) as bucket,
			COUNT(*), COALESCE(SUM(cost_usd), 0), COALESCE(AVG(latency_ms), 0),
			COALESCE(SUM(input_tokens + output_tokens), 0)
		FROM turn_metrics
		WHERE created_at >= ?
		GROUP BY bucket
		ORDER BY bucket ASC`, since)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	defer rows.Close()

	var points []TimeseriesPoint
	for rows.Next() {
		var p TimeseriesPoint
		if err := rows.Scan(&p.Bucket, &p.Turns, &p.Cost, &p.AvgLat, &p.Tokens); err != nil {
			continue
		}
		points = append(points, p)
	}
	if points == nil {
		points = []TimeseriesPoint{}
	}
	return c.JSON(http.StatusOK, points)
}

// AuditEntry represents a unified audit event.
type AuditEntry struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Source    string `json:"source"`
	Summary   string `json:"summary"`
	Detail    string `json:"detail,omitempty"`
}

// GetUnifiedAudit returns a merged timeline of session audit + exec audit + turn metrics.
func (h *DevHandler) GetUnifiedAudit(c echo.Context) error {
	if h.readDB == nil {
		return c.JSON(http.StatusOK, []AuditEntry{})
	}
	convID := c.Param("conversationId")
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	var entries []AuditEntry

	// Session tool audit entries
	rows, err := h.readDB.Query(`
		SELECT id, created_at, event_type, tool_name, payload_bytes, is_error
		FROM session_tool_audit_logs
		WHERE conversation_id = ?
		ORDER BY created_at DESC
		LIMIT ?`, convID, limit)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var e AuditEntry
			var payloadBytes int
			var isError int
			if err := rows.Scan(&e.ID, &e.Timestamp, &e.Type, &e.Source, &payloadBytes, &isError); err != nil {
				continue
			}
			e.Summary = e.Type + " " + e.Source
			entries = append(entries, e)
		}
	}

	// Turn metrics entries
	rows2, err := h.readDB.Query(`
		SELECT turn_id, created_at, model, status, tool_count, latency_ms
		FROM turn_metrics
		WHERE conversation_id = ?
		ORDER BY created_at DESC
		LIMIT ?`, convID, limit)
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var e AuditEntry
			var model, status string
			var toolCount int
			var latencyMs float64
			if err := rows2.Scan(&e.ID, &e.Timestamp, &model, &status, &toolCount, &latencyMs); err != nil {
				continue
			}
			e.Type = "turn"
			e.Source = model
			e.Summary = "turn completed: " + status
			entries = append(entries, e)
		}
	}

	if entries == nil {
		entries = []AuditEntry{}
	}
	return c.JSON(http.StatusOK, entries)
}
