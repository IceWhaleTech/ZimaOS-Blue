package server

import (
	"context"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel/bluebubbles"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel/dingtalk"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel/discord"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel/feishu"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel/googlechat"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel/imessage"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel/instagram"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel/line"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel/matrix"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel/mattermost"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel/messenger"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel/qq"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel/signal"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel/slack"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel/teams"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel/telegram"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel/twitch"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel/twitter"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel/validator"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel/viber"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel/wechat"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel/whatsapp"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel/zalo"
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
	case "teams":
		return f.createTeams(cfg)
	case "mattermost":
		return f.createMattermost(cfg)
	case "bluebubbles":
		return f.createBlueBubbles(cfg)
	case "zalo":
		return f.createZalo(cfg)
	case "whatsapp":
		return f.createWhatsApp(cfg)
	case "signal":
		return f.createSignal(cfg)
	case "dingtalk":
		return f.createDingTalk(cfg)
	case "imessage":
		return f.createIMessage(cfg)
	case "googlechat":
		return f.createGoogleChat(cfg)
	case "line":
		return f.createLine(cfg)
	case "messenger":
		return f.createMessenger(cfg)
	case "viber":
		return f.createViber(cfg)
	case "twitter":
		return f.createTwitter(cfg)
	case "instagram":
		return f.createInstagram(cfg)
	case "twitch":
		return f.createTwitch(cfg)
	case "qq":
		return f.createQQ(cfg)
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

func (f *ChannelFactory) createTeams(cfg *ChannelConfig) (channel.Channel, error) {
	teamsCfg := channel.TeamsConfig{
		Enabled:     cfg.Enabled,
		AppID:       cfg.Config["app_id"],
		AppPassword: cfg.Config["app_password"],
		TenantID:    cfg.Config["tenant_id"],
	}
	return teams.New(teamsCfg, f.logger), nil
}

func (f *ChannelFactory) createMattermost(cfg *ChannelConfig) (channel.Channel, error) {
	mattermostCfg := channel.MattermostConfig{
		Enabled:   cfg.Enabled,
		ServerURL: cfg.Config["server_url"],
		BotToken:  cfg.Config["bot_token"],
	}
	return mattermost.New(mattermostCfg, f.logger), nil
}

func (f *ChannelFactory) createBlueBubbles(cfg *ChannelConfig) (channel.Channel, error) {
	bluebubblesCfg := channel.BlueBubblesConfig{
		Enabled:   cfg.Enabled,
		ServerURL: cfg.Config["server_url"],
		Password:  cfg.Config["password"],
	}
	return bluebubbles.New(bluebubblesCfg, f.logger), nil
}

func (f *ChannelFactory) createZalo(cfg *ChannelConfig) (channel.Channel, error) {
	zaloCfg := channel.ZaloConfig{
		Enabled:      cfg.Enabled,
		OAID:         cfg.Config["oa_id"],
		AccessToken:  cfg.Config["access_token"],
		RefreshToken: cfg.Config["refresh_token"],
		AppID:        cfg.Config["app_id"],
		SecretKey:    cfg.Config["secret_key"],
	}
	return zalo.New(zaloCfg, f.logger), nil
}

func (f *ChannelFactory) createWhatsApp(cfg *ChannelConfig) (channel.Channel, error) {
	whatsappCfg := whatsapp.Config{
		Enabled:     cfg.Enabled,
		PhoneNumber: cfg.Config["phone_number"],
		SessionPath: cfg.Config["session_path"],
	}
	if whatsappCfg.SessionPath == "" {
		whatsappCfg.SessionPath = "./data/whatsapp"
	}
	return whatsapp.New(whatsappCfg, f.logger), nil
}

func (f *ChannelFactory) createSignal(cfg *ChannelConfig) (channel.Channel, error) {
	signalCfg := signal.Config{
		Enabled:     cfg.Enabled,
		PhoneNumber: cfg.Config["phone_number"],
		ConfigPath:  cfg.Config["config_path"],
	}
	if signalCfg.ConfigPath == "" {
		signalCfg.ConfigPath = "./data/signal"
	}
	return signal.New(signalCfg, f.logger), nil
}

func (f *ChannelFactory) createDingTalk(cfg *ChannelConfig) (channel.Channel, error) {
	dingtalkCfg := dingtalk.Config{
		Enabled:    cfg.Enabled,
		AppKey:     cfg.Config["app_key"],
		AppSecret:  cfg.Config["app_secret"],
		RobotCode:  cfg.Config["robot_code"],
		WebhookURL: cfg.Config["webhook_url"],
	}
	return dingtalk.New(dingtalkCfg, f.logger), nil
}

func (f *ChannelFactory) createIMessage(cfg *ChannelConfig) (channel.Channel, error) {
	imessageCfg := imessage.Config{
		Enabled:      cfg.Enabled,
		DatabasePath: cfg.Config["database_path"],
	}
	return imessage.New(imessageCfg, f.logger), nil
}

func (f *ChannelFactory) createGoogleChat(cfg *ChannelConfig) (channel.Channel, error) {
	return googlechat.New(googlechat.Config{Enabled: cfg.Enabled, WebhookURL: cfg.Config["webhook_url"]}, f.logger), nil
}

func (f *ChannelFactory) createLine(cfg *ChannelConfig) (channel.Channel, error) {
	return line.New(line.Config{Enabled: cfg.Enabled, Token: cfg.Config["token"]}, f.logger), nil
}

func (f *ChannelFactory) createMessenger(cfg *ChannelConfig) (channel.Channel, error) {
	return messenger.New(messenger.Config{Enabled: cfg.Enabled, Token: cfg.Config["token"]}, f.logger), nil
}

func (f *ChannelFactory) createViber(cfg *ChannelConfig) (channel.Channel, error) {
	return viber.New(viber.Config{Enabled: cfg.Enabled, Token: cfg.Config["token"]}, f.logger), nil
}

func (f *ChannelFactory) createTwitter(cfg *ChannelConfig) (channel.Channel, error) {
	return twitter.New(twitter.Config{Enabled: cfg.Enabled, Token: cfg.Config["token"]}, f.logger), nil
}

func (f *ChannelFactory) createInstagram(cfg *ChannelConfig) (channel.Channel, error) {
	return instagram.New(instagram.Config{Enabled: cfg.Enabled, Token: cfg.Config["token"]}, f.logger), nil
}

func (f *ChannelFactory) createTwitch(cfg *ChannelConfig) (channel.Channel, error) {
	return twitch.New(twitch.Config{Enabled: cfg.Enabled, Token: cfg.Config["token"]}, f.logger), nil
}

func (f *ChannelFactory) createQQ(cfg *ChannelConfig) (channel.Channel, error) {
	return qq.New(qq.Config{Enabled: cfg.Enabled, Token: cfg.Config["token"]}, f.logger), nil
}

// ValidationResult represents the result of a connection validation.
type ValidationResult struct {
	Success    bool                   `json:"success"`
	Message    string                 `json:"message"`
	MessageKey string                 `json:"message_key,omitempty"`
	Details    map[string]interface{} `json:"details,omitempty"`
}

// ValidateConnection tests a channel connection without creating a full channel instance.
func (f *ChannelFactory) ValidateConnection(ctx context.Context, channelType string, config map[string]string) ValidationResult {
	var v validator.Validator

	switch channelType {
	case "telegram":
		v = telegram.NewValidator()
	case "discord":
		v = discord.NewValidator()
	case "feishu":
		v = feishu.NewValidator()
	case "slack":
		v = slack.NewValidator()
	case "wechat":
		v = wechat.NewValidator()
	case "matrix":
		v = matrix.NewValidator()
	case "teams":
		v = teams.NewValidator()
	case "mattermost":
		v = mattermost.NewValidator()
	case "bluebubbles":
		v = bluebubbles.NewValidator()
	case "zalo":
		v = zalo.NewValidator()
	case "whatsapp", "signal":
		// These channels now have validators
		if channelType == "whatsapp" {
			v = whatsapp.NewValidator()
		} else {
			v = signal.NewValidator()
		}
	case "dingtalk":
		v = dingtalk.NewValidator()
	case "imessage":
		v = imessage.NewValidator()
	case "googlechat":
		v = googlechat.NewValidator()
	case "line", "messenger", "viber", "twitter", "instagram", "twitch", "qq":
		// These channels have basic validators
		if channelType == "line" {
			v = line.NewValidator()
		} else if channelType == "messenger" {
			v = messenger.NewValidator()
		} else if channelType == "viber" {
			v = viber.NewValidator()
		} else if channelType == "twitter" {
			v = twitter.NewValidator()
		} else if channelType == "instagram" {
			v = instagram.NewValidator()
		} else if channelType == "twitch" {
			v = twitch.NewValidator()
		} else {
			v = qq.NewValidator()
		}
	default:
		return ValidationResult{
			Success: false,
			Message: "Unsupported channel type: " + channelType,
		}
	}

	result := v.Validate(ctx, config)

	message := result.MessageKey
	if result.Error != "" {
		message = result.Error
	}

	return ValidationResult{
		Success:    result.Success,
		Message:    message,
		MessageKey: result.MessageKey,
		Details:    result.Data,
	}
}
