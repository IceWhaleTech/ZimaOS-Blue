package channel

import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channelconfig"

// Pure channel configuration types live in channelconfig so config loading
// does not need to import the runtime channel package.
type (
	HeartbeatConfig     = channelconfig.HeartbeatConfig
	Config              = channelconfig.Config
	TelegramConfig      = channelconfig.TelegramConfig
	DiscordConfig       = channelconfig.DiscordConfig
	SlackConfig         = channelconfig.SlackConfig
	WeChatWorkConfig    = channelconfig.WeChatWorkConfig
	FeishuConfig        = channelconfig.FeishuConfig
	MatrixConfig        = channelconfig.MatrixConfig
	IMessageConfig      = channelconfig.IMessageConfig
	WhatsAppConfig      = channelconfig.WhatsAppConfig
	SignalConfig        = channelconfig.SignalConfig
	TeamsConfig         = channelconfig.TeamsConfig
	MattermostConfig    = channelconfig.MattermostConfig
	NextcloudTalkConfig = channelconfig.NextcloudTalkConfig
	BlueBubblesConfig   = channelconfig.BlueBubblesConfig
	ZaloConfig          = channelconfig.ZaloConfig
)

func DefaultHeartbeatConfig() HeartbeatConfig {
	return channelconfig.DefaultHeartbeatConfig()
}

func DefaultConfig() Config {
	return channelconfig.DefaultConfig()
}
