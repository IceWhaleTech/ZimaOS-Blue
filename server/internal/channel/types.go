// Package channel provides the abstraction layer for messaging channels.
// It defines interfaces and types for integrating various messaging platforms
// like Telegram, Discord, Slack, WeChat Work, Feishu, and Matrix.
package channel

import (
	"context"
	"time"
)

// Status represents the current state of a channel.
type Status string

const (
	// StatusDisconnected indicates the channel is not connected.
	StatusDisconnected Status = "disconnected"
	// StatusConnecting indicates the channel is attempting to connect.
	StatusConnecting Status = "connecting"
	// StatusConnected indicates the channel is connected and ready.
	StatusConnected Status = "connected"
	// StatusReconnecting indicates the channel is attempting to reconnect.
	StatusReconnecting Status = "reconnecting"
	// StatusError indicates the channel encountered an error.
	StatusError Status = "error"
)

// MessageType represents the type of message content.
type MessageType string

const (
	// MessageTypeText represents a plain text message.
	MessageTypeText MessageType = "text"
	// MessageTypeImage represents an image message.
	MessageTypeImage MessageType = "image"
	// MessageTypeFile represents a file attachment.
	MessageTypeFile MessageType = "file"
	// MessageTypeAudio represents an audio message.
	MessageTypeAudio MessageType = "audio"
	// MessageTypeVideo represents a video message.
	MessageTypeVideo MessageType = "video"
	// MessageTypeCard represents a rich card/embed message.
	MessageTypeCard MessageType = "card"
)

// Message represents a unified message format across all channels.
type Message struct {
	// ID is the unique identifier of the message.
	ID string `json:"id"`
	// ChannelName is the name of the channel this message belongs to.
	ChannelName string `json:"channel_name"`
	// ChatID is the identifier of the chat/conversation.
	ChatID string `json:"chat_id"`
	// UserID is the identifier of the user who sent the message.
	UserID string `json:"user_id"`
	// Username is the display name of the user.
	Username string `json:"username"`
	// Type is the type of message content.
	Type MessageType `json:"type"`
	// Content is the text content of the message.
	Content string `json:"content"`
	// ReplyToID is the ID of the message being replied to (if any).
	ReplyToID string `json:"reply_to_id,omitempty"`
	// Attachments contains any file attachments.
	Attachments []Attachment `json:"attachments,omitempty"`
	// Metadata contains channel-specific metadata.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	// Timestamp is when the message was created.
	Timestamp time.Time `json:"timestamp"`
	// IsGroup indicates if the message is from a group chat.
	IsGroup bool `json:"is_group"`
	// GroupName is the name of the group (if IsGroup is true).
	GroupName string `json:"group_name,omitempty"`
}

// Attachment represents a file attachment in a message.
type Attachment struct {
	// ID is the unique identifier of the attachment.
	ID string `json:"id"`
	// Type is the type of attachment.
	Type MessageType `json:"type"`
	// Name is the filename.
	Name string `json:"name"`
	// URL is the download URL.
	URL string `json:"url,omitempty"`
	// Data is the raw file data (for small files).
	Data []byte `json:"-"`
	// Thumbnail is the cover/thumbnail image data (for video attachments).
	Thumbnail []byte `json:"-"`
	// Size is the file size in bytes.
	Size int64 `json:"size"`
	// MimeType is the MIME type of the file.
	MimeType string `json:"mime_type"`
}

// OutgoingMessage represents a message to be sent through a channel.
type OutgoingMessage struct {
	// ChatID is the target chat/conversation ID.
	ChatID string `json:"chat_id"`
	// Content is the text content to send.
	Content string `json:"content"`
	// ReplyToID is the message ID to reply to (optional).
	ReplyToID string `json:"reply_to_id,omitempty"`
	// Attachments are files to send with the message.
	Attachments []Attachment `json:"attachments,omitempty"`
	// Format specifies the message format (markdown, html, plain).
	Format string `json:"format,omitempty"`
	// Metadata contains channel-specific options.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Info contains information about a channel.
type Info struct {
	// Name is the unique identifier of the channel.
	Name string `json:"name"`
	// Type is the channel type (telegram, discord, etc.).
	Type string `json:"type"`
	// Status is the current connection status.
	Status Status `json:"status"`
	// Enabled indicates if the channel is enabled in config.
	Enabled bool `json:"enabled"`
	// ConnectedAt is when the channel connected (if connected).
	ConnectedAt *time.Time `json:"connected_at,omitempty"`
	// LastError is the last error message (if any).
	LastError string `json:"last_error,omitempty"`
	// LastErrorAt is when the last error occurred.
	LastErrorAt *time.Time `json:"last_error_at,omitempty"`
	// MessageCount is the total number of messages processed.
	MessageCount int64 `json:"message_count"`
	// MessagesReceived is the number of messages received from users.
	MessagesReceived int64 `json:"messages_received"`
	// MessagesSent is the number of messages sent (replies).
	MessagesSent int64 `json:"messages_sent"`
	// LastMessageAt is when the last message was received.
	LastMessageAt *time.Time `json:"last_message_at,omitempty"`
	// LastReplyAt is when the last reply was sent.
	LastReplyAt *time.Time `json:"last_reply_at,omitempty"`
	// Metadata contains channel-specific information.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Channel defines the interface that all messaging channels must implement.
type Channel interface {
	// Name returns the unique name of this channel.
	Name() string

	// Type returns the channel type (e.g., "telegram", "discord").
	Type() string

	// Start initializes and starts the channel.
	// It should establish connections and begin listening for messages.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the channel.
	// It should close connections and clean up resources.
	Stop(ctx context.Context) error

	// Send sends a message through the channel.
	Send(ctx context.Context, msg OutgoingMessage) error

	// SendStreaming sends a message with streaming support.
	// The content channel receives chunks of the response.
	// The done channel is closed when streaming is complete.
	SendStreaming(ctx context.Context, chatID string, replyToID string, content <-chan string, done chan<- struct{}) error

	// Info returns current information about the channel.
	Info() Info

	// IsConnected returns true if the channel is currently connected.
	IsConnected() bool

	// Messages returns a channel for receiving incoming messages.
	// This channel is closed when the channel is stopped.
	Messages() <-chan Message
}

// MessageHandler is a function that handles incoming messages.
type MessageHandler func(ctx context.Context, msg Message) (*OutgoingMessage, error)

// StreamingHandler is a function that handles incoming messages with streaming response.
type StreamingHandler func(ctx context.Context, msg Message) (<-chan string, error)

// TypingIndicator is an optional interface channels can implement to show typing status.
// When a message is received, the manager will call SendTyping before invoking the handler
// so the user sees the bot is working while the LLM processes the request.
type TypingIndicator interface {
	// SendTyping sends a "typing" indicator to the given chat.
	// Best-effort: errors are logged but not propagated.
	SendTyping(ctx context.Context, chatID string) error
}

// Config contains common configuration for all channels.
type Config struct {
	// Enabled indicates if channels are globally enabled.
	Enabled bool `yaml:"enabled"`
	// DefaultTimeoutSeconds is the default timeout for operations.
	DefaultTimeoutSeconds int `yaml:"default_timeout_seconds"`
	// MaxMessageLength is the maximum message length.
	MaxMessageLength int `yaml:"max_message_length"`
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
		DefaultTimeoutSeconds: 30,
		MaxMessageLength:      4096,
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
		BlueBubbles: BlueBubblesConfig{
			Enabled: false,
		},
		Zalo: ZaloConfig{
			Enabled: false,
		},
	}
}
