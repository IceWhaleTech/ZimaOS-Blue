<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { securityApi, type CORSConfig, type TLSConfig, type CertificateInfo, type ACMEStatus } from '@/api/security'

const { t } = useI18n()

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
const requestingACME = ref(false)

// HTTPS-only state
const updatingTLSSettings = ref(false)

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
  const domains = selfSignedDomains.value.split(',').map(d => d.trim()).filter(d => d)
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
  const domains = acmeDomains.value.split(',').map(d => d.trim()).filter(d => d)
  if (!acmeEmail.value.trim() || domains.length === 0) {
    emit('status-change', t('settings.network.tls.acme.emailDomainRequired'))
    return
  }
  requestingACME.value = true
  try {
    const response = await securityApi.requestACMECert(acmeEmail.value, domains, acmeProvider.value)
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

async function updateHTTPSOnly(enabled: boolean) {
  updatingTLSSettings.value = true
  try {
    const port = tlsConfig.value?.https_port || 443
    const response = await securityApi.updateTLSSettings(enabled, port)
    tlsConfig.value = response.data
    emit('status-change', enabled ? t('settings.network.tls.httpsOnlyEnabled') : t('settings.network.tls.httpsOnlyDisabled'))
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

onMounted(() => {
  fetchCORSConfig()
  fetchTLSConfig()
})
</script>

<template>
  <div class="space-y-6">
    <!-- TLS Configuration -->
    <div class="glass-card p-6">
      <div class="flex items-center justify-between mb-4">
        <div>
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
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
            class="px-3 py-1.5 text-sm bg-gray-700 dark:bg-gray-700 text-white rounded-lg hover:bg-gray-700 dark:bg-gray-700/90"
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
      <div v-if="tlsConfig?.has_cert && tlsConfig.cert_info" class="p-4 bg-green-50 dark:bg-green-900/20 rounded-lg">
        <div class="flex items-center gap-2 mb-3">
          <svg class="w-5 h-5 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
          </svg>
          <span class="font-medium text-green-700 dark:text-green-300">{{ t('settings.network.tls.certActive') }}</span>
          <span v-if="tlsConfig.cert_info.is_self_signed" class="px-2 py-0.5 text-xs bg-yellow-100 dark:bg-yellow-900/50 text-yellow-700 dark:text-yellow-300 rounded">
            {{ t('settings.network.tls.selfSignedLabel') }}
          </span>
          <button class="ml-auto px-2 py-1 text-xs bg-gray-100 dark:bg-gray-700 rounded hover:bg-gray-200 dark:hover:bg-gray-600" @click="reloadCertificate">
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
            <div :class="certExpiry?.expired ? 'text-red-600' : certExpiry && certExpiry.days < 30 ? 'text-yellow-600' : 'text-gray-900 dark:text-white'">
              {{ new Date(tlsConfig.cert_info.not_after).toLocaleDateString() }}
              <span v-if="certExpiry" class="text-xs">
                ({{ certExpiry.expired ? t('settings.network.tls.expired') : t('settings.network.tls.daysLeft', { days: certExpiry.days }) }})
              </span>
            </div>
          </div>
        </div>
      </div>

      <!-- HTTPS-Only Toggle -->
      <div v-if="tlsConfig?.has_cert" class="mt-4 p-4 bg-gray-50 dark:bg-gray-700 rounded-lg">
        <div class="flex items-center justify-between">
          <div>
            <h4 class="font-medium text-gray-900 dark:text-white">{{ t('settings.network.tls.httpsOnly') }}</h4>
            <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('settings.network.tls.httpsOnlyDescription') }}</p>
          </div>
          <label class="relative inline-flex items-center cursor-pointer">
            <input
              type="checkbox"
              :checked="tlsConfig.https_only"
              :disabled="updatingTLSSettings"
              class="sr-only peer"
              @change="updateHTTPSOnly(($event.target as HTMLInputElement).checked)"
            />
            <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-gray-400/20 dark:peer-focus:ring-gray-400/40 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-gray-700 dark:bg-gray-700"></div>
          </label>
        </div>
      </div>

      <!-- No Certificate -->
      <div v-else class="p-4 bg-gray-50 dark:bg-gray-700 rounded-lg text-center">
        <svg class="w-12 h-12 mx-auto text-gray-400 mb-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
        </svg>
        <p class="text-gray-500 dark:text-gray-400">{{ t('settings.network.tls.noCert') }}</p>
      </div>
    </div>

    <!-- CORS Configuration -->
    <div class="glass-card p-6">
      <div class="flex items-center justify-between mb-4">
        <div>
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('settings.network.corsTitle') }}</h3>
          <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('settings.network.corsDescription') }}</p>
        </div>
        <button class="px-3 py-1.5 text-sm bg-gray-100 dark:bg-gray-700 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600" :disabled="loading" @click="fetchCORSConfig">
          {{ t('common.refresh') }}
        </button>
      </div>

      <div v-if="loading" class="flex items-center justify-center py-8">
        <svg class="animate-spin w-6 h-6 text-gray-900 dark:text-gray-300" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
        </svg>
      </div>

      <template v-else-if="corsConfig">
        <div class="flex gap-2 mb-4">
          <input v-model="newOrigin" type="text" :placeholder="t('settings.network.originPlaceholder')" class="flex-1 bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-400 border border-gray-200 dark:border-gray-600" @keyup.enter="addOrigin" />
          <button class="px-4 py-2 bg-gray-700 dark:bg-gray-700 text-white rounded-lg hover:bg-gray-700 dark:bg-gray-700/90 disabled:opacity-50" :disabled="addingOrigin || !newOrigin.trim()" @click="addOrigin">{{ t('common.add') }}</button>
        </div>

        <div v-if="corsConfig.dynamic_origins.length > 0" class="mb-4">
          <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('settings.network.dynamicOrigins') }}</h4>
          <div class="space-y-2">
            <div v-for="origin in corsConfig.dynamic_origins" :key="origin" class="flex items-center justify-between p-3 bg-gray-100 dark:bg-gray-700/30 rounded-lg border border-gray-200 dark:border-gray-600">
              <span class="text-sm text-gray-900 dark:text-white font-mono">{{ origin }}</span>
              <button class="p-1 text-gray-400 hover:text-red-500 transition-colors" @click="removeOrigin(origin)">
                <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg>
              </button>
            </div>
          </div>
        </div>

        <div>
          <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('settings.network.defaultOrigins') }}</h4>
          <div class="space-y-2">
            <div v-for="origin in corsConfig.allowed_origins.filter(o => !corsConfig!.dynamic_origins.includes(o))" :key="origin" class="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700 rounded-lg">
              <span class="text-sm text-gray-600 dark:text-gray-400 font-mono">{{ origin }}</span>
              <span class="text-xs text-gray-400 dark:text-gray-500">{{ t('settings.network.builtIn') }}</span>
            </div>
          </div>
        </div>
      </template>
    </div>

    <!-- Upload Certificate Dialog -->
    <Teleport to="body">
      <div v-if="showUploadDialog" class="fixed inset-0 bg-black/50 flex items-center justify-center z-50" @click.self="showUploadDialog = false">
        <div class="bg-white dark:bg-gray-700 rounded-xl p-6 w-full max-w-2xl mx-4 max-h-[90vh] overflow-y-auto">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">{{ t('settings.network.tls.uploadCert') }}</h3>
          <div class="space-y-4">
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ t('settings.network.tls.certPem') }}</label>
              <textarea v-model="certPem" rows="6" class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 font-mono text-sm" :placeholder="t('settings.network.tls.certPlaceholder')" @blur="parseCert" />
            </div>
            <div v-if="parsedCertInfo" class="p-3 bg-gray-700 dark:bg-gray-700 dark:bg-gray-700 dark:bg-gray-700/20 rounded-lg text-sm">
              <div><strong>{{ t('settings.network.tls.domains') }}:</strong> {{ parsedCertInfo.domains.join(', ') }}</div>
              <div><strong>{{ t('settings.network.tls.validUntil') }}:</strong> {{ new Date(parsedCertInfo.not_after).toLocaleDateString() }}</div>
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ t('settings.network.tls.keyPem') }}</label>
              <textarea v-model="keyPem" rows="6" class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 font-mono text-sm" :placeholder="t('settings.network.tls.keyPlaceholder')" />
            </div>
          </div>
          <div class="flex justify-end gap-2 mt-6">
            <button class="px-4 py-2 bg-gray-100 dark:bg-gray-700 rounded-lg" @click="showUploadDialog = false">{{ t('common.cancel') }}</button>
            <button class="px-4 py-2 bg-gray-700 dark:bg-gray-700 text-white rounded-lg disabled:opacity-50" :disabled="uploading || !certPem || !keyPem" @click="uploadCertificate">
              {{ uploading ? t('common.uploading') : t('common.upload') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Self-Signed Dialog -->
    <Teleport to="body">
      <div v-if="showSelfSignedDialog" class="fixed inset-0 bg-black/50 flex items-center justify-center z-50" @click.self="showSelfSignedDialog = false">
        <div class="bg-white dark:bg-gray-700 rounded-xl p-6 w-full max-w-md mx-4">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">{{ t('settings.network.tls.generateSelfSigned') }}</h3>
          <div class="space-y-4">
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ t('settings.network.tls.domains') }}</label>
              <input v-model="selfSignedDomains" type="text" class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2" placeholder="localhost, example.com" />
              <p class="text-xs text-gray-500 mt-1">{{ t('settings.network.tls.domainsHint') }}</p>
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ t('settings.network.tls.validDays') }}</label>
              <input v-model.number="selfSignedDays" type="number" min="1" max="3650" class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2" />
            </div>
          </div>
          <div class="flex justify-end gap-2 mt-6">
            <button class="px-4 py-2 bg-gray-100 dark:bg-gray-700 rounded-lg" @click="showSelfSignedDialog = false">{{ t('common.cancel') }}</button>
            <button class="px-4 py-2 bg-gray-700 dark:bg-gray-700 text-white rounded-lg disabled:opacity-50" :disabled="generating" @click="generateSelfSigned">
              {{ generating ? t('common.generating') : t('common.generate') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- ACME Dialog -->
    <Teleport to="body">
      <div v-if="showACMEDialog" class="fixed inset-0 bg-black/50 flex items-center justify-center z-50" @click.self="showACMEDialog = false">
        <div class="bg-white dark:bg-gray-700 rounded-xl p-6 w-full max-w-md mx-4">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">{{ t('settings.network.tls.acme.title') }}</h3>
          <div class="space-y-4">
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ t('settings.network.tls.acme.provider') }}</label>
              <select v-model="acmeProvider" class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2">
                <option value="letsencrypt">Let's Encrypt</option>
                <option value="zerossl">ZeroSSL</option>
                <option value="letsencrypt-staging">Let's Encrypt (Staging)</option>
              </select>
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ t('settings.network.tls.acme.email') }}</label>
              <input v-model="acmeEmail" type="email" class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2" placeholder="admin@example.com" />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ t('settings.network.tls.domains') }}</label>
              <input v-model="acmeDomains" type="text" class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2" placeholder="example.com, www.example.com" />
              <p class="text-xs text-gray-500 mt-1">{{ t('settings.network.tls.acme.domainsHint') }}</p>
            </div>
            <div class="p-3 bg-yellow-50 dark:bg-yellow-900/20 rounded-lg text-sm text-yellow-700 dark:text-yellow-300">
              <strong>{{ t('settings.network.tls.acme.note') }}:</strong> {{ t('settings.network.tls.acme.noteText') }}
            </div>
          </div>
          <div class="flex justify-end gap-2 mt-6">
            <button class="px-4 py-2 bg-gray-100 dark:bg-gray-700 rounded-lg" @click="showACMEDialog = false">{{ t('common.cancel') }}</button>
            <button class="px-4 py-2 bg-green-600 text-white rounded-lg disabled:opacity-50" :disabled="requestingACME || !acmeEmail || !acmeDomains" @click="requestACMECert">
              {{ requestingACME ? t('common.requesting') : t('settings.network.tls.acme.request') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
