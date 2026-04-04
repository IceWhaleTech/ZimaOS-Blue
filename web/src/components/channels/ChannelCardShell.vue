<script setup lang="ts">
import { computed } from 'vue'

type StatusTone = 'connected' | 'connecting' | 'error' | 'disconnected'

const props = withDefaults(
  defineProps<{
    classPrefix: string
    title: string
    description: string
    statusTone: StatusTone
    statusText: string
    statusTitle?: string
    expanded: boolean
    descriptionError?: boolean
    metaLabel?: string
    iconSrc: string
    iconAlt: string
    iconStyle?: Record<string, string>
  }>(),
  {
    statusTitle: '',
    descriptionError: false,
    metaLabel: '',
    iconStyle: () => ({}),
  }
)

const emit = defineEmits<{
  headerClick: []
}>()

function prefixedElementClass(element: string): string {
  return `${props.classPrefix}__${element}`
}

function prefixedBlockModifierClass(modifier: string): string {
  return `${props.classPrefix}--${modifier}`
}

function prefixedElementModifierClass(element: string, modifier: string): string {
  return `${props.classPrefix}__${element}--${modifier}`
}

const rootClasses = computed(() => [
  'channel-card-shell',
  'dashboard-card-surface',
  props.classPrefix,
  `channel-card-shell--${props.statusTone}`,
  prefixedBlockModifierClass(props.statusTone),
  {
    'channel-card-shell--expanded': props.expanded,
    [prefixedBlockModifierClass('expanded')]: props.expanded,
  },
])

const headerClasses = computed(() => ['channel-card-shell__header', prefixedElementClass('header')])
const identityClasses = computed(() => [
  'channel-card-shell__identity',
  prefixedElementClass('identity'),
])
const iconShellClasses = computed(() => [
  'channel-card-shell__icon-shell',
  prefixedElementClass('icon-shell'),
])
const iconClasses = computed(() => ['channel-card-shell__icon', prefixedElementClass('icon')])
const copyClasses = computed(() => ['channel-card-shell__copy', prefixedElementClass('copy')])
const titleRowClasses = computed(() => [
  'channel-card-shell__title-row',
  prefixedElementClass('title-row'),
])
const titleClasses = computed(() => ['channel-card-shell__title', prefixedElementClass('title')])
const metaBadgeClasses = computed(() => [
  'channel-card-shell__meta-badge',
  prefixedElementClass('badge'),
])
const statusBadgeClasses = computed(() => [
  'channel-card-shell__status-badge',
  prefixedElementClass('status-badge'),
  `channel-card-shell__status-badge--${props.statusTone}`,
  prefixedElementModifierClass('status-badge', props.statusTone),
])
const statusDotClasses = computed(() => [
  'channel-card-shell__status-dot',
  prefixedElementClass('status-dot'),
])
const descriptionClasses = computed(() => [
  'channel-card-shell__description',
  prefixedElementClass('description'),
  {
    'channel-card-shell__description--error': props.descriptionError,
    [prefixedElementModifierClass('description', 'error')]: props.descriptionError,
  },
])
const asideClasses = computed(() => ['channel-card-shell__aside', prefixedElementClass('aside')])
const resolvedStatusTitle = computed(() => props.statusTitle || props.statusText)
</script>

<template>
  <div :class="rootClasses">
    <div :class="headerClasses" @click="emit('headerClick')">
      <div :class="identityClasses">
        <div :class="iconShellClasses">
          <img :src="iconSrc" :alt="iconAlt" :class="iconClasses" :style="iconStyle" />
        </div>
        <div :class="copyClasses">
          <div :class="titleRowClasses">
            <h3 :class="titleClasses">{{ title }}</h3>
            <span v-if="metaLabel" :class="metaBadgeClasses">
              {{ metaLabel }}
            </span>
            <slot name="title-meta"></slot>
            <span :class="statusBadgeClasses" :title="resolvedStatusTitle">
              <span :class="statusDotClasses"></span>
              {{ statusText }}
            </span>
          </div>
          <p :class="descriptionClasses">{{ description }}</p>
        </div>
      </div>

      <div v-if="$slots.actions" :class="asideClasses">
        <slot name="actions"></slot>
      </div>
    </div>
  </div>
</template>

<style scoped>
.channel-card-shell {
  position: relative;
  overflow: hidden;
  border: 1px solid rgba(226, 232, 240, 0.96);
  border-radius: 1.25rem;
  background: #ffffff;
  box-shadow: none;
  color: #0f172a;
  transition:
    transform 180ms ease,
    border-color 180ms ease,
    box-shadow 180ms ease;
}

.channel-card-shell:hover {
  transform: translateY(-1px);
}

.channel-card-shell::before {
  content: '';
  position: absolute;
  inset: 0 0 auto 0;
  height: 2px;
  opacity: 0;
  transition: opacity 180ms ease;
}

.channel-card-shell--expanded {
  border-color: rgba(15, 23, 42, 0.14);
  box-shadow: 0 18px 38px -30px rgba(15, 23, 42, 0.4);
}

.channel-card-shell--expanded::before,
.channel-card-shell:hover::before {
  opacity: 1;
}

.channel-card-shell--connected::before {
  opacity: 1;
  background: rgba(22, 163, 74, 0.9);
}

.channel-card-shell--connecting::before {
  opacity: 1;
  background: rgba(245, 158, 11, 0.92);
}

.channel-card-shell--error::before {
  opacity: 1;
  background: rgba(239, 68, 68, 0.92);
}

.channel-card-shell__header {
  display: flex;
  align-items: center;
  gap: 1.1rem;
  min-height: 4.9rem;
  padding: 0.92rem 1rem;
  cursor: pointer;
  transition: background-color 160ms ease;
}

.channel-card-shell__header:hover {
  background: rgba(148, 163, 184, 0.08);
}

.channel-card-shell__identity {
  min-width: 0;
  flex: 1;
  display: flex;
  align-items: center;
  gap: 0.82rem;
}

.channel-card-shell__icon-shell {
  width: 2.55rem;
  height: 2.55rem;
  flex-shrink: 0;
  display: grid;
  place-items: center;
  border-radius: 0.82rem;
  background: #f8fafc;
  border: 1px solid rgba(226, 232, 240, 0.96);
  box-shadow: none;
  transition:
    border-color 180ms ease,
    background-color 180ms ease,
    transform 180ms ease;
}

.channel-card-shell__icon {
  width: 1.58rem;
  height: 1.58rem;
  object-fit: contain;
  transform: scale(var(--channel-icon-scale, 1));
  transform-origin: center;
}

.channel-card-shell__copy {
  min-width: 0;
  flex: 1;
}

.channel-card-shell__title-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.42rem;
  min-height: 1.3rem;
}

.channel-card-shell__title {
  font-size: 0.9rem;
  font-weight: 700;
  color: #0f172a;
}

.channel-card-shell__description {
  margin: 0.28rem 0 0;
  font-size: 0.76rem;
  line-height: 1.4;
  color: #64748b;
  display: -webkit-box;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.channel-card-shell__description--error {
  color: #b91c1c;
}

.channel-card-shell__meta-badge {
  display: inline-flex;
  align-items: center;
  min-height: 1.24rem;
  padding: 0.14rem 0.42rem;
  border-radius: 999px;
  border: 1px solid rgba(203, 213, 225, 0.85);
  background: rgba(248, 250, 252, 0.96);
  color: #334155;
  font-size: 0.58rem;
  font-weight: 700;
  line-height: 1;
}

.channel-card-shell__status-badge {
  display: inline-flex;
  align-items: center;
  gap: 0.26rem;
  min-height: 1.24rem;
  padding: 0.14rem 0.42rem;
  border-radius: 999px;
  border: 1px solid transparent;
  font-size: 0.58rem;
  font-weight: 700;
  line-height: 1;
}

.channel-card-shell__status-dot {
  width: 0.24rem;
  height: 0.24rem;
  border-radius: 999px;
  background: currentColor;
}

.channel-card-shell__status-badge--connected {
  color: #166534;
  background: rgba(220, 252, 231, 0.95);
  border-color: rgba(134, 239, 172, 0.72);
}

.channel-card-shell__status-badge--connecting {
  color: #92400e;
  background: rgba(254, 243, 199, 0.95);
  border-color: rgba(252, 211, 77, 0.72);
}

.channel-card-shell__status-badge--error {
  color: #b91c1c;
  background: rgba(254, 226, 226, 0.95);
  border-color: rgba(252, 165, 165, 0.72);
}

.channel-card-shell__status-badge--disconnected {
  color: #475569;
  background: rgba(226, 232, 240, 0.92);
  border-color: rgba(203, 213, 225, 0.85);
}

.channel-card-shell__status-badge--connecting .channel-card-shell__status-dot {
  animation: channel-card-shell-status-pulse 1.35s ease-in-out infinite;
}

.channel-card-shell__aside {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  gap: 0.72rem;
}

.channel-card-shell--connected .channel-card-shell__icon-shell {
  border-color: rgba(134, 239, 172, 0.8);
  background: rgba(240, 253, 244, 0.96);
  box-shadow: none;
}

.channel-card-shell--connecting .channel-card-shell__icon-shell {
  border-color: rgba(252, 211, 77, 0.82);
  background: rgba(255, 251, 235, 0.98);
  box-shadow: none;
}

.channel-card-shell--error .channel-card-shell__icon-shell {
  border-color: rgba(252, 165, 165, 0.8);
  background: rgba(254, 242, 242, 0.98);
  box-shadow: none;
}

@keyframes channel-card-shell-status-pulse {
  0%,
  100% {
    transform: scale(1);
    opacity: 1;
  }

  50% {
    transform: scale(1.45);
    opacity: 0.55;
  }
}

@media (max-width: 639px) {
  .channel-card-shell__header {
    align-items: flex-start;
    padding: 0.82rem 0.88rem;
  }

  .channel-card-shell__identity {
    align-items: flex-start;
  }
}

:root.dark .channel-card-shell,
[data-theme='dark'] .channel-card-shell,
html.dark .channel-card-shell {
  border-color: rgba(71, 85, 105, 0.46);
  background: #111827;
  box-shadow: none;
  color: #e2e8f0;
}

:root.dark .channel-card-shell--expanded,
[data-theme='dark'] .channel-card-shell--expanded,
html.dark .channel-card-shell--expanded {
  border-color: rgba(148, 163, 184, 0.42);
  box-shadow: 0 18px 38px -30px rgba(15, 23, 42, 0.78);
}

:root.dark .channel-card-shell__icon-shell,
[data-theme='dark'] .channel-card-shell__icon-shell,
html.dark .channel-card-shell__icon-shell {
  background: rgba(15, 23, 42, 0.72);
  border-color: rgba(71, 85, 105, 0.5);
}

:root.dark .channel-card-shell__header:hover,
[data-theme='dark'] .channel-card-shell__header:hover,
html.dark .channel-card-shell__header:hover {
  background: rgba(51, 65, 85, 0.28);
}

:root.dark .channel-card-shell__title,
[data-theme='dark'] .channel-card-shell__title,
html.dark .channel-card-shell__title {
  color: #f8fafc;
}

:root.dark .channel-card-shell__description,
[data-theme='dark'] .channel-card-shell__description,
html.dark .channel-card-shell__description {
  color: #cbd5e1;
}

:root.dark .channel-card-shell__description--error,
[data-theme='dark'] .channel-card-shell__description--error,
html.dark .channel-card-shell__description--error {
  color: #fecaca;
}

:root.dark .channel-card-shell__meta-badge,
[data-theme='dark'] .channel-card-shell__meta-badge,
html.dark .channel-card-shell__meta-badge {
  color: #cbd5e1;
  background: rgba(51, 65, 85, 0.86);
  border-color: rgba(100, 116, 139, 0.72);
}

:root.dark .channel-card-shell__status-badge--disconnected,
[data-theme='dark'] .channel-card-shell__status-badge--disconnected,
html.dark .channel-card-shell__status-badge--disconnected {
  color: #cbd5e1;
  background: rgba(51, 65, 85, 0.86);
  border-color: rgba(100, 116, 139, 0.72);
}

:root.dark .channel-card-shell__status-badge--connected,
[data-theme='dark'] .channel-card-shell__status-badge--connected,
html.dark .channel-card-shell__status-badge--connected {
  color: #bbf7d0;
  background: rgba(20, 83, 45, 0.52);
  border-color: rgba(74, 222, 128, 0.34);
}

:root.dark .channel-card-shell__status-badge--connecting,
[data-theme='dark'] .channel-card-shell__status-badge--connecting,
html.dark .channel-card-shell__status-badge--connecting {
  color: #fde68a;
  background: rgba(120, 53, 15, 0.48);
  border-color: rgba(251, 191, 36, 0.34);
}

:root.dark .channel-card-shell__status-badge--error,
[data-theme='dark'] .channel-card-shell__status-badge--error,
html.dark .channel-card-shell__status-badge--error {
  color: #fecaca;
  background: rgba(127, 29, 29, 0.5);
  border-color: rgba(248, 113, 113, 0.34);
}
</style>
