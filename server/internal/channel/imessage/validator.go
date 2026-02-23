package imessage

import (
	"context"
	"os"
	"runtime"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/validator"
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
		homeDir, _ := os.UserHomeDir()
		if homeDir != "" {
			dbPath = homeDir + "/Library/Messages/chat.db"
		} else {
			dbPath = os.ExpandEnv("$HOME/Library/Messages/chat.db")
		}
	}

	// Check if database exists and is accessible
	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			return validator.Result{
				Success:    false,
				Error:      "Messages database not found at: " + dbPath,
				MessageKey: "channels.imessageDatabaseNotFound",
			}
		}
		if os.IsPermission(err) {
			return validator.Result{
				Success:    false,
				Error:      "Messages database access denied — grant Full Disk Access in System Settings > Privacy & Security",
				MessageKey: "channels.imessageAccessDenied",
			}
		}
		return validator.Result{
			Success:    false,
			Error:      "Cannot access Messages database: " + err.Error(),
			MessageKey: "channels.imessageDatabaseNotFound",
		}
	}

	return validator.Result{
		Success:    true,
		MessageKey: "channels.connectionSuccess",
	}
}
