const TTS_AUTO_PLAY_STORAGE_KEY = 'tts-auto-play'

export function isTtsAutoPlayEnabled(): boolean {
  if (typeof window === 'undefined' || !window.localStorage) {
    return false
  }
  return window.localStorage.getItem(TTS_AUTO_PLAY_STORAGE_KEY) === 'true'
}

export function setTtsAutoPlayEnabled(enabled: boolean): void {
  if (typeof window === 'undefined' || !window.localStorage) {
    return
  }
  window.localStorage.setItem(TTS_AUTO_PLAY_STORAGE_KEY, enabled.toString())
}
