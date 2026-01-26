import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useNotificationStore } from '@/stores/notification'

describe('Notification Store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.useFakeTimers()
  })

  it('should add a notification', () => {
    const store = useNotificationStore()

    const id = store.add({
      type: 'success',
      title: 'Test notification',
    })

    expect(store.notifications).toHaveLength(1)
    expect(store.notifications[0].id).toBe(id)
    expect(store.notifications[0].type).toBe('success')
    expect(store.notifications[0].title).toBe('Test notification')
  })

  it('should remove a notification', () => {
    const store = useNotificationStore()

    const id = store.add({
      type: 'info',
      title: 'Test',
    })

    expect(store.notifications).toHaveLength(1)

    store.remove(id)

    expect(store.notifications).toHaveLength(0)
  })

  it('should auto-dismiss after duration', () => {
    const store = useNotificationStore()

    store.add({
      type: 'success',
      title: 'Test',
      duration: 3000,
    })

    expect(store.notifications).toHaveLength(1)

    vi.advanceTimersByTime(3000)

    expect(store.notifications).toHaveLength(0)
  })

  it('should not auto-dismiss when duration is 0', () => {
    const store = useNotificationStore()

    store.add({
      type: 'error',
      title: 'Test',
      duration: 0,
    })

    expect(store.notifications).toHaveLength(1)

    vi.advanceTimersByTime(10000)

    expect(store.notifications).toHaveLength(1)
  })

  it('should clear all notifications', () => {
    const store = useNotificationStore()

    store.add({ type: 'success', title: 'Test 1' })
    store.add({ type: 'info', title: 'Test 2' })
    store.add({ type: 'warning', title: 'Test 3' })

    expect(store.notifications).toHaveLength(3)

    store.clear()

    expect(store.notifications).toHaveLength(0)
  })

  it('should have convenience methods', () => {
    const store = useNotificationStore()

    store.success('Success title', 'Success message')
    expect(store.notifications[0].type).toBe('success')

    store.error('Error title', 'Error message')
    expect(store.notifications[1].type).toBe('error')
    expect(store.notifications[1].duration).toBe(0) // Errors don't auto-dismiss

    store.warning('Warning title')
    expect(store.notifications[2].type).toBe('warning')

    store.info('Info title')
    expect(store.notifications[3].type).toBe('info')
  })

  it('should compute hasNotifications correctly', () => {
    const store = useNotificationStore()

    expect(store.hasNotifications).toBe(false)

    store.add({ type: 'info', title: 'Test' })

    expect(store.hasNotifications).toBe(true)

    store.clear()

    expect(store.hasNotifications).toBe(false)
  })
})
