package server

import (
	"context"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/bluebubbles"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/dingtalk"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/discord"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/feishu"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/googlechat"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/imessage"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/instagram"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/line"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/matrix"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/mattermost"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/messenger"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/nextcloudtalk"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/nostr"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/qq"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/signal"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/slack"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/teams"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/telegram"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/twitch"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/twitter"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/validator"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/viber"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/wechat"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/wechatilink"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/whatsapp"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel/zalo"
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
	case "wechat_ilink":
		return f.createWechatILink(cfg)
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
	case "nextcloudtalk":
		return f.createNextcloudTalk(cfg)
	case "nostr":
		return f.createNostr(cfg)
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
	disableTypingReaction := false
	if v, ok := parseConfigBool(cfg.Config["typing_indicator"]); ok {
		disableTypingReaction = !v
	}
	if v, ok := parseConfigBool(cfg.Config["typingIndicator"]); ok {
		disableTypingReaction = !v
	}
	if v, ok := parseConfigBool(cfg.Config["disable_typing_reaction"]); ok {
		disableTypingReaction = v
	}
	if v, ok := parseConfigBool(cfg.Config["disableTypingReaction"]); ok {
		disableTypingReaction = v
	}

	sessionMode := false
	if v, ok := parseConfigBool(cfg.Config["session_mode"]); ok {
		sessionMode = v
	}
	if v, ok := parseConfigBool(cfg.Config["sessionMode"]); ok {
		sessionMode = v
	}
	if v, ok := parseConfigBool(cfg.Config["reply_in_thread"]); ok {
		sessionMode = !v
	}
	if v, ok := parseConfigBool(cfg.Config["replyInThread"]); ok {
		sessionMode = !v
	}
	if raw := strings.TrimSpace(strings.ToLower(cfg.Config["reply_mode"])); raw != "" {
		switch raw {
		case "session", "chat", "flat", "new_topic", "topic":
			sessionMode = true
		case "reply", "thread":
			sessionMode = false
		}
	}

	feishuCfg := channel.FeishuConfig{
		Enabled:               cfg.Enabled,
		AppID:                 cfg.Config["app_id"],
		AppSecret:             cfg.Config["app_secret"],
		VerificationToken:     cfg.Config["verification_token"],
		EncryptKey:            cfg.Config["encrypt_key"],
		DisableTypingReaction: disableTypingReaction,
		SessionMode:           sessionMode,
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

func (f *ChannelFactory) createWechatILink(cfg *ChannelConfig) (channel.Channel, error) {
	wechatCfg := channel.WeChatILinkConfig{
		Enabled:    cfg.Enabled,
		APIBaseURL: cfg.Config["api_base_url"],
		BotToken:   cfg.Config["bot_token"],
	}
	return wechatilink.New(wechatCfg, f.logger), nil
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
		CLIPath:     cfg.Config["cli_path"],
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
	return googlechat.New(googlechat.Config{
		Enabled:    cfg.Enabled,
		WebhookURL: cfg.Config["webhook_url"],
		SpaceID:    cfg.Config["space_id"],
		APIKey:     cfg.Config["api_key"],
	}, f.logger), nil
}

func (f *ChannelFactory) createLine(cfg *ChannelConfig) (channel.Channel, error) {
	return line.New(line.Config{
		Enabled:            cfg.Enabled,
		ChannelID:          cfg.Config["channel_id"],
		ChannelSecret:      cfg.Config["channel_secret"],
		ChannelAccessToken: cfg.Config["channel_access_token"],
	}, f.logger), nil
}

func (f *ChannelFactory) createMessenger(cfg *ChannelConfig) (channel.Channel, error) {
	return messenger.New(messenger.Config{
		Enabled:         cfg.Enabled,
		PageAccessToken: cfg.Config["page_access_token"],
		AppSecret:       cfg.Config["app_secret"],
		VerifyToken:     cfg.Config["verify_token"],
	}, f.logger), nil
}

func (f *ChannelFactory) createViber(cfg *ChannelConfig) (channel.Channel, error) {
	return viber.New(viber.Config{
		Enabled:    cfg.Enabled,
		AuthToken:  cfg.Config["auth_token"],
		BotName:    cfg.Config["bot_name"],
		BotAvatar:  cfg.Config["bot_avatar"],
		WebhookURL: cfg.Config["webhook_url"],
	}, f.logger), nil
}

func (f *ChannelFactory) createTwitter(cfg *ChannelConfig) (channel.Channel, error) {
	return twitter.New(twitter.Config{
		Enabled:           cfg.Enabled,
		APIKey:            cfg.Config["api_key"],
		APISecret:         cfg.Config["api_secret"],
		AccessToken:       cfg.Config["access_token"],
		AccessTokenSecret: cfg.Config["access_token_secret"],
		BearerToken:       cfg.Config["bearer_token"],
	}, f.logger), nil
}

func (f *ChannelFactory) createInstagram(cfg *ChannelConfig) (channel.Channel, error) {
	return instagram.New(instagram.Config{
		Enabled:         cfg.Enabled,
		PageAccessToken: cfg.Config["page_access_token"],
		AppSecret:       cfg.Config["app_secret"],
		VerifyToken:     cfg.Config["verify_token"],
		IGAccountID:     cfg.Config["ig_account_id"],
	}, f.logger), nil
}

func (f *ChannelFactory) createTwitch(cfg *ChannelConfig) (channel.Channel, error) {
	return twitch.New(twitch.Config{
		Enabled:      cfg.Enabled,
		ClientID:     cfg.Config["client_id"],
		ClientSecret: cfg.Config["client_secret"],
		OAuthToken:   cfg.Config["oauth_token"],
		BotUsername:  cfg.Config["bot_username"],
		Channels:     cfg.Config["channels"],
	}, f.logger), nil
}

func (f *ChannelFactory) createQQ(cfg *ChannelConfig) (channel.Channel, error) {
	return qq.New(qq.Config{
		Enabled:   cfg.Enabled,
		AppID:     cfg.Config["app_id"],
		AppSecret: cfg.Config["app_secret"],
		Token:     cfg.Config["token"],
		Sandbox:   cfg.Config["sandbox"] == "true",
	}, f.logger), nil
}

func (f *ChannelFactory) createNextcloudTalk(cfg *ChannelConfig) (channel.Channel, error) {
	return nextcloudtalk.New(nextcloudtalk.Config{
		Enabled:   cfg.Enabled,
		ServerURL: cfg.Config["server_url"],
		Username:  cfg.Config["username"],
		Password:  cfg.Config["password"],
		RoomToken: cfg.Config["room_token"],
	}, f.logger), nil
}

func (f *ChannelFactory) createNostr(cfg *ChannelConfig) (channel.Channel, error) {
	return nostr.New(nostr.Config{
		Enabled:    cfg.Enabled,
		PrivateKey: cfg.Config["private_key"],
		PublicKey:  cfg.Config["public_key"],
		Relays:     cfg.Config["relays"],
	}, f.logger), nil
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
	case "wechat_ilink":
		v = wechatilink.NewValidator()
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
	case "nextcloudtalk":
		v = nextcloudtalk.NewValidator()
	case "nostr":
		v = nostr.NewValidator()
	case "line":
		v = line.NewValidator()
	case "messenger":
		v = messenger.NewValidator()
	case "viber":
		v = viber.NewValidator()
	case "twitter":
		v = twitter.NewValidator()
	case "instagram":
		v = instagram.NewValidator()
	case "twitch":
		v = twitch.NewValidator()
	case "qq":
		v = qq.NewValidator()
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

func parseConfigBool(raw string) (bool, bool) {
	s := strings.TrimSpace(strings.ToLower(raw))
	if s == "" {
		return false, false
	}
	switch s {
	case "1", "true", "yes", "y", "on":
		return true, true
	case "0", "false", "no", "n", "off":
		return false, true
	default:
		v, err := strconv.ParseBool(s)
		if err != nil {
			return false, false
		}
		return v, true
	}
}
