package server

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/pbkdf2"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
)

// UserDataExport represents the exported user data structure
type UserDataExport struct {
	Version     string                 `json:"version"`
	ExportedAt  time.Time              `json:"exported_at"`
	DataType    string                 `json:"data_type"` // "json" or "encrypted"
	Settings    *UserSettings          `json:"settings,omitempty"`
	ChatHistory *ChatHistoryExport     `json:"chat_history,omitempty"`
	Checksum    string                 `json:"checksum,omitempty"`
}

// UserSettings represents user preferences
type UserSettings struct {
	SelectedProviderModel string  `json:"selected_provider_model,omitempty"`
	Temperature           float64 `json:"temperature"`
	MaxTokens             int     `json:"max_tokens"`
	ThemeStyle            string  `json:"theme_style,omitempty"`
	Theme                 string  `json:"theme,omitempty"`
	Locale                string  `json:"locale,omitempty"`
	Timezone              string  `json:"timezone,omitempty"`
}

// ChatHistoryExport represents exported chat history
type ChatHistoryExport struct {
	Conversations []ConversationExport `json:"conversations"`
	TotalMessages int                  `json:"total_messages"`
}

// ConversationExport represents a single conversation with messages
type ConversationExport struct {
	ID        string          `json:"id"`
	Title     string          `json:"title"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	Messages  []MessageExport `json:"messages"`
}

// MessageExport represents a single message
type MessageExport struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Provider  string    `json:"provider,omitempty"`
	Model     string    `json:"model,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ExportRequest represents the export request body
type ExportRequest struct {
	Password string `json:"password"`
	Format   string `json:"format"` // "json" or "encrypted"
	Settings *UserSettings `json:"settings,omitempty"` // Frontend settings to include
}

// ImportRequest represents the import request body
type ImportRequest struct {
	Password string `json:"password"`
	Data     string `json:"data"` // Base64 encoded data for encrypted format
}

// EncryptedExport represents encrypted export data
type EncryptedExport struct {
	Version   string `json:"version"`
	Format    string `json:"format"`
	Salt      string `json:"salt"`
	IV        string `json:"iv"`
	Data      string `json:"data"`
	Checksum  string `json:"checksum"`
}

// UserDataHandler handles user data export/import operations
type UserDataHandler struct {
	memoryStore *memory.Store
}

// NewUserDataHandler creates a new user data handler
func NewUserDataHandler(memoryStore *memory.Store) *UserDataHandler {
	return &UserDataHandler{
		memoryStore: memoryStore,
	}
}

// RegisterRoutes registers the user data routes
func (h *UserDataHandler) RegisterRoutes(e *echo.Echo) {
	g := e.Group("/api/userdata")
	g.POST("/export", h.Export)
	g.POST("/import", h.Import)
	g.POST("/import/preview", h.ImportPreview)
}

// Export handles user data export
func (h *UserDataHandler) Export(c echo.Context) error {
	var req ExportRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	if req.Password == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Password is required"})
	}

	if req.Format == "" {
		req.Format = "json"
	}

	// Build export data
	export := &UserDataExport{
		Version:    "1.0",
		ExportedAt: time.Now(),
		DataType:   req.Format,
		Settings:   req.Settings,
	}

	// Export chat history
	chatHistory, err := h.exportChatHistory(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Failed to export chat history: %v", err)})
	}
	export.ChatHistory = chatHistory

	// Calculate checksum
	dataBytes, err := json.Marshal(export)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to serialize data"})
	}
	export.Checksum = calculateChecksum(dataBytes)

	if req.Format == "encrypted" {
		// Encrypt the data
		encryptedExport, err := h.encryptData(export, req.Password)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Failed to encrypt data: %v", err)})
		}
		return c.JSON(http.StatusOK, encryptedExport)
	}

	// Return JSON format (password is used for verification on import)
	return c.JSON(http.StatusOK, export)
}

// Import handles user data import
func (h *UserDataHandler) Import(c echo.Context) error {
	var req ImportRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	if req.Password == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Password is required"})
	}

	if req.Data == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Data is required"})
	}

	// Decode base64 data
	dataBytes, err := base64.StdEncoding.DecodeString(req.Data)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid data format"})
	}

	// Try to parse as encrypted format first
	var encryptedExport EncryptedExport
	if err := json.Unmarshal(dataBytes, &encryptedExport); err == nil && encryptedExport.Format == "encrypted" {
		// Decrypt the data
		export, err := h.decryptData(&encryptedExport, req.Password)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Failed to decrypt data: %v", err)})
		}
		return h.importData(c, export)
	}

	// Try to parse as JSON format
	var export UserDataExport
	if err := json.Unmarshal(dataBytes, &export); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid export data format"})
	}

	return h.importData(c, &export)
}

// ImportPreview returns a preview of the import data without actually importing
func (h *UserDataHandler) ImportPreview(c echo.Context) error {
	var req ImportRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	if req.Password == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Password is required"})
	}

	if req.Data == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Data is required"})
	}

	// Decode base64 data
	dataBytes, err := base64.StdEncoding.DecodeString(req.Data)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid data format"})
	}

	// Try to parse as encrypted format first
	var encryptedExport EncryptedExport
	if err := json.Unmarshal(dataBytes, &encryptedExport); err == nil && encryptedExport.Format == "encrypted" {
		// Decrypt the data
		export, err := h.decryptData(&encryptedExport, req.Password)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Failed to decrypt data: %v", err)})
		}
		return c.JSON(http.StatusOK, h.buildPreview(export))
	}

	// Try to parse as JSON format
	var export UserDataExport
	if err := json.Unmarshal(dataBytes, &export); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid export data format"})
	}

	return c.JSON(http.StatusOK, h.buildPreview(&export))
}

func (h *UserDataHandler) exportChatHistory(ctx context.Context) (*ChatHistoryExport, error) {
	// Get all conversations
	conversations, err := h.memoryStore.ListConversations(ctx, 10000, 0)
	if err != nil {
		return nil, err
	}

	chatHistory := &ChatHistoryExport{
		Conversations: make([]ConversationExport, 0, len(conversations)),
	}

	for _, conv := range conversations {
		// Get messages for each conversation
		messages, err := h.memoryStore.GetMessages(ctx, conv.ID, 10000, 0)
		if err != nil {
			continue // Skip conversations with errors
		}

		convExport := ConversationExport{
			ID:        conv.ID,
			Title:     conv.Title,
			CreatedAt: conv.CreatedAt,
			UpdatedAt: conv.UpdatedAt,
			Messages:  make([]MessageExport, 0, len(messages)),
		}

		for _, msg := range messages {
			convExport.Messages = append(convExport.Messages, MessageExport{
				ID:        msg.ID,
				Role:      msg.Role,
				Content:   msg.Content,
				Provider:  msg.Provider,
				Model:     msg.Model,
				CreatedAt: msg.CreatedAt,
			})
		}

		chatHistory.Conversations = append(chatHistory.Conversations, convExport)
		chatHistory.TotalMessages += len(messages)
	}

	return chatHistory, nil
}

func (h *UserDataHandler) importData(c echo.Context, export *UserDataExport) error {
	ctx := c.Request().Context()
	imported := struct {
		Conversations int `json:"conversations"`
		Messages      int `json:"messages"`
		Settings      bool `json:"settings"`
	}{}

	// Import chat history
	if export.ChatHistory != nil {
		for _, conv := range export.ChatHistory.Conversations {
			// Create conversation
			newConv, err := h.memoryStore.CreateConversation(ctx, conv.Title)
			if err != nil {
				continue
			}

			// Add messages
			for _, msg := range conv.Messages {
				_, err := h.memoryStore.AddMessage(ctx, newConv.ID, memory.Message{
					Role:     msg.Role,
					Content:  msg.Content,
					Provider: msg.Provider,
					Model:    msg.Model,
				})
				if err != nil {
					continue
				}
				imported.Messages++
			}
			imported.Conversations++
		}
	}

	// Settings are returned to frontend to apply
	if export.Settings != nil {
		imported.Settings = true
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":  true,
		"imported": imported,
		"settings": export.Settings,
	})
}

func (h *UserDataHandler) buildPreview(export *UserDataExport) map[string]interface{} {
	preview := map[string]interface{}{
		"version":     export.Version,
		"exported_at": export.ExportedAt,
		"data_type":   export.DataType,
	}

	if export.Settings != nil {
		preview["has_settings"] = true
		preview["settings_preview"] = map[string]interface{}{
			"theme":       export.Settings.Theme,
			"theme_style": export.Settings.ThemeStyle,
			"locale":      export.Settings.Locale,
		}
	}

	if export.ChatHistory != nil {
		preview["has_chat_history"] = true
		preview["chat_preview"] = map[string]interface{}{
			"conversations": len(export.ChatHistory.Conversations),
			"messages":      export.ChatHistory.TotalMessages,
		}
	}

	return preview
}

func (h *UserDataHandler) encryptData(export *UserDataExport, password string) (*EncryptedExport, error) {
	// Serialize the export data
	plaintext, err := json.Marshal(export)
	if err != nil {
		return nil, err
	}

	// Generate salt
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}

	// Derive key using PBKDF2
	key := pbkdf2.Key([]byte(password), salt, 100000, 32, sha256.New)

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Encrypt
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	return &EncryptedExport{
		Version:  "1.0",
		Format:   "encrypted",
		Salt:     base64.StdEncoding.EncodeToString(salt),
		IV:       base64.StdEncoding.EncodeToString(nonce),
		Data:     base64.StdEncoding.EncodeToString(ciphertext),
		Checksum: calculateChecksum(plaintext),
	}, nil
}

func (h *UserDataHandler) decryptData(encrypted *EncryptedExport, password string) (*UserDataExport, error) {
	// Decode salt
	salt, err := base64.StdEncoding.DecodeString(encrypted.Salt)
	if err != nil {
		return nil, fmt.Errorf("invalid salt")
	}

	// Decode IV/nonce
	nonce, err := base64.StdEncoding.DecodeString(encrypted.IV)
	if err != nil {
		return nil, fmt.Errorf("invalid IV")
	}

	// Decode ciphertext
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted.Data)
	if err != nil {
		return nil, fmt.Errorf("invalid data")
	}

	// Derive key using PBKDF2
	key := pbkdf2.Key([]byte(password), salt, 100000, 32, sha256.New)

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: incorrect password or corrupted data")
	}

	// Verify checksum
	if encrypted.Checksum != "" && calculateChecksum(plaintext) != encrypted.Checksum {
		return nil, fmt.Errorf("checksum mismatch: data may be corrupted")
	}

	// Parse the decrypted data
	var export UserDataExport
	if err := json.Unmarshal(plaintext, &export); err != nil {
		return nil, fmt.Errorf("failed to parse decrypted data")
	}

	return &export, nil
}

func calculateChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return base64.StdEncoding.EncodeToString(hash[:])
}
