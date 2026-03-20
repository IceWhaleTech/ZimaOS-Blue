package channel

import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channelconfig"

// Pure channel configuration types live in channelconfig so config loading
// does not need to import the runtime channel package.
type (
	HeartbeatConfig     = channelconfig.HeartbeatConfig
	GroupPolicy         = channelconfig.GroupPolicy
	GroupMentionPolicy  = channelconfig.GroupMentionPolicy
	GroupAccessConfig   = channelconfig.GroupAccessConfig
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

const (
	GroupPolicyOpen             = channelconfig.GroupPolicyOpen
	GroupPolicyAllowlist        = channelconfig.GroupPolicyAllowlist
	GroupPolicyDisabled         = channelconfig.GroupPolicyDisabled
	GroupMentionPolicyMentioned = channelconfig.GroupMentionPolicyMentioned
	GroupMentionPolicyAlways    = channelconfig.GroupMentionPolicyAlways
)

func DefaultHeartbeatConfig() HeartbeatConfig {
	return channelconfig.DefaultHeartbeatConfig()
}

func DefaultGroupAccessConfig() GroupAccessConfig {
	return channelconfig.DefaultGroupAccessConfig()
}

func DefaultConfig() Config {
	return channelconfig.DefaultConfig()
}
