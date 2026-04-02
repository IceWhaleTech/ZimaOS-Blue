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
    active: route.path.startsWith('/automation/harness') || route.path.startsWith('/harness'),
  },
])
</script>

<template>
  <nav class="automation-tabs" :aria-label="tr('nav.automation', 'Automation')">
    <RouterLink
      v-for="tab in tabs"
      :key="tab.id"
      :to="tab.to"
      class="automation-tab"
      :class="{ 'is-active': tab.active }"
      :aria-current="tab.active ? 'page' : undefined"
      :data-testid="`automation-tab-${tab.id}`"
    >
      <span class="automation-tab__label">{{ tab.label }}</span>
      <span class="automation-tab__description">{{ tab.description }}</span>
    </RouterLink>
  </nav>
</template>

<style scoped>
.automation-tabs {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 0.75rem;
}

.automation-tab {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
  padding: 0.9rem 1rem;
  border-radius: 1rem;
  border: 1px solid rgba(148, 163, 184, 0.2);
  background: rgba(255, 255, 255, 0.8);
  color: inherit;
  text-decoration: none;
  transition:
    border-color 0.16s ease,
    transform 0.16s ease,
    background 0.16s ease,
    box-shadow 0.16s ease;
}

.automation-tab:hover {
  transform: translateY(-1px);
  border-color: rgba(37, 99, 235, 0.28);
  background: rgba(37, 99, 235, 0.05);
}

.automation-tab.is-active {
  border-color: rgba(15, 23, 42, 0.18);
  background: linear-gradient(180deg, rgba(15, 23, 42, 0.96), rgba(30, 41, 59, 0.96));
  box-shadow: 0 16px 36px rgba(15, 23, 42, 0.14);
}

.automation-tab__label {
  font-size: 0.95rem;
  font-weight: 700;
}

.automation-tab__description {
  font-size: 0.84rem;
  line-height: 1.45;
  color: rgba(15, 23, 42, 0.68);
}

.automation-tab.is-active .automation-tab__label,
.automation-tab.is-active .automation-tab__description {
  color: rgba(255, 255, 255, 0.96);
}

@media (max-width: 640px) {
  .automation-tabs {
    grid-template-columns: 1fr;
  }
}
</style>
