<template>
  <div class="a2ui-tabs">
    <div class="tabs-header">
      <button
        v-for="(tab, index) in tabs"
        :key="index"
        class="tab-btn"
        :class="{ active: activeTab === index }"
        @click="activeTab = index"
      >
        {{ tab.label }}
      </button>
    </div>
    <div class="tabs-content">
      <div v-for="(tab, index) in tabs" :key="index" v-show="activeTab === index" class="tab-panel">
        <A2UIComponent
          v-for="child in getTabChildren(index)"
          :key="child.id"
          :component="child"
          :form-data="formData"
          @action="$emit('action', $event)"
          @update:form-data="$emit('update:form-data', $event)"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import type { Component } from '@/api/a2ui'
import A2UIComponent from '../A2UIComponent.vue'

interface Tab {
  label: string
  children?: number[]
}

const props = defineProps<{
  component: Component
  formData: Record<string, unknown>
}>()

defineEmits<{
  (e: 'action', payload: { actionId: string; confirmRequired: boolean; confirmDialog?: { title: string; message: string; confirm_text: string; cancel_text: string } }): void
  (e: 'update:form-data', payload: { key: string; value: unknown }): void
}>()

const activeTab = ref(0)

const tabs = computed(() => {
  const tabsData = props.component.props?.tabs
  if (Array.isArray(tabsData)) {
    return tabsData as Tab[]
  }
  return []
})

function getTabChildren(tabIndex: number): Component[] {
  const tab = tabs.value[tabIndex]
  if (!tab || !props.component.children) return []

  if (tab.children && Array.isArray(tab.children)) {
    return tab.children.map(i => props.component.children![i]).filter(Boolean)
  }

  // If no specific children mapping, distribute children evenly
  const childrenPerTab = Math.ceil(props.component.children.length / tabs.value.length)
  const start = tabIndex * childrenPerTab
  return props.component.children.slice(start, start + childrenPerTab)
}
</script>

<style scoped>
.a2ui-tabs {
  width: 100%;
}

.tabs-header {
  display: flex;
  gap: 0.25rem;
  border-bottom: 1px solid var(--color-border, #e5e7eb);
  margin-bottom: 1rem;
}

.tab-btn {
  padding: 0.75rem 1.25rem;
  border: none;
  background: none;
  cursor: pointer;
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--color-text-muted, #6b7280);
  border-bottom: 2px solid transparent;
  margin-bottom: -1px;
  transition: all 0.2s;
}

.tab-btn:hover {
  color: var(--color-text, #1f2937);
}

.tab-btn.active {
  color: var(--color-primary, #3b82f6);
  border-bottom-color: var(--color-primary, #3b82f6);
}

.tab-panel {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
</style>
