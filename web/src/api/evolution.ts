import api from './client'

export interface EvolutionOverviewRevisionCounts {
  accepted: number
}

export interface EvolutionOverviewInstructionCounts {
  pending: number
}

export interface EvolutionOverview {
  skill_id?: string
  revisions: EvolutionOverviewRevisionCounts
  instructions: EvolutionOverviewInstructionCounts
}

export interface EvolutionOverviewParams {
  skill_id?: string
}

export const evolutionApi = {
  getOverview: (params: EvolutionOverviewParams = {}) =>
    api.get<EvolutionOverview>('/harness/evolution/overview', {
      params: {
        skill_id: params.skill_id,
      },
    }),
}

export default evolutionApi
