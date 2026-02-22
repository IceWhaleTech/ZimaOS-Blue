#!/usr/bin/env node
/**
 * Extract all translation keys from en-US.ts for translation
 * This script extracts the full translation object structure
 */

const fs = require('fs');
const path = require('path');

const EN_US_PATH = path.join(__dirname, '../src/i18n/locales/en-US.ts');

// Read the en-US.ts file
const content = fs.readFileSync(EN_US_PATH, 'utf-8');

// Remove the export default and comments to get just the object
const objectContent = content
  .replace(/^\/\/.*$/gm, '') // Remove single-line comments
  .replace(/\/\*[\s\S]*?\*\//g, '') // Remove multi-line comments
  .replace(/export default\s*/, '') // Remove export default
  .trim();

console.log('en-US.ts structure extracted');
console.log('Total characters:', objectContent.length);
console.log('Total lines:', objectContent.split('\n').length);

// Save to a JSON-like file for easier processing
const outputPath = path.join(__dirname, 'en-US-extracted.txt');
fs.writeFileSync(outputPath, objectContent, 'utf-8');
console.log(`Saved to: ${outputPath}`);
