import { describe, it, expect, vi, afterEach } from 'vitest'
import { EnergyVAD } from '@/utils/vad'

type TestVAD = {
  vad: any
  emitSpeech: ReturnType<typeof vi.fn>
  discardSpeech: ReturnType<typeof vi.fn>
}

function buildSilencePendingVAD(params: {
  rmsFrames: number[]
  silenceStartDeltaMs: number
  nowMs: number
}): TestVAD {
  const vad = new EnergyVAD({
    speechThreshold: 0.015,
    silenceThreshold: 0.01,
    noiseCalibrationDuration: 0,
    silenceDuration: 120,
    minSpeechDuration: 0,
  }) as any

  vad.analyser = { getByteTimeDomainData: vi.fn() }
  vad.dataArray = new Uint8Array(32)
  vad.state = 'silence_pending'
  vad.paused = false
  vad._isSpeaking = true
  vad.listeningStartTime = 0
  vad.speechStartTime = params.nowMs - 500
  vad.silenceStartTime = params.nowMs - params.silenceStartDeltaMs
  vad.noiseFloor = 0.012

  const rmsQueue = [...params.rmsFrames]
  vad.computeRMS = vi.fn(() => rmsQueue.shift() ?? 0)
  vad.computeAdaptiveThresholds = vi.fn(() => ({ speech: 0.02, silence: 0.015 }))

  const emitSpeech = vi.fn()
  const discardSpeech = vi.fn()
  vad.emitSpeech = emitSpeech
  vad.discardSpeech = discardSpeech

  return { vad, emitSpeech, discardSpeech }
}

afterEach(() => {
  vi.restoreAllMocks()
})

describe('EnergyVAD silence pending hysteresis', () => {
  it('does not resume speaking on a single noisy spike', () => {
    const nowMs = 10_000
    vi.spyOn(Date, 'now').mockReturnValue(nowMs)
    const { vad, emitSpeech } = buildSilencePendingVAD({
      rmsFrames: [0.023, 0.014],
      silenceStartDeltaMs: 200,
      nowMs,
    })

    vad.tick()
    expect(vad.state).toBe('silence_pending')

    vad.tick()
    expect(vad.state).toBe('idle')
    expect(emitSpeech).toHaveBeenCalledTimes(1)
  })

  it('resumes speaking on consecutive high-energy frames', () => {
    const nowMs = 20_000
    vi.spyOn(Date, 'now').mockReturnValue(nowMs)
    const { vad, emitSpeech, discardSpeech } = buildSilencePendingVAD({
      rmsFrames: [0.023, 0.024],
      silenceStartDeltaMs: 50,
      nowMs,
    })

    vad.tick()
    expect(vad.state).toBe('silence_pending')

    vad.tick()
    expect(vad.state).toBe('speaking')
    expect(emitSpeech).not.toHaveBeenCalled()
    expect(discardSpeech).not.toHaveBeenCalled()
  })
})
