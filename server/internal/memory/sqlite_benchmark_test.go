package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func BenchmarkStoreAddMessageTrustedDurability(b *testing.B) {
	for _, durability := range []string{"full", "normal"} {
		b.Run(durability, func(b *testing.B) {
			ctx := context.Background()
			dbPath := filepath.Join(b.TempDir(), "chat.db")
			opts := DefaultChatStoreOptions(dbPath)
			opts.Durability = durability
			opts.CheckpointInterval = 0
			opts.RuntimeStateCleanupInterval = 0
			opts.AttachmentExternalStore = false

			store, err := NewStoreWithOptions(dbPath, opts)
			if err != nil {
				b.Fatalf("NewStoreWithOptions: %v", err)
			}
			defer store.Close()

			conv, err := store.CreateConversation(ctx, "bench")
			if err != nil {
				b.Fatalf("CreateConversation: %v", err)
			}

			stats := &MessageStats{
				InputTokens:     256,
				OutputTokens:    128,
				TotalTokens:     384,
				LatencyMs:       1200,
				TTFTMs:          300,
				TokensPerSecond: 65.2,
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := store.AddMessageTrusted(ctx, conv.ID, Message{
					Role:     "assistant",
					Content:  fmt.Sprintf("message-%d", i),
					Provider: "bench",
					Model:    "bench-model",
					Stats:    stats,
				})
				if err != nil {
					b.Fatalf("AddMessageTrusted(%d): %v", i, err)
				}
			}
		})
	}
}

func BenchmarkStoreGetRecentMessagesHeavy(b *testing.B) {
	for _, mode := range []struct {
		name       string
		external   bool
		lite       bool
		messageCnt int
	}{
		{name: "legacy_full", external: false, lite: false, messageCnt: 200},
		{name: "legacy_lite", external: false, lite: true, messageCnt: 200},
		{name: "external_full", external: true, lite: false, messageCnt: 200},
		{name: "external_lite", external: true, lite: true, messageCnt: 200},
	} {
		b.Run(mode.name, func(b *testing.B) {
			ctx := context.Background()
			dbDir := b.TempDir()
			dbPath := filepath.Join(dbDir, "chat.db")
			opts := DefaultChatStoreOptions(dbPath)
			opts.Durability = "normal"
			opts.CheckpointInterval = 0
			opts.RuntimeStateCleanupInterval = 0
			opts.AttachmentExternalStore = mode.external
			if mode.external {
				opts.AttachmentDir = filepath.Join(dbDir, "attachments")
			}

			store, err := NewStoreWithOptions(dbPath, opts)
			if err != nil {
				b.Fatalf("NewStoreWithOptions: %v", err)
			}
			defer store.Close()

			conv, err := store.CreateConversation(ctx, "history")
			if err != nil {
				b.Fatalf("CreateConversation: %v", err)
			}

			attachmentPayload := strings.Repeat("A", 24*1024)
			attachments := []MessageAttachment{
				{
					Type:     "file",
					Name:     "artifact.txt",
					MimeType: "text/plain",
					Data:     attachmentPayload,
				},
			}
			stats := &MessageStats{
				InputTokens:     1024,
				OutputTokens:    512,
				TotalTokens:     1536,
				LatencyMs:       2500,
				TTFTMs:          450,
				TokensPerSecond: 72.1,
			}

			for i := 0; i < mode.messageCnt; i++ {
				msg := Message{
					Role:     "assistant",
					Content:  fmt.Sprintf("history-%03d", i),
					Provider: "bench",
					Model:    "bench-model",
					Stats:    stats,
				}
				if i%4 == 0 {
					msg.Attachments = attachments
				}
				if _, err := store.AddMessageTrusted(ctx, conv.ID, msg); err != nil {
					b.Fatalf("AddMessageTrusted(%d): %v", i, err)
				}
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if mode.lite {
					if _, err := store.GetRecentMessagesLite(ctx, conv.ID, 80); err != nil {
						b.Fatalf("GetRecentMessagesLite: %v", err)
					}
				} else {
					if _, err := store.GetRecentMessages(ctx, conv.ID, 80); err != nil {
						b.Fatalf("GetRecentMessages: %v", err)
					}
				}
			}
		})
	}
}

func BenchmarkStoreLegacyAttachmentDecode(b *testing.B) {
	ctx := context.Background()
	dbPath := filepath.Join(b.TempDir(), "legacy.db")
	opts := DefaultChatStoreOptions(dbPath)
	opts.AttachmentExternalStore = false
	opts.CheckpointInterval = 0
	opts.RuntimeStateCleanupInterval = 0

	store, err := NewStoreWithOptions(dbPath, opts)
	if err != nil {
		b.Fatalf("NewStoreWithOptions: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(ctx, "legacy")
	if err != nil {
		b.Fatalf("CreateConversation: %v", err)
	}

	attachments := make([]MessageAttachment, 0, 4)
	for i := 0; i < 4; i++ {
		attachments = append(attachments, MessageAttachment{
			Type:     "file",
			Name:     fmt.Sprintf("file-%d.txt", i),
			MimeType: "text/plain",
			Data:     strings.Repeat("B", 8*1024),
		})
	}
	encoded, err := json.Marshal(attachments)
	if err != nil {
		b.Fatalf("json.Marshal(attachments): %v", err)
	}
	for i := 0; i < 120; i++ {
		if _, err := store.db.ExecContext(ctx,
			`INSERT INTO messages (id, conversation_id, role, content, attachments, has_attachments, created_at)
			 VALUES (?, ?, 'assistant', ?, ?, 1, ?)`,
			fmt.Sprintf("legacy-%03d", i),
			conv.ID,
			fmt.Sprintf("legacy-content-%03d", i),
			string(encoded),
			time.Now().UTC(),
		); err != nil {
			b.Fatalf("insert legacy row %d: %v", i, err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.GetRecentMessages(ctx, conv.ID, 80); err != nil {
			b.Fatalf("GetRecentMessages: %v", err)
		}
	}
}
