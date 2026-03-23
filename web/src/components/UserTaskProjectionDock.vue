<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { UserTaskProjection } from '@/api/tasks'
import {
  localizeTaskProjectionSubtitle,
  localizeTaskProjectionTitle,
} from '@/utils/taskProjectionText'

const TASK_DOCK_COLLAPSED_KEY = 'zima.chat.task_projection_dock_collapsed.v1'

const props = defineProps<{
  tasks: UserTaskProjection[]
}>()

const emit = defineEmits<{
  open: [task: UserTaskProjection]
  cancel: [taskId: string]
}>()

const { t } = useI18n()
const collapsed = ref(loadCollapsedState())

const leadTask = computed(() => props.tasks[0] || null)

function loadCollapsedState(): boolean {
  try {
    return localStorage.getItem(TASK_DOCK_COLLAPSED_KEY) !== '0'
  } catch {
    return true
  }
}

function persistCollapsedState(value: boolean) {
  try {
    if (value) {
      localStorage.removeItem(TASK_DOCK_COLLAPSED_KEY)
      return
    }
    localStorage.setItem(TASK_DOCK_COLLAPSED_KEY, '0')
  } catch {
    // Ignore storage errors
  }
}

function stageLabel(stage?: string) {
  switch (String(stage || '').trim()) {
    case 'planning':
      return t('chat.taskStagePlanning', 'Planning')
    case 'working':
      return t('chat.taskStageWorking', 'Working')
    case 'verifying':
      return t('chat.taskStageVerifying', 'Verifying')
    case 'waiting_user':
      return t('chat.taskStageWaiting', 'Waiting')
    default:
      return stage || t('chat.taskRunningElsewhere', 'Running')
  }
}

function taskTitle(task: UserTaskProjection): string {
  return localizeTaskProjectionTitle(task.title, task.kind, t)
}

function taskSubtitle(task: UserTaskProjection): string {
  return localizeTaskProjectionSubtitle(task.subtitle, task.kind, t)
}

watch(
  () => props.tasks.length,
  (count) => {
    if (count === 0) collapsed.value = true
  }
)

watch(collapsed, (value) => {
  persistCollapsedState(value)
})
</script>

<template>
  <div data-testid="task-projection-dock" class="relative z-10 w-full min-w-0 h-12 -mb-1">
    <section
      class="absolute inset-x-0 bottom-0 overflow-hidden rounded-[1.25rem] border border-slate-200/80 bg-white/95 shadow-xl backdrop-blur-md dark:border-slate-700/80 dark:bg-slate-900/90"
    >
      <button
        type="button"
        class="flex min-h-12 w-full items-start gap-3 bg-slate-50/80 px-4 py-3 text-left hover:bg-slate-100/80 dark:bg-slate-950/60 dark:hover:bg-slate-900/70"
        :aria-expanded="!collapsed"
        @click="collapsed = !collapsed"
      >
        <span
          class="mt-0.5 inline-flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full bg-blue-100 text-xs font-semibold text-blue-700 dark:bg-blue-900/40 dark:text-blue-200"
        >
          {{ props.tasks.length }}
        </span>

        <span class="min-w-0 flex-1">
          <span class="flex min-w-0 items-center gap-2">
            <span class="truncate text-sm font-semibold text-slate-800 dark:text-slate-100">
              {{ t('chat.backgroundTasks', 'Background tasks') }}
            </span>
          </span>

          <span
            class="mt-1 flex min-w-0 flex-wrap items-center gap-2 text-xs text-slate-500 dark:text-slate-400"
          >
            <span
              v-if="leadTask"
              class="rounded-full bg-blue-100 px-2.5 py-1 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200"
            >
              {{ stageLabel(leadTask.stage) }}
            </span>
            <span v-if="leadTask">{{ Math.round(leadTask.progress || 0) }}%</span>
            <span class="min-w-0 flex-1 truncate">
              {{
                (leadTask ? taskTitle(leadTask) : '') ||
                t('chat.taskRunningElsewhere', 'Track active work running in other conversations.')
              }}
            </span>
          </span>
        </span>

        <svg
          class="mt-1 h-4 w-4 flex-shrink-0 text-slate-400 transition-transform duration-200"
          :class="{ 'rotate-180': !collapsed }"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M19 9l-7 7-7-7"
          />
        </svg>
      </button>

      <div
        v-if="!collapsed"
        class="max-h-72 space-y-2 overflow-y-auto border-t border-slate-100 px-3 py-3 dark:border-slate-800"
      >
        <article
          v-for="task in props.tasks"
          :key="task.id"
          class="rounded-2xl border border-slate-200 bg-white px-3 py-3 dark:border-slate-800 dark:bg-slate-950/60"
        >
          <div class="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
            <div class="min-w-0 flex-1">
              <div
                class="flex flex-wrap items-center gap-2 text-xs text-slate-500 dark:text-slate-400"
              >
                <span
                  class="rounded-full bg-blue-100 px-2.5 py-1 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200"
                >
                  {{ stageLabel(task.stage) }}
                </span>
                <span>{{ Math.round(task.progress || 0) }}%</span>
                <span>{{
                  task.kind === 'research'
                    ? t('chat.taskKindResearch', 'Research')
                    : t('chat.taskKindAgent', 'Agent')
                }}</span>
              </div>
              <div class="mt-2 break-words text-sm font-medium text-slate-800 dark:text-slate-100">
                {{ taskTitle(task) }}
              </div>
              <div v-if="taskSubtitle(task)" class="mt-1 text-xs text-slate-500 dark:text-slate-400">
                {{ taskSubtitle(task) }}
              </div>
              <div class="mt-3 h-1.5 overflow-hidden rounded-full bg-slate-100 dark:bg-slate-800">
                <div
                  class="h-full bg-blue-500 transition-all duration-300"
                  :style="{
                    width: `${Math.max(0, Math.min(100, Math.round(task.progress || 0)))}%`,
                  }"
                />
              </div>
            </div>

            <div class="flex items-center gap-2 md:flex-col md:items-end">
              <button
                v-if="task.actions.can_open_chat"
                type="button"
                class="rounded-full border border-slate-200 px-3 py-1.5 text-xs font-medium text-slate-700 hover:bg-slate-100 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800"
                @click="emit('open', task)"
              >
                {{ t('chat.taskBackToConversation', 'Back to task') }}
              </button>
              <button
                v-if="task.actions.can_cancel"
                type="button"
                class="rounded-full border border-amber-200 px-3 py-1.5 text-xs font-medium text-amber-700 hover:bg-amber-50 dark:border-amber-900/60 dark:text-amber-200 dark:hover:bg-amber-950/30"
                @click="emit('cancel', task.id)"
              >
                {{ t('chat.taskCancel', 'Cancel') }}
              </button>
            </div>
          </div>
        </article>
      </div>
    </section>
  </div>
</template>
