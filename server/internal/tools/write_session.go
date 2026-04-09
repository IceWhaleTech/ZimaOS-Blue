package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const defaultWriteSessionTTL = 30 * time.Minute

type writeSession struct {
	ID           string
	OwnerID      string
	TargetPath   string
	RelativePath string
	OriginalPath string
	TempPath     string
	BytesWritten int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type WriteSessionManager struct {
	mu          sync.Mutex
	sessions    map[string]*writeSession
	ttl         time.Duration
	maxFileSize int64
}

func NewWriteSessionManager(maxFileSize int64) *WriteSessionManager {
	if maxFileSize <= 0 || maxFileSize > maxFSToolBytes {
		maxFileSize = maxFSToolBytes
	}
	return &WriteSessionManager{
		sessions:    make(map[string]*writeSession),
		ttl:         defaultWriteSessionTTL,
		maxFileSize: maxFileSize,
	}
}

func (m *WriteSessionManager) Begin(absPath, relPath, originalPath, ownerID string, createDirs bool) (*writeSession, error) {
	if m == nil {
		return nil, errors.New("write session manager is not configured")
	}

	now := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	m.pruneExpiredLocked(now)

	if info, err := os.Stat(absPath); err == nil && info.IsDir() {
		return nil, fmt.Errorf("path is a directory: %s", relPath)
	}

	dir := filepath.Dir(absPath)
	if createDirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create directory: %w", err)
		}
	}

	pattern := "." + filepath.Base(absPath) + ".blue-write-*"
	tempFile, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary file: %w", err)
	}
	tempPath := tempFile.Name()
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return nil, fmt.Errorf("failed to close temporary file: %w", err)
	}
	if err := os.Chmod(tempPath, 0o644); err != nil {
		_ = os.Remove(tempPath)
		return nil, fmt.Errorf("failed to set temporary file permissions: %w", err)
	}

	session := &writeSession{
		ID:           "write_" + uuid.NewString(),
		OwnerID:      strings.TrimSpace(ownerID),
		TargetPath:   absPath,
		RelativePath: relPath,
		OriginalPath: strings.TrimSpace(originalPath),
		TempPath:     tempPath,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	m.sessions[session.ID] = session
	return cloneWriteSession(session), nil
}

func (m *WriteSessionManager) resolveSessionLocked(sessionID, ownerID string) (*writeSession, error) {
	trimmedID := strings.TrimSpace(sessionID)
	if trimmedID != "" {
		session, ok := m.sessions[trimmedID]
		if !ok {
			return nil, fmt.Errorf("write session not found: %s", trimmedID)
		}
		if err := authorizeWriteSession(session, ownerID); err != nil {
			return nil, err
		}
		return session, nil
	}

	accessible := make([]*writeSession, 0, len(m.sessions))
	for _, session := range m.sessions {
		if authorizeWriteSession(session, ownerID) != nil {
			continue
		}
		accessible = append(accessible, session)
	}
	switch len(accessible) {
	case 0:
		return nil, errors.New("session_id is required; call write_begin first or pass the session_id returned by write_begin")
	case 1:
		return accessible[0], nil
	default:
		return nil, fmt.Errorf("session_id is required; multiple active write sessions exist: %s", formatWriteSessionChoices(accessible))
	}
}

func formatWriteSessionChoices(sessions []*writeSession) string {
	if len(sessions) == 0 {
		return ""
	}

	ordered := make([]*writeSession, 0, len(sessions))
	for _, session := range sessions {
		if session != nil {
			ordered = append(ordered, session)
		}
	}
	sort.Slice(ordered, func(i, j int) bool {
		left := strings.TrimSpace(ordered[i].RelativePath)
		right := strings.TrimSpace(ordered[j].RelativePath)
		if left == right {
			return ordered[i].ID < ordered[j].ID
		}
		return left < right
	})

	const maxChoices = 3
	parts := make([]string, 0, len(ordered))
	for i, session := range ordered {
		if i >= maxChoices {
			parts = append(parts, fmt.Sprintf("... %d more", len(ordered)-maxChoices))
			break
		}
		label := session.ID
		if relPath := strings.TrimSpace(session.RelativePath); relPath != "" {
			label += " (" + relPath + ")"
		}
		parts = append(parts, label)
	}
	return strings.Join(parts, ", ")
}

func (m *WriteSessionManager) Append(sessionID, ownerID, content string) (*writeSession, error) {
	if m == nil {
		return nil, errors.New("write session manager is not configured")
	}

	now := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	m.pruneExpiredLocked(now)

	session, err := m.resolveSessionLocked(sessionID, ownerID)
	if err != nil {
		return nil, err
	}

	nextSize := session.BytesWritten + int64(len(content))
	if nextSize > m.maxFileSize {
		return nil, fmt.Errorf("result too large: %d bytes (max: %d bytes)", nextSize, m.maxFileSize)
	}

	file, err := os.OpenFile(session.TempPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to open temporary file: %w", err)
	}
	if _, err := file.WriteString(content); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("failed to append temporary file: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("failed to close temporary file: %w", err)
	}

	session.BytesWritten = nextSize
	session.UpdatedAt = now
	return cloneWriteSession(session), nil
}

func (m *WriteSessionManager) Preview(sessionID, ownerID string) (*writeSession, error) {
	if m == nil {
		return nil, errors.New("write session manager is not configured")
	}

	now := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	m.pruneExpiredLocked(now)

	session, err := m.resolveSessionLocked(sessionID, ownerID)
	if err != nil {
		return nil, err
	}
	return cloneWriteSession(session), nil
}

func (m *WriteSessionManager) Commit(sessionID, ownerID string, expectedBytes int64, expectedSHA256 string) (*writeSession, string, error) {
	if m == nil {
		return nil, "", errors.New("write session manager is not configured")
	}

	expectedSHA256 = strings.ToLower(strings.TrimSpace(expectedSHA256))
	if expectedSHA256 != "" {
		if _, err := hex.DecodeString(expectedSHA256); err != nil || len(expectedSHA256) != sha256.Size*2 {
			return nil, "", errors.New("expected_sha256 must be a 64-character lowercase hex string")
		}
	}

	now := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	m.pruneExpiredLocked(now)

	session, err := m.resolveSessionLocked(sessionID, ownerID)
	if err != nil {
		return nil, "", err
	}

	if expectedBytes > 0 && session.BytesWritten != expectedBytes {
		return nil, "", fmt.Errorf("write session size mismatch: wrote %d bytes, expected %d bytes", session.BytesWritten, expectedBytes)
	}

	actualSHA256, err := sha256File(session.TempPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to hash temporary file: %w", err)
	}
	if expectedSHA256 != "" && actualSHA256 != expectedSHA256 {
		return nil, "", fmt.Errorf("write session sha256 mismatch: got %s, expected %s", actualSHA256, expectedSHA256)
	}

	if err := replaceFileFromTemp(session.TempPath, session.TargetPath); err != nil {
		return nil, "", err
	}

	delete(m.sessions, session.ID)
	return cloneWriteSession(session), actualSHA256, nil
}

func (m *WriteSessionManager) Abort(sessionID, ownerID string) (*writeSession, error) {
	if m == nil {
		return nil, errors.New("write session manager is not configured")
	}

	now := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	m.pruneExpiredLocked(now)

	session, err := m.resolveSessionLocked(sessionID, ownerID)
	if err != nil {
		return nil, err
	}

	delete(m.sessions, session.ID)
	_ = os.Remove(session.TempPath)
	return cloneWriteSession(session), nil
}

func (m *WriteSessionManager) pruneExpiredLocked(now time.Time) {
	if len(m.sessions) == 0 || m.ttl <= 0 {
		return
	}
	for id, session := range m.sessions {
		if now.Sub(session.UpdatedAt) <= m.ttl {
			continue
		}
		_ = os.Remove(session.TempPath)
		delete(m.sessions, id)
	}
}

func authorizeWriteSession(session *writeSession, ownerID string) error {
	ownerID = strings.TrimSpace(ownerID)
	if session == nil {
		return errors.New("write session is not available")
	}
	if session.OwnerID == "" || ownerID == "" {
		return nil
	}
	if session.OwnerID != ownerID {
		return errors.New("write session belongs to another user")
	}
	return nil
}

func cloneWriteSession(session *writeSession) *writeSession {
	if session == nil {
		return nil
	}
	clone := *session
	return &clone
}

func sha256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func replaceFileFromTemp(tempPath, targetPath string) error {
	if info, err := os.Stat(targetPath); err == nil && info.IsDir() {
		return fmt.Errorf("path is a directory: %s", targetPath)
	}
	if err := os.Rename(tempPath, targetPath); err == nil {
		return nil
	} else {
		data, readErr := os.ReadFile(tempPath)
		if readErr != nil {
			return fmt.Errorf("failed to finalize file: rename failed: %w", err)
		}
		if writeErr := os.WriteFile(targetPath, data, 0o644); writeErr != nil {
			return fmt.Errorf("failed to finalize file: rename failed: %v; fallback write failed: %w", err, writeErr)
		}
		_ = os.Remove(tempPath)
		return nil
	}
}

type FileWriteBeginTool struct {
	AllowedPaths []string
	scope        *fsToolScope
	sessions     *WriteSessionManager
}

func NewFileWriteBeginTool(allowedPaths []string, sessions *WriteSessionManager) *FileWriteBeginTool {
	return &FileWriteBeginTool{
		AllowedPaths: allowedPaths,
		scope:        newFSToolScope(allowedPaths),
		sessions:     sessions,
	}
}

func (t *FileWriteBeginTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "write_begin",
		Description: "Starts a transactional multi-part file write. Use this when the output would exceed 200 lines or 32 KiB in a single call, then send repeated write_chunk calls and a final write_commit.",
		Icon:        "file-write",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The path to the file to create or overwrite",
				},
				"create_dirs": map[string]interface{}{
					"type":        "boolean",
					"description": "If true, create parent directories when missing (default: true)",
				},
			},
			"required": []string{"path"},
		},
	}
}

func (t *FileWriteBeginTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	path, err := fsAsString(args, "path")
	if err != nil || strings.TrimSpace(path) == "" {
		return nil, errors.New("path must be a non-empty string")
	}
	createDirs, err := fsAsBool(args, "create_dirs", true)
	if err != nil {
		return nil, err
	}

	absPath, relPath, _, err := t.scope.resolvePathWithContext(ctx, "write_begin", path, false)
	if err != nil {
		return nil, err
	}
	if err := enforceWritePathGuard(ctx, absPath); err != nil {
		return nil, err
	}

	session, err := t.sessions.Begin(absPath, relPath, path, GetUserID(ctx), createDirs)
	if err != nil {
		return nil, err
	}

	result, _ := json.Marshal(map[string]interface{}{
		"session_id":      session.ID,
		"path":            session.RelativePath,
		"absolute_path":   session.TargetPath,
		"original_path":   session.OriginalPath,
		"success":         true,
		"max_chunk_bytes": maxFileWriteChunkBytes,
		"max_chunk_lines": maxFileWriteChunkLines,
		"max_total_bytes": t.sessions.maxFileSize,
		"transactional":   true,
	})
	return string(result), nil
}

type FileWriteChunkTool struct {
	sessions *WriteSessionManager
}

func NewFileWriteChunkTool(sessions *WriteSessionManager) *FileWriteChunkTool {
	return &FileWriteChunkTool{sessions: sessions}
}

func optionalWriteSessionIDArg(args map[string]interface{}) (string, error) {
	value, ok := firstCompatValueDeep(args, fsCompatKeys("session_id")...)
	if !ok || value == nil {
		return "", nil
	}
	sessionID, ok := value.(string)
	if !ok {
		return "", errors.New("session_id must be a string")
	}
	return sessionID, nil
}

func (t *FileWriteChunkTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "write_chunk",
		Description: "Appends one chunk to an active transactional file write session started by write_begin. Keep each chunk at or below 200 lines and 32 KiB.",
		Icon:        "file-write",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "The write session id returned by write_begin",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "One chunk of text content to append. Keep each chunk at or below 200 lines and 32 KiB.",
				},
			},
			"required": []string{"session_id", "content"},
		},
	}
}

func (t *FileWriteChunkTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	sessionID, err := optionalWriteSessionIDArg(args)
	if err != nil {
		return nil, err
	}
	content, err := fsAsTextContent(args, "content")
	if err != nil {
		return nil, errors.New("content must be text-compatible (string, number, boolean, object, or array)")
	}
	if err := validateWriteContentChunk(content, "write_chunk", "split into smaller chunks"); err != nil {
		return nil, err
	}

	session, err := t.sessions.Append(sessionID, GetUserID(ctx), content)
	if err != nil {
		return nil, err
	}

	result, _ := json.Marshal(map[string]interface{}{
		"session_id":    session.ID,
		"path":          session.RelativePath,
		"success":       true,
		"chunk_bytes":   len(content),
		"written_bytes": session.BytesWritten,
	})
	return string(result), nil
}

type FileWriteCommitTool struct {
	sessions *WriteSessionManager
}

func NewFileWriteCommitTool(sessions *WriteSessionManager) *FileWriteCommitTool {
	return &FileWriteCommitTool{sessions: sessions}
}

func (t *FileWriteCommitTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "write_commit",
		Description: "Finalizes a transactional file write. Optionally verifies the exact byte length and sha256 before publishing the file.",
		Icon:        "file-write",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "The write session id returned by write_begin",
				},
				"expected_bytes": map[string]interface{}{
					"type":        "integer",
					"description": "Optional exact byte length to verify before commit",
				},
				"expected_sha256": map[string]interface{}{
					"type":        "string",
					"description": "Optional lowercase SHA-256 hex digest to verify before commit",
				},
			},
			"required": []string{"session_id"},
		},
	}
}

func (t *FileWriteCommitTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	sessionID, err := optionalWriteSessionIDArg(args)
	if err != nil {
		return nil, err
	}
	expectedBytes, err := fsAsInt(args, "expected_bytes", 0)
	if err != nil {
		return nil, err
	}
	expectedSHA256, err := fsOptionalString(args, "", "expected_sha256", "expected_sha256", "expectedSha256")
	if err != nil {
		return nil, err
	}

	session, err := t.sessions.Preview(sessionID, GetUserID(ctx))
	if err != nil {
		return nil, err
	}
	if err := enforceWritePathGuard(ctx, session.TargetPath); err != nil {
		return nil, err
	}

	session, actualSHA256, err := t.sessions.Commit(sessionID, GetUserID(ctx), int64(expectedBytes), expectedSHA256)
	if err != nil {
		return nil, err
	}

	result, _ := json.Marshal(map[string]interface{}{
		"session_id":    session.ID,
		"path":          session.RelativePath,
		"absolute_path": session.TargetPath,
		"original_path": session.OriginalPath,
		"success":       true,
		"size":          session.BytesWritten,
		"sha256":        actualSHA256,
		"transactional": true,
	})
	return string(result), nil
}

type FileWriteAbortTool struct {
	sessions *WriteSessionManager
}

func NewFileWriteAbortTool(sessions *WriteSessionManager) *FileWriteAbortTool {
	return &FileWriteAbortTool{sessions: sessions}
}

func (t *FileWriteAbortTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "write_abort",
		Description: "Aborts an active transactional file write and removes its temporary data.",
		Icon:        "file-write",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "The write session id returned by write_begin",
				},
			},
			"required": []string{"session_id"},
		},
	}
}

func (t *FileWriteAbortTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	sessionID, err := optionalWriteSessionIDArg(args)
	if err != nil {
		return nil, err
	}

	session, err := t.sessions.Abort(sessionID, GetUserID(ctx))
	if err != nil {
		return nil, err
	}

	result, _ := json.Marshal(map[string]interface{}{
		"session_id":    session.ID,
		"path":          session.RelativePath,
		"absolute_path": session.TargetPath,
		"original_path": session.OriginalPath,
		"success":       true,
		"aborted":       true,
	})
	return string(result), nil
}
