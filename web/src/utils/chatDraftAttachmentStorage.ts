export interface DraftAttachmentPayload {
  id: string
  name: string
  type: string
  duration?: number
  lastModified?: number
  blob: Blob
}

interface DraftAttachmentStoreRecord {
  key: string
  attachments: DraftAttachmentPayload[]
  updatedAt: number
}

interface SerializedDraftAttachmentPayload {
  id: string
  name: string
  type: string
  duration?: number
  lastModified?: number
  data: string
}

interface SerializedDraftAttachmentStoreRecord {
  updatedAt: number
  attachments: SerializedDraftAttachmentPayload[]
}

const DB_NAME = 'zima-chat-drafts'
const DB_VERSION = 1
const STORE_NAME = 'chat-input-attachments'
const LOCAL_STORAGE_KEY_PREFIX = 'zima.chat.input_attachment_draft.v1'

let dbPromise: Promise<IDBDatabase | null> | null = null
const writeQueueByKey = new Map<string, Promise<void>>()

function hasIndexedDb(): boolean {
  return typeof indexedDB !== 'undefined'
}

function hasLocalStorage(): boolean {
  return typeof localStorage !== 'undefined'
}

function toLocalStorageKey(key: string): string {
  return `${LOCAL_STORAGE_KEY_PREFIX}:${key}`
}

async function waitForPendingWrites(key: string): Promise<void> {
  await (writeQueueByKey.get(key) ?? Promise.resolve()).catch(() => {})
}

function enqueueWrite(key: string, operation: () => Promise<void>): Promise<void> {
  const previous = writeQueueByKey.get(key) ?? Promise.resolve()
  const next = previous.catch(() => {}).then(operation)

  writeQueueByKey.set(key, next)

  return next.finally(() => {
    if (writeQueueByKey.get(key) === next) {
      writeQueueByKey.delete(key)
    }
  })
}

function requestToPromise<T>(request: IDBRequest<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(request.error ?? new Error('IndexedDB request failed'))
  })
}

function transactionToPromise(transaction: IDBTransaction): Promise<void> {
  return new Promise((resolve, reject) => {
    transaction.oncomplete = () => resolve()
    transaction.onerror = () =>
      reject(transaction.error ?? new Error('IndexedDB transaction failed'))
    transaction.onabort = () =>
      reject(transaction.error ?? new Error('IndexedDB transaction aborted'))
  })
}

async function openDatabase(): Promise<IDBDatabase | null> {
  if (!hasIndexedDb()) return null
  if (dbPromise) return dbPromise

  dbPromise = new Promise((resolve) => {
    try {
      const request = indexedDB.open(DB_NAME, DB_VERSION)

      request.onupgradeneeded = () => {
        const db = request.result
        if (!db.objectStoreNames.contains(STORE_NAME)) {
          db.createObjectStore(STORE_NAME, { keyPath: 'key' })
        }
      }

      request.onsuccess = () => resolve(request.result)
      request.onerror = () => resolve(null)
      request.onblocked = () => resolve(null)
    } catch {
      resolve(null)
    }
  })

  return dbPromise
}

async function blobToBase64(blob: Blob): Promise<string> {
  return await new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onloadend = () => {
      const dataUrl = typeof reader.result === 'string' ? reader.result : ''
      resolve(dataUrl.split(',')[1] || '')
    }
    reader.onerror = () => reject(reader.error ?? new Error('Failed to read blob'))
    reader.readAsDataURL(blob)
  })
}

function base64ToBlob(base64: string, contentType: string): Blob {
  const byteCharacters = atob(base64)
  const bytes = new Uint8Array(byteCharacters.length)
  for (let index = 0; index < byteCharacters.length; index += 1) {
    bytes[index] = byteCharacters.charCodeAt(index)
  }
  return new Blob([bytes], { type: contentType })
}

function normalizeDraftAttachments(value: unknown): DraftAttachmentPayload[] {
  if (!Array.isArray(value)) return []

  return value
    .map((item) => {
      if (!item || typeof item !== 'object') return null
      const candidate = item as Partial<DraftAttachmentPayload>
      if (!(candidate.blob instanceof Blob)) return null
      const name = typeof candidate.name === 'string' ? candidate.name.trim() : ''
      const type = typeof candidate.type === 'string' ? candidate.type.trim() : ''
      if (!name || !type) return null
      const normalized: DraftAttachmentPayload = {
        id:
          typeof candidate.id === 'string' && candidate.id.trim()
            ? candidate.id.trim()
            : Math.random().toString(36).slice(2),
        name,
        type,
        blob: candidate.blob,
      }
      if (typeof candidate.duration === 'number' && Number.isFinite(candidate.duration)) {
        normalized.duration = candidate.duration
      }
      if (
        typeof candidate.lastModified === 'number' &&
        Number.isFinite(candidate.lastModified)
      ) {
        normalized.lastModified = candidate.lastModified
      }
      return normalized
    })
    .filter((item): item is DraftAttachmentPayload => item !== null)
}

async function loadFromIndexedDb(
  db: IDBDatabase,
  key: string
): Promise<DraftAttachmentPayload[]> {
  try {
    const transaction = db.transaction(STORE_NAME, 'readonly')
    const request = transaction.objectStore(STORE_NAME).get(key) as IDBRequest<
      DraftAttachmentStoreRecord | undefined
    >
    const record = await requestToPromise(request)
    await transactionToPromise(transaction)
    return normalizeDraftAttachments(record?.attachments)
  } catch {
    return []
  }
}

async function saveToIndexedDb(
  db: IDBDatabase,
  key: string,
  attachments: DraftAttachmentPayload[]
): Promise<boolean> {
  try {
    const transaction = db.transaction(STORE_NAME, 'readwrite')
    transaction.objectStore(STORE_NAME).put({
      key,
      attachments,
      updatedAt: Date.now(),
    } satisfies DraftAttachmentStoreRecord)
    await transactionToPromise(transaction)
    return true
  } catch {
    return false
  }
}

async function clearFromIndexedDb(db: IDBDatabase, key: string): Promise<boolean> {
  try {
    const transaction = db.transaction(STORE_NAME, 'readwrite')
    transaction.objectStore(STORE_NAME).delete(key)
    await transactionToPromise(transaction)
    return true
  } catch {
    return false
  }
}

function loadFromLocalStorage(key: string): DraftAttachmentPayload[] {
  if (!hasLocalStorage()) return []

  try {
    const raw = localStorage.getItem(toLocalStorageKey(key))
    if (!raw) return []
    const parsed = JSON.parse(raw) as SerializedDraftAttachmentStoreRecord
    if (!parsed || !Array.isArray(parsed.attachments)) return []

    return parsed.attachments
      .map((item) => {
        if (!item || typeof item !== 'object') return null
        const name = typeof item.name === 'string' ? item.name.trim() : ''
        const type = typeof item.type === 'string' ? item.type.trim() : ''
        const data = typeof item.data === 'string' ? item.data : ''
        if (!name || !type || !data) return null
        const attachment: DraftAttachmentPayload = {
          id:
            typeof item.id === 'string' && item.id.trim()
              ? item.id.trim()
              : Math.random().toString(36).slice(2),
          name,
          type,
          blob: base64ToBlob(data, type),
        }
        if (typeof item.duration === 'number' && Number.isFinite(item.duration)) {
          attachment.duration = item.duration
        }
        if (typeof item.lastModified === 'number' && Number.isFinite(item.lastModified)) {
          attachment.lastModified = item.lastModified
        }
        return attachment
      })
      .filter((item): item is DraftAttachmentPayload => item !== null)
  } catch {
    return []
  }
}

async function saveToLocalStorage(key: string, attachments: DraftAttachmentPayload[]): Promise<void> {
  if (!hasLocalStorage()) return

  const serializedAttachments = await Promise.all(
    attachments.map(async (attachment) => ({
      id: attachment.id,
      name: attachment.name,
      type: attachment.type,
      duration: attachment.duration,
      lastModified: attachment.lastModified,
      data: await blobToBase64(attachment.blob),
    }))
  )

  try {
    localStorage.setItem(
      toLocalStorageKey(key),
      JSON.stringify({
        updatedAt: Date.now(),
        attachments: serializedAttachments,
      } satisfies SerializedDraftAttachmentStoreRecord)
    )
  } catch {
    // Ignore local storage quota / privacy mode failures.
  }
}

function clearFromLocalStorage(key: string): void {
  if (!hasLocalStorage()) return

  try {
    localStorage.removeItem(toLocalStorageKey(key))
  } catch {
    // Ignore storage failures.
  }
}

export async function loadDraftAttachments(key: string): Promise<DraftAttachmentPayload[]> {
  await waitForPendingWrites(key)

  const db = await openDatabase()
  if (db) {
    return await loadFromIndexedDb(db, key)
  }
  return loadFromLocalStorage(key)
}

export async function saveDraftAttachments(
  key: string,
  attachments: DraftAttachmentPayload[]
): Promise<void> {
  await enqueueWrite(key, async () => {
    if (attachments.length === 0) {
      const db = await openDatabase()
      if (db) {
        const clearedFromIndexedDb = await clearFromIndexedDb(db, key)
        if (clearedFromIndexedDb) {
          clearFromLocalStorage(key)
          return
        }
      }
      clearFromLocalStorage(key)
      return
    }

    const db = await openDatabase()
    if (db) {
      const savedToIndexedDb = await saveToIndexedDb(db, key, attachments)
      if (savedToIndexedDb) {
        clearFromLocalStorage(key)
        return
      }
    }
    await saveToLocalStorage(key, attachments)
  })
}

export async function clearDraftAttachments(key: string): Promise<void> {
  await enqueueWrite(key, async () => {
    const db = await openDatabase()
    if (db) {
      const clearedFromIndexedDb = await clearFromIndexedDb(db, key)
      if (clearedFromIndexedDb) {
        clearFromLocalStorage(key)
        return
      }
    }
    clearFromLocalStorage(key)
  })
}
