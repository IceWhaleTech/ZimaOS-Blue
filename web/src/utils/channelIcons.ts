// Icon mapping for channels and plugins
// Maps channel/plugin type/id to their respective icon paths

export const channelIcons: Record<string, string> = {
  // Messaging channels
  telegram: '/icons/channels/telegram.svg',
  discord: '/icons/channels/discord.svg',
  slack: '/icons/channels/slack.svg',
  whatsapp: '/icons/channels/whatsapp.svg',
  wechat: '/icons/channels/wechat.svg',
  signal: '/icons/channels/signal.svg',
  matrix: '/icons/channels/matrix.svg',
  imessage: '/icons/channels/imessage.svg',
  feishu: '/icons/channels/feishu.svg',
  dingtalk: '/icons/channels/dingtalk.svg',
  qq: '/icons/channels/qq.svg',
  teams: '/icons/channels/teams.svg',
  'microsoft-teams': '/icons/channels/teams.svg',
  msteams: '/icons/channels/teams.svg',
  nextcloud: '/icons/channels/nextcloud.svg',
  'nextcloud-talk': '/icons/channels/nextcloud.svg',
  nextcloudtalk: '/icons/channels/nextcloud.svg',
  // Social & messaging
  twitter: '/icons/channels/twitter.svg',
  x: '/icons/channels/twitter.svg',
  instagram: '/icons/channels/instagram.svg',
  messenger: '/icons/channels/messenger.svg',
  'facebook-messenger': '/icons/channels/messenger.svg',
  viber: '/icons/channels/viber.svg',
  // Tools & Utilities
  browser: '/icons/channels/browser.svg',
  webhook: '/icons/channels/webhook.svg',
  webhooks: '/icons/channels/webhook.svg',

  // Moltbot extensions
  bluebubbles: '/icons/extensions/bluebubbles.svg',
  googlechat: '/icons/channels/googlechat.svg',
  'google-chat': '/icons/channels/googlechat.svg',
  line: '/icons/extensions/line.svg',
  mattermost: '/icons/channels/mattermost.svg',
  nostr: '/icons/extensions/nostr.svg',
  twitch: '/icons/extensions/twitch.svg',
  zalo: '/icons/extensions/zalo.svg',
  zalouser: '/icons/extensions/zalouser.svg',
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
  auto: '/icons/tunnel/auto.svg',
  ngrok: '/icons/tunnel/ngrok.svg',
  cloudflare: '/icons/tunnel/cloudflare.svg',
  bore: '/icons/tunnel/bore.svg',
  serveo: '/icons/tunnel/serveo.svg',
  localtunnel: '/icons/tunnel/localtunnel.svg',
}

export function getTunnelProviderIcon(providerId: string): string | undefined {
  return tunnelProviderIcons[providerId?.toLowerCase()]
}

export function getChannelIcon(channelType: string): string | undefined {
  return channelIcons[channelType.toLowerCase()]
}

export function getChannelIconOrDefault(
  channelType: string,
  defaultIcon = '/icons/channels/default.svg'
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
  defaultIcon = '/icons/channels/default.svg'
): string {
  return getPluginIcon(plugin) || defaultIcon
}
