package imessage

import (
	"context"
	"os"
	"runtime"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel/validator"
)

// Validator validates iMessage configuration.
type Validator struct{}

// NewValidator creates a new iMessage validator.
func NewValidator() *Validator {
	return &Validator{}
}

// Validate validates the iMessage configuration.
func (v *Validator) Validate(ctx context.Context, config map[string]string) validator.Result {
	// iMessage only works on macOS
	if runtime.GOOS != "darwin" {
		return validator.Result{
			Success:    false,
			Error:      "iMessage is only available on macOS",
			MessageKey: "channels.imessageNotAvailable",
		}
	}

	// Check if database path is provided
	dbPath := config["database_path"]
	if dbPath == "" {
		dbPath = os.ExpandEnv("$HOME/Library/Messages/chat.db")
	}

	// Check if database exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return validator.Result{
			Success:    false,
			Error:      "Messages database not found at: " + dbPath,
			MessageKey: "channels.imessageDatabaseNotFound",
		}
	}

	return validator.Result{
		Success:    true,
		MessageKey: "channels.connectionSuccess",
	}
}
