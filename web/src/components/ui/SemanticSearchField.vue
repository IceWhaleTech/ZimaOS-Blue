<script setup lang="ts">
import { computed, ref } from 'vue'

const props = withDefaults(
  defineProps<{
    modelValue: string
    placeholder?: string
    clearLabel?: string
    disabled?: boolean
    testId?: string
    enterKeyHint?: 'search' | 'done' | 'go' | 'send' | 'enter' | 'next' | 'previous'
  }>(),
  {
    placeholder: '',
    clearLabel: 'Clear',
    disabled: false,
    testId: undefined,
    enterKeyHint: 'search',
  }
)

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
  (e: 'clear'): void
  (e: 'submit-shortcut'): void
}>()

const inputRef = ref<HTMLInputElement | null>(null)
const isFocused = ref(false)

const hasContent = computed(() => props.modelValue.trim().length > 0)

function handleInput(event: Event) {
  emit('update:modelValue', (event.target as HTMLInputElement).value.replace(/\s+/g, ' '))
}

function handleFocus() {
  isFocused.value = true
}

function handleBlur() {
  isFocused.value = false
}

function clearField() {
  emit('update:modelValue', '')
  emit('clear')

  requestAnimationFrame(() => {
    inputRef.value?.focus()
  })
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter') {
    event.preventDefault()
    emit('submit-shortcut')
    return
  }

  if (event.key === 'Escape' && hasContent.value) {
    event.preventDefault()
    clearField()
  }
}
</script>

<template>
  <div
    class="semantic-search-field"
    :class="{
      'is-focused': isFocused,
      'is-disabled': disabled,
    }"
  >
    <div class="semantic-search-shell">
      <svg
        class="semantic-search-icon"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="1.85"
        aria-hidden="true"
      >
        <circle
          cx="11"
          cy="11"
          r="7.5"
        />
        <path d="m20 20-3.5-3.5" />
      </svg>

      <input
        ref="inputRef"
        :data-testid="testId"
        class="semantic-search-input"
        type="text"
        :value="modelValue"
        :placeholder="placeholder"
        :disabled="disabled"
        :enterkeyhint="enterKeyHint"
        spellcheck="false"
        @input="handleInput"
        @focus="handleFocus"
        @blur="handleBlur"
        @keydown="handleKeydown"
      >

      <button
        v-if="hasContent"
        type="button"
        class="semantic-search-clear"
        :title="clearLabel"
        :aria-label="clearLabel"
        @mousedown.prevent
        @click="clearField"
      >
        <svg
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
        >
          <path d="M6 6l12 12M18 6 6 18" />
        </svg>
      </button>
    </div>
  </div>
</template>

<style scoped>
.semantic-search-field {
  --semantic-shell-border: var(--glass-border, rgba(148, 163, 184, 0.28));
  --semantic-shell-bg: var(--glass-bg, rgba(255, 255, 255, 0.94));
  --semantic-shell-bg-focus: var(--glass-bg-hover, rgba(255, 255, 255, 0.98));
  --semantic-icon-color: var(--color-text-secondary, #475569);
  --semantic-text-color: var(--color-text-primary, #0f172a);
  --semantic-placeholder-color: var(--color-text-muted, #64748b);
  --semantic-clear-border: rgba(148, 163, 184, 0.22);
  --semantic-clear-bg: rgba(148, 163, 184, 0.08);
  --semantic-clear-bg-hover: rgba(148, 163, 184, 0.14);
  position: relative;
  display: block;
  width: 100%;
  border-radius: 24px;
  transition:
    transform 0.22s ease,
    opacity 0.22s ease;
}

.semantic-search-shell {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 52px;
  padding: 12px 14px 12px 16px;
  border-radius: 22px;
  border: 1px solid var(--semantic-shell-border);
  background: var(--semantic-shell-bg);
  backdrop-filter: blur(18px);
  -webkit-backdrop-filter: blur(18px);
  transition:
    min-height 0.22s ease,
    background-color 0.22s ease,
    border-color 0.22s ease;
  overflow: hidden;
}

.semantic-search-shell::before {
  content: '';
  position: absolute;
  inset: 0;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.06), rgba(255, 255, 255, 0) 32%),
    linear-gradient(120deg, rgba(255, 255, 255, 0.02), rgba(255, 255, 255, 0) 52%);
  opacity: 0.26;
  pointer-events: none;
  transition: opacity 0.35s ease;
}

.semantic-search-shell::after {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: inherit;
  padding: 1px;
  background: linear-gradient(
    100deg,
    rgba(34, 197, 94, 0) 0%,
    rgba(34, 197, 94, 0) 34%,
    rgba(56, 189, 248, 0.2) 45%,
    rgba(99, 102, 241, 0.34) 50%,
    rgba(168, 85, 247, 0.28) 55%,
    rgba(168, 85, 247, 0) 66%,
    rgba(148, 163, 184, 0) 100%
  );
  background-size: 220% 100%;
  background-position: 0% 50%;
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.28s ease;
  -webkit-mask:
    linear-gradient(#fff 0 0) content-box,
    linear-gradient(#fff 0 0);
  -webkit-mask-composite: xor;
  mask:
    linear-gradient(#fff 0 0) content-box,
    linear-gradient(#fff 0 0);
  mask-composite: exclude;
}

.semantic-search-icon {
  position: relative;
  z-index: 1;
  width: 18px;
  height: 18px;
  flex-shrink: 0;
  color: var(--semantic-icon-color);
  transition:
    color 0.22s ease,
    transform 0.22s ease;
}

.semantic-search-input {
  position: relative;
  z-index: 1;
  flex: 1;
  width: 100%;
  min-width: 0;
  border: 0;
  padding: 0;
  margin: 0;
  height: 28px;
  background: transparent;
  color: var(--semantic-text-color);
  font: inherit;
  font-size: 0.95rem;
  line-height: 1.2;
  letter-spacing: 0.01em;
  outline: none;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.semantic-search-input::placeholder {
  color: var(--semantic-placeholder-color);
}

.semantic-search-clear {
  position: relative;
  z-index: 1;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  flex-shrink: 0;
  border: 1px solid var(--semantic-clear-border);
  border-radius: 999px;
  background: var(--semantic-clear-bg);
  color: var(--semantic-icon-color);
  cursor: pointer;
  transition:
    background-color 0.18s ease,
    color 0.18s ease,
    transform 0.18s ease,
    border-color 0.18s ease;
}

.semantic-search-clear:hover {
  background: var(--semantic-clear-bg-hover);
  border-color: rgba(129, 140, 248, 0.26);
  color: var(--semantic-text-color);
  transform: scale(1.03);
}

.semantic-search-clear svg {
  width: 14px;
  height: 14px;
}

.semantic-search-field.is-focused {
  transform: none;
}

.semantic-search-field.is-focused .semantic-search-shell {
  border-color: rgba(129, 140, 248, 0.16);
  background: var(--semantic-shell-bg-focus);
}

.semantic-search-field.is-focused .semantic-search-shell::before {
  opacity: 0.36;
}

.semantic-search-field.is-focused .semantic-search-shell::after {
  opacity: 1;
  animation: semantic-border-sweep 6.2s linear infinite;
}

.semantic-search-field.is-focused .semantic-search-icon {
  color: rgba(99, 102, 241, 0.82);
  transform: none;
}

.semantic-search-field.is-disabled {
  opacity: 0.66;
  filter: saturate(0.8);
}

@keyframes semantic-border-sweep {
  0% {
    background-position: 0% 50%;
  }
  100% {
    background-position: 220% 50%;
  }
}

@media (max-width: 640px) {
  .semantic-search-shell {
    min-height: 50px;
    padding: 11px 11px 11px 13px;
  }
}
</style>
