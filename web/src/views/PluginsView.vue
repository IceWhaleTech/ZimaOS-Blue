<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { SkillTab, ToolTab } from '@/components/extensions'

const { t } = useI18n()

// Main tab state - extensions tab removed (extensions are now native)
const activeMainTab = ref<'skill' | 'tool'>('skill')

// Tab definitions for cleaner template
const tabs = computed(() => [
  {
    id: 'skill' as const,
    label: t('extensions.skills'),
    icon: 'M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5',
  },
  {
    id: 'tool' as const,
    label: t('extensions.tools'),
    icon: 'M3 3h7v7H3zM14 3h7v7h-7zM14 14h7v7h-7zM3 14h7v7H3z',
  },
])

function setActiveTab(tabId: 'skill' | 'tool') {
  activeMainTab.value = tabId
}
</script>

<template>
  <div class="plugins-page">
    <!-- Header -->
    <div class="header">
      <div class="title-section">
        <h1>{{ t('plugins.title') }}</h1>
        <p class="subtitle">{{ t('plugins.subtitle') }}</p>
      </div>
    </div>

    <!-- Extension Tabs -->
    <div class="main-tabs" role="tablist">
      <button
        v-for="tab in tabs"
        :key="tab.id"
        role="tab"
        :aria-selected="activeMainTab === tab.id"
        :class="['main-tab', { active: activeMainTab === tab.id }]"
        @click="setActiveTab(tab.id)"
      >
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path :d="tab.icon" />
        </svg>
        <span>{{ tab.label }}</span>
      </button>
    </div>

    <!-- Tab Content with transition -->
    <div class="tab-content">
      <Transition name="tab-fade" mode="out-in">
        <SkillTab v-if="activeMainTab === 'skill'" key="skill" />
        <ToolTab v-else key="tool" />
      </Transition>
    </div>
  </div>
</template>

<style scoped>
.plugins-page {
  padding: 24px;
  max-width: 1400px;
  margin: 0 auto;
  --bg-primary: var(--color-bg-elevated, #1E293B);
  --bg-secondary: var(--color-bg-surface, #334155);
  --bg-hover: var(--glass-bg-hover, rgba(255, 255, 255, 0.1));
  --text-primary: var(--color-text-primary, #F8FAFC);
  --text-secondary: var(--color-text-secondary, #94A3B8);
  --text-muted: var(--color-text-muted, #64748B);
  --border: var(--glass-border, rgba(255, 255, 255, 0.1));
  --primary: var(--color-accent, #3B82F6);
}

:root.light .plugins-page,
[data-theme="light"] .plugins-page {
  --bg-primary: var(--color-bg-elevated, #FFFFFF);
  --bg-secondary: var(--color-bg-surface, #F1F5F9);
  --bg-hover: var(--glass-bg-hover, rgba(0, 0, 0, 0.05));
  --text-primary: var(--color-text-primary, #0F172A);
  --text-secondary: var(--color-text-secondary, #475569);
  --text-muted: var(--color-text-muted, #64748B);
  --border: var(--glass-border, rgba(0, 0, 0, 0.1));
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 24px;
}

.title-section h1 {
  font-size: 28px;
  font-weight: 600;
  margin: 0;
  color: var(--text-primary);
}

.subtitle {
  color: var(--text-secondary);
  margin: 4px 0 0;
}

.main-tabs {
  display: flex;
  gap: 8px;
  margin-bottom: 24px;
  padding: 8px;
  background: var(--glass-bg, rgba(255, 255, 255, 0.05));
  border: 1px solid var(--border);
  border-radius: 12px;
}

.main-tab {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 12px 20px;
  border: none;
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  font-size: 14px;
  font-weight: 500;
  border-radius: 8px;
  transition: all 0.2s;
}

.main-tab svg {
  width: 20px;
  height: 20px;
}

.main-tab:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.main-tab.active {
  background: linear-gradient(135deg, var(--primary), #6366f1);
  color: white;
  box-shadow: 0 4px 12px rgba(59, 130, 246, 0.3);
}

.tab-content {
  min-height: 400px;
  position: relative;
}

/* Tab transition animations */
.tab-fade-enter-active,
.tab-fade-leave-active {
  transition: opacity 0.15s ease, transform 0.15s ease;
}

.tab-fade-enter-from {
  opacity: 0;
  transform: translateY(8px);
}

.tab-fade-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}
</style>
