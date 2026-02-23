import api from './client'

// Types
export interface FillTemplate {
  id: string
  name: string
  is_default: boolean
  fields: Record<string, string>
  created_at: string
  updated_at: string
}

export interface FieldPatterns {
  patterns: Record<string, string[]>
  updated_at: string
}

export interface SiteMapping {
  domain: string
  field_mappings: Record<string, string>
  last_used: string
  user_corrections: number
}

export interface FieldAttributes {
  id?: string
  name?: string
  type?: string
  placeholder?: string
  autocomplete?: string
  label?: string
}

export interface DetectedField {
  selector: string
  field_type: string
  confidence: number
  detection_source: 'attribute' | 'label' | 'pattern' | 'llm'
  suggested_value?: string
  attributes: FieldAttributes
}

export interface DetectResponse {
  fields: DetectedField[]
  count: number
}

export interface FormFillerConfig {
  enabled: boolean
  widget: {
    default_position: string
    keyboard_shortcut: string
    auto_show_on_forms: boolean
  }
  detection: {
    use_llm_fallback: boolean
    confidence_threshold: number
    scan_interval_ms: number
  }
  learning: {
    enabled: boolean
    sync_enabled: boolean
    max_site_mappings: number
  }
}

export interface CreateTemplateRequest {
  name: string
  is_default?: boolean
  fields?: Record<string, string>
}

export interface UpdateTemplateRequest {
  name?: string
  is_default?: boolean
  fields?: Record<string, string>
}

// Field types for UI
export const FIELD_TYPES = [
  { value: 'firstName', label: 'First Name' },
  { value: 'lastName', label: 'Last Name' },
  { value: 'fullName', label: 'Full Name' },
  { value: 'email', label: 'Email' },
  { value: 'phone', label: 'Phone' },
  { value: 'address', label: 'Address' },
  { value: 'city', label: 'City' },
  { value: 'state', label: 'State/Province' },
  { value: 'zipCode', label: 'ZIP/Postal Code' },
  { value: 'country', label: 'Country' },
  { value: 'username', label: 'Username' },
  { value: 'birthDate', label: 'Birth Date' },
  { value: 'company', label: 'Company' },
  { value: 'title', label: 'Job Title' },
] as const

// Template API
export const templateApi = {
  list: () => api.get<FillTemplate[]>('/formfiller/templates'),

  get: (id: string) => api.get<FillTemplate>(`/formfiller/templates/${id}`),

  create: (data: CreateTemplateRequest) =>
    api.post<FillTemplate>('/formfiller/templates', data),

  update: (id: string, data: UpdateTemplateRequest) =>
    api.put<FillTemplate>(`/formfiller/templates/${id}`, data),

  delete: (id: string) => api.delete(`/formfiller/templates/${id}`),
}

// Pattern API
export const patternApi = {
  get: () => api.get<FieldPatterns>('/formfiller/patterns'),

  update: (patterns: Record<string, string[]>) =>
    api.put<FieldPatterns>('/formfiller/patterns', patterns),
}

// Site Mapping API
export const siteMappingApi = {
  get: (domain: string) => api.get<SiteMapping>(`/formfiller/sites/${domain}`),

  save: (domain: string, mapping: Partial<SiteMapping>) =>
    api.put<SiteMapping>(`/formfiller/sites/${domain}`, mapping),
}

// Detection API
export const detectApi = {
  detect: (fields: FieldAttributes[], templateId?: string, fillData?: string) =>
    api.post<DetectResponse>('/formfiller/detect', {
      fields,
      template_id: templateId,
      fill_data: fillData,
    }),
}

// Config API
export const configApi = {
  get: () => api.get<FormFillerConfig>('/formfiller/config'),
}
