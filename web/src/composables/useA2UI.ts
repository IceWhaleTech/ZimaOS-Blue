import { ref, onMounted, onUnmounted, readonly } from 'vue'
import type { Canvas, ActionResult } from '@/api/a2ui'
import { getCanvas } from '@/api/a2ui'
import { WebSocketClient, type WebSocketStatus } from '@/utils/websocket'

export interface A2UIWebSocketConfig {
  canvasId: string
  onCanvasUpdate?: (canvas: Canvas) => void
  onActionResult?: (result: ActionResult) => void
  onError?: (error: string) => void
}

export function useA2UIWebSocket(config: A2UIWebSocketConfig) {
  const canvas = ref<Canvas | null>(null)
  const status = ref<WebSocketStatus>('disconnected')
  const error = ref<string | null>(null)
  const loading = ref(false)

  let wsClient: WebSocketClient | null = null

  async function loadCanvas() {
    loading.value = true
    error.value = null

    try {
      canvas.value = await getCanvas(config.canvasId)
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to load canvas'
      config.onError?.(error.value)
    } finally {
      loading.value = false
    }
  }

  function connect() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${protocol}//${window.location.host}/api/v1/a2ui/ws/${config.canvasId}`

    wsClient = new WebSocketClient({
      url: wsUrl,
      reconnect: true,
      reconnectInterval: 3000,
      maxReconnectAttempts: 10,
      onStatusChange: (newStatus) => {
        status.value = newStatus
      },
      onError: () => {
        error.value = 'WebSocket connection error'
        config.onError?.(error.value)
      },
    })

    // Handle canvas updates
    wsClient.on<Canvas>('canvas_update', (updatedCanvas) => {
      canvas.value = updatedCanvas
      config.onCanvasUpdate?.(updatedCanvas)
    })

    // Handle component updates
    wsClient.on<{ components: Canvas['components'] }>('components_update', (data) => {
      if (canvas.value) {
        canvas.value = { ...canvas.value, components: data.components }
        config.onCanvasUpdate?.(canvas.value)
      }
    })

    // Handle action results
    wsClient.on<ActionResult>('action_result', (result) => {
      config.onActionResult?.(result)

      if (result.new_canvas) {
        canvas.value = result.new_canvas
        config.onCanvasUpdate?.(result.new_canvas)
      }
    })

    // Handle errors
    wsClient.on<{ message: string }>('error', (data) => {
      error.value = data.message
      config.onError?.(data.message)
    })

    wsClient.connect()
  }

  function disconnect() {
    wsClient?.disconnect()
    wsClient = null
  }

  function subscribeToCanvas(canvasId: string) {
    wsClient?.send('subscribe', { canvas_id: canvasId })
  }

  function unsubscribeFromCanvas(canvasId: string) {
    wsClient?.send('unsubscribe', { canvas_id: canvasId })
  }

  onMounted(async () => {
    await loadCanvas()
    connect()
    subscribeToCanvas(config.canvasId)
  })

  onUnmounted(() => {
    unsubscribeFromCanvas(config.canvasId)
    disconnect()
  })

  return {
    canvas: readonly(canvas),
    status: readonly(status),
    error: readonly(error),
    loading: readonly(loading),
    reload: loadCanvas,
    connect,
    disconnect,
  }
}
