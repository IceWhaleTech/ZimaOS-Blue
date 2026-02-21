<template>
  <div class="browser-automation-view">
    <div class="header">
      <h1>{{ t('browserAutomation.title') }}</h1>
      <div class="header-actions">
        <button class="btn btn-primary" @click="showCreateModal = true">
          + {{ t('browserAutomation.newTask') }}
        </button>
      </div>
    </div>

    <!-- Stats Cards -->
    <div class="stats-grid">
      <div class="stat-card">
        <div class="stat-value">{{ tasks.length }}</div>
        <div class="stat-label">{{ t('browserAutomation.stats.totalTasks') }}</div>
      </div>
      <div class="stat-card">
        <div class="stat-value">{{ runningTasks }}</div>
        <div class="stat-label">{{ t('browserAutomation.stats.running') }}</div>
      </div>
      <div class="stat-card">
        <div class="stat-value">{{ completedTasks }}</div>
        <div class="stat-label">{{ t('browserAutomation.stats.completed') }}</div>
      </div>
      <div class="stat-card">
        <div class="stat-value">{{ sessions.length }}</div>
        <div class="stat-label">{{ t('browserAutomation.stats.activeSessions') }}</div>
      </div>
    </div>

    <!-- Tabs -->
    <div class="tabs">
      <button
        class="tab-btn"
        :class="{ active: activeTab === 'tasks' }"
        @click="activeTab = 'tasks'"
      >
        {{ t('browserAutomation.tabs.tasks') }}
      </button>
      <button
        class="tab-btn"
        :class="{ active: activeTab === 'sessions' }"
        @click="activeTab = 'sessions'"
      >
        {{ t('browserAutomation.tabs.sessions') }}
      </button>
      <button
        class="tab-btn"
        :class="{ active: activeTab === 'templates' }"
        @click="activeTab = 'templates'"
      >
        {{ t('browserAutomation.tabs.templates') }}
      </button>
      <button
        class="tab-btn"
        :class="{ active: activeTab === 'security' }"
        @click="activeTab = 'security'"
      >
        {{ t('browserAutomation.tabs.security') }}
      </button>
    </div>

    <!-- Tasks Tab -->
    <div v-if="activeTab === 'tasks'" class="tasks-section">
      <div v-if="tasks.length === 0" class="empty-state">
        <p>{{ t('browserAutomation.tasks.empty') }}</p>
        <button class="btn btn-primary" @click="showCreateModal = true">
          {{ t('browserAutomation.newTask') }}
        </button>
      </div>
      <div v-else class="tasks-list">
        <div
          v-for="task in tasks"
          :key="task.id"
          class="task-card"
          @click="selectTask(task)"
        >
          <div class="task-header">
            <span class="task-name">{{ task.name }}</span>
            <span class="task-status" :style="{ color: getStatusColor(task.status) }">
              {{ task.status }}
            </span>
          </div>
          <div class="task-description">{{ task.description || t('browserAutomation.tasks.noDescription') }}</div>
          <div class="task-meta">
            <span>{{ task.steps.length }} {{ t('browserAutomation.tasks.steps') }}</span>
            <span>{{ formatDate(task.created_at) }}</span>
          </div>
          <div class="task-actions" @click.stop>
            <button
              v-if="task.status === 'pending' || task.status === 'failed'"
              class="btn btn-sm btn-success"
              @click="runTask(task.id)"
            >
              {{ t('browserAutomation.tasks.run') }}
            </button>
            <button
              v-if="task.status === 'running'"
              class="btn btn-sm btn-warning"
              @click="cancelTask(task.id)"
            >
              {{ t('browserAutomation.tasks.cancel') }}
            </button>
            <button class="btn btn-sm btn-danger" @click="deleteTask(task.id)">
              {{ t('browserAutomation.tasks.delete') }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Sessions Tab -->
    <div v-if="activeTab === 'sessions'" class="sessions-section">
      <div class="sessions-header">
        <button class="btn btn-primary" @click="createSession">
          + {{ t('browserAutomation.sessions.newSession') }}
        </button>
      </div>
      <div v-if="sessions.length === 0" class="empty-state">
        <p>{{ t('browserAutomation.sessions.empty') }}</p>
      </div>
      <div v-else class="sessions-list">
        <div
          v-for="session in sessions"
          :key="session.id"
          class="session-card"
          @click="selectSession(session)"
        >
          <div class="session-header">
            <span class="session-id">{{ t('browserAutomation.sessions.session') }} {{ session.id.slice(0, 8) }}</span>
            <span class="session-status" :style="{ color: getStatusColor(session.status) }">
              {{ session.status }}
            </span>
          </div>
          <div class="session-url">{{ session.current_url || t('browserAutomation.sessions.noPageLoaded') }}</div>
          <div class="session-meta">
            <span>{{ formatDate(session.last_activity) }}</span>
          </div>
          <div class="session-actions" @click.stop>
            <button class="btn btn-sm btn-secondary" @click="takeScreenshot(session.id)">
              {{ t('browserAutomation.sessions.screenshot') }}
            </button>
            <button class="btn btn-sm btn-danger" @click="closeSession(session.id)">
              {{ t('browserAutomation.sessions.close') }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Templates Tab -->
    <div v-if="activeTab === 'templates'" class="templates-section">
      <div class="templates-grid">
        <div
          v-for="template in templates"
          :key="template.id"
          class="template-card"
          @click="useTemplate(template)"
        >
          <div class="template-icon">{{ getTemplateIcon(template.category) }}</div>
          <div class="template-name">{{ getTemplateName(template.id) }}</div>
          <div class="template-description">{{ getTemplateDescription(template.id) }}</div>
          <div class="template-category">{{ getTemplateCategory(template.category) }}</div>
        </div>
      </div>
    </div>

    <!-- Security Tab -->
    <div v-if="activeTab === 'security'" class="security-section">
      <div class="security-grid">
        <!-- Allowed Domains -->
        <div class="security-card">
          <div class="security-card-header">
            <h3>{{ t('browserAutomation.security.allowedDomains') }}</h3>
            <p class="security-hint">{{ t('browserAutomation.security.allowedDomainsHint') }}</p>
          </div>
          <div class="domain-input">
            <input
              v-model="newAllowedDomain"
              type="text"
              :placeholder="t('browserAutomation.security.allowedDomainsPlaceholder')"
              @keyup.enter="addAllowedDomain"
            />
            <button class="btn btn-primary btn-sm" :disabled="!newAllowedDomain" @click="addAllowedDomain">
              {{ t('browserAutomation.security.add') }}
            </button>
          </div>
          <div v-if="securityConfig.allowed_domains.length === 0" class="empty-domains">
            <p>{{ t('browserAutomation.security.noAllowedDomains') }}</p>
          </div>
          <div v-else class="domain-list">
            <div v-for="domain in securityConfig.allowed_domains" :key="domain" class="domain-item allowed">
              <span class="domain-name">{{ domain }}</span>
              <button class="remove-btn" @click="removeAllowedDomain(domain)">&times;</button>
            </div>
          </div>
        </div>

        <!-- Blocked Domains -->
        <div class="security-card">
          <div class="security-card-header">
            <h3>{{ t('browserAutomation.security.blockedDomains') }}</h3>
            <p class="security-hint">{{ t('browserAutomation.security.blockedDomainsHint') }}</p>
          </div>
          <div class="domain-input">
            <input
              v-model="newBlockedDomain"
              type="text"
              :placeholder="t('browserAutomation.security.blockedDomainsPlaceholder')"
              @keyup.enter="addBlockedDomain"
            />
            <button class="btn btn-danger btn-sm" :disabled="!newBlockedDomain" @click="addBlockedDomain">
              {{ t('browserAutomation.security.block') }}
            </button>
          </div>
          <div v-if="securityConfig.blocked_domains.length === 0" class="empty-domains">
            <p>{{ t('browserAutomation.security.noBlockedDomains') }}</p>
          </div>
          <div v-else class="domain-list">
            <div v-for="domain in securityConfig.blocked_domains" :key="domain" class="domain-item blocked">
              <span class="domain-name">{{ domain }}</span>
              <button class="remove-btn" @click="removeBlockedDomain(domain)">&times;</button>
            </div>
          </div>
        </div>
      </div>

      <!-- URL Tester -->
      <div class="url-tester">
        <h3>{{ t('browserAutomation.security.testUrl') }}</h3>
        <p class="security-hint">{{ t('browserAutomation.security.testUrlHint') }}</p>
        <div class="tester-input">
          <input
            v-model="testUrlInput"
            type="url"
            :placeholder="t('browserAutomation.security.testUrlPlaceholder')"
            @keyup.enter="testUrl"
          />
          <button class="btn btn-secondary" :disabled="!testUrlInput || testingUrl" @click="testUrl">
            {{ testingUrl ? t('browserAutomation.security.testing') : t('browserAutomation.security.test') }}
          </button>
        </div>
        <div v-if="testResult" class="test-result" :class="{ allowed: testResult.allowed, blocked: !testResult.allowed }">
          <span class="result-icon">{{ testResult.allowed ? '✓' : '✕' }}</span>
          <span class="result-text">{{ testResult.allowed ? t('browserAutomation.security.urlAllowed') : t('browserAutomation.security.urlBlocked') }}</span>
          <span v-if="testResult.reason" class="result-reason">{{ testResult.reason }}</span>
        </div>
      </div>
    </div>

    <!-- Task Detail Modal -->
    <div v-if="selectedTask" class="modal-overlay" @click="selectedTask = null">
      <div class="modal" @click.stop>
        <div class="modal-header">
          <h2>{{ selectedTask.name }}</h2>
          <button class="close-btn" @click="selectedTask = null">&times;</button>
        </div>
        <div class="modal-body">
          <div class="task-detail-status">
            <span>{{ t('browserAutomation.taskDetail.status') }}</span>
            <span :style="{ color: getStatusColor(selectedTask.status) }">
              {{ selectedTask.status }}
            </span>
          </div>
          <div v-if="selectedTask.error" class="task-error">
            {{ t('browserAutomation.taskDetail.error') }} {{ selectedTask.error }}
          </div>
          <h3>{{ t('browserAutomation.taskDetail.steps') }}</h3>
          <div class="steps-list">
            <div
              v-for="(step, index) in selectedTask.steps"
              :key="step.id"
              class="step-item"
              :class="step.status"
            >
              <span class="step-number">{{ index + 1 }}</span>
              <span class="step-icon">{{ getStepIcon(step.type) }}</span>
              <span class="step-type">{{ getStepLabel(step.type) }}</span>
              <span class="step-status">{{ step.status }}</span>
            </div>
          </div>
          <div v-if="selectedTask.result" class="task-result">
            <h3>{{ t('browserAutomation.taskDetail.result') }}</h3>
            <p>{{ t('browserAutomation.taskDetail.duration') }} {{ formatDuration(selectedTask.result.duration_ms) }}</p>
            <p>{{ t('browserAutomation.taskDetail.finalUrl') }} {{ selectedTask.result.final_url }}</p>
            <div v-if="selectedTask.result.screenshots.length" class="screenshots">
              <h4>{{ t('browserAutomation.taskDetail.screenshots') }}</h4>
              <div class="screenshots-grid">
                <img
                  v-for="(screenshot, i) in selectedTask.result.screenshots"
                  :key="i"
                  :src="'data:image/png;base64,' + screenshot"
                  :alt="t('browserAutomation.screenshot.title')"
                  class="screenshot-thumb"
                />
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Create Task Modal -->
    <div v-if="showCreateModal" class="modal-overlay" @click="showCreateModal = false">
      <div class="modal modal-large" @click.stop>
        <div class="modal-header">
          <h2>{{ t('browserAutomation.createTask.title') }}</h2>
          <button class="close-btn" @click="showCreateModal = false">&times;</button>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label>{{ t('browserAutomation.createTask.taskName') }}</label>
            <input v-model="newTask.name" type="text" :placeholder="t('browserAutomation.createTask.taskNamePlaceholder')" />
          </div>
          <div class="form-group">
            <label>{{ t('browserAutomation.createTask.description') }}</label>
            <textarea
              v-model="newTask.description"
              :placeholder="t('browserAutomation.createTask.descriptionPlaceholder')"
            ></textarea>
          </div>
          <div class="steps-builder">
            <h3>{{ t('browserAutomation.createTask.steps') }}</h3>
            <div
              v-for="(step, index) in newTask.steps"
              :key="index"
              class="step-builder-item"
            >
              <div class="step-builder-header">
                <span>{{ t('browserAutomation.createTask.step', { index: index + 1 }) }}</span>
                <button class="btn btn-sm btn-danger" @click="removeStep(index)">
                  {{ t('browserAutomation.createTask.remove') }}
                </button>
              </div>
              <div class="step-builder-content">
                <select v-model="step.type">
                  <option value="navigate">{{ t('browserAutomation.createTask.stepTypes.navigate') }}</option>
                  <option value="click">{{ t('browserAutomation.createTask.stepTypes.click') }}</option>
                  <option value="type">{{ t('browserAutomation.createTask.stepTypes.type') }}</option>
                  <option value="screenshot">{{ t('browserAutomation.createTask.stepTypes.screenshot') }}</option>
                  <option value="wait">{{ t('browserAutomation.createTask.stepTypes.wait') }}</option>
                  <option value="extract">{{ t('browserAutomation.createTask.stepTypes.extract') }}</option>
                  <option value="scroll">{{ t('browserAutomation.createTask.stepTypes.scroll') }}</option>
                </select>
                <div v-if="step.type === 'navigate'" class="step-params">
                  <input
                    v-model="step.params.url"
                    type="url"
                    :placeholder="t('browserAutomation.createTask.params.urlPlaceholder')"
                  />
                </div>
                <div v-if="step.type === 'click' || step.type === 'type' || step.type === 'extract'" class="step-params">
                  <input
                    v-model="step.params.selector"
                    type="text"
                    :placeholder="t('browserAutomation.createTask.params.selectorPlaceholder')"
                  />
                </div>
                <div v-if="step.type === 'type'" class="step-params">
                  <input
                    v-model="step.params.text"
                    type="text"
                    :placeholder="t('browserAutomation.createTask.params.textPlaceholder')"
                  />
                </div>
                <div v-if="step.type === 'wait'" class="step-params">
                  <input
                    v-model.number="step.params.timeout"
                    type="number"
                    :placeholder="t('browserAutomation.createTask.params.timeoutPlaceholder')"
                  />
                </div>
              </div>
            </div>
            <button class="btn btn-secondary" @click="addStep">{{ t('browserAutomation.createTask.addStep') }}</button>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="showCreateModal = false">
            {{ t('browserAutomation.createTask.cancel') }}
          </button>
          <button class="btn btn-primary" :disabled="!canCreateTask" @click="createTask">
            {{ t('browserAutomation.createTask.create') }}
          </button>
        </div>
      </div>
    </div>

    <!-- Screenshot Modal -->
    <div v-if="screenshotPreview" class="modal-overlay" @click="screenshotPreview = null">
      <div class="modal modal-large" @click.stop>
        <div class="modal-header">
          <h2>{{ t('browserAutomation.screenshot.title') }}</h2>
          <button class="close-btn" @click="screenshotPreview = null">&times;</button>
        </div>
        <div class="modal-body screenshot-preview">
          <img :src="'data:image/png;base64,' + screenshotPreview" :alt="t('browserAutomation.screenshot.title')" />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import * as browserApi from '@/api/browser'
import type { BrowserTask, BrowserSession, TaskTemplate, StepType, BrowserSecurityConfig } from '@/api/browser'

const { t } = useI18n()

// State
const tasks = ref<BrowserTask[]>([])
const sessions = ref<BrowserSession[]>([])
const templates = ref<TaskTemplate[]>(browserApi.taskTemplates)
const activeTab = ref<'tasks' | 'sessions' | 'templates' | 'security'>('tasks')
const selectedTask = ref<BrowserTask | null>(null)
const showCreateModal = ref(false)
const screenshotPreview = ref<string | null>(null)

// Security state
const securityConfig = ref<BrowserSecurityConfig>({
  allowed_domains: [],
  blocked_domains: [],
})
const newAllowedDomain = ref('')
const newBlockedDomain = ref('')
const testUrlInput = ref('')
const testingUrl = ref(false)
const testResult = ref<{ allowed: boolean; reason?: string } | null>(null)

const newTask = ref({
  name: '',
  description: '',
  steps: [] as { type: StepType; params: Record<string, unknown> }[],
})

// Computed
const runningTasks = computed(() => tasks.value.filter((t) => t.status === 'running').length)
const completedTasks = computed(() => tasks.value.filter((t) => t.status === 'completed').length)
const canCreateTask = computed(() => newTask.value.name && newTask.value.steps.length > 0)

// Methods
async function loadTasks() {
  try {
    const result = await browserApi.getTasks()
    tasks.value = Array.isArray(result) ? result : []
  } catch (error) {
    console.error('Failed to load tasks:', error)
    tasks.value = []
  }
}

async function loadSessions() {
  try {
    const result = await browserApi.getSessions()
    sessions.value = Array.isArray(result) ? result : []
  } catch (error) {
    console.error('Failed to load sessions:', error)
    sessions.value = []
  }
}

async function createTask() {
  try {
    await browserApi.createTask({
      name: newTask.value.name,
      description: newTask.value.description,
      steps: newTask.value.steps,
    })
    showCreateModal.value = false
    newTask.value = { name: '', description: '', steps: [] }
    await loadTasks()
  } catch (error) {
    console.error('Failed to create task:', error)
  }
}

async function runTask(taskId: string) {
  try {
    await browserApi.runTask(taskId)
    await loadTasks()
  } catch (error) {
    console.error('Failed to run task:', error)
  }
}

async function cancelTask(taskId: string) {
  try {
    await browserApi.cancelTask(taskId)
    await loadTasks()
  } catch (error) {
    console.error('Failed to cancel task:', error)
  }
}

async function deleteTask(taskId: string) {
  if (!confirm(t('browserAutomation.tasks.deleteConfirm'))) return
  try {
    await browserApi.deleteTask(taskId)
    await loadTasks()
  } catch (error) {
    console.error('Failed to delete task:', error)
  }
}

async function createSession() {
  try {
    await browserApi.createSession()
    await loadSessions()
  } catch (error) {
    console.error('Failed to create session:', error)
  }
}

async function closeSession(sessionId: string) {
  try {
    await browserApi.closeSession(sessionId)
    await loadSessions()
  } catch (error) {
    console.error('Failed to close session:', error)
  }
}

async function takeScreenshot(sessionId: string) {
  try {
    const screenshot = await browserApi.takeScreenshot(sessionId)
    screenshotPreview.value = screenshot
  } catch (error) {
    console.error('Failed to take screenshot:', error)
  }
}

function selectTask(task: BrowserTask) {
  selectedTask.value = task
}

function selectSession(session: BrowserSession) {
  // Could open a live view of the session
}

function useTemplate(template: TaskTemplate) {
  newTask.value = {
    name: template.name,
    description: template.description,
    steps: JSON.parse(JSON.stringify(template.steps)),
  }
  showCreateModal.value = true
}

function addStep() {
  newTask.value.steps.push({
    type: 'navigate',
    params: {},
  })
}

function removeStep(index: number) {
  newTask.value.steps.splice(index, 1)
}

function getStatusColor(status: string): string {
  return browserApi.getStatusColor(status)
}

function getStepIcon(type: StepType): string {
  return browserApi.getStepIcon(type)
}

function formatDuration(ms: number): string {
  return browserApi.formatDuration(ms)
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleString([], {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

function getTemplateIcon(category: string): string {
  const icons: Record<string, string> = {
    Data: '📊',
    Automation: '🤖',
    Monitoring: '👁️',
    Testing: '🧪',
  }
  return icons[category] || '📦'
}

// Template i18n helpers
function getTemplateName(templateId: string): string {
  const keyMap: Record<string, string> = {
    'web-scrape': 'browserAutomation.templates.webScraping.name',
    'form-fill': 'browserAutomation.templates.formFilling.name',
    'page-monitor': 'browserAutomation.templates.pageMonitor.name',
    'login-test': 'browserAutomation.templates.loginTest.name',
  }
  const key = keyMap[templateId]
  return key ? t(key) : templateId
}

function getTemplateDescription(templateId: string): string {
  const keyMap: Record<string, string> = {
    'web-scrape': 'browserAutomation.templates.webScraping.description',
    'form-fill': 'browserAutomation.templates.formFilling.description',
    'page-monitor': 'browserAutomation.templates.pageMonitor.description',
    'login-test': 'browserAutomation.templates.loginTest.description',
  }
  const key = keyMap[templateId]
  return key ? t(key) : ''
}

function getTemplateCategory(category: string): string {
  const keyMap: Record<string, string> = {
    'Data': 'browserAutomation.categories.data',
    'Automation': 'browserAutomation.categories.automation',
    'Monitoring': 'browserAutomation.categories.monitoring',
    'Testing': 'browserAutomation.categories.testing',
  }
  const key = keyMap[category]
  return key ? t(key) : category
}

function getStepLabel(type: StepType): string {
  const keyMap: Record<StepType, string> = {
    navigate: 'browserAutomation.stepLabels.navigate',
    click: 'browserAutomation.stepLabels.click',
    type: 'browserAutomation.stepLabels.type',
    screenshot: 'browserAutomation.stepLabels.screenshot',
    wait: 'browserAutomation.stepLabels.wait',
    extract: 'browserAutomation.stepLabels.extract',
    scroll: 'browserAutomation.stepLabels.scroll',
    select: 'browserAutomation.stepLabels.select',
    hover: 'browserAutomation.stepLabels.hover',
    press_key: 'browserAutomation.stepLabels.pressKey',
    evaluate: 'browserAutomation.stepLabels.evaluate',
  }
  const key = keyMap[type]
  return key ? t(key) : type
}

// Security methods
async function loadSecurityConfig() {
  try {
    securityConfig.value = await browserApi.getSecurityConfig()
  } catch (error) {
    console.error('Failed to load security config:', error)
    // Use default empty config
    securityConfig.value = { allowed_domains: [], blocked_domains: [] }
  }
}

async function addAllowedDomain() {
  if (!newAllowedDomain.value) return
  const domain = newAllowedDomain.value.trim().toLowerCase()
  if (securityConfig.value.allowed_domains.includes(domain)) {
    newAllowedDomain.value = ''
    return
  }
  try {
    await browserApi.addAllowedDomain(domain)
    securityConfig.value.allowed_domains.push(domain)
    newAllowedDomain.value = ''
  } catch (error) {
    console.error('Failed to add allowed domain:', error)
  }
}

async function removeAllowedDomain(domain: string) {
  try {
    await browserApi.removeAllowedDomain(domain)
    securityConfig.value.allowed_domains = securityConfig.value.allowed_domains.filter(d => d !== domain)
  } catch (error) {
    console.error('Failed to remove allowed domain:', error)
  }
}

async function addBlockedDomain() {
  if (!newBlockedDomain.value) return
  const domain = newBlockedDomain.value.trim().toLowerCase()
  if (securityConfig.value.blocked_domains.includes(domain)) {
    newBlockedDomain.value = ''
    return
  }
  try {
    await browserApi.addBlockedDomain(domain)
    securityConfig.value.blocked_domains.push(domain)
    newBlockedDomain.value = ''
  } catch (error) {
    console.error('Failed to add blocked domain:', error)
  }
}

async function removeBlockedDomain(domain: string) {
  try {
    await browserApi.removeBlockedDomain(domain)
    securityConfig.value.blocked_domains = securityConfig.value.blocked_domains.filter(d => d !== domain)
  } catch (error) {
    console.error('Failed to remove blocked domain:', error)
  }
}

async function testUrl() {
  if (!testUrlInput.value) return
  testingUrl.value = true
  testResult.value = null
  try {
    testResult.value = await browserApi.testUrl(testUrlInput.value)
  } catch {
    testResult.value = { allowed: false, reason: 'Failed to test URL' }
  } finally {
    testingUrl.value = false
  }
}

// Lifecycle
onMounted(() => {
  loadTasks()
  loadSessions()
  loadSecurityConfig()
})
</script>

<style scoped>
.browser-automation-view {
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

.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 1rem;
  margin-bottom: 1.5rem;
}

.stat-card {
  background: var(--color-background-soft);
  border-radius: 0.75rem;
  padding: 1.25rem;
  text-align: center;
}

.stat-value {
  font-size: 2rem;
  font-weight: 700;
  color: var(--color-primary);
}

.stat-label {
  color: var(--color-text-muted);
  font-size: 0.875rem;
}

.tabs {
  display: flex;
  gap: 0.5rem;
  margin-bottom: 1.5rem;
  border-bottom: 1px solid var(--color-border);
  padding-bottom: 0.5rem;
}

.tab-btn {
  padding: 0.75rem 1.5rem;
  border: none;
  background: none;
  cursor: pointer;
  font-size: 1rem;
  color: var(--color-text-muted);
  border-bottom: 2px solid transparent;
  transition: all 0.2s;
}

.tab-btn.active {
  color: var(--color-primary);
  border-bottom-color: var(--color-primary);
}

.empty-state {
  text-align: center;
  padding: 3rem;
  color: var(--color-text-muted);
}

.tasks-list,
.sessions-list {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.task-card,
.session-card {
  background: var(--color-background-soft);
  border-radius: 0.75rem;
  padding: 1.25rem;
  cursor: pointer;
  transition: all 0.2s;
}

.task-card:hover,
.session-card:hover {
  background: var(--color-background-mute);
}

.task-header,
.session-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 0.5rem;
}

.task-name,
.session-id {
  font-weight: 600;
  font-size: 1.1rem;
}

.task-description,
.session-url {
  color: var(--color-text-muted);
  font-size: 0.875rem;
  margin-bottom: 0.75rem;
}

.task-meta,
.session-meta {
  display: flex;
  gap: 1rem;
  font-size: 0.75rem;
  color: var(--color-text-muted);
  margin-bottom: 0.75rem;
}

.task-actions,
.session-actions {
  display: flex;
  gap: 0.5rem;
}

.sessions-header {
  margin-bottom: 1rem;
}

.templates-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
  gap: 1rem;
}

.template-card {
  background: var(--color-background-soft);
  border-radius: 0.75rem;
  padding: 1.5rem;
  cursor: pointer;
  transition: all 0.2s;
  text-align: center;
}

.template-card:hover {
  background: var(--color-background-mute);
  transform: translateY(-2px);
}

.template-icon {
  font-size: 2.5rem;
  margin-bottom: 0.75rem;
}

.template-name {
  font-weight: 600;
  font-size: 1.1rem;
  margin-bottom: 0.5rem;
}

.template-description {
  color: var(--color-text-muted);
  font-size: 0.875rem;
  margin-bottom: 0.75rem;
}

.template-category {
  display: inline-block;
  padding: 0.25rem 0.75rem;
  background: var(--color-primary);
  color: white;
  border-radius: 1rem;
  font-size: 0.75rem;
}

/* Modal styles */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal {
  background: var(--color-background);
  border-radius: 0.75rem;
  width: 90%;
  max-width: 500px;
  max-height: 90vh;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.modal-large {
  max-width: 800px;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem 1.5rem;
  border-bottom: 1px solid var(--color-border);
}

.modal-header h2 {
  margin: 0;
  font-size: 1.25rem;
}

.close-btn {
  background: none;
  border: none;
  font-size: 1.5rem;
  cursor: pointer;
  color: var(--color-text-muted);
}

.modal-body {
  padding: 1.5rem;
  overflow-y: auto;
  flex: 1;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 0.75rem;
  padding: 1rem 1.5rem;
  border-top: 1px solid var(--color-border);
}

.task-detail-status {
  display: flex;
  gap: 0.5rem;
  margin-bottom: 1rem;
}

.task-error {
  background: #fef2f2;
  color: #991b1b;
  padding: 0.75rem;
  border-radius: 0.5rem;
  margin-bottom: 1rem;
}

.steps-list {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.step-item {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.75rem;
  background: var(--color-background-soft);
  border-radius: 0.5rem;
}

.step-item.completed {
  background: #f0fdf4;
}

.step-item.failed {
  background: #fef2f2;
}

.step-item.running {
  background: #eff6ff;
}

.step-number {
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--color-primary);
  color: white;
  border-radius: 50%;
  font-size: 0.75rem;
}

.step-icon {
  font-size: 1.25rem;
}

.step-type {
  flex: 1;
}

.step-status {
  font-size: 0.75rem;
  color: var(--color-text-muted);
}

.screenshots-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
  gap: 0.75rem;
  margin-top: 0.75rem;
}

.screenshot-thumb {
  width: 100%;
  border-radius: 0.5rem;
  cursor: pointer;
}

.screenshot-preview img {
  width: 100%;
  border-radius: 0.5rem;
}

/* Form styles */
.form-group {
  margin-bottom: 1rem;
}

.form-group label {
  display: block;
  margin-bottom: 0.5rem;
  font-weight: 500;
}

.form-group input,
.form-group textarea,
.form-group select {
  width: 100%;
  padding: 0.75rem;
  border: 1px solid var(--color-border);
  border-radius: 0.5rem;
  background: var(--color-background);
  color: var(--color-text);
}

.form-group textarea {
  min-height: 80px;
  resize: vertical;
}

.steps-builder h3 {
  margin-bottom: 1rem;
}

.step-builder-item {
  background: var(--color-background-soft);
  border-radius: 0.5rem;
  padding: 1rem;
  margin-bottom: 0.75rem;
}

.step-builder-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 0.75rem;
}

.step-builder-content {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.step-params {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

/* Button styles */
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
  background: var(--color-background-soft);
  color: var(--color-text);
  border: 1px solid var(--color-border);
}

.btn-success {
  background: #22c55e;
  color: white;
}

.btn-warning {
  background: #f59e0b;
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

/* Security section styles */
.security-section {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.security-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 1.5rem;
}

.security-card {
  background: var(--color-background-soft);
  border-radius: 0.75rem;
  padding: 1.25rem;
}

.security-card-header {
  margin-bottom: 1rem;
}

.security-card-header h3 {
  margin: 0 0 0.25rem 0;
  font-size: 1.125rem;
}

.security-hint {
  margin: 0;
  font-size: 0.75rem;
  color: var(--color-text-muted);
}

.domain-input {
  display: flex;
  gap: 0.5rem;
  margin-bottom: 1rem;
}

.domain-input input {
  flex: 1;
  padding: 0.5rem 0.75rem;
  border: 1px solid var(--color-border);
  border-radius: 0.375rem;
  font-size: 0.875rem;
  background: var(--color-background);
  color: var(--color-text);
}

.empty-domains {
  padding: 1rem;
  text-align: center;
  color: var(--color-text-muted);
  font-size: 0.875rem;
  background: var(--color-background);
  border-radius: 0.5rem;
}

.domain-list {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  max-height: 200px;
  overflow-y: auto;
}

.domain-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.5rem 0.75rem;
  border-radius: 0.375rem;
  font-size: 0.875rem;
}

.domain-item.allowed {
  background: #f0fdf4;
  color: #166534;
}

.domain-item.blocked {
  background: #fef2f2;
  color: #991b1b;
}

.domain-name {
  font-family: monospace;
}

.remove-btn {
  background: none;
  border: none;
  font-size: 1.25rem;
  cursor: pointer;
  opacity: 0.5;
  transition: opacity 0.2s;
  padding: 0;
  line-height: 1;
}

.remove-btn:hover {
  opacity: 1;
}

.url-tester {
  background: var(--color-background-soft);
  border-radius: 0.75rem;
  padding: 1.25rem;
}

.url-tester h3 {
  margin: 0 0 0.25rem 0;
  font-size: 1.125rem;
}

.tester-input {
  display: flex;
  gap: 0.5rem;
  margin-top: 1rem;
}

.tester-input input {
  flex: 1;
  padding: 0.625rem 0.875rem;
  border: 1px solid var(--color-border);
  border-radius: 0.5rem;
  font-size: 0.875rem;
  background: var(--color-background);
  color: var(--color-text);
}

.test-result {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-top: 1rem;
  padding: 0.75rem 1rem;
  border-radius: 0.5rem;
}

.test-result.allowed {
  background: #f0fdf4;
  color: #166534;
}

.test-result.blocked {
  background: #fef2f2;
  color: #991b1b;
}

.result-icon {
  font-size: 1.25rem;
  font-weight: 700;
}

.result-text {
  font-weight: 500;
}

.result-reason {
  font-size: 0.75rem;
  opacity: 0.8;
  margin-left: auto;
}

@media (max-width: 768px) {
  .stats-grid {
    grid-template-columns: repeat(2, 1fr);
  }

  .security-grid {
    grid-template-columns: 1fr;
  }
}
</style>
