// Package preview provides preview mode functionality for ZimaOS-Blue.
// Preview mode allows users to experience the product without creating an account.
package preview

import (
	"context"

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
}

// NewModeService creates a new ModeService.
func NewModeService(userService *user.Service) *ModeService {
	return &ModeService{
		userService: userService,
	}
}

// IsPreviewMode checks if the system is in preview mode.
// Preview mode is active when no admin user exists.
func (s *ModeService) IsPreviewMode(ctx context.Context) (bool, error) {
	adminExists, err := s.userService.AdminExists(ctx)
	if err != nil {
		return false, err
	}

	isPreview := !adminExists
	return isPreview, nil
}

// GetSystemMode returns the current system mode and available features.
func (s *ModeService) GetSystemMode(ctx context.Context) (*SystemMode, error) {
	isPreview, err := s.IsPreviewMode(ctx)
	if err != nil {
		return nil, err
	}

	mode := &SystemMode{
		Features: make(map[string]bool),
	}

	if isPreview {
		mode.Mode = "preview"
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
