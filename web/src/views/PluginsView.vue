<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { SkillTab, SkillStoreTab, ToolTab } from '@/components/extensions'
import { useSkillStore } from '@/stores/skill'

const { t } = useI18n()
const skillStore = useSkillStore()

// Main tab state
const activeMainTab = ref<'skill' | 'store' | 'tool'>('skill')

// Upload modal state
const showUploadModal = ref(false)
const uploadType = ref<'skill' | 'plugin'>('skill')
const uploadFile = ref<File | null>(null)
const uploading = ref(false)
const uploadError = ref('')

// Tab definitions
const tabs = computed(() => [
  {
    id: 'skill' as const,
    label: t('extensions.skills'),
    icon: 'M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5',
  },
  {
    id: 'store' as const,
    label: t('skillStore.title'),
    icon: 'M3 3h2l.4 2M7 13h10l4-8H5.4M7 13L5.4 5M7 13l-2.293 2.293c-.63.63-.184 1.707.707 1.707H17m0 0a2 2 0 100 4 2 2 0 000-4zm-8 2a2 2 0 11-4 0 2 2 0 014 0z',
  },
  {
    id: 'tool' as const,
    label: t('extensions.tools'),
    icon: 'M3 3h7v7H3zM14 3h7v7h-7zM14 14h7v7h-7zM3 14h7v7H3z',
  },
])

function setActiveTab(tabId: 'skill' | 'store' | 'tool') {
  activeMainTab.value = tabId
}

function openUploadModal(type: 'skill' | 'plugin') {
  uploadType.value = type
  uploadFile.value = null
  uploadError.value = ''
  showUploadModal.value = true
}

function handleFileSelect(event: Event) {
  const input = event.target as HTMLInputElement
  if (input.files && input.files[0]) {
    uploadFile.value = input.files[0]
    uploadError.value = ''
  }
}

function handleDrop(event: DragEvent) {
  event.preventDefault()
  if (event.dataTransfer?.files && event.dataTransfer.files[0]) {
    uploadFile.value = event.dataTransfer.files[0]
    uploadError.value = ''
  }
}

function handleDragOver(event: DragEvent) {
  event.preventDefault()
}

async function uploadSkill() {
  if (!uploadFile.value) return

  uploading.value = true
  uploadError.value = ''

  try {
    await skillStore.uploadSkill(uploadFile.value)
    showUploadModal.value = false
    uploadFile.value = null
  } catch (err) {
    uploadError.value = err instanceof Error ? err.message : t('plugins.uploadFailed')
  } finally {
    uploading.value = false
  }
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
      <div class="header-actions">
        <button class="btn-upload" @click="openUploadModal('skill')">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
            <polyline points="17,8 12,3 7,8" />
            <line x1="12" y1="3" x2="12" y2="15" />
          </svg>
          {{ t('plugins.uploadSkill') }}
        </button>
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
        <SkillStoreTab v-else-if="activeMainTab === 'store'" key="store" />
        <ToolTab v-else key="tool" />
      </Transition>
    </div>

    <!-- Upload Modal -->
    <div v-if="showUploadModal" class="modal-overlay" @click.self="showUploadModal = false">
      <div class="modal">
        <div class="modal-header">
          <h2>{{ uploadType === 'skill' ? t('plugins.uploadSkillTitle') : t('plugins.uploadPluginTitle') }}</h2>
          <button class="modal-close" @click="showUploadModal = false">×</button>
        </div>
        <div class="modal-body">
          <!-- Drop Zone -->
          <div
            class="upload-dropzone"
            :class="{ 'has-file': uploadFile }"
            @drop="handleDrop"
            @dragover="handleDragOver"
          >
            <input
              type="file"
              accept=".zip,.tar.gz,.tgz"
              class="file-input"
              @change="handleFileSelect"
            />
            <svg v-if="!uploadFile" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
              <path d="M12 16.5V9.75m0 0l3 3m-3-3l-3 3M6.75 19.5a4.5 4.5 0 01-1.41-8.775 5.25 5.25 0 0110.233-2.33 3 3 0 013.758 3.848A3.752 3.752 0 0118 19.5H6.75z" />
            </svg>
            <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="file-icon">
              <path d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <p v-if="!uploadFile">{{ t('plugins.dropFileHere') }}</p>
            <p v-else class="file-name">{{ uploadFile.name }}</p>
            <span class="upload-hint">{{ t('plugins.supportedFormats') }}: .zip, .tar.gz</span>
          </div>

          <!-- Error -->
          <div v-if="uploadError" class="upload-error">
            {{ uploadError }}
          </div>

          <!-- Info -->
          <div class="upload-info">
            <h4>{{ t('plugins.skillPackageInfo') }}</h4>
            <ul>
              <li>{{ t('plugins.skillPackageRequirement1') }}</li>
              <li>{{ t('plugins.skillPackageRequirement2') }}</li>
            </ul>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn-cancel" @click="showUploadModal = false">{{ t('common.cancel') }}</button>
          <button
            class="btn-confirm"
            :disabled="!uploadFile || uploading"
            @click="uploadSkill"
          >
            <span v-if="uploading" class="spinner"></span>
            {{ uploading ? t('plugins.uploading') : t('plugins.upload') }}
          </button>
        </div>
      </div>
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

.header-actions {
  display: flex;
  gap: 12px;
}

.btn-upload {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 16px;
  background: linear-gradient(135deg, var(--primary), #6366f1);
  color: white;
  border: none;
  border-radius: 8px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
}

.btn-upload:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(59, 130, 246, 0.4);
}

.btn-upload svg {
  width: 18px;
  height: 18px;
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

/* Modal */
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal {
  background: var(--bg-primary);
  border: 1px solid var(--border);
  border-radius: 16px;
  width: 90%;
  max-width: 500px;
  overflow: hidden;
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border);
}

.modal-header h2 {
  margin: 0;
  font-size: 18px;
  color: var(--text-primary);
}

.modal-close {
  background: none;
  border: none;
  font-size: 24px;
  color: var(--text-secondary);
  cursor: pointer;
}

.modal-body {
  padding: 20px;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  padding: 16px 20px;
  border-top: 1px solid var(--border);
}

.btn-cancel {
  padding: 10px 20px;
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 8px;
  color: var(--text-secondary);
  cursor: pointer;
}

.btn-confirm {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 20px;
  background: linear-gradient(135deg, var(--primary), #6366f1);
  border: none;
  border-radius: 8px;
  color: white;
  font-weight: 500;
  cursor: pointer;
}

.btn-confirm:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.upload-dropzone {
  position: relative;
  border: 2px dashed var(--border);
  border-radius: 12px;
  padding: 32px;
  text-align: center;
  transition: all 0.2s;
  cursor: pointer;
}

.upload-dropzone:hover {
  border-color: var(--primary);
  background: rgba(59, 130, 246, 0.05);
}

.upload-dropzone.has-file {
  border-color: #22c55e;
  background: rgba(34, 197, 94, 0.05);
}

.upload-dropzone svg {
  width: 48px;
  height: 48px;
  color: var(--text-muted);
  margin-bottom: 12px;
}

.upload-dropzone.has-file svg {
  color: #22c55e;
}

.upload-dropzone p {
  margin: 0 0 8px;
  color: var(--text-primary);
  font-size: 14px;
}

.upload-dropzone .file-name {
  color: #22c55e;
  font-weight: 500;
}

.upload-hint {
  font-size: 12px;
  color: var(--text-muted);
}

.file-input {
  position: absolute;
  inset: 0;
  opacity: 0;
  cursor: pointer;
}

.upload-error {
  margin-top: 12px;
  padding: 10px 12px;
  background: rgba(239, 68, 68, 0.1);
  border: 1px solid rgba(239, 68, 68, 0.3);
  border-radius: 8px;
  color: #ef4444;
  font-size: 13px;
}

.upload-info {
  margin-top: 16px;
  padding: 12px;
  background: var(--bg-secondary);
  border-radius: 8px;
}

.upload-info h4 {
  margin: 0 0 8px;
  font-size: 13px;
  color: var(--text-primary);
}

.upload-info ul {
  margin: 0;
  padding-left: 20px;
  font-size: 12px;
  color: var(--text-secondary);
}

.upload-info li {
  margin-bottom: 4px;
}

.spinner {
  width: 16px;
  height: 16px;
  border: 2px solid rgba(255, 255, 255, 0.3);
  border-top-color: white;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}
</style>
