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

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO message_attachments (
				message_id, attachment_index, type, name, mime_type, duration, file_path, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			messageID,
			idx,
			strings.TrimSpace(attachment.Type),
			strings.TrimSpace(attachment.Name),
			strings.TrimSpace(attachment.MimeType),
			attachment.Duration,
			filePath,
			createdAtValue,
		); err != nil {
			cleanupFiles()
			return fmt.Errorf("insert message attachment %d: %w", idx, err)
		}
	}

	return nil
}

func (s *Store) loadExternalAttachments(ctx context.Context, messageID string) ([]MessageAttachment, error) {
	if s == nil || s.db == nil || strings.TrimSpace(messageID) == "" {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT file_path FROM message_attachments WHERE message_id = ? ORDER BY attachment_index ASC`,
		messageID,
	)
	if err != nil {
		return nil, fmt.Errorf("query external attachments: %w", err)
	}
	defer rows.Close()

	var attachments []MessageAttachment
	for rows.Next() {
		var filePath string
		if err := rows.Scan(&filePath); err != nil {
			return nil, fmt.Errorf("scan external attachment: %w", err)
		}
		raw, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("read external attachment %q: %w", filePath, err)
		}
		var attachment MessageAttachment
		if err := json.Unmarshal(raw, &attachment); err != nil {
			return nil, fmt.Errorf("decode external attachment %q: %w", filePath, err)
		}
		attachments = append(attachments, attachment)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return attachments, nil
}

func parseLegacyAttachments(raw sql.NullString) ([]MessageAttachment, error) {
	if !raw.Valid || strings.TrimSpace(raw.String) == "" {
		return nil, nil
	}
	var attachments []MessageAttachment
	if err := json.Unmarshal([]byte(raw.String), &attachments); err != nil {
		return nil, fmt.Errorf("failed to unmarshal attachments: %w", err)
	}
	return attachments, nil
}
