<script setup lang="ts">
import type { TypelessCardFile } from '@/types/typeless'

const props = defineProps<{
  card: TypelessCardFile
}>()

// File type icons
const fileIcons: Record<string, string> = {
  pdf: '📄',
  doc: '📝',
  docx: '📝',
  xls: '📊',
  xlsx: '📊',
  ppt: '📽️',
  pptx: '📽️',
  txt: '📃',
  csv: '📋',
  json: '🔧',
  xml: '🔧',
  zip: '📦',
  rar: '📦',
  '7z': '📦',
  tar: '📦',
  gz: '📦',
  png: '🖼️',
  jpg: '🖼️',
  jpeg: '🖼️',
  gif: '🖼️',
  svg: '🖼️',
  webp: '🖼️',
  mp3: '🎵',
  wav: '🎵',
  flac: '🎵',
  mp4: '🎬',
  avi: '🎬',
  mkv: '🎬',
  mov: '🎬',
  py: '🐍',
  js: '📜',
  ts: '📜',
  go: '🔵',
  rs: '🦀',
  java: '☕',
  cpp: '⚙️',
  c: '⚙️',
  html: '🌐',
  css: '🎨',
  md: '📖',
}

function getFileIcon(): string {
  if (props.card.icon) return props.card.icon
  const ext = props.card.filename.split('.').pop()?.toLowerCase() || ''
  return fileIcons[ext] || '📁'
}

function getFileExtension(): string {
  return props.card.filename.split('.').pop()?.toUpperCase() || 'FILE'
}

function handleDownload() {
  if (props.card.downloadUrl) {
    window.open(props.card.downloadUrl, '_blank')
  }
}

function handlePreview() {
  if (props.card.previewUrl) {
    window.open(props.card.previewUrl, '_blank')
  }
}
</script>

<template>
  <div class="file-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-700">
    <div class="p-4 flex items-center gap-4">
      <!-- File icon -->
      <div class="flex-shrink-0 w-12 h-12 rounded-lg bg-gray-100 dark:bg-gray-700 flex items-center justify-center text-2xl">
        {{ getFileIcon() }}
      </div>

      <!-- File info -->
      <div class="flex-1 min-w-0">
        <h4 class="font-medium text-gray-900 dark:text-white truncate">{{ card.filename }}</h4>
        <div class="flex items-center gap-2 mt-1 text-sm text-gray-500 dark:text-gray-400">
          <span class="px-1.5 py-0.5 rounded bg-gray-100 dark:bg-gray-700 text-xs font-medium">
            {{ getFileExtension() }}
          </span>
          <span v-if="card.size">{{ card.size }}</span>
          <span v-if="card.mimeType" class="truncate">{{ card.mimeType }}</span>
        </div>
      </div>

      <!-- Actions -->
      <div class="flex-shrink-0 flex items-center gap-2">
        <button
          v-if="card.previewUrl"
          class="p-2 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
          title="Preview"
          @click="handlePreview"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"
            />
          </svg>
        </button>
        <button
          v-if="card.downloadUrl"
          class="p-2 text-gray-900 dark:text-white hover:text-gray-900 dark:text-white hover:bg-gray-700 dark:bg-gray-700 dark:hover:bg-gray-700 dark:bg-gray-700/20 rounded-lg transition-colors"
          title="Download"
          @click="handleDownload"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"
            />
          </svg>
        </button>
      </div>
    </div>
  </div>
</template>
