<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  securityApi,
  type CORSConfig,
  type TLSConfig,
  type CertificateInfo,
  type ACMEStatus,
} from '@/api/security'
import { systemApi } from '@/api/system'

const { t } = useI18n()

interface NetworkSettingsProps {
  showPortSection?: boolean
  showSecuritySections?: boolean
}

const props = withDefaults(defineProps<NetworkSettingsProps>(), {
  showPortSection: true,
  showSecuritySections: true,
})

const emit = defineEmits<{
  'status-change': [message: string]
}>()

const loading = ref(false)
const corsConfig = ref<CORSConfig | null>(null)
const tlsConfig = ref<TLSConfig | null>(null)
const acmeStatus = ref<ACMEStatus | null>(null)
const newOrigin = ref('')
const addingOrigin = ref(false)

// TLS state
const showUploadDialog = ref(false)
const showSelfSignedDialog = ref(false)
const showACMEDialog = ref(false)
const certPem = ref('')
const keyPem = ref('')
const selfSignedDomains = ref('localhost')
const selfSignedDays = ref(365)
const uploading = ref(false)
const generating = ref(false)
const parsedCertInfo = ref<CertificateInfo | null>(null)

// ACME state
const acmeEmail = ref('')
const acmeDomains = ref('')
const acmeProvider = ref('letsencrypt')
const acmeChallengeType = ref('http-01')
const acmeDNSProvider = ref('cloudflare')
const acmeDNSCredentials = ref<Array<{ key: string; value: string }>>([{ key: '', value: '' }])
const requestingACME = ref(false)

// HTTPS-only state
const updatingTLSSettings = ref(false)

// Port configuration state
interface ServerConfig {
  host: string
  port: number
  actual_port: number
  port_auto_fallback: boolean
}
const serverConfig = ref<ServerConfig | null>(null)
const portInput = ref('')
const portEditing = ref(false)
const portChangeConfirm = ref(false)
const portChangeCountdown = ref(0)
const previousPort = ref(0)
let countdownTimer: ReturnType<typeof setInterval> | null = null

const portChanged = computed(() => {
  if (!serverConfig.value) return false
  return serverConfig.value.port !== serverConfig.value.actual_port
})

async function fetchCORSConfig() {
  loading.value = true
  try {
    const response = await securityApi.getCORSConfig()
    corsConfig.value = response.data
  } catch (e) {
    console.error('Failed to fetch CORS config:', e)
  } finally {
    loading.value = false
  }
}

async function fetchTLSConfig() {
  try {
    const response = await securityApi.getTLSConfig()
    tlsConfig.value = response.data
    // Also fetch ACME status
    const acmeResponse = await securityApi.getACMEStatus()
    acmeStatus.value = acmeResponse.data
  } catch (e) {
    console.error('Failed to fetch TLS config:', e)
  }
}

async function addOrigin() {
  if (!newOrigin.value.trim()) return
  try {
    new URL(newOrigin.value)
  } catch {
    emit('status-change', t('settings.network.invalidUrl'))
    return
  }
  addingOrigin.value = true
  try {
    const response = await securityApi.updateCORSConfig({ add_origins: [newOrigin.value.trim()] })
    corsConfig.value = response.data
    newOrigin.value = ''
    emit('status-change', t('settings.network.originAdded'))
  } catch (e) {
    console.error('Failed to add origin:', e)
  } finally {
    addingOrigin.value = false
  }
}

async function removeOrigin(origin: string) {
  if (!window.confirm(t('settings.network.confirmRemoveDynamicOrigin', { origin }))) {
    return
  }
  if (!window.confirm(t('settings.network.confirmRemoveDynamicOriginSecond', { origin }))) {
    return
  }
  try {
    const response = await securityApi.updateCORSConfig({ remove_origins: [origin] })
    corsConfig.value = response.data
    emit('status-change', t('settings.network.originRemoved'))
  } catch (e) {
    console.error('Failed to remove origin:', e)
  }
}

// TLS functions
async function parseCert() {
  if (!certPem.value.trim()) return
  try {
    const response = await securityApi.parseCertificate(certPem.value)
    parsedCertInfo.value = response.data
  } catch (_e: unknown) {
    parsedCertInfo.value = null
  }
}

async function uploadCertificate() {
  if (!certPem.value.trim() || !keyPem.value.trim()) {
    emit('status-change', t('settings.network.tls.certKeyRequired'))
    return
  }
  uploading.value = true
  try {
    const response = await securityApi.uploadTLSCert(certPem.value, keyPem.value)
    tlsConfig.value = response.data
    showUploadDialog.value = false
    certPem.value = ''
    keyPem.value = ''
    parsedCertInfo.value = null
    emit('status-change', t('settings.network.tls.certUploaded'))
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : t('settings.network.tls.uploadFailed')
    emit('status-change', msg)
  } finally {
    uploading.value = false
  }
}

async function generateSelfSigned() {
  const domains = selfSignedDomains.value
    .split(',')
    .map((d) => d.trim())
    .filter((d) => d)
  if (domains.length === 0) domains.push('localhost')
  generating.value = true
  try {
    const response = await securityApi.generateSelfSignedCert(domains, selfSignedDays.value)
    tlsConfig.value = response.data
    showSelfSignedDialog.value = false
    emit('status-change', t('settings.network.tls.selfSignedGenerated'))
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : t('settings.network.tls.generateFailed')
    emit('status-change', msg)
  } finally {
    generating.value = false
  }
}

const certExpiry = computed(() => {
  if (!tlsConfig.value?.cert_info?.not_after) return null
  const date = new Date(tlsConfig.value.cert_info.not_after)
  const now = new Date()
  const days = Math.ceil((date.getTime() - now.getTime()) / (1000 * 60 * 60 * 24))
  return { date, days, expired: days < 0 }
})

async function requestACMECert() {
  const domains = acmeDomains.value
    .split(',')
    .map((d) => d.trim())
    .filter((d) => d)
  if (!acmeEmail.value.trim() || domains.length === 0) {
    emit('status-change', t('settings.network.tls.acme.emailDomainRequired'))
    return
  }
  requestingACME.value = true
  try {
    const dnsCredentials =
      acmeChallengeType.value === 'dns-01'
        ? Object.fromEntries(
            acmeDNSCredentials.value.filter((c) => c.key && c.value).map((c) => [c.key, c.value])
          )
        : undefined
    const response = await securityApi.requestACMECert(
      acmeEmail.value,
      domains,
      acmeProvider.value,
      acmeChallengeType.value,
      acmeChallengeType.value === 'dns-01' ? acmeDNSProvider.value : undefined,
      dnsCredentials
    )
    acmeStatus.value = response.data
    showACMEDialog.value = false
    emit('status-change', t('settings.network.tls.acme.configured'))
    // Refresh TLS config
    fetchTLSConfig()
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : t('settings.network.tls.acme.requestFailed')
    emit('status-change', msg)
  } finally {
    requestingACME.value = false
  }
}

function addDNSCredential() {
  acmeDNSCredentials.value.push({ key: '', value: '' })
}

function removeDNSCredential(index: number) {
  acmeDNSCredentials.value.splice(index, 1)
}

// DNS provider credential templates
const dnsProviderCredentials: Record<string, string[]> = {
  cloudflare: ['CF_DNS_API_TOKEN'],
  route53: ['AWS_ACCESS_KEY_ID', 'AWS_SECRET_ACCESS_KEY', 'AWS_REGION'],
  godaddy: ['GODADDY_API_KEY', 'GODADDY_API_SECRET'],
  namecheap: ['NAMECHEAP_API_USER', 'NAMECHEAP_API_KEY'],
  alidns: ['ALICLOUD_ACCESS_KEY', 'ALICLOUD_SECRET_KEY'],
  tencentcloud: ['TENCENTCLOUD_SECRET_ID', 'TENCENTCLOUD_SECRET_KEY'],
}

function onDNSProviderChange() {
  const template = dnsProviderCredentials[acmeDNSProvider.value]
  if (template) {
    acmeDNSCredentials.value = template.map((key) => ({ key, value: '' }))
  }
}

async function updateHTTPSOnly(enabled: boolean) {
  updatingTLSSettings.value = true
  try {
    const port = tlsConfig.value?.https_port || 443
    const response = await securityApi.updateTLSSettings(enabled, port)
    tlsConfig.value = response.data
    emit(
      'status-change',
      enabled
        ? t('settings.network.tls.httpsOnlyEnabled')
        : t('settings.network.tls.httpsOnlyDisabled')
    )
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : t('settings.network.tls.updateFailed')
    emit('status-change', msg)
  } finally {
    updatingTLSSettings.value = false
  }
}

async function reloadCertificate() {
  try {
    const response = await securityApi.reloadTLSCert()
    tlsConfig.value = response.data
    emit('status-change', t('settings.network.tls.certReloaded'))
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : t('settings.network.tls.reloadFailed')
    emit('status-change', msg)
  }
}

// Port configuration functions
async function fetchServerConfig() {
  try {
    const response = await systemApi.getConfig()
    const config = response.data as { server?: ServerConfig }
    if (config.server) {
      serverConfig.value = config.server
      portInput.value = String(config.server.port)
    }
  } catch (e) {
    console.error('Failed to fetch server config:', e)
  }
}

function startEditPort() {
  if (serverConfig.value) {
    portInput.value = String(serverConfig.value.port)
    portEditing.value = true
  }
}

function cancelEditPort() {
  if (serverConfig.value) {
    portInput.value = String(serverConfig.value.port)
  }
  portEditing.value = false
}

function validatePort(value: string): boolean {
  const port = parseInt(value, 10)
  return !isNaN(port) && port >= 1 && port <= 65535
}

async function savePort() {
  if (!validatePort(portInput.value)) {
    emit('status-change', t('service.invalidPort'))
    return
  }
  const newPort = parseInt(portInput.value, 10)
  const currentPort = serverConfig.value?.actual_port || serverConfig.value?.port || 80
  if (newPort === currentPort) {
    portEditing.value = false
    return
  }
  try {
    const response = await systemApi.updateConfig({ server: { port: newPort } })
    if (response.data.success) {
      portEditing.value = false
      localStorage.setItem(
        'portChangeInfo',
        JSON.stringify({
          previousPort: currentPort,
          newPort: newPort,
          timestamp: Date.now(),
        })
      )
      await systemApi.restartService()
      setTimeout(() => {
        const currentUrl = new URL(window.location.href)
        currentUrl.port = String(newPort)
        window.location.href = currentUrl.toString()
      }, 2000)
    } else {
      emit(
        'status-change',
        t('service.portSaveFailed') + (response.data.message ? `: ${response.data.message}` : '')
      )
    }
  } catch (e) {
    emit(
      'status-change',
      t('service.portSaveFailed') + (e instanceof Error ? `: ${e.message}` : '')
    )
  }
}

function checkPortChangeConfirmation() {
  const portChangeInfoStr = localStorage.getItem('portChangeInfo')
  if (!portChangeInfoStr) return
  try {
    const portChangeInfo = JSON.parse(portChangeInfoStr)
    const elapsed = Date.now() - portChangeInfo.timestamp
    if (elapsed < 30000) {
      previousPort.value = portChangeInfo.previousPort
      portChangeConfirm.value = true
      portChangeCountdown.value = Math.max(1, Math.floor((30000 - elapsed) / 1000))
      countdownTimer = setInterval(() => {
        portChangeCountdown.value--
        if (portChangeCountdown.value <= 0) revertPort()
      }, 1000)
    } else {
      localStorage.removeItem('portChangeInfo')
    }
  } catch {
    localStorage.removeItem('portChangeInfo')
  }
}

function confirmPortChange() {
  if (countdownTimer) {
    clearInterval(countdownTimer)
    countdownTimer = null
  }
  portChangeConfirm.value = false
  localStorage.removeItem('portChangeInfo')
  emit('status-change', t('service.portChangeConfirmed'))
}

async function revertPort() {
  if (countdownTimer) {
    clearInterval(countdownTimer)
    countdownTimer = null
  }
  portChangeConfirm.value = false
  const portChangeInfoStr = localStorage.getItem('portChangeInfo')
  let prevPort = previousPort.value
  if (portChangeInfoStr) {
    try {
      prevPort = JSON.parse(portChangeInfoStr).previousPort
    } catch {
      /* use previousPort */
    }
  }
  localStorage.removeItem('portChangeInfo')
  try {
    await systemApi.updateConfig({ server: { port: prevPort } })
    await systemApi.restartService()
    setTimeout(() => {
      const currentUrl = new URL(window.location.href)
      currentUrl.port = String(prevPort)
      window.location.href = currentUrl.toString()
    }, 2000)
  } catch {
    emit('status-change', t('service.portRevertFailed'))
  }
}

onMounted(() => {
  if (props.showSecuritySections) {
    fetchCORSConfig()
    fetchTLSConfig()
  }
  if (props.showPortSection) {
    fetchServerConfig()
    checkPortChangeConfirmation()
  }
})

onUnmounted(() => {
  if (countdownTimer) clearInterval(countdownTimer)
})
</script>

<template>
  <div class="space-y-6">
    <!-- Port Change Confirmation Dialog -->
    <div
      v-if="props.showPortSection && portChangeConfirm"
      class="p-4 bg-gray-700 dark:bg-gray-500/30 border border-gray-900 dark:border-white rounded-lg"
    >
      <div class="flex items-center justify-between">
        <div>
          <div class="font-medium text-gray-900 dark:text-white">
            {{ t('service.portChangeConfirmTitle') }}
          </div>
          <div class="text-sm text-gray-900 dark:text-white mt-1">
            {{ t('service.portChangeConfirmDesc', { seconds: portChangeCountdown }) }}
          </div>
        </div>
        <div class="flex items-center gap-3">
          <div class="text-2xl font-bold text-gray-900 dark:text-white w-10 text-center">
            {{ portChangeCountdown }}
          </div>
          <button
            class="px-4 py-2 bg-green-600 hover:bg-green-700 text-white rounded-lg text-sm transition-colors"
            @click="confirmPortChange"
          >
            {{ t('service.confirmKeep') }}
          </button>
          <button
            class="px-4 py-2 bg-gray-200 dark:bg-gray-600 hover:bg-gray-300 dark:hover:bg-gray-500 text-gray-700 dark:text-white rounded-lg text-sm transition-colors"
            @click="revertPort"
          >
            {{ t('service.revertNow') }}
          </button>
        </div>
      </div>
    </div>

    <!-- Port Configuration -->
    <div
      v-if="props.showPortSection"
      class="glass-card p-4"
    >
      <div v-if="serverConfig">
        <div class="flex items-center justify-between">
          <div>
            <h3 class="text-sm text-gray-500 dark:text-slate-400">
              {{ t('service.port') }}
            </h3>
            <div class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('service.portDescription') }}
            </div>
          </div>
          <div class="flex items-center gap-3">
            <template v-if="!portEditing">
              <div class="text-end">
                <div class="font-mono text-lg text-gray-900 dark:text-white">
                  {{ serverConfig?.actual_port || serverConfig?.port || '-' }}
                </div>
                <div
                  v-if="portChanged"
                  class="text-xs text-yellow-600 dark:text-yellow-400"
                >
                  {{ t('service.configuredPort') }}: {{ serverConfig?.port }}
                </div>
              </div>
              <button
                class="px-3 py-1.5 text-sm bg-gray-200 dark:bg-gray-600 hover:bg-gray-300 dark:hover:bg-gray-500 text-gray-700 dark:text-white rounded-lg transition-colors"
                @click="startEditPort"
              >
                {{ t('common.edit') }}
              </button>
            </template>
            <template v-else>
              <input
                v-model="portInput"
                type="number"
                min="1"
                max="65535"
                class="w-24 px-3 py-1.5 text-sm font-mono bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400"
                :class="{ 'border-red-500': !validatePort(portInput) }"
                @keyup.enter="savePort"
                @keyup.escape="cancelEditPort"
              >
              <button
                class="px-3 py-1.5 text-sm bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg transition-colors"
                :disabled="!validatePort(portInput)"
                @click="savePort"
              >
                {{ t('common.save') }}
              </button>
              <button
                class="px-3 py-1.5 text-sm bg-gray-200 dark:bg-gray-600 hover:bg-gray-300 dark:hover:bg-gray-500 text-gray-700 dark:text-white rounded-lg transition-colors"
                @click="cancelEditPort"
              >
                {{ t('common.cancel') }}
              </button>
            </template>
          </div>
        </div>
        <div
          v-if="serverConfig?.port_auto_fallback && portChanged"
          class="mt-3 p-2 bg-yellow-50 dark:bg-yellow-900/20 rounded text-xs text-yellow-700 dark:text-yellow-300"
        >
          {{ t('service.portAutoFallbackInfo') }}
        </div>
        <div
          v-if="portEditing"
          class="mt-3 text-xs text-gray-500 dark:text-gray-400"
        >
          {{ t('service.portEditHint') }}
        </div>
      </div>
    </div>

    <template v-if="props.showSecuritySections">
      <!-- TLS Configuration -->
      <div class="glass-card security-outlined-card p-6">
        <div class="flex items-center justify-between mb-4">
          <div>
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">
              {{ t('settings.network.tls.title') }}
            </h3>
            <p class="text-sm text-gray-500 dark:text-gray-400">
              {{ t('settings.network.tls.description') }}
            </p>
          </div>
          <div class="flex gap-2">
            <button
              class="px-3 py-1.5 text-sm bg-green-600 text-white rounded-lg hover:bg-green-700"
              @click="showACMEDialog = true"
            >
              {{ t('settings.network.tls.acme.request') }}
            </button>
            <button
              class="px-3 py-1.5 text-sm bg-gray-700 dark:bg-gray-500 text-white rounded-lg hover:bg-gray-800 dark:hover:bg-gray-400"
              @click="showUploadDialog = true"
            >
              {{ t('settings.network.tls.uploadCert') }}
            </button>
            <button
              class="px-3 py-1.5 text-sm bg-gray-100 dark:bg-gray-700 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600"
              @click="showSelfSignedDialog = true"
            >
              {{ t('settings.network.tls.selfSigned') }}
            </button>
          </div>
        </div>

        <!-- Current Certificate Info -->
        <div
          v-if="tlsConfig?.has_cert && tlsConfig.cert_info"
          class="rounded-lg border border-green-200 bg-green-50 p-4 dark:border-green-900/40 dark:bg-green-900/20"
        >
          <div class="flex items-center gap-2 mb-3">
            <svg
              class="w-5 h-5 text-green-500"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"
              />
            </svg>
            <span class="font-medium text-green-700 dark:text-green-300">{{
              t('settings.network.tls.certActive')
            }}</span>
            <span
              v-if="tlsConfig.cert_info.is_self_signed"
              class="px-2 py-0.5 text-xs bg-yellow-100 dark:bg-yellow-900/50 text-yellow-700 dark:text-yellow-300 rounded"
            >
              {{ t('settings.network.tls.selfSignedLabel') }}
            </span>
            <button
              class="ms-auto px-2 py-1 text-xs bg-gray-100 dark:bg-gray-700 rounded hover:bg-gray-200 dark:hover:bg-gray-600"
              @click="reloadCertificate"
            >
              {{ t('settings.network.tls.reload') }}
            </button>
          </div>
          <div class="grid grid-cols-2 gap-4 text-sm">
            <div>
              <span class="text-gray-500 dark:text-gray-400">{{ t('settings.network.tls.domains') }}:</span>
              <div class="font-mono text-gray-900 dark:text-white">
                {{ tlsConfig.cert_info.domains.join(', ') }}
              </div>
            </div>
            <div>
              <span class="text-gray-500 dark:text-gray-400">{{ t('settings.network.tls.issuer') }}:</span>
              <div class="font-mono text-gray-900 dark:text-white truncate">
                {{ tlsConfig.cert_info.issuer }}
              </div>
            </div>
            <div>
              <span class="text-gray-500 dark:text-gray-400">{{ t('settings.network.tls.validFrom') }}:</span>
              <div class="text-gray-900 dark:text-white">
                {{ new Date(tlsConfig.cert_info.not_before).toLocaleDateString() }}
              </div>
            </div>
            <div>
              <span class="text-gray-500 dark:text-gray-400">{{ t('settings.network.tls.validUntil') }}:</span>
              <div
                :class="
                  certExpiry?.expired
                    ? 'text-red-600'
                    : certExpiry && certExpiry.days < 30
                      ? 'text-yellow-600'
                      : 'text-gray-900 dark:text-white'
                "
              >
                {{ new Date(tlsConfig.cert_info.not_after).toLocaleDateString() }}
                <span
                  v-if="certExpiry"
                  class="text-xs"
                >
                  ({{
                    certExpiry.expired
                      ? t('settings.network.tls.expired')
                      : t('settings.network.tls.daysLeft', { days: certExpiry.days })
                  }})
                </span>
              </div>
            </div>
          </div>
        </div>

        <!-- HTTPS-Only Toggle -->
        <div
          v-if="tlsConfig?.has_cert"
          class="mt-4 rounded-lg border border-gray-200 bg-gray-50 p-4 dark:border-slate-700 dark:bg-slate-800/35"
        >
          <div class="flex items-center justify-between">
            <div>
              <h4 class="font-medium text-gray-900 dark:text-white">
                {{ t('settings.network.tls.httpsOnly') }}
              </h4>
              <p class="text-sm text-gray-500 dark:text-gray-400">
                {{ t('settings.network.tls.httpsOnlyDescription') }}
              </p>
            </div>
            <label class="relative inline-flex items-center cursor-pointer">
              <input
                type="checkbox"
                :checked="tlsConfig.https_only"
                :disabled="updatingTLSSettings"
                class="sr-only peer"
                @change="updateHTTPSOnly(($event.target as HTMLInputElement).checked)"
              >
              <div
                class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-gray-400/20 dark:peer-focus:ring-gray-400/40 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full rtl:peer-checked:after:-translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:start-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-green-600 dark:peer-checked:bg-green-500"
              />
            </label>
          </div>
        </div>

        <!-- No Certificate -->
        <div
          v-else
          class="rounded-lg border border-gray-200 bg-gray-50 p-4 text-center dark:border-slate-700 dark:bg-slate-800/35"
        >
          <svg
            class="w-12 h-12 mx-auto text-gray-400 mb-2"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"
            />
          </svg>
          <p class="text-gray-500 dark:text-gray-400">
            {{ t('settings.network.tls.noCert') }}
          </p>
        </div>
      </div>

      <!-- CORS Configuration -->
      <div class="glass-card security-outlined-card p-6">
        <div class="flex items-center justify-between mb-4">
          <div>
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">
              {{ t('settings.network.corsTitle') }}
            </h3>
            <p class="text-sm text-gray-500 dark:text-gray-400">
              {{ t('settings.network.corsDescription') }}
            </p>
          </div>
          <button
            class="px-3 py-1.5 text-sm bg-gray-100 dark:bg-gray-700 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600"
            :disabled="loading"
            @click="fetchCORSConfig"
          >
            {{ t('common.refresh') }}
          </button>
        </div>

        <div
          v-if="loading"
          class="flex items-center justify-center py-8"
        >
          <svg
            class="animate-spin w-6 h-6 text-gray-900 dark:text-gray-300"
            fill="none"
            viewBox="0 0 24 24"
          >
            <circle
              class="opacity-25"
              cx="12"
              cy="12"
              r="10"
              stroke="currentColor"
              stroke-width="4"
            />
            <path
              class="opacity-75"
              fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
            />
          </svg>
        </div>

        <template v-else-if="corsConfig">
          <div class="flex gap-2 mb-4">
            <input
              v-model="newOrigin"
              type="text"
              :placeholder="t('settings.network.originPlaceholder')"
              class="flex-1 bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-400 border border-gray-200 dark:border-gray-600"
              @keyup.enter="addOrigin"
            >
            <button
              class="px-4 py-2 bg-gray-700 dark:bg-gray-500 text-white rounded-lg hover:bg-gray-800 dark:hover:bg-gray-400 disabled:opacity-50"
              :disabled="addingOrigin || !newOrigin.trim()"
              @click="addOrigin"
            >
              {{ t('common.add') }}
            </button>
          </div>

          <div
            v-if="corsConfig.dynamic_origins.length > 0"
            class="mb-4"
          >
            <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              {{ t('settings.network.dynamicOrigins') }}
            </h4>
            <div class="space-y-2">
              <div
                v-for="origin in corsConfig.dynamic_origins"
                :key="origin"
                class="flex items-center justify-between p-3 bg-gray-100 dark:bg-gray-700/30 rounded-lg border border-gray-200 dark:border-gray-600"
              >
                <span class="text-sm text-gray-900 dark:text-white font-mono">{{ origin }}</span>
                <button
                  class="p-1 text-gray-400 hover:text-red-500 transition-colors"
                  @click="removeOrigin(origin)"
                >
                  <svg
                    class="w-5 h-5"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
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
            </div>
          </div>

          <div>
            <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              {{ t('settings.network.defaultOrigins') }}
            </h4>
            <div class="space-y-2">
              <div
                v-for="origin in corsConfig.allowed_origins.filter(
                  (o) => !corsConfig!.dynamic_origins.includes(o)
                )"
                :key="origin"
                class="flex items-center justify-between rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-slate-700 dark:bg-slate-800/35"
              >
                <span class="text-sm text-gray-600 dark:text-gray-400 font-mono">{{ origin }}</span>
                <span class="text-xs text-gray-400 dark:text-gray-500">{{
                  t('settings.network.builtIn')
                }}</span>
              </div>
            </div>
          </div>
        </template>
      </div>

      <!-- Upload Certificate Dialog -->
      <Teleport to="body">
        <div
          v-if="showUploadDialog"
          class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
          @click.self="showUploadDialog = false"
        >
          <div
            class="bg-white dark:bg-gray-700 rounded-xl p-6 w-full max-w-2xl mx-4 max-h-[90vh] overflow-y-auto"
          >
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">
              {{ t('settings.network.tls.uploadCert') }}
            </h3>
            <div class="space-y-4">
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{
                  t('settings.network.tls.certPem')
                }}</label>
                <textarea
                  v-model="certPem"
                  rows="6"
                  class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 font-mono text-sm"
                  :placeholder="t('settings.network.tls.certPlaceholder')"
                  @blur="parseCert"
                />
              </div>
              <div
                v-if="parsedCertInfo"
                class="p-3 bg-gray-700 dark:bg-gray-500/20 rounded-lg text-sm"
              >
                <div>
                  <strong>{{ t('settings.network.tls.domains') }}:</strong>
                  {{ parsedCertInfo.domains.join(', ') }}
                </div>
                <div>
                  <strong>{{ t('settings.network.tls.validUntil') }}:</strong>
                  {{ new Date(parsedCertInfo.not_after).toLocaleDateString() }}
                </div>
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{
                  t('settings.network.tls.keyPem')
                }}</label>
                <textarea
                  v-model="keyPem"
                  rows="6"
                  class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 font-mono text-sm"
                  :placeholder="t('settings.network.tls.keyPlaceholder')"
                />
              </div>
            </div>
            <div class="flex justify-end gap-2 mt-6">
              <button
                class="px-4 py-2 bg-gray-100 dark:bg-gray-700 rounded-lg"
                @click="showUploadDialog = false"
              >
                {{ t('common.cancel') }}
              </button>
              <button
                class="px-4 py-2 bg-gray-700 dark:bg-gray-500 text-white rounded-lg disabled:opacity-50"
                :disabled="uploading || !certPem || !keyPem"
                @click="uploadCertificate"
              >
                {{ uploading ? t('common.uploading') : t('common.upload') }}
              </button>
            </div>
          </div>
        </div>
      </Teleport>

      <!-- Self-Signed Dialog -->
      <Teleport to="body">
        <div
          v-if="showSelfSignedDialog"
          class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
          @click.self="showSelfSignedDialog = false"
        >
          <div class="bg-white dark:bg-gray-700 rounded-xl p-6 w-full max-w-md mx-4">
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">
              {{ t('settings.network.tls.generateSelfSigned') }}
            </h3>
            <div class="space-y-4">
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{
                  t('settings.network.tls.domains')
                }}</label>
                <input
                  v-model="selfSignedDomains"
                  type="text"
                  class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2"
                  placeholder="localhost, example.com"
                >
                <p class="text-xs text-gray-500 mt-1">
                  {{ t('settings.network.tls.domainsHint') }}
                </p>
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{
                  t('settings.network.tls.validDays')
                }}</label>
                <input
                  v-model.number="selfSignedDays"
                  type="number"
                  min="1"
                  max="3650"
                  class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2"
                >
              </div>
            </div>
            <div class="flex justify-end gap-2 mt-6">
              <button
                class="px-4 py-2 bg-gray-100 dark:bg-gray-700 rounded-lg"
                @click="showSelfSignedDialog = false"
              >
                {{ t('common.cancel') }}
              </button>
              <button
                class="px-4 py-2 bg-gray-700 dark:bg-gray-500 text-white rounded-lg disabled:opacity-50"
                :disabled="generating"
                @click="generateSelfSigned"
              >
                {{ generating ? t('common.generating') : t('common.generate') }}
              </button>
            </div>
          </div>
        </div>
      </Teleport>

      <!-- ACME Dialog -->
      <Teleport to="body">
        <div
          v-if="showACMEDialog"
          class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
          @click.self="showACMEDialog = false"
        >
          <div
            class="bg-white dark:bg-gray-800 rounded-xl p-6 w-full max-w-md mx-4 max-h-[90vh] overflow-y-auto"
          >
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">
              {{ t('settings.network.tls.acme.title') }}
            </h3>
            <div class="space-y-4">
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{
                  t('settings.network.tls.acme.provider')
                }}</label>
                <select
                  v-model="acmeProvider"
                  class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2"
                >
                  <option value="letsencrypt">
                    Let's Encrypt
                  </option>
                  <option value="zerossl">
                    ZeroSSL
                  </option>
                  <option value="letsencrypt-staging">
                    Let's Encrypt (Staging)
                  </option>
                </select>
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{
                  t('settings.network.tls.acme.email')
                }}</label>
                <input
                  v-model="acmeEmail"
                  type="email"
                  class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2"
                  placeholder="admin@example.com"
                >
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{
                  t('settings.network.tls.domains')
                }}</label>
                <input
                  v-model="acmeDomains"
                  type="text"
                  class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2"
                  placeholder="example.com, www.example.com"
                >
                <p class="text-xs text-gray-500 mt-1">
                  {{ t('settings.network.tls.acme.domainsHint') }}
                </p>
              </div>

              <!-- Challenge Type -->
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{
                  t('settings.network.tls.acme.challengeType')
                }}</label>
                <div class="flex gap-2">
                  <button
                    v-for="ct in ['http-01', 'dns-01'] as const"
                    :key="ct"
                    :class="[
                      'flex-1 px-4 py-2 rounded-lg text-sm font-medium transition-colors',
                      acmeChallengeType === ct
                        ? 'bg-gray-700 text-white'
                        : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600',
                    ]"
                    @click="acmeChallengeType = ct"
                  >
                    {{
                      ct === 'http-01'
                        ? t('settings.network.tls.acme.httpChallenge')
                        : t('settings.network.tls.acme.dnsChallenge')
                    }}
                  </button>
                </div>
              </div>

              <!-- DNS Provider (only for dns-01) -->
              <template v-if="acmeChallengeType === 'dns-01'">
                <div>
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{
                    t('settings.network.tls.acme.dnsProvider')
                  }}</label>
                  <select
                    v-model="acmeDNSProvider"
                    class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2"
                    @change="onDNSProviderChange"
                  >
                    <option value="cloudflare">
                      Cloudflare
                    </option>
                    <option value="route53">
                      AWS Route53
                    </option>
                    <option value="godaddy">
                      GoDaddy
                    </option>
                    <option value="namecheap">
                      Namecheap
                    </option>
                    <option value="alidns">
                      Aliyun DNS
                    </option>
                    <option value="tencentcloud">
                      Tencent Cloud / DNSPod
                    </option>
                  </select>
                </div>

                <!-- DNS Credentials -->
                <div>
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{
                    t('settings.network.tls.acme.dnsCredentials')
                  }}</label>
                  <p class="text-xs text-gray-500 mb-2">
                    {{ t('settings.network.tls.acme.dnsCredentialHint') }}
                  </p>
                  <div class="space-y-2">
                    <div
                      v-for="(cred, index) in acmeDNSCredentials"
                      :key="index"
                      class="flex gap-2"
                    >
                      <input
                        v-model="cred.key"
                        type="text"
                        class="flex-1 bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-3 py-1.5 text-sm font-mono"
                        :placeholder="t('settings.network.tls.acme.envKey')"
                      >
                      <input
                        v-model="cred.value"
                        type="password"
                        class="flex-1 bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-3 py-1.5 text-sm"
                        :placeholder="t('settings.network.tls.acme.envValue')"
                      >
                      <button
                        v-if="acmeDNSCredentials.length > 1"
                        class="px-2 text-gray-400 hover:text-red-500"
                        @click="removeDNSCredential(index)"
                      >
                        ✕
                      </button>
                    </div>
                  </div>
                  <button
                    class="mt-2 text-sm text-blue-600 dark:text-blue-400 hover:underline"
                    @click="addDNSCredential"
                  >
                    + {{ t('settings.network.tls.acme.addCredential') }}
                  </button>
                </div>
              </template>

              <div
                class="p-3 bg-yellow-50 dark:bg-yellow-900/20 rounded-lg text-sm text-yellow-700 dark:text-yellow-300"
              >
                <strong>{{ t('settings.network.tls.acme.note') }}:</strong>
                {{
                  acmeChallengeType === 'dns-01'
                    ? t('settings.network.tls.acme.noteTextDNS')
                    : t('settings.network.tls.acme.noteText')
                }}
              </div>
            </div>
            <div class="flex justify-end gap-2 mt-6">
              <button
                class="px-4 py-2 bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 rounded-lg"
                @click="showACMEDialog = false"
              >
                {{ t('common.cancel') }}
              </button>
              <button
                class="px-4 py-2 bg-green-600 text-white rounded-lg disabled:opacity-50"
                :disabled="requestingACME || !acmeEmail || !acmeDomains"
                @click="requestACMECert"
              >
                {{
                  requestingACME ? t('common.requesting') : t('settings.network.tls.acme.request')
                }}
              </button>
            </div>
          </div>
        </div>
      </Teleport>
    </template>
  </div>
</template>
