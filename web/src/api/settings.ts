import api from './client'

// User settings stored on backend
export interface Settings {
  locale?: string   // User's preferred locale (e.g., "zh-CN", "en-US")
  timezone?: string // User's timezone
}

// Settings API
export const settingsApi = {
  // Get user settings
  get: () => api.get<Settings>('/settings'),

  // Update user settings (full update)
  update: (settings: Settings) => api.put<Settings>('/settings', settings),

  // Patch user settings (partial update)
  patch: (updates: Partial<Settings>) => api.patch<Settings>('/settings', updates),
}
