import api from './client'

// User settings stored on backend
export interface Settings {
  locale?: string   // User's preferred locale (e.g., "zh-CN", "en-US")
  timezone?: string // User's timezone
  smart_tool_selection?: boolean // IR-based tool filtering (default true)
  agent_mode?: boolean // Autonomous agent mode (default false)
  agent_auto_confirm?: boolean // Skip confirmation in agent mode (default false)
}

// Smart tool selection stats
export interface ToolSelectorStats {
  requests: number
  tools_total: number
  tools_sent: number
  tools_skipped: number
  tokens_saved: number
}

// Settings API
export const settingsApi = {
  // Get user settings
  get: () => api.get<Settings>('/settings'),

  // Update user settings (full update)
  update: (settings: Settings) => api.put<Settings>('/settings', settings),

  // Patch user settings (partial update)
  patch: (updates: Partial<Settings>) => api.patch<Settings>('/settings', updates),

  // Get smart tool selection stats
  getToolStats: () => api.get<ToolSelectorStats>('/tools/stats'),
}
