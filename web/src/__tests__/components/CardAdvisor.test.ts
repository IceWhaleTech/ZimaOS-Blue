import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

import CardAdvisor from '@/components/typeless/CardAdvisor.vue'

function createTestI18n(locale = 'en-US') {
  return createI18n({
    legacy: false,
    locale,
    fallbackLocale: 'en-US',
    messages: {
      'en-US': {
        tools: {
          names: {
            advisor: 'Advisor',
          },
        },
        advisorCard: {
          pending: 'Advisor in progress',
          recommendation: 'Recommendation',
          rationale: 'Why',
          winner: 'Winner',
          topCandidates: 'Top candidates',
          weightedCriteria: 'Weighted criteria',
          tradeoffs: 'Tradeoffs',
          risks: 'Risks',
          bestPractices: 'Best practices',
          alternatives: 'Alternatives',
          confidence: 'Confidence',
          evidence: 'Evidence',
          progress: 'Progress',
          jobId: 'Job ID',
          secondOpinion: 'Second opinion',
        },
      },
      'zh-CN': {
        tools: {
          names: {
            advisor: '顾问建议',
          },
        },
        advisorCard: {
          pending: '顾问建议进行中',
          recommendation: '建议',
          rationale: '原因',
          winner: '推荐胜出项',
          topCandidates: '候选排序',
          weightedCriteria: '加权标准',
          tradeoffs: '取舍',
          risks: '风险',
          bestPractices: '最佳实践',
          alternatives: '备选方案',
          confidence: '置信度',
          evidence: '证据',
          progress: '进度',
          jobId: '任务 ID',
          secondOpinion: '第二意见',
        },
      },
    },
  })
}

describe('CardAdvisor', () => {
  it('renders a dedicated advisor summary card with scorecard details and evidence links', () => {
    const wrapper = mount(CardAdvisor, {
      props: {
        card: {
          type: 'advisor',
          id: 'advisor-card-1',
          recommendation: 'Use Go for the latency-sensitive API and keep Python for batch jobs.',
          confidence: 0.82,
          why: ['Go keeps p99 latency lower under burst traffic.'],
          winner: 'Go',
          pack_id: 'solution_selection_v1',
          candidates: [
            { name: 'Go', rank: 1, total_score: 89, verdict: 'Best fit' },
            { name: 'Python', rank: 2, total_score: 74, verdict: 'Good for data workflows' },
          ],
          weights: [
            { criterion: 'performance_efficiency', label: 'Performance', weight: 0.3 },
            { criterion: 'team_velocity', label: 'Team velocity', weight: 0.2 },
          ],
          tradeoffs: ['Hiring for experienced Go backend engineers may take longer.'],
          risks: ['Migration can stall if shared libraries remain Python-only.'],
          best_practices: ['Start with one service and keep a shared contract test suite.'],
          alternatives: ['Keep Python for internal tools and async jobs.'],
          evidence_count: 2,
          evidence: [
            { label: 'Go docs', url: 'https://go.dev/doc/' },
            { label: 'Python docs', url: 'https://docs.python.org/3/' },
          ],
          second_opinion: { used: true, summary: 'A challenger model reached the same winner.' },
        } as any,
      },
      global: {
        plugins: [createTestI18n('zh-CN')],
      },
    })

    expect(wrapper.attributes('id')).toBe('advisor-card-1')
    expect(wrapper.text()).toContain('顾问建议')
    expect(wrapper.text()).toContain('建议')
    expect(wrapper.text()).toContain('Use Go for the latency-sensitive API')
    expect(wrapper.text()).toContain('推荐胜出项')
    expect(wrapper.text()).toContain('Go')
    expect(wrapper.text()).toContain('候选排序')
    expect(wrapper.text()).toContain('加权标准')
    expect(wrapper.text()).toContain('取舍')
    expect(wrapper.text()).toContain('风险')
    expect(wrapper.text()).toContain('最佳实践')
    expect(wrapper.text()).toContain('备选方案')
    expect(wrapper.text()).toContain('第二意见')
    expect(wrapper.text()).toContain('82%')

    const links = wrapper.findAll('a')
    expect(links).toHaveLength(2)
    expect(links[0]?.attributes('href')).toBe('https://go.dev/doc/')
  })

  it('renders async advisor progress cards with progress and job metadata', () => {
    const wrapper = mount(CardAdvisor, {
      props: {
        card: {
          type: 'advisor',
          id: 'advisor-job-1',
          status: 'running',
          recommendation: 'Go vs Python vs Node',
          progress: 42,
          job_id: 'job-123',
          mode: 'advisor',
        } as any,
      },
      global: {
        plugins: [createTestI18n('en-US')],
      },
    })

    expect(wrapper.text()).toContain('Advisor in progress')
    expect(wrapper.text()).toContain('Go vs Python vs Node')
    expect(wrapper.text()).toContain('Progress')
    expect(wrapper.text()).toContain('42%')
    expect(wrapper.text()).toContain('Job ID')
    expect(wrapper.text()).toContain('job-123')
  })
})
