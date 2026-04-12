// Icon mapping for channels and plugins
// Maps channel/plugin type/id to their respective icon paths
import { publicAsset } from '@/utils/publicAsset'

export const channelIcons: Record<string, string> = {
  // Messaging channels
  telegram: publicAsset('icons/channels/telegram.svg'),
  discord: publicAsset('icons/channels/discord.svg'),
  slack: publicAsset('icons/channels/slack.svg'),
  whatsapp: publicAsset('icons/channels/whatsapp.svg'),
  wechat: publicAsset('icons/channels/wechat.svg'),
  signal: publicAsset('icons/channels/signal.svg'),
  matrix: publicAsset('icons/channels/matrix.svg'),
  imessage: publicAsset('icons/channels/imessage.svg'),
  feishu: publicAsset('icons/channels/feishu.svg'),
  dingtalk: publicAsset('icons/channels/dingtalk.svg'),
  qq: publicAsset('icons/channels/qq.svg'),
  teams: publicAsset('icons/channels/teams.svg'),
  'microsoft-teams': publicAsset('icons/channels/teams.svg'),
  msteams: publicAsset('icons/channels/teams.svg'),
  nextcloud: publicAsset('icons/channels/nextcloud.svg'),
  'nextcloud-talk': publicAsset('icons/channels/nextcloud.svg'),
  nextcloudtalk: publicAsset('icons/channels/nextcloud.svg'),
  // Social & messaging
  twitter: publicAsset('icons/channels/twitter.svg'),
  x: publicAsset('icons/channels/twitter.svg'),
  instagram: publicAsset('icons/channels/instagram.svg'),
  messenger: publicAsset('icons/channels/messenger.svg'),
  'facebook-messenger': publicAsset('icons/channels/messenger.svg'),
  viber: publicAsset('icons/channels/viber.svg'),
  // Tools & Utilities
  browser: publicAsset('icons/channels/browser.svg'),
  webhook: publicAsset('icons/channels/webhook.svg'),
  webhooks: publicAsset('icons/channels/webhook.svg'),

  // Moltbot extensions
  bluebubbles: publicAsset('icons/extensions/bluebubbles.svg'),
  googlechat: publicAsset('icons/channels/googlechat.svg'),
  'google-chat': publicAsset('icons/channels/googlechat.svg'),
  line: publicAsset('icons/extensions/line.svg'),
  mattermost: publicAsset('icons/channels/mattermost.svg'),
  nostr: publicAsset('icons/extensions/nostr.svg'),
  twitch: publicAsset('icons/extensions/twitch.svg'),
  zalo: publicAsset('icons/extensions/zalo.svg'),
  zalouser: publicAsset('icons/extensions/zalouser.svg'),
}

interface ChannelIconTuning {
  scale?: number
}

// A few logos ship with noticeably more internal whitespace than others.
// These scale nudges keep their perceived size consistent in shared icon shells.
const channelIconTuning: Record<string, ChannelIconTuning> = {
  feishu: { scale: 1.18 },
  mattermost: { scale: 1.36 },
  nextcloud: { scale: 1.24 },
  'nextcloud-talk': { scale: 1.24 },
  nextcloudtalk: { scale: 1.24 },
  teams: { scale: 1.12 },
  webhook: { scale: 1.08 },
}

/** Tunnel provider icons (remote access: ngrok, Cloudflare, localtunnel, etc.) */
export const tunnelProviderIcons: Record<string, string> = {
  auto: publicAsset('icons/tunnel/auto.svg'),
  ngrok: publicAsset('icons/tunnel/ngrok.svg'),
  cloudflare: publicAsset('icons/tunnel/cloudflare.svg'),
  bore: publicAsset('icons/tunnel/bore.svg'),
  serveo: publicAsset('icons/tunnel/serveo.svg'),
  localtunnel: publicAsset('icons/tunnel/localtunnel.svg'),
}

export function getTunnelProviderIcon(providerId: string): string | undefined {
  return tunnelProviderIcons[providerId?.toLowerCase()]
}

export function getChannelIcon(channelType: string): string | undefined {
  return channelIcons[channelType.toLowerCase()]
}

export function getChannelIconOrDefault(
  channelType: string,
  defaultIcon = publicAsset('icons/channels/default.svg')
): string {
  return channelIcons[channelType.toLowerCase()] || defaultIcon
}

export function getChannelIconStyleVars(channelType: string): Record<string, string> {
  const tuning = channelIconTuning[channelType?.toLowerCase()] ?? {}
  return {
    '--channel-icon-scale': String(tuning.scale ?? 1),
  }
}

/**
 * Get icon for a plugin based on its ID, name, or channels
 * Priority: plugin ID > plugin name > first channel > default
 */
export function getPluginIcon(plugin: {
  id?: string
  name?: string
  channels?: string[]
}): string | undefined {
  // Try plugin ID (e.g., "telegram-channel" -> "telegram")
  if (plugin.id) {
    const idKey = plugin.id.toLowerCase().replace(/-?(channel|plugin|provider)$/i, '')
    if (channelIcons[idKey]) {
      return channelIcons[idKey]
    }
  }

  // Try plugin name
  if (plugin.name) {
    const nameKey = plugin.name.toLowerCase().replace(/\s+/g, '')
    if (channelIcons[nameKey]) {
      return channelIcons[nameKey]
    }
  }

  // Try first channel
  if (plugin.channels?.length) {
    const firstChannel = plugin.channels[0]
    if (firstChannel) {
      const channelKey = firstChannel.toLowerCase()
      if (channelIcons[channelKey]) {
        return channelIcons[channelKey]
      }
    }
  }

  return undefined
}

/**
 * Get icon for a plugin with fallback to default
 */
export function getPluginIconOrDefault(
  plugin: { id?: string; name?: string; channels?: string[] },
  defaultIcon = publicAsset('icons/channels/default.svg')
): string {
  return getPluginIcon(plugin) || defaultIcon
}
