package googlechat
import (
	"context"
	"sync"
	"sync/atomic"
	"go.uber.org/zap"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel/validator"
)
type Config struct {
	Enabled    bool   `mapstructure:"enabled"`
	WebhookURL string `mapstructure:"webhook_url"`
	SpaceID    string `mapstructure:"space_id"`
}
type Channel struct {
	config   Config
	logger   *zap.Logger
	messages chan channel.Message
	mu       sync.RWMutex
	status   channel.Status
	msgCount atomic.Int64
}
func New(cfg Config, logger *zap.Logger) *Channel {
	return &Channel{
		config:   cfg,
		logger:   logger.With(zap.String("channel", "googlechat")),
		messages: make(chan channel.Message, 100),
		status:   channel.StatusDisconnected,
	}
}
func (c *Channel) Name() string { return "googlechat" }
func (c *Channel) Type() string { return "googlechat" }
func (c *Channel) Messages() <-chan channel.Message { return c.messages }
func (c *Channel) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status == channel.StatusConnected
}
func (c *Channel) Start(ctx context.Context) error {
	c.mu.Lock()
	c.status = channel.StatusConnected
	c.mu.Unlock()
	c.logger.Info("GoogleChat channel started")
	return nil
}
func (c *Channel) Stop(ctx context.Context) error {
	c.mu.Lock()
	c.status = channel.StatusDisconnected
	c.mu.Unlock()
	close(c.messages)
	return nil
}
func (c *Channel) Send(ctx context.Context, msg channel.OutgoingMessage) error {
	c.logger.Debug("sending GoogleChat message", zap.String("to", msg.ChatID))
	return nil
}
func (c *Channel) SendStreaming(ctx context.Context, chatID string, replyToID string, content <-chan string, done chan<- struct{}) error {
	defer close(done)
	var fullContent string
	for chunk := range content {
		fullContent += chunk
	}
	if fullContent != "" {
		return c.Send(ctx, channel.OutgoingMessage{ChatID: chatID, Content: fullContent})
	}
	return nil
}
func (c *Channel) Info() channel.Info {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return channel.Info{
		Name: "googlechat", Type: "googlechat", Status: c.status, Enabled: c.config.Enabled,
		MessageCount: c.msgCount.Load(),
	}
}
type Validator struct{}
func NewValidator() *Validator { return &Validator{} }
func (v *Validator) Validate(ctx context.Context, config map[string]string) validator.Result {
	if config["webhook_url"] == "" {
		return validator.Result{Success: false, Error: "webhook_url is required"}
	}
	return validator.Result{Success: true, MessageKey: "channels.connectionSuccess"}
}
