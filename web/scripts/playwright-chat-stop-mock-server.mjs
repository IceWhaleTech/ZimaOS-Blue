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

const dreamState = {
  enabled: true,
  archive_dir: '/tmp/playwright-dream-archive',
  pending_capsules: 2,
  archived_daily_logs: 1,
  promoted_count: 5,
  last_run_at: '2026-04-17T10:00:00.000Z',
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
    return writeJson(res, 200, {
      locale: 'zh-CN',
      timezone: 'Asia/Shanghai',
      memory_recall_mode: 'balanced',
    })
  }

  if (method === 'GET' && pathname === '/api/v1/companion/settings') {
    return writeJson(res, 200, {
      retention: {
        events_days: 30,
        sessions_days: 30,
        alerts_days: 30,
      },
      storage_info: {
        session_count: 0,
        alert_count: 0,
        event_count: 0,
      },
    })
  }

  if (method === 'GET' && pathname === '/api/v1/backup') {
    return writeJson(res, 200, [])
  }

  if (method === 'GET' && pathname === '/api/v1/memory/stats') {
    return writeJson(res, 200, {
      total_chunks: 1,
      total_size_bytes: 256,
      total_display_count: 1,
      total_display_size_bytes: 256,
      daily_logs_count: 0,
      daily_entries_count: 0,
      daily_total_size_bytes: 0,
      backend: 'markdown',
      oldest_chunk: '2026-04-17T09:00:00.000Z',
      newest_chunk: '2026-04-17T09:30:00.000Z',
    })
  }

  if (method === 'GET' && pathname === '/api/v1/memory/dream/status') {
    return writeJson(res, 200, dreamState)
  }

  if (method === 'POST' && pathname === '/api/v1/memory/dream/run') {
    dreamState.pending_capsules = 0
    dreamState.archived_daily_logs = 2
    dreamState.promoted_count = 7
    dreamState.last_run_at = '2026-04-17T10:05:00.000Z'

    return writeJson(res, 200, {
      run_id: 'dream-run-playwright',
      promoted_count: 2,
      archived_daily_count: 1,
      processed_capsules: 2,
    })
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
