import { describe, it, expect } from 'vitest'
import { detectLanguage, getVoiceForLanguage, detectVoiceForText } from './language'

describe('detectLanguage', () => {
  it('should detect English text', () => {
    expect(detectLanguage('Hello, how are you today?')).toBe('en')
    expect(detectLanguage('This is a test message')).toBe('en')
  })

  it('should detect Chinese text', () => {
    expect(detectLanguage('你好，今天天气怎么样？')).toBe('zh')
    expect(detectLanguage('这是一条测试消息')).toBe('zh')
  })

  it('should detect Japanese text', () => {
    expect(detectLanguage('こんにちは、元気ですか？')).toBe('ja')
    expect(detectLanguage('これはテストメッセージです')).toBe('ja')
  })

  it('should detect Korean text', () => {
    expect(detectLanguage('안녕하세요, 오늘 날씨가 어때요?')).toBe('ko')
    expect(detectLanguage('이것은 테스트 메시지입니다')).toBe('ko')
  })

  it('should detect German text', () => {
    expect(detectLanguage('Ich möchte einen Kaffee, bitte')).toBe('de') // Has ö
    expect(detectLanguage('Die Größe ist wichtig')).toBe('de') // Has ö, ß
  })

  it('should detect French text', () => {
    expect(detectLanguage('Je suis très heureux')).toBe('fr') // Has è
    expect(detectLanguage('Ça va bien, merci')).toBe('fr') // Has ç
  })

  it('should detect Spanish text', () => {
    expect(detectLanguage('Hola, cómo estás?')).toBe('es') // Has ó, á
    expect(detectLanguage('¿Qué tal?')).toBe('es') // Has ¿
    expect(detectLanguage('El niño está bien')).toBe('es') // Has ñ, á
  })

  it('should handle mixed Chinese and English text', () => {
    // Mostly Chinese with some English
    expect(detectLanguage('今天我学习了编程语言')).toBe('zh')
    // More Chinese than English characters
    expect(detectLanguage('你好世界Hi')).toBe('zh') // 4 Chinese, 2 English
    // Mostly English with some Chinese
    expect(detectLanguage('I learned 你好 today in Chinese class and it was great')).toBe('en')
  })

  it('should handle empty or null text', () => {
    expect(detectLanguage('')).toBe('en')
    expect(detectLanguage(null as unknown as string)).toBe('en')
    expect(detectLanguage(undefined as unknown as string)).toBe('en')
  })
})

describe('getVoiceForLanguage', () => {
  it('should return correct voice for each language', () => {
    expect(getVoiceForLanguage('en')).toBe('en-US-AriaNeural')
    expect(getVoiceForLanguage('zh')).toBe('zh-CN-XiaoxiaoNeural')
    expect(getVoiceForLanguage('ja')).toBe('ja-JP-NanamiNeural')
    expect(getVoiceForLanguage('ko')).toBe('ko-KR-SunHiNeural')
    expect(getVoiceForLanguage('de')).toBe('de-DE-KatjaNeural')
    expect(getVoiceForLanguage('fr')).toBe('fr-FR-DeniseNeural')
    expect(getVoiceForLanguage('es')).toBe('es-ES-ElviraNeural')
  })

  it('should return English voice for unknown language', () => {
    expect(getVoiceForLanguage('unknown')).toBe('en-US-AriaNeural')
    expect(getVoiceForLanguage('')).toBe('en-US-AriaNeural')
  })
})

describe('detectVoiceForText', () => {
  it('should return Chinese voice for Chinese text', () => {
    expect(detectVoiceForText('你好，世界！')).toBe('zh-CN-XiaoxiaoNeural')
  })

  it('should return English voice for English text', () => {
    expect(detectVoiceForText('Hello, world!')).toBe('en-US-AriaNeural')
  })

  it('should return Japanese voice for Japanese text', () => {
    expect(detectVoiceForText('こんにちは、世界！')).toBe('ja-JP-NanamiNeural')
  })

  it('should handle mixed text with dominant Chinese', () => {
    const text = '今天我们来学习一下如何使用Edge TTS进行语音合成，这是一个非常有用的功能。'
    expect(detectVoiceForText(text)).toBe('zh-CN-XiaoxiaoNeural')
  })
})
