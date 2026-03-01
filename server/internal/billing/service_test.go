package billing

import (
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
)

type mockStorage struct {
	records []*providerpool.UsageRecord
}

func (m *mockStorage) SaveProvider(provider *providerpool.Provider) error               { return nil }
func (m *mockStorage) LoadProvider(id string) (*providerpool.Provider, error)           { return nil, nil }
func (m *mockStorage) LoadAllProviders() ([]*providerpool.Provider, error)              { return nil, nil }
func (m *mockStorage) DeleteProvider(id string) error                                   { return nil }
func (m *mockStorage) SaveModels(providerID string, models []*providerpool.Model) error { return nil }
func (m *mockStorage) LoadModels(providerID string) ([]*providerpool.Model, error)      { return nil, nil }
func (m *mockStorage) AppendUsage(record *providerpool.UsageRecord) error               { return nil }
func (m *mockStorage) SavePricingConfig(config *providerpool.PricingConfig) error       { return nil }
func (m *mockStorage) LoadPricingConfig() (*providerpool.PricingConfig, error)          { return nil, nil }
func (m *mockStorage) SaveConfig(config *providerpool.PoolConfig) error                 { return nil }
func (m *mockStorage) LoadConfig() (*providerpool.PoolConfig, error)                    { return nil, nil }

func (m *mockStorage) LoadUsage(providerID string, start, end time.Time) ([]*providerpool.UsageRecord, error) {
	out := make([]*providerpool.UsageRecord, 0, len(m.records))
	for _, rec := range m.records {
		if rec == nil {
			continue
		}
		if providerID != "" && rec.ProviderID != providerID {
			continue
		}
		if !rec.Timestamp.After(start) || !rec.Timestamp.Before(end) {
			continue
		}
		out = append(out, rec)
	}
	return out, nil
}

func TestServiceGetSummary(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	storage := &mockStorage{records: []*providerpool.UsageRecord{
		{
			ProviderID:    "p1",
			ModelID:       "m1",
			Timestamp:     now.Add(-2 * time.Hour),
			InputTokens:   100,
			OutputTokens:  40,
			RequestCount:  1,
			Success:       true,
			EstimatedCost: 0.012,
		},
		{
			ProviderID:    "p2",
			ModelID:       "m2",
			Timestamp:     now.Add(-90 * time.Minute),
			InputTokens:   70,
			OutputTokens:  30,
			RequestCount:  1,
			Success:       false,
			EstimatedCost: 0.02,
		},
	}}

	svc := NewService(storage)
	resp, err := svc.GetSummary(QueryOptions{
		Start:   now.Add(-24 * time.Hour),
		End:     now,
		GroupBy: "provider",
	})
	if err != nil {
		t.Fatalf("GetSummary failed: %v", err)
	}

	if resp.GroupBy != "provider" {
		t.Fatalf("group_by=%q, want provider", resp.GroupBy)
	}
	if resp.Totals.RequestCount != 2 {
		t.Fatalf("request_count=%d, want 2", resp.Totals.RequestCount)
	}
	if resp.Totals.SuccessCount != 1 || resp.Totals.FailureCount != 1 {
		t.Fatalf("success/failure=%d/%d, want 1/1", resp.Totals.SuccessCount, resp.Totals.FailureCount)
	}
	if len(resp.Breakdown) != 2 {
		t.Fatalf("breakdown len=%d, want 2", len(resp.Breakdown))
	}
}

func TestServiceGetLinesPagination(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	storage := &mockStorage{records: []*providerpool.UsageRecord{
		{ProviderID: "p1", ModelID: "m1", Timestamp: now.Add(-3 * time.Hour), InputTokens: 10, OutputTokens: 4, RequestCount: 1, Success: true},
		{ProviderID: "p1", ModelID: "m1", Timestamp: now.Add(-2 * time.Hour), InputTokens: 20, OutputTokens: 5, RequestCount: 1, Success: true},
		{ProviderID: "p1", ModelID: "m1", Timestamp: now.Add(-1 * time.Hour), InputTokens: 30, OutputTokens: 6, RequestCount: 1, Success: true},
	}}

	svc := NewService(storage)
	resp, err := svc.GetLines(QueryOptions{
		Start:      now.Add(-24 * time.Hour),
		End:        now,
		ProviderID: "p1",
	}, 2, 1)
	if err != nil {
		t.Fatalf("GetLines failed: %v", err)
	}

	if resp.Total != 3 {
		t.Fatalf("total=%d, want 3", resp.Total)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("items len=%d, want 1", len(resp.Items))
	}
	if got := resp.Items[0].InputTokens; got != 20 {
		t.Fatalf("page item input_tokens=%d, want 20", got)
	}
}

func TestServiceExportCSV(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	storage := &mockStorage{records: []*providerpool.UsageRecord{
		{ProviderID: "p1", ModelID: "m1", Timestamp: now.Add(-2 * time.Hour), InputTokens: 50, OutputTokens: 25, RequestCount: 1, Success: true, EstimatedCost: 0.004},
	}}

	svc := NewService(storage)
	csvBytes, err := svc.ExportCSV(QueryOptions{Start: now.Add(-24 * time.Hour), End: now})
	if err != nil {
		t.Fatalf("ExportCSV failed: %v", err)
	}

	out := string(csvBytes)
	if !strings.Contains(out, "timestamp,provider_id,model_id") {
		t.Fatalf("csv header missing, got: %s", out)
	}
	if !strings.Contains(out, "p1,m1") {
		t.Fatalf("csv row missing provider/model, got: %s", out)
	}
}
