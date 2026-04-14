<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { UserTaskActionDescriptor } from '@/api/tasks'
import type { TaskActionDialogPayload } from '@/composables/useTaskActionDialog'

const props = withDefaults(
  defineProps<{
    open: boolean
    action: Pick<UserTaskActionDescriptor, 'label' | 'input'> | null
    submitting?: boolean
    submitError?: string
  }>(),
  {
    submitting: false,
    submitError: '',
  }
)

const emit = defineEmits<{
  close: []
  confirm: [payload?: TaskActionDialogPayload]
}>()

const { t, te } = useI18n()

const fieldDrafts = ref<Record<string, string>>({})
const validationError = ref('')
const payloadPlaceholder = '{"ticket":"A-9"}'
const schemaInputRefs = new Map<string, HTMLInputElement | HTMLTextAreaElement>()
type ActionInput = NonNullable<UserTaskActionDescriptor['input']>
type ActionField = NonNullable<ActionInput['fields']>[number]
type ActionInputMode = 'decision_payload' | 'decision' | 'payload'

function legacyInputRecord(input: UserTaskActionDescriptor['input'] | null | undefined) {
  if (!input || typeof input !== 'object') return {}
  return input as Record<string, unknown>
}

function legacyInputString(
  input: UserTaskActionDescriptor['input'] | null | undefined,
  key: string
): string {
  return String(legacyInputRecord(input)[key] || '').trim()
}

function legacyInputBoolean(
  input: UserTaskActionDescriptor['input'] | null | undefined,
  key: string
): boolean {
  return Boolean(legacyInputRecord(input)[key])
}

function legacyInputStringArray(
  input: UserTaskActionDescriptor['input'] | null | undefined,
  key: string
): string[] {
  const value = legacyInputRecord(input)[key]
  return Array.isArray(value) ? value.map((item) => String(item || '').trim()).filter(Boolean) : []
}

function dialogTextWithFallback(key: string, fallback: string) {
  return te(key) ? t(key) : fallback
}

function resetFormState() {
  fieldDrafts.value = {}
  validationError.value = ''
}

function requestClose() {
  if (props.submitting) return
  emit('close')
}

function setSchemaInputRef(key: string, element: HTMLInputElement | HTMLTextAreaElement | null) {
  const normalizedKey = String(key || '').trim()
  if (!normalizedKey) return
  if (!element) {
    schemaInputRefs.delete(normalizedKey)
    return
  }
  schemaInputRefs.set(normalizedKey, element)
}

function focusSchemaField(key: string) {
  nextTick(() => {
    const element = schemaInputRefs.get(String(key || '').trim())
    element?.focus()
    if (typeof element?.select === 'function') {
      element.select()
    }
  })
}

function focusFirstSchemaField() {
  const firstField = normalizedActionFields.value[0]
  if (!firstField) return
  focusSchemaField(firstField.key)
}

function applyFieldOption(fieldKey: string, option: string) {
  setFieldDraft(fieldKey, option)
  const currentIndex = normalizedActionFields.value.findIndex((field) => field.key === fieldKey)
  const nextField = normalizedActionFields.value[currentIndex + 1]
  if (nextField) {
    focusSchemaField(nextField.key)
    return
  }
  focusSchemaField(fieldKey)
}

function setFieldDraft(fieldKey: string, value: string) {
  fieldDrafts.value = {
    ...fieldDrafts.value,
    [fieldKey]: value,
  }
}

function normalizeInputMode(value: unknown): ActionInputMode {
  const mode = String(value || '')
    .trim()
    .toLowerCase()
  switch (mode) {
    case 'decision':
      return 'decision'
    case 'payload':
      return 'payload'
    default:
      return 'decision_payload'
  }
}

function normalizeFieldKind(value: unknown): 'choice' | 'text' | 'textarea' | 'json' {
  const kind = String(value || '')
    .trim()
    .toLowerCase()
  switch (kind) {
    case 'choice':
      return 'choice'
    case 'text':
      return 'text'
    case 'json':
      return 'json'
    default:
      return 'textarea'
  }
}

function synthesizeLegacyFields(input: ActionInput | null | undefined): ActionField[] {
  const mode = normalizeInputMode(legacyInputString(input, 'mode'))
  const fields: ActionField[] = []
  const decisionOptions = legacyInputStringArray(input, 'decision_options')
  const payloadFormat = legacyInputString(input, 'payload_format').toLowerCase() || 'json_object'

  if (mode === 'decision' || mode === 'decision_payload') {
    fields.push({
      key: 'decision',
      label:
        legacyInputString(input, 'decision_label') ||
        dialogTextWithFallback('chat.taskActionDialog.decision', 'Decision'),
      kind: decisionOptions.length ? 'choice' : 'text',
      target: 'decision',
      required: legacyInputBoolean(input, 'decision_required'),
      placeholder:
        legacyInputString(input, 'decision_placeholder') ||
        dialogTextWithFallback('chat.taskActionDialog.decisionPlaceholder', 'approve'),
      options: decisionOptions.length ? decisionOptions : undefined,
    })
  }

  if (mode === 'payload' || mode === 'decision_payload') {
    const isTextPayload = payloadFormat === 'text'
    const payloadKey = legacyInputString(input, 'payload_text_key') || 'message'
    fields.push({
      key: isTextPayload ? payloadKey : 'payload',
      label:
        legacyInputString(input, 'payload_label') ||
        dialogTextWithFallback('chat.taskActionDialog.payload', 'Payload'),
      kind: isTextPayload ? 'textarea' : 'json',
      target: isTextPayload ? 'payload' : 'payload_root',
      payload_key: isTextPayload ? payloadKey : undefined,
      required: legacyInputBoolean(input, 'payload_required'),
      placeholder:
        legacyInputString(input, 'payload_placeholder') ||
        dialogTextWithFallback(
          'chat.taskActionDialog.payloadPlaceholder',
          isTextPayload ? 'Provide additional context' : payloadPlaceholder
        ),
    })
  }

  return fields
}

function parseJSONObject(rawPayload: string): Record<string, unknown> {
  let parsed: unknown
  try {
    parsed = JSON.parse(rawPayload)
  } catch {
    throw new Error(
      dialogTextWithFallback('chat.taskActionDialog.invalidJson', 'Payload must be valid JSON.')
    )
  }

  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error(
      dialogTextWithFallback(
        'chat.taskActionDialog.invalidObject',
        'Payload must be a JSON object.'
      )
    )
  }

  return parsed as Record<string, unknown>
}

function buildPayloadFromFields(): TaskActionDialogPayload | undefined {
  let decision = ''
  let payload: Record<string, unknown> | undefined
  let root: Record<string, unknown> | undefined

  for (const field of normalizedActionFields.value) {
    const rawValue = String(fieldDrafts.value[field.key] || '').trim()
    if (!rawValue) {
      if (field.required) {
        throw new Error(`${field.label} is required.`)
      }
      continue
    }

    if (String(field.target || '').trim() === 'decision') {
      decision = rawValue
      continue
    }

    if (String(field.target || '').trim() === 'root') {
      if (!root) root = {}
      if (normalizeFieldKind(field.kind) === 'json') {
        root = {
          ...root,
          ...parseJSONObject(rawValue),
        }
        continue
      }
      root[field.payload_key || field.key] = rawValue
      continue
    }

    if (!payload) payload = {}

    if (
      String(field.target || '').trim() === 'payload_root' ||
      normalizeFieldKind(field.kind) === 'json'
    ) {
      payload = {
        ...payload,
        ...parseJSONObject(rawValue),
      }
      continue
    }

    payload[field.payload_key || field.key] = rawValue
  }

  if (!decision && payload === undefined) {
    if (root !== undefined) {
      return root
    }
    return undefined
  }

  if (root !== undefined) {
    return {
      ...root,
      ...(decision ? { decision } : {}),
      ...(payload !== undefined ? { payload } : {}),
    }
  }

  return {
    ...(decision ? { decision } : {}),
    ...(payload !== undefined ? { payload } : {}),
  }
}

function fieldTestID(field: { key: string; target?: string }): string {
  if (String(field.target || '').trim() === 'decision') {
    return 'task-action-decision-input'
  }
  if (
    String(field.target || '').trim() === 'payload' ||
    String(field.target || '').trim() === 'payload_root'
  ) {
    return 'task-action-payload-input'
  }
  return `task-action-field-${field.key}`
}

function confirm() {
  if (props.submitting) return
  validationError.value = ''

  try {
    emit('confirm', buildPayloadFromFields())
  } catch (e) {
    validationError.value = e instanceof Error ? e.message.trim() : ''
  }
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    event.preventDefault()
    requestClose()
    return
  }

  if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) {
    event.preventDefault()
    confirm()
  }
}

const resolvedError = computed(() => {
  if (validationError.value.trim()) return validationError.value.trim()
  return String(props.submitError || '').trim()
})
const normalizedActionFields = computed(() => {
  const schemaFields = Array.isArray(props.action?.input?.fields)
    ? props.action.input.fields.filter((field) => String(field?.key || '').trim())
    : []
  if (schemaFields.length > 0) {
    return schemaFields
  }
  return synthesizeLegacyFields(props.action?.input)
})

const hasDecisionField = computed(() =>
  normalizedActionFields.value.some((field) => String(field.target || '').trim() === 'decision')
)
const payloadFields = computed(() =>
  normalizedActionFields.value.filter((field) => String(field.target || '').trim() !== 'decision')
)
const hasTextPayloadField = computed(() =>
  payloadFields.value.some((field) => normalizeFieldKind(field.kind) !== 'json')
)
const descriptionText = computed(() => {
  if (props.action?.input?.description) {
    return props.action.input.description
  }
  if (hasDecisionField.value && payloadFields.value.length === 0) {
    return dialogTextWithFallback(
      'chat.taskActionDialog.decisionOnlyDescription',
      'Provide the required decision for this action.'
    )
  }
  if (!hasDecisionField.value && payloadFields.value.length > 0) {
    return dialogTextWithFallback(
      'chat.taskActionDialog.payloadOnlyDescription',
      hasTextPayloadField.value
        ? 'Provide the required response for this action.'
        : 'Provide the required JSON payload for this action.'
    )
  }
  if (hasDecisionField.value && hasTextPayloadField.value) {
    return dialogTextWithFallback(
      'chat.taskActionDialog.decisionWithCommentDescription',
      'Provide the required decision and any optional supporting comment.'
    )
  }
  return dialogTextWithFallback(
    'chat.taskActionDialog.description',
    'Provide an optional decision and JSON payload for this action.'
  )
})
const resolvedTitle = computed(
  () =>
    props.action?.input?.title ||
    props.action?.label ||
    dialogTextWithFallback('chat.taskActionDialog.title', 'Task action')
)
const resolvedSubmitLabel = computed(
  () =>
    props.action?.input?.submit_label ||
    props.action?.label ||
    dialogTextWithFallback('common.confirm', 'Confirm')
)

watch(
  () => [props.open, props.action?.label, JSON.stringify(normalizedActionFields.value)] as const,
  ([open]) => {
    if (!open) return
    resetFormState()
    focusFirstSchemaField()
  },
  { immediate: true }
)

watch(
  () => JSON.stringify(fieldDrafts.value),
  () => {
    if (!validationError.value) return
    validationError.value = ''
  }
)
</script>

<template>
  <Teleport to="body">
    <Transition name="fade">
      <div
        v-if="open && action"
        data-testid="task-action-dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="task-action-dialog-title"
        class="fixed inset-0 z-[9999] flex items-center justify-center bg-black/40 backdrop-blur-sm"
        @click.self="requestClose"
        @keydown="handleKeydown"
      >
        <div
          class="w-full max-w-lg mx-4 rounded-2xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 shadow-2xl overflow-hidden"
        >
          <div class="p-6">
            <p
              class="mb-3 text-[11px] font-semibold uppercase tracking-[0.24em] text-gray-400 dark:text-gray-500"
            >
              {{ dialogTextWithFallback('chat.taskActionDialog.eyebrow', 'Task input') }}
            </p>
            <h3
              id="task-action-dialog-title"
              class="text-lg font-semibold text-gray-900 dark:text-white"
            >
              {{ resolvedTitle }}
            </h3>
            <p class="mt-2 text-sm leading-6 text-gray-500 dark:text-gray-400">
              {{ descriptionText }}
            </p>

            <div class="mt-5 space-y-4">
              <label
                v-for="field in normalizedActionFields"
                :key="field.key"
                class="block"
              >
                <span class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-200">
                  {{ field.label }}
                </span>

                <div
                  v-if="normalizeFieldKind(field.kind) === 'choice' && field.options?.length"
                  class="mb-2 flex flex-wrap gap-2"
                >
                  <button
                    v-for="option in field.options"
                    :key="`${field.key}-${option}`"
                    type="button"
                    class="rounded-full border border-slate-200 px-3 py-1 text-xs font-medium text-slate-700 transition hover:bg-slate-100 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800"
                    @click="applyFieldOption(field.key, option)"
                  >
                    {{ option }}
                  </button>
                </div>

                <input
                  v-if="
                    normalizeFieldKind(field.kind) === 'choice' ||
                      normalizeFieldKind(field.kind) === 'text'
                  "
                  :ref="(el) => setSchemaInputRef(field.key, el as HTMLInputElement | null)"
                  :value="fieldDrafts[field.key] || ''"
                  :data-testid="fieldTestID(field)"
                  type="text"
                  class="w-full rounded-xl border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 outline-none transition focus:border-blue-500 focus:ring-2 focus:ring-blue-200 dark:border-gray-600 dark:bg-gray-900 dark:text-white dark:focus:border-blue-400 dark:focus:ring-blue-500/30"
                  :placeholder="field.placeholder || ''"
                  @input="setFieldDraft(field.key, ($event.target as HTMLInputElement).value)"
                >

                <textarea
                  v-else
                  :ref="(el) => setSchemaInputRef(field.key, el as HTMLTextAreaElement | null)"
                  :value="fieldDrafts[field.key] || ''"
                  :data-testid="fieldTestID(field)"
                  :rows="normalizeFieldKind(field.kind) === 'json' ? 6 : 4"
                  class="w-full rounded-xl border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 outline-none transition focus:border-blue-500 focus:ring-2 focus:ring-blue-200 dark:border-gray-600 dark:bg-gray-900 dark:text-white dark:focus:border-blue-400 dark:focus:ring-blue-500/30"
                  :placeholder="field.placeholder || ''"
                  @input="setFieldDraft(field.key, ($event.target as HTMLTextAreaElement).value)"
                />
              </label>

              <p
                v-if="resolvedError"
                class="rounded-xl border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700 dark:border-rose-900/60 dark:bg-rose-950/30 dark:text-rose-200"
              >
                {{ resolvedError }}
              </p>
            </div>

            <div class="mt-6 flex gap-3">
              <button
                class="flex-1 px-4 py-2.5 text-sm font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors cursor-pointer disabled:cursor-not-allowed disabled:opacity-60"
                :disabled="submitting"
                @click="requestClose"
              >
                {{ dialogTextWithFallback('common.cancel', 'Cancel') }}
              </button>
              <button
                data-testid="task-action-confirm"
                class="flex-1 px-4 py-2.5 text-sm font-medium rounded-lg bg-blue-600 hover:bg-blue-700 text-white transition-colors cursor-pointer disabled:cursor-not-allowed disabled:opacity-60"
                :disabled="submitting"
                @click="confirm"
              >
                {{
                  submitting
                    ? dialogTextWithFallback('chat.taskActionDialog.submitting', 'Submitting...')
                    : resolvedSubmitLabel
                }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>
