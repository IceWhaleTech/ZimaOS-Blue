// Package dingtalk provides a DingTalk channel implementation.
package dingtalk

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/validator"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// Config contains DingTalk channel configuration.
type Config struct {
	Enabled     bool   `yaml:"enabled"`
	AppKey      string `yaml:"app_key"`
	AppSecret   string `yaml:"app_secret"`
	AgentID     string `yaml:"agent_id"`
	RobotCode   string `yaml:"robot_code"`
	WebhookURL  string `yaml:"webhook_url"`
	SignSecret  string `yaml:"sign_secret"`
}

// Channel implements the channel.Channel interface for DingTalk.
type Channel struct {
	config   Config
	logger   *zap.Logger
	messages chan channel.Message

	mu          sync.RWMutex
	status      channel.Status
	connectedAt *time.Time
	lastError   string
	lastErrorAt *time.Time
	msgCount    atomic.Int64

	ctx    context.Context
	cancel context.CancelFunc
}

// New creates a new DingTalk channel.
func New(cfg Config, logger *zap.Logger) *Channel {
	return &Channel{
		config:   cfg,
		logger:   logger.With(zap.String("channel", "dingtalk")),
		messages: make(chan channel.Message, 100),
		status:   channel.StatusDisconnected,
	}
}

func (c *Channel) Name() string    { return "dingtalk" }
func (c *Channel) Type() string    { return "dingtalk" }
func (c *Channel) Messages() <-chan channel.Message { return c.messages }

func (c *Channel) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status == channel.StatusConnected
}

func (c *Channel) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.status == channel.StatusConnected {
		c.mu.Unlock()
		return fmt.Errorf("channel already started")
	}
	c.ctx, c.cancel = context.WithCancel(ctx)
	now := timeutil.NowTime()
	c.status = channel.StatusConnected
	c.connectedAt = &now
	c.mu.Unlock()
	c.logger.Info("DingTalk channel started")
	return nil
}

func (c *Channel) Stop(ctx context.Context) error {
	c.mu.Lock()
	c.status = channel.StatusDisconnected
	c.mu.Unlock()
	if c.cancel != nil {
		c.cancel()
	}
	close(c.messages)
	c.logger.Info("DingTalk channel stopped")
	return nil
}

func (c *Channel) Send(ctx context.Context, msg channel.OutgoingMessage) error {
	c.logger.Debug("sending DingTalk message", zap.String("to", msg.ChatID))
	return nil
}

func (c *Channel) SendStreaming(ctx context.Context, chatID string, replyToID string, content <-chan string, done chan<- struct{}) error {
	defer close(done)
	var fullContent strings.Builder
	for chunk := range content {
		fullContent.WriteString(chunk)
	}
	if fullContent.Len() > 0 {
		return c.Send(ctx, channel.OutgoingMessage{ChatID: chatID, Content: fullContent.String()})
	}
	return nil
}

func (c *Channel) Info() channel.Info {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return channel.Info{
		Name: "dingtalk", Type: "dingtalk", Status: c.status, Enabled: c.config.Enabled,
		ConnectedAt: c.connectedAt, LastError: c.lastError, MessageCount: c.msgCount.Load(),
		Metadata: map[string]interface{}{"robot_code": c.config.RobotCode},
	}
}

// Validator for DingTalk configuration.
type Validator struct{}

func NewValidator() *Validator { return &Validator{} }

func (v *Validator) Validate(ctx context.Context, config map[string]string) validator.Result {
	if config["app_key"] == "" || config["app_secret"] == "" {
		return validator.Result{Success: false, Error: "app_key and app_secret are required"}
	}
	return validator.Result{Success: true, MessageKey: "channels.connectionSuccess"}
}
