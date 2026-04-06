import type { Component } from 'vue'
export { embeddedMermaidBundleDisabled } from './mermaidRuntimeLoader'

export const loadMermaidCardComponent: () => Promise<{ default: Component }> = () =>
  import('@/components/typeless/CardMermaid.vue')
