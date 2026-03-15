export type LocaleMessages = Record<string, unknown>

function isPlainObject(value: unknown): value is LocaleMessages {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

function cloneMessageValue<T>(value: T): T {
  if (Array.isArray(value)) {
    return value.map((item) => cloneMessageValue(item)) as T
  }

  if (isPlainObject(value)) {
    return Object.fromEntries(
      Object.entries(value).map(([key, nestedValue]) => [key, cloneMessageValue(nestedValue)])
    ) as T
  }

  return value
}

export function deepMergeMessages<T extends LocaleMessages>(base: T, overrides: LocaleMessages): T {
  const merged = cloneMessageValue(base) as LocaleMessages

  for (const [key, overrideValue] of Object.entries(overrides)) {
    const baseValue = merged[key]
    if (isPlainObject(baseValue) && isPlainObject(overrideValue)) {
      merged[key] = deepMergeMessages(baseValue, overrideValue)
      continue
    }

    merged[key] = cloneMessageValue(overrideValue)
  }

  return merged as T
}
