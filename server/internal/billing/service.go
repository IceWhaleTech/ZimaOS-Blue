package billing

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
)

const (
	defaultPageSize = 50
	maxPageSize     = 500
)

var errStorageNotConfigured = errors.New("billing storage is not configured")

// Service provides billing query/export operations.
type Service struct {
	storage providerpool.Storage
}

// NewService creates a billing service.
func NewService(storage providerpool.Storage) *Service {
	return &Service{storage: storage}
}

// GetSummary returns aggregate billing data for a range.
func (s *Service) GetSummary(opts QueryOptions) (*SummaryResponse, error) {
	records, err := s.loadRecords(opts)
	if err != nil {
		return nil, err
	}

	resp := &SummaryResponse{
		From:     opts.Start,
		To:       opts.End,
		Currency: "USD",
		GroupBy:  normalizeGroupBy(opts.GroupBy),
	}

	groups := make(map[string]*SummaryBreakdown)
	for _, rec := range records {
		addToTotals(&resp.Totals, rec)

		key, bucket := summaryBucket(rec, resp.GroupBy)
		entry, ok := groups[key]
		if !ok {
			entry = &SummaryBreakdown{Key: key}
			if bucket.Day != "" {
				entry.Day = bucket.Day
			}
			if bucket.ProviderID != "" {
				entry.ProviderID = bucket.ProviderID
			}
			if bucket.ModelID != "" {
				entry.ModelID = bucket.ModelID
			}
			groups[key] = entry
		}
		addToTotals(&entry.Totals, rec)
	}

	resp.Breakdown = make([]SummaryBreakdown, 0, len(groups))
	for _, item := range groups {
		resp.Breakdown = append(resp.Breakdown, *item)
	}
	sort.Slice(resp.Breakdown, func(i, j int) bool {
		return resp.Breakdown[i].Key < resp.Breakdown[j].Key
	})

	return resp, nil
}

// GetLines returns paginated billing line items.
func (s *Service) GetLines(opts QueryOptions, page, pageSize int) (*LinesResponse, error) {
	records, err := s.loadRecords(opts)
	if err != nil {
		return nil, err
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].Timestamp.After(records[j].Timestamp)
	})

	page = clampPage(page)
	pageSize = clampPageSize(pageSize)
	startIdx := (page - 1) * pageSize
	endIdx := startIdx + pageSize
	if startIdx > len(records) {
		startIdx = len(records)
	}
	if endIdx > len(records) {
		endIdx = len(records)
	}

	items := make([]LineItem, 0, endIdx-startIdx)
	for _, rec := range records[startIdx:endIdx] {
		items = append(items, toLineItem(rec))
	}

	return &LinesResponse{
		From:     opts.Start,
		To:       opts.End,
		Page:     page,
		PageSize: pageSize,
		Total:    len(records),
		Items:    items,
	}, nil
}

// ExportCSV exports filtered billing lines as CSV bytes.
func (s *Service) ExportCSV(opts QueryOptions) ([]byte, error) {
	records, err := s.loadRecords(opts)
	if err != nil {
		return nil, err
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].Timestamp.After(records[j].Timestamp)
	})

	buf := &bytes.Buffer{}
	w := csv.NewWriter(buf)
	if err := w.Write([]string{
		"timestamp",
		"provider_id",
		"model_id",
		"input_tokens",
		"output_tokens",
		"cache_read_tokens",
		"cache_write_tokens",
		"total_tokens",
		"estimated_cost",
		"request_count",
		"success",
		"latency_ms",
		"user_id",
		"session_id",
	}); err != nil {
		return nil, err
	}

	for _, rec := range records {
		item := toLineItem(rec)
		if err := w.Write([]string{
			item.Timestamp.Format(timeFormatRFC3339),
			item.ProviderID,
			item.ModelID,
			fmt.Sprintf("%d", item.InputTokens),
			fmt.Sprintf("%d", item.OutputTokens),
			fmt.Sprintf("%d", item.CacheReadTokens),
			fmt.Sprintf("%d", item.CacheWriteTokens),
			fmt.Sprintf("%d", item.TotalTokens),
			fmt.Sprintf("%.8f", item.EstimatedCost),
			fmt.Sprintf("%d", item.RequestCount),
			fmt.Sprintf("%t", item.Success),
			fmt.Sprintf("%d", item.LatencyMs),
			item.UserID,
			item.SessionID,
		}); err != nil {
			return nil, err
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type summaryBucketKey struct {
	ProviderID string
	ModelID    string
	Day        string
}

func summaryBucket(rec *providerpool.UsageRecord, groupBy string) (string, summaryBucketKey) {
	groupBy = normalizeGroupBy(groupBy)
	switch groupBy {
	case "provider":
		provider := strings.TrimSpace(rec.ProviderID)
		if provider == "" {
			provider = "unknown"
		}
		return provider, summaryBucketKey{ProviderID: provider}
	case "model":
		model := strings.TrimSpace(rec.ModelID)
		if model == "" {
			model = "unknown"
		}
		return model, summaryBucketKey{ModelID: model}
	default:
		day := rec.Timestamp.Format("2006-01-02")
		return day, summaryBucketKey{Day: day}
	}
}

func normalizeGroupBy(groupBy string) string {
	switch strings.ToLower(strings.TrimSpace(groupBy)) {
	case "provider", "model", "day":
		return strings.ToLower(strings.TrimSpace(groupBy))
	default:
		return "day"
	}
}

func (s *Service) loadRecords(opts QueryOptions) ([]*providerpool.UsageRecord, error) {
	if s == nil || s.storage == nil {
		return nil, errStorageNotConfigured
	}
	records, err := s.storage.LoadUsage(strings.TrimSpace(opts.ProviderID), opts.Start, opts.End)
	if err != nil {
		return nil, err
	}

	modelFilter := strings.TrimSpace(opts.ModelID)
	if modelFilter == "" {
		return records, nil
	}

	out := make([]*providerpool.UsageRecord, 0, len(records))
	for _, rec := range records {
		if strings.EqualFold(strings.TrimSpace(rec.ModelID), modelFilter) {
			out = append(out, rec)
		}
	}
	return out, nil
}

func addToTotals(t *Totals, rec *providerpool.UsageRecord) {
	if t == nil || rec == nil {
		return
	}
	t.InputTokens += rec.InputTokens
	t.OutputTokens += rec.OutputTokens
	t.CacheReadTokens += rec.CacheReadTokens
	t.CacheWriteTokens += rec.CacheWriteTokens
	t.TotalTokens += rec.InputTokens + rec.OutputTokens
	t.RequestCount += rec.RequestCount
	if rec.Success {
		t.SuccessCount += rec.RequestCount
	} else {
		t.FailureCount += rec.RequestCount
	}
	t.EstimatedCost += rec.EstimatedCost
}

func toLineItem(rec *providerpool.UsageRecord) LineItem {
	if rec == nil {
		return LineItem{}
	}
	providerID := strings.TrimSpace(rec.ProviderID)
	if providerID == "" {
		providerID = "unknown"
	}
	modelID := strings.TrimSpace(rec.ModelID)
	if modelID == "" {
		modelID = "unknown"
	}

	return LineItem{
		Timestamp:        rec.Timestamp,
		ProviderID:       providerID,
		ModelID:          modelID,
		InputTokens:      rec.InputTokens,
		OutputTokens:     rec.OutputTokens,
		CacheReadTokens:  rec.CacheReadTokens,
		CacheWriteTokens: rec.CacheWriteTokens,
		TotalTokens:      rec.InputTokens + rec.OutputTokens,
		EstimatedCost:    rec.EstimatedCost,
		RequestCount:     rec.RequestCount,
		Success:          rec.Success,
		LatencyMs:        rec.LatencyMs,
		UserID:           strings.TrimSpace(rec.UserID),
		SessionID:        strings.TrimSpace(rec.SessionID),
	}
}

func clampPage(page int) int {
	if page < 1 {
		return 1
	}
	return page
}

func clampPageSize(size int) int {
	if size <= 0 {
		return defaultPageSize
	}
	if size > maxPageSize {
		return maxPageSize
	}
	return size
}
