function hasCdpVersionShape(data) {
  return !!data && typeof data === 'object' && 'Browser' in data && 'Protocol-Version' in data
}

export function classifyRelayCheckResponse(res, port) {
  if (!res) {
    return { action: 'throw', error: 'No response from service worker' }
  }

  if (res.status === 401) {
    return { action: 'status', kind: 'error', message: 'Relay token rejected. Check the token and save again.' }
  }

  if (res.error) {
    return { action: 'throw', error: res.error }
  }

  if (!res.ok) {
    return { action: 'throw', error: `HTTP ${res.status}` }
  }

  const contentType = String(res.contentType || '')
  if (!contentType.includes('application/json')) {
    return {
      action: 'status',
      kind: 'error',
      message: 'Wrong port: expected Blue relay JSON response.',
    }
  }

  if (!hasCdpVersionShape(res.json)) {
    return {
      action: 'status',
      kind: 'error',
      message: 'Wrong port: expected Blue relay /json/version response.',
    }
  }

  return { action: 'status', kind: 'ok', message: `Blue relay reachable and authenticated at http://127.0.0.1:${port}/` }
}

export function classifyRelayCheckException(err, port) {
  const message = String(err || '').toLowerCase()
  if (message.includes('json') || message.includes('syntax')) {
    return {
      kind: 'error',
      message: 'Wrong port: this is not a Blue relay endpoint.',
    }
  }

  return {
    kind: 'error',
    message: `Blue relay not reachable/authenticated at http://127.0.0.1:${port}/.`,
  }
}
