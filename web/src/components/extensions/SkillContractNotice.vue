<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type {
  SkillContractMetadata,
  SkillContractSource,
  SkillContractStatus,
} from '@/api/skill'

const props = withDefaults(
  defineProps<{
    contract?: SkillContractMetadata | null
    warnings?: string[]
    compact?: boolean
  }>(),
  {
    contract: null,
    warnings: () => [],
    compact: false,
  }
)

const { locale, t, te } = useI18n()

const isZhLocale = computed(() => locale.value.toLowerCase().startsWith('zh'))

function fallbackText(english: string, chinese: string): string {
  return isZhLocale.value ? chinese : english
}

function contractText(key: string, english: string, chinese: string): string {
  return te(key) ? String(t(key)) : fallbackText(english, chinese)
}

const normalizedStatus = computed<SkillContractStatus | ''>(() => {
  const status = props.contract?.contract_status?.trim()
  if (
    status === 'strict_contract' ||
    status === 'legacy_fallback' ||
    status === 'generated_contract'
  ) {
    return status
  }
  return ''
})

const normalizedSource = computed<SkillContractSource | ''>(() => {
  const source = props.contract?.contract_source?.trim()
  if (
    source === 'declared_frontmatter' ||
    source === 'legacy_frontmatter_fallback' ||
    source === 'generated_safe_defaults'
  ) {
    return source
  }
  return ''
})

const mergedNotes = computed(() => {
  const values = [...(props.contract?.contract_notes || []), ...(props.warnings || [])]
  const seen = new Set<string>()
  return values
    .map((value) => value.trim())
    .filter((value) => {
      if (!value || seen.has(value)) return false
      seen.add(value)
      return true
    })
})

const statusMeta = computed(() => {
  switch (normalizedStatus.value) {
    case 'strict_contract':
      return {
        tone: 'strict',
        label: contractText(
          'skills.contract.status.strict',
          'Strict contract',
          '严格契约'
        ),
        summary: contractText(
          'skills.contract.summary.strict',
          'Uses a declared frontmatter contract.',
          '使用声明式 frontmatter 契约。'
        ),
      }
    case 'legacy_fallback':
      return {
        tone: 'legacy',
        label: contractText(
          'skills.contract.status.legacy',
          'Legacy fallback',
          '兼容回退'
        ),
        summary: contractText(
          'skills.contract.summary.legacy',
          'Installed with legacy frontmatter fallback. Missing structural fields were repaired without guessing semantic fields.',
          '通过 legacy frontmatter 兼容回退安装。缺失的结构字段被补齐，但不会猜测语义字段。'
        ),
      }
    case 'generated_contract':
      return {
        tone: 'generated',
        label: contractText(
          'skills.contract.status.generated',
          'Generated contract',
          '生成契约'
        ),
        summary: contractText(
          'skills.contract.summary.generated',
          'Installed by generating safe structural defaults because the source did not declare a full contract.',
          '因来源未声明完整契约，系统仅生成安全的结构默认值完成安装。'
        ),
      }
    default:
      return {
        tone: 'neutral',
        label: contractText(
          'skills.contract.status.unknown',
          'Contract notes',
          '契约说明'
        ),
        summary: contractText(
          'skills.contract.summary.unknown',
          'Contract metadata is available for review.',
          '可查看当前契约元数据。'
        ),
      }
  }
})

const sourceLabel = computed(() => {
  switch (normalizedSource.value) {
    case 'declared_frontmatter':
      return contractText(
        'skills.contract.source.declared',
        'Declared frontmatter',
        '声明式 frontmatter'
      )
    case 'legacy_frontmatter_fallback':
      return contractText(
        'skills.contract.source.legacy',
        'Legacy frontmatter fallback',
        'legacy frontmatter 回退'
      )
    case 'generated_safe_defaults':
      return contractText(
        'skills.contract.source.generated',
        'Generated safe defaults',
        '生成的安全默认值'
      )
    default:
      return ''
  }
})

const showNotice = computed(() => !!normalizedStatus.value || mergedNotes.value.length > 0)
</script>

<template>
  <section
    v-if="showNotice"
    :class="[
      'skill-contract-notice',
      `skill-contract-notice--${statusMeta.tone}`,
      { 'skill-contract-notice--compact': compact },
    ]"
  >
    <div class="skill-contract-notice__header">
      <div class="skill-contract-notice__copy">
        <p class="skill-contract-notice__eyebrow">
          {{ contractText('skills.contract.heading', 'Contract', '契约状态') }}
        </p>
        <div class="skill-contract-notice__title-row">
          <span class="skill-contract-notice__badge">{{ statusMeta.label }}</span>
          <span
            v-if="sourceLabel"
            class="skill-contract-notice__source"
          >{{ sourceLabel }}</span>
        </div>
      </div>
    </div>

    <p class="skill-contract-notice__summary">
      {{ statusMeta.summary }}
    </p>

    <ul
      v-if="mergedNotes.length"
      class="skill-contract-notice__notes"
    >
      <li
        v-for="note in mergedNotes"
        :key="note"
      >
        {{ note }}
      </li>
    </ul>
  </section>
</template>

<style scoped>
.skill-contract-notice {
  display: flex;
  flex-direction: column;
  gap: 0.7rem;
  padding: 0.95rem 1rem;
  border-radius: 1rem;
  border: 1px solid rgba(148, 163, 184, 0.18);
  background: rgba(15, 23, 42, 0.3);
}

.skill-contract-notice--compact {
  padding: 0.8rem 0.9rem;
  gap: 0.55rem;
}

.skill-contract-notice--strict {
  border-color: rgba(110, 231, 183, 0.28);
  background: linear-gradient(180deg, rgba(20, 83, 45, 0.18), rgba(15, 23, 42, 0.28));
}

.skill-contract-notice--legacy {
  border-color: rgba(251, 191, 36, 0.28);
  background: linear-gradient(180deg, rgba(120, 53, 15, 0.2), rgba(15, 23, 42, 0.28));
}

.skill-contract-notice--generated {
  border-color: rgba(96, 165, 250, 0.3);
  background: linear-gradient(180deg, rgba(30, 64, 175, 0.18), rgba(15, 23, 42, 0.28));
}

.skill-contract-notice__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
}

.skill-contract-notice__copy {
  min-width: 0;
}

.skill-contract-notice__eyebrow {
  margin: 0 0 0.28rem;
  color: rgba(148, 163, 184, 0.92);
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.skill-contract-notice__title-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.45rem;
}

.skill-contract-notice__badge,
.skill-contract-notice__source {
  display: inline-flex;
  align-items: center;
  min-height: 1.75rem;
  padding: 0.2rem 0.65rem;
  border-radius: 999px;
  font-size: 0.78rem;
  font-weight: 700;
}

.skill-contract-notice__badge {
  border: 1px solid rgba(255, 255, 255, 0.12);
  background: rgba(15, 23, 42, 0.46);
  color: #f8fafc;
}

.skill-contract-notice__source {
  border: 1px solid rgba(148, 163, 184, 0.16);
  background: rgba(15, 23, 42, 0.24);
  color: rgba(226, 232, 240, 0.82);
  font-weight: 600;
}

.skill-contract-notice__summary {
  margin: 0;
  color: rgba(226, 232, 240, 0.9);
  font-size: 0.9rem;
  line-height: 1.55;
}

.skill-contract-notice__notes {
  margin: 0;
  padding-left: 1.1rem;
  color: rgba(226, 232, 240, 0.78);
  font-size: 0.82rem;
  line-height: 1.55;
}

.skill-contract-notice__notes li + li {
  margin-top: 0.22rem;
}

:global(.light .skill-contract-notice),
:global([data-theme='light'] .skill-contract-notice) {
  background: rgba(255, 255, 255, 0.9);
  border-color: rgba(203, 213, 225, 0.86);
}

:global(.light .skill-contract-notice--strict),
:global([data-theme='light'] .skill-contract-notice--strict) {
  background: linear-gradient(180deg, rgba(220, 252, 231, 0.84), rgba(248, 250, 252, 0.98));
  border-color: rgba(134, 239, 172, 0.5);
}

:global(.light .skill-contract-notice--legacy),
:global([data-theme='light'] .skill-contract-notice--legacy) {
  background: linear-gradient(180deg, rgba(254, 243, 199, 0.92), rgba(255, 251, 235, 0.98));
  border-color: rgba(251, 191, 36, 0.42);
}

:global(.light .skill-contract-notice--generated),
:global([data-theme='light'] .skill-contract-notice--generated) {
  background: linear-gradient(180deg, rgba(219, 234, 254, 0.92), rgba(248, 250, 252, 0.98));
  border-color: rgba(96, 165, 250, 0.38);
}

:global(.light .skill-contract-notice__eyebrow),
:global([data-theme='light'] .skill-contract-notice__eyebrow) {
  color: #64748b;
}

:global(.light .skill-contract-notice__badge),
:global([data-theme='light'] .skill-contract-notice__badge) {
  background: rgba(255, 255, 255, 0.88);
  border-color: rgba(203, 213, 225, 0.88);
  color: #0f172a;
}

:global(.light .skill-contract-notice__source),
:global([data-theme='light'] .skill-contract-notice__source) {
  background: rgba(248, 250, 252, 0.94);
  border-color: rgba(203, 213, 225, 0.88);
  color: #475569;
}

:global(.light .skill-contract-notice__summary),
:global([data-theme='light'] .skill-contract-notice__summary) {
  color: #334155;
}

:global(.light .skill-contract-notice__notes),
:global([data-theme='light'] .skill-contract-notice__notes) {
  color: #475569;
}
</style>
