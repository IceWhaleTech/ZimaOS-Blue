<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useChatStore } from '@/stores/chat'
import { useTaskProjectionsStore } from '@/stores/taskProjections'
import { getLocalizedToolName } from '@/utils/toolLocalization'

const { t, te } = useI18n()
const chatStore = useChatStore()
const taskProjections = useTaskProjectionsStore()

const approval = computed(() => chatStore.pendingApproval)
const submittingDecision = ref<'deny' | 'approve' | 'always-allow' | null>(null)
const isSubmitting = computed(() => submittingDecision.value !== null)

const translatedToolName = computed(() => {
  if (!approval.value?.tool_name) return ''
  return getLocalizedToolName(approval.value.tool_name, t, te)
})

const argsDisplay = computed(() => {
  if (!approval.value?.arguments) return []
  return Object.entries(approval.value.arguments).map(([key, value]) => ({
    key,
    value: typeof value === 'string' ? value : JSON.stringify(value, null, 2),
  }))
})

async function runDecision(
  decision: 'deny' | 'approve' | 'always-allow',
  action: () => Promise<unknown>
) {
  if (isSubmitting.value) return
  submittingDecision.value = decision
  try {
    await action()
    await taskProjections.refreshNow().catch(() => {})
  } finally {
    submittingDecision.value = null
  }
}

function approve() {
  return runDecision('approve', () => chatStore.resolveApproval('approve'))
}

function alwaysAllow() {
  return runDecision('always-allow', () => chatStore.resolveApproval('approve', true))
}

function deny() {
  return runDecision('deny', () => chatStore.resolveApproval('deny'))
}
</script>

<template>
  <Teleport to="body">
    <Transition name="approval-fade">
      <div
        v-if="approval"
        class="fixed inset-0 z-[9999] flex items-center justify-center bg-black/40 backdrop-blur-sm"
      >
        <div
          class="w-full max-w-md mx-4 rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 shadow-2xl overflow-hidden"
        >
          <!-- Header -->
          <div
            class="flex items-center gap-3 px-5 py-4 border-b border-gray-100 dark:border-gray-700 bg-amber-50 dark:bg-amber-900/20"
          >
            <div
              class="flex-shrink-0 w-10 h-10 rounded-full bg-amber-100 dark:bg-amber-800/40 flex items-center justify-center"
            >
              <svg
                class="w-5 h-5 text-amber-600 dark:text-amber-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L4.082 16.5c-.77.833.192 2.5 1.732 2.5z"
                />
              </svg>
            </div>
            <div>
              <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
                {{ t('approval.title', 'Tool Call Approval') }}
              </h3>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('approval.subtitle', 'A tool is requesting permission to execute') }}
              </p>
            </div>
          </div>

          <!-- Body -->
          <div class="px-5 py-4 space-y-3">
            <!-- Tool name -->
            <div class="flex items-center gap-2">
              <span class="text-xs text-gray-500 dark:text-gray-400 uppercase tracking-wide">{{
                t('approval.tool', 'Tool')
              }}</span>
              <span
                class="px-2 py-0.5 text-sm font-mono font-medium rounded bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-200"
              >
                {{ translatedToolName }}
              </span>
            </div>

            <!-- Arguments -->
            <div
              v-if="argsDisplay.length > 0"
              class="space-y-1"
            >
              <span class="text-xs text-gray-500 dark:text-gray-400 uppercase tracking-wide">{{
                t('approval.arguments', 'Arguments')
              }}</span>
              <div
                class="rounded-lg bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 p-3 max-h-48 overflow-y-auto"
              >
                <div
                  v-for="arg in argsDisplay"
                  :key="arg.key"
                  class="flex gap-2 text-xs mb-1 last:mb-0"
                >
                  <span class="font-mono text-blue-600 dark:text-blue-400 flex-shrink-0">{{ arg.key }}:</span>
                  <span
                    class="font-mono text-gray-700 dark:text-gray-300 break-all whitespace-pre-wrap"
                  >{{ arg.value }}</span>
                </div>
              </div>
            </div>
          </div>

          <!-- Actions -->
          <div
            class="flex gap-2 px-5 py-4 border-t border-gray-100 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/50"
          >
            <button
              :disabled="isSubmitting"
              class="px-4 py-2.5 text-sm font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors cursor-pointer disabled:cursor-not-allowed disabled:opacity-60"
              @click="deny"
            >
              {{
                submittingDecision === 'deny'
                  ? t('common.processing', 'Processing...')
                  : t('approval.deny', 'Deny')
              }}
            </button>
            <button
              :disabled="isSubmitting"
              class="flex-1 px-4 py-2.5 text-sm font-medium rounded-lg border border-green-300 dark:border-green-700 text-green-700 dark:text-green-300 hover:bg-green-50 dark:hover:bg-green-900/20 transition-colors cursor-pointer disabled:cursor-not-allowed disabled:opacity-60"
              @click="approve"
            >
              {{
                submittingDecision === 'approve'
                  ? t('common.processing', 'Processing...')
                  : t('approval.allow', 'Allow')
              }}
            </button>
            <button
              :disabled="isSubmitting"
              class="flex-1 px-4 py-2.5 text-sm font-medium rounded-lg bg-green-600 hover:bg-green-700 text-white transition-colors cursor-pointer disabled:cursor-not-allowed disabled:opacity-60"
              @click="alwaysAllow"
            >
              {{
                submittingDecision === 'always-allow'
                  ? t('common.processing', 'Processing...')
                  : t('approval.alwaysAllow', 'Always Allow')
              }}
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.approval-fade-enter-active,
.approval-fade-leave-active {
  transition: opacity 0.2s ease;
}
.approval-fade-enter-from,
.approval-fade-leave-to {
  opacity: 0;
}
</style>
