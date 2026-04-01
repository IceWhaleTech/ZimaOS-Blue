package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	z "github.com/IceWhaleTech/zorm"
)

func (s *Store) shouldExternalizeAttachments() bool {
	return s != nil && s.options.AttachmentExternalStore && strings.TrimSpace(s.options.AttachmentDir) != ""
}

func (s *Store) persistExternalAttachmentsTx(ctx context.Context, tx *sql.Tx, messageID string, attachments []MessageAttachment, createdAt time.Time) error {
	if !s.shouldExternalizeAttachments() || len(attachments) == 0 {
		return nil
	}
	baseDir := filepath.Join(s.options.AttachmentDir, messageID)
	if err := os.MkdirAll(baseDir, 0o750); err != nil {
		return fmt.Errorf("create message attachment dir: %w", err)
	}

	createdAtValue := createdAt.UTC().Format(time.RFC3339Nano)
	writtenPaths := make([]string, 0, len(attachments))
	cleanupFiles := func() {
		for _, path := range writtenPaths {
			_ = os.Remove(path)
		}
	}

	for idx, attachment := range attachments {
		filePath := filepath.Join(baseDir, fmt.Sprintf("%04d.json", idx))
		payload, err := json.Marshal(attachment)
		if err != nil {
			cleanupFiles()
			return fmt.Errorf("marshal attachment %d: %w", idx, err)
		}
		if err := os.WriteFile(filePath, payload, 0o600); err != nil {
			cleanupFiles()
			return fmt.Errorf("write attachment %d: %w", idx, err)
		}
		writtenPaths = append(writtenPaths, filePath)

		if _, err := z.TableContext(ctx, tx, "message_attachments").Insert(z.V{
			"message_id":       messageID,
			"attachment_index": idx,
			"type":             strings.TrimSpace(attachment.Type),
			"name":             strings.TrimSpace(attachment.Name),
			"mime_type":        strings.TrimSpace(attachment.MimeType),
			"duration":         attachment.Duration,
			"file_path":        filePath,
			"created_at":       createdAtValue,
		}); err != nil {
			cleanupFiles()
			return fmt.Errorf("insert message attachment %d: %w", idx, err)
		}
	}

	return nil
}

func (s *Store) loadExternalAttachments(ctx context.Context, messageID string) ([]MessageAttachment, error) {
	if s == nil || s.reader() == nil || strings.TrimSpace(messageID) == "" {
		return nil, nil
	}
	var rows []messageAttachmentFileRow
	_, err := s.attachmentsRead(ctx).Select(&rows,
		z.Fields("file_path"),
		z.Where(z.Eq("message_id", messageID)),
		z.OrderBy("attachment_index ASC"),
	)
	if err != nil {
		return nil, fmt.Errorf("query external attachments: %w", err)
	}

	attachments := make([]MessageAttachment, 0, len(rows))
	for i := range rows {
		raw, err := os.ReadFile(rows[i].FilePath)
		if err != nil {
			return nil, fmt.Errorf("read external attachment %q: %w", rows[i].FilePath, err)
		}
		var attachment MessageAttachment
		if err := json.Unmarshal(raw, &attachment); err != nil {
			return nil, fmt.Errorf("decode external attachment %q: %w", rows[i].FilePath, err)
		}
		attachments = append(attachments, attachment)
	}
	return attachments, nil
}

func parseLegacyAttachments(raw *string) ([]MessageAttachment, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	var attachments []MessageAttachment
	if err := json.Unmarshal([]byte(*raw), &attachments); err != nil {
		return nil, fmt.Errorf("failed to unmarshal attachments: %w", err)
	}
	return attachments, nil
}
