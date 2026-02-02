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
  // LLM providers
  openai: '/icons/channels/openai.svg',
  anthropic: '/icons/channels/anthropic.svg',
  google: '/icons/channels/google.svg',
  ollama: '/icons/channels/ollama.svg',
  mistral: '/icons/channels/mistral.svg',
  deepseek: '/icons/channels/deepseek.svg',
  huggingface: '/icons/channels/huggingface.svg',
  'hugging-face': '/icons/channels/huggingface.svg',
  // Smart home & IoT
  homeassistant: '/icons/channels/homeassistant.svg',
  'home-assistant': '/icons/channels/homeassistant.svg',
  philipshue: '/icons/channels/philipshue.svg',
  'philips-hue': '/icons/channels/philipshue.svg',
  hue: '/icons/channels/philipshue.svg',
  sonos: '/icons/channels/sonos.svg',
  // Productivity
  notion: '/icons/channels/notion.svg',
  obsidian: '/icons/channels/obsidian.svg',
  trello: '/icons/channels/trello.svg',
  github: '/icons/channels/github.svg',
  gmail: '/icons/channels/gmail.svg',
  // Media & Entertainment
  spotify: '/icons/channels/spotify.svg',
  shazam: '/icons/channels/shazam.svg',
  // Social
  twitter: '/icons/channels/twitter.svg',
  x: '/icons/channels/twitter.svg',
  instagram: '/icons/channels/instagram.svg',
  messenger: '/icons/channels/messenger.svg',
  'facebook-messenger': '/icons/channels/messenger.svg',
  viber: '/icons/channels/viber.svg',
  // Tools & Utilities
  '1password': '/icons/channels/1password.svg',
  onepassword: '/icons/channels/1password.svg',
  browser: '/icons/channels/browser.svg',
  webhook: '/icons/channels/webhook.svg',
  webhooks: '/icons/channels/webhook.svg',
  apple: '/icons/channels/apple.svg',

  // Moltbot extensions
  bluebubbles: '/icons/extensions/bluebubbles.svg',
  'copilot-proxy': '/icons/extensions/copilot-proxy.svg',
  copilotproxy: '/icons/extensions/copilot-proxy.svg',
  'diagnostics-otel': '/icons/extensions/diagnostics-otel.svg',
  diagnosticsotel: '/icons/extensions/diagnostics-otel.svg',
  opentelemetry: '/icons/extensions/diagnostics-otel.svg',
  'google-antigravity-auth': '/icons/extensions/google-antigravity-auth.svg',
  'google-gemini-cli-auth': '/icons/extensions/google-gemini-cli-auth.svg',
  gemini: '/icons/extensions/google-gemini-cli-auth.svg',
  googlechat: '/icons/channels/googlechat.svg',
  'google-chat': '/icons/channels/googlechat.svg',
  line: '/icons/extensions/line.svg',
  'llm-task': '/icons/extensions/llm-task.svg',
  llmtask: '/icons/extensions/llm-task.svg',
  lobster: '/icons/extensions/lobster.svg',
  mattermost: '/icons/channels/mattermost.svg',
  'memory-core': '/icons/extensions/memory-core.svg',
  memorycore: '/icons/extensions/memory-core.svg',
  'memory-lancedb': '/icons/extensions/memory-lancedb.svg',
  memorylancedb: '/icons/extensions/memory-lancedb.svg',
  lancedb: '/icons/extensions/memory-lancedb.svg',
  nostr: '/icons/extensions/nostr.svg',
  'open-prose': '/icons/extensions/open-prose.svg',
  openprose: '/icons/extensions/open-prose.svg',
  'qwen-portal-auth': '/icons/extensions/qwen-portal-auth.svg',
  qwen: '/icons/extensions/qwen-portal-auth.svg',
  tlon: '/icons/extensions/tlon.svg',
  twitch: '/icons/extensions/twitch.svg',
  'voice-call': '/icons/extensions/voice-call.svg',
  voicecall: '/icons/extensions/voice-call.svg',
  zalo: '/icons/extensions/zalo.svg',
  zalouser: '/icons/extensions/zalouser.svg',
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

export function getChannelIconOrDefault(channelType: string, defaultIcon = '/icons/channels/default.svg'): string {
  return channelIcons[channelType.toLowerCase()] || defaultIcon
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
