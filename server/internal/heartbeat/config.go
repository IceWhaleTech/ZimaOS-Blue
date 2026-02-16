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
	Enabled         bool            `yaml:"enabled"`
	Interval        time.Duration   `yaml:"interval"`
	Prompt          string          `yaml:"prompt"`
	AckMaxChars     int             `yaml:"ack_max_chars"`
	WorkspaceDir    string          `yaml:"workspace_dir"`
	LLMProvider     string          `yaml:"llm_provider"`
	LLMModel        string          `yaml:"llm_model"`
	ActiveHours     *ActiveHours    `yaml:"active_hours"`
	Visibility      VisibilityConfig `yaml:"visibility"`
	DeliveryChannel string          `yaml:"delivery_channel"`
	DeliveryChatID  string          `yaml:"delivery_chat_id"`
}

// ActiveHours defines the time window when heartbeat is allowed to run.
type ActiveHours struct {
	Start    string `yaml:"start"`    // "HH:MM" format
	End      string `yaml:"end"`      // "HH:MM" format, "24:00" allowed
	Timezone string `yaml:"timezone"` // IANA timezone or "Local"
}

// VisibilityConfig controls what heartbeat results are delivered.
type VisibilityConfig struct {
	ShowOk       bool `yaml:"show_ok"`
	ShowAlerts   bool `yaml:"show_alerts"`
	UseIndicator bool `yaml:"use_indicator"`
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
