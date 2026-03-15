/**
 * Convert audio blob to WAV format using Web Audio API.
 * Returns null (with console warning) if the audio cannot be decoded.
 */
export async function convertToWav(audioBlob: Blob): Promise<Blob | null> {
  if (audioBlob.size === 0) {
    console.warn('[audio] Empty audio blob, skipping conversion')
    return null
  }

  const audioContext = new AudioContext({ sampleRate: 16000 })

  try {
    const arrayBuffer = await audioBlob.arrayBuffer()
    const audioBuffer = await audioContext.decodeAudioData(arrayBuffer)

    // Resample to 16kHz mono
    const offlineContext = new OfflineAudioContext(1, audioBuffer.duration * 16000, 16000)
    const source = offlineContext.createBufferSource()
    source.buffer = audioBuffer
    source.connect(offlineContext.destination)
    source.start()

    const renderedBuffer = await offlineContext.startRendering()
    return encodeWav(renderedBuffer)
  } catch (err) {
    console.warn('[audio] Failed to decode audio data, skipping:', err)
    return null
  } finally {
    await audioContext.close()
  }
}

/**
 * Encode AudioBuffer to WAV format
 */
function encodeWav(audioBuffer: AudioBuffer): Blob {
  const numChannels = 1
  const sampleRate = audioBuffer.sampleRate
  const format = 1 // PCM
  const bitDepth = 16

  const samples = audioBuffer.getChannelData(0)
  const buffer = new ArrayBuffer(44 + samples.length * 2)
  const view = new DataView(buffer)

  // WAV header
  writeString(view, 0, 'RIFF')
  view.setUint32(4, 36 + samples.length * 2, true)
  writeString(view, 8, 'WAVE')
  writeString(view, 12, 'fmt ')
  view.setUint32(16, 16, true) // fmt chunk size
  view.setUint16(20, format, true)
  view.setUint16(22, numChannels, true)
  view.setUint32(24, sampleRate, true)
  view.setUint32(28, (sampleRate * numChannels * bitDepth) / 8, true)
  view.setUint16(32, (numChannels * bitDepth) / 8, true)
  view.setUint16(34, bitDepth, true)
  writeString(view, 36, 'data')
  view.setUint32(40, samples.length * 2, true)

  // Write samples
  let offset = 44
  for (let i = 0; i < samples.length; i++) {
    const sample = Math.max(-1, Math.min(1, samples[i] ?? 0))
    view.setInt16(offset, sample < 0 ? sample * 0x8000 : sample * 0x7fff, true)
    offset += 2
  }

  return new Blob([buffer], { type: 'audio/wav' })
}

function writeString(view: DataView, offset: number, str: string) {
  for (let i = 0; i < str.length; i++) {
    view.setUint8(offset + i, str.charCodeAt(i))
  }
}
