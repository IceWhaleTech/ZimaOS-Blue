<template>
  <div class="home-assistant-view">
    <div class="header">
      <h1>{{ t('homeAssistant.title') }}</h1>
      <div class="connection-status" :class="{ connected: isConnected }">
        <span class="status-dot"></span>
        {{ isConnected ? t('homeAssistant.connected') : t('homeAssistant.disconnected') }}
      </div>
    </div>

    <!-- Connection Form -->
    <div v-if="!isConnected" class="connection-form card">
      <h2>{{ t('homeAssistant.connectTitle') }}</h2>
      <form @submit.prevent="handleConnect">
        <div class="form-group">
          <label for="ha-url">{{ t('homeAssistant.url') }}</label>
          <input
            id="ha-url"
            v-model="connectionForm.url"
            type="url"
            :placeholder="t('homeAssistant.urlPlaceholder')"
            required
          />
        </div>
        <div class="form-group">
          <label for="ha-token">{{ t('homeAssistant.token') }}</label>
          <input
            id="ha-token"
            v-model="connectionForm.token"
            type="password"
            :placeholder="t('homeAssistant.tokenPlaceholder')"
            required
          />
          <small>
            {{ t('homeAssistant.tokenHint') }}
          </small>
        </div>
        <button type="submit" class="btn btn-primary" :disabled="connecting">
          {{ connecting ? t('homeAssistant.connecting') : t('homeAssistant.connect') }}
        </button>
      </form>
    </div>

    <!-- Connected View -->
    <template v-else>
      <!-- Voice Command -->
      <div class="voice-command card">
        <h2>{{ t('homeAssistant.voiceCommand') }}</h2>
        <div class="command-input">
          <input
            v-model="voiceCommand"
            type="text"
            :placeholder="t('homeAssistant.voicePlaceholder')"
            @keyup.enter="handleVoiceCommand"
          />
          <button class="btn btn-primary" :disabled="processingCommand" @click="handleVoiceCommand">
            {{ processingCommand ? t('homeAssistant.processing') : t('homeAssistant.send') }}
          </button>
        </div>
        <div v-if="commandResult" class="command-result" :class="{ success: commandResult.success }">
          <p>{{ commandResult.message }}</p>
          <div v-if="commandResult.suggestions" class="suggestions">
            <p>{{ t('homeAssistant.trySuggestions') }}</p>
            <ul>
              <li v-for="suggestion in commandResult.suggestions" :key="suggestion">
                {{ suggestion }}
              </li>
            </ul>
          </div>
        </div>
      </div>

      <!-- Quick Actions -->
      <div class="quick-actions card">
        <h2>{{ t('homeAssistant.scenes') }}</h2>
        <div class="scenes-grid">
          <button
            v-for="scene in scenes"
            :key="scene.entity_id"
            class="scene-btn"
            @click="activateScene(scene.entity_id)"
          >
            <span class="scene-icon">{{ getSceneEmoji(scene.name) }}</span>
            <span class="scene-name">{{ scene.name }}</span>
          </button>
        </div>
      </div>

      <!-- Domain Tabs -->
      <div class="domain-tabs">
        <button
          v-for="domain in availableDomains"
          :key="domain"
          class="tab-btn"
          :class="{ active: selectedDomain === domain }"
          @click="selectedDomain = domain"
        >
          {{ formatDomainName(domain) }}
        </button>
      </div>

      <!-- Entities Grid -->
      <div class="entities-grid">
        <div
          v-for="entity in filteredEntities"
          :key="entity.entity_id"
          class="entity-card"
          :class="{ on: isEntityOn(entity) }"
        >
          <div class="entity-header">
            <span class="entity-icon">{{ getEntityEmoji(entity) }}</span>
            <span class="entity-name">{{ getEntityName(entity) }}</span>
          </div>
          <div class="entity-state">{{ formatState(entity) }}</div>
          <div class="entity-actions">
            <button
              v-if="canToggle(entity)"
              class="btn btn-sm"
              :class="isEntityOn(entity) ? 'btn-danger' : 'btn-success'"
              @click="toggleEntity(entity)"
            >
              {{ isEntityOn(entity) ? t('homeAssistant.turnOff') : t('homeAssistant.turnOn') }}
            </button>
            <input
              v-if="getEntityDomain(entity.entity_id) === 'light' && isEntityOn(entity)"
              type="range"
              min="0"
              max="100"
              :value="getBrightness(entity)"
              class="brightness-slider"
              @change="setBrightness(entity, $event)"
            />
          </div>
        </div>
      </div>

      <!-- Automations -->
      <div class="automations card">
        <h2>{{ t('homeAssistant.automations') }}</h2>
        <div class="automations-list">
          <div
            v-for="automation in automations"
            :key="automation.entity_id"
            class="automation-item"
          >
            <div class="automation-info">
              <span class="automation-name">{{ automation.name }}</span>
              <span class="automation-status" :class="automation.state">
                {{ automation.state }}
              </span>
            </div>
            <div class="automation-actions">
              <button
                class="btn btn-sm btn-secondary"
                @click="triggerAutomation(automation.entity_id)"
              >
                {{ t('homeAssistant.trigger') }}
              </button>
              <button
                class="btn btn-sm"
                :class="automation.state === 'on' ? 'btn-danger' : 'btn-success'"
                @click="toggleAutomation(automation.entity_id, automation.state !== 'on')"
              >
                {{ automation.state === 'on' ? t('homeAssistant.disable') : t('homeAssistant.enable') }}
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- Disconnect -->
      <div class="disconnect-section">
        <button class="btn btn-danger" @click="handleDisconnect">
          {{ t('homeAssistant.disconnect') }}
        </button>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import * as haApi from '@/api/homeassistant'
import type { HAEntity, HAScene, HAAutomation, HACommandResult } from '@/api/homeassistant'

const { t, te } = useI18n()

// State
const isConnected = ref(false)
const connecting = ref(false)
const connectionForm = ref({
  url: '',
  token: '',
})

const entities = ref<HAEntity[]>([])
const scenes = ref<HAScene[]>([])
const automations = ref<HAAutomation[]>([])
const selectedDomain = ref('light')

const voiceCommand = ref('')
const processingCommand = ref(false)
const commandResult = ref<HACommandResult | null>(null)

// Computed
const availableDomains = computed(() => {
  const domains = new Set<string>()
  entities.value.forEach((entity) => {
    domains.add(haApi.getEntityDomain(entity.entity_id))
  })
  // Filter to common controllable domains
  const controllable = ['light', 'switch', 'climate', 'cover', 'lock', 'fan', 'media_player']
  return controllable.filter((d) => domains.has(d))
})

const filteredEntities = computed(() => {
  return entities.value.filter(
    (entity) => haApi.getEntityDomain(entity.entity_id) === selectedDomain.value
  )
})

// Methods
async function checkConnection() {
  try {
    const status = await haApi.getStatus()
    isConnected.value = status.connected
    if (status.connected) {
      await loadData()
    }
  } catch {
    isConnected.value = false
  }
}

async function handleConnect() {
  connecting.value = true
  try {
    await haApi.connect(connectionForm.value.url, connectionForm.value.token)
    isConnected.value = true
    await loadData()
  } catch (error) {
    console.error('Failed to connect:', error)
    alert(t('homeAssistant.connectionFailed'))
  } finally {
    connecting.value = false
  }
}

async function handleDisconnect() {
  try {
    await haApi.disconnect()
    isConnected.value = false
    entities.value = []
    scenes.value = []
    automations.value = []
  } catch (error) {
    console.error('Failed to disconnect:', error)
  }
}

async function loadData() {
  try {
    const [entitiesData, scenesData, automationsData] = await Promise.all([
      haApi.getEntities(),
      haApi.getScenes(),
      haApi.getAutomations(),
    ])
    entities.value = entitiesData
    scenes.value = scenesData
    automations.value = automationsData
  } catch (error) {
    console.error('Failed to load data:', error)
  }
}

async function handleVoiceCommand() {
  if (!voiceCommand.value.trim()) return

  processingCommand.value = true
  commandResult.value = null

  try {
    commandResult.value = await haApi.processCommand(voiceCommand.value)
    if (commandResult.value.success) {
      voiceCommand.value = ''
      // Refresh entities to show updated state
      await loadData()
    }
  } catch (error) {
    console.error('Command failed:', error)
    commandResult.value = {
      success: false,
      message: t('homeAssistant.commandFailed'),
    }
  } finally {
    processingCommand.value = false
  }
}

async function toggleEntity(entity: HAEntity) {
  const action = haApi.isEntityOn(entity) ? 'turn_off' : 'turn_on'
  try {
    await haApi.controlEntity(entity.entity_id, action)
    // Update local state
    entity.state = action === 'turn_on' ? 'on' : 'off'
  } catch (error) {
    console.error('Failed to toggle entity:', error)
  }
}

async function setBrightness(entity: HAEntity, event: Event) {
  const target = event.target as HTMLInputElement
  const brightness = Math.round((parseInt(target.value) / 100) * 255)
  try {
    await haApi.controlEntity(entity.entity_id, 'turn_on', { brightness })
  } catch (error) {
    console.error('Failed to set brightness:', error)
  }
}

async function activateScene(sceneId: string) {
  try {
    await haApi.activateScene(sceneId)
    await loadData()
  } catch (error) {
    console.error('Failed to activate scene:', error)
  }
}

async function triggerAutomation(automationId: string) {
  try {
    await haApi.triggerAutomation(automationId)
  } catch (error) {
    console.error('Failed to trigger automation:', error)
  }
}

async function toggleAutomation(automationId: string, enable: boolean) {
  try {
    await haApi.toggleAutomation(automationId, enable)
    // Update local state
    const automation = automations.value.find((a) => a.entity_id === automationId)
    if (automation) {
      automation.state = enable ? 'on' : 'off'
    }
  } catch (error) {
    console.error('Failed to toggle automation:', error)
  }
}

// Helper functions
function getEntityName(entity: HAEntity): string {
  return haApi.getEntityName(entity)
}

function getEntityDomain(entityId: string): string {
  return haApi.getEntityDomain(entityId)
}

function isEntityOn(entity: HAEntity): boolean {
  return haApi.isEntityOn(entity)
}

function canToggle(entity: HAEntity): boolean {
  return haApi.canToggle(entity)
}

function formatState(entity: HAEntity): string {
  return haApi.formatState(entity)
}

function getBrightness(entity: HAEntity): number {
  const brightness = entity.attributes.brightness as number
  if (brightness !== undefined) {
    return Math.round((brightness / 255) * 100)
  }
  return 100
}

function formatDomainName(domain: string): string {
  const key = `homeAssistant.domains.${domain}`
  return te(key) ? t(key) : domain
}

function getEntityEmoji(entity: HAEntity): string {
  const domain = getEntityDomain(entity.entity_id)
  const emojis: Record<string, string> = {
    light: '💡',
    switch: '🔌',
    climate: '🌡️',
    cover: '🪟',
    lock: '🔒',
    fan: '🌀',
    media_player: '📺',
    sensor: '📊',
    binary_sensor: '⚡',
    camera: '📷',
    vacuum: '🤖',
  }
  return emojis[domain] || '📦'
}

function getSceneEmoji(name: string): string {
  const nameLower = name.toLowerCase()
  if (nameLower.includes('movie') || nameLower.includes('cinema')) return '🎬'
  if (nameLower.includes('night') || nameLower.includes('sleep')) return '🌙'
  if (nameLower.includes('morning') || nameLower.includes('wake')) return '☀️'
  if (nameLower.includes('party')) return '🎉'
  if (nameLower.includes('romantic') || nameLower.includes('dinner')) return '🕯️'
  if (nameLower.includes('work') || nameLower.includes('focus')) return '💼'
  if (nameLower.includes('relax') || nameLower.includes('chill')) return '🛋️'
  return '🎨'
}

// Lifecycle
onMounted(() => {
  checkConnection()
})
</script>

<style scoped>
.home-assistant-view {
  padding: 1.5rem;
  max-width: 1200px;
  margin: 0 auto;
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1.5rem;
}

.header h1 {
  margin: 0;
  font-size: 1.75rem;
}

.connection-status {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 1rem;
  border-radius: 2rem;
  background: var(--color-background-soft);
  font-size: 0.875rem;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #ef4444;
}

.connection-status.connected .status-dot {
  background: #22c55e;
}

.card {
  background: var(--color-background-soft);
  border-radius: 0.75rem;
  padding: 1.5rem;
  margin-bottom: 1.5rem;
}

.card h2 {
  margin: 0 0 1rem 0;
  font-size: 1.25rem;
}

.form-group {
  margin-bottom: 1rem;
}

.form-group label {
  display: block;
  margin-bottom: 0.5rem;
  font-weight: 500;
}

.form-group input {
  width: 100%;
  padding: 0.75rem;
  border: 1px solid var(--color-border);
  border-radius: 0.5rem;
  background: var(--color-background);
  color: var(--color-text);
}

.form-group small {
  display: block;
  margin-top: 0.25rem;
  color: var(--color-text-muted);
  font-size: 0.75rem;
}

.btn {
  padding: 0.75rem 1.5rem;
  border: none;
  border-radius: 0.5rem;
  cursor: pointer;
  font-weight: 500;
  transition: opacity 0.2s;
}

.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-primary {
  background: var(--color-primary);
  color: white;
}

.btn-secondary {
  background: var(--color-background);
  color: var(--color-text);
  border: 1px solid var(--color-border);
}

.btn-success {
  background: #22c55e;
  color: white;
}

.btn-danger {
  background: #ef4444;
  color: white;
}

.btn-sm {
  padding: 0.5rem 1rem;
  font-size: 0.875rem;
}

.voice-command .command-input {
  display: flex;
  gap: 0.5rem;
}

.voice-command input {
  flex: 1;
  padding: 0.75rem;
  border: 1px solid var(--color-border);
  border-radius: 0.5rem;
  background: var(--color-background);
  color: var(--color-text);
}

.command-result {
  margin-top: 1rem;
  padding: 1rem;
  border-radius: 0.5rem;
  background: #fef2f2;
  color: #991b1b;
}

.command-result.success {
  background: #f0fdf4;
  color: #166534;
}

.command-result .suggestions {
  margin-top: 0.5rem;
}

.command-result .suggestions ul {
  margin: 0.5rem 0 0 1.5rem;
  padding: 0;
}

.scenes-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));
  gap: 0.75rem;
}

.scene-btn {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
  padding: 1rem;
  border: 1px solid var(--color-border);
  border-radius: 0.75rem;
  background: var(--color-background);
  cursor: pointer;
  transition: all 0.2s;
}

.scene-btn:hover {
  background: var(--color-background-soft);
  border-color: var(--color-primary);
}

.scene-icon {
  font-size: 1.5rem;
}

.scene-name {
  font-size: 0.875rem;
  text-align: center;
}

.domain-tabs {
  display: flex;
  gap: 0.5rem;
  margin-bottom: 1rem;
  overflow-x: auto;
  padding-bottom: 0.5rem;
}

.tab-btn {
  padding: 0.5rem 1rem;
  border: 1px solid var(--color-border);
  border-radius: 2rem;
  background: var(--color-background);
  cursor: pointer;
  white-space: nowrap;
  transition: all 0.2s;
}

.tab-btn.active {
  background: var(--color-primary);
  color: white;
  border-color: var(--color-primary);
}

.entities-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 1rem;
  margin-bottom: 1.5rem;
}

.entity-card {
  background: var(--color-background-soft);
  border-radius: 0.75rem;
  padding: 1rem;
  transition: all 0.2s;
}

.entity-card.on {
  background: linear-gradient(135deg, #fef3c7 0%, #fde68a 100%);
}

.entity-header {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-bottom: 0.5rem;
}

.entity-icon {
  font-size: 1.25rem;
}

.entity-name {
  font-weight: 500;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.entity-state {
  color: var(--color-text-muted);
  font-size: 0.875rem;
  margin-bottom: 0.75rem;
}

.entity-actions {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.brightness-slider {
  width: 100%;
  height: 4px;
  -webkit-appearance: none;
  appearance: none;
  background: var(--color-border);
  border-radius: 2px;
  outline: none;
}

.brightness-slider::-webkit-slider-thumb {
  -webkit-appearance: none;
  appearance: none;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: var(--color-primary);
  cursor: pointer;
}

.automations-list {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.automation-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem;
  background: var(--color-background);
  border-radius: 0.5rem;
}

.automation-info {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.automation-name {
  font-weight: 500;
}

.automation-status {
  font-size: 0.75rem;
  padding: 0.125rem 0.5rem;
  border-radius: 1rem;
  width: fit-content;
}

.automation-status.on {
  background: #dcfce7;
  color: #166534;
}

.automation-status.off {
  background: #f3f4f6;
  color: #6b7280;
}

.automation-actions {
  display: flex;
  gap: 0.5rem;
}

.disconnect-section {
  text-align: center;
  padding: 1rem;
}
</style>
