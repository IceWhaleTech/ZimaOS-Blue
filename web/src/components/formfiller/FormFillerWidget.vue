<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useFormFillerWidget } from '@/composables/useFormFillerWidget'

const { t } = useI18n()

const {
  state,
  canUndo,
  hasClipboardData: _hasClipboardData,
  parsedFieldCount,
  setClipboardData,
  readFromClipboard,
  clearClipboardData,
  fillCurrentField,
  fillAllFields,
  undoLastFill,
  hideWidget,
  selectTemplate,
  togglePasswordVisibility,
  isPasswordRevealed,
  setup,
  cleanup,
} = useFormFillerWidget()

// Local state
const showPasteArea = ref(false)
const pasteText = ref('')
const filledCount = ref(0)
const showFilledMessage = ref(false)

// Drag state
const isDragging = ref(false)
const dragOffset = ref({ x: 0, y: 0 })

// Computed position style
const positionStyle = computed(() => ({
  top: `${state.position.y}px`,
  left: `${state.position.x}px`,
}))

// Get field name for display
const currentFieldName = computed(() => {
  if (!state.focusedElement) return ''
  const el = state.focusedElement
  return el.name || el.id || ('placeholder' in el ? el.placeholder : '') || el.type || 'field'
})

// Check if current field is a password field
const isPasswordField = computed(() => {
  if (!state.focusedElement) return false
  const el = state.focusedElement as HTMLInputElement
  // Check if it's currently a password field OR if it was revealed (type changed to text)
  return el.type === 'password' || state.revealedPasswordFields.has(el)
})

// Check if current password field is revealed
const isCurrentPasswordRevealed = computed(() => {
  if (!state.focusedElement) return false
  return isPasswordRevealed(state.focusedElement as HTMLInputElement)
})

// Toggle password visibility for current field
function handleTogglePassword() {
  if (state.focusedElement) {
    togglePasswordVisibility(state.focusedElement as HTMLInputElement)
  }
}

// Handle paste from textarea
function handlePasteInput() {
  setClipboardData(pasteText.value)
}

// Handle paste from clipboard button
async function handleReadClipboard() {
  await readFromClipboard()
  pasteText.value = state.clipboardData
}

// Fill current field
function handleFillCurrent() {
  fillCurrentField()
}

// Fill all fields
function handleFillAll() {
  const count = fillAllFields()
  filledCount.value = count
  showFilledMessage.value = true
  setTimeout(() => {
    showFilledMessage.value = false
  }, 2000)
}

// Clear and close paste area
function handleClearPaste() {
  pasteText.value = ''
  clearClipboardData()
  showPasteArea.value = false
}

// Prevent widget from stealing focus (except for textarea)
function preventFocusLoss(event: MouseEvent) {
  const target = event.target as HTMLElement
  // Allow focus on textarea for typing
  if (target.tagName === 'TEXTAREA' || target.tagName === 'SELECT') {
    return
  }
  event.preventDefault()
}

// Drag handlers
function startDrag(event: MouseEvent) {
  // Only start drag from header
  if (!(event.target as HTMLElement).closest('.widget-header')) return

  isDragging.value = true
  dragOffset.value = {
    x: event.clientX - state.position.x,
    y: event.clientY - state.position.y,
  }

  document.addEventListener('mousemove', onDrag)
  document.addEventListener('mouseup', stopDrag)
  event.preventDefault()
}

function onDrag(event: MouseEvent) {
  if (!isDragging.value) return

  const newX = event.clientX - dragOffset.value.x
  const newY = event.clientY - dragOffset.value.y

  // Keep widget within viewport
  const maxX = window.innerWidth - 280 // widget width
  const maxY = window.innerHeight - 100 // approximate min height

  state.position.x = Math.max(0, Math.min(newX, maxX))
  state.position.y = Math.max(0, Math.min(newY, maxY))
}

function stopDrag() {
  isDragging.value = false
  document.removeEventListener('mousemove', onDrag)
  document.removeEventListener('mouseup', stopDrag)
}

// Lifecycle
onMounted(() => {
  setup()
})

onUnmounted(() => {
  cleanup()
  document.removeEventListener('mousemove', onDrag)
  document.removeEventListener('mouseup', stopDrag)
})

// Sync paste text with state
watch(() => state.clipboardData, (newVal) => {
  if (newVal !== pasteText.value) {
    pasteText.value = newVal
  }
})
</script>

<template>
  <Teleport to="body">
    <div
      v-if="state.isVisible"
      class="fixed z-[99999] w-[280px] rounded-lg shadow-lg overflow-hidden font-sans text-[13px]
             bg-white dark:bg-gray-800 text-gray-800 dark:text-gray-100
             border border-gray-200 dark:border-gray-700"
      :style="positionStyle"
      @mousedown="preventFocusLoss"
    >
      <!-- Draggable Header -->
      <div
        class="widget-header flex items-center gap-1.5 px-2.5 py-2 bg-blue-500 text-white cursor-move select-none"
        @mousedown="startDrag"
      >
        <span class="text-sm">📝</span>
        <span class="flex-1 font-medium text-[13px]">{{ t('formFiller.widget.title') }}</span>
        <button
          class="w-5 h-5 rounded flex items-center justify-center text-sm leading-none
                 bg-white/20 hover:bg-white/30 border-none text-white cursor-pointer"
          :title="t('common.close')"
          @click="hideWidget"
        >×</button>
      </div>

      <!-- Main Content -->
      <div class="p-2.5">
        <!-- Error Message -->
        <div
          v-if="state.error"
          class="mb-2 px-2 py-1.5 rounded text-xs bg-red-100 dark:bg-red-900/30 text-red-800 dark:text-red-300"
        >
          {{ state.error }}
        </div>

        <!-- Success Message -->
        <div
          v-if="showFilledMessage"
          class="mb-2 px-2 py-1.5 rounded text-xs bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-300"
        >
          {{ t('formFiller.widget.filledFields', { count: filledCount }) }}
        </div>

        <!-- Template Selector (compact) -->
        <div v-if="state.templates.length > 1" class="mb-2">
          <select
            :value="state.selectedTemplate?.id"
            class="w-full px-2 py-1.5 rounded text-xs cursor-pointer
                   bg-white dark:bg-gray-700 text-gray-800 dark:text-gray-100
                   border border-gray-300 dark:border-gray-600"
            @change="(e) => {
              const template = state.templates.find(t => t.id === (e.target as HTMLSelectElement).value)
              if (template) selectTemplate(template)
            }"
          >
            <option v-for="template in state.templates" :key="template.id" :value="template.id">
              {{ template.name }}
            </option>
          </select>
        </div>

        <!-- Paste Area Toggle -->
        <div class="mb-2">
          <button
            v-if="!showPasteArea"
            class="w-full px-2 py-2 rounded text-xs cursor-pointer transition-colors
                   bg-gray-100 dark:bg-gray-700 border border-dashed border-gray-300 dark:border-gray-600
                   text-gray-500 dark:text-gray-400 hover:bg-gray-200 dark:hover:bg-gray-600 hover:text-gray-700 dark:hover:text-gray-200"
            @click="showPasteArea = true"
          >
            📋 {{ t('formFiller.widget.pasteData') }}
          </button>

          <!-- Expanded Paste Area -->
          <div
            v-else
            class="rounded overflow-hidden border border-gray-200 dark:border-gray-600"
          >
            <div
class="flex justify-between items-center px-2 py-1.5 text-[11px]
                        bg-gray-50 dark:bg-gray-700 text-gray-500 dark:text-gray-400">
              <span>{{ t('formFiller.widget.pasteDataHint') }}</span>
              <button
                class="p-0.5 text-sm opacity-70 hover:opacity-100 bg-transparent border-none cursor-pointer"
                :title="t('formFiller.widget.readClipboard')"
                @click="handleReadClipboard"
              >📋</button>
            </div>
            <textarea
              v-model="pasteText"
              :placeholder="t('formFiller.widget.pasteExample')"
              class="w-full p-2 text-[11px] font-mono resize-none outline-none
                     bg-white dark:bg-gray-800 text-gray-800 dark:text-gray-100
                     border-t border-gray-200 dark:border-gray-600
                     placeholder:text-gray-400 dark:placeholder:text-gray-500"
              rows="4"
              @input="handlePasteInput"
            ></textarea>
            <div
class="flex justify-between items-center px-2 py-1
                        bg-gray-50 dark:bg-gray-700 border-t border-gray-200 dark:border-gray-600">
              <span v-if="parsedFieldCount > 0" class="text-[11px] text-emerald-600 dark:text-emerald-400">
                {{ t('formFiller.widget.parsedFields', { count: parsedFieldCount }) }}
              </span>
              <button
                class="px-1.5 py-0.5 text-[11px] bg-transparent border-none cursor-pointer
                       text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200"
                @click="handleClearPaste"
              >
                {{ t('common.clear') }}
              </button>
            </div>
          </div>
        </div>

        <!-- Action Buttons -->
        <div class="flex gap-1.5">
          <button
            class="flex-1 px-2.5 py-2 rounded text-xs font-medium cursor-pointer transition-colors
                   bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-200
                   border border-gray-300 dark:border-gray-600
                   hover:bg-gray-200 dark:hover:bg-gray-600
                   disabled:opacity-50 disabled:cursor-not-allowed"
            :disabled="!state.focusedElement"
            :title="currentFieldName"
            @click="handleFillCurrent"
          >
            {{ t('formFiller.widget.fillThis') }}
          </button>
          <!-- Password visibility toggle -->
          <button
            v-if="isPasswordField"
            class="w-9 px-2 py-2 rounded text-sm cursor-pointer transition-colors
                   bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-200
                   border border-gray-300 dark:border-gray-600
                   hover:bg-gray-200 dark:hover:bg-gray-600"
            :title="isCurrentPasswordRevealed ? t('formFiller.widget.hidePassword') : t('formFiller.widget.showPassword')"
            @click="handleTogglePassword"
          >
            {{ isCurrentPasswordRevealed ? '🙈' : '👁️' }}
          </button>
          <button
            class="flex-1 px-2.5 py-2 rounded text-xs font-medium cursor-pointer transition-colors
                   bg-blue-500 text-white border-none
                   hover:bg-blue-600"
            @click="handleFillAll"
          >
            {{ t('formFiller.widget.fillAll') }}
          </button>
        </div>

        <!-- Undo -->
        <button
          v-if="canUndo"
          class="w-full mt-1.5 px-2 py-1.5 rounded text-[11px] cursor-pointer transition-colors
                 bg-transparent text-gray-500 dark:text-gray-400
                 border border-dashed border-gray-300 dark:border-gray-600
                 hover:bg-gray-50 dark:hover:bg-gray-700 hover:text-gray-700 dark:hover:text-gray-200"
          @click="undoLastFill"
        >
          ↩ {{ t('formFiller.widget.undo') }}
        </button>
      </div>
    </div>
  </Teleport>
</template>
