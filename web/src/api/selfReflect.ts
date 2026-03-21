import api from './client'

export type SelfReflectProposalStatus = 'pending' | 'approved' | 'rejected'

export interface SelfReflectProposal {
  id: string
  owner_user_id?: string
  source_kind?: string
  source_id?: string
  proposal_mode?: 'review_only'
  target_file: string
  target_section?: string
  status: SelfReflectProposalStatus
  dedup_key?: string
  lesson: string
  when_to_apply?: string
  evidence: string
  evidence_ids?: string[]
  evaluation_summary?: Record<string, unknown> | null
  calibration_summary?: Record<string, unknown> | null
  patch_preview?: string
  review_note?: string
  created_at: string
  updated_at: string
  reviewed_at?: string | null
}

export interface SelfReflectProposalListParams {
  status?: SelfReflectProposalStatus | SelfReflectProposalStatus[]
  source_kind?: string
  source_id?: string
  limit?: number
}

function normalizeQueryArray(value?: string | string[]) {
  if (!value) return undefined
  return Array.isArray(value) ? value.join(',') : value
}

export const selfReflectApi = {
  listProposals: (params: SelfReflectProposalListParams = {}) =>
    api.get<SelfReflectProposal[]>('/self-reflect/proposals', {
      params: {
        status: normalizeQueryArray(params.status),
        source_kind: params.source_kind,
        source_id: params.source_id,
        limit: params.limit,
      },
    }),

  getProposal: (id: string) => api.get<SelfReflectProposal>(`/self-reflect/proposals/${id}`),

  getPatchPreview: (id: string) =>
    api.get<{ id: string; target_file: string; patch_preview: string }>(
      `/self-reflect/proposals/${id}/patch`
    ),

  approveProposal: (id: string, reviewNote = '') =>
    api.post<SelfReflectProposal>(`/self-reflect/proposals/${id}/approve`, {
      review_note: reviewNote,
    }),

  rejectProposal: (id: string, reviewNote = '') =>
    api.post<SelfReflectProposal>(`/self-reflect/proposals/${id}/reject`, {
      review_note: reviewNote,
    }),
}

export default selfReflectApi
