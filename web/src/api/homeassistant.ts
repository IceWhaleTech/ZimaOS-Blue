import api from './index'

// Types
export interface HAEntity {
  entity_id: string
  state: string
  attributes: Record<string, unknown>
  last_changed: string
  last_updated: string
}

export interface HAScene {
  entity_id: string
  name: string
  icon?: string
}

export interface HAAutomation {
  entity_id: string
  name: string
  state: string
  last_triggered?: string
}

export interface HACommandResult {
  success: boolean
  message: string
  entity_id?: string
  action?: string
  parameters?: Record<string, unknown>
  suggestions?: string[]
}

export interface HAConnectionStatus {
  connected: boolean
}

// API functions
export async function connect(url: string, token: string): Promise<void> {
  await api.post('/api/homeassistant/connect', { url, token })
}

export async function disconnect(): Promise<void> {
  await api.post('/api/homeassistant/disconnect')
}

export async function getStatus(): Promise<HAConnectionStatus> {
  const response = await api.get('/api/homeassistant/status')
  return response.data
}

export async function getEntities(domain?: string): Promise<HAEntity[]> {
  const params = domain ? { domain } : {}
  const response = await api.get('/api/homeassistant/entities', { params })
  return response.data
}

export async function getEntity(entityId: string): Promise<HAEntity> {
  const response = await api.get(`/api/homeassistant/entities/${encodeURIComponent(entityId)}`)
  return response.data
}

export async function controlEntity(
  entityId: string,
  action: string,
  parameters?: Record<string, unknown>
): Promise<void> {
  await api.post(`/api/homeassistant/entities/${encodeURIComponent(entityId)}/control`, {
    action,
    parameters,
  })
}

export async function getScenes(): Promise<HAScene[]> {
  const response = await api.get('/api/homeassistant/scenes')
  return response.data
}

export async function activateScene(sceneId: string): Promise<void> {
  await api.post(`/api/homeassistant/scenes/${encodeURIComponent(sceneId)}/activate`)
}

export async function getAutomations(): Promise<HAAutomation[]> {
  const response = await api.get('/api/homeassistant/automations')
  return response.data
}

export async function triggerAutomation(automationId: string): Promise<void> {
  await api.post(`/api/homeassistant/automations/${encodeURIComponent(automationId)}/trigger`)
}

export async function toggleAutomation(automationId: string, enable: boolean): Promise<void> {
  await api.post(`/api/homeassistant/automations/${encodeURIComponent(automationId)}/toggle`, {
    enable,
  })
}

export async function processCommand(command: string): Promise<HACommandResult> {
  const response = await api.post('/api/homeassistant/command', { command })
  return response.data
}

// Helper functions
export function getEntityDomain(entityId: string): string {
  return entityId.split('.')[0] || ''
}

export function getEntityName(entity: HAEntity): string {
  return (entity.attributes.friendly_name as string) || entity.entity_id
}

export function getEntityIcon(entity: HAEntity): string {
  const icon = entity.attributes.icon as string
  if (icon) {
    return icon.replace('mdi:', '')
  }

  // Default icons by domain
  const domain = getEntityDomain(entity.entity_id)
  const defaultIcons: Record<string, string> = {
    light: 'lightbulb',
    switch: 'toggle-switch',
    sensor: 'eye',
    binary_sensor: 'checkbox-blank-circle',
    climate: 'thermostat',
    cover: 'window-shutter',
    lock: 'lock',
    camera: 'camera',
    media_player: 'play-circle',
    fan: 'fan',
    vacuum: 'robot-vacuum',
    scene: 'palette',
    automation: 'robot',
    script: 'script-text',
    input_boolean: 'toggle-switch-outline',
    input_number: 'ray-vertex',
    input_select: 'format-list-bulleted',
    input_text: 'form-textbox',
    person: 'account',
    zone: 'map-marker',
    sun: 'weather-sunny',
    weather: 'weather-partly-cloudy',
  }

  return defaultIcons[domain] || 'help-circle'
}

export function isEntityOn(entity: HAEntity): boolean {
  const onStates = ['on', 'open', 'unlocked', 'playing', 'home', 'active']
  return onStates.includes(entity.state.toLowerCase())
}

export function canToggle(entity: HAEntity): boolean {
  const toggleableDomains = [
    'light',
    'switch',
    'fan',
    'input_boolean',
    'automation',
    'script',
    'media_player',
  ]
  return toggleableDomains.includes(getEntityDomain(entity.entity_id))
}

export function formatState(entity: HAEntity): string {
  const state = entity.state

  // Handle unavailable/unknown
  if (state === 'unavailable' || state === 'unknown') {
    return state
  }

  // Handle specific domains
  const domain = getEntityDomain(entity.entity_id)

  switch (domain) {
    case 'sensor':
    case 'binary_sensor': {
      const unit = entity.attributes.unit_of_measurement as string
      return unit ? `${state} ${unit}` : state
    }
    case 'climate': {
      const temp = entity.attributes.current_temperature as number
      const unit = entity.attributes.temperature_unit as string
      if (temp !== undefined) {
        return `${state} (${temp}${unit || '°'})`
      }
      return state
    }
    case 'cover': {
      const position = entity.attributes.current_position as number
      if (position !== undefined) {
        return `${state} (${position}%)`
      }
      return state
    }
    case 'light': {
      if (state === 'on') {
        const brightness = entity.attributes.brightness as number
        if (brightness !== undefined) {
          const percent = Math.round((brightness / 255) * 100)
          return `on (${percent}%)`
        }
      }
      return state
    }
    default:
      return state
  }
}
