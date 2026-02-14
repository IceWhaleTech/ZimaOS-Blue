package heartbeat

import "time"

const (
	DefaultInterval    = 30 * time.Minute
	DefaultAckMaxChars = 300
	DefaultPrompt      = "Read HEARTBEAT.md if it exists (workspace context). Follow it strictly. Do not infer or repeat old tasks from prior chats. If nothing needs attention, reply HEARTBEAT_OK."
	HeartbeatFilename  = "HEARTBEAT.md"
)

// Config holds heartbeat service configuration.
type Config struct {
	Enabled         bool            `mapstructure:"enabled"`
	Interval        time.Duration   `mapstructure:"interval"`
	Prompt          string          `mapstructure:"prompt"`
	AckMaxChars     int             `mapstructure:"ack_max_chars"`
	WorkspaceDir    string          `mapstructure:"workspace_dir"`
	LLMProvider     string          `mapstructure:"llm_provider"`
	LLMModel        string          `mapstructure:"llm_model"`
	ActiveHours     *ActiveHours    `mapstructure:"active_hours"`
	Visibility      VisibilityConfig `mapstructure:"visibility"`
	DeliveryChannel string          `mapstructure:"delivery_channel"`
	DeliveryChatID  string          `mapstructure:"delivery_chat_id"`
}

// ActiveHours defines the time window when heartbeat is allowed to run.
type ActiveHours struct {
	Start    string `mapstructure:"start"`    // "HH:MM" format
	End      string `mapstructure:"end"`      // "HH:MM" format, "24:00" allowed
	Timezone string `mapstructure:"timezone"` // IANA timezone or "Local"
}

// VisibilityConfig controls what heartbeat results are delivered.
type VisibilityConfig struct {
	ShowOk       bool `mapstructure:"show_ok"`
	ShowAlerts   bool `mapstructure:"show_alerts"`
	UseIndicator bool `mapstructure:"use_indicator"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Enabled:     false,
		Interval:    DefaultInterval,
		Prompt:      DefaultPrompt,
		AckMaxChars: DefaultAckMaxChars,
		Visibility: VisibilityConfig{
			ShowOk:       false,
			ShowAlerts:   true,
			UseIndicator: true,
		},
	}
}
