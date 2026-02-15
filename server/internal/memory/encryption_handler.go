package memory

import (
	"context"
	"net/http"
	"sync/atomic"

	"github.com/labstack/echo/v4"
)

// EncryptionHandler provides HTTP endpoints for memory encryption management.
type EncryptionHandler struct {
	repo      *MemoryRepository
	encryptor *ContentEncryptor
	migrating atomic.Bool
	progress  atomic.Int64
	total     atomic.Int64
}

// NewEncryptionHandler creates a new encryption handler.
func NewEncryptionHandler(repo *MemoryRepository, enc *ContentEncryptor) *EncryptionHandler {
	return &EncryptionHandler{repo: repo, encryptor: enc}
}

// RegisterRoutes registers encryption API routes.
func (h *EncryptionHandler) RegisterRoutes(g *echo.Group) {
	enc := g.Group("/encryption")
	enc.GET("/status", h.GetStatus)
	enc.POST("/enable", h.Enable)
	enc.POST("/disable", h.Disable)
	enc.POST("/rotate-key", h.RotateKey)
}

// GetStatus returns encryption status.
// GET /encryption/status
func (h *EncryptionHandler) GetStatus(c echo.Context) error {
	encrypted, plaintext, err := h.repo.CountEncrypted(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"enabled":          h.encryptor.IsEnabled(),
		"encrypted_count":  encrypted,
		"plaintext_count":  plaintext,
		"total_count":      encrypted + plaintext,
		"migrating":        h.migrating.Load(),
		"migration_progress": h.progress.Load(),
		"migration_total":  h.total.Load(),
	})
}

// Enable enables encryption and starts background migration.
// POST /encryption/enable
func (h *EncryptionHandler) Enable(c echo.Context) error {
	if h.migrating.Load() {
		return c.JSON(http.StatusConflict, map[string]string{"error": "migration in progress"})
	}
	var req struct {
		Passphrase string `json:"passphrase"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if req.Passphrase == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "passphrase is required"})
	}

	if err := h.encryptor.InitKey(req.Passphrase); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	h.encryptor.SetEnabled(true)

	// Start background migration
	go h.migrateEncrypt(context.Background())

	return c.JSON(http.StatusOK, map[string]any{
		"success":           true,
		"message":           "encryption enabled, migration started",
		"migration_started": true,
	})
}

// Disable disables encryption and starts background decryption.
// POST /encryption/disable
func (h *EncryptionHandler) Disable(c echo.Context) error {
	if h.migrating.Load() {
		return c.JSON(http.StatusConflict, map[string]string{"error": "migration in progress"})
	}
	if !h.encryptor.IsEnabled() {
		return c.JSON(http.StatusOK, map[string]any{
			"success": true,
			"message": "encryption already disabled",
		})
	}

	// Start background decryption before disabling
	go h.migrateDecrypt(context.Background())

	return c.JSON(http.StatusOK, map[string]any{
		"success":           true,
		"message":           "decryption migration started",
		"migration_started": true,
	})
}

// RotateKey re-encrypts all entries with a new key.
// POST /encryption/rotate-key
func (h *EncryptionHandler) RotateKey(c echo.Context) error {
	if h.migrating.Load() {
		return c.JSON(http.StatusConflict, map[string]string{"error": "migration in progress"})
	}
	if !h.encryptor.IsEnabled() {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "encryption not enabled"})
	}
	var req struct {
		OldPassphrase string `json:"old_passphrase"`
		NewPassphrase string `json:"new_passphrase"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if req.NewPassphrase == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "new_passphrase is required"})
	}

	oldKey, err := h.encryptor.RotateKey(req.NewPassphrase)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	go h.migrateRotate(context.Background(), oldKey)

	return c.JSON(http.StatusOK, map[string]any{
		"success":           true,
		"message":           "key rotation started",
		"migration_started": true,
	})
}

const migrationBatchSize = 100

func (h *EncryptionHandler) migrateEncrypt(ctx context.Context) {
	if !h.migrating.CompareAndSwap(false, true) {
		return
	}
	defer h.migrating.Store(false)

	total, err := h.repo.CountAll(ctx)
	if err != nil {
		return
	}
	h.total.Store(int64(total))
	h.progress.Store(0)

	offset := 0
	for {
		rows, err := h.repo.ListRaw(ctx, offset, migrationBatchSize)
		if err != nil || len(rows) == 0 {
			break
		}
		for _, row := range rows {
			if h.encryptor.IsEncrypted(row.Content) {
				h.progress.Add(1)
				continue
			}
			encrypted, err := h.encryptor.Encrypt(row.Content)
			if err != nil {
				continue
			}
			_ = h.repo.UpdateContentRaw(ctx, row.ID, encrypted)
			h.progress.Add(1)
		}
		offset += len(rows)
		if len(rows) < migrationBatchSize {
			break
		}
	}
}

func (h *EncryptionHandler) migrateDecrypt(ctx context.Context) {
	if !h.migrating.CompareAndSwap(false, true) {
		return
	}
	defer func() {
		h.encryptor.SetEnabled(false)
		h.migrating.Store(false)
	}()

	total, err := h.repo.CountAll(ctx)
	if err != nil {
		return
	}
	h.total.Store(int64(total))
	h.progress.Store(0)

	offset := 0
	for {
		rows, err := h.repo.ListRaw(ctx, offset, migrationBatchSize)
		if err != nil || len(rows) == 0 {
			break
		}
		for _, row := range rows {
			if !h.encryptor.IsEncrypted(row.Content) {
				h.progress.Add(1)
				continue
			}
			plaintext, err := h.encryptor.Decrypt(row.Content)
			if err != nil {
				continue
			}
			_ = h.repo.UpdateContentRaw(ctx, row.ID, plaintext)
			h.progress.Add(1)
		}
		offset += len(rows)
		if len(rows) < migrationBatchSize {
			break
		}
	}
}

func (h *EncryptionHandler) migrateRotate(ctx context.Context, oldKey []byte) {
	if !h.migrating.CompareAndSwap(false, true) {
		return
	}
	defer h.migrating.Store(false)

	total, err := h.repo.CountAll(ctx)
	if err != nil {
		return
	}
	h.total.Store(int64(total))
	h.progress.Store(0)

	// Build a temporary decryptor with the old key
	oldEnc := &ContentEncryptor{key: oldKey, enabled: true}

	offset := 0
	for {
		rows, err := h.repo.ListRaw(ctx, offset, migrationBatchSize)
		if err != nil || len(rows) == 0 {
			break
		}
		for _, row := range rows {
			if !oldEnc.IsEncrypted(row.Content) {
				// Plaintext — encrypt with new key
				encrypted, err := h.encryptor.Encrypt(row.Content)
				if err == nil {
					_ = h.repo.UpdateContentRaw(ctx, row.ID, encrypted)
				}
			} else {
				// Decrypt with old key, re-encrypt with new key
				plaintext, err := oldEnc.Decrypt(row.Content)
				if err != nil {
					h.progress.Add(1)
					continue
				}
				encrypted, err := h.encryptor.Encrypt(plaintext)
				if err == nil {
					_ = h.repo.UpdateContentRaw(ctx, row.ID, encrypted)
				}
			}
			h.progress.Add(1)
		}
		offset += len(rows)
		if len(rows) < migrationBatchSize {
			break
		}
	}
}

// RawEntry is a minimal struct for migration operations.
type RawEntry struct {
	ID      string
	Content string
}
