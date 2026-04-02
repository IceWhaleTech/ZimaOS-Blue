import { describe, expect, it } from 'vitest'

import {
  firstMarketplaceSourceCandidate,
  localizeMarketplaceSource,
  localizeMarketplaceSourceFromCandidates,
  localizeMarketplaceSourceOptionDescription,
  localizeMarketplaceSourceOptionLabel,
  trimMarketplaceSourceToken,
} from '@/utils/skillMarketplaceSources'

const translate = (path: string, fallback: string, params?: Record<string, unknown>) => {
  const messages: Record<string, string> = {
    'sources.skillhub.label': 'SkillHub',
    'sources.skillhub.description': '聚合说明',
    'sources.tencentSkillHub.label': '腾讯 SkillHub',
    'sources.tencentSkillHub.description': '腾讯来源说明',
    'sources.seedInstance.label': '发现页 {index}',
    'sources.seedInstance.description': '发现页说明 {index}',
    'sources.clawhubMirror.label': 'ClawHub 镜像',
    'sources.clawhubMirror.description': '镜像说明 {index}',
  }

  const template = messages[path] || fallback
  if (!params) return template
  return Object.entries(params).reduce(
    (text, [name, value]) => text.split(`{${name}}`).join(String(value ?? '')),
    template
  )
}

describe('skillMarketplaceSources', () => {
  it('trims source tokens and finds the first non-empty candidate', () => {
    expect(trimMarketplaceSourceToken('  skillhub ')).toBe('skillhub')
    expect(trimMarketplaceSourceToken('   ')).toBe('')
    expect(firstMarketplaceSourceCandidate([undefined, '  ', ' skillhub '])).toBe('skillhub')
  })

  it('localizes known source ids and source groups', () => {
    expect(localizeMarketplaceSource('tencent-skillhub', 'label', translate)).toBe('腾讯 SkillHub')
    expect(localizeMarketplaceSource('skillhub', 'description', translate)).toBe('聚合说明')
  })

  it('supports seed and mirror source instances with params', () => {
    expect(localizeMarketplaceSource('seed-7', 'label', translate)).toBe('发现页 7')
    expect(localizeMarketplaceSource('Discovery Page 8', 'label', translate)).toBe('发现页 8')
    expect(localizeMarketplaceSource('clawhub-mirror-2', 'description', translate)).toBe(
      '镜像说明 2'
    )
  })

  it('walks candidates in order and falls back for options', () => {
    expect(
      localizeMarketplaceSourceFromCandidates([undefined, 'Tencent SkillHub', 'skillhub'], 'label', translate)
    ).toBe('腾讯 SkillHub')
    expect(
      localizeMarketplaceSourceOptionLabel(
        { value: 'skillhub', label: 'Tencent SkillHub' },
        translate,
        'Marketplace'
      )
    ).toBe('SkillHub')
    expect(
      localizeMarketplaceSourceOptionDescription(
        { value: 'skillhub', label: 'Tencent SkillHub' },
        translate
      )
    ).toBe('聚合说明')
    expect(
      localizeMarketplaceSourceOptionLabel({ value: 'custom-source', label: 'Custom Source' }, translate, 'Marketplace')
    ).toBe('Custom Source')
  })
})
