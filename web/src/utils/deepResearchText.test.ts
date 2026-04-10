import { describe, expect, it } from 'vitest'

import {
  localizeDeepResearchAction,
  localizeDeepResearchGap,
  localizeDeepResearchReportStyle,
  localizeDeepResearchSegment,
  localizeDeepResearchSourceType,
  localizeDeepResearchStopReason,
  localizeDeepResearchStructuredValue,
  localizeDeepResearchSummary,
  localizeDeepResearchTimeWindow,
  localizeDeepResearchStatus,
  localizeResearchProgressLabel,
  localizeResearchRunningElsewhereLabel,
  localizeResearchSurfaceTitle,
} from '@/utils/deepResearchText'

const zhMessages: Record<string, string> = {
  'chat.deepResearchStageCompleted': '已完成',
  'chat.deepResearchActionCompleted': '已完成',
  'chat.deepResearchActionVerificationCompleted': '验证已完成',
  'chat.deepResearchActionFollowupPlanned': '已计划后续跟进',
  'chat.deepResearchActionLoopStopped': '研究循环已停止',
  'chat.deepResearchActionResearchBriefPrepared': '研究摘要已准备',
  'chat.deepResearchActionDraftSynthesisReady': '研究草稿已生成',
  'chat.deepResearchActionDetectedResearchGap': '发现研究缺口',
  'chat.deepResearchGapNeedEvidenceCoverage': '需要补充证据',
  'chat.deepResearchGapNeedBroaderSourceDiversity': '需要更广泛的来源多样性',
  'chat.deepResearchGapNeedPrimaryOrOfficialSources': '需要一手或官方来源',
  'chat.deepResearchPlannedTasks': '规划任务',
  'chat.deepResearchLiveSources': '实时来源',
  'chat.deepResearchSummaryGapCoverage': '{focus}仅覆盖 {evidenceCount} 条证据 / {domainCount} 个来源域名，需要继续深挖。',
  'chat.deepResearchSummaryOfficialGap': '{focus}尚未拿到稳定的一手/官方来源支撑。',
  'chat.deepResearchSummaryResolvedCoverage': '{focus}已覆盖 {evidenceCount} 条证据 / {domainCount} 个来源域名。',
  'chat.deepResearchSummaryDomainCoverage': '当前已覆盖 {domainCount} 个来源域名。',
  'chat.deepResearchSummaryFreshnessCoverage': '已覆盖到 {year} 年的较新来源。',
  'chat.deepResearchSummaryClaimSupportCoverage': '主要结论已被 {supportCount} 组支持信号覆盖。',
  'chat.deepResearchOpenQuestionPrimarySourceVerification': '建议回查一手来源并进行最终核验。',
  'chat.deepResearchSourceTypeLaw': '法规',
  'chat.deepResearchMetaOfficial': '官方',
  'chat.deepResearchStopReasonCoverage': '已达到覆盖目标',
  'chat.deepResearchReportStyleKnowledgeBase': '知识库',
  'chat.deepResearchWorkflowScope': '范围',
  'chat.deepResearchAxisLatest': '最新动态',
  'chat.deepResearchAxisOfficial': '官方来源',
  'chat.deepResearchAxisComparison': '对比信息',
  'chat.deepResearchFocusClaimValidation': '结论核验',
  'chat.deepResearchTimeWindowEarlier': '早期',
  'chat.deepResearchTimeWindowRecent': '近期',
  'system.warning': '警告',
  'system.info': '信息',
  'harness.group.overview': '概览',
}

function translate(key: string, fallback: string): string {
  return zhMessages[key] || fallback
}

describe('deepResearchText', () => {
  it('localizes bracket-wrapped status tokens', () => {
    expect(localizeDeepResearchStatus('[completed]', translate)).toBe('已完成')
    expect(localizeDeepResearchStatus('【completed】', translate)).toBe('已完成')
  })

  it('localizes common badge status tokens', () => {
    expect(localizeDeepResearchStatus('warning', translate)).toBe('警告')
    expect(localizeDeepResearchStatus('info', translate)).toBe('信息')
  })

  it('localizes exact english event summary phrases through action mappings', () => {
    expect(localizeDeepResearchAction('Research brief prepared', translate)).toBe('研究摘要已准备')
    expect(localizeDeepResearchAction('Draft synthesis ready', translate)).toBe('研究草稿已生成')
    expect(localizeDeepResearchAction('Detected a research gap', translate)).toBe('发现研究缺口')
  })

  it('localizes bracket-wrapped segment tokens', () => {
    expect(localizeDeepResearchSegment('[completed]', translate)).toBe('已完成')
    expect(localizeDeepResearchSegment('【completed】', translate)).toBe('已完成')
  })

  it('localizes compound gap strings segment by segment', () => {
    expect(
      localizeDeepResearchGap(
        'Need evidence coverage • Need broader source diversity • Need primary or official sources',
        translate
      )
    ).toBe('需要补充证据 · 需要更广泛的来源多样性 · 需要一手或官方来源')
  })

  it('localizes common primary-source gap phrase variants', () => {
    expect(localizeDeepResearchGap('Need official source', translate)).toBe('需要一手或官方来源')
    expect(localizeDeepResearchGap('Need one more primary source.', translate)).toBe(
      '需要一手或官方来源'
    )
  })

  it('localizes structured deep research values', () => {
    expect(localizeDeepResearchSourceType('law', translate)).toBe('法规')
    expect(localizeDeepResearchStructuredValue('official', translate)).toBe('官方来源')
    expect(localizeDeepResearchStructuredValue('Overview', translate)).toBe('概览')
    expect(localizeDeepResearchStructuredValue('Scope', translate)).toBe('范围')
    expect(localizeDeepResearchStructuredValue('Latest', translate)).toBe('最新动态')
    expect(localizeDeepResearchStructuredValue('Claim validation', translate)).toBe('结论核验')
  })

  it('localizes time window tokens without rewriting raw ranges', () => {
    expect(localizeDeepResearchTimeWindow('earlier', translate)).toBe('早期')
    expect(localizeDeepResearchTimeWindow('recent', translate)).toBe('近期')
    expect(localizeDeepResearchTimeWindow('2025-2026', translate)).toBe('2025-2026')
  })

  it('localizes deep research payload summaries and stop reasons', () => {
    expect(localizeDeepResearchSummary('Research brief prepared', translate)).toBe('研究摘要已准备')
    expect(localizeDeepResearchSummary('Draft synthesis ready', translate)).toBe('研究草稿已生成')
    expect(localizeDeepResearchSummary('Detected a research gap', translate)).toBe('发现研究缺口')
    expect(localizeDeepResearchSummary('Planned 5 research task(s)', translate)).toBe(
      '规划任务: 5'
    )
    expect(localizeDeepResearchSummary('Collected 3 source(s)', translate)).toBe('实时来源: 3')
    expect(localizeDeepResearchSummary('Verification pass completed', translate)).toBe(
      '验证已完成'
    )
    expect(localizeDeepResearchSummary('Planned a follow-up research pass', translate)).toBe(
      '已计划后续跟进'
    )
    expect(localizeDeepResearchSummary('Deep research loop stopped', translate)).toBe(
      '研究循环已停止'
    )
    expect(localizeDeepResearchStopReason('coverage_sufficient', translate)).toBe('已达到覆盖目标')
  })

  it('localizes numeric deep research summaries and known english verification prompts', () => {
    expect(localizeDeepResearchStructuredValue('Official sources', translate)).toBe('官方来源')
    expect(
      localizeDeepResearchSummary(
        'Overview only covers 0 evidence item(s) across 0 domain(s); follow-up research is needed.',
        translate
      )
    ).toBe('概览仅覆盖 0 条证据 / 0 个来源域名，需要继续深挖。')
    expect(
      localizeDeepResearchSummary(
        'Official sources still lacks stable primary or official sources.',
        translate
      )
    ).toBe('官方来源尚未拿到稳定的一手/官方来源支撑。')
    expect(
      localizeDeepResearchSummary(
        'Comparison is covered by 8 evidence item(s) across 8 domain(s).',
        translate
      )
    ).toBe('对比信息已覆盖 8 条证据 / 8 个来源域名。')
    expect(localizeDeepResearchSummary('Coverage spans 8 unique domain(s).', translate)).toBe(
      '当前已覆盖 8 个来源域名。'
    )
    expect(localizeDeepResearchSummary('Fresh evidence reaches 2026.', translate)).toBe(
      '已覆盖到 2026 年的较新来源。'
    )
    expect(
      localizeDeepResearchSummary('Core conclusions are supported across 7 claim group(s).', translate)
    ).toBe('主要结论已被 7 组支持信号覆盖。')
    expect(
      localizeDeepResearchSummary('Check primary sources for final verification.', translate)
    ).toBe('建议回查一手来源并进行最终核验。')
  })

  it('localizes deep research report styles', () => {
    expect(localizeDeepResearchReportStyle('knowledge_base', translate)).toBe('知识库')
  })

  it('prefers deep research title and progress labels when available', () => {
    const translateDeepResearch = (key: string, fallback: string): string =>
      (
        ({
          'chat.deepResearchTitle': '深度研究',
          'chat.deepResearchProgress': '深度研究进行中',
          'chat.deepResearchRunningElsewhere': '跨会话跟踪当前进行中的深度研究任务。',
        }) as Record<string, string>
      )[key] || fallback

    expect(localizeResearchSurfaceTitle(translateDeepResearch)).toBe('深度研究')
    expect(localizeResearchProgressLabel(translateDeepResearch)).toBe('深度研究进行中')
    expect(localizeResearchRunningElsewhereLabel(translateDeepResearch)).toBe(
      '跨会话跟踪当前进行中的深度研究任务。'
    )
  })

  it('falls back to generic research aliases when deep research labels are absent', () => {
    const translateResearch = (key: string, fallback: string): string =>
      (
        ({
          'chat.researchTitle': '研究',
          'chat.researchProgress': '研究进行中',
          'chat.researchRunningElsewhere': '跨会话跟踪当前进行中的研究任务。',
        }) as Record<string, string>
      )[key] || fallback

    expect(localizeResearchSurfaceTitle(translateResearch)).toBe('研究')
    expect(localizeResearchProgressLabel(translateResearch)).toBe('研究进行中')
    expect(localizeResearchRunningElsewhereLabel(translateResearch)).toBe('跨会话跟踪当前进行中的研究任务。')
  })
})
