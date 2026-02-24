import api from './client'

// Binary source types
export type BinarySource = 'embedded' | 'downloaded' | 'system'

// Version response from GET /api/v1/claudecode/version
export interface ClaudeCodeVersionResponse {
  embedded_version?: string
  installed_version?: string
  system_version?: string
  latest_version?: string
  active_version: string
  mode: string
  source: BinarySource
  binary_path: string
  platform: string
  update_available: boolean
  validated: boolean
  last_check?: string
}

// Check update response from POST /api/v1/claudecode/version/check
export interface CheckUpdateResponse {
  current_version: string
  latest_version: string
  update_available: boolean
  release_notes?: string
  release_date?: string
  download_size?: string
}

// Update request for POST /api/v1/claudecode/version/update
export interface UpdateRequest {
  version: string // "latest" or specific version
}

// Update response from POST /api/v1/claudecode/version/update
export interface UpdateResponse {
  success: boolean
  previous_version: string
  new_version: string
  message: string
}

// Validate request for POST /api/v1/claudecode/validate
export interface ValidateRequest {
  binary_path?: string // Optional: specific path to validate
}

// Validate response from POST /api/v1/claudecode/validate
export interface ValidateResponse {
  valid: boolean
  version?: string
  binary_path: string
  message: string
  dry_run_ok: boolean
}

// Clear cache response from POST /api/v1/claudecode/cache/clear
export interface ClearCacheResponse {
  success: boolean
  message: string
}

// Directory whitelist entry
export interface DirectoryWhitelistEntry {
  path: string
  alias?: string // Optional alias for display
}

// Config response from GET /api/v1/claudecode/config
export interface ClaudeCodeConfigResponse {
  enabled: boolean
  default_model: string
  sandbox_enabled: boolean
  network_enabled: boolean
  whitelist_enabled: boolean
  directory_whitelist?: DirectoryWhitelistEntry[]
}

// Config request for PUT /api/v1/claudecode/config
export interface ClaudeCodeConfigRequest {
  enabled?: boolean
  default_model?: string
  sandbox_enabled?: boolean
  network_enabled?: boolean
  whitelist_enabled?: boolean
  directory_whitelist?: DirectoryWhitelistEntry[]
}

// Browse directories response
export interface BrowseDirEntry {
  name: string
  path: string
}

export interface BrowseDirsResponse {
  current: string
  parent?: string
  dirs: BrowseDirEntry[]
  os: string // "windows", "darwin", "linux"
}

// Claude Code CLI API
export const claudeCodeApi = {
  // Get current version information
  getVersion: () => api.get<ClaudeCodeVersionResponse>('/claudecode/version'),

  // Check for updates
  checkForUpdates: () => api.post<CheckUpdateResponse>('/claudecode/version/check'),

  // Update to specific version
  update: (version: string = 'latest') =>
    api.post<UpdateResponse>('/claudecode/version/update', { version }),

  // Validate CLI binary
  validate: (binaryPath?: string) =>
    api.post<ValidateResponse>('/claudecode/validate', { binary_path: binaryPath }),

  // Clear downloaded cache
  clearCache: () => api.post<ClearCacheResponse>('/claudecode/cache/clear'),

  // Get configuration
  getConfig: () => api.get<ClaudeCodeConfigResponse>('/claudecode/config'),

  // Update configuration
  setConfig: (config: ClaudeCodeConfigRequest) =>
    api.put<ClaudeCodeConfigResponse>('/claudecode/config', config),

  // Browse directories for whitelist picker
  browseDirs: (path?: string) =>
    api.get<BrowseDirsResponse>('/claudecode/browse-dirs', { params: path ? { path } : undefined }),
}
