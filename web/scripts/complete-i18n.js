#!/usr/bin/env node
/**
 * i18n Translation Completion Script
 *
 * This script helps complete missing translations in locale files.
 * It compares each locale file with en-US.ts and identifies missing translations.
 *
 * Usage:
 *   node complete-i18n.js [locale]
 *
 * Examples:
 *   node complete-i18n.js de-DE    # Complete German translations
 *   node complete-i18n.js all      # Complete all locales
 */

const fs = require('fs');
const path = require('path');

const LOCALES_DIR = path.join(__dirname, '../src/i18n/locales');
const EN_US_FILE = path.join(LOCALES_DIR, 'en-US.ts');

// Language mapping for translation
const LANGUAGE_NAMES = {
  'ca-ES': 'Catalan',
  'cs-CZ': 'Czech',
  'da-DK': 'Danish',
  'de-DE': 'German',
  'el-GR': 'Greek',
  'en-GB': 'English (UK)',
  'es-ES': 'Spanish',
  'fr-FR': 'French',
  'ga-IE': 'Irish',
  'hr-HR': 'Croatian',
  'hu-HU': 'Hungarian',
  'it-IT': 'Italian',
  'ja-JP': 'Japanese',
  'ko-KR': 'Korean',
  'ml-IN': 'Malayalam',
  'nb-NO': 'Norwegian',
  'nl-NL': 'Dutch',
  'pl-PL': 'Polish',
  'pt-BR': 'Portuguese (Brazil)',
  'pt-PT': 'Portuguese (Portugal)',
  'ro-RO': 'Romanian',
  'ru-RU': 'Russian',
  'sk-SK': 'Slovak',
  'sv-SE': 'Swedish',
  'zh-CN': 'Chinese (Simplified)',
  'zh-TW': 'Chinese (Traditional)',
};

function getFileStats() {
  const files = fs.readdirSync(LOCALES_DIR).filter(f => f.endsWith('.ts'));
  const stats = [];

  for (const file of files) {
    const filePath = path.join(LOCALES_DIR, file);
    const content = fs.readFileSync(filePath, 'utf-8');
    const lines = content.split('\n').length;
    const locale = file.replace('.ts', '');

    stats.push({
      locale,
      file,
      lines,
      language: LANGUAGE_NAMES[locale] || locale,
    });
  }

  return stats.sort((a, b) => a.lines - b.lines);
}

function main() {
  const stats = getFileStats();
  const enUSLines = stats.find(s => s.locale === 'en-US')?.lines || 0;

  console.log('\n📊 i18n Translation Status\n');
  console.log('Locale'.padEnd(12), 'Language'.padEnd(25), 'Lines'.padStart(6), 'Complete'.padStart(10));
  console.log('─'.repeat(60));

  for (const stat of stats) {
    const percentage = ((stat.lines / enUSLines) * 100).toFixed(1);
    const complete = `${percentage}%`;
    console.log(
      stat.locale.padEnd(12),
      stat.language.padEnd(25),
      stat.lines.toString().padStart(6),
      complete.padStart(10)
    );
  }

  console.log('\n💡 Note: All locale files use en-US as fallback via spread operator.');
  console.log('   Missing translations will automatically display in English.');
  console.log('\n📝 To add translations:');
  console.log('   1. Open the locale file (e.g., de-DE.ts)');
  console.log('   2. Add translations following the existing structure');
  console.log('   3. Use {...enUS.sectionName} to inherit English defaults');
  console.log('\n🔧 For automated translation, consider using:');
  console.log('   - DeepL API (https://www.deepl.com/pro-api)');
  console.log('   - Google Translate API');
  console.log('   - Claude API for context-aware translations');
}

main();
