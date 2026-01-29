package server

import (
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel/discord"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel/feishu"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel/matrix"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel/slack"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel/telegram"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel/wechat"
)

// ChannelFactory creates channel instances from stored configurations.
type ChannelFactory struct {
	logger *zap.Logger
}

// NewChannelFactory creates a new channel factory.
func NewChannelFactory(logger *zap.Logger) *ChannelFactory {
	return &ChannelFactory{logger: logger}
}

// CreateChannel creates a channel instance from a stored configuration.
func (f *ChannelFactory) CreateChannel(cfg *ChannelConfig) (channel.Channel, error) {
	switch cfg.ID {
	case "telegram":
		return f.createTelegram(cfg)
	case "discord":
		return f.createDiscord(cfg)
	case "feishu":
		return f.createFeishu(cfg)
	case "slack":
		return f.createSlack(cfg)
	case "wechat":
		return f.createWechat(cfg)
	case "matrix":
		return f.createMatrix(cfg)
	// These channels are not yet fully implemented or require special setup
	case "whatsapp", "signal", "teams", "googlechat", "dingtalk", "qq", "imessage":
		return nil, nil // Not supported yet
	default:
		return nil, nil // Unknown channel type
	}
}

func (f *ChannelFactory) createTelegram(cfg *ChannelConfig) (channel.Channel, error) {
	telegramCfg := channel.TelegramConfig{
		Enabled:  cfg.Enabled,
		BotToken: cfg.Config["bot_token"],
	}
	return telegram.New(telegramCfg, f.logger), nil
}

func (f *ChannelFactory) createDiscord(cfg *ChannelConfig) (channel.Channel, error) {
	discordCfg := channel.DiscordConfig{
		Enabled:       cfg.Enabled,
		BotToken:      cfg.Config["bot_token"],
		ApplicationID: cfg.Config["application_id"],
	}
	return discord.New(discordCfg, f.logger), nil
}

func (f *ChannelFactory) createFeishu(cfg *ChannelConfig) (channel.Channel, error) {
	feishuCfg := channel.FeishuConfig{
		Enabled:           cfg.Enabled,
		AppID:             cfg.Config["app_id"],
		AppSecret:         cfg.Config["app_secret"],
		VerificationToken: cfg.Config["verification_token"],
		EncryptKey:        cfg.Config["encrypt_key"],
	}
	return feishu.New(feishuCfg, f.logger), nil
}

func (f *ChannelFactory) createSlack(cfg *ChannelConfig) (channel.Channel, error) {
	slackCfg := channel.SlackConfig{
		Enabled:       cfg.Enabled,
		BotToken:      cfg.Config["bot_token"],
		AppToken:      cfg.Config["app_token"],
		SigningSecret: cfg.Config["signing_secret"],
	}
	return slack.New(slackCfg, f.logger), nil
}

func (f *ChannelFactory) createWechat(cfg *ChannelConfig) (channel.Channel, error) {
	wechatCfg := channel.WeChatWorkConfig{
		Enabled: cfg.Enabled,
		CorpID:  cfg.Config["corp_id"],
		AgentID: cfg.Config["agent_id"],
		Secret:  cfg.Config["secret"],
	}
	return wechat.New(wechatCfg, f.logger), nil
}

func (f *ChannelFactory) createMatrix(cfg *ChannelConfig) (channel.Channel, error) {
	matrixCfg := channel.MatrixConfig{
		Enabled:     cfg.Enabled,
		Homeserver:  cfg.Config["homeserver"],
		UserID:      cfg.Config["user_id"],
		AccessToken: cfg.Config["access_token"],
	}
	return matrix.New(matrixCfg, f.logger), nil
}
