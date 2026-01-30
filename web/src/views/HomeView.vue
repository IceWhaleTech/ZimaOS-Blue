<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSystemStore } from '@/stores/system'
import { storeToRefs } from 'pinia'
import Skeleton from '@/components/Skeleton.vue'

const { t } = useI18n()
const systemStore = useSystemStore()
const { health, loading } = storeToRefs(systemStore)

let refreshInterval: ReturnType<typeof setInterval> | null = null
const autoRefresh = ref(true)

// Format uptime string to limit decimal places
function formatUptime(uptime: string): string {
  return uptime.replace(/(\d+)\.(\d{2})\d*s/g, '$1.$2s')
}

onMounted(() => {
  systemStore.fetchAll()
  refreshInterval = setInterval(() => {
    if (autoRefresh.value) {
      systemStore.fetchAll()
    }
  }, 5000)
})

onUnmounted(() => {
  if (refreshInterval) {
    clearInterval(refreshInterval)
  }
})
</script>

<template>
  <div class="home-page">
    <!-- Hero Section -->
    <div class="hero-section">
      <div class="hero-icon">
        <svg xmlns="http://www.w3.org/2000/svg" class="h-10 w-10 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
        </svg>
      </div>
      <h1 class="hero-title">{{ t('home.welcome') }}</h1>
      <p class="hero-description">{{ t('home.description') }}</p>
    </div>

    <!-- Auto Refresh Toggle -->
    <div class="refresh-toggle">
      <label class="toggle-label">
        <input v-model="autoRefresh" type="checkbox" class="toggle-checkbox" />
        <span class="toggle-text">{{ t('dashboard.autoRefresh') }}</span>
      </label>
    </div>

    <!-- Stats Grid -->
    <div class="stats-grid">
      <!-- Status Card -->
      <div class="stat-card">
        <div class="stat-icon status-icon">
          <svg xmlns="http://www.w3.org/2000/svg" class="h-7 w-7" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        </div>
        <div class="stat-content">
          <span class="stat-label">{{ t('home.status') }}</span>
          <Skeleton v-if="loading && !health" height="1.75rem" width="4rem" rounded="md" />
          <span v-else class="stat-value" :class="health?.status === 'ok' ? 'text-cta' : 'text-red-400'">
            {{ health?.status === 'ok' ? t('common.online') : (health?.status?.toUpperCase() || '-') }}
          </span>
        </div>
      </div>

      <!-- Version Card -->
      <div class="stat-card">
        <div class="stat-icon version-icon">
          <svg xmlns="http://www.w3.org/2000/svg" class="h-7 w-7" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.994 1.994 0 013 12V7a4 4 0 014-4z" />
          </svg>
        </div>
        <div class="stat-content">
          <span class="stat-label">{{ t('home.version') }}</span>
          <Skeleton v-if="loading && !health" height="1.75rem" width="5rem" rounded="md" />
          <span v-else class="stat-value">{{ health?.version || '-' }}</span>
        </div>
      </div>

      <!-- Uptime Card -->
      <div class="stat-card">
        <div class="stat-icon uptime-icon">
          <svg xmlns="http://www.w3.org/2000/svg" class="h-7 w-7" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        </div>
        <div class="stat-content">
          <span class="stat-label">{{ t('home.uptime') }}</span>
          <Skeleton v-if="loading && !health" height="1.75rem" width="6rem" rounded="md" />
          <span v-else class="stat-value">{{ health ? formatUptime(health.uptime) : '-' }}</span>
        </div>
      </div>

      <!-- Memory Card -->
      <div class="stat-card">
        <div class="stat-icon memory-icon">
          <svg xmlns="http://www.w3.org/2000/svg" class="h-7 w-7" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z" />
          </svg>
        </div>
        <div class="stat-content">
          <span class="stat-label">{{ t('home.memory') }}</span>
          <Skeleton v-if="loading && !health" height="1.75rem" width="5rem" rounded="md" />
          <span v-else class="stat-value">{{ health ? (health.mem_alloc_bytes / 1024 / 1024).toFixed(1) + ' MB' : '-' }}</span>
        </div>
      </div>

      <!-- Goroutines Card -->
      <div class="stat-card">
        <div class="stat-icon goroutines-icon">
          <svg xmlns="http://www.w3.org/2000/svg" class="h-7 w-7" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
        </div>
        <div class="stat-content">
          <span class="stat-label">{{ t('home.goroutines') }}</span>
          <Skeleton v-if="loading && !health" height="1.75rem" width="3rem" rounded="md" />
          <span v-else class="stat-value">{{ health?.goroutines || '-' }}</span>
        </div>
      </div>

      <!-- CPUs Card -->
      <div class="stat-card">
        <div class="stat-icon cpu-icon">
          <svg xmlns="http://www.w3.org/2000/svg" class="h-7 w-7" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2z" />
          </svg>
        </div>
        <div class="stat-content">
          <span class="stat-label">{{ t('dashboard.cpus') }}</span>
          <Skeleton v-if="loading && !health" height="1.75rem" width="2rem" rounded="md" />
          <span v-else class="stat-value">{{ health?.num_cpu || '-' }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.home-page {
  max-width: 900px;
  margin: 0 auto;
  padding: 0 16px;
}

/* Hero Section */
.hero-section {
  text-align: center;
  margin-bottom: 32px;
}

.hero-icon {
  width: 64px;
  height: 64px;
  margin: 0 auto 16px;
  border-radius: 16px;
  background: linear-gradient(135deg, var(--color-accent, #3B82F6), var(--color-cta, #10B981));
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 8px 24px rgba(59, 130, 246, 0.3);
}

.hero-title {
  font-size: 24px;
  font-weight: 700;
  color: var(--color-text-primary);
  margin: 0 0 8px;
}

.hero-description {
  font-size: 14px;
  color: var(--color-text-secondary);
  max-width: 480px;
  margin: 0 auto;
  line-height: 1.5;
}

/* Refresh Toggle */
.refresh-toggle {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 16px;
}

.toggle-label {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
}

.toggle-checkbox {
  width: 16px;
  height: 16px;
  border-radius: 4px;
  border: 1px solid var(--glass-border);
  background: var(--glass-bg);
  cursor: pointer;
  accent-color: var(--color-accent);
}

.toggle-text {
  font-size: 13px;
  color: var(--color-text-secondary);
}

/* Stats Grid */
.stats-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
  margin-bottom: 24px;
}

@media (min-width: 640px) {
  .stats-grid {
    grid-template-columns: repeat(3, 1fr);
  }
}

.stat-card {
  background: var(--glass-bg);
  border: 1px solid var(--glass-border);
  border-radius: 12px;
  padding: 16px;
  display: flex;
  align-items: flex-start;
  gap: 12px;
  transition: all 0.2s ease;
}

.stat-card:hover {
  background: var(--glass-bg-hover);
  transform: translateY(-2px);
}

.stat-icon {
  width: 48px;
  height: 48px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.status-icon { background: rgba(16, 185, 129, 0.15); color: #10B981; }
.version-icon { background: rgba(59, 130, 246, 0.15); color: #3B82F6; }
.uptime-icon { background: rgba(139, 92, 246, 0.15); color: #8B5CF6; }
.memory-icon { background: rgba(245, 158, 11, 0.15); color: #F59E0B; }
.goroutines-icon { background: rgba(6, 182, 212, 0.15); color: #06B6D4; }
.cpu-icon { background: rgba(236, 72, 153, 0.15); color: #EC4899; }

.stat-content {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.stat-label {
  font-size: 12px;
  color: var(--color-text-secondary);
}

.stat-value {
  font-size: 18px;
  font-weight: 600;
  color: var(--color-text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* Dark mode adjustments */
:root.light .stat-card {
  background: rgba(255, 255, 255, 0.8);
  border-color: rgba(0, 0, 0, 0.08);
}

:root.light .stat-card:hover {
  background: rgba(255, 255, 255, 0.95);
}
</style>
