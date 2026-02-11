package signal

import (
	"context"
	"os"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel/validator"
)

// Validator validates Signal configuration.
type Validator struct{}

// NewValidator creates a new Signal validator.
func NewValidator() *Validator {
	return &Validator{}
}

// Validate validates the Signal configuration.
func (v *Validator) Validate(ctx context.Context, config map[string]string) validator.Result {
	phoneNumber := config["phone_number"]
	if phoneNumber == "" {
		return validator.Result{
			Success:    false,
			Error:      "phone_number is required",
			MessageKey: "channels.phoneNumberRequired",
		}
	}

	configPath := config["config_path"]
	if configPath == "" {
		configPath = "./data/signal"
	}

	// Check if config directory exists or can be created
	if err := os.MkdirAll(configPath, 0755); err != nil {
		return validator.Result{
			Success:    false,
			Error:      "cannot create config directory: " + err.Error(),
			MessageKey: "channels.configDirError",
		}
	}

	return validator.Result{
		Success:    true,
		MessageKey: "channels.connectionSuccess",
	}
}
