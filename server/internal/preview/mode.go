// Package preview provides preview mode functionality for ZimaOS-Blue.
// Preview mode allows users to experience the product without creating an account.
package preview

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/user"
)

// SystemMode represents the current system mode.
type SystemMode struct {
	Mode     string          `json:"mode"`     // "preview" or "normal"
	Features map[string]bool `json:"features"` // Available features
}

// ModeService provides preview mode detection and management.
type ModeService struct {
	userService *user.Service
	dataDir     string
}

// NewModeService creates a new ModeService.
func NewModeService(userService *user.Service) *ModeService {
	return &ModeService{
		userService: userService,
		dataDir:     "./data",
	}
}

// SetDataDir sets the data directory for logging.
func (s *ModeService) SetDataDir(dataDir string) {
	s.dataDir = dataDir
}

// logToFile writes a log message to preview_mode.log in dataDir
func (s *ModeService) logToFile(message string) {
	if s.dataDir == "" {
		return
	}

	logPath := filepath.Join(s.dataDir, "preview_mode.log")
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	logMsg := fmt.Sprintf("[%s] %s\n", timestamp, message)

	// Ensure data directory exists
	os.MkdirAll(s.dataDir, 0750)

	// Append to log file
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(logMsg)
}

// IsPreviewMode checks if the system is in preview mode.
// Preview mode is active when no admin user exists.
func (s *ModeService) IsPreviewMode(ctx context.Context) (bool, error) {
	s.logToFile("IsPreviewMode: Checking if admin exists...")

	adminExists, err := s.userService.AdminExists(ctx)
	if err != nil {
		s.logToFile(fmt.Sprintf("IsPreviewMode: ERROR checking admin exists: %v", err))
		return false, err
	}

	isPreview := !adminExists
	s.logToFile(fmt.Sprintf("IsPreviewMode: adminExists=%v, isPreview=%v", adminExists, isPreview))

	return isPreview, nil
}

// GetSystemMode returns the current system mode and available features.
func (s *ModeService) GetSystemMode(ctx context.Context) (*SystemMode, error) {
	s.logToFile("GetSystemMode: Called")

	isPreview, err := s.IsPreviewMode(ctx)
	if err != nil {
		s.logToFile(fmt.Sprintf("GetSystemMode: ERROR from IsPreviewMode: %v", err))
		return nil, err
	}

	mode := &SystemMode{
		Features: make(map[string]bool),
	}

	if isPreview {
		mode.Mode = "preview"
		s.logToFile("GetSystemMode: Returning PREVIEW mode")
		// In preview mode, all features are available (user is treated as admin)
		mode.Features = map[string]bool{
			"chat":            true,
			"attachment":      true,
			"image_upload":    true,
			"voice_play":      true,
			"voice_input":     true,
			"provider_config": true,
			"settings":        true,
			"admin":           true,
			"user_management": true,
		}
	} else {
		mode.Mode = "normal"
		s.logToFile("GetSystemMode: Returning NORMAL mode")
		// In normal mode, all features are available
		mode.Features = map[string]bool{
			"chat":            true,
			"attachment":      true,
			"image_upload":    true,
			"voice_play":      true,
			"voice_input":     true,
			"provider_config": true,
			"settings":        true,
			"admin":           true,
			"user_management": true,
		}
	}

	return mode, nil
}
