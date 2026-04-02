<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'

const route = useRoute()
const { t, te } = useI18n()

function tr(key: string, fallback: string): string {
  return te(key) ? t(key) : fallback
}

const tabs = computed(() => [
  {
    id: 'cron',
    to: '/automation',
    label: tr('automation.tabs.cron', 'Scheduled Tasks'),
    description: tr('automation.tabs.cronDesc', 'Manage cron jobs and scheduled executions'),
    icon:
      '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M12 6.75v5.25l3 1.75"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/>',
    active: route.path === '/automation' || route.path === '/cron',
  },
  {
    id: 'harness',
    to: '/automation/harness',
    label: tr('automation.tabs.harness', 'Harness'),
    description: tr(
      'automation.tabs.harnessDesc',
      'Review run records, eval groups, and scoring results.'
    ),
    icon:
      '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M4.75 18.25h14.5"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M7.75 15.25V10.5"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M12 15.25V7.25"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M16.25 15.25v-2.5"/>',
    active: route.path.startsWith('/automation/harness') || route.path.startsWith('/harness'),
  },
])
</script>

<template>
  <nav class="automation-tab-nav" :aria-label="tr('nav.automation', 'Automation')">
    <RouterLink
      v-for="tab in tabs"
      :key="tab.id"
      :to="tab.to"
      class="automation-tab-button"
      :class="{ 'automation-tab-button--active': tab.active }"
      :aria-current="tab.active ? 'page' : undefined"
      :aria-selected="tab.active ? 'true' : 'false'"
      :title="tab.description"
      :data-testid="`automation-tab-${tab.id}`"
    >
      <span class="automation-tab-button__icon" aria-hidden="true">
        <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" v-html="tab.icon" />
      </span>
      <span class="automation-tab-button__body">
        <span class="automation-tab-button__label">{{ tab.label }}</span>
      </span>
      <span class="automation-tab-button__state" aria-hidden="true"></span>
    </RouterLink>
  </nav>
</template>

<style scoped>
.automation-tab-nav {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  padding: 0;
  border: 0;
  background: transparent;
  box-shadow: none;
}

.automation-tab-button {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 8px;
  min-height: 100%;
  padding: 10px 12px;
  border-radius: 1rem;
  border: 1px solid rgba(226, 232, 240, 0.96);
  background: #ffffff;
  color: #0f172a;
  text-decoration: none;
  box-shadow: none;
  transition:
    border-color 0.22s ease,
    background-color 0.22s ease,
    color 0.22s ease;
}

.automation-tab-button:hover {
  border-color: rgba(148, 163, 184, 0.52);
  background: #f8fafc;
}

.automation-tab-button--active {
  border-color: rgba(148, 163, 184, 0.58);
  background: #f1f5f9;
  color: #0f172a;
}

.automation-tab-button__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2rem;
  height: 2rem;
  flex-shrink: 0;
  border-radius: 0.65rem;
  background: rgba(148, 163, 184, 0.16);
  color: #64748b;
}

.automation-tab-button__icon svg {
  width: 1rem;
  height: 1rem;
}

.automation-tab-button--active .automation-tab-button__icon {
  background: rgba(148, 163, 184, 0.22);
  color: #475569;
}

.automation-tab-button__body {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 0.32rem;
}

.automation-tab-button__label {
  font-size: 0.9rem;
  font-weight: 700;
  line-height: 1.25;
}

.automation-tab-button__state {
  width: 9px;
  height: 9px;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.35);
  transition:
    transform 0.22s ease,
    background-color 0.22s ease;
}

.automation-tab-button--active .automation-tab-button__state {
  transform: scale(1.05);
  background: #64748b;
}

:root.dark .automation-tab-button,
[data-theme='dark'] .automation-tab-button,
html.dark .automation-tab-button {
  border-color: rgba(71, 85, 105, 0.78);
  background: rgba(15, 23, 42, 0.82);
  color: #e2e8f0;
}

:root.dark .automation-tab-button--active,
[data-theme='dark'] .automation-tab-button--active,
html.dark .automation-tab-button--active {
  border-color: rgba(100, 116, 139, 0.9);
  background: rgba(30, 41, 59, 0.94);
}

:root.dark .automation-tab-button:hover,
[data-theme='dark'] .automation-tab-button:hover,
html.dark .automation-tab-button:hover {
  background: rgba(30, 41, 59, 0.9);
}

:root.dark .automation-tab-button__icon,
[data-theme='dark'] .automation-tab-button__icon,
html.dark .automation-tab-button__icon {
  background: rgba(51, 65, 85, 0.92);
  color: #cbd5e1;
}

:root.dark .automation-tab-button--active .automation-tab-button__icon,
[data-theme='dark'] .automation-tab-button--active .automation-tab-button__icon,
html.dark .automation-tab-button--active .automation-tab-button__icon {
  background: rgba(71, 85, 105, 0.92);
  color: #f8fafc;
}

:root.dark .automation-tab-button__state,
[data-theme='dark'] .automation-tab-button__state,
html.dark .automation-tab-button__state {
  background: rgba(148, 163, 184, 0.48);
}

:root.dark .automation-tab-button--active .automation-tab-button__state,
[data-theme='dark'] .automation-tab-button--active .automation-tab-button__state,
html.dark .automation-tab-button--active .automation-tab-button__state {
  background: #e2e8f0;
}

@media (max-width: 640px) {
  .automation-tab-nav {
    grid-template-columns: 1fr;
  }
}
</style>
