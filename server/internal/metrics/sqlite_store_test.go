package metrics

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSQLiteStore(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_metrics.db")

	store, err := NewSQLiteStore(dbPath)
	require.NoError(t, err)
	require.NotNil(t, store)
	defer store.Close()

	// Verify database file was created
	_, err = os.Stat(dbPath)
	assert.NoError(t, err)
}

func TestNewSQLiteStore_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "subdir", "nested", "test_metrics.db")

	store, err := NewSQLiteStore(dbPath)
	require.NoError(t, err)
	require.NotNil(t, store)
	defer store.Close()

	// Verify directory was created
	_, err = os.Stat(filepath.Dir(dbPath))
	assert.NoError(t, err)
}

func TestSQLiteStore_TokenUsage(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_metrics.db")

	store, err := NewSQLiteStore(dbPath)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	// Test saving token usage
	usage := &TokenUsage{
		InputTokens:      1000,
		OutputTokens:     500,
		TotalTokens:      1500,
		CacheReadTokens:  200,
		CacheWriteTokens: 100,
		EstimatedCost:    0.0025,
	}

	err = store.SaveTokenUsage(ctx, usage)
	require.NoError(t, err)

	// Test loading token usage
	loaded, err := store.LoadTokenUsage(ctx)
	require.NoError(t, err)
	require.NotNil(t, loaded)

	assert.Equal(t, usage.InputTokens, loaded.InputTokens)
	assert.Equal(t, usage.OutputTokens, loaded.OutputTokens)
	assert.Equal(t, usage.TotalTokens, loaded.TotalTokens)
	assert.Equal(t, usage.CacheReadTokens, loaded.CacheReadTokens)
	assert.Equal(t, usage.CacheWriteTokens, loaded.CacheWriteTokens)
	assert.InDelta(t, usage.EstimatedCost, loaded.EstimatedCost, 0.0001)
}

func TestSQLiteStore_TokenUsage_Update(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_metrics.db")

	store, err := NewSQLiteStore(dbPath)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	// Save initial usage
	usage1 := &TokenUsage{
		InputTokens:   1000,
		OutputTokens:  500,
		TotalTokens:   1500,
		EstimatedCost: 0.0025,
	}
	err = store.SaveTokenUsage(ctx, usage1)
	require.NoError(t, err)

	// Update with new values
	usage2 := &TokenUsage{
		InputTokens:   2000,
		OutputTokens:  1000,
		TotalTokens:   3000,
		EstimatedCost: 0.0050,
	}
	err = store.SaveTokenUsage(ctx, usage2)
	require.NoError(t, err)

	// Verify updated values
	loaded, err := store.LoadTokenUsage(ctx)
	require.NoError(t, err)

	assert.Equal(t, usage2.InputTokens, loaded.InputTokens)
	assert.Equal(t, usage2.OutputTokens, loaded.OutputTokens)
	assert.Equal(t, usage2.TotalTokens, loaded.TotalTokens)
	assert.InDelta(t, usage2.EstimatedCost, loaded.EstimatedCost, 0.0001)
}

func TestSQLiteStore_ModelMetrics(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_metrics.db")

	store, err := NewSQLiteStore(dbPath)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	// Test saving model metrics
	stats := []ModelStats{
		{
			Model:            "gpt-4",
			Calls:            100,
			SuccessfulCalls:  95,
			FailedCalls:      5,
			InputTokens:      50000,
			OutputTokens:     25000,
			TotalTokens:      75000,
			CacheReadTokens:  10000,
			CacheWriteTokens: 5000,
			EstimatedCost:    1.25,
			AvgLatency:       150.5,
		},
		{
			Model:           "claude-3-sonnet",
			Calls:           50,
			SuccessfulCalls: 48,
			FailedCalls:     2,
			InputTokens:     30000,
			OutputTokens:    15000,
			TotalTokens:     45000,
			EstimatedCost:   0.75,
			AvgLatency:      200.0,
		},
	}

	err = store.SaveModelMetrics(ctx, stats)
	require.NoError(t, err)

	// Test loading model metrics
	loaded, err := store.LoadModelMetrics(ctx)
	require.NoError(t, err)
	require.Len(t, loaded, 2)

	// Find gpt-4 stats
	var gpt4Stats *ModelStats
	for i := range loaded {
		if loaded[i].Model == "gpt-4" {
			gpt4Stats = &loaded[i]
			break
		}
	}
	require.NotNil(t, gpt4Stats)

	assert.Equal(t, int64(100), gpt4Stats.Calls)
	assert.Equal(t, int64(95), gpt4Stats.SuccessfulCalls)
	assert.Equal(t, int64(5), gpt4Stats.FailedCalls)
	assert.Equal(t, int64(50000), gpt4Stats.InputTokens)
	assert.Equal(t, int64(25000), gpt4Stats.OutputTokens)
	assert.Equal(t, int64(75000), gpt4Stats.TotalTokens)
	assert.InDelta(t, 1.25, gpt4Stats.EstimatedCost, 0.01)
	assert.InDelta(t, 95.0, gpt4Stats.SuccessRate, 0.1)
	assert.InDelta(t, 150.5, gpt4Stats.AvgLatency, 0.1)
}

func TestSQLiteStore_ModelMetrics_Upsert(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_metrics.db")

	store, err := NewSQLiteStore(dbPath)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	// Save initial stats
	stats1 := []ModelStats{
		{
			Model:           "gpt-4",
			Calls:           100,
			SuccessfulCalls: 95,
			EstimatedCost:   1.25,
		},
	}
	err = store.SaveModelMetrics(ctx, stats1)
	require.NoError(t, err)

	// Update with new values (upsert)
	stats2 := []ModelStats{
		{
			Model:           "gpt-4",
			Calls:           200,
			SuccessfulCalls: 190,
			EstimatedCost:   2.50,
		},
	}
	err = store.SaveModelMetrics(ctx, stats2)
	require.NoError(t, err)

	// Verify updated values
	loaded, err := store.LoadModelMetrics(ctx)
	require.NoError(t, err)
	require.Len(t, loaded, 1)

	assert.Equal(t, "gpt-4", loaded[0].Model)
	assert.Equal(t, int64(200), loaded[0].Calls)
	assert.Equal(t, int64(190), loaded[0].SuccessfulCalls)
	assert.InDelta(t, 2.50, loaded[0].EstimatedCost, 0.01)
}

func TestSQLiteStore_ModelTokenUsage(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_metrics.db")

	store, err := NewSQLiteStore(dbPath)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	// Test saving model token usage
	usages := []ModelTokenUsage{
		{
			Model:            "gpt-4",
			InputTokens:      50000,
			OutputTokens:     25000,
			TotalTokens:      75000,
			CacheReadTokens:  10000,
			CacheWriteTokens: 5000,
			EstimatedCost:    1.25,
		},
		{
			Model:         "claude-3-sonnet",
			InputTokens:   30000,
			OutputTokens:  15000,
			TotalTokens:   45000,
			EstimatedCost: 0.75,
		},
	}

	err = store.SaveModelTokenUsage(ctx, usages)
	require.NoError(t, err)

	// Test loading model token usage
	loaded, err := store.LoadModelTokenUsage(ctx)
	require.NoError(t, err)
	require.Len(t, loaded, 2)

	// Find gpt-4 usage
	var gpt4Usage *ModelTokenUsage
	for i := range loaded {
		if loaded[i].Model == "gpt-4" {
			gpt4Usage = &loaded[i]
			break
		}
	}
	require.NotNil(t, gpt4Usage)

	assert.Equal(t, int64(50000), gpt4Usage.InputTokens)
	assert.Equal(t, int64(25000), gpt4Usage.OutputTokens)
	assert.Equal(t, int64(75000), gpt4Usage.TotalTokens)
	assert.Equal(t, int64(10000), gpt4Usage.CacheReadTokens)
	assert.Equal(t, int64(5000), gpt4Usage.CacheWriteTokens)
	assert.InDelta(t, 1.25, gpt4Usage.EstimatedCost, 0.01)
}

func TestSQLiteStore_WritePoint(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_metrics.db")

	store, err := NewSQLiteStore(dbPath)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	// Write a point
	point := &Point{
		Measurement: "api_calls",
		Tags:        map[string]string{"model": "gpt-4", "status": "success"},
		Fields:      map[string]interface{}{"latency": 150.5, "tokens": 1000},
		Timestamp:   time.Now(),
	}

	err = store.Write(ctx, point)
	require.NoError(t, err)

	// Query the point
	start := time.Now().Add(-1 * time.Hour)
	end := time.Now().Add(1 * time.Hour)
	result, err := store.QueryRange(ctx, "api_calls", start, end, "")
	require.NoError(t, err)
	require.Len(t, result.Series, 1)
	require.Len(t, result.Series[0].Values, 1)
}

func TestSQLiteStore_WriteBatch(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_metrics.db")

	store, err := NewSQLiteStore(dbPath)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	// Write multiple points
	now := time.Now()
	points := []*Point{
		{
			Measurement: "api_calls",
			Tags:        map[string]string{"model": "gpt-4"},
			Fields:      map[string]interface{}{"latency": 100.0},
			Timestamp:   now.Add(-2 * time.Minute),
		},
		{
			Measurement: "api_calls",
			Tags:        map[string]string{"model": "gpt-4"},
			Fields:      map[string]interface{}{"latency": 150.0},
			Timestamp:   now.Add(-1 * time.Minute),
		},
		{
			Measurement: "api_calls",
			Tags:        map[string]string{"model": "gpt-4"},
			Fields:      map[string]interface{}{"latency": 200.0},
			Timestamp:   now,
		},
	}

	err = store.WriteBatch(ctx, points)
	require.NoError(t, err)

	// Query the points
	start := now.Add(-1 * time.Hour)
	end := now.Add(1 * time.Hour)
	result, err := store.QueryRange(ctx, "api_calls", start, end, "")
	require.NoError(t, err)
	require.Len(t, result.Series, 1)
	require.Len(t, result.Series[0].Values, 3)
}

func TestSQLiteStore_Cleanup(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_metrics.db")

	store, err := NewSQLiteStore(dbPath)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	// Write points with different timestamps
	now := time.Now()
	points := []*Point{
		{
			Measurement: "api_calls",
			Tags:        map[string]string{"model": "gpt-4"},
			Fields:      map[string]interface{}{"latency": 100.0},
			Timestamp:   now.Add(-2 * time.Hour), // Old point
		},
		{
			Measurement: "api_calls",
			Tags:        map[string]string{"model": "gpt-4"},
			Fields:      map[string]interface{}{"latency": 150.0},
			Timestamp:   now, // Recent point
		},
	}

	err = store.WriteBatch(ctx, points)
	require.NoError(t, err)

	// Cleanup old data (retention: 1 hour)
	err = store.Cleanup(ctx, 1*time.Hour)
	require.NoError(t, err)

	// Query remaining points
	start := now.Add(-3 * time.Hour)
	end := now.Add(1 * time.Hour)
	result, err := store.QueryRange(ctx, "api_calls", start, end, "")
	require.NoError(t, err)
	require.Len(t, result.Series, 1)
	require.Len(t, result.Series[0].Values, 1) // Only recent point should remain
}

func TestSQLiteStore_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_metrics.db")

	// Create store and save data
	store1, err := NewSQLiteStore(dbPath)
	require.NoError(t, err)

	ctx := context.Background()

	usage := &TokenUsage{
		InputTokens:   5000,
		OutputTokens:  2500,
		TotalTokens:   7500,
		EstimatedCost: 0.125,
	}
	err = store1.SaveTokenUsage(ctx, usage)
	require.NoError(t, err)

	stats := []ModelStats{
		{
			Model:           "gpt-4",
			Calls:           50,
			SuccessfulCalls: 48,
			EstimatedCost:   0.75,
		},
	}
	err = store1.SaveModelMetrics(ctx, stats)
	require.NoError(t, err)

	// Close the store
	store1.Close()

	// Reopen the store and verify data persisted
	store2, err := NewSQLiteStore(dbPath)
	require.NoError(t, err)
	defer store2.Close()

	// Verify token usage
	loadedUsage, err := store2.LoadTokenUsage(ctx)
	require.NoError(t, err)
	assert.Equal(t, usage.InputTokens, loadedUsage.InputTokens)
	assert.Equal(t, usage.OutputTokens, loadedUsage.OutputTokens)
	assert.Equal(t, usage.TotalTokens, loadedUsage.TotalTokens)
	assert.InDelta(t, usage.EstimatedCost, loadedUsage.EstimatedCost, 0.001)

	// Verify model metrics
	loadedStats, err := store2.LoadModelMetrics(ctx)
	require.NoError(t, err)
	require.Len(t, loadedStats, 1)
	assert.Equal(t, "gpt-4", loadedStats[0].Model)
	assert.Equal(t, int64(50), loadedStats[0].Calls)
	assert.InDelta(t, 0.75, loadedStats[0].EstimatedCost, 0.01)
}

func TestSQLiteStore_EmptyLoad(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_metrics.db")

	store, err := NewSQLiteStore(dbPath)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	// Load from empty database
	usage, err := store.LoadTokenUsage(ctx)
	require.NoError(t, err)
	require.NotNil(t, usage)
	assert.Equal(t, int64(0), usage.InputTokens)

	stats, err := store.LoadModelMetrics(ctx)
	require.NoError(t, err)
	assert.Empty(t, stats)

	modelUsages, err := store.LoadModelTokenUsage(ctx)
	require.NoError(t, err)
	assert.Empty(t, modelUsages)
}

func TestSQLiteStore_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_metrics.db")

	store, err := NewSQLiteStore(dbPath)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	// Run concurrent writes
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			usage := &TokenUsage{
				InputTokens:  int64(idx * 100),
				OutputTokens: int64(idx * 50),
				TotalTokens:  int64(idx * 150),
			}
			err := store.SaveTokenUsage(ctx, usage)
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify data can be loaded
	usage, err := store.LoadTokenUsage(ctx)
	require.NoError(t, err)
	require.NotNil(t, usage)
}

func TestNewSQLiteStoreWithDBDoesNotCloseSharedDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "shared_metrics.db")
	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	defer db.Close()

	store, err := NewSQLiteStoreWithDB(db)
	require.NoError(t, err)
	require.NotNil(t, store)

	err = store.Close()
	require.NoError(t, err)

	_, err = db.Exec("SELECT 1")
	require.NoError(t, err)
}
