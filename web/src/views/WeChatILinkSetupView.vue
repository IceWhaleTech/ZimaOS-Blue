<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import type { LocationQueryRaw } from 'vue-router'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  completeWeChatILinkSetupSession,
  getWeChatILinkSetupSession,
} from '@/api/wechat-ilink-setup'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()

const loading = ref(false)
const submitting = ref(false)
const errorMessage = ref('')
const success = ref(false)
const pairingPayload = ref('')
const apiBaseURL = ref('')
const botToken = ref('')

const sessionId = computed(() => String(route.query.session_id || '').trim())

function readQueryString(value: unknown): string {
  if (Array.isArray(value)) {
    return typeof value[0] === 'string' ? value[0].trim() : ''
  }
  return typeof value === 'string' ? value.trim() : ''
}

function readQueryParam(keys: string[]) {
  for (const key of keys) {
    const value = readQueryString(route.query[key])
    if (value) {
      return { key, value }
    }
  }
  return { key: '', value: '' }
}

function extractAutoSetupPayload() {
  const cleanedQuery: LocationQueryRaw = { ...route.query }
  let changed = false
  const removeKeys = (...keys: string[]) => {
    for (const key of keys) {
      if (key in cleanedQuery) {
        delete cleanedQuery[key]
        changed = true
      }
    }
  }

  const pairingPayloadParam = readQueryParam(['pairing_payload', 'payload'])
  if (pairingPayloadParam.value) {
    pairingPayload.value = pairingPayloadParam.value
    removeKeys('pairing_payload', 'payload', 'api_base_url', 'apiBaseURL', 'bot_token', 'botToken')
    return {
      payload: pairingPayloadParam.value,
      cleanedQuery,
      changed,
    }
  }

  const apiBaseURLParam = readQueryParam(['api_base_url', 'apiBaseURL'])
  const botTokenParam = readQueryParam(['bot_token', 'botToken'])

  if (apiBaseURLParam.value) {
    apiBaseURL.value = apiBaseURLParam.value
  }
  if (botTokenParam.value) {
    botToken.value = botTokenParam.value
  }
  if (apiBaseURLParam.value || botTokenParam.value) {
    removeKeys('api_base_url', 'apiBaseURL', 'bot_token', 'botToken')
  }

  return {
    payload:
      apiBaseURLParam.value && botTokenParam.value
        ? {
            api_base_url: apiBaseURLParam.value,
            bot_token: botTokenParam.value,
          }
        : null,
    cleanedQuery,
    changed,
  }
}

function buildPairingPayload(): unknown {
  if (pairingPayload.value.trim()) {
    try {
      return JSON.parse(pairingPayload.value)
    } catch {
      return pairingPayload.value.trim()
    }
  }

  return {
    api_base_url: apiBaseURL.value.trim(),
    bot_token: botToken.value.trim(),
  }
}

async function loadSession() {
  if (!sessionId.value) {
    errorMessage.value = t('channels.wechatILinkSetupMissingSession')
    return
  }

  loading.value = true
  errorMessage.value = ''
  try {
    const response = await getWeChatILinkSetupSession(sessionId.value)
    if (response.status >= 400) {
      errorMessage.value = response.data?.error || t('channels.wechatILinkSetupLoadFailed')
      return
    }
    if (response.data?.status === 'connected') {
      success.value = true
    }
    if (response.data?.error) {
      errorMessage.value = response.data.error
    }
  } catch {
    errorMessage.value = t('channels.wechatILinkSetupLoadFailed')
  } finally {
    loading.value = false
  }
}

async function clearSensitiveQuery(cleanedQuery: LocationQueryRaw) {
  await router.replace({ query: cleanedQuery })
}

async function submitSetup(payloadOverride?: unknown) {
  if (!sessionId.value) {
    errorMessage.value = t('channels.wechatILinkSetupMissingSession')
    return
  }

  submitting.value = true
  errorMessage.value = ''
  try {
    const response = await completeWeChatILinkSetupSession(
      sessionId.value,
      payloadOverride ?? buildPairingPayload()
    )
    if (response.status >= 400) {
      errorMessage.value = response.data?.error || t('channels.wechatILinkSetupSubmitFailed')
      return
    }
    success.value = true
  } catch {
    errorMessage.value = t('channels.wechatILinkSetupSubmitFailed')
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  void (async () => {
    const autoSetup = extractAutoSetupPayload()
    if (autoSetup.changed) {
      await clearSensitiveQuery(autoSetup.cleanedQuery)
    }

    await loadSession()
    if (!success.value && !errorMessage.value && autoSetup.payload) {
      await submitSetup(autoSetup.payload)
    }
  })()
})
</script>

<template>
  <main class="wechat-ilink-setup">
    <section class="wechat-ilink-setup__panel">
      <p class="wechat-ilink-setup__eyebrow">
        {{ t('channels.wechatILinkSetupEyebrow') }}
      </p>
      <h1 class="wechat-ilink-setup__title">
        {{ t('channels.wechatILinkSetupTitle') }}
      </h1>
      <p class="wechat-ilink-setup__description">
        {{ t('channels.wechatILinkSetupDescription') }}
      </p>

      <div
        v-if="loading"
        class="wechat-ilink-setup__status"
      >
        {{ t('common.loading') }}
      </div>
      <div
        v-else-if="success"
        class="wechat-ilink-setup__status wechat-ilink-setup__status--success"
      >
        {{ t('channels.wechatILinkSetupSuccess') }}
      </div>
      <div
        v-else
        class="wechat-ilink-setup__form"
      >
        <label class="wechat-ilink-setup__field">
          <span>{{ t('channels.wechatILinkPairingPayload') }}</span>
          <textarea
            v-model="pairingPayload"
            rows="6"
            class="wechat-ilink-setup__textarea"
            :placeholder="t('channels.wechatILinkPairingPayloadPlaceholder')"
          />
        </label>

        <div class="wechat-ilink-setup__divider">
          {{ t('common.optional') }}
        </div>

        <label class="wechat-ilink-setup__field">
          <span>{{ t('channels.apiBaseURL') }}</span>
          <input
            v-model="apiBaseURL"
            type="url"
            class="wechat-ilink-setup__input"
            :placeholder="t('channels.placeholderILinkAPIBaseURL')"
          >
        </label>

        <label class="wechat-ilink-setup__field">
          <span>{{ t('channels.botToken') }}</span>
          <input
            v-model="botToken"
            type="password"
            class="wechat-ilink-setup__input"
            :placeholder="t('channels.placeholderBotTokenGeneric')"
          >
        </label>

        <p
          v-if="errorMessage"
          class="wechat-ilink-setup__error"
        >
          {{ errorMessage }}
        </p>

        <button
          type="button"
          class="wechat-ilink-setup__submit"
          :disabled="submitting"
          @click="submitSetup()"
        >
          {{ submitting ? t('channels.wechatILinkSetupSubmitting') : t('channels.wechatILinkSetupSubmit') }}
        </button>
      </div>
    </section>
  </main>
</template>

<style scoped>
.wechat-ilink-setup {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 24px;
  background:
    radial-gradient(circle at top, rgba(34, 197, 94, 0.16), transparent 42%),
    linear-gradient(180deg, #f4fbf6 0%, #eef4f0 100%);
}

.wechat-ilink-setup__panel {
  width: min(100%, 480px);
  border-radius: 24px;
  background: rgba(255, 255, 255, 0.92);
  border: 1px solid rgba(22, 101, 52, 0.12);
  box-shadow: 0 18px 50px rgba(15, 23, 42, 0.08);
  padding: 28px;
}

.wechat-ilink-setup__eyebrow {
  margin: 0 0 8px;
  font-size: 12px;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: #166534;
}

.wechat-ilink-setup__title {
  margin: 0;
  font-size: 28px;
  line-height: 1.1;
  color: #0f172a;
}

.wechat-ilink-setup__description {
  margin: 12px 0 0;
  color: #475569;
  line-height: 1.6;
}

.wechat-ilink-setup__form {
  margin-top: 24px;
  display: grid;
  gap: 14px;
}

.wechat-ilink-setup__field {
  display: grid;
  gap: 8px;
  color: #0f172a;
  font-weight: 600;
}

.wechat-ilink-setup__input,
.wechat-ilink-setup__textarea {
  border-radius: 14px;
  border: 1px solid rgba(148, 163, 184, 0.5);
  background: #fff;
  padding: 12px 14px;
  font: inherit;
  color: #0f172a;
}

.wechat-ilink-setup__textarea {
  resize: vertical;
  min-height: 120px;
}

.wechat-ilink-setup__divider {
  text-align: center;
  font-size: 12px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: #64748b;
}

.wechat-ilink-setup__status {
  margin-top: 24px;
  border-radius: 16px;
  background: rgba(15, 23, 42, 0.04);
  padding: 16px;
  color: #0f172a;
}

.wechat-ilink-setup__status--success {
  background: rgba(34, 197, 94, 0.12);
  color: #166534;
}

.wechat-ilink-setup__error {
  margin: 0;
  color: #b91c1c;
}

.wechat-ilink-setup__submit {
  border: none;
  border-radius: 999px;
  background: linear-gradient(135deg, #15803d 0%, #22c55e 100%);
  color: #fff;
  font: inherit;
  font-weight: 700;
  padding: 12px 18px;
}

.wechat-ilink-setup__submit:disabled {
  opacity: 0.7;
}
</style>
