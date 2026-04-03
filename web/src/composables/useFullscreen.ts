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

function setBodyOverflowHidden(hidden: boolean) {
  document.body.style.overflow = hidden ? 'hidden' : ''
}

function applyFullscreenContent(content: FullscreenContent) {
  fullscreenContent.value = content
  isFullscreen.value = true
  setBodyOverflowHidden(true)
}

export function openSerializedFullscreen(type: string, dataJson: string, isBase64 = false) {
  try {
    const json = isBase64 ? decodeURIComponent(escape(atob(dataJson))) : dataJson
    const data = JSON.parse(json)

    applyFullscreenContent({
      type: type as FullscreenContent['type'],
      ...data,
    })
  } catch (err) {
    console.error('Failed to open fullscreen:', err)
  }
}

export function useFullscreen() {
  function openFullscreen(content: FullscreenContent) {
    applyFullscreenContent(content)
  }

  function closeFullscreen() {
    isFullscreen.value = false
    fullscreenContent.value = null
    setBodyOverflowHidden(false)
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
