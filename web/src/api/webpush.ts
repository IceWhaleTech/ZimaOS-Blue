import api from './client'

export async function getVapidKey(): Promise<string> {
  const { data } = await api.get<{ public_key: string }>('/webpush/vapid-key')
  return data.public_key
}

export async function subscribePush(subscription: PushSubscriptionJSON): Promise<void> {
  await api.post('/webpush/subscribe', subscription)
}

export async function unsubscribePush(endpoint: string): Promise<void> {
  await api.delete('/webpush/subscribe', { data: { endpoint } })
}
