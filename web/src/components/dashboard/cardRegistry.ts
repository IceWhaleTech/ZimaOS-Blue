import type { DashboardCardConfig } from './types'

// Card registry - all available dashboard cards for System Overview page
// Note: Metrics-related cards are NOT included here as they belong to the Metrics tab
export const cardRegistry: DashboardCardConfig[] = [
  // Overview Cards
  {
    id: 'system-status',
    titleKey: 'dashboard.cards.systemStatus',
    icon: 'status',
    iconColor: 'green',
    component: 'SystemStatusCard',
    category: 'overview',
    defaultEnabled: true,
    defaultOrder: 1,
    minWidth: 1,
  },
  {
    id: 'uptime',
    titleKey: 'dashboard.cards.uptime',
    icon: 'clock',
    iconColor: 'blue',
    component: 'UptimeCard',
    category: 'overview',
    defaultEnabled: true,
    defaultOrder: 2,
    minWidth: 1,
  },
  {
    id: 'memory-usage',
    titleKey: 'dashboard.cards.memoryUsage',
    icon: 'memory',
    iconColor: 'purple',
    component: 'MemoryUsageCard',
    category: 'overview',
    defaultEnabled: true,
    defaultOrder: 3,
    minWidth: 1,
  },
  {
    id: 'goroutines',
    titleKey: 'dashboard.cards.goroutines',
    icon: 'cpu',
    iconColor: 'orange',
    component: 'GoroutinesCard',
    category: 'overview',
    defaultEnabled: true,
    defaultOrder: 4,
    minWidth: 1,
  },
  {
    id: 'failover-status',
    titleKey: 'dashboard.cards.failoverStatus',
    icon: 'shield',
    iconColor: 'blue',
    component: 'FailoverStatusCard',
    category: 'overview',
    defaultEnabled: true,
    defaultOrder: 5,
    minWidth: 4,
  },

  // System Cards
  {
    id: 'cpu-chart',
    titleKey: 'dashboard.cards.cpuChart',
    icon: 'cpu',
    iconColor: 'blue',
    component: 'CpuChartCard',
    category: 'system',
    defaultEnabled: true,
    defaultOrder: 5,
    minWidth: 2,
  },
  {
    id: 'memory-chart',
    titleKey: 'dashboard.cards.memoryChart',
    icon: 'memory',
    iconColor: 'green',
    component: 'MemoryChartCard',
    category: 'system',
    defaultEnabled: true,
    defaultOrder: 6,
    minWidth: 2,
  },
  {
    id: 'goroutines-chart',
    titleKey: 'dashboard.cards.goroutinesChart',
    icon: 'chart',
    iconColor: 'purple',
    component: 'GoroutinesChartCard',
    category: 'system',
    defaultEnabled: false,
    defaultOrder: 7,
    minWidth: 2,
  },
  {
    id: 'heap-chart',
    titleKey: 'dashboard.cards.heapChart',
    icon: 'memory',
    iconColor: 'orange',
    component: 'HeapChartCard',
    category: 'system',
    defaultEnabled: false,
    defaultOrder: 8,
    minWidth: 2,
  },
  {
    id: 'system-info',
    titleKey: 'dashboard.cards.systemInfo',
    icon: 'info',
    iconColor: 'gray',
    component: 'SystemInfoCard',
    category: 'system',
    defaultEnabled: true,
    defaultOrder: 9,
    minWidth: 4,
  },
]

// Get card config by ID
export function getCardConfig(id: string): DashboardCardConfig | undefined {
  return cardRegistry.find((card) => card.id === id)
}

// Get cards by category
export function getCardsByCategory(category: DashboardCardConfig['category']): DashboardCardConfig[] {
  return cardRegistry.filter((card) => card.category === category)
}

// Get default card states
export function getDefaultCardStates(): { id: string; enabled: boolean; order: number; collapsed: boolean }[] {
  return cardRegistry.map((card) => ({
    id: card.id,
    enabled: card.defaultEnabled,
    order: card.defaultOrder,
    collapsed: false,
  }))
}
