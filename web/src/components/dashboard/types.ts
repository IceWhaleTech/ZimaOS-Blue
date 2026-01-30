import type { Component } from 'vue'

export interface DashboardCardConfig {
  id: string
  titleKey: string // i18n key
  icon: string
  iconColor: 'blue' | 'green' | 'purple' | 'orange' | 'red' | 'gray'
  component: Component | string
  category: 'overview' | 'metrics' | 'system' | 'network'
  defaultEnabled: boolean
  defaultOrder: number
  minWidth?: 1 | 2 | 3 | 4 // grid columns (out of 4)
  props?: Record<string, unknown>
}

export interface DashboardCardState {
  id: string
  enabled: boolean
  order: number
  collapsed: boolean
}

export interface DashboardLayout {
  cards: DashboardCardState[]
  lastModified: string
}
