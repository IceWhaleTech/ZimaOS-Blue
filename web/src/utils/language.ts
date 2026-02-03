/**
 * Simple language detection utility for TTS voice selection
 */

// Unicode ranges for different scripts
const CHINESE_REGEX = /[\u4e00-\u9fff\u3400-\u4dbf]/
const JAPANESE_REGEX = /[\u3040-\u309f\u30a0-\u30ff]/
const KOREAN_REGEX = /[\uac00-\ud7af\u1100-\u11ff]/
const ARABIC_REGEX = /[\u0600-\u06ff]/
const CYRILLIC_REGEX = /[\u0400-\u04ff]/
const GERMAN_CHARS_REGEX = /[äöüßÄÖÜ]/
const FRENCH_CHARS_REGEX = /[àâçéèêëîïôùûüÿœæÀÂÇÉÈÊËÎÏÔÙÛÜŸŒÆ]/
const SPANISH_CHARS_REGEX = /[áéíóúñ¿¡ÁÉÍÓÚÑ]/

/**
 * Detect the dominant language in text
 * Returns a language code suitable for Edge TTS voice selection
 */
export function detectLanguage(text: string): string {
  if (!text) return 'en'

  // Count characters for each script
  let chineseCount = 0
  let japaneseCount = 0
  let koreanCount = 0
  let arabicCount = 0
  let cyrillicCount = 0
  let latinCount = 0

  for (const char of text) {
    if (CHINESE_REGEX.test(char)) chineseCount++
    else if (JAPANESE_REGEX.test(char)) japaneseCount++
    else if (KOREAN_REGEX.test(char)) koreanCount++
    else if (ARABIC_REGEX.test(char)) arabicCount++
    else if (CYRILLIC_REGEX.test(char)) cyrillicCount++
    else if (/[a-zA-Z]/.test(char)) latinCount++
  }

  // If Japanese hiragana/katakana is present, prioritize Japanese
  if (japaneseCount > 0) {
    return 'ja'
  }

  // If Chinese characters are dominant (more than Latin), return Chinese
  if (chineseCount > 0 && chineseCount >= latinCount) {
    return 'zh'
  }

  // If Korean is present, return Korean
  if (koreanCount > 0) {
    return 'ko'
  }

  // If Arabic is present, return Arabic
  if (arabicCount > 0) {
    return 'ar'
  }

  // If Cyrillic is present, return Russian
  if (cyrillicCount > 0) {
    return 'ru'
  }

  // For Latin text, try to detect specific European language by special characters
  // Check Spanish first (has unique ñ and ¿¡)
  if (SPANISH_CHARS_REGEX.test(text)) return 'es'
  // Then German (has unique ß and umlauts)
  if (GERMAN_CHARS_REGEX.test(text)) return 'de'
  // Then French (has unique ç, œ, æ and accents)
  if (FRENCH_CHARS_REGEX.test(text)) return 'fr'

  return 'en'
}

/**
 * Get the appropriate Edge TTS voice for a language
 */
export function getVoiceForLanguage(lang: string): string {
  const voiceMap: Record<string, string> = {
    'en': 'en-US-AriaNeural',
    'zh': 'zh-CN-XiaoxiaoNeural',
    'ja': 'ja-JP-NanamiNeural',
    'ko': 'ko-KR-SunHiNeural',
    'de': 'de-DE-KatjaNeural',
    'fr': 'fr-FR-DeniseNeural',
    'es': 'es-ES-ElviraNeural',
    'ru': 'ru-RU-SvetlanaNeural',
    'ar': 'ar-SA-ZariyahNeural',
  }

  return voiceMap[lang] || voiceMap['en']
}

/**
 * Detect language and return appropriate voice
 */
export function detectVoiceForText(text: string): string {
  const lang = detectLanguage(text)
  return getVoiceForLanguage(lang)
}
