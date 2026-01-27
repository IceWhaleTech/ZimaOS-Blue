<template>
  <div class="a2ui-accordion">
    <div
      v-for="(section, index) in sections"
      :key="index"
      class="accordion-item"
      :class="{ expanded: expandedSections.includes(index) }"
    >
      <button class="accordion-header" @click="toggleSection(index)">
        <span class="accordion-title">{{ section.title }}</span>
        <span class="accordion-icon">{{ expandedSections.includes(index) ? '−' : '+' }}</span>
      </button>
      <div v-show="expandedSections.includes(index)" class="accordion-content">
        <A2UIComponent
          v-for="child in getSectionChildren(index)"
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

interface Section {
  title: string
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

const expandedSections = ref<number[]>([0])
const allowMultiple = computed(() => props.component.props?.allowMultiple as boolean ?? true)

const sections = computed(() => {
  const sectionsData = props.component.props?.sections
  if (Array.isArray(sectionsData)) {
    return sectionsData as Section[]
  }
  return []
})

function toggleSection(index: number) {
  const idx = expandedSections.value.indexOf(index)
  if (idx > -1) {
    expandedSections.value.splice(idx, 1)
  } else {
    if (allowMultiple.value) {
      expandedSections.value.push(index)
    } else {
      expandedSections.value = [index]
    }
  }
}

function getSectionChildren(sectionIndex: number): Component[] {
  const section = sections.value[sectionIndex]
  if (!section || !props.component.children) return []

  if (section.children && Array.isArray(section.children)) {
    return section.children
      .map(i => props.component.children![i])
      .filter((c): c is Component => c !== undefined)
  }

  // If no specific children mapping, distribute children evenly
  const childrenPerSection = Math.ceil(props.component.children.length / sections.value.length)
  const start = sectionIndex * childrenPerSection
  return props.component.children.slice(start, start + childrenPerSection)
}
</script>

<style scoped>
.a2ui-accordion {
  width: 100%;
  border: 1px solid var(--color-border, #e5e7eb);
  border-radius: 0.5rem;
  overflow: hidden;
}

.accordion-item {
  border-bottom: 1px solid var(--color-border, #e5e7eb);
}

.accordion-item:last-child {
  border-bottom: none;
}

.accordion-header {
  width: 100%;
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem;
  border: none;
  background: var(--color-background, white);
  cursor: pointer;
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--color-text, #1f2937);
  transition: background 0.2s;
}

.accordion-header:hover {
  background: var(--color-background-soft, #f9fafb);
}

.accordion-icon {
  font-size: 1.25rem;
  color: var(--color-text-muted, #6b7280);
}

.accordion-content {
  padding: 1rem;
  padding-top: 0;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.accordion-item.expanded .accordion-header {
  background: var(--color-background-soft, #f9fafb);
}
</style>
