package sessionaudit

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
)

func BenchmarkStoreRecordSingleVsBatch(b *testing.B) {
	for _, batchSize := range []int{1, 64} {
		name := fmt.Sprintf("batch_%d", batchSize)
		b.Run(name, func(b *testing.B) {
			dbPath := filepath.Join(b.TempDir(), "audit.db")
			store, err := NewSQLiteStore(dbPath, StoreConfig{
				RetentionDays:      30,
				CleanupBatchSize:   500,
				Durability:         "normal",
				WALAutoCheckpoint:  4000,
				CheckpointInterval: 0,
			})
			if err != nil {
				b.Fatalf("NewSQLiteStore: %v", err)
			}
			defer store.Close()

			ctx := context.Background()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				entries := make([]Entry, 0, batchSize)
				for j := 0; j < batchSize; j++ {
					entries = append(entries, Entry{
						ConversationID: "conv-bench",
						EventType:      "tool_result",
						Role:           "tool",
						ToolCallID:     fmt.Sprintf("tc-%d-%d", i, j),
						ToolName:       "web_search",
						Payload:        `{"ok":true}`,
					})
				}
				if batchSize == 1 {
					if err := store.Record(ctx, entries[0]); err != nil {
						b.Fatalf("Record: %v", err)
					}
				} else {
					if err := store.RecordBatch(ctx, entries); err != nil {
						b.Fatalf("RecordBatch: %v", err)
					}
				}
			}
		})
	}
}
