import { ref, onMounted, onUnmounted } from 'vue'
import { authFetch } from '@/api/client'

export interface NetworkInterface {
  interface: string
  address: string
  type: 'wifi' | 'ethernet' | 'loopback' | 'virtual' | 'unknown'
  is_up: boolean
  is_ipv6: boolean
}

export interface NetworkAddresses {
  local: string
  lan: NetworkInterface[]
  hostname?: string
  port: number
  preferred: string
}

export interface NetworkStatus {
  status: 'healthy' | 'degraded'
  interfaces_count: number
  has_lan_access: boolean
  has_hostname?: boolean
  error?: string
}

export function useNetwork() {
  const addresses = ref<NetworkAddresses | null>(null)
  const status = ref<NetworkStatus | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)
  const copied = ref(false)
  let copyTimeout: ReturnType<typeof setTimeout> | null = null

  async function fetchAddresses() {
    loading.value = true
    error.value = null
    try {
      const response = await authFetch('/api/v1/network/addresses')
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`)
      }
      addresses.value = await response.json()
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch network addresses'
      console.error('Failed to fetch network addresses:', e)
    } finally {
      loading.value = false
    }
  }

  async function fetchStatus() {
    try {
      const response = await authFetch('/api/v1/network/status')
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`)
      }
      status.value = await response.json()
    } catch (e) {
      console.error('Failed to fetch network status:', e)
    }
  }

  async function copyToClipboard(text: string): Promise<boolean> {
    try {
      await navigator.clipboard.writeText(text)
      copied.value = true

      // Clear previous timeout
      if (copyTimeout) {
        clearTimeout(copyTimeout)
      }

      // Reset copied state after 2 seconds
      copyTimeout = setTimeout(() => {
        copied.value = false
      }, 2000)

      return true
    } catch (e) {
      console.error('Failed to copy to clipboard:', e)
      return false
    }
  }

  function getInterfaceIcon(type: NetworkInterface['type']): string {
    switch (type) {
      case 'wifi':
        return 'M8.111 16.404a5.5 5.5 0 017.778 0M12 20h.01m-7.08-7.071c3.904-3.905 10.236-3.905 14.141 0M1.394 9.393c5.857-5.857 15.355-5.857 21.213 0'
      case 'ethernet':
        return 'M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z'
      case 'loopback':
        return 'M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15'
      default:
        return 'M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9'
    }
  }

  onMounted(() => {
    fetchAddresses()
    fetchStatus()
  })

  onUnmounted(() => {
    if (copyTimeout) {
      clearTimeout(copyTimeout)
    }
  })

  return {
    addresses,
    status,
    loading,
    error,
    copied,
    fetchAddresses,
    fetchStatus,
    copyToClipboard,
    getInterfaceIcon,
  }
}
