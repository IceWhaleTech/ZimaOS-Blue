import http from 'node:http'
import { URL } from 'node:url'

const port = Number(process.env.PLAYWRIGHT_API_PORT || '8787')
const heartbeatIntervalMs = 15_000

const conversation = {
  id: 'conv-1',
  title: 'Stop flow browser e2e',
  created_at: '2026-04-12T00:00:00.000Z',
  updated_at: '2026-04-12T00:00:02.000Z',
}

const bootstrapPayload = {
  command_state: {
    conversation_id: conversation.id,
    selected_provider_id: '',
    selected_model_id: '',
    offline: false,
  },
  active_stream: {
    conversation_id: conversation.id,
    active: true,
    stream_id: 'stream-live-stop',
  },
  current_tasks: [],
  background_tasks: [],
  pending_approval: null,
  pending_question: null,
  pending_exec_approval: null,
}

function writeJson(res, statusCode, payload) {
  res.writeHead(statusCode, {
    'Content-Type': 'application/json; charset=utf-8',
    'Cache-Control': 'no-store',
  })
  res.end(JSON.stringify(payload))
}

function writeSSEHeaders(res) {
  res.writeHead(200, {
    'Content-Type': 'text/event-stream; charset=utf-8',
    'Cache-Control': 'no-cache, no-transform',
    Connection: 'keep-alive',
  })
}

function handleEvents(req, res) {
  writeSSEHeaders(res)
  res.write(': connected\n\n')

  const timer = setInterval(() => {
    try {
      res.write(': heartbeat\n\n')
    } catch {
      clearInterval(timer)
    }
  }, heartbeatIntervalMs)

  req.on('close', () => {
    clearInterval(timer)
    res.end()
  })
}

function handleRequest(req, res) {
  const url = new URL(req.url || '/', `http://${req.headers.host || '127.0.0.1'}`)
  const { pathname } = url
  const method = req.method || 'GET'

  if (method === 'GET' && pathname === '/api/v1/system/mode') {
    return writeJson(res, 200, { mode: 'preview', features: {} })
  }

  if (method === 'POST' && pathname === '/api/v1/preview/token') {
    return writeJson(res, 200, { token: 'preview-token-playwright' })
  }

  if (method === 'GET' && pathname === '/api/v1/events') {
    return handleEvents(req, res)
  }

  if (method === 'GET' && pathname === '/api/v1/providers') {
    return writeJson(res, 200, { providers: [], total: 0 })
  }

  if (method === 'GET' && pathname === '/api/v1/providers/trial/quota') {
    return writeJson(res, 200, {
      tokens_used: 0,
      tokens_remaining: 1000,
      token_limit: 1000,
      is_exhausted: false,
    })
  }

  if (method === 'GET' && pathname === '/api/v1/config/routing-mode') {
    return writeJson(res, 200, { mode: 'auto' })
  }

  if (method === 'GET' && pathname === '/api/v1/settings') {
    return writeJson(res, 200, { locale: 'zh-CN', timezone: 'Asia/Shanghai' })
  }

  if (method === 'GET' && pathname === '/api/v1/conversations') {
    return writeJson(res, 200, [conversation])
  }

  if (method === 'GET' && pathname === `/api/v1/conversations/${conversation.id}`) {
    return writeJson(res, 200, conversation)
  }

  if (method === 'GET' && pathname === `/api/v1/conversations/${conversation.id}/messages`) {
    return writeJson(res, 200, [])
  }

  if (method === 'GET' && pathname === `/api/v1/conversations/${conversation.id}/bootstrap`) {
    return writeJson(res, 200, bootstrapPayload)
  }

  if (method === 'PATCH' && pathname === `/api/v1/conversations/${conversation.id}/command-state`) {
    return writeJson(res, 200, bootstrapPayload.command_state)
  }

  if (method === 'POST' && pathname === `/api/v1/conversations/${conversation.id}/messages/cancel`) {
    return writeJson(res, 200, {
      success: true,
      stream_id: 'stream-live-stop',
      message: 'Stream cancelled successfully',
    })
  }

  if (method === 'POST' && pathname === `/api/v1/conversations/${conversation.id}/warmup`) {
    res.writeHead(204)
    return res.end()
  }

  if (method === 'GET' && pathname === '/api/v1/ask-user-question/pending') {
    return writeJson(res, 200, { pending: false })
  }

  if (method === 'GET' && pathname === '/api/v1/exec/approvals/pending') {
    return writeJson(res, 200, { pending: false })
  }

  return writeJson(res, 200, {})
}

const server = http.createServer(handleRequest)

server.listen(port, '127.0.0.1', () => {
  process.stdout.write(`[playwright-chat-stop-mock] listening on http://127.0.0.1:${port}\n`)
})

function shutdown() {
  server.close(() => {
    process.exit(0)
  })
}

process.on('SIGINT', shutdown)
process.on('SIGTERM', shutdown)
