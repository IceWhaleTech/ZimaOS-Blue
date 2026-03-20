export const PagePermissions = {
  CHAT: 'page.chat',
  HOME: 'page.home',
  CHANNELS: 'page.channels',
  SETTINGS: 'page.settings',
  SECURITY: 'page.security',
  USERS: 'page.users',
  PROFILE: 'page.profile',
  PROVIDERS: 'page.providers',
  AUTOMATION: 'page.automation',
  PLUGINS: 'page.plugins',
  TOOLS: 'page.tools',
  SKILLS: 'page.skills',
} as const

export type PagePermission = (typeof PagePermissions)[keyof typeof PagePermissions]
