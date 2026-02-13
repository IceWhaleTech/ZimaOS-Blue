<script setup lang="ts">
import { useNotificationStore, type NotificationType } from '@/stores/notification'

const notificationStore = useNotificationStore()

function getIcon(type: NotificationType): string {
  switch (type) {
    case 'success':
      return 'M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z'
    case 'error':
      return 'M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z'
    case 'warning':
      return 'M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z'
    case 'info':
      return 'M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z'
  }
}

function getColorClasses(type: NotificationType): string {
  switch (type) {
    case 'success':
      return 'bg-green-50 dark:bg-green-900/30 border-green-200 dark:border-green-800 text-green-800 dark:text-green-200'
    case 'error':
      return 'bg-red-50 dark:bg-red-900/30 border-red-200 dark:border-red-800 text-red-800 dark:text-red-200'
    case 'warning':
      return 'bg-yellow-50 dark:bg-yellow-900/30 border-yellow-200 dark:border-yellow-800 text-yellow-800 dark:text-yellow-200'
    case 'info':
      return 'bg-gray-700 dark:bg-gray-500/30 border-gray-900 dark:border-white dark:border-gray-900 dark:border-white text-gray-900 dark:text-white dark:text-white'
  }
}

function getIconColorClass(type: NotificationType): string {
  switch (type) {
    case 'success':
      return 'text-green-500 dark:text-green-400'
    case 'error':
      return 'text-red-500 dark:text-red-400'
    case 'warning':
      return 'text-yellow-500 dark:text-yellow-400'
    case 'info':
      return 'text-gray-900 dark:text-white dark:text-white'
  }
}
</script>

<template>
  <Teleport to="body">
    <div
      class="fixed top-4 right-4 z-50 flex flex-col gap-3 max-w-sm w-full pointer-events-none"
      aria-live="polite"
    >
      <TransitionGroup name="notification">
        <div
          v-for="notification in notificationStore.notifications"
          :key="notification.id"
          class="pointer-events-auto rounded-lg border shadow-lg p-4 transition-all duration-300"
          :class="getColorClasses(notification.type)"
          role="alert"
        >
          <div class="flex items-start gap-3">
            <!-- Icon -->
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-5 w-5 flex-shrink-0 mt-0.5"
              :class="getIconColorClass(notification.type)"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                :d="getIcon(notification.type)"
              />
            </svg>

            <!-- Content -->
            <div class="flex-1 min-w-0">
              <p class="font-medium text-sm">{{ notification.title }}</p>
              <p v-if="notification.message" class="mt-1 text-sm opacity-80">
                {{ notification.message }}
              </p>

              <!-- Action button -->
              <button
                v-if="notification.action"
                class="mt-2 text-sm font-medium underline hover:no-underline"
                @click="notification.action.handler"
              >
                {{ notification.action.label }}
              </button>
            </div>

            <!-- Dismiss button -->
            <button
              v-if="notification.dismissible"
              class="flex-shrink-0 p-1 rounded hover:bg-black/10 dark:hover:bg-white/10 transition-colors"
              aria-label="Dismiss notification"
              @click="notificationStore.remove(notification.id)"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-4 w-4"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
          </div>
        </div>
      </TransitionGroup>
    </div>
  </Teleport>
</template>

<style scoped>
.notification-enter-active,
.notification-leave-active {
  transition: all 0.3s ease;
}

.notification-enter-from {
  opacity: 0;
  transform: translateX(100%);
}

.notification-leave-to {
  opacity: 0;
  transform: translateX(100%);
}

.notification-move {
  transition: transform 0.3s ease;
}
</style>
