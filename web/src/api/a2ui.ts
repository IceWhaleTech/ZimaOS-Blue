import api from './index'

// Component Types
export type ComponentType =
  | 'text'
  | 'button'
  | 'input'
  | 'select'
  | 'checkbox'
  | 'radio'
  | 'slider'
  | 'image'
  | 'card'
  | 'list'
  | 'table'
  | 'chart'
  | 'form'
  | 'progress'
  | 'alert'
  | 'code'
  | 'markdown'
  | 'container'
  | 'grid'
  | 'tabs'
  | 'accordion'

// Validation rules for input components
export interface Validation {
  required?: boolean
  min_length?: number
  max_length?: number
  min?: number
  max?: number
  pattern?: string
  message?: string
}

// Confirmation dialog settings
export interface ConfirmDialog {
  title: string
  message: string
  confirm_text: string
  cancel_text: string
}

// Action that can be triggered by a component
export interface Action {
  id: string
  type: string
  label?: string
  handler: string
  params?: Record<string, unknown>
  confirm?: ConfirmDialog
}

// UI Component
export interface Component {
  id: string
  type: ComponentType
  props?: Record<string, unknown>
  children?: Component[]
  actions?: Action[]
  style?: Record<string, unknown>
  validation?: Validation
}

// Canvas containing components
export interface Canvas {
  id: string
  title?: string
  description?: string
  components: Component[]
  layout?: 'vertical' | 'horizontal' | 'grid'
  metadata?: Record<string, unknown>
  created_at: string
  expires_at?: string
}

// Result of an action execution
export interface ActionResult {
  success: boolean
  data?: unknown
  error?: string
  updated_components?: Component[]
  new_canvas?: Canvas
}

// API Response types
export interface ListCanvasesResponse {
  canvases: string[]
  total: number
}

// API functions
export async function listCanvases(): Promise<ListCanvasesResponse> {
  const response = await api.get('/api/v1/a2ui/canvases')
  return response.data
}

export async function getCanvas(canvasId: string): Promise<Canvas> {
  const response = await api.get(`/api/v1/a2ui/canvases/${canvasId}`)
  return response.data
}

export async function createCanvas(canvas: Omit<Canvas, 'created_at'>): Promise<Canvas> {
  const response = await api.post('/api/v1/a2ui/canvases', canvas)
  return response.data
}

export async function updateCanvas(canvasId: string, canvas: Partial<Canvas>): Promise<Canvas> {
  const response = await api.put(`/api/v1/a2ui/canvases/${canvasId}`, canvas)
  return response.data
}

export async function deleteCanvas(canvasId: string): Promise<void> {
  await api.delete(`/api/v1/a2ui/canvases/${canvasId}`)
}

export async function executeAction(
  canvasId: string,
  actionId: string,
  formData: Record<string, unknown>
): Promise<ActionResult> {
  const response = await api.post(`/api/v1/a2ui/canvases/${canvasId}/actions/${actionId}`, {
    form_data: formData,
  })
  return response.data
}

// Helper functions
export function getComponentIcon(type: ComponentType): string {
  const icons: Record<ComponentType, string> = {
    text: 'T',
    button: 'B',
    input: 'I',
    select: 'S',
    checkbox: 'C',
    radio: 'R',
    slider: 'L',
    image: 'M',
    card: 'D',
    list: 'L',
    table: 'T',
    chart: 'H',
    form: 'F',
    progress: 'P',
    alert: 'A',
    code: '<>',
    markdown: 'MD',
    container: 'C',
    grid: 'G',
    tabs: 'TB',
    accordion: 'AC',
  }
  return icons[type] || '?'
}

export function getComponentLabel(type: ComponentType): string {
  const labels: Record<ComponentType, string> = {
    text: 'Text',
    button: 'Button',
    input: 'Input',
    select: 'Select',
    checkbox: 'Checkbox',
    radio: 'Radio',
    slider: 'Slider',
    image: 'Image',
    card: 'Card',
    list: 'List',
    table: 'Table',
    chart: 'Chart',
    form: 'Form',
    progress: 'Progress',
    alert: 'Alert',
    code: 'Code',
    markdown: 'Markdown',
    container: 'Container',
    grid: 'Grid',
    tabs: 'Tabs',
    accordion: 'Accordion',
  }
  return labels[type] || type
}
