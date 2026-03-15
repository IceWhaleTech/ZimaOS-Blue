<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { approvalApi } from '@/api/approval'
import type { ApprovalConfig, Policy } from '@/api/approval'
import { toolApi } from '@/api/chat'

const { t, te } = useI18n()
const emit = defineEmits<{ 'status-change': [msg: string] }>()

const config = ref<ApprovalConfig>({
  enabled: true,
  default_policy: 'auto',
  tool_policies: {},
})
const tools = ref<{ name: string; description: string }[]>([])
const loading = ref(true)
const overridesExpanded = ref(false)

const policyOptions: { value: Policy; labelKey: string }[] = [
  { value: 'auto', labelKey: 'approval.policyAuto' },
  { value: 'ask', labelKey: 'approval.policyAsk' },
  { value: 'deny', labelKey: 'approval.policyDeny' },
]

function tr(key: string, fallback = ''): string {
  return te(key) ? t(key) : fallback
}

onMounted(async () => {
  try {
    const [configRes, toolsRes] = await Promise.all([approvalApi.getConfig(), toolApi.list()])
    config.value = configRes.data
    tools.value = toolsRes.data || []
  } catch (e) {
    console.error('Failed to load approval config:', e)
  } finally {
    loading.value = false
  }
})

async function save() {
  try {
    await approvalApi.updateConfig(config.value)
    emit('status-change', t('settings.saved', 'Saved'))
  } catch (e) {
    console.error('Failed to save approval config:', e)
  }
}

function toggleEnabled() {
  config.value.enabled = !config.value.enabled
  save()
}

function setDefaultPolicy(policy: Policy) {
  config.value.default_policy = policy
  save()
}

function setToolPolicy(toolName: string, policy: Policy) {
  if (policy === config.value.default_policy) {
    // Remove override if same as default
    delete config.value.tool_policies[toolName]
  } else {
    config.value.tool_policies[toolName] = policy
  }
  save()
}

function getToolPolicy(toolName: string): Policy {
  return config.value.tool_policies[toolName] || config.value.default_policy
}
</script>

<template>
  <div class="glass-card p-4">
    <div class="flex items-center justify-between">
      <div>
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
          {{ t('approval.settingsTitle', 'Tool Call Approval') }}
        </h3>
        <p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
          {{
            t('approval.settingsDesc', 'Control which tools require confirmation before execution')
          }}
        </p>
      </div>
      <button
        class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors cursor-pointer"
        :class="config.enabled ? 'bg-green-600 dark:bg-green-500' : 'bg-gray-300 dark:bg-gray-600'"
        @click="toggleEnabled"
      >
        <span
          class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform"
          :class="config.enabled ? 'translate-x-6' : 'translate-x-1'"
        />
      </button>
    </div>

    <template v-if="config.enabled && !loading">
      <!-- Default policy -->
      <div class="mt-3">
        <label class="text-xs text-gray-500 dark:text-gray-400 mb-1.5 block">
          {{ t('approval.defaultPolicy', 'Default Policy') }}
        </label>
        <div class="flex gap-1">
          <button
            v-for="opt in policyOptions"
            :key="opt.value"
            class="px-3 py-1.5 text-xs rounded-md transition-colors cursor-pointer"
            :class="
              config.default_policy === opt.value
                ? 'bg-gray-800 dark:bg-gray-200 text-white dark:text-gray-900'
                : 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600'
            "
            @click="setDefaultPolicy(opt.value)"
          >
            {{ tr(opt.labelKey, opt.value) }}
          </button>
        </div>
      </div>

      <!-- Per-tool overrides (collapsible) -->
      <div v-if="tools.length > 0" class="mt-3">
        <button
          class="flex items-center gap-1 text-xs text-gray-500 dark:text-gray-400 mb-1.5 cursor-pointer hover:text-gray-700 dark:hover:text-gray-300 transition-colors"
          @click="overridesExpanded = !overridesExpanded"
        >
          <svg
            class="w-3 h-3 transition-transform"
            :class="overridesExpanded ? 'rotate-90' : ''"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M9 5l7 7-7 7"
            />
          </svg>
          {{ t('approval.perTool', 'Per-Tool Overrides') }}
          <span class="text-gray-400 dark:text-gray-500">({{ tools.length }})</span>
        </button>
        <div v-show="overridesExpanded" class="space-y-1 max-h-64 overflow-y-auto">
          <div
            v-for="tool in tools"
            :key="tool.name"
            class="flex items-center justify-between py-1.5 px-2 rounded hover:bg-gray-50 dark:hover:bg-gray-700/50"
          >
            <div class="flex-1 min-w-0 mr-3">
              <span class="text-sm text-gray-800 dark:text-gray-200 truncate">{{
                t(`tools.names.${tool.name}`, tool.name)
              }}</span>
            </div>
            <div class="flex gap-0.5 flex-shrink-0">
              <button
                v-for="opt in policyOptions"
                :key="opt.value"
                class="px-2 py-1 text-[11px] rounded transition-colors cursor-pointer"
                :class="
                  getToolPolicy(tool.name) === opt.value
                    ? 'bg-gray-800 dark:bg-gray-200 text-white dark:text-gray-900'
                    : 'bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-400 hover:bg-gray-200 dark:hover:bg-gray-600'
                "
                @click="setToolPolicy(tool.name, opt.value)"
              >
                {{ tr(opt.labelKey, opt.value) }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>
