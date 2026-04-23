<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useChatStore } from '@/stores/chat'
import { useTaskProjectionsStore } from '@/stores/taskProjections'

const { t, te } = useI18n()
const chatStore = useChatStore()
const taskProjections = useTaskProjectionsStore()

const approval = computed(() => chatStore.pendingExecApproval)
const submittingDecision = ref<'deny' | 'allow-once' | 'allow-always' | null>(null)
const isSubmitting = computed(() => submittingDecision.value !== null)
const isCommandApproval = computed(() => approval.value?.type === 'command')
const dialogTitle = computed(() =>
  isCommandApproval.value ? t('execApproval.commandTitle') : t('execApproval.title')
)
const dialogSubtitle = computed(() =>
  isCommandApproval.value ? t('execApproval.commandSubtitle') : t('execApproval.subtitle')
)
const primaryActionLabel = computed(() =>
  isCommandApproval.value ? t('execApproval.allowAlwaysCommand') : t('execApproval.allowAlways')
)
const shouldShowWorkdir = computed(() => {
  const current = approval.value
  if (!current?.workdir) return false
  return current.type === 'command' || current.workdir !== current.directory
})

function resolveText(key: string, fallback: string) {
  return te(key) ? String(t(key)) : fallback
}

const summaryItems = computed(() => {
  const current = approval.value
  if (!current) return []
  return [
    {
      key: 'purpose',
      label: resolveText('execApproval.purpose', 'Purpose'),
      value: current.purpose?.trim() || '',
    },
    {
      key: 'risk',
      label: resolveText('execApproval.risk', 'Risk'),
      value: current.risk_summary?.trim() || '',
    },
    {
      key: 'scope',
      label: resolveText('execApproval.scope', 'Scope'),
      value: current.scope_summary?.trim() || '',
    },
    {
      key: 'effects',
      label: resolveText('execApproval.effects', 'Expected Effects'),
      value: current.expected_effects?.trim() || '',
    },
  ].filter((item) => item.value)
})

const affectedTargets = computed(() =>
  (approval.value?.affected_targets || []).map((target) => target.trim()).filter(Boolean)
)

// Countdown timer
const remainingSeconds = ref(0)
let countdownTimer: ReturnType<typeof setInterval> | null = null

watch(approval, (a) => {
  if (countdownTimer) {
    clearInterval(countdownTimer)
    countdownTimer = null
  }
  if (a) {
    const updateRemaining = () => {
      const ms = a.expires_at - Date.now()
      remainingSeconds.value = Math.max(0, Math.ceil(ms / 1000))
      if (remainingSeconds.value <= 0 && countdownTimer) {
        clearInterval(countdownTimer)
        countdownTimer = null
        chatStore.dismissExecApproval()
      }
    }
    updateRemaining()
    countdownTimer = setInterval(updateRemaining, 1000)
  }
})

async function runDecision(
  decision: 'deny' | 'allow-once' | 'allow-always',
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

function allowOnce() {
  return runDecision('allow-once', () => chatStore.resolveExecApproval('allow-once'))
}
function allowAlways() {
  return runDecision('allow-always', () => chatStore.resolveExecApproval('allow-always'))
}
function deny() {
  return runDecision('deny', () => chatStore.resolveExecApproval('deny'))
}
</script>

<template>
  <Teleport to="body">
    <Transition name="exec-fade">
      <div
        v-if="approval"
        class="fixed inset-0 z-[9999] flex items-center justify-center bg-black/40 backdrop-blur-sm"
      >
        <div
          class="w-full max-w-lg mx-4 rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 shadow-2xl overflow-hidden"
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
            <div class="flex-1 min-w-0">
              <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
                {{ dialogTitle }}
              </h3>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                {{ dialogSubtitle }}
              </p>
            </div>
            <span
              v-if="remainingSeconds > 0"
              class="text-xs text-gray-400 dark:text-gray-500 tabular-nums"
            >
              {{ remainingSeconds }}s
            </span>
          </div>

          <!-- Body -->
          <div class="px-5 py-4 space-y-3">
            <div
              v-if="approval.directory"
              class="space-y-1"
            >
              <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                {{ t('execApproval.directory') }}
              </p>
              <code
                class="block text-sm px-3 py-2 rounded-md bg-gray-100 dark:bg-gray-900 text-red-600 dark:text-red-400 break-all"
              >{{ approval.directory }}</code>
            </div>
            <div
              v-if="shouldShowWorkdir"
              class="space-y-1"
            >
              <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                {{ t('execApproval.workdir') }}
              </p>
              <code
                class="block text-sm px-3 py-2 rounded-md bg-gray-100 dark:bg-gray-900 text-gray-800 dark:text-gray-200 break-all"
              >{{ approval.workdir }}</code>
            </div>
            <div
              v-if="approval.command"
              class="space-y-1"
            >
              <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                {{ t('execApproval.command') }}
              </p>
              <code
                class="block text-sm px-3 py-2 rounded-md bg-gray-100 dark:bg-gray-900 text-gray-800 dark:text-gray-200 whitespace-pre-wrap break-words max-h-32 overflow-y-auto"
              >{{ approval.command }}</code>
            </div>
            <div
              v-if="summaryItems.length > 0"
              class="space-y-2"
            >
              <div
                v-for="item in summaryItems"
                :key="item.key"
                class="rounded-lg border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900/60 px-3 py-2"
              >
                <p class="text-[11px] font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                  {{ item.label }}
                </p>
                <p class="mt-1 text-sm leading-6 text-gray-800 dark:text-gray-200 whitespace-pre-wrap">
                  {{ item.value }}
                </p>
              </div>
            </div>
            <div
              v-if="affectedTargets.length > 0"
              class="space-y-1"
            >
              <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
                {{ resolveText('execApproval.targets', 'Affected Targets') }}
              </p>
              <div class="rounded-lg bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 p-3 space-y-1 max-h-40 overflow-y-auto">
                <p
                  v-for="target in affectedTargets"
                  :key="target"
                  class="font-mono text-xs text-gray-700 dark:text-gray-300 break-all"
                >
                  {{ target }}
                </p>
              </div>
            </div>
            <p
              v-if="isCommandApproval"
              class="text-xs leading-5 text-gray-500 dark:text-gray-400"
            >
              {{ t('execApproval.commandHint') }}
            </p>
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
                  : t('execApproval.deny')
              }}
            </button>
            <button
              :disabled="isSubmitting"
              class="px-4 py-2.5 text-sm font-medium rounded-lg border border-amber-300 dark:border-amber-600 text-amber-700 dark:text-amber-300 hover:bg-amber-50 dark:hover:bg-amber-900/30 transition-colors cursor-pointer disabled:cursor-not-allowed disabled:opacity-60"
              @click="allowOnce"
            >
              {{
                submittingDecision === 'allow-once'
                  ? t('common.processing', 'Processing...')
                  : t('execApproval.allowOnce')
              }}
            </button>
            <button
              :disabled="isSubmitting"
              class="flex-1 px-4 py-2.5 text-sm font-medium rounded-lg bg-amber-600 hover:bg-amber-700 dark:bg-amber-500 dark:hover:bg-amber-600 text-white transition-colors cursor-pointer disabled:cursor-not-allowed disabled:opacity-60"
              @click="allowAlways"
            >
              {{
                submittingDecision === 'allow-always'
                  ? t('common.processing', 'Processing...')
                  : primaryActionLabel
              }}
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.exec-fade-enter-active,
.exec-fade-leave-active {
  transition: opacity 0.2s ease;
}
.exec-fade-enter-from,
.exec-fade-leave-to {
  opacity: 0;
}
</style>
