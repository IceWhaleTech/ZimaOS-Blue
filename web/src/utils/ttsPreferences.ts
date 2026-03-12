const TTS_AUTO_PLAY_STORAGE_KEY = 'tts-auto-play'
const TTS_SPEECH_VOLUME_STORAGE_KEY = 'tts-speech-volume'
const DEFAULT_TTS_SPEECH_VOLUME = 100

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

export function getTtsSpeechVolume(): number {
  if (typeof window === 'undefined' || !window.localStorage) {
    return DEFAULT_TTS_SPEECH_VOLUME
  }
  const rawValue = window.localStorage.getItem(TTS_SPEECH_VOLUME_STORAGE_KEY)
  if (rawValue === null) {
    return DEFAULT_TTS_SPEECH_VOLUME
  }
  const parsed = Number.parseFloat(rawValue)
  return Number.isFinite(parsed) ? parsed : DEFAULT_TTS_SPEECH_VOLUME
}

export function isTtsSpeechMuted(): boolean {
  return getTtsSpeechVolume() <= 0
}
