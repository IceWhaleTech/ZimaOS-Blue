import { ref } from 'vue'

export interface FullscreenContent {
  type: 'code' | 'diff' | 'terminal'
  title?: string
  language?: string
  content: string
  // For diff
  oldCode?: string
  newCode?: string
  oldLabel?: string
  newLabel?: string
}

// Global fullscreen state
const isFullscreen = ref(false)
const fullscreenContent = ref<FullscreenContent | null>(null)

export function useFullscreen() {
  function openFullscreen(content: FullscreenContent) {
    fullscreenContent.value = content
    isFullscreen.value = true
    // Prevent body scroll
    document.body.style.overflow = 'hidden'
  }

  function closeFullscreen() {
    isFullscreen.value = false
    fullscreenContent.value = null
    // Restore body scroll
    document.body.style.overflow = ''
  }

  return {
    isFullscreen,
    fullscreenContent,
    openFullscreen,
    closeFullscreen,
  }
}

// Export global state for the modal component
export { isFullscreen, fullscreenContent }
