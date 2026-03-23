package channelconfig

import "time"

// HeartbeatConfig controls periodic "still alive" messages sent to the channel
// while the LLM handler is processing. This prevents the bot from appearing dead
// during long responses or multi-round tool execution.
type HeartbeatConfig struct {
	// Enabled turns heartbeat messages on/off. Default: true.
	Enabled bool `yaml:"enabled"`
	// InitialDelay is the wait before the first heartbeat. Must be < 5s. Default: 3s.
	InitialDelay time.Duration `yaml:"initial_delay"`
	// Interval is the gap between subsequent heartbeats. Default: 8s.
	Interval time.Duration `yaml:"interval"`
	// Emojis is the rotating list of emojis to send. Default: ["💬","⌨️"].
	Emojis []string `yaml:"emojis"`
}

// DefaultHeartbeatConfig returns sensible defaults.
func DefaultHeartbeatConfig() HeartbeatConfig {
	return HeartbeatConfig{
		Enabled:      true,
		InitialDelay: 3 * time.Second,
		Interval:     8 * time.Second,
		Emojis:       []string{"💬", "⌨️"},
	}
}

// GroupPolicy controls whether inbound group messages are accepted.
type GroupPolicy string

const (
	// GroupPolicyOpen accepts group messages from any chat.
	GroupPolicyOpen GroupPolicy = "open"
	// GroupPolicyAllowlist only accepts group messages from explicitly allowed chats.
	GroupPolicyAllowlist GroupPolicy = "allowlist"
	// GroupPolicyDisabled rejects all group messages.
	GroupPolicyDisabled GroupPolicy = "disabled"
)

// GroupMentionPolicy controls whether an allowed group message must mention the bot.
type GroupMentionPolicy string

const (
	// GroupMentionPolicyMentioned only accepts group messages that explicitly mention the bot.
	GroupMentionPolicyMentioned GroupMentionPolicy = "mentioned"
	// GroupMentionPolicyAlways accepts all allowed group messages without requiring a mention.
	GroupMentionPolicyAlways GroupMentionPolicy = "always"
)

// GroupAccessConfig controls inbound group message access across all channels.
type GroupAccessConfig struct {
	// Policy controls whether group messages are accepted.
	Policy GroupPolicy `yaml:"policy" json:"policy"`
	// MentionPolicy controls whether allowed group messages must mention the bot.
	MentionPolicy GroupMentionPolicy `yaml:"mention_policy" json:"mention_policy"`
	// AllowedChatIDs stores per-channel allowed group chat IDs when Policy is allowlist.
	AllowedChatIDs map[string][]string `yaml:"allowed_chat_ids" json:"allowed_chat_ids"`
}

// DefaultGroupAccessConfig returns the default inbound group behavior.
func DefaultGroupAccessConfig() GroupAccessConfig {
	return GroupAccessConfig{
		Policy:        GroupPolicyOpen,
		MentionPolicy: GroupMentionPolicyMentioned,
	}
}

// Config contains common configuration for all channels.
type Config struct {
	// Enabled indicates if channels are globally enabled.
	Enabled bool `yaml:"enabled"`
	// DefaultTimeoutSeconds is the default timeout for operations.
	DefaultTimeoutSeconds int `yaml:"default_timeout_seconds"`
	// MaxMessageLength is the maximum message length.
	MaxMessageLength int `yaml:"max_message_length"`
	// Heartbeat controls periodic "still alive" messages during long responses.
	Heartbeat HeartbeatConfig `yaml:"heartbeat"`
	// GroupAccess controls which group chats may send inbound messages.
	GroupAccess GroupAccessConfig `yaml:"group_access"`
	// Telegram configuration.
	Telegram TelegramConfig `yaml:"telegram"`
	// Discord configuration.
	Discord DiscordConfig `yaml:"discord"`
	// Slack configuration.
	Slack SlackConfig `yaml:"slack"`
	// WeChatWork configuration.
	WeChatWork WeChatWorkConfig `yaml:"wechat_work"`
	// Feishu configuration.
	Feishu FeishuConfig `yaml:"feishu"`
	// Matrix configuration.
	Matrix MatrixConfig `yaml:"matrix"`
	// iMessage configuration (macOS only).
	IMessage IMessageConfig `yaml:"imessage"`
	// WhatsApp configuration.
	WhatsApp WhatsAppConfig `yaml:"whatsapp"`
	// Signal configuration.
	Signal SignalConfig `yaml:"signal"`
	// Teams configuration.
	Teams TeamsConfig `yaml:"teams"`
	// Mattermost configuration.
	Mattermost MattermostConfig `yaml:"mattermost"`
	// Nextcloud Talk configuration.
	NextcloudTalk NextcloudTalkConfig `yaml:"nextcloudtalk"`
	// BlueBubbles configuration.
	BlueBubbles BlueBubblesConfig `yaml:"bluebubbles"`
	// Zalo configuration.
	Zalo ZaloConfig `yaml:"zalo"`
}

// TelegramConfig contains Telegram bot configuration.
type TelegramConfig struct {
	Enabled       bool     `yaml:"enabled"`
	BotToken      string   `yaml:"bot_token"`
	WebhookURL    string   `yaml:"webhook_url"`
	AllowedUsers  []string `yaml:"allowed_users"`
	AllowedGroups []string `yaml:"allowed_groups"`
	Proxy         string   `yaml:"proxy"`
}

// DiscordConfig contains Discord bot configuration.
type DiscordConfig struct {
	Enabled       bool     `yaml:"enabled"`
	BotToken      string   `yaml:"bot_token"`
	ApplicationID string   `yaml:"application_id"`
	AllowedGuilds []string `yaml:"allowed_guilds"`
	AllowedUsers  []string `yaml:"allowed_users"`
}

// SlackConfig contains Slack bot configuration.
type SlackConfig struct {
	Enabled         bool     `yaml:"enabled"`
	BotToken        string   `yaml:"bot_token"`
	AppToken        string   `yaml:"app_token"`
	SigningSecret   string   `yaml:"signing_secret"`
	AllowedChannels []string `yaml:"allowed_channels"`
	AllowedUsers    []string `yaml:"allowed_users"`
}

// WeChatWorkConfig contains WeChat Work configuration.
type WeChatWorkConfig struct {
	Enabled        bool   `yaml:"enabled"`
	CorpID         string `yaml:"corp_id"`
	AgentID        string `yaml:"agent_id"`
	Secret         string `yaml:"secret"`
	Token          string `yaml:"token"`
	EncodingAESKey string `yaml:"encoding_aes_key"`
	CallbackURL    string `yaml:"callback_url"`
}

// FeishuConfig contains Feishu/Lark configuration.
type FeishuConfig struct {
	Enabled           bool   `yaml:"enabled"`
	AppID             string `yaml:"app_id"`
	AppSecret         string `yaml:"app_secret"`
	VerificationToken string `yaml:"verification_token"`
	EncryptKey        string `yaml:"encrypt_key"`
	// DisableTypingReaction disables the transient Typing reaction indicator.
	DisableTypingReaction bool `yaml:"disable_typing_reaction"`
	// SessionMode sends replies as plain chat messages instead of the reply endpoint.
	SessionMode bool `yaml:"session_mode"`
}

// MatrixConfig contains Matrix configuration.
type MatrixConfig struct {
	Enabled      bool     `yaml:"enabled"`
	Homeserver   string   `yaml:"homeserver"`
	UserID       string   `yaml:"user_id"`
	AccessToken  string   `yaml:"access_token"`
	DeviceID     string   `yaml:"device_id"`
	AllowedRooms []string `yaml:"allowed_rooms"`
}

// IMessageConfig contains iMessage configuration (macOS only).
type IMessageConfig struct {
	Enabled        bool     `yaml:"enabled"`
	DatabasePath   string   `yaml:"database_path"`
	PollIntervalMS int      `yaml:"poll_interval_ms"`
	AllowedNumbers []string `yaml:"allowed_numbers"`
	AllowedEmails  []string `yaml:"allowed_emails"`
}

// WhatsAppConfig contains WhatsApp configuration.
type WhatsAppConfig struct {
	Enabled        bool     `yaml:"enabled"`
	PhoneNumber    string   `yaml:"phone_number"`
	SessionPath    string   `yaml:"session_path"`
	AllowedNumbers []string `yaml:"allowed_numbers"`
	QRTimeout      int      `yaml:"qr_timeout_seconds"`
	ReconnectDelay int      `yaml:"reconnect_delay_seconds"`
}

// SignalConfig contains Signal configuration.
type SignalConfig struct {
	Enabled        bool     `yaml:"enabled"`
	PhoneNumber    string   `yaml:"phone_number"`
	ConfigPath     string   `yaml:"config_path"`
	SignalCLIPath  string   `yaml:"signal_cli_path"`
	AllowedNumbers []string `yaml:"allowed_numbers"`
	UseJsonRpc     bool     `yaml:"use_json_rpc"`
}

// TeamsConfig contains Microsoft Teams configuration.
type TeamsConfig struct {
	Enabled      bool     `yaml:"enabled"`
	AppID        string   `yaml:"app_id"`
	AppPassword  string   `yaml:"app_password"`
	TenantID     string   `yaml:"tenant_id"`
	AllowedTeams []string `yaml:"allowed_teams"`
	AllowedUsers []string `yaml:"allowed_users"`
}

// MattermostConfig contains Mattermost configuration.
type MattermostConfig struct {
	Enabled         bool     `yaml:"enabled"`
	ServerURL       string   `yaml:"server_url"`
	BotToken        string   `yaml:"bot_token"`
	AllowedChannels []string `yaml:"allowed_channels"`
	AllowedUsers    []string `yaml:"allowed_users"`
}

// NextcloudTalkConfig contains Nextcloud Talk configuration.
type NextcloudTalkConfig struct {
	Enabled   bool   `yaml:"enabled"`
	ServerURL string `yaml:"server_url"`
	Username  string `yaml:"username"`
	Password  string `yaml:"password"`
	RoomToken string `yaml:"room_token"`
}

// BlueBubblesConfig contains BlueBubbles (iMessage bridge) configuration.
type BlueBubblesConfig struct {
	Enabled      bool     `yaml:"enabled"`
	ServerURL    string   `yaml:"server_url"`
	Password     string   `yaml:"password"`
	AllowedChats []string `yaml:"allowed_chats"`
}

// ZaloConfig contains Zalo Official Account configuration.
type ZaloConfig struct {
	Enabled      bool   `yaml:"enabled"`
	OAID         string `yaml:"oa_id"`
	AccessToken  string `yaml:"access_token"`
	RefreshToken string `yaml:"refresh_token"`
	AppID        string `yaml:"app_id"`
	SecretKey    string `yaml:"secret_key"`
}

// DefaultConfig returns the default channel configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:               false,
		DefaultTimeoutSeconds: 90,
		MaxMessageLength:      4096,
		Heartbeat:             DefaultHeartbeatConfig(),
		GroupAccess:           DefaultGroupAccessConfig(),
		Telegram: TelegramConfig{
			Enabled: false,
		},
		Discord: DiscordConfig{
			Enabled: false,
		},
		Slack: SlackConfig{
			Enabled: false,
		},
		WeChatWork: WeChatWorkConfig{
			Enabled: false,
		},
		Feishu: FeishuConfig{
			Enabled: false,
		},
		Matrix: MatrixConfig{
			Enabled: false,
		},
		IMessage: IMessageConfig{
			Enabled:        false,
			PollIntervalMS: 1000,
		},
		WhatsApp: WhatsAppConfig{
			Enabled:        false,
			SessionPath:    "./data/whatsapp",
			QRTimeout:      60,
			ReconnectDelay: 5,
		},
		Signal: SignalConfig{
			Enabled:       false,
			ConfigPath:    "./data/signal",
			SignalCLIPath: "signal-cli",
			UseJsonRpc:    true,
		},
		Teams: TeamsConfig{
			Enabled: false,
		},
		Mattermost: MattermostConfig{
			Enabled: false,
		},
		NextcloudTalk: NextcloudTalkConfig{
			Enabled: false,
		},
		BlueBubbles: BlueBubblesConfig{
			Enabled: false,
		},
		Zalo: ZaloConfig{
			Enabled: false,
		},
	}
}
