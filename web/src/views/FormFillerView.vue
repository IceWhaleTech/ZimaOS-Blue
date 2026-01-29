<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  templateApi,
  patternApi,
  FIELD_TYPES,
  type FillTemplate,
  type FieldPatterns,
} from '@/api/formfiller'

const { t } = useI18n()

// State
const templates = ref<FillTemplate[]>([])
const patterns = ref<FieldPatterns | null>(null)
const selectedTemplate = ref<FillTemplate | null>(null)
const isLoading = ref(false)
const error = ref<string | null>(null)
const saveStatus = ref<string | null>(null)

// Dialog state
const showCreateDialog = ref(false)
const showEditDialog = ref(false)
const showPatternDialog = ref(false)
const newTemplateName = ref('')
const editingTemplate = ref<FillTemplate | null>(null)

// Load data
async function loadTemplates() {
  isLoading.value = true
  error.value = null
  try {
    const response = await templateApi.list()
    templates.value = response.data
    // Select default template
    const defaultTemplate = templates.value.find(t => t.is_default)
    if (defaultTemplate) {
      selectedTemplate.value = defaultTemplate
    } else if (templates.value.length > 0) {
      selectedTemplate.value = templates.value[0]
    }
  } catch (e) {
    error.value = 'Failed to load templates'
    console.error(e)
  } finally {
    isLoading.value = false
  }
}

async function loadPatterns() {
  try {
    const response = await patternApi.get()
    patterns.value = response.data
  } catch (e) {
    console.error('Failed to load patterns:', e)
  }
}

// Template operations
async function createTemplate() {
  if (!newTemplateName.value.trim()) return

  try {
    const response = await templateApi.create({
      name: newTemplateName.value.trim(),
      is_default: templates.value.length === 0,
      fields: {},
    })
    templates.value.push(response.data)
    selectedTemplate.value = response.data
    showCreateDialog.value = false
    newTemplateName.value = ''
    showSaveStatus(t('formFiller.templateCreated'))
  } catch (e) {
    error.value = 'Failed to create template'
    console.error(e)
  }
}

async function updateTemplate() {
  if (!editingTemplate.value) return

  try {
    const response = await templateApi.update(editingTemplate.value.id, {
      name: editingTemplate.value.name,
      is_default: editingTemplate.value.is_default,
      fields: editingTemplate.value.fields,
    })
    const index = templates.value.findIndex(t => t.id === response.data.id)
    if (index !== -1) {
      templates.value[index] = response.data
    }
    if (selectedTemplate.value?.id === response.data.id) {
      selectedTemplate.value = response.data
    }
    showEditDialog.value = false
    editingTemplate.value = null
    showSaveStatus(t('formFiller.templateUpdated'))
  } catch (e) {
    error.value = 'Failed to update template'
    console.error(e)
  }
}

async function deleteTemplate(id: string) {
  if (!confirm(t('formFiller.confirmDelete'))) return

  try {
    await templateApi.delete(id)
    templates.value = templates.value.filter(t => t.id !== id)
    if (selectedTemplate.value?.id === id) {
      selectedTemplate.value = templates.value[0] || null
    }
    showSaveStatus(t('formFiller.templateDeleted'))
  } catch (e) {
    error.value = 'Failed to delete template'
    console.error(e)
  }
}

async function setDefaultTemplate(id: string) {
  try {
    await templateApi.update(id, { is_default: true })
    // Reload to get updated default status
    await loadTemplates()
    showSaveStatus(t('formFiller.defaultSet'))
  } catch (e) {
    error.value = 'Failed to set default template'
    console.error(e)
  }
}

async function saveFieldValue(fieldType: string, value: string) {
  if (!selectedTemplate.value) return

  try {
    const updatedFields = { ...selectedTemplate.value.fields, [fieldType]: value }
    const response = await templateApi.update(selectedTemplate.value.id, {
      fields: updatedFields,
    })
    selectedTemplate.value = response.data
    const index = templates.value.findIndex(t => t.id === response.data.id)
    if (index !== -1) {
      templates.value[index] = response.data
    }
  } catch (e) {
    console.error('Failed to save field value:', e)
  }
}

function openEditDialog(template: FillTemplate) {
  editingTemplate.value = { ...template, fields: { ...template.fields } }
  showEditDialog.value = true
}

function showSaveStatus(message: string) {
  saveStatus.value = message
  setTimeout(() => {
    saveStatus.value = null
  }, 2000)
}

// Computed
const sortedFieldTypes = computed(() => {
  return FIELD_TYPES.map(ft => ({
    ...ft,
    value: selectedTemplate.value?.fields[ft.value] || '',
  }))
})

onMounted(() => {
  loadTemplates()
  loadPatterns()
})
</script>

<template>
  <div class="form-filler-view">
    <div class="page-header">
      <h1>{{ t('formFiller.title') }}</h1>
      <p class="subtitle">{{ t('formFiller.subtitle') }}</p>
    </div>

    <!-- Save Status Toast -->
    <div v-if="saveStatus" class="save-status">
      {{ saveStatus }}
    </div>

    <!-- Error Message -->
    <div v-if="error" class="error-message">
      {{ error }}
      <button @click="error = null" class="close-btn">&times;</button>
    </div>

    <!-- Loading State -->
    <div v-if="isLoading" class="loading">
      <div class="spinner"></div>
      <span>{{ t('common.loading') }}</span>
    </div>

    <div v-else class="content-grid">
      <!-- Templates Section -->
      <section class="templates-section">
        <div class="section-header">
          <h2>{{ t('formFiller.templates') }}</h2>
          <button @click="showCreateDialog = true" class="btn btn-primary">
            <span class="icon">+</span>
            {{ t('formFiller.newTemplate') }}
          </button>
        </div>

        <div class="templates-list">
          <div
            v-for="template in templates"
            :key="template.id"
            :class="['template-card', { active: selectedTemplate?.id === template.id }]"
            @click="selectedTemplate = template"
          >
            <div class="template-info">
              <span class="template-name">{{ template.name }}</span>
              <span v-if="template.is_default" class="default-badge">
                {{ t('formFiller.default') }}
              </span>
            </div>
            <div class="template-actions">
              <button
                v-if="!template.is_default"
                @click.stop="setDefaultTemplate(template.id)"
                class="btn btn-sm btn-ghost"
                :title="t('formFiller.setDefault')"
              >
                ★
              </button>
              <button
                @click.stop="openEditDialog(template)"
                class="btn btn-sm btn-ghost"
                :title="t('common.edit')"
              >
                ✎
              </button>
              <button
                @click.stop="deleteTemplate(template.id)"
                class="btn btn-sm btn-ghost btn-danger"
                :title="t('common.delete')"
              >
                ✕
              </button>
            </div>
          </div>

          <div v-if="templates.length === 0" class="empty-state">
            {{ t('formFiller.noTemplates') }}
          </div>
        </div>
      </section>

      <!-- Fields Section -->
      <section class="fields-section" v-if="selectedTemplate">
        <div class="section-header">
          <h2>{{ t('formFiller.fields') }} - {{ selectedTemplate.name }}</h2>
        </div>

        <div class="fields-grid">
          <div v-for="field in sortedFieldTypes" :key="field.label" class="field-item">
            <label :for="field.label">{{ field.label }}</label>
            <input
              :id="field.label"
              type="text"
              :value="selectedTemplate.fields[field.label.toLowerCase().replace(/\s+/g, '')] || ''"
              @blur="(e) => saveFieldValue(field.label.toLowerCase().replace(/\s+/g, ''), (e.target as HTMLInputElement).value)"
              :placeholder="t('formFiller.enterValue')"
            />
          </div>
        </div>
      </section>

      <!-- Patterns Section -->
      <section class="patterns-section">
        <div class="section-header">
          <h2>{{ t('formFiller.patterns') }}</h2>
          <button @click="showPatternDialog = true" class="btn btn-secondary">
            {{ t('formFiller.editPatterns') }}
          </button>
        </div>

        <div class="patterns-info">
          <p>{{ t('formFiller.patternsDescription') }}</p>
          <div v-if="patterns" class="pattern-count">
            {{ Object.keys(patterns.patterns).length }} {{ t('formFiller.fieldTypes') }}
          </div>
        </div>
      </section>
    </div>

    <!-- Create Template Dialog -->
    <div v-if="showCreateDialog" class="dialog-overlay" @click.self="showCreateDialog = false">
      <div class="dialog">
        <h3>{{ t('formFiller.createTemplate') }}</h3>
        <div class="form-group">
          <label for="template-name">{{ t('formFiller.templateName') }}</label>
          <input
            id="template-name"
            v-model="newTemplateName"
            type="text"
            :placeholder="t('formFiller.templateNamePlaceholder')"
            @keyup.enter="createTemplate"
          />
        </div>
        <div class="dialog-actions">
          <button @click="showCreateDialog = false" class="btn btn-secondary">
            {{ t('common.cancel') }}
          </button>
          <button @click="createTemplate" class="btn btn-primary" :disabled="!newTemplateName.trim()">
            {{ t('common.create') }}
          </button>
        </div>
      </div>
    </div>

    <!-- Edit Template Dialog -->
    <div v-if="showEditDialog && editingTemplate" class="dialog-overlay" @click.self="showEditDialog = false">
      <div class="dialog">
        <h3>{{ t('formFiller.editTemplate') }}</h3>
        <div class="form-group">
          <label for="edit-template-name">{{ t('formFiller.templateName') }}</label>
          <input
            id="edit-template-name"
            v-model="editingTemplate.name"
            type="text"
          />
        </div>
        <div class="form-group">
          <label class="checkbox-label">
            <input type="checkbox" v-model="editingTemplate.is_default" />
            {{ t('formFiller.setAsDefault') }}
          </label>
        </div>
        <div class="dialog-actions">
          <button @click="showEditDialog = false" class="btn btn-secondary">
            {{ t('common.cancel') }}
          </button>
          <button @click="updateTemplate" class="btn btn-primary">
            {{ t('common.save') }}
          </button>
        </div>
      </div>
    </div>

    <!-- Pattern Editor Dialog -->
    <div v-if="showPatternDialog && patterns" class="dialog-overlay" @click.self="showPatternDialog = false">
      <div class="dialog dialog-large">
        <h3>{{ t('formFiller.editPatterns') }}</h3>
        <div class="patterns-editor">
          <div v-for="(keywords, fieldType) in patterns.patterns" :key="fieldType" class="pattern-row">
            <label>{{ fieldType }}</label>
            <input
              type="text"
              :value="keywords.join(', ')"
              @blur="(e) => {
                const value = (e.target as HTMLInputElement).value
                patterns!.patterns[fieldType as keyof typeof patterns.patterns] = value.split(',').map(s => s.trim()).filter(Boolean)
              }"
            />
          </div>
        </div>
        <div class="dialog-actions">
          <button @click="showPatternDialog = false" class="btn btn-secondary">
            {{ t('common.close') }}
          </button>
          <button @click="async () => { await patternApi.update(patterns!.patterns); showPatternDialog = false; showSaveStatus(t('formFiller.patternsSaved')); }" class="btn btn-primary">
            {{ t('common.save') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.form-filler-view {
  padding: 24px;
  max-width: 1200px;
  margin: 0 auto;
}

.page-header {
  margin-bottom: 32px;
}

.page-header h1 {
  font-size: 28px;
  font-weight: 600;
  margin: 0 0 8px 0;
}

.subtitle {
  color: var(--text-secondary);
  margin: 0;
}

.save-status {
  position: fixed;
  top: 20px;
  right: 20px;
  background: var(--success-color, #10b981);
  color: white;
  padding: 12px 24px;
  border-radius: 8px;
  z-index: 1000;
  animation: slideIn 0.3s ease;
}

@keyframes slideIn {
  from {
    transform: translateX(100%);
    opacity: 0;
  }
  to {
    transform: translateX(0);
    opacity: 1;
  }
}

.error-message {
  background: var(--error-bg, #fef2f2);
  color: var(--error-color, #dc2626);
  padding: 12px 16px;
  border-radius: 8px;
  margin-bottom: 24px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.close-btn {
  background: none;
  border: none;
  font-size: 20px;
  cursor: pointer;
  color: inherit;
}

.loading {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 48px;
}

.spinner {
  width: 24px;
  height: 24px;
  border: 3px solid var(--border-color);
  border-top-color: var(--primary-color);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.content-grid {
  display: grid;
  gap: 24px;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.section-header h2 {
  font-size: 18px;
  font-weight: 600;
  margin: 0;
}

/* Templates Section */
.templates-section {
  background: var(--card-bg);
  border-radius: 12px;
  padding: 20px;
  border: 1px solid var(--border-color);
}

.templates-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.template-card {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background: var(--bg-secondary);
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
  border: 2px solid transparent;
}

.template-card:hover {
  background: var(--bg-hover);
}

.template-card.active {
  border-color: var(--primary-color);
  background: var(--primary-bg);
}

.template-info {
  display: flex;
  align-items: center;
  gap: 8px;
}

.template-name {
  font-weight: 500;
}

.default-badge {
  font-size: 12px;
  padding: 2px 8px;
  background: var(--primary-color);
  color: white;
  border-radius: 4px;
}

.template-actions {
  display: flex;
  gap: 4px;
}

.empty-state {
  text-align: center;
  padding: 32px;
  color: var(--text-secondary);
}

/* Fields Section */
.fields-section {
  background: var(--card-bg);
  border-radius: 12px;
  padding: 20px;
  border: 1px solid var(--border-color);
}

.fields-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
}

.field-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.field-item label {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-secondary);
}

.field-item input {
  padding: 10px 12px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  font-size: 14px;
  background: var(--input-bg);
  color: var(--text-primary);
}

.field-item input:focus {
  outline: none;
  border-color: var(--primary-color);
  box-shadow: 0 0 0 3px var(--primary-bg);
}

/* Patterns Section */
.patterns-section {
  background: var(--card-bg);
  border-radius: 12px;
  padding: 20px;
  border: 1px solid var(--border-color);
}

.patterns-info {
  color: var(--text-secondary);
}

.pattern-count {
  margin-top: 8px;
  font-weight: 500;
  color: var(--text-primary);
}

/* Buttons */
.btn {
  padding: 8px 16px;
  border-radius: 6px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
  border: none;
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.btn-primary {
  background: var(--primary-color);
  color: white;
}

.btn-primary:hover {
  background: var(--primary-hover);
}

.btn-primary:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-secondary {
  background: var(--bg-secondary);
  color: var(--text-primary);
  border: 1px solid var(--border-color);
}

.btn-secondary:hover {
  background: var(--bg-hover);
}

.btn-sm {
  padding: 4px 8px;
  font-size: 12px;
}

.btn-ghost {
  background: transparent;
  color: var(--text-secondary);
}

.btn-ghost:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.btn-danger:hover {
  color: var(--error-color);
}

.icon {
  font-size: 16px;
}

/* Dialog */
.dialog-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.dialog {
  background: var(--card-bg);
  border-radius: 12px;
  padding: 24px;
  width: 100%;
  max-width: 400px;
  max-height: 90vh;
  overflow-y: auto;
}

.dialog-large {
  max-width: 600px;
}

.dialog h3 {
  margin: 0 0 20px 0;
  font-size: 18px;
  font-weight: 600;
}

.form-group {
  margin-bottom: 16px;
}

.form-group label {
  display: block;
  margin-bottom: 6px;
  font-size: 14px;
  font-weight: 500;
}

.form-group input[type="text"] {
  width: 100%;
  padding: 10px 12px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  font-size: 14px;
  background: var(--input-bg);
  color: var(--text-primary);
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
}

.dialog-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 24px;
}

/* Patterns Editor */
.patterns-editor {
  max-height: 400px;
  overflow-y: auto;
}

.pattern-row {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-bottom: 12px;
}

.pattern-row label {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-secondary);
}

.pattern-row input {
  padding: 8px 12px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  font-size: 13px;
  background: var(--input-bg);
  color: var(--text-primary);
}
</style>
