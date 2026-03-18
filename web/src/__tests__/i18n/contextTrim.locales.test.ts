import { describe, expect, it } from 'vitest'

type ContextTrimLocaleMessages = {
  chat?: {
    contextCompacting?: string
    contextCompacted?: string
  }
}

const localeModules = import.meta.glob('@/i18n/locales/*.ts', { eager: true }) as Record<
  string,
  { default: ContextTrimLocaleMessages }
>

describe('context trim locale coverage', () => {
  it('exposes compaction status keys in every locale module', () => {
    const entries = Object.entries(localeModules)
    expect(entries).toHaveLength(27)

    for (const [path, mod] of entries) {
      const chat = mod.default?.chat
      expect(chat, `${path} missing chat section`).toBeTruthy()
      expect(chat?.contextCompacting, `${path} missing chat.contextCompacting`).toBeTruthy()
      expect(chat?.contextCompacted, `${path} missing chat.contextCompacted`).toBeTruthy()
    }
  })
})
