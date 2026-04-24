<script setup lang="ts">
import { ref, watch } from 'vue'
import QRCode from 'qrcode'

const props = withDefaults(
  defineProps<{
    value: string
    alt?: string
    size?: number
    imageClass?: string
    wrapperClass?: string
    errorText?: string
  }>(),
  {
    alt: 'QR Code',
    size: 256,
    imageClass: '',
    wrapperClass: '',
    errorText: 'Failed to render QR code',
  }
)

const dataUrl = ref('')
const hasError = ref(false)

watch(
  () => [props.value, props.size] as const,
  async ([value, size]) => {
    const trimmedValue = value.trim()
    if (!trimmedValue) {
      dataUrl.value = ''
      hasError.value = false
      return
    }

    try {
      hasError.value = false
      dataUrl.value = await QRCode.toDataURL(trimmedValue, {
        width: size,
        margin: 1,
      })
    } catch (error) {
      console.error('Failed to generate QR code', error)
      dataUrl.value = ''
      hasError.value = true
    }
  },
  { immediate: true }
)
</script>

<template>
  <div
    v-if="value"
    class="qr-code-display"
    :class="wrapperClass"
    :data-qr-value="value"
  >
    <img
      v-if="dataUrl"
      :src="dataUrl"
      :alt="alt"
      :class="imageClass"
    >
    <div
      v-else
      class="qr-code-display__fallback"
      :data-qr-error="hasError ? 'true' : 'false'"
    >
      {{ errorText }}
    </div>
  </div>
</template>

<style scoped>
.qr-code-display {
  display: flex;
  justify-content: center;
}

.qr-code-display__fallback {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 6rem;
  min-width: 6rem;
  border-radius: 0.75rem;
  border: 1px dashed rgba(148, 163, 184, 0.5);
  color: rgb(100 116 139);
  font-size: 0.875rem;
  text-align: center;
  padding: 0.75rem;
}
</style>
