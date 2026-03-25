import api from './index'
import type { AxiosError } from 'axios'

// Returns true if the error is a 404 (feature not enabled on backend)
function isNotFound(error: unknown): boolean {
  return (error as AxiosError)?.response?.status === 404
}

// Types
export interface BrowserTask {
  id: string
  name: string
  description?: string
  status: 'pending' | 'running' | 'completed' | 'failed' | 'cancelled'
  steps: TaskStep[]
  created_at: string
  started_at?: string
  completed_at?: string
  error?: string
  result?: TaskResult
}

export interface TaskStep {
  id: string
  type: StepType
  params: Record<string, unknown>
  status: 'pending' | 'running' | 'completed' | 'failed' | 'skipped'
  result?: StepResult
  error?: string
}

export type StepType =
  | 'navigate'
  | 'click'
  | 'type'
  | 'screenshot'
  | 'wait'
  | 'extract'
  | 'scroll'
  | 'select'
  | 'hover'
  | 'press_key'
  | 'evaluate'

export interface StepResult {
  screenshot?: string
  extracted_data?: unknown
  element_found?: boolean
  page_title?: string
  page_url?: string
}

export interface TaskResult {
  screenshots: string[]
  extracted_data: Record<string, unknown>
  final_url: string
  duration_ms: number
}

export interface BrowserSession {
  id: string
  status: 'active' | 'idle' | 'closed'
  current_url?: string
  page_title?: string
  created_at: string
  last_activity: string
  engine: string
  engine_detail?: string
  session_layer?: string
  monitor_kind: 'text' | 'image'
}

export interface BrowserSessionScreenshot {
  data: string
  url?: string
  title?: string
  captured_at: string
  scope?: string
}

export interface BrowserSessionScreenshotResponse {
  screenshot: string
  history: BrowserSessionScreenshot[]
  error?: string
}

export interface BrowserSessionTextMonitor {
  title?: string
  url?: string
  summary?: string
  tree_preview?: string
  interactive_count?: number
  updated_at?: string
  status?: string
}

export interface BrowserSessionImageMonitor {
  screenshot?: string
  history?: BrowserSessionScreenshot[]
  updated_at?: string
  status?: string
}

export interface BrowserSessionMonitorResponse {
  kind: 'text' | 'image'
  image?: BrowserSessionImageMonitor
  text?: BrowserSessionTextMonitor
  error?: string
}

export interface CreateTaskRequest {
  name: string
  description?: string
  steps: Omit<TaskStep, 'id' | 'status' | 'result' | 'error'>[]
}

export interface TaskTemplate {
  id: string
  name: string
  description: string
  category: string
  steps: Omit<TaskStep, 'id' | 'status' | 'result' | 'error'>[]
}

function normalizeSession(payload: unknown): BrowserSession {
  const raw = payload && typeof payload === 'object' ? (payload as Record<string, unknown>) : {}
  const monitorKind = raw.monitor_kind === 'text' ? 'text' : 'image'
  const engine =
    typeof raw.engine === 'string' && raw.engine.trim()
      ? raw.engine
      : monitorKind === 'text'
        ? 'lightpanda'
        : 'chromium_managed'
  const engineDetail =
    typeof raw.engine_detail === 'string' && raw.engine_detail.trim() ? raw.engine_detail : ''
  const sessionLayer =
    typeof raw.session_layer === 'string' && raw.session_layer.trim()
      ? raw.session_layer
      : monitorKind === 'text' || engine === 'lightpanda'
        ? 'read'
        : engineDetail === 'lightpanda_binary'
          ? 'browser_lite'
          : 'full_browser'

  return {
    id: typeof raw.id === 'string' ? raw.id : '',
    status: raw.status === 'closed' ? 'closed' : raw.status === 'idle' ? 'idle' : 'active',
    current_url: typeof raw.current_url === 'string' ? raw.current_url : '',
    page_title: typeof raw.page_title === 'string' ? raw.page_title : '',
    created_at: typeof raw.created_at === 'string' ? raw.created_at : '',
    last_activity: typeof raw.last_activity === 'string' ? raw.last_activity : '',
    engine,
    engine_detail: engineDetail,
    session_layer: sessionLayer,
    monitor_kind: monitorKind,
  }
}

// API functions
// Note: Browser routes are registered under /api/browser/* (without /v1)
export async function getTasks(): Promise<BrowserTask[]> {
  try {
    const response = await api.get('/browser/tasks', { baseURL: '/api' })
    return response.data
  } catch (e) {
    if (isNotFound(e)) return []
    throw e
  }
}

export async function getTask(taskId: string): Promise<BrowserTask> {
  const response = await api.get(`/browser/tasks/${taskId}`, { baseURL: '/api' })
  return response.data
}

export async function createTask(request: CreateTaskRequest): Promise<BrowserTask> {
  const response = await api.post('/browser/tasks', request, { baseURL: '/api' })
  return response.data
}

export async function runTask(taskId: string): Promise<void> {
  await api.post(`/browser/tasks/${taskId}/run`, null, { baseURL: '/api' })
}

export async function cancelTask(taskId: string): Promise<void> {
  await api.post(`/browser/tasks/${taskId}/cancel`, null, { baseURL: '/api' })
}

export async function deleteTask(taskId: string): Promise<void> {
  await api.delete(`/browser/tasks/${taskId}`, { baseURL: '/api' })
}

export async function getSessions(): Promise<BrowserSession[]> {
  try {
    const response = await api.get('/browser/sessions', { baseURL: '/api' })
    return Array.isArray(response.data) ? response.data.map(normalizeSession) : []
  } catch (e) {
    if (isNotFound(e)) return []
    throw e
  }
}

export async function getSession(sessionId: string): Promise<BrowserSession> {
  const response = await api.get(`/browser/sessions/${sessionId}`, { baseURL: '/api' })
  return normalizeSession(response.data)
}

export async function createSession(): Promise<BrowserSession> {
  const response = await api.post('/browser/sessions', null, { baseURL: '/api' })
  return normalizeSession(response.data)
}

export async function closeSession(sessionId: string): Promise<void> {
  await api.delete(`/browser/sessions/${sessionId}`, { baseURL: '/api' })
}

export async function takeScreenshot(sessionId: string): Promise<BrowserSessionScreenshotResponse> {
  const response = await api.post(`/browser/sessions/${sessionId}/screenshot`, null, {
    baseURL: '/api',
  })
  const payload = response.data || {}
  return {
    screenshot: typeof payload.screenshot === 'string' ? payload.screenshot : '',
    history: Array.isArray(payload.history)
      ? payload.history.filter((item: unknown): item is BrowserSessionScreenshot => !!item)
      : [],
    error: typeof payload.error === 'string' ? payload.error : '',
  }
}

export async function getSessionMonitor(sessionId: string): Promise<BrowserSessionMonitorResponse> {
  const response = await api.post(`/browser/sessions/${sessionId}/monitor`, null, {
    baseURL: '/api',
  })
  const payload = response.data || {}
  return {
    kind: payload.kind === 'text' ? 'text' : 'image',
    image:
      payload.image && typeof payload.image === 'object'
        ? {
            screenshot:
              typeof payload.image.screenshot === 'string' ? payload.image.screenshot : '',
            history: Array.isArray(payload.image.history)
              ? payload.image.history.filter(
                  (item: unknown): item is BrowserSessionScreenshot => !!item
                )
              : [],
            updated_at:
              typeof payload.image.updated_at === 'string' ? payload.image.updated_at : '',
            status: typeof payload.image.status === 'string' ? payload.image.status : '',
          }
        : undefined,
    text:
      payload.text && typeof payload.text === 'object'
        ? {
            title: typeof payload.text.title === 'string' ? payload.text.title : '',
            url: typeof payload.text.url === 'string' ? payload.text.url : '',
            summary: typeof payload.text.summary === 'string' ? payload.text.summary : '',
            tree_preview:
              typeof payload.text.tree_preview === 'string' ? payload.text.tree_preview : '',
            interactive_count:
              typeof payload.text.interactive_count === 'number'
                ? payload.text.interactive_count
                : 0,
            updated_at: typeof payload.text.updated_at === 'string' ? payload.text.updated_at : '',
            status: typeof payload.text.status === 'string' ? payload.text.status : '',
          }
        : undefined,
    error: typeof payload.error === 'string' ? payload.error : '',
  }
}

export async function navigateTo(sessionId: string, url: string): Promise<void> {
  await api.post(`/browser/sessions/${sessionId}/navigate`, { url }, { baseURL: '/api' })
}

export async function executeStep(
  sessionId: string,
  step: Omit<TaskStep, 'id' | 'status'>
): Promise<StepResult> {
  const response = await api.post(`/browser/sessions/${sessionId}/execute`, step, {
    baseURL: '/api',
  })
  return response.data
}

// Task templates
export const taskTemplates: TaskTemplate[] = [
  {
    id: 'web-scrape',
    name: 'Web Scraping',
    description: 'Extract data from a webpage',
    category: 'Data',
    steps: [
      { type: 'navigate', params: { url: '' } },
      { type: 'wait', params: { selector: 'body', timeout: 5000 } },
      { type: 'extract', params: { selector: '', attribute: 'text' } },
      { type: 'screenshot', params: { fullPage: false } },
    ],
  },
  {
    id: 'form-fill',
    name: 'Form Filling',
    description: 'Automatically fill out a web form',
    category: 'Automation',
    steps: [
      { type: 'navigate', params: { url: '' } },
      { type: 'wait', params: { selector: 'form', timeout: 5000 } },
      { type: 'type', params: { selector: '', text: '' } },
      { type: 'click', params: { selector: 'button[type="submit"]' } },
      { type: 'screenshot', params: { fullPage: false } },
    ],
  },
  {
    id: 'page-monitor',
    name: 'Page Monitor',
    description: 'Take screenshots of a page for monitoring',
    category: 'Monitoring',
    steps: [
      { type: 'navigate', params: { url: '' } },
      { type: 'wait', params: { timeout: 3000 } },
      { type: 'screenshot', params: { fullPage: true } },
    ],
  },
  {
    id: 'login-test',
    name: 'Login Test',
    description: 'Test a login flow',
    category: 'Testing',
    steps: [
      { type: 'navigate', params: { url: '' } },
      { type: 'type', params: { selector: 'input[name="username"]', text: '' } },
      { type: 'type', params: { selector: 'input[name="password"]', text: '' } },
      { type: 'click', params: { selector: 'button[type="submit"]' } },
      { type: 'wait', params: { timeout: 3000 } },
      { type: 'screenshot', params: { fullPage: false } },
    ],
  },
]

// Helper functions
export function getStepIcon(type: StepType): string {
  const icons: Record<StepType, string> = {
    navigate: '🌐',
    click: '👆',
    type: '⌨️',
    screenshot: '📷',
    wait: '⏳',
    extract: '📋',
    scroll: '📜',
    select: '📝',
    hover: '🖱️',
    press_key: '⌨️',
    evaluate: '🔧',
  }
  return icons[type] || '❓'
}

export function getStepLabel(type: StepType): string {
  const labels: Record<StepType, string> = {
    navigate: 'Navigate to URL',
    click: 'Click Element',
    type: 'Type Text',
    screenshot: 'Take Screenshot',
    wait: 'Wait',
    extract: 'Extract Data',
    scroll: 'Scroll Page',
    select: 'Select Option',
    hover: 'Hover Element',
    press_key: 'Press Key',
    evaluate: 'Run JavaScript',
  }
  return labels[type] || type
}

export function getStatusColor(status: string): string {
  const colors: Record<string, string> = {
    pending: '#6b7280',
    running: '#3b82f6',
    completed: '#22c55e',
    failed: '#ef4444',
    cancelled: '#f59e0b',
    skipped: '#9ca3af',
    active: '#22c55e',
    idle: '#f59e0b',
    closed: '#6b7280',
  }
  return colors[status] || '#6b7280'
}

export function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  return `${(ms / 60000).toFixed(1)}m`
}

// Security configuration types
export interface BrowserSecurityConfig {
  allowed_domains: string[]
  blocked_domains: string[]
}

// Security API functions
// Note: Browser security routes are also under /api/browser/* (without /v1)
export async function getSecurityConfig(): Promise<BrowserSecurityConfig> {
  try {
    const response = await api.get('/browser/security', { baseURL: '/api' })
    return response.data
  } catch (e) {
    if (isNotFound(e)) return { allowed_domains: [], blocked_domains: [] }
    throw e
  }
}

export async function updateSecurityConfig(config: BrowserSecurityConfig): Promise<void> {
  await api.put('/browser/security', config, { baseURL: '/api' })
}

export async function addAllowedDomain(domain: string): Promise<void> {
  await api.post('/browser/security/allowed', { domain }, { baseURL: '/api' })
}

export async function removeAllowedDomain(domain: string): Promise<void> {
  await api.delete(`/browser/security/allowed/${encodeURIComponent(domain)}`, { baseURL: '/api' })
}

export async function addBlockedDomain(domain: string): Promise<void> {
  await api.post('/browser/security/blocked', { domain }, { baseURL: '/api' })
}

export async function removeBlockedDomain(domain: string): Promise<void> {
  await api.delete(`/browser/security/blocked/${encodeURIComponent(domain)}`, { baseURL: '/api' })
}

export async function testUrl(url: string): Promise<{ allowed: boolean; reason?: string }> {
  const response = await api.post('/browser/security/test', { url }, { baseURL: '/api' })
  return response.data
}
