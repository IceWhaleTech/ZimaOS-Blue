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

// OutboundMarkdownMode controls how markdown payloads should be prepared before send.
type OutboundMarkdownMode uint8

const (
	// OutboundMarkdownModeChunked splits markdown into send-safe chunks.
	OutboundMarkdownModeChunked OutboundMarkdownMode = iota
	// OutboundMarkdownModePreserveWhole keeps the markdown body intact for channel-specific rendering.
	OutboundMarkdownModePreserveWhole
)

// OutboundCapabilities describes channel-specific outbound formatting behavior.
type OutboundCapabilities struct {
	// MarkdownMode controls how markdown text is chunked before send.
	MarkdownMode OutboundMarkdownMode `json:"markdown_mode,omitempty"`
	// HumanizerPreset selects the renderer used when humanizing plain-like outbound text.
	HumanizerPreset string `json:"humanizer_preset,omitempty"`
	// SupportsMarkdownFormat indicates the channel can accept structured markdown/html payloads.
	SupportsMarkdownFormat bool `json:"supports_markdown_format,omitempty"`
	// AutoPromoteMarkdownReport upgrades long structured plain text reports to markdown.
	AutoPromoteMarkdownReport bool `json:"auto_promote_markdown_report,omitempty"`
}

// DefaultOutboundCapabilities returns the default outbound behavior for channels.
func DefaultOutboundCapabilities() OutboundCapabilities {
	return OutboundCapabilities{MarkdownMode: OutboundMarkdownModeChunked}
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

// MessageSenderWithID is an optional interface for channels that can return
// the outbound message ID of a sent message. This enables later in-place edits.
type MessageSenderWithID interface {
	SendWithID(ctx context.Context, msg OutgoingMessage) (string, error)
}

// MessageEditor is an optional interface for channels that can edit an
// existing outbound message in place.
type MessageEditor interface {
	EditMessage(ctx context.Context, chatID string, messageID string, msg OutgoingMessage) error
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

// TypingReactionCleaner is an optional interface channels can implement to
// clear a transient typing reaction for a specific inbound message.
// This is used when newer user messages supersede older pending turns.
type TypingReactionCleaner interface {
	ClearTypingReaction(ctx context.Context, messageID string)
}

// OutboundCapabilityProvider is an optional interface for channels to describe
// how manager-level outbound preparation should treat markdown.
type OutboundCapabilityProvider interface {
	OutboundCapabilities() OutboundCapabilities
}

// BotMentionTargetProvider is an optional interface for channels to expose the
// stable bot identities that inbound mentions should be matched against.
type BotMentionTargetProvider interface {
	BotMentionTargets() []string
}
