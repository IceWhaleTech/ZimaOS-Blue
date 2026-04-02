<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { workspaceApi } from '@/api/workspace'
import type { WorkspaceFile, WorkspaceStats } from '@/api/workspace'
import { getWorkspaceVisibleTokenCount } from '@/utils/workspaceTokenEstimate'

const { t, te } = useI18n()
const emit = defineEmits<{ 'status-change': [msg: string] }>()

const files = ref<WorkspaceFile[]>([])
const stats = ref<WorkspaceStats | null>(null)
const loading = ref(false)
const editingFile = ref<string | null>(null)
const editDraft = ref('')
const saving = ref(false)
const editorRef = ref<HTMLTextAreaElement | null>(null)

// File display config
const fileInfo: Record<string, { icon: string; labelKey: string; descKey: string }> = {
  'SOUL.md': { icon: '🧠', labelKey: 'workspace.label.soul', descKey: 'workspace.desc.soul' },
  'USER.md': { icon: '👤', labelKey: 'workspace.label.user', descKey: 'workspace.desc.user' },
  'IDENTITY.md': {
    icon: '🏷️',
    labelKey: 'workspace.label.identity',
    descKey: 'workspace.desc.identity',
  },
  'MEMORY.md': { icon: '💾', labelKey: 'workspace.label.memory', descKey: 'workspace.desc.memory' },
  'AGENTS.md': { icon: '📋', labelKey: 'workspace.label.agents', descKey: 'workspace.desc.agents' },
  'TOOLS.md': { icon: '🧰', labelKey: 'workspace.label.tools', descKey: 'workspace.desc.tools' },
}

const editableFileNames = new Set(Object.keys(fileInfo))

const visibleTokenCount = computed(() =>
  getWorkspaceVisibleTokenCount(stats.value, editableFileNames)
)

function tr(
  key?: string,
  fallback = '',
  values?: Record<string, string | number>
): string {
  if (!key) return fallback
  if (!te(key)) return fallback
  return values ? t(key, values) : t(key)
}

async function fetchFiles() {
  loading.value = true
  try {
    const [filesRes, statsRes] = await Promise.all([
      workspaceApi.listFiles(),
      workspaceApi.getStats(),
    ])
    files.value = (filesRes.data.files || []).filter((f) => editableFileNames.has(f.name))
    stats.value = statsRes.data
  } catch (e) {
    console.error('Failed to fetch workspace files:', e)
  } finally {
    loading.value = false
  }
}

function startEdit(file: WorkspaceFile) {
  editingFile.value = file.name
  editDraft.value = file.content || ''
  nextTick(() => {
    const el = editorRef.value
    if (el) {
      el.setSelectionRange(0, 0)
      el.focus()
    }
  })
}

function cancelEdit() {
  if (saving.value) return
  editingFile.value = null
  editDraft.value = ''
}

async function saveEdit(name: string) {
  saving.value = true
  try {
    await workspaceApi.putFile(name, editDraft.value)
    const f = files.value.find((f) => f.name === name)
    if (f) {
      f.content = editDraft.value
      f.missing = false
    }
    editingFile.value = null
    emit('status-change', t('workspace.saved', { name }))
    workspaceApi
      .getStats()
      .then((res) => {
        stats.value = res.data
      })
      .catch(() => {})
  } catch (e) {
    console.error('Failed to save workspace file:', e)
  } finally {
    saving.value = false
  }
}

// Escape key to close editor
function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && editingFile.value) {
    cancelEdit()
  }
}

onMounted(() => {
  fetchFiles()
  document.addEventListener('keydown', onKeydown)
})
onUnmounted(() => {
  document.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <div class="space-y-3">
    <!-- Header -->
    <div class="flex items-center justify-between">
      <div>
        <h3 class="text-base font-semibold text-gray-900 dark:text-white">
          {{ t('workspace.title') }}
        </h3>
        <p class="text-sm text-gray-500 dark:text-gray-400 mt-0.5">
          {{ t('workspace.description') }}
        </p>
      </div>
      <span
        v-if="stats"
        class="text-xs px-2 py-1 rounded-full flex-shrink-0"
        :title="tr('workspace.coreTokensHint', 'Counts only the core workspace files shown here.')"
        :class="
          visibleTokenCount > 4096
            ? 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400'
            : 'bg-gray-100 text-gray-500 dark:bg-gray-700 dark:text-gray-400'
        "
      >
        {{
          tr('workspace.coreTokens', `~${visibleTokenCount.toLocaleString()} core-file tokens`, {
            count: visibleTokenCount.toLocaleString(),
          })
        }}
      </span>
    </div>

    <!-- Loading -->
    <div
      v-if="loading && files.length === 0"
      class="text-sm text-gray-500 dark:text-gray-400 py-4 text-center"
    >
      {{ t('common.loading') }}
    </div>

    <!-- Empty state -->
    <div
      v-else-if="files.length === 0"
      class="text-sm text-gray-500 dark:text-gray-400 py-4 text-center"
    >
      {{ t('workspace.noFiles') }}
    </div>

    <!-- Grid + Editor -->
    <div v-else class="relative">
      <!-- Grid of cards -->
      <Transition name="ws-grid">
        <div v-if="!editingFile" class="grid grid-cols-2 sm:grid-cols-3 gap-2">
          <button
            v-for="file in files"
            :key="file.name"
            class="ws-card group flex flex-col items-center gap-1 p-3 rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800/50 hover:border-gray-300 dark:hover:border-gray-600 hover:shadow-sm transition-all duration-150 cursor-pointer text-center"
            @click="startEdit(file)"
          >
            <span class="text-2xl leading-none">{{
              (fileInfo[file.name] || { icon: '📄' }).icon
            }}</span>
            <span class="text-xs font-medium text-gray-900 dark:text-white truncate w-full">{{
              fileInfo[file.name]?.labelKey
                ? tr(fileInfo[file.name]?.labelKey, file.name.replace('.md', ''))
                : file.name.replace('.md', '')
            }}</span>
            <span
              v-if="fileInfo[file.name]"
              class="text-[10px] text-gray-400 dark:text-gray-500 leading-tight"
            >
              {{ tr(fileInfo[file.name]?.descKey, '') }}
            </span>
          </button>
        </div>
      </Transition>

      <!-- Expanded editor -->
      <Transition name="ws-expand">
        <div
          v-if="editingFile"
          class="rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800/80 overflow-hidden"
        >
          <!-- Editor header -->
          <div
            class="flex items-center justify-between px-4 py-2 border-b border-gray-100 dark:border-gray-700"
          >
            <div class="flex items-center gap-2 min-w-0">
              <span class="text-lg flex-shrink-0">{{
                (fileInfo[editingFile] || { icon: '📄' }).icon
              }}</span>
              <span class="text-sm font-medium text-gray-900 dark:text-white truncate">{{
                fileInfo[editingFile]?.labelKey
                  ? tr(fileInfo[editingFile]?.labelKey, editingFile || '')
                  : editingFile
              }}</span>
              <span
                v-if="fileInfo[editingFile]"
                class="text-xs text-gray-400 dark:text-gray-500 hidden sm:inline flex-shrink-0"
              >
                {{ tr(fileInfo[editingFile]?.descKey, '') }}
              </span>
            </div>
            <div class="flex items-center gap-1.5 flex-shrink-0">
              <button
                class="px-3 py-1 text-xs rounded-lg transition-colors bg-gray-800 dark:bg-gray-200 text-white dark:text-gray-900 hover:bg-gray-700 dark:hover:bg-gray-300 disabled:opacity-50"
                :disabled="saving"
                @click="saveEdit(editingFile)"
              >
                {{ saving ? t('common.saving') : t('common.save') }}
              </button>
              <button
                class="px-3 py-1 text-xs rounded-lg transition-colors bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600 disabled:opacity-50"
                :disabled="saving"
                @click="cancelEdit"
              >
                {{ t('common.cancel') }}
              </button>
            </div>
          </div>
          <!-- Textarea -->
          <textarea
            ref="editorRef"
            v-model="editDraft"
            rows="12"
            class="w-full px-4 py-3 text-sm font-mono bg-transparent text-gray-800 dark:text-gray-200 focus:outline-none resize-y min-h-[200px]"
            @keydown.meta.enter="saveEdit(editingFile!)"
            @keydown.ctrl.enter="saveEdit(editingFile!)"
          />
          <div
            class="px-4 py-1.5 text-[10px] text-gray-400 dark:text-gray-500 border-t border-gray-100 dark:border-gray-700 flex justify-between"
          >
            <span>Esc {{ t('common.cancel') }} · ⌘↵ {{ t('common.save') }}</span>
            <span>{{ t('workspace.chars', { count: editDraft.length.toLocaleString() }) }}</span>
          </div>
        </div>
      </Transition>
    </div>
  </div>
</template>

<style scoped>
.ws-card:hover {
  transform: translateY(-1px);
}

/* Grid fade */
.ws-grid-enter-active,
.ws-grid-leave-active {
  transition: opacity 0.15s ease;
}
.ws-grid-enter-from,
.ws-grid-leave-to {
  opacity: 0;
}

/* Editor expand */
.ws-expand-enter-active {
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}
.ws-expand-leave-active {
  transition: all 0.15s cubic-bezier(0.4, 0, 0.2, 1);
}
.ws-expand-enter-from {
  opacity: 0;
  transform: scale(0.96) translateY(-4px);
}
.ws-expand-leave-to {
  opacity: 0;
  transform: scale(0.96);
}
</style>
