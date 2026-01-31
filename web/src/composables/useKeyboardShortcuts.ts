import { onMounted, onUnmounted } from 'vue'

export interface KeyboardShortcut {
  key: string
  ctrl?: boolean
  shift?: boolean
  alt?: boolean
  meta?: boolean
  description: string
  handler: () => void
}

export function useKeyboardShortcuts(shortcuts: KeyboardShortcut[]) {
  function handleKeyDown(event: KeyboardEvent) {
    for (const shortcut of shortcuts) {
      const keyMatch = event.key.toLowerCase() === shortcut.key.toLowerCase()
      const ctrlMatch = !!shortcut.ctrl === (event.ctrlKey || event.metaKey)
      const shiftMatch = !!shortcut.shift === event.shiftKey
      const altMatch = !!shortcut.alt === event.altKey

      if (keyMatch && ctrlMatch && shiftMatch && altMatch) {
        // Don't trigger shortcuts when typing in input fields
        const target = event.target as HTMLElement
        const isInputField =
          target.tagName === 'INPUT' ||
          target.tagName === 'TEXTAREA' ||
          target.isContentEditable

        // Allow some shortcuts even in input fields
        const allowInInput = shortcut.ctrl || shortcut.meta

        if (!isInputField || allowInInput) {
          event.preventDefault()
          shortcut.handler()
          return
        }
      }
    }
  }

  onMounted(() => {
    window.addEventListener('keydown', handleKeyDown)
  })

  onUnmounted(() => {
    window.removeEventListener('keydown', handleKeyDown)
  })

  return {
    shortcuts,
  }
}

// Common shortcuts for the chat view
// Using Alt combinations to avoid browser shortcut conflicts (Ctrl+N, Ctrl+B)
export function useChatShortcuts(options: {
  onNewChat: () => void
  onFocusInput: () => void
  onToggleSidebar: () => void
  onCancelStream?: () => void
  onSearch?: () => void
}) {
  return useKeyboardShortcuts([
    {
      key: 'n',
      alt: true,
      description: 'New conversation',
      handler: options.onNewChat,
    },
    {
      key: '/',
      description: 'Focus message input',
      handler: options.onFocusInput,
    },
    {
      key: 'b',
      alt: true,
      description: 'Toggle sidebar',
      handler: options.onToggleSidebar,
    },
    ...(options.onCancelStream
      ? [
          {
            key: 'Escape',
            description: 'Cancel streaming',
            handler: options.onCancelStream,
          },
        ]
      : []),
    ...(options.onSearch
      ? [
          {
            key: 'k',
            ctrl: true,
            description: 'Search conversations',
            handler: options.onSearch,
          },
        ]
      : []),
  ])
}

// Global shortcuts
export function useGlobalShortcuts(options: {
  onToggleTheme?: () => void
  onGoToChat?: () => void
  onGoToSettings?: () => void
  onGoToDashboard?: () => void
  onGoToPlugins?: () => void
}) {
  return useKeyboardShortcuts([
    ...(options.onToggleTheme
      ? [
          {
            key: 'd',
            ctrl: true,
            shift: true,
            description: 'Toggle dark mode',
            handler: options.onToggleTheme,
          },
        ]
      : []),
    ...(options.onGoToChat
      ? [
          {
            key: '1',
            alt: true,
            description: 'Go to Chat',
            handler: options.onGoToChat,
          },
        ]
      : []),
    ...(options.onGoToDashboard
      ? [
          {
            key: '2',
            alt: true,
            description: 'Go to Dashboard',
            handler: options.onGoToDashboard,
          },
        ]
      : []),
    ...(options.onGoToSettings
      ? [
          {
            key: '3',
            alt: true,
            description: 'Go to Settings',
            handler: options.onGoToSettings,
          },
        ]
      : []),
    ...(options.onGoToPlugins
      ? [
          {
            key: '4',
            alt: true,
            description: 'Go to Plugins',
            handler: options.onGoToPlugins,
          },
        ]
      : []),
  ])
}
