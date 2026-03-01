import type { AxiosResponse } from 'axios'
import api from './client'

export type BillingGroupBy = 'day' | 'provider' | 'model'

export interface BillingTotals {
  input_tokens: number
  output_tokens: number
  cache_read_tokens: number
  cache_write_tokens: number
  total_tokens: number
  request_count: number
  success_count: number
  failure_count: number
  estimated_cost: number
}

export interface BillingSummaryBreakdown extends BillingTotals {
  key: string
  provider_id?: string
  model_id?: string
  day?: string
}

export interface BillingSummaryResponse {
  from: string
  to: string
  currency: string
  group_by: BillingGroupBy
  totals: BillingTotals
  breakdown: BillingSummaryBreakdown[]
}

export interface BillingLineItem {
  timestamp: string
  provider_id: string
  model_id: string
  input_tokens: number
  output_tokens: number
  cache_read_tokens: number
  cache_write_tokens: number
  total_tokens: number
  estimated_cost: number
  request_count: number
  success: boolean
  latency_ms: number
  user_id?: string
  session_id?: string
}

export interface BillingLinesResponse {
  from: string
  to: string
  page: number
  page_size: number
  total: number
  items: BillingLineItem[]
}

export interface BillingQuery {
  from?: string
  to?: string
  provider_id?: string
  model_id?: string
}

export interface BillingSummaryQuery extends BillingQuery {
  group_by?: BillingGroupBy
}

export interface BillingLinesQuery extends BillingQuery {
  page?: number
  page_size?: number
}

function compactQuery(params?: object) {
  if (!params) return undefined
  const compacted: Record<string, string | number> = {}
  Object.entries(params as Record<string, unknown>).forEach(([key, value]) => {
    if ((typeof value === 'string' || typeof value === 'number') && value !== '') {
      compacted[key] = value
    }
  })
  return compacted
}

export const billingApi = {
  getSummary: (params?: BillingSummaryQuery) =>
    api.get<BillingSummaryResponse>('/billing/summary', { params: compactQuery(params) }),

  getLines: (params?: BillingLinesQuery) =>
    api.get<BillingLinesResponse>('/billing/lines', { params: compactQuery(params) }),

  exportCSV: (params?: BillingQuery): Promise<AxiosResponse<Blob>> =>
    api.get('/billing/export', {
      params: compactQuery({ ...params, format: 'csv' }),
      responseType: 'blob',
    }),
}
