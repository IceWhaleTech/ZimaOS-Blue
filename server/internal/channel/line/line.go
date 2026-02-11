package line
import (
	"context"
	"sync"
	"sync/atomic"
	"go.uber.org/zap"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel/validator"
)
type Config struct {
	Enabled bool `mapstructure:"enabled"`
	Token   string `mapstructure:"token"`
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
		logger:   logger.With(zap.String("channel", "line")),
		messages: make(chan channel.Message, 100),
		status:   channel.StatusDisconnected,
	}
}
func (c *Channel) Name() string { return "line" }
func (c *Channel) Type() string { return "line" }
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
	return nil
}
func (c *Channel) SendStreaming(ctx context.Context, chatID string, replyToID string, content <-chan string, done chan<- struct{}) error {
	defer close(done)
	for range content {}
	return nil
}
func (c *Channel) Info() channel.Info {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return channel.Info{
		Name: "line", Type: "line", Status: c.status, Enabled: c.config.Enabled,
		MessageCount: c.msgCount.Load(),
	}
}
type Validator struct{}
func NewValidator() *Validator { return &Validator{} }
func (v *Validator) Validate(ctx context.Context, config map[string]string) validator.Result {
	if config["token"] == "" {
		return validator.Result{Success: false, Error: "token is required"}
	}
	return validator.Result{Success: true, MessageKey: "channels.connectionSuccess"}
}
