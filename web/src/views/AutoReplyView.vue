<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAutoReplyStore } from '@/stores/autoreply'
import AutoReplyRuleCard from '@/components/AutoReplyRuleCard.vue'
import AutoReplyRuleForm from '@/components/AutoReplyRuleForm.vue'
import AutoReplyTestPanel from '@/components/AutoReplyTestPanel.vue'
import type { AutoReplyRule, CreateRuleRequest } from '@/api/autoreply'

const { t } = useI18n()
const autoReplyStore = useAutoReplyStore()

// Search and filter
const searchQuery = ref('')
const filterType = ref<string>('all')
const filterStatus = ref<string>('all')

// Modal states
const showCreateModal = ref(false)
const showEditModal = ref(false)
const showTestModal = ref(false)
const showDeleteConfirm = ref(false)
const selectedRule = ref<AutoReplyRule | null>(null)
const testPanelRef = ref<InstanceType<typeof AutoReplyTestPanel> | null>(null)

// Filtered rules
const filteredRules = computed(() => {
  let result = autoReplyStore.rules

  // Search filter
  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    result = result.filter(
      (r) =>
        r.name.toLowerCase().includes(query) ||
        r.trigger_value.toLowerCase().includes(query) ||
        r.responses.some((resp) => resp.toLowerCase().includes(query))
    )
  }

  // Type filter
  if (filterType.value !== 'all') {
    result = result.filter((r) => r.trigger_type === filterType.value)
  }

  // Status filter
  if (filterStatus.value !== 'all') {
    if (filterStatus.value === 'enabled') {
      result = result.filter((r) => r.enabled)
    } else if (filterStatus.value === 'disabled') {
      result = result.filter((r) => !r.enabled)
    }
  }

  // Sort by priority
  return [...result].sort((a, b) => b.priority - a.priority)
})

// Stats
const stats = computed(() => ({
  total: autoReplyStore.rules.length,
  enabled: autoReplyStore.enabledRules.length,
  disabled: autoReplyStore.disabledRules.length,
  totalMatches: autoReplyStore.rules.reduce((sum, r) => sum + r.match_count, 0),
}))

onMounted(async () => {
  await autoReplyStore.fetchRules()
})

// Handlers
async function handleToggle(rule: AutoReplyRule): Promise<void> {
  if (rule.enabled) {
    await autoReplyStore.disableRule(rule.id)
  } else {
    await autoReplyStore.enableRule(rule.id)
  }
}

function openCreateModal(): void {
  selectedRule.value = null
  showCreateModal.value = true
}

function openEditModal(rule: AutoReplyRule): void {
  selectedRule.value = rule
  showEditModal.value = true
}

function openTestModal(rule?: AutoReplyRule): void {
  selectedRule.value = rule || null
  showTestModal.value = true
}

function openDeleteConfirm(rule: AutoReplyRule): void {
  selectedRule.value = rule
  showDeleteConfirm.value = true
}

function closeModals(): void {
  showCreateModal.value = false
  showEditModal.value = false
  showTestModal.value = false
  showDeleteConfirm.value = false
  selectedRule.value = null
}

async function handleCreate(data: CreateRuleRequest): Promise<void> {
  const result = await autoReplyStore.createRule(data)
  if (result) {
    closeModals()
  }
}

async function handleUpdate(data: CreateRuleRequest): Promise<void> {
  if (selectedRule.value) {
    const success = await autoReplyStore.updateRule(selectedRule.value.id, data)
    if (success) {
      closeModals()
    }
  }
}

async function handleDelete(): Promise<void> {
  if (selectedRule.value) {
    const success = await autoReplyStore.deleteRule(selectedRule.value.id)
    if (success) {
      closeModals()
    }
  }
}

async function handleTest(message: string, channel?: string): Promise<void> {
  const result = await autoReplyStore.testMessage(message, channel)
  if (testPanelRef.value) {
    testPanelRef.value.setResult(result)
  }
}
</script>

<template>
  <div class="autoreply-view p-6 max-w-6xl mx-auto">
    <!-- Header -->
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-2xl font-bold text-gray-900 dark:text-white">{{ t('autoReply.title') }}</h1>
      <div class="flex items-center gap-3">
        <button
          class="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded-lg text-sm transition-colors flex items-center gap-2"
          @click="openTestModal()"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-4 w-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"
            />
          </svg>
          {{ t('autoReply.testRules') }}
        </button>
        <button
          class="px-4 py-2 bg-gray-700 dark:bg-gray-700 hover:bg-gray-700 dark:bg-gray-700 text-white rounded-lg text-sm transition-colors flex items-center gap-2"
          @click="openCreateModal"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-4 w-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M12 4v16m8-8H4"
            />
          </svg>
          {{ t('autoReply.createRule') }}
        </button>
      </div>
    </div>

    <!-- Stats Cards -->
    <div class="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
      <div class="bg-white dark:bg-gray-700 rounded-lg p-4 shadow">
        <div class="text-2xl font-bold text-gray-900 dark:text-white">{{ stats.total }}</div>
        <div class="text-sm text-gray-500 dark:text-gray-400">{{ t('autoReply.totalRules') }}</div>
      </div>
      <div class="bg-white dark:bg-gray-700 rounded-lg p-4 shadow">
        <div class="text-2xl font-bold text-green-600 dark:text-green-400">{{ stats.enabled }}</div>
        <div class="text-sm text-gray-500 dark:text-gray-400">{{ t('autoReply.enabled') }}</div>
      </div>
      <div class="bg-white dark:bg-gray-700 rounded-lg p-4 shadow">
        <div class="text-2xl font-bold text-gray-500 dark:text-gray-400">{{ stats.disabled }}</div>
        <div class="text-sm text-gray-500 dark:text-gray-400">{{ t('autoReply.disabled') }}</div>
      </div>
      <div class="bg-white dark:bg-gray-700 rounded-lg p-4 shadow">
        <div class="text-2xl font-bold text-gray-900 dark:text-white dark:text-gray-900 dark:text-white">{{ stats.totalMatches }}</div>
        <div class="text-sm text-gray-500 dark:text-gray-400">{{ t('autoReply.totalMatches') }}</div>
      </div>
    </div>

    <!-- Filters -->
    <div class="bg-white dark:bg-gray-700 rounded-lg p-4 mb-6 shadow">
      <div class="flex flex-col md:flex-row gap-4">
        <!-- Search -->
        <div class="flex-1">
          <div class="relative">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-5 w-5 absolute left-3 top-1/2 -translate-y-1/2 text-gray-400"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
              />
            </svg>
            <input
              v-model="searchQuery"
              type="text"
              :placeholder="t('autoReply.searchRules')"
              class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg pl-10 pr-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400 border border-gray-300 dark:border-gray-600"
            />
          </div>
        </div>

        <!-- Type Filter -->
        <select
          v-model="filterType"
          class="bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400 border border-gray-300 dark:border-gray-600"
        >
          <option value="all">{{ t('autoReply.allTypes') }}</option>
          <option value="keyword">{{ t('autoReply.keyword') }}</option>
          <option value="contains">{{ t('autoReply.contains') }}</option>
          <option value="prefix">{{ t('autoReply.prefix') }}</option>
          <option value="suffix">{{ t('autoReply.suffix') }}</option>
          <option value="regex">{{ t('autoReply.regex') }}</option>
        </select>

        <!-- Status Filter -->
        <select
          v-model="filterStatus"
          class="bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400 border border-gray-300 dark:border-gray-600"
        >
          <option value="all">{{ t('autoReply.allStatus') }}</option>
          <option value="enabled">{{ t('autoReply.enabled') }}</option>
          <option value="disabled">{{ t('autoReply.disabled') }}</option>
        </select>
      </div>
    </div>

    <!-- Error Message -->
    <div
      v-if="autoReplyStore.error"
      class="bg-red-900/30 border border-red-800 rounded-lg p-4 mb-6 text-red-300"
    >
      {{ autoReplyStore.error }}
    </div>

    <!-- Rules Grid -->
    <div v-if="filteredRules.length > 0" class="grid md:grid-cols-2 lg:grid-cols-3 gap-4">
      <AutoReplyRuleCard
        v-for="rule in filteredRules"
        :key="rule.id"
        :rule="rule"
        :loading="autoReplyStore.loading"
        @toggle="handleToggle(rule)"
        @edit="openEditModal(rule)"
        @delete="openDeleteConfirm(rule)"
        @test="openTestModal(rule)"
      />
    </div>

    <!-- Empty State -->
    <div v-else class="bg-white dark:bg-gray-700 rounded-lg p-12 text-center shadow">
      <svg
        xmlns="http://www.w3.org/2000/svg"
        class="h-12 w-12 mx-auto text-gray-600 mb-4"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M8 10h.01M12 10h.01M16 10h.01M9 16H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-5l-5 5v-5z"
        />
      </svg>
      <h3 class="text-lg font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('autoReply.noRulesFound') }}</h3>
      <p class="text-gray-500 dark:text-gray-500 mb-4">
        {{
          searchQuery || filterType !== 'all' || filterStatus !== 'all'
            ? t('autoReply.tryAdjustingFilters')
            : t('autoReply.createFirstRule')
        }}
      </p>
      <button
        v-if="!searchQuery && filterType === 'all' && filterStatus === 'all'"
        class="px-4 py-2 bg-gray-700 dark:bg-gray-700 hover:bg-gray-700 dark:bg-gray-700 text-white rounded-lg transition-colors"
        @click="openCreateModal"
      >
        {{ t('autoReply.createRule') }}
      </button>
    </div>

    <!-- Create Modal -->
    <div
      v-if="showCreateModal"
      class="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4"
      @click.self="closeModals"
    >
      <div class="bg-white dark:bg-gray-700 rounded-lg max-w-lg w-full max-h-[90vh] overflow-y-auto shadow-xl">
        <div class="p-6">
          <div class="flex items-center justify-between mb-4">
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('autoReply.createRule') }}</h3>
            <button class="p-1 text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-white" @click="closeModals">
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-5 w-5"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
          </div>
          <AutoReplyRuleForm
            :loading="autoReplyStore.loading"
            @save="handleCreate"
            @cancel="closeModals"
          />
        </div>
      </div>
    </div>

    <!-- Edit Modal -->
    <div
      v-if="showEditModal && selectedRule"
      class="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4"
      @click.self="closeModals"
    >
      <div class="bg-white dark:bg-gray-700 rounded-lg max-w-lg w-full max-h-[90vh] overflow-y-auto shadow-xl">
        <div class="p-6">
          <div class="flex items-center justify-between mb-4">
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('autoReply.editRule', { name: selectedRule.name }) }}</h3>
            <button class="p-1 text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-white" @click="closeModals">
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-5 w-5"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
          </div>
          <AutoReplyRuleForm
            :rule="selectedRule"
            :loading="autoReplyStore.loading"
            @save="handleUpdate"
            @cancel="closeModals"
          />
        </div>
      </div>
    </div>

    <!-- Test Modal -->
    <div
      v-if="showTestModal"
      class="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4"
      @click.self="closeModals"
    >
      <div class="bg-white dark:bg-gray-700 rounded-lg max-w-lg w-full max-h-[90vh] overflow-y-auto shadow-xl">
        <div class="p-6">
          <div class="flex items-center justify-between mb-4">
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('autoReply.testAutoReplyRules') }}</h3>
            <button class="p-1 text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-white" @click="closeModals">
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-5 w-5"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
          </div>
          <AutoReplyTestPanel
            ref="testPanelRef"
            :loading="autoReplyStore.loading"
            @test="handleTest"
            @close="closeModals"
          />
        </div>
      </div>
    </div>

    <!-- Delete Confirmation Modal -->
    <div
      v-if="showDeleteConfirm && selectedRule"
      class="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4"
      @click.self="closeModals"
    >
      <div class="bg-white dark:bg-gray-700 rounded-lg max-w-md w-full p-6 shadow-xl">
        <div class="flex items-center gap-3 mb-4">
          <div class="p-2 bg-red-100 dark:bg-red-600/20 rounded-full">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-6 w-6 text-red-500 dark:text-red-400"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
              />
            </svg>
          </div>
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('autoReply.deleteRule') }}</h3>
        </div>
        <p class="text-gray-600 dark:text-gray-300 mb-6">
          {{ t('autoReply.deleteConfirmation', { name: selectedRule.name }) }}
        </p>
        <div class="flex justify-end gap-3">
          <button
            class="px-4 py-2 bg-gray-700 dark:bg-gray-700 hover:bg-gray-300 dark:hover:bg-gray-600 text-gray-700 dark:text-white rounded-lg transition-colors"
            @click="closeModals"
          >
            {{ t('common.cancel') }}
          </button>
          <button
            class="px-4 py-2 bg-red-600 hover:bg-red-500 text-white rounded-lg transition-colors"
            :disabled="autoReplyStore.loading"
            @click="handleDelete"
          >
            {{ autoReplyStore.loading ? t('autoReply.deleting') : t('common.delete') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
