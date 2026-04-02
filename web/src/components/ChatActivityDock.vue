<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { UserTaskActionID, UserTaskProjection } from '@/api/tasks'
import type { StreamUIState } from '@/stores/chat'
import type { TodoChecklistSummary } from '@/utils/todoChecklist'
import UserTaskProjectionCard from '@/components/UserTaskProjectionCard.vue'
import {
  localizeTaskProjectionSubtitle,
  localizeTaskProjectionTitle,
} from '@/utils/taskProjectionText'
import { canOpenTaskConversation } from '@/utils/taskProjectionActions'
import { projectHarnessQuickLinks } from '@/utils/taskProjectionHarness'

const props = withDefaults(
  defineProps<{
    expanded?: boolean
    streamState: StreamUIState
    canStop: boolean
    currentTasks: UserTaskProjection[]
    backgroundTasks: UserTaskProjection[]
    recentOutcome?: UserTaskProjection | null
    todoSummary?: TodoChecklistSummary | null
    todoCollapsed?: boolean
  }>(),
  {
    expanded: true,
    recentOutcome: null,
    todoSummary: null,
    todoCollapsed: false,
  }
)

const emit = defineEmits<{
  'update:expanded': [value: boolean]
  cancel: []
  retry: []
  action: [task: UserTaskProjection, actionId: UserTaskActionID, payload?: unknown]
  open: [task: UserTaskProjection]
  navigate: [href: string]
  'dismiss-outcome': []
  'todo-toggle': []
  'todo-jump': []
}>()

const { t, te } = useI18n()

function tr(key: string, fallback: string): string {
  return te(key) ? String(t(key)) : fallback
}

function toggleExpanded() {
  if (!hasDetails.value) return
  emit('update:expanded', !props.expanded)
}

function stageLabel(task: Pick<UserTaskProjection, 'stage'> | null | undefined): string {
  switch (String(task?.stage || '').trim()) {
    case 'planning':
      return tr('chat.taskStagePlanning', 'Planning')
    case 'working':
      return tr('chat.taskStageWorking', 'Working')
    case 'verifying':
      return tr('chat.taskStageVerifying', 'Verifying')
    case 'waiting_user':
      return tr('chat.taskStageWaiting', 'Waiting')
    case 'completed':
      return tr('chat.taskStageCompleted', 'Completed')
    case 'partial':
      return tr('chat.taskStagePartial', 'Partially passed')
    case 'failed':
      return tr('chat.taskStageFailed', 'Failed')
    case 'cancelled':
      return tr('chat.taskStageCancelled', 'Cancelled')
    default:
      return String(task?.stage || '').trim()
  }
}

function statusTone(task: Pick<UserTaskProjection, 'status' | 'stage'> | null | undefined): string {
  switch (String(task?.status || task?.stage || '').trim()) {
    case 'waiting_user':
      return 'is-warning'
    case 'completed':
      return 'is-success'
    case 'failed':
      return 'is-danger'
    case 'cancelled':
      return 'is-muted'
    default:
      return 'is-running'
  }
}

function taskTitle(task: UserTaskProjection | null | undefined): string {
  if (!task) return ''
  return localizeTaskProjectionTitle(task.title, task.kind, tr)
}

function taskSubtitle(task: UserTaskProjection | null | undefined): string {
  if (!task) return ''
  return localizeTaskProjectionSubtitle(task.subtitle, task.kind, tr)
}

function blockerText(task: Pick<UserTaskProjection, 'blocker'> | null | undefined): string {
  if (!task?.blocker) return ''
  switch (task.blocker.kind) {
    case 'approval':
      return tr('chat.taskWaitingForApproval', 'Waiting for your approval')
    case 'question':
      return tr('chat.taskWaitingForAnswer', 'Waiting for your answer')
    default:
      return String(task.blocker.label || '').trim()
  }
}

function streamPhaseLabel(): string {
  switch (props.streamState.phase) {
    case 'connecting':
      return tr('chat.streamConnecting', 'Connecting')
    case 'streaming':
      return tr('chat.streamStreaming', 'Streaming')
    case 'executing':
      return tr('chat.streamExecuting', 'Executing')
    case 'recovering':
      return tr('chat.streamRecovering', 'Recovering')
    case 'awaiting_confirmation':
      return tr('chat.streamAwaitingConfirmation', 'Waiting')
    case 'interrupted':
      return tr('chat.streamInterrupted', 'Interrupted')
    case 'completed':
      return tr('chat.taskStageCompleted', 'Completed')
    default:
      return tr('chat.waitingThinking', 'Thinking')
  }
}

function taskCountLabel(count: number, singular: string, plural: string): string {
  return `${count} ${count === 1 ? singular : plural}`
}

function subagentChips(task: UserTaskProjection | null | undefined): string[] {
  const summary = task?.subagent_summary
  if (!summary?.total) return []
  const chips = [taskCountLabel(summary.total, 'subagent', 'subagents')]
  if (summary.running) chips.push(taskCountLabel(summary.running, 'running', 'running'))
  if (summary.waiting_user) chips.push(taskCountLabel(summary.waiting_user, 'waiting', 'waiting'))
  if (summary.failed) chips.push(taskCountLabel(summary.failed, 'failed', 'failed'))
  if (summary.completed) chips.push(taskCountLabel(summary.completed, 'completed', 'completed'))
  if (summary.cancelled) chips.push(taskCountLabel(summary.cancelled, 'cancelled', 'cancelled'))
  return chips
}

function outcomePreview(task: UserTaskProjection | null | undefined): string {
  if (!task) return ''
  return String(task.error_preview || task.result_preview || '').trim()
}

const hasStreamStatus = computed(
  () => props.streamState.phase !== 'idle' && props.streamState.phase !== 'completed'
)
const hasPriorityStreamStatus = computed(() =>
  ['interrupted', 'recovering', 'awaiting_confirmation'].includes(props.streamState.phase)
)
const hasStreamActivity = computed(() => hasStreamStatus.value || props.canStop)
const primaryWaitingTask = computed(
  () => props.currentTasks.find((task) => task.status === 'waiting_user' || !!task.blocker) || null
)
const primaryCurrentTask = computed(() => primaryWaitingTask.value || props.currentTasks[0] || null)
const primaryBackgroundTask = computed(() => props.backgroundTasks[0] || null)
const hasDetails = computed(
  () =>
    props.currentTasks.length > 0 ||
    props.backgroundTasks.length > 0 ||
    !!props.recentOutcome ||
    !!props.todoSummary
)

const headerState = computed(() => {
  if (hasPriorityStreamStatus.value) {
    return {
      tone:
        props.streamState.phase === 'interrupted'
          ? 'is-danger'
          : props.streamState.phase === 'awaiting_confirmation'
            ? 'is-warning'
            : 'is-running',
      badge: streamPhaseLabel(),
      title: props.streamState.label || tr('chat.waitingThinking', 'Thinking'),
      subtitle: props.streamState.detail || '',
      meta:
        props.currentTasks.length > 0
          ? taskCountLabel(
              props.currentTasks.length,
              tr('chat.currentTaskSingular', 'current task'),
              tr('chat.currentTaskPlural', 'current tasks')
            )
          : '',
    }
  }

  if (primaryWaitingTask.value) {
    return {
      tone: 'is-warning',
      badge: stageLabel(primaryWaitingTask.value),
      title: taskTitle(primaryWaitingTask.value),
      subtitle: blockerText(primaryWaitingTask.value) || taskSubtitle(primaryWaitingTask.value),
      meta: subagentChips(primaryWaitingTask.value).join(' · '),
    }
  }

  if (primaryCurrentTask.value) {
    return {
      tone: statusTone(primaryCurrentTask.value),
      badge: stageLabel(primaryCurrentTask.value),
      title: taskTitle(primaryCurrentTask.value),
      subtitle: taskSubtitle(primaryCurrentTask.value),
      meta: subagentChips(primaryCurrentTask.value).join(' · '),
    }
  }

  if (primaryBackgroundTask.value) {
    return {
      tone: statusTone(primaryBackgroundTask.value),
      badge: tr('chat.backgroundTasks', 'Background tasks'),
      title: taskTitle(primaryBackgroundTask.value),
      subtitle: taskSubtitle(primaryBackgroundTask.value),
      meta: subagentChips(primaryBackgroundTask.value).join(' · '),
    }
  }

  if (props.recentOutcome) {
    return {
      tone: statusTone(props.recentOutcome),
      badge: tr('chat.recentOutcome', 'Latest result'),
      title: taskTitle(props.recentOutcome),
      subtitle: outcomePreview(props.recentOutcome),
      meta:
        props.recentOutcome.kind === 'research' && props.recentOutcome.research_sources?.length
          ? `${props.recentOutcome.research_sources.length} ${tr('chat.sourcesLabel', 'sources')}`
          : '',
    }
  }

  if (props.todoSummary) {
    return {
      tone: props.todoSummary.allCompleted ? 'is-success' : 'is-running',
      badge: tr('chat.activeTodoTitle', 'Active checklist'),
      title: tr('chat.activeTodoTitle', 'Active checklist'),
      subtitle: tr(
        'chat.activeTodoProgressFallback',
        `${props.todoSummary.completedCount} out of ${props.todoSummary.totalCount} tasks completed`
      )
        .replace('{completed}', String(props.todoSummary.completedCount))
        .replace('{total}', String(props.todoSummary.totalCount)),
      meta: props.todoSummary.items[0]?.text || '',
    }
  }

  if (hasStreamActivity.value) {
    return {
      tone: props.streamState.phase === 'interrupted' ? 'is-danger' : 'is-running',
      badge: streamPhaseLabel(),
      title: props.streamState.label || tr('chat.waitingThinking', 'Thinking'),
      subtitle: props.streamState.detail || '',
      meta:
        props.currentTasks.length > 0
          ? taskCountLabel(
              props.currentTasks.length,
              tr('chat.currentTaskSingular', 'current task'),
              tr('chat.currentTaskPlural', 'current tasks')
            )
          : '',
    }
  }

  return {
    tone: 'is-running',
    badge: tr('chat.activityDock', 'Activity'),
    title: tr('chat.activityDockIdle', 'Ready'),
    subtitle: '',
    meta: '',
  }
})

const canRetry = computed(
  () => props.streamState.phase === 'interrupted' && Boolean(props.streamState.canRetry)
)
const recentOutcomeLinks = computed(() =>
  props.recentOutcome ? projectHarnessQuickLinks(props.recentOutcome, tr) : []
)
const showOutcomeOpen = computed(
  () => !!props.recentOutcome && canOpenTaskConversation(props.recentOutcome)
)
</script>

<template>
  <section
    v-if="hasStreamActivity || hasDetails"
    data-testid="chat-activity-dock"
    class="chat-activity-dock rounded-[1.45rem] border border-slate-200/85 bg-white/95 shadow-xl backdrop-blur-md dark:border-slate-700/80 dark:bg-slate-900/92"
  >
    <div class="chat-activity-dock__header" :class="headerState.tone">
      <button
        type="button"
        data-testid="chat-activity-dock-toggle"
        class="chat-activity-dock__summary"
        :aria-expanded="props.expanded ? 'true' : 'false'"
        @click="toggleExpanded"
      >
        <span class="chat-activity-dock__badge" :class="headerState.tone">{{
          headerState.badge
        }}</span>
        <span class="chat-activity-dock__copy">
          <span class="chat-activity-dock__title">{{ headerState.title }}</span>
          <span v-if="headerState.subtitle" class="chat-activity-dock__subtitle">
            {{ headerState.subtitle }}
          </span>
          <span v-if="headerState.meta" class="chat-activity-dock__meta">{{
            headerState.meta
          }}</span>
        </span>
      </button>

      <div class="chat-activity-dock__actions">
        <button
          v-if="canRetry"
          type="button"
          data-testid="chat-activity-dock-retry"
          class="chat-activity-dock__action"
          @click="emit('retry')"
        >
          {{ tr('common.retry', 'Retry') }}
        </button>
        <button
          v-if="props.canStop"
          type="button"
          data-testid="chat-activity-dock-stop"
          class="chat-activity-dock__action is-danger"
          @click="emit('cancel')"
        >
          {{ tr('chat.stopGenerating', 'Stop generating') }}
        </button>
        <button
          v-if="hasDetails"
          type="button"
          class="chat-activity-dock__action is-icon"
          :aria-label="
            props.expanded ? tr('chat.collapse', 'Collapse') : tr('chat.expand', 'Expand')
          "
          @click="toggleExpanded"
        >
          <svg
            class="h-4 w-4 transition-transform duration-200"
            :class="{ 'rotate-180': props.expanded }"
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
      </div>
    </div>

    <div
      v-if="props.expanded && hasDetails"
      data-testid="chat-activity-dock-details"
      class="chat-activity-dock__body"
    >
      <section
        v-if="props.currentTasks.length > 0"
        data-testid="chat-activity-dock-current"
        class="chat-activity-dock__section"
      >
        <div class="chat-activity-dock__section-head">
          <h3>{{ tr('chat.currentTasks', 'Current tasks') }}</h3>
          <span>{{ props.currentTasks.length }}</span>
        </div>
        <UserTaskProjectionCard
          v-for="task in props.currentTasks"
          :key="`current-${task.id}`"
          :task="task"
          :collapse-by-default="false"
          @action="
            (task, actionId, payload) => {
              emit('action', task, actionId, payload)
            }
          "
          @open="emit('open', $event)"
          @navigate="emit('navigate', $event)"
        />
      </section>

      <section
        v-if="props.backgroundTasks.length > 0"
        data-testid="chat-activity-dock-background"
        class="chat-activity-dock__section"
      >
        <div class="chat-activity-dock__section-head">
          <h3>{{ tr('chat.backgroundTasks', 'Background tasks') }}</h3>
          <span>{{ props.backgroundTasks.length }}</span>
        </div>
        <UserTaskProjectionCard
          v-for="task in props.backgroundTasks"
          :key="`background-${task.id}`"
          :task="task"
          :collapse-by-default="true"
          @action="
            (task, actionId, payload) => {
              emit('action', task, actionId, payload)
            }
          "
          @open="emit('open', $event)"
          @navigate="emit('navigate', $event)"
        />
      </section>

      <section
        v-if="props.recentOutcome"
        data-testid="chat-activity-dock-outcome"
        class="chat-activity-dock__section"
      >
        <div class="chat-activity-dock__section-head">
          <h3>{{ tr('chat.recentOutcome', 'Latest result') }}</h3>
          <span>{{ stageLabel(props.recentOutcome) }}</span>
        </div>
        <article class="chat-activity-dock__outcome">
          <div class="chat-activity-dock__outcome-copy">
            <div class="chat-activity-dock__outcome-title">
              {{ taskTitle(props.recentOutcome) }}
            </div>
            <div
              v-if="taskSubtitle(props.recentOutcome)"
              class="chat-activity-dock__outcome-subtitle"
            >
              {{ taskSubtitle(props.recentOutcome) }}
            </div>
            <div
              v-if="outcomePreview(props.recentOutcome)"
              class="chat-activity-dock__outcome-preview"
            >
              {{ outcomePreview(props.recentOutcome) }}
            </div>
            <div
              v-if="
                props.recentOutcome.kind === 'research' &&
                props.recentOutcome.research_sources?.length
              "
              class="chat-activity-dock__outcome-meta"
            >
              {{
                `${props.recentOutcome.research_sources.length} ${tr('chat.sourcesLabel', 'sources')}`
              }}
            </div>
          </div>
          <div class="chat-activity-dock__outcome-actions">
            <button
              v-if="showOutcomeOpen"
              type="button"
              class="chat-activity-dock__action"
              @click="emit('open', props.recentOutcome)"
            >
              {{ tr('chat.taskBackToConversation', 'Back to task') }}
            </button>
            <button
              v-for="link in recentOutcomeLinks"
              :key="`${props.recentOutcome.id}-${link.id}`"
              type="button"
              class="chat-activity-dock__action"
              @click="emit('navigate', link.href)"
            >
              {{ link.label }}
            </button>
            <button
              type="button"
              data-testid="chat-activity-dock-dismiss-outcome"
              class="chat-activity-dock__action"
              @click="emit('dismiss-outcome')"
            >
              {{ tr('common.dismiss', 'Dismiss') }}
            </button>
          </div>
        </article>
      </section>

      <section
        v-if="props.todoSummary"
        data-testid="chat-activity-dock-todo"
        class="chat-activity-dock__section"
      >
        <div class="chat-activity-dock__section-head">
          <h3>{{ tr('chat.activeTodoTitle', 'Active checklist') }}</h3>
          <span>{{ `${props.todoSummary.completedCount}/${props.todoSummary.totalCount}` }}</span>
        </div>
        <div class="chat-activity-dock__todo">
          <div class="chat-activity-dock__todo-head">
            <button
              type="button"
              data-testid="chat-activity-dock-todo-jump"
              class="chat-activity-dock__action"
              @click="emit('todo-jump')"
            >
              {{ tr('chat.activeTodo.jumpToMessage', 'Jump to checklist message') }}
            </button>
            <button
              type="button"
              data-testid="chat-activity-dock-todo-toggle"
              class="chat-activity-dock__action"
              @click="emit('todo-toggle')"
            >
              {{
                props.todoCollapsed
                  ? tr('chat.activeTodo.expand', 'Expand todo list')
                  : tr('chat.activeTodo.collapse', 'Collapse todo list')
              }}
            </button>
          </div>
          <ol v-if="!props.todoCollapsed" class="chat-activity-dock__todo-list">
            <li
              v-for="(item, index) in props.todoSummary.items"
              :key="`${props.todoSummary.messageId}-${index}`"
              :class="{ 'is-checked': item.checked }"
            >
              <span>{{ index + 1 }}.</span>
              <span>{{ item.text }}</span>
            </li>
          </ol>
        </div>
      </section>
    </div>
  </section>
</template>

<style scoped>
.chat-activity-dock {
  overflow: hidden;
}

.chat-activity-dock__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.95rem 1rem;
}

.chat-activity-dock__summary {
  min-width: 0;
  display: flex;
  align-items: flex-start;
  gap: 0.75rem;
  flex: 1 1 auto;
  text-align: left;
}

.chat-activity-dock__badge {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 2rem;
  padding: 0.3rem 0.7rem;
  border-radius: 999px;
  font-size: 0.72rem;
  font-weight: 700;
  line-height: 1;
}

.chat-activity-dock__badge.is-running {
  background: rgba(219, 234, 254, 0.95);
  color: rgb(29, 78, 216);
}

.chat-activity-dock__badge.is-warning {
  background: rgba(254, 243, 199, 0.95);
  color: rgb(180, 83, 9);
}

.chat-activity-dock__badge.is-success {
  background: rgba(220, 252, 231, 0.95);
  color: rgb(21, 128, 61);
}

.chat-activity-dock__badge.is-danger {
  background: rgba(254, 226, 226, 0.95);
  color: rgb(185, 28, 28);
}

.chat-activity-dock__badge.is-muted {
  background: rgba(226, 232, 240, 0.95);
  color: rgb(71, 85, 105);
}

.chat-activity-dock__copy {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 0.18rem;
}

.chat-activity-dock__title {
  color: rgb(15, 23, 42);
  font-size: 0.95rem;
  font-weight: 700;
  line-height: 1.25rem;
}

.chat-activity-dock__subtitle,
.chat-activity-dock__meta {
  color: rgb(100, 116, 139);
  font-size: 0.78rem;
  line-height: 1.1rem;
}

.chat-activity-dock__actions {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  flex-shrink: 0;
}

.chat-activity-dock__action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid rgba(203, 213, 225, 0.9);
  border-radius: 999px;
  padding: 0.38rem 0.8rem;
  background: rgba(255, 255, 255, 0.94);
  color: rgb(51, 65, 85);
  font-size: 0.76rem;
  font-weight: 600;
  transition:
    background-color 0.18s ease,
    border-color 0.18s ease,
    color 0.18s ease;
}

.chat-activity-dock__action:hover {
  background: rgba(248, 250, 252, 1);
  border-color: rgba(96, 165, 250, 0.46);
  color: rgb(30, 64, 175);
}

.chat-activity-dock__action.is-danger {
  border-color: rgba(248, 113, 113, 0.45);
  color: rgb(220, 38, 38);
  background: rgba(254, 242, 242, 0.96);
}

.chat-activity-dock__action.is-danger:hover {
  background: rgba(254, 226, 226, 1);
  border-color: rgba(248, 113, 113, 0.62);
}

.chat-activity-dock__action.is-icon {
  min-width: 2rem;
  padding-inline: 0.55rem;
}

.chat-activity-dock__body {
  border-top: 1px solid rgba(226, 232, 240, 0.92);
  padding: 0.75rem;
  display: flex;
  flex-direction: column;
  gap: 0.85rem;
  max-height: 28rem;
  overflow-y: auto;
}

.chat-activity-dock__section {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.chat-activity-dock__section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0 0.15rem;
}

.chat-activity-dock__section-head h3 {
  margin: 0;
  color: rgb(30, 41, 59);
  font-size: 0.8rem;
  font-weight: 700;
}

.chat-activity-dock__section-head span {
  color: rgb(100, 116, 139);
  font-size: 0.72rem;
  font-weight: 700;
}

.chat-activity-dock__outcome,
.chat-activity-dock__todo {
  border: 1px solid rgba(226, 232, 240, 0.95);
  border-radius: 1rem;
  background: rgba(248, 250, 252, 0.72);
  padding: 0.8rem 0.9rem;
}

.chat-activity-dock__outcome {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.8rem;
}

.chat-activity-dock__outcome-copy {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 0.28rem;
}

.chat-activity-dock__outcome-title {
  color: rgb(15, 23, 42);
  font-size: 0.88rem;
  font-weight: 700;
}

.chat-activity-dock__outcome-subtitle,
.chat-activity-dock__outcome-meta {
  color: rgb(100, 116, 139);
  font-size: 0.76rem;
}

.chat-activity-dock__outcome-preview {
  color: rgb(51, 65, 85);
  font-size: 0.8rem;
  line-height: 1.25rem;
}

.chat-activity-dock__outcome-actions,
.chat-activity-dock__todo-head {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 0.45rem;
}

.chat-activity-dock__todo {
  display: flex;
  flex-direction: column;
  gap: 0.7rem;
}

.chat-activity-dock__todo-list {
  display: flex;
  flex-direction: column;
  gap: 0.42rem;
  margin: 0;
  padding-left: 1.05rem;
  color: rgb(51, 65, 85);
  font-size: 0.8rem;
}

.chat-activity-dock__todo-list li {
  display: flex;
  gap: 0.45rem;
  line-height: 1.2rem;
}

.chat-activity-dock__todo-list li.is-checked {
  color: rgb(100, 116, 139);
  text-decoration: line-through;
}

:root.dark .chat-activity-dock,
[data-theme='dark'] .chat-activity-dock {
  box-shadow: 0 20px 42px -34px rgba(2, 6, 23, 0.82);
}

:root.dark .chat-activity-dock__title,
[data-theme='dark'] .chat-activity-dock__title,
:root.dark .chat-activity-dock__outcome-title,
[data-theme='dark'] .chat-activity-dock__outcome-title,
:root.dark .chat-activity-dock__section-head h3,
[data-theme='dark'] .chat-activity-dock__section-head h3 {
  color: rgb(226, 232, 240);
}

:root.dark .chat-activity-dock__subtitle,
[data-theme='dark'] .chat-activity-dock__subtitle,
:root.dark .chat-activity-dock__meta,
[data-theme='dark'] .chat-activity-dock__meta,
:root.dark .chat-activity-dock__section-head span,
[data-theme='dark'] .chat-activity-dock__section-head span,
:root.dark .chat-activity-dock__outcome-subtitle,
[data-theme='dark'] .chat-activity-dock__outcome-subtitle,
:root.dark .chat-activity-dock__outcome-meta,
[data-theme='dark'] .chat-activity-dock__outcome-meta,
:root.dark .chat-activity-dock__todo-list,
[data-theme='dark'] .chat-activity-dock__todo-list {
  color: rgb(148, 163, 184);
}

:root.dark .chat-activity-dock__action,
[data-theme='dark'] .chat-activity-dock__action {
  border-color: rgba(71, 85, 105, 0.88);
  background: rgba(15, 23, 42, 0.84);
  color: rgb(226, 232, 240);
}

:root.dark .chat-activity-dock__action:hover,
[data-theme='dark'] .chat-activity-dock__action:hover {
  border-color: rgba(59, 130, 246, 0.46);
  background: rgba(30, 41, 59, 0.9);
  color: rgb(191, 219, 254);
}

:root.dark .chat-activity-dock__action.is-danger,
[data-theme='dark'] .chat-activity-dock__action.is-danger {
  border-color: rgba(127, 29, 29, 0.96);
  background: rgba(69, 10, 10, 0.75);
  color: rgb(254, 202, 202);
}

:root.dark .chat-activity-dock__body,
[data-theme='dark'] .chat-activity-dock__body {
  border-top-color: rgba(51, 65, 85, 0.86);
}

:root.dark .chat-activity-dock__outcome,
[data-theme='dark'] .chat-activity-dock__outcome,
:root.dark .chat-activity-dock__todo,
[data-theme='dark'] .chat-activity-dock__todo {
  border-color: rgba(51, 65, 85, 0.86);
  background: rgba(15, 23, 42, 0.6);
}

:root.dark .chat-activity-dock__outcome-preview,
[data-theme='dark'] .chat-activity-dock__outcome-preview {
  color: rgb(203, 213, 225);
}

@media (max-width: 767px) {
  .chat-activity-dock__header,
  .chat-activity-dock__outcome {
    flex-direction: column;
  }

  .chat-activity-dock__actions,
  .chat-activity-dock__outcome-actions,
  .chat-activity-dock__todo-head {
    width: 100%;
    justify-content: flex-start;
  }
}
</style>
