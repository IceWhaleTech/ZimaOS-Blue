import api from './client'

export async function getVapidKey(): Promise<string> {
  const { data } = await api.get<{ public_key: string }>('/v1/webpush/vapid-key')
  return data.public_key
}

export async function subscribePush(subscription: PushSubscriptionJSON): Promise<void> {
  await api.post('/v1/webpush/subscribe', subscription)
}

export async function unsubscribePush(endpoint: string): Promise<void> {
  await api.delete('/v1/webpush/subscribe', { data: { endpoint } })
}
