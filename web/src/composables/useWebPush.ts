import { getVapidKey, subscribePush, unsubscribePush } from '@/api/webpush'

function urlBase64ToUint8Array(base64String: string): Uint8Array {
  const padding = '='.repeat((4 - (base64String.length % 4)) % 4)
  const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/')
  const raw = atob(base64)
  return Uint8Array.from([...raw].map((c) => c.charCodeAt(0)))
}

export function useWebPush() {
  async function subscribe(): Promise<boolean> {
    if (!('serviceWorker' in navigator) || !('PushManager' in window)) return false

    const permission = await Notification.requestPermission()
    if (permission !== 'granted') return false

    // Register service worker
    const reg = await navigator.serviceWorker.register('/sw.js')

    // Check if already subscribed
    const existing = await reg.pushManager.getSubscription()
    if (existing) return true

    // Get VAPID public key from server
    const publicKey = await getVapidKey()

    // Subscribe to push
    const sub = await reg.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: urlBase64ToUint8Array(publicKey),
    })

    // Send subscription to server
    await subscribePush(sub.toJSON())
    return true
  }

  async function unsubscribe(): Promise<void> {
    const reg = await navigator.serviceWorker.getRegistration()
    if (!reg) return
    const sub = await reg.pushManager.getSubscription()
    if (!sub) return
    await unsubscribePush(sub.endpoint)
    await sub.unsubscribe()
  }

  return { subscribe, unsubscribe }
}
