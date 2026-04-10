import type { LocaleKey } from './locale-catalog'

const chatQuickNavBackfills: Partial<Record<LocaleKey, object>> = {
  'ca-ES': { chat: { quickNav: { title: 'Navegació ràpida', jumpToMessage: 'Vés a {preview}' } } },
  'cs-CZ': {
    chat: { quickNav: { title: 'Rychlá navigace', jumpToMessage: 'Přejít na {preview}' } },
  },
  'da-DK': {
    chat: { quickNav: { title: 'Hurtig navigation', jumpToMessage: 'Gå til {preview}' } },
  },
  'de-DE': {
    chat: { quickNav: { title: 'Schnellnavigation', jumpToMessage: 'Zu {preview} springen' } },
  },
  'el-GR': {
    chat: { quickNav: { title: 'Γρήγορη πλοήγηση', jumpToMessage: 'Μετάβαση σε {preview}' } },
  },
  'en-GB': {
    chat: { quickNav: { title: 'Quick navigation', jumpToMessage: 'Jump to {preview}' } },
  },
  'en-US': {
    chat: { quickNav: { title: 'Quick navigation', jumpToMessage: 'Jump to {preview}' } },
  },
  'es-ES': { chat: { quickNav: { title: 'Navegación rápida', jumpToMessage: 'Ir a {preview}' } } },
  'fr-FR': {
    chat: { quickNav: { title: 'Navigation rapide', jumpToMessage: 'Aller à {preview}' } },
  },
  'ga-IE': {
    chat: { quickNav: { title: 'Nascleanúint thapa', jumpToMessage: 'Léim go {preview}' } },
  },
  'hr-HR': {
    chat: { quickNav: { title: 'Brza navigacija', jumpToMessage: 'Skoči na {preview}' } },
  },
  'hu-HU': {
    chat: { quickNav: { title: 'Gyors navigáció', jumpToMessage: 'Ugrás ide: {preview}' } },
  },
  'it-IT': {
    chat: { quickNav: { title: 'Navigazione rapida', jumpToMessage: 'Vai a {preview}' } },
  },
  'ja-JP': {
    chat: { quickNav: { title: 'クイックナビゲーション', jumpToMessage: '{preview} へ移動' } },
  },
  'ko-KR': { chat: { quickNav: { title: '빠른 탐색', jumpToMessage: '{preview}(으)로 이동' } } },
  'ml-IN': {
    chat: { quickNav: { title: 'ദ്രുത നാവിഗേഷൻ', jumpToMessage: '{preview} ലേക്ക് ചാടുക' } },
  },
  'nb-NO': { chat: { quickNav: { title: 'Hurtignavigasjon', jumpToMessage: 'Gå til {preview}' } } },
  'nl-NL': {
    chat: { quickNav: { title: 'Snelle navigatie', jumpToMessage: 'Ga naar {preview}' } },
  },
  'pl-PL': {
    chat: { quickNav: { title: 'Szybka nawigacja', jumpToMessage: 'Przejdź do {preview}' } },
  },
  'pt-BR': {
    chat: { quickNav: { title: 'Navegação rápida', jumpToMessage: 'Ir para {preview}' } },
  },
  'pt-PT': {
    chat: { quickNav: { title: 'Navegação rápida', jumpToMessage: 'Ir para {preview}' } },
  },
  'ro-RO': { chat: { quickNav: { title: 'Navigare rapidă', jumpToMessage: 'Salt la {preview}' } } },
  'ru-RU': {
    chat: { quickNav: { title: 'Быстрая навигация', jumpToMessage: 'Перейти к {preview}' } },
  },
  'sk-SK': {
    chat: { quickNav: { title: 'Rýchla navigácia', jumpToMessage: 'Prejsť na {preview}' } },
  },
  'sv-SE': { chat: { quickNav: { title: 'Snabbnavigering', jumpToMessage: 'Gå till {preview}' } } },
  'zh-CN': { chat: { quickNav: { title: '快速导航', jumpToMessage: '跳转到 {preview}' } } },
  'zh-TW': { chat: { quickNav: { title: '快速導覽', jumpToMessage: '跳到 {preview}' } } },
}

export default chatQuickNavBackfills
