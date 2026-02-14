package whatsapp

import (
	"context"
	"os"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/validator"
)

// Validator validates WhatsApp configuration.
type Validator struct{}

// NewValidator creates a new WhatsApp validator.
func NewValidator() *Validator {
	return &Validator{}
}

// Validate validates the WhatsApp configuration.
func (v *Validator) Validate(ctx context.Context, config map[string]string) validator.Result {
	phoneNumber := config["phone_number"]
	if phoneNumber == "" {
		return validator.Result{
			Success:    false,
			Error:      "phone_number is required",
			MessageKey: "channels.phoneNumberRequired",
		}
	}

	sessionPath := config["session_path"]
	if sessionPath == "" {
		sessionPath = "./data/whatsapp"
	}

	// Check if session directory exists or can be created
	if err := os.MkdirAll(sessionPath, 0755); err != nil {
		return validator.Result{
			Success:    false,
			Error:      "cannot create session directory: " + err.Error(),
			MessageKey: "channels.sessionDirError",
		}
	}

	return validator.Result{
		Success:    true,
		MessageKey: "channels.connectionSuccess",
	}
}
