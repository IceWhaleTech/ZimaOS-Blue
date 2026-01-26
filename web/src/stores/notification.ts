import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export type NotificationType = 'success' | 'error' | 'warning' | 'info'

export interface Notification {
  id: string
  type: NotificationType
  title: string
  message?: string
  duration?: number
  dismissible?: boolean
  action?: {
    label: string
    handler: () => void
  }
}

const DEFAULT_DURATION = 5000

export const useNotificationStore = defineStore('notification', () => {
  const notifications = ref<Notification[]>([])
  const timers = new Map<string, ReturnType<typeof setTimeout>>()

  const hasNotifications = computed(() => notifications.value.length > 0)

  function generateId(): string {
    return `notification-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`
  }

  function add(notification: Omit<Notification, 'id'>): string {
    const id = generateId()
    const newNotification: Notification = {
      id,
      dismissible: true,
      duration: DEFAULT_DURATION,
      ...notification,
    }

    notifications.value.push(newNotification)

    // Auto-dismiss after duration
    if (newNotification.duration && newNotification.duration > 0) {
      const timer = setTimeout(() => {
        remove(id)
      }, newNotification.duration)
      timers.set(id, timer)
    }

    return id
  }

  function remove(id: string) {
    const index = notifications.value.findIndex((n) => n.id === id)
    if (index !== -1) {
      notifications.value.splice(index, 1)
    }

    // Clear timer if exists
    const timer = timers.get(id)
    if (timer) {
      clearTimeout(timer)
      timers.delete(id)
    }
  }

  function clear() {
    // Clear all timers
    timers.forEach((timer) => clearTimeout(timer))
    timers.clear()

    notifications.value = []
  }

  // Convenience methods
  function success(title: string, message?: string, options?: Partial<Notification>) {
    return add({ type: 'success', title, message, ...options })
  }

  function error(title: string, message?: string, options?: Partial<Notification>) {
    return add({ type: 'error', title, message, duration: 0, ...options }) // Errors don't auto-dismiss
  }

  function warning(title: string, message?: string, options?: Partial<Notification>) {
    return add({ type: 'warning', title, message, ...options })
  }

  function info(title: string, message?: string, options?: Partial<Notification>) {
    return add({ type: 'info', title, message, ...options })
  }

  return {
    notifications,
    hasNotifications,
    add,
    remove,
    clear,
    success,
    error,
    warning,
    info,
  }
})
