<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useSettingsStore } from '@/stores/settings'

const settingsStore = useSettingsStore()

const showApiKey = ref<Record<string, boolean>>({})
const tempApiKeys = ref<Record<string, string>>({})
const saveStatus = ref<string | null>(null)

const temperatureDisplay = computed(() => settingsStore.temperature.toFixed(1))

function toggleShowApiKey(provider: string) {
  showApiKey.value = {
    ...showApiKey.value,
    [provider]: !showApiKey.value[provider],
  }
}

function handleApiKeyInput(provider: string, value: string) {
  tempApiKeys.value = { ...tempApiKeys.value, [provider]: value }
}

function saveApiKey(provider: string) {
  const key = tempApiKeys.value[provider]
  if (key) {
    settingsStore.setApiKey(provider, key)
    tempApiKeys.value = { ...tempApiKeys.value, [provider]: '' }
    showSaveStatus('API key saved')
  }
}

function clearApiKey(provider: string) {
  settingsStore.clearApiKey(provider)
  showSaveStatus('API key cleared')
}

function showSaveStatus(message: string) {
  saveStatus.value = message
  setTimeout(() => {
    saveStatus.value = null
  }, 2000)
}

function getApiKeyMask(provider: string): string {
  const key = settingsStore.apiKeys[provider]
  if (!key) return ''
  if (key.length <= 8) return '••••••••'
  return key.slice(0, 4) + '••••••••' + key.slice(-4)
}

onMounted(async () => {
  await settingsStore.fetchProviders()
  await settingsStore.fetchTools()
})
</script>

<template>
  <div class="settings-view p-6 max-w-4xl mx-auto">
    <h1 class="text-2xl font-bold text-white mb-6">Settings</h1>

    <!-- Save status notification -->
    <div
      v-if="saveStatus"
      class="fixed top-20 right-4 bg-green-600 text-white px-4 py-2 rounded-lg shadow-lg z-50"
    >
      {{ saveStatus }}
    </div>

    <!-- LLM Provider Settings -->
    <section class="mb-8">
      <h2 class="text-lg font-semibold text-white mb-4 flex items-center gap-2">
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
            d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
          />
        </svg>
        LLM Provider
      </h2>

      <div class="bg-gray-800 rounded-lg p-4 space-y-4">
        <!-- Provider selection -->
        <div>
          <label class="block text-sm text-gray-400 mb-2">Provider</label>
          <select
            :value="settingsStore.selectedProvider"
            class="w-full bg-gray-700 text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500"
            @change="settingsStore.setProvider(($event.target as HTMLSelectElement).value)"
          >
            <option
              v-for="provider in settingsStore.providers"
              :key="provider.name"
              :value="provider.name"
            >
              {{ provider.name }}
            </option>
          </select>
        </div>

        <!-- Model selection -->
        <div>
          <label class="block text-sm text-gray-400 mb-2">Model</label>
          <select
            :value="settingsStore.selectedModel"
            class="w-full bg-gray-700 text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500"
            @change="settingsStore.setModel(($event.target as HTMLSelectElement).value)"
          >
            <option
              v-for="model in settingsStore.availableModels"
              :key="model"
              :value="model"
            >
              {{ model }}
            </option>
          </select>
        </div>
      </div>
    </section>

    <!-- API Keys -->
    <section class="mb-8">
      <h2 class="text-lg font-semibold text-white mb-4 flex items-center gap-2">
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
            d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"
          />
        </svg>
        API Keys
      </h2>

      <div class="space-y-4">
        <div
          v-for="provider in settingsStore.providers"
          :key="provider.name"
          class="bg-gray-800 rounded-lg p-4"
        >
          <div class="flex items-center justify-between mb-2">
            <h3 class="text-white font-medium capitalize">{{ provider.name }}</h3>
            <span
              v-if="provider.name === 'ollama'"
              class="text-xs text-green-400 bg-green-900/50 px-2 py-1 rounded"
            >
              No API key required
            </span>
            <span
              v-else-if="settingsStore.apiKeys[provider.name]"
              class="text-xs text-green-400 bg-green-900/50 px-2 py-1 rounded"
            >
              Configured
            </span>
            <span
              v-else
              class="text-xs text-yellow-400 bg-yellow-900/50 px-2 py-1 rounded"
            >
              Not configured
            </span>
          </div>

          <div v-if="provider.name !== 'ollama'" class="space-y-2">
            <!-- Current key display -->
            <div v-if="settingsStore.apiKeys[provider.name]" class="flex items-center gap-2">
              <input
                :type="showApiKey[provider.name] ? 'text' : 'password'"
                :value="showApiKey[provider.name] ? settingsStore.apiKeys[provider.name] : getApiKeyMask(provider.name)"
                readonly
                class="flex-1 bg-gray-700 text-white rounded px-3 py-2 text-sm"
              />
              <button
                class="p-2 text-gray-400 hover:text-white"
                @click="toggleShowApiKey(provider.name)"
              >
                <svg
                  v-if="showApiKey[provider.name]"
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
                    d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21"
                  />
                </svg>
                <svg
                  v-else
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
                    d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
                  />
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"
                  />
                </svg>
              </button>
              <button
                class="p-2 text-red-400 hover:text-red-300"
                title="Clear API key"
                @click="clearApiKey(provider.name)"
              >
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
                    d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                  />
                </svg>
              </button>
            </div>

            <!-- New key input -->
            <div class="flex items-center gap-2">
              <input
                type="password"
                :value="tempApiKeys[provider.name] || ''"
                :placeholder="settingsStore.apiKeys[provider.name] ? 'Enter new API key...' : 'Enter API key...'"
                class="flex-1 bg-gray-700 text-white rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                @input="handleApiKeyInput(provider.name, ($event.target as HTMLInputElement).value)"
              />
              <button
                :disabled="!tempApiKeys[provider.name]"
                class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded text-sm disabled:opacity-50 disabled:cursor-not-allowed"
                @click="saveApiKey(provider.name)"
              >
                Save
              </button>
            </div>
          </div>

          <p v-else class="text-sm text-gray-400">
            Ollama runs locally and doesn't require an API key.
          </p>
        </div>
      </div>
    </section>

    <!-- Model Parameters -->
    <section class="mb-8">
      <h2 class="text-lg font-semibold text-white mb-4 flex items-center gap-2">
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
            d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4"
          />
        </svg>
        Model Parameters
      </h2>

      <div class="bg-gray-800 rounded-lg p-4 space-y-6">
        <!-- Temperature -->
        <div>
          <div class="flex items-center justify-between mb-2">
            <label class="text-sm text-gray-400">Temperature</label>
            <span class="text-sm text-white font-mono">{{ temperatureDisplay }}</span>
          </div>
          <input
            type="range"
            :value="settingsStore.temperature"
            min="0"
            max="2"
            step="0.1"
            class="w-full h-2 bg-gray-700 rounded-lg appearance-none cursor-pointer"
            @input="settingsStore.setTemperature(parseFloat(($event.target as HTMLInputElement).value))"
          />
          <div class="flex justify-between text-xs text-gray-500 mt-1">
            <span>Precise</span>
            <span>Creative</span>
          </div>
        </div>

        <!-- Max Tokens -->
        <div>
          <div class="flex items-center justify-between mb-2">
            <label class="text-sm text-gray-400">Max Tokens</label>
            <span class="text-sm text-white font-mono">{{ settingsStore.maxTokens }}</span>
          </div>
          <input
            type="range"
            :value="settingsStore.maxTokens"
            min="256"
            max="8192"
            step="256"
            class="w-full h-2 bg-gray-700 rounded-lg appearance-none cursor-pointer"
            @input="settingsStore.setMaxTokens(parseInt(($event.target as HTMLInputElement).value))"
          />
          <div class="flex justify-between text-xs text-gray-500 mt-1">
            <span>256</span>
            <span>8192</span>
          </div>
        </div>
      </div>
    </section>

    <!-- Available Tools -->
    <section class="mb-8">
      <h2 class="text-lg font-semibold text-white mb-4 flex items-center gap-2">
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
            d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"
          />
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
          />
        </svg>
        Available Tools
      </h2>

      <div class="bg-gray-800 rounded-lg p-4">
        <div v-if="settingsStore.tools.length === 0" class="text-gray-400 text-center py-4">
          No tools available
        </div>
        <div v-else class="space-y-3">
          <div
            v-for="tool in settingsStore.tools"
            :key="tool.name"
            class="flex items-start gap-3 p-3 bg-gray-700/50 rounded-lg"
          >
            <div class="w-8 h-8 rounded bg-blue-600/20 flex items-center justify-center flex-shrink-0">
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-4 w-4 text-blue-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M13 10V3L4 14h7v7l9-11h-7z"
                />
              </svg>
            </div>
            <div>
              <h4 class="text-white font-medium">{{ tool.name }}</h4>
              <p class="text-sm text-gray-400">{{ tool.description }}</p>
            </div>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
input[type='range'] {
  -webkit-appearance: none;
}

input[type='range']::-webkit-slider-thumb {
  -webkit-appearance: none;
  width: 16px;
  height: 16px;
  background: #3b82f6;
  border-radius: 50%;
  cursor: pointer;
}

input[type='range']::-moz-range-thumb {
  width: 16px;
  height: 16px;
  background: #3b82f6;
  border-radius: 50%;
  cursor: pointer;
  border: none;
}
</style>
