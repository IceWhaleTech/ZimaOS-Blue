<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useCompanionStore } from '@/stores/companion'
import ReplayControls from '@/components/companion/ReplayControls.vue'
import FlowViewer from '@/components/companion/FlowViewer.vue'
import SecurityBadge from '@/components/companion/SecurityBadge.vue'
import type { SessionEvent } from '@/api/companion'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const companionStore = useCompanionStore()

const sessionId = computed(() => route.params.id as string)

// Playback state
const isPlaying = ref(false)
const currentTime = ref(0)
const speed = ref(1)
const currentEventIndex = ref(0)

// Timeline data
const timeline = ref<{ events: SessionEvent[]; duration: number }>({ events: [], duration: 0 })
const highlightedEventId = ref<string | null>(null)

// Playback timer
let playbackTimer: ReturnType<typeof setInterval> | null = null

// Computed
const session = computed(() => companionStore.currentSession)
const events = computed(() => timeline.value.events)
const duration = computed(() => timeline.value.duration)
const currentEvent = computed(() => events.value[currentEventIndex.value] || null)

const formattedDuration = computed(() => {
  if (!session.value) return '0:00'
  const seconds = Math.floor(session.value.duration / 1000)
  const minutes = Math.floor(seconds / 60)
  const remainingSeconds = seconds % 60
  return `${minutes}:${remainingSeconds.toString().padStart(2, '0')}`
})

// Build timeline from events
function buildTimeline() {
  const sessionEvents = companionStore.sessionEvents
  if (sessionEvents.length === 0) {
    timeline.value = { events: [], duration: 0 }
    return
  }

  // Sort events by timestamp
  const sorted = [...sessionEvents].sort(
    (a, b) => new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime()
  )

  const startTime = new Date(sorted[0].timestamp).getTime()
  const endTime = new Date(sorted[sorted.length - 1].timestamp).getTime()

  timeline.value = {
    events: sorted,
    duration: endTime - startTime,
  }
}

// Playback controls
function play() {
  if (currentEventIndex.value >= events.value.length - 1) {
    // Reset to beginning if at end
    currentEventIndex.value = 0
    currentTime.value = 0
  }

  isPlaying.value = true
  startPlayback()
}

function pause() {
  isPlaying.value = false
  stopPlayback()
}

function startPlayback() {
  stopPlayback()

  const interval = 100 // Update every 100ms
  playbackTimer = setInterval(() => {
    if (!isPlaying.value) {
      stopPlayback()
      return
    }

    // Advance time
    currentTime.value += interval * speed.value

    // Check if we've reached the next event
    while (
      currentEventIndex.value < events.value.length - 1 &&
      getEventRelativeTime(events.value[currentEventIndex.value + 1]) <= currentTime.value
    ) {
      currentEventIndex.value++
      highlightedEventId.value = events.value[currentEventIndex.value].id
    }

    // Check if we've reached the end
    if (currentTime.value >= duration.value) {
      currentTime.value = duration.value
      isPlaying.value = false
      stopPlayback()
    }
  }, interval)
}

function stopPlayback() {
  if (playbackTimer) {
    clearInterval(playbackTimer)
    playbackTimer = null
  }
}

function seek(time: number) {
  currentTime.value = time

  // Find the event at this time
  let index = 0
  for (let i = 0; i < events.value.length; i++) {
    if (getEventRelativeTime(events.value[i]) <= time) {
      index = i
    } else {
      break
    }
  }

  currentEventIndex.value = index
  highlightedEventId.value = events.value[index]?.id || null
}

function stepForward() {
  if (currentEventIndex.value < events.value.length - 1) {
    currentEventIndex.value++
    const event = events.value[currentEventIndex.value]
    currentTime.value = getEventRelativeTime(event)
    highlightedEventId.value = event.id
  }
}

function stepBackward() {
  if (currentEventIndex.value > 0) {
    currentEventIndex.value--
    const event = events.value[currentEventIndex.value]
    currentTime.value = getEventRelativeTime(event)
    highlightedEventId.value = event.id
  }
}

function changeSpeed(newSpeed: number) {
  speed.value = newSpeed
}

function getEventRelativeTime(event: SessionEvent): number {
  if (events.value.length === 0) return 0
  const startTime = new Date(events.value[0].timestamp).getTime()
  return new Date(event.timestamp).getTime() - startTime
}

function goBack() {
  router.push('/companion')
}

function selectEvent(eventId: string) {
  const index = events.value.findIndex((e) => e.id === eventId)
  if (index !== -1) {
    currentEventIndex.value = index
    currentTime.value = getEventRelativeTime(events.value[index])
    highlightedEventId.value = eventId
  }
}

// Lifecycle
onMounted(async () => {
  if (sessionId.value) {
    await companionStore.fetchSession(sessionId.value)
    await companionStore.fetchSessionEvents(sessionId.value, { limit: 10000 })
    await companionStore.fetchSessionFlow(sessionId.value)
    buildTimeline()
  }
})

onUnmounted(() => {
  stopPlayback()
  companionStore.clearCurrentSession()
})

watch(
  () => companionStore.sessionEvents,
  () => {
    buildTimeline()
  }
)
</script>

<template>
  <div class="min-h-screen bg-gray-50 dark:bg-gray-900">
    <!-- Header -->
    <div class="bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 px-6 py-4">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-4">
          <button
            class="p-2 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
            @click="goBack"
          >
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
            </svg>
          </button>
          <div>
            <h1 class="text-xl font-semibold text-gray-900 dark:text-white">
              {{ t('companion.replay.title') }}
            </h1>
            <p v-if="session" class="text-sm text-gray-500 dark:text-gray-400">
              {{ session.platform }} · {{ session.userId || t('companion.anonymous') }} · {{ formattedDuration }}
            </p>
          </div>
        </div>

        <div v-if="session" class="flex items-center gap-4">
          <SecurityBadge :threat-level="session.threatLevel" :threat-score="session.threatScore" />
          <span
            :class="[
              'px-3 py-1 text-sm rounded-full',
              session.status === 'active'
                ? 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300'
                : 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400'
            ]"
          >
            {{ t(`companion.status.${session.status}`) }}
          </span>
        </div>
      </div>
    </div>

    <!-- Loading state -->
    <div v-if="companionStore.loadingSession || companionStore.loadingEvents" class="flex items-center justify-center h-96">
      <div class="text-center">
        <svg class="animate-spin w-8 h-8 text-accent mx-auto mb-4" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
        </svg>
        <p class="text-gray-500 dark:text-gray-400">{{ t('common.loading') }}</p>
      </div>
    </div>

    <!-- Main content -->
    <div v-else class="flex flex-col h-[calc(100vh-80px)]">
      <!-- Flow viewer -->
      <div class="flex-1 p-4">
        <div class="bg-white dark:bg-gray-800 rounded-lg shadow-sm h-full overflow-hidden">
          <FlowViewer
            v-if="companionStore.sessionFlow"
            :flow="companionStore.sessionFlow"
            :highlighted-node-id="highlightedEventId"
            @node-click="selectEvent"
          />
          <div v-else class="flex items-center justify-center h-full text-gray-500 dark:text-gray-400">
            {{ t('companion.replay.noEvents') }}
          </div>
        </div>
      </div>

      <!-- Event timeline sidebar -->
      <div class="w-full border-t border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800">
        <div class="p-4">
          <!-- Current event info -->
          <div v-if="currentEvent" class="mb-4 p-3 bg-gray-50 dark:bg-gray-700/50 rounded-lg">
            <div class="flex items-center justify-between mb-2">
              <span class="text-sm font-medium text-gray-900 dark:text-white">
                {{ t(`companion.eventTypes.${currentEvent.eventType}`) }}
              </span>
              <span class="text-xs text-gray-500 dark:text-gray-400">
                {{ new Date(currentEvent.timestamp).toLocaleTimeString() }}
              </span>
            </div>
            <div class="text-xs text-gray-600 dark:text-gray-400">
              <template v-if="currentEvent.message">
                {{ currentEvent.message.direction === 'inbound' ? '←' : '→' }}
                {{ currentEvent.message.content.substring(0, 100) }}{{ currentEvent.message.content.length > 100 ? '...' : '' }}
              </template>
              <template v-else-if="currentEvent.toolCall">
                {{ currentEvent.toolCall.toolName }} ({{ currentEvent.toolCall.status }})
              </template>
              <template v-else-if="currentEvent.llmRequest">
                {{ currentEvent.llmRequest.model }} · {{ currentEvent.llmRequest.totalTokens }} tokens
              </template>
              <template v-else-if="currentEvent.security">
                {{ t(`companion.threatLevel.${currentEvent.security.threatLevel}`) }}
              </template>
            </div>
          </div>

          <!-- Event list (horizontal scroll) -->
          <div class="flex gap-2 overflow-x-auto pb-2 mb-4">
            <button
              v-for="(event, index) in events"
              :key="event.id"
              :class="[
                'flex-shrink-0 px-3 py-2 rounded-lg text-xs transition-colors',
                index === currentEventIndex
                  ? 'bg-accent text-white'
                  : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600'
              ]"
              @click="selectEvent(event.id)"
            >
              <div class="font-medium">{{ t(`companion.eventTypes.${event.eventType}`) }}</div>
              <div class="opacity-75">{{ new Date(event.timestamp).toLocaleTimeString() }}</div>
            </button>
          </div>

          <!-- Replay controls -->
          <ReplayControls
            :duration="duration"
            :current-time="currentTime"
            :is-playing="isPlaying"
            :speed="speed"
            :total-events="events.length"
            :current-event-index="currentEventIndex"
            @play="play"
            @pause="pause"
            @seek="seek"
            @speed-change="changeSpeed"
            @step-forward="stepForward"
            @step-backward="stepBackward"
          />
        </div>
      </div>
    </div>
  </div>
</template>
