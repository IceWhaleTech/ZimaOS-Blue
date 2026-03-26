import api from './client'

export interface SkillParameter {
  name: string
  type: string
  description?: string
  required?: boolean
  default?: unknown
}

export interface Skill {
  id: string
  name: string
  version: string
  description: string
  author?: string
  category?: string
  icon?: string
  tags?: string[]
  enabled: boolean
  builtin: boolean
  inputs?: SkillParameter[]
  outputs?: SkillParameter[]
}

export interface SkillSource {
  id: string
  name: string
  url: string
  type: 'clawdhub' | 'github' | 'custom'
  description?: string
  enabled: boolean
}

export type SecurityBadge = 'green' | 'yellow' | 'red'
export type InstallType =
  | 'builtin_commands'
  | 'raw_skill'
  | 'git_repo'
  | 'source_archive'
  | 'script_package'
  | 'binary_package'
  | 'manual_external'
export type ArtifactKind = 'open_source' | 'closed_binary' | 'mixed' | 'unknown'
export type VulnerabilityStatus = 'none' | 'unknown' | 'suspected' | 'detected' | 'not_applicable'

export interface SkillSecurityEvidence {
  type: string
  severity: string
  title: string
  description?: string
  value?: string
}

export interface SkillInstallSurface {
  install_type: InstallType | string
  artifact_kind: ArtifactKind | string
  installable: boolean
  has_binary: boolean
  has_scripts: boolean
  dependency_manifests?: string[]
}

export interface SkillSecurityReport {
  id?: string
  skill_id: string
  version: string
  score: number
  risk_level: string
  security_badge: SecurityBadge | string
  vulnerability_status: VulnerabilityStatus | string
  risks?: Array<{
    type: string
    severity: string
    pattern?: string
    message: string
    command?: string
    permission?: string
  }>
  permissions: string[]
  secrets?: string[]
  vulnerabilities?: string[]
  evidence?: SkillSecurityEvidence[]
  install_surface: SkillInstallSurface
  has_vulnerabilities: boolean
  has_prompt_injection: boolean
  has_shell_injection: boolean
  has_data_exfiltration: boolean
  has_binary?: boolean
  scanner_version?: string
  llm_status?: string
  llm_verdict_json?: string
}

export interface RemoteSkill {
  id: string
  name: string
  version?: string
  latest_version?: string
  description?: string
  summary?: string
  author?: string
  category?: string
  tags?: string[] | string
  source_id?: string
  source_name?: string
  source_group?: string
  download_url?: string
  homepage?: string
  source_url?: string
  stars?: number
  downloads?: number
  reviews?: number
  rating?: number
  versions?: number
  changelog?: string
  readme?: string
  dedup_key?: string
  installed?: boolean
  enabled?: boolean
  builtin?: boolean
  created_at?: string
  updated_at?: string
  synced_at?: string
  last_updated?: string
  security_score?: number
  risk_level?: string
  security_badge?: SecurityBadge | string
  installable?: boolean
  install_type?: InstallType | string
  artifact_kind?: ArtifactKind | string
  vulnerability_status?: VulnerabilityStatus | string
  has_vulnerabilities?: boolean
  has_prompt_injection?: boolean
  has_shell_injection?: boolean
  has_data_exfiltration?: boolean
  has_binary?: boolean
  has_scripts?: boolean
  curated_rank?: number
  curated_boost?: number
  curated_label?: string
  curated_reason?: string
  trending_score?: number
}

export interface LocalSkill {
  id: string
  name: string
  description: string
  version?: string
  author?: string
  category?: string
  tags?: string[]
  file_path: string
  discovered_at: string
  last_modified: string
  installed: boolean
  builtin?: boolean
}

export interface BrowseParams {
  source?: string
  category?: string
  search?: string
  page?: number
  page_size?: number
}

export interface SearchParams {
  q?: string
  semantic?: boolean
  category?: string
  categories?: string
  sort?: string
  sources?: string
  min_stars?: number
  sort_by?: 'relevance' | 'stars' | 'downloads' | 'updated' | 'name'
  sort_order?: 'asc' | 'desc'
  page?: number
  page_size?: number
  cursor?: string
  count?: number
}

export interface MarketSearchParams {
  q?: string
  semantic?: boolean
  category?: string
  categories?: string
  sources?: string
  sort?: string
  page?: number
  page_size?: number
  risk_badges?: string
  install_types?: string
  artifact_kinds?: string
  installable?: boolean
  curated?: boolean
  open_source_only?: boolean
  has_vulnerabilities?: boolean
  has_prompt_injection?: boolean
  has_shell_injection?: boolean
  has_data_exfiltration?: boolean
}

export interface SearchResult {
  id: string
  name: string
  version: string
  summary: string
  description: string
  author: string
  category: string
  tags: string
  source_id: string
  source_name: string
  homepage: string
  download_url: string
  stars: number
  downloads: number
  reviews: number
  rating: number
  versions: number
  changelog: string
  installed: boolean
  enabled: boolean
  created_at: string
  updated_at: string
  synced_at: string
  score: number
}

export interface SearchResponse {
  skills: SearchResult[]
  total: number
  page: number
  page_size: number
  total_pages: number
  next_cursor?: string
  has_more: boolean
  initializing?: boolean
}

export interface MarketSearchResult {
  skill: RemoteSkill
  score: number
  keyword_score?: number
  semantic_score?: number
  match_source?: string
}

export interface MarketSearchResponse {
  skills: MarketSearchResult[]
  total: number
  page: number
  page_size: number
  total_pages: number
}

export interface FilterOption {
  value: string
  label: string
  count: number
}

export interface SkillFiltersResponse {
  categories: FilterOption[]
  sources: FilterOption[]
  risk_badges: FilterOption[]
  install_types: FilterOption[]
  artifact_kinds: FilterOption[]
  installable: Record<string, number>
  security_signals: Record<string, number>
}

export interface FeaturedSkillsResponse {
  skills: RemoteSkill[]
  count: number
}

export interface SyncProgress {
  current_page: number
  skills_synced: number
  started_at: string
}

export interface SyncStatus {
  id: number
  source_id: string
  last_sync_at: string
  skill_count: number
  sync_duration_ms: number
  status: 'success' | 'failed' | 'in_progress' | 'pending'
  error_message?: string
  next_sync_at: string
  progress?: SyncProgress
}

export interface BrowseResponse {
  skills: RemoteSkill[]
  total: number
  page: number
  page_size: number
  total_pages: number
}

export interface VerifyResponse {
  id: string
  visible: boolean
  enabled?: boolean
  builtin?: boolean
  name?: string
  version?: string
  description?: string
  error?: string
}

export interface LocalSkillsResponse {
  skills: LocalSkill[]
  count: number
}

export interface InstallFromURLRequest {
  url: string
  name?: string
  description?: string
}

export interface SkillContentResponse {
  id: string
  name: string
  content: string
  source: 'database' | 'builtin' | 'none' | 'directory'
}

export interface MarketplaceSkillDetail {
  skill: RemoteSkill
  version?: {
    version: string
    commit_hash?: string
    source_url?: string
    checksum?: string
    skill_path?: string
    released_at?: string
  }
  security?: SkillSecurityReport
  installed: boolean
  enabled: boolean
}

export interface InstalledMarketplaceSkill {
  skill_id: string
  name?: string
  installed_version: string
  checksum?: string
  source_url?: string
  enabled: boolean
  auto_update: boolean
  installed_at?: string
  updated_at?: string
  last_security_score: number
  latest_version?: string
  update_available?: boolean
}

export interface InstalledMarketplaceSkillsResponse {
  skills: InstalledMarketplaceSkill[]
  count: number
}

export interface MarketplaceInstallRequest {
  id?: string
  version?: string
  github?: string
  ack_risk?: boolean
}

export interface MarketplaceInstallResult {
  skill_id: string
  version: string
  path: string
  cache_path?: string
  warnings?: string[]
  security: SkillSecurityReport
  installed_at?: string
}

export interface MarketplaceUpdateInfo {
  skill_id: string
  current_version: string
  latest_version: string
  current_checksum?: string
  latest_checksum?: string
  action: string
  checked_at?: string
}

export interface MarketplaceUpdatesResponse {
  updates: MarketplaceUpdateInfo[]
  count: number
}

export interface InstalledSkillDecision {
  query: string
  selected_skill: string
  confidence: number
  need_clarify: boolean
  reason: string
  stage: string
  candidates?: Array<{
    name: string
    score: number
    description?: string
  }>
  matched_signals?: string[]
  conflict_flags?: string[]
  confidence_reason?: string
}

export interface MarketplaceAdviceRequest {
  query: string
  installed_decision?: InstalledSkillDecision
}

export interface MarketplaceAdviceResponse {
  query: string
  installed_decision?: InstalledSkillDecision
  need_store_search: boolean
  reason?: string
  search_queries?: string[]
  capability_tags?: string[]
  results?: MarketSearchResult[]
  recommended_ids?: string[]
  install_mode?: string
  skill_selector_error?: string
  search_error?: string
}

export interface DiscoverResponse {
  sources_processed?: number
  discovered?: number
  updated?: number
  failed?: number
}

export interface DiscoverStatusResponse {
  accepted?: boolean
  running: boolean
  started_at?: string
  finished_at?: string
  last_error?: string
  total_sources?: number
  processed_sources?: number
  current_source_id?: string
  current_source_name?: string
  message?: string
  result?: DiscoverResponse
  batch_inserted?: number
  batch_updated?: number
  batch_failed?: number
  phase?: 'started' | 'batch' | 'source_complete' | 'completed' | 'error'
}

export interface EmbeddingStatusResponse {
  running: boolean
  started_at?: string
  finished_at?: string
  last_error?: string
  total_skills?: number
  processed_skills?: number
  embedded_skills?: number
  failed_skills?: number
  current_skill_id?: string
  current_skill_name?: string
  phase?: 'started' | 'progress' | 'completed' | 'error'
}

export const skillApi = {
  list: () => api.get<Skill[]>('/skills'),
  get: (id: string) => api.get<Skill>(`/skills/${id}`),
  getContent: (id: string) => api.get<SkillContentResponse>(`/skills/${id}/content`),
  enable: (id: string) => api.post<{ success: boolean; message: string }>(`/skills/${id}/enable`),
  disable: (id: string) => api.post<{ success: boolean; message: string }>(`/skills/${id}/disable`),

  listLocal: () => api.get<LocalSkillsResponse>('/skills/local'),
  scanLocal: () => api.post<{ success: boolean; skills_found: number }>('/skills/local/scan'),
  verify: (id: string) => api.get<VerifyResponse>(`/skills/verify/${id}`),

  searchMarket: (params?: MarketSearchParams) =>
    api.get<MarketSearchResponse>('/skills/search', { params }),
  adviseMarket: (req: MarketplaceAdviceRequest) =>
    api.post<MarketplaceAdviceResponse>('/skills/advise', req),
  featuredMarket: (params?: { category?: string; source?: string; limit?: number }) =>
    api.get<FeaturedSkillsResponse>('/skills/featured', { params }),
  filtersMarket: () => api.get<SkillFiltersResponse>('/skills/filters'),
  getMarketplaceSkill: (id: string) => api.get<MarketplaceSkillDetail>(`/skills/${id}`),
  getMarketplaceSecurity: (id: string, version?: string) =>
    api.get<SkillSecurityReport>(`/skills/security/${id}`, {
      params: version ? { version } : undefined,
    }),
  installMarket: (req: MarketplaceInstallRequest) =>
    api.post<MarketplaceInstallResult>('/skills/install', req),
  listInstalledMarket: () => api.get<InstalledMarketplaceSkillsResponse>('/skills/installed'),
  uninstallMarket: (id: string) =>
    api.post<{ success: boolean; skill_id: string }>(`/skills/${id}/uninstall`),
  updateMarket: (id: string, ackRisk?: boolean) =>
    api.post<MarketplaceInstallResult>(`/skills/${id}/update`, null, {
      params: ackRisk ? { ack_risk: true } : undefined,
    }),
  discoverRefresh: () => api.post<DiscoverStatusResponse>('/skills/discover/refresh'),
  discoverStatus: () => api.get<DiscoverStatusResponse>('/skills/discover/status'),
  embeddingStatus: () => api.get<EmbeddingStatusResponse>('/skills/embedding/status'),
  listMarketUpdates: () => api.get<MarketplaceUpdatesResponse>('/skills/updates'),

  listSources: () => api.get<SkillSource[]>('/skill-store/sources'),
  addSource: (source: Omit<SkillSource, 'enabled'> & { enabled?: boolean }) =>
    api.post<{ success: boolean; message: string }>('/skill-store/sources', source),
  removeSource: (id: string) =>
    api.delete<{ success: boolean; message: string }>(`/skill-store/sources/${id}`),
  browse: (params?: BrowseParams) => api.get<BrowseResponse>('/skill-store/browse', { params }),
  featured: (category?: string) =>
    api.get<RemoteSkill[]>('/skill-store/featured', {
      params: category ? { category } : undefined,
    }),
  install: (id: string) =>
    api.post<{ success: boolean; message: string; skill?: RemoteSkill }>(
      `/skill-store/install/${id}`
    ),
  installFromURL: (req: InstallFromURLRequest) =>
    api.post<{
      success: boolean
      skill?: { id: string; name: string; version: string; description: string }
    }>('/skill-store/install-url', req),
  uninstall: (id: string) =>
    api.post<{ success: boolean; message: string }>(`/skill-store/uninstall/${id}`),
  refresh: () =>
    api.post<{
      success: boolean
      skills_count?: number
      message?: string
      syncing?: boolean
      sync_status?: SyncStatus[]
    }>('/skill-store/refresh'),
  sync: (sourceId?: string) =>
    api.post<{ success: boolean; message: string }>('/skill-store/sync', null, {
      params: sourceId ? { source: sourceId } : undefined,
    }),
  upload: (file: File) => {
    const formData = new FormData()
    formData.append('file', file)
    return api.post<{
      success: boolean
      message?: string
      skill?: { id: string; name: string; version: string }
    }>('/skills/upload', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
  },
  categories: () => api.get<string[]>('/skill-store/categories'),
  stats: () =>
    api.get<{ total_skills: number; installed: number; by_source: Record<string, number> }>(
      '/skill-store/stats'
    ),
  search: (params?: SearchParams) => api.get<SearchResponse>('/skill-store/search', { params }),
  popular: (limit?: number) =>
    api.get<RemoteSkill[]>('/skill-store/popular', { params: limit ? { limit } : undefined }),
  recent: (limit?: number) =>
    api.get<RemoteSkill[]>('/skill-store/recent', { params: limit ? { limit } : undefined }),
  syncStatus: () => api.get<SyncStatus[]>('/skill-store/sync-status'),
}
