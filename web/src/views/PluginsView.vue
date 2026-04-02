<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import type { UploadSkillResult, URLSkillInstallResult } from '@/api/skill'
import { SkillTab, SkillStoreTab, ToolTab } from '@/components/extensions'
import SkillContractNotice from '@/components/extensions/SkillContractNotice.vue'
import { useSkillStore } from '@/stores/skill'

const { t } = useI18n()
const skillStore = useSkillStore()
const route = useRoute()

const activeMainTab = ref<'skill' | 'store' | 'tool'>('skill')

const showUploadModal = ref(false)
const uploadType = ref<'skill' | 'plugin'>('skill')
const installMethod = ref<'url' | 'file'>('url')
const installUrl = ref('')
const uploadFile = ref<File | null>(null)
const uploading = ref(false)
const uploadError = ref('')
const lastInstallResult = ref<(URLSkillInstallResult | UploadSkillResult) | null>(null)

const tabs = computed(() => [
  {
    id: 'skill' as const,
    label: t('extensions.skills'),
    description: t('plugins.subtitle'),
    accent: '#38bdf8',
    soft: 'rgba(56, 189, 248, 0.18)',
    icon: 'M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5',
  },
  {
    id: 'store' as const,
    label: t('skillStore.title'),
    description: t('skillStore.subtitle'),
    accent: '#f59e0b',
    soft: 'rgba(245, 158, 11, 0.18)',
    icon: 'M3 3h2l.4 2M7 13h10l4-8H5.4M7 13L5.4 5M7 13l-2.293 2.293c-.63.63-.184 1.707.707 1.707H17m0 0a2 2 0 100 4 2 2 0 000-4zm-8 2a2 2 0 11-4 0 2 2 0 014 0z',
  },
  {
    id: 'tool' as const,
    label: t('extensions.tools'),
    description: t('workspace.desc.tools'),
    accent: '#14b8a6',
    soft: 'rgba(20, 184, 166, 0.18)',
    icon: 'M3 3h7v7H3zM14 3h7v7h-7zM14 14h7v7h-7zM3 14h7v7H3z',
  },
])

const activeTabMeta = computed(
  () => tabs.value.find((tab) => tab.id === activeMainTab.value) ?? tabs.value[0]!
)
const skillStoreInitialQuery = computed(() => {
  const value = route.query.q
  if (typeof value === 'string') return value.trim()
  if (Array.isArray(value)) return String(value[0] || '').trim()
  return ''
})
const installResultSkillName = computed(() => {
  const skill = lastInstallResult.value?.skill
  return skill?.name?.trim() || skill?.id?.trim() || t('plugins.installSkillTitle')
})
const installResultMessage = computed(() => {
  const result = lastInstallResult.value
  if (!result) return ''
  if (result.message?.trim()) return result.message
  if (result.entry_file?.trim()) return `Entry file: ${result.entry_file}`
  return 'Review the contract summary below.'
})

watch(
  () => [route.query.tab, route.query.q],
  ([tabRaw, queryRaw]) => {
    const tab = typeof tabRaw === 'string' ? tabRaw.trim().toLowerCase() : ''
    const query =
      typeof queryRaw === 'string'
        ? queryRaw.trim()
        : Array.isArray(queryRaw)
          ? String(queryRaw[0] || '').trim()
          : ''

    if (tab === 'store' || tab === 'tool' || tab === 'skill') {
      activeMainTab.value = tab
      return
    }
    activeMainTab.value = query ? 'store' : 'skill'
  },
  { immediate: true }
)

function setActiveTab(tabId: 'skill' | 'store' | 'tool') {
  activeMainTab.value = tabId
}

function openUploadModal(type: 'skill' | 'plugin') {
  uploadType.value = type
  installMethod.value = 'url'
  installUrl.value = ''
  uploadFile.value = null
  uploadError.value = ''
  showUploadModal.value = true
}

function closeUploadModal() {
  showUploadModal.value = false
}

function dismissInstallResult() {
  lastInstallResult.value = null
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

async function installSkill() {
  uploading.value = true
  uploadError.value = ''
  lastInstallResult.value = null

  try {
    if (installMethod.value === 'url') {
      if (!installUrl.value.trim()) {
        uploadError.value = t('plugins.urlRequired')
        return
      }
      const result = await skillStore.installFromURL({ url: installUrl.value.trim() })
      if (!result?.success) {
        uploadError.value = result?.message || t('plugins.installFailed')
        return
      }
      lastInstallResult.value = result
    } else {
      if (!uploadFile.value) {
        uploadError.value = t('plugins.fileRequired')
        return
      }
      const result = await skillStore.uploadSkill(uploadFile.value)
      if (!result?.success) {
        uploadError.value = result?.message || t('plugins.installFailed')
        return
      }
      lastInstallResult.value = result
    }
    closeUploadModal()
    installUrl.value = ''
    uploadFile.value = null
  } catch (err) {
    uploadError.value = err instanceof Error ? err.message : t('plugins.installFailed')
  } finally {
    uploading.value = false
  }
}
</script>

<template>
  <div class="plugins-page dashboard-page-frame">
    <section class="plugins-stage dashboard-page-stage configuration-page-stage">
      <section class="plugins-hero dashboard-page-hero configuration-page-hero">
        <div class="hero-copy dashboard-page-copy configuration-page-copy">
          <span class="dashboard-page-eyebrow">{{ t('nav.configuration') }}</span>
          <h1 class="plugins-page-title dashboard-page-title configuration-page-title">
            {{ activeTabMeta.label }}
          </h1>
          <p class="hero-description dashboard-page-description configuration-page-description">
            {{ activeTabMeta.description }}
          </p>
        </div>
      </section>

      <section
        v-if="lastInstallResult?.success"
        class="install-result-banner dashboard-card-surface"
        :data-contract-status="lastInstallResult.contract_status || 'unknown'"
      >
        <div class="install-result-banner__header">
          <div class="install-result-banner__copy">
            <span class="install-result-banner__eyebrow">Install result</span>
            <h2 class="install-result-banner__title">{{ installResultSkillName }}</h2>
            <p class="install-result-banner__message">{{ installResultMessage }}</p>
          </div>

          <button
            type="button"
            class="install-result-banner__dismiss"
            aria-label="Dismiss install result"
            @click="dismissInstallResult"
          >
            ×
          </button>
        </div>

        <SkillContractNotice
          compact
          :contract="lastInstallResult"
          :warnings="lastInstallResult.warnings"
        />
      </section>

      <section class="plugins-shell">
        <nav class="plugins-tab-nav dashboard-card-surface" aria-label="Extension sections">
          <button
            v-for="tab in tabs"
            :key="tab.id"
            type="button"
            role="tab"
            :aria-selected="activeMainTab === tab.id"
            :class="[
              'plugins-tab-button dashboard-card-subsurface',
              { 'plugins-tab-button--active': activeMainTab === tab.id },
            ]"
            @click="setActiveTab(tab.id)"
          >
            <span class="plugins-tab-button__icon">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75">
                <path :d="tab.icon" />
              </svg>
            </span>

            <span class="plugins-tab-button__body">
              <span class="plugins-tab-button__label-row">
                <span class="plugins-tab-button__label">{{ tab.label }}</span>
              </span>
            </span>

            <span class="plugins-tab-button__state" aria-hidden="true"></span>
          </button>
        </nav>
        <section
          class="content-shell dashboard-card-surface"
          :style="{ '--shell-accent': activeTabMeta.accent, '--shell-soft': activeTabMeta.soft }"
        >
          <div class="tab-content">
            <Transition name="tab-fade" mode="out-in">
              <SkillTab
                v-if="activeMainTab === 'skill'"
                key="skill"
                @install-skill="openUploadModal('skill')"
              />
              <SkillStoreTab
                v-else-if="activeMainTab === 'store'"
                key="store"
                :initial-search-query="skillStoreInitialQuery"
              />
              <ToolTab v-else key="tool" />
            </Transition>
          </div>
        </section>
      </section>
    </section>

    <div v-if="showUploadModal" class="modal-overlay" @click.self="closeUploadModal">
      <div class="modal">
        <div class="modal-header">
          <h2>{{ t('plugins.installSkillTitle') }}</h2>
          <button class="modal-close" @click="closeUploadModal">&times;</button>
        </div>

        <div class="modal-body">
          <div class="install-method-tabs">
            <button
              :class="['method-tab', { active: installMethod === 'url' }]"
              @click="installMethod = 'url'"
            >
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path
                  d="M13.828 10.172a4 4 0 0 0-5.656 0l-4 4a4 4 0 1 0 5.656 5.656l1.102-1.101m-.758-4.899a4 4 0 0 0 5.656 0l4-4a4 4 0 0 0-5.656-5.656l-1.1 1.1"
                />
              </svg>
              {{ t('plugins.installFromUrl') }}
            </button>
            <button
              :class="['method-tab', { active: installMethod === 'file' }]"
              @click="installMethod = 'file'"
            >
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
                <polyline points="17,8 12,3 7,8" />
                <line x1="12" y1="3" x2="12" y2="15" />
              </svg>
              {{ t('plugins.uploadFile') }}
            </button>
          </div>

          <div v-if="installMethod === 'url'" class="url-input-section">
            <label>{{ t('plugins.skillUrlLabel') }}</label>
            <input
              v-model="installUrl"
              type="url"
              :placeholder="t('plugins.skillUrlPlaceholder')"
              class="url-input"
              @keyup.enter="installSkill"
            />
            <p class="url-hint">{{ t('plugins.skillUrlHint') }}</p>
          </div>

          <div v-else>
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
              <svg
                v-if="!uploadFile"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="1.5"
              >
                <path
                  d="M12 16.5V9.75m0 0l3 3m-3-3l-3 3M6.75 19.5a4.5 4.5 0 0 1-1.41-8.775 5.25 5.25 0 0 1 10.233-2.33 3 3 0 0 1 3.758 3.848A3.752 3.752 0 0 1 18 19.5H6.75z"
                />
              </svg>
              <svg
                v-else
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                class="file-icon"
              >
                <path d="M9 12l2 2 4-4m6 2a9 9 0 1 1-18 0 9 9 0 0 1 18 0z" />
              </svg>
              <p v-if="!uploadFile">{{ t('plugins.dropFileHere') }}</p>
              <p v-else class="file-name">{{ uploadFile.name }}</p>
              <span class="upload-hint">{{ t('plugins.supportedFormats') }}: .zip, .tar.gz</span>
            </div>
          </div>

          <div v-if="uploadError" class="upload-error">
            {{ uploadError }}
          </div>

          <div class="upload-info">
            <h4>{{ t('plugins.skillPackageInfo') }}</h4>
            <ul>
              <li>{{ t('plugins.skillPackageRequirement1') }}</li>
              <li>{{ t('plugins.skillPackageRequirement2') }}</li>
            </ul>
          </div>
        </div>

        <div class="modal-footer">
          <button class="btn-cancel" @click="closeUploadModal">{{ t('common.cancel') }}</button>
          <button
            class="btn-confirm"
            :disabled="(installMethod === 'url' ? !installUrl.trim() : !uploadFile) || uploading"
            @click="installSkill"
          >
            <span v-if="uploading" class="spinner"></span>
            {{ uploading ? t('plugins.installing') : t('plugins.install') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.plugins-page {
  position: relative;
  isolation: isolate;
  padding: 0 0.5rem 1rem;
  --dashboard-page-accent: 56, 189, 248;
  --bg-primary: #ffffff;
  --bg-secondary: #f8fafc;
  --text-primary: #0f172a;
  --text-secondary: #475569;
  --text-muted: var(--color-text-muted, #64748b);
  --border: rgba(203, 213, 225, 0.84);
  --primary: var(--color-accent, #3b82f6);
  --surface-strong: rgba(255, 255, 255, 0.98);
  --surface-soft: rgba(243, 246, 250, 0.96);
  --surface-muted: rgba(243, 246, 249, 0.92);
  --surface-float: rgba(243, 246, 249, 0.92);
  --white-tint: rgba(255, 255, 255, 0.72);
  --hero-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.9), 0 22px 36px -34px rgba(15, 23, 42, 0.2);
  --shell-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.9), 0 22px 36px -34px rgba(15, 23, 42, 0.2);
}

.plugins-page::before,
.plugins-page::after {
  content: none;
}

.plugins-stage {
  position: relative;
  padding: 0.8rem 0 0.2rem;
}

.plugins-shell {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  width: 100%;
  max-width: 82rem;
  margin: 0 auto;
}

.install-result-banner {
  display: flex;
  flex-direction: column;
  gap: 0.85rem;
  width: 100%;
  max-width: 82rem;
  margin: 0 auto 0.75rem;
  padding: 1rem;
  border: 1px solid rgba(148, 163, 184, 0.18);
  border-radius: 1rem;
  background: var(--surface-strong);
  box-shadow: var(--shell-shadow);
}

.install-result-banner[data-contract-status='strict_contract'] {
  border-color: rgba(110, 231, 183, 0.38);
}

.install-result-banner[data-contract-status='legacy_fallback'] {
  border-color: rgba(251, 191, 36, 0.34);
}

.install-result-banner[data-contract-status='generated_contract'] {
  border-color: rgba(96, 165, 250, 0.34);
}

.install-result-banner__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.install-result-banner__copy {
  min-width: 0;
}

.install-result-banner__eyebrow {
  display: inline-flex;
  margin-bottom: 0.28rem;
  color: var(--text-muted);
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.install-result-banner__title {
  margin: 0;
  color: var(--text-primary);
  font-size: 1rem;
  line-height: 1.25;
}

.install-result-banner__message {
  margin: 0.32rem 0 0;
  color: var(--text-secondary);
  font-size: 0.86rem;
  line-height: 1.5;
}

.install-result-banner__dismiss {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2rem;
  height: 2rem;
  border: 0;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.12);
  color: var(--text-secondary);
  font-size: 1.15rem;
  cursor: pointer;
  transition:
    background-color 0.18s ease,
    color 0.18s ease;
}

.install-result-banner__dismiss:hover {
  background: rgba(148, 163, 184, 0.18);
  color: var(--text-primary);
}

:root.dark .plugins-page,
[data-theme='dark'] .plugins-page,
html.dark .plugins-page {
  --bg-primary: var(--color-bg-elevated, #1e293b);
  --bg-secondary: var(--color-bg-surface, #334155);
  --text-primary: var(--color-text-primary, #f8fafc);
  --text-secondary: var(--color-text-secondary, #94a3b8);
  --text-muted: var(--color-text-muted, #64748b);
  --border: var(--glass-border, rgba(255, 255, 255, 0.12));
  --surface-strong: rgba(30, 41, 59, 0.96);
  --surface-soft: rgba(15, 23, 42, 0.82);
  --surface-muted: rgba(15, 23, 42, 0.82);
  --surface-float: rgba(15, 23, 42, 0.82);
  --white-tint: rgba(255, 255, 255, 0.08);
  --hero-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.05), 0 24px 38px -34px rgba(2, 6, 23, 0.64);
  --shell-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.05), 0 24px 38px -34px rgba(2, 6, 23, 0.64);
}

.plugins-hero {
  position: relative;
  overflow: visible;
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
  width: 100%;
  max-width: 82rem;
  margin: 0 auto;
  padding: 0 0 0.2rem;
  border: 0;
  border-radius: 0;
  background: transparent;
  box-shadow: none;
}

.hero-copy {
  flex: 1;
  min-width: 0;
  max-width: 34rem;
  padding-top: 0;
}

.hero-description {
  margin: 0.28rem 0 0;
  max-width: 28rem;
  color: #9ca3af;
  font-size: 0.78rem;
  line-height: 1.4;
}

.plugins-tab-nav {
  position: relative;
  z-index: 1;
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
  margin-top: 0;
  overflow: hidden;
  padding: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
  box-shadow: none;
}

.plugins-tab-button {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  min-height: 100%;
  border: 1px solid rgba(226, 232, 240, 0.96);
  border-radius: 1rem;
  background: #ffffff;
  color: #0f172a;
  text-align: start;
  cursor: pointer;
  box-shadow: none;
  transition:
    border-color 0.22s ease,
    background-color 0.22s ease,
    color 0.22s ease;
}

.plugins-tab-button:hover {
  border-color: rgba(148, 163, 184, 0.52);
  background: #f8fafc;
}

.plugins-tab-button--active {
  border-color: rgba(148, 163, 184, 0.58);
  background: #f1f5f9;
  color: #0f172a;
  box-shadow: none;
}

.plugins-tab-button__icon {
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

.plugins-tab-button__icon svg {
  width: 1rem;
  height: 1rem;
}

.plugins-tab-button--active .plugins-tab-button__icon {
  background: rgba(148, 163, 184, 0.22);
  color: #475569;
}

.plugins-tab-button__body {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
}

.plugins-tab-button__label-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.3rem;
}

.plugins-tab-button__label {
  font-size: 0.78rem;
  font-weight: 700;
  line-height: 1.25;
}

.plugins-tab-button__state {
  width: 9px;
  height: 9px;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.35);
  transition:
    transform 0.22s ease,
    background-color 0.22s ease;
}

.plugins-tab-button--active .plugins-tab-button__state {
  transform: scale(1.05);
  background: #64748b;
}

.content-shell {
  position: relative;
  margin-top: 0;
  overflow: hidden;
  padding: 0.75rem;
  border-radius: 1rem;
  background: var(--surface-strong);
  box-shadow: var(--shell-shadow);
}

.content-shell::before {
  content: none;
  position: absolute;
  inset: 0 0 auto 0;
  height: 1px;
  background: none;
  opacity: 0.9;
}

.tab-content {
  min-height: 420px;
  position: relative;
}

.tab-fade-enter-active,
.tab-fade-leave-active {
  transition:
    opacity 0.18s ease,
    transform 0.18s ease;
}

.tab-fade-enter-from {
  opacity: 0;
  transform: translateY(10px);
}

.tab-fade-leave-to {
  opacity: 0;
  transform: translateY(-10px);
}

:root.dark .plugins-tab-button,
[data-theme='dark'] .plugins-tab-button,
html.dark .plugins-tab-button {
  border-color: rgba(71, 85, 105, 0.46);
  background: #111827;
  color: #cbd5e1;
  box-shadow: none;
}

:root.dark .plugins-tab-button--active,
[data-theme='dark'] .plugins-tab-button--active,
html.dark .plugins-tab-button--active {
  color: #f8fafc;
  border-color: rgba(148, 163, 184, 0.4);
  background: #1f2937;
}

:root.dark .plugins-tab-button:hover,
[data-theme='dark'] .plugins-tab-button:hover,
html.dark .plugins-tab-button:hover {
  border-color: rgba(148, 163, 184, 0.28);
  background: #1f2937;
}

:root.dark .plugins-tab-button__state,
[data-theme='dark'] .plugins-tab-button__state,
html.dark .plugins-tab-button__state {
  background: rgba(148, 163, 184, 0.32);
}

:root.dark .plugins-tab-button__icon,
[data-theme='dark'] .plugins-tab-button__icon,
html.dark .plugins-tab-button__icon {
  background: rgba(148, 163, 184, 0.18);
  color: #cbd5e1;
}

:root.dark .plugins-tab-button--active .plugins-tab-button__icon,
[data-theme='dark'] .plugins-tab-button--active .plugins-tab-button__icon,
html.dark .plugins-tab-button--active .plugins-tab-button__icon {
  background: rgba(148, 163, 184, 0.24);
  color: #f8fafc;
}

:root.dark .plugins-tab-button--active .plugins-tab-button__state,
[data-theme='dark'] .plugins-tab-button--active .plugins-tab-button__state,
html.dark .plugins-tab-button--active .plugins-tab-button__state {
  background: #cbd5e1;
}

:root.dark .install-result-banner__dismiss,
[data-theme='dark'] .install-result-banner__dismiss,
html.dark .install-result-banner__dismiss {
  background: rgba(148, 163, 184, 0.14);
  color: #cbd5e1;
}

:deep(.skill-tab),
:deep(.skill-store-tab),
:deep(.tool-tab) {
  position: relative;
}

:deep(.overview-grid),
:deep(.stats) {
  margin-bottom: 12px;
}

:deep(.filters) {
  margin-bottom: 12px;
  padding: 10px;
  border-radius: 14px;
  border: 1px solid rgba(148, 163, 184, 0.18);
  background: var(--surface-float);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.08);
}

:deep(.search-input),
:deep(.filter-select),
:deep(.btn-refresh),
:deep(.btn-add-source),
:deep(.overview-item),
:deep(.meta-entry),
:deep(.detail-section),
:deep(.item-card),
:deep(.skill-detail-panel),
:deep(.store-detail-panel),
:deep(.sync-banner),
:deep(.error-banner) {
  border-radius: 16px;
}

:deep(.search-input),
:deep(.filter-select),
:deep(.btn-refresh),
:deep(.btn-add-source) {
  min-height: 34px;
  border-color: rgba(148, 163, 184, 0.18);
  background: var(--surface-float);
}

:deep(.item-card),
:deep(.skill-detail-panel),
:deep(.store-detail-panel) {
  border-color: rgba(148, 163, 184, 0.18);
  background: var(--surface-float);
  box-shadow: 0 22px 42px -36px rgba(15, 23, 42, 0.62);
}

:deep(.item-card:hover) {
  transform: translateY(-6px);
  box-shadow: 0 28px 48px -34px rgba(15, 23, 42, 0.72);
}

:deep(.skill-detail-panel),
:deep(.store-detail-panel) {
  top: 20px;
}

:deep(.detail-section),
:deep(.meta-entry),
:deep(.overview-item) {
  border-color: rgba(148, 163, 184, 0.16);
  background: rgba(255, 255, 255, 0.04);
}

:deep(.sync-banner),
:deep(.error-banner) {
  border-width: 1px;
}

:deep(.item-title-meta),
:deep(.item-tags) {
  row-gap: 6px;
}

:deep(.stats) {
  padding: 10px;
  border: 1px solid rgba(148, 163, 184, 0.18);
  background: var(--surface-float);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.08);
}

.modal-overlay {
  position: fixed;
  inset: 0;
  z-index: 1000;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 14px;
  background: rgba(2, 6, 23, 0.62);
  backdrop-filter: blur(14px);
  -webkit-backdrop-filter: blur(14px);
}

.modal {
  width: min(460px, 100%);
  max-height: calc(100vh - 28px);
  overflow: hidden;
  display: flex;
  flex-direction: column;
  border-radius: 18px;
  border: 1px solid var(--border);
  background: var(--surface-strong);
  box-shadow: 0 44px 120px -58px rgba(15, 23, 42, 0.95);
}

.modal-header,
.modal-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 14px 16px;
}

.modal-header {
  border-bottom: 1px solid rgba(148, 163, 184, 0.18);
}

.modal-header h2 {
  margin: 0;
  font-size: 1rem;
  color: var(--text-primary);
}

.modal-close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  border: 0;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.12);
  color: var(--text-secondary);
  font-size: 18px;
  cursor: pointer;
  transition:
    background 0.2s ease,
    color 0.2s ease;
}

.modal-close:hover {
  background: rgba(148, 163, 184, 0.2);
  color: var(--text-primary);
}

.modal-body {
  overflow: auto;
  padding: 16px;
}

.install-method-tabs {
  display: flex;
  gap: 8px;
  padding: 4px;
  margin-bottom: 14px;
  border-radius: 14px;
  background: rgba(148, 163, 184, 0.12);
}

.method-tab {
  flex: 1;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 8px 10px;
  border: 1px solid transparent;
  border-radius: 10px;
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  transition:
    background 0.2s ease,
    border-color 0.2s ease,
    color 0.2s ease,
    transform 0.2s ease;
}

.method-tab:hover {
  color: var(--text-primary);
}

.method-tab.active {
  border-color: rgba(148, 163, 184, 0.18);
  background: rgba(255, 255, 255, 0.08);
  color: var(--text-primary);
  box-shadow: 0 18px 28px -24px rgba(15, 23, 42, 0.45);
}

.method-tab svg {
  width: 14px;
  height: 14px;
}

.url-input-section {
  margin-bottom: 12px;
}

.url-input-section label {
  display: block;
  margin-bottom: 6px;
  color: var(--text-primary);
  font-size: 12px;
  font-weight: 600;
}

.url-input {
  width: 100%;
  box-sizing: border-box;
  padding: 10px 12px;
  border-radius: 10px;
  border: 1px solid rgba(148, 163, 184, 0.18);
  background: rgba(255, 255, 255, 0.06);
  color: var(--text-primary);
  font-size: 12px;
}

.url-input:focus {
  outline: none;
  border-color: var(--primary);
  box-shadow: 0 0 0 4px rgba(59, 130, 246, 0.16);
}

.url-hint,
.upload-hint {
  margin-top: 6px;
  color: var(--text-muted);
  font-size: 10px;
  line-height: 1.4;
}

.upload-dropzone {
  position: relative;
  padding: 22px 16px;
  text-align: center;
  border-radius: 14px;
  border: 1.5px dashed rgba(148, 163, 184, 0.34);
  background: rgba(255, 255, 255, 0.04);
  transition:
    border-color 0.2s ease,
    background 0.2s ease,
    transform 0.2s ease;
  cursor: pointer;
}

.upload-dropzone:hover {
  transform: translateY(-1px);
  border-color: rgba(56, 189, 248, 0.58);
  background: rgba(56, 189, 248, 0.08);
}

.upload-dropzone.has-file {
  border-color: rgba(34, 197, 94, 0.54);
  background: rgba(34, 197, 94, 0.08);
}

.upload-dropzone svg {
  width: 34px;
  height: 34px;
  margin-bottom: 8px;
  color: var(--text-muted);
}

.upload-dropzone.has-file svg {
  color: #22c55e;
}

.upload-dropzone p {
  margin: 0 0 6px;
  color: var(--text-primary);
  font-size: 12px;
}

.upload-dropzone .file-name {
  color: #22c55e;
  font-weight: 600;
}

.file-input {
  position: absolute;
  inset: 0;
  opacity: 0;
  cursor: pointer;
}

.upload-error {
  margin-top: 10px;
  padding: 10px 12px;
  border-radius: 10px;
  border: 1px solid rgba(239, 68, 68, 0.28);
  background: rgba(239, 68, 68, 0.12);
  color: #f87171;
  font-size: 11px;
}

.upload-info {
  margin-top: 12px;
  padding: 10px;
  border-radius: 12px;
  border: 1px solid rgba(148, 163, 184, 0.16);
  background: rgba(255, 255, 255, 0.04);
}

.upload-info h4 {
  margin: 0 0 8px;
  color: var(--text-primary);
  font-size: 11px;
}

.upload-info ul {
  margin: 0;
  padding-inline-start: 16px;
  color: var(--text-secondary);
  font-size: 10px;
  line-height: 1.45;
}

.modal-footer {
  border-top: 1px solid rgba(148, 163, 184, 0.18);
  justify-content: flex-end;
}

.btn-cancel,
.btn-confirm {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-width: 84px;
  padding: 8px 12px;
  border-radius: 10px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition:
    transform 0.2s ease,
    opacity 0.2s ease,
    background 0.2s ease;
}

.btn-cancel {
  border: 1px solid rgba(148, 163, 184, 0.18);
  background: rgba(148, 163, 184, 0.12);
  color: var(--text-primary);
}

.btn-confirm {
  border: 0;
  background: #0f172a;
  color: white;
}

.btn-cancel:hover,
.btn-confirm:hover:not(:disabled) {
  transform: translateY(-1px);
}

.btn-confirm:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.spinner {
  width: 14px;
  height: 14px;
  border: 2px solid rgba(255, 255, 255, 0.24);
  border-top-color: currentColor;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 1180px) {
  :deep(.skill-layout),
  :deep(.store-layout) {
    grid-template-columns: 1fr;
  }

  :deep(.skill-detail-panel),
  :deep(.store-detail-panel) {
    position: static;
    max-height: none;
  }
}

@media (max-width: 900px) {
  .plugins-tab-nav {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 720px) {
  .plugins-page {
    padding: 0.5rem 0.5rem 1rem;
  }

  .plugins-hero {
    flex-direction: column;
    align-items: stretch;
  }

  .plugins-tab-nav {
    grid-template-columns: 1fr;
  }

  .plugins-tab-button {
    padding: 10px 12px;
  }

  .content-shell {
    padding: 12px;
    border-radius: 1rem;
  }

  .modal-overlay {
    padding: 12px;
  }

  .modal {
    max-height: calc(100vh - 24px);
    border-radius: 16px;
  }

  .modal-header,
  .modal-footer,
  .modal-body {
    padding-inline: 16px;
  }

  .modal-footer {
    flex-direction: column-reverse;
    align-items: stretch;
  }

  .btn-cancel,
  .btn-confirm {
    width: 100%;
  }
}
</style>
