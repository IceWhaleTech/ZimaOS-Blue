package billing

import "time"

// QueryOptions controls billing queries.
type QueryOptions struct {
	Start      time.Time
	End        time.Time
	ProviderID string
	ModelID    string
	GroupBy    string
}

// Totals is an aggregate usage/cost snapshot.
type Totals struct {
	InputTokens      int64   `json:"input_tokens"`
	OutputTokens     int64   `json:"output_tokens"`
	CacheReadTokens  int64   `json:"cache_read_tokens"`
	CacheWriteTokens int64   `json:"cache_write_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	RequestCount     int64   `json:"request_count"`
	SuccessCount     int64   `json:"success_count"`
	FailureCount     int64   `json:"failure_count"`
	EstimatedCost    float64 `json:"estimated_cost"`
}

// SummaryBreakdown is one grouped bucket in billing summary.
type SummaryBreakdown struct {
	Key        string `json:"key"`
	ProviderID string `json:"provider_id,omitempty"`
	ModelID    string `json:"model_id,omitempty"`
	Day        string `json:"day,omitempty"`
	Totals
}

// SummaryResponse is the billing summary payload.
type SummaryResponse struct {
	From      time.Time          `json:"from"`
	To        time.Time          `json:"to"`
	Currency  string             `json:"currency"`
	GroupBy   string             `json:"group_by"`
	Totals    Totals             `json:"totals"`
	Breakdown []SummaryBreakdown `json:"breakdown"`
}

// LineItem is one billing line in detailed list/export.
type LineItem struct {
	Timestamp        time.Time `json:"timestamp"`
	ProviderID       string    `json:"provider_id"`
	ModelID          string    `json:"model_id"`
	InputTokens      int64     `json:"input_tokens"`
	OutputTokens     int64     `json:"output_tokens"`
	CacheReadTokens  int64     `json:"cache_read_tokens"`
	CacheWriteTokens int64     `json:"cache_write_tokens"`
	TotalTokens      int64     `json:"total_tokens"`
	EstimatedCost    float64   `json:"estimated_cost"`
	RequestCount     int64     `json:"request_count"`
	Success          bool      `json:"success"`
	LatencyMs        int64     `json:"latency_ms"`
	UserID           string    `json:"user_id,omitempty"`
	SessionID        string    `json:"session_id,omitempty"`
}

// LinesResponse is paginated billing lines.
type LinesResponse struct {
	From     time.Time  `json:"from"`
	To       time.Time  `json:"to"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
	Total    int        `json:"total"`
	Items    []LineItem `json:"items"`
}
