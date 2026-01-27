<template>
  <component
    :is="componentMap[component.type] || 'A2UIUnknown'"
    :component="component"
    :form-data="formData"
    @action="$emit('action', $event.actionId, $event.confirmRequired, $event.confirmDialog)"
    @update:form-data="$emit('update:form-data', $event.key, $event.value)"
  />
</template>

<script setup lang="ts">
import type { Component, ComponentType } from '@/api/a2ui'
import A2UIText from './components/A2UIText.vue'
import A2UIButton from './components/A2UIButton.vue'
import A2UIInput from './components/A2UIInput.vue'
import A2UISelect from './components/A2UISelect.vue'
import A2UICheckbox from './components/A2UICheckbox.vue'
import A2UISlider from './components/A2UISlider.vue'
import A2UIImage from './components/A2UIImage.vue'
import A2UICard from './components/A2UICard.vue'
import A2UIList from './components/A2UIList.vue'
import A2UITable from './components/A2UITable.vue'
import A2UIForm from './components/A2UIForm.vue'
import A2UIProgress from './components/A2UIProgress.vue'
import A2UIAlert from './components/A2UIAlert.vue'
import A2UICode from './components/A2UICode.vue'
import A2UIMarkdown from './components/A2UIMarkdown.vue'
import A2UIContainer from './components/A2UIContainer.vue'
import A2UIGrid from './components/A2UIGrid.vue'
import A2UITabs from './components/A2UITabs.vue'
import A2UIAccordion from './components/A2UIAccordion.vue'
import A2UIUnknown from './components/A2UIUnknown.vue'

defineProps<{
  component: Component
  formData: Record<string, unknown>
}>()

defineEmits<{
  (e: 'action', payload: { actionId: string; confirmRequired: boolean; confirmDialog?: { title: string; message: string; confirm_text: string; cancel_text: string } }): void
  (e: 'update:form-data', payload: { key: string; value: unknown }): void
}>()

const componentMap: Record<ComponentType, unknown> = {
  text: A2UIText,
  button: A2UIButton,
  input: A2UIInput,
  select: A2UISelect,
  checkbox: A2UICheckbox,
  radio: A2UICheckbox, // Radio uses same component with different mode
  slider: A2UISlider,
  image: A2UIImage,
  card: A2UICard,
  list: A2UIList,
  table: A2UITable,
  chart: A2UIUnknown, // Chart requires additional library
  form: A2UIForm,
  progress: A2UIProgress,
  alert: A2UIAlert,
  code: A2UICode,
  markdown: A2UIMarkdown,
  container: A2UIContainer,
  grid: A2UIGrid,
  tabs: A2UITabs,
  accordion: A2UIAccordion,
}
</script>
