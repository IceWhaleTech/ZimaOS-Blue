import { ref, reactive, computed } from 'vue'
import { templateApi, configApi, type FillTemplate, type FormFillerConfig } from '@/api/formfiller'
import { parseClipboardData, parseClipboardFields } from '@/utils/clipboardParser'
import { getStoredAccessToken } from '@/utils/authStorage'

type FillableField = HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement

export interface FillHistoryEntry {
  timestamp: number
  fields: Array<{
    element: FillableField
    oldValue: string
    newValue: string
  }>
}

export interface WidgetPosition {
  x: number
  y: number
}

export interface WidgetState {
  isVisible: boolean
  isMinimized: boolean
  position: WidgetPosition
  focusedElement: FillableField | null
  selectedTemplate: FillTemplate | null
  templates: FillTemplate[]
  config: FormFillerConfig | null
  fillHistory: FillHistoryEntry[]
  isLoading: boolean
  error: string | null
  isInitialized: boolean
  clipboardData: string
  parsedClipboardFields: Record<string, string>
  parsedClipboardTokens: string[]
  revealedPasswordFields: Set<HTMLInputElement>
}

// Singleton state to share across components
const globalState = reactive<WidgetState>({
  isVisible: false,
  isMinimized: false,
  position: { x: 0, y: 0 },
  focusedElement: null,
  selectedTemplate: null,
  templates: [],
  config: null,
  fillHistory: [],
  isLoading: false,
  error: null,
  isInitialized: false,
  clipboardData: '',
  parsedClipboardFields: {},
  parsedClipboardTokens: [],
  revealedPasswordFields: new Set(),
})

const currentDomain = ref<string>('')
let keyboardListenerAdded = false
let focusListenerAdded = false

// Track if paste area is expanded (shared state for blur handling)
let pasteAreaExpanded = false

export function setPasteAreaExpanded(expanded: boolean) {
  pasteAreaExpanded = expanded
}

const FORM_FILLER_SCOPE_ATTRIBUTE = 'data-form-filler-scope'
const FORM_FILLER_SCOPE_SELECTOR = `[${FORM_FILLER_SCOPE_ATTRIBUTE}]`
const allowedFormFillerScopes = new Set(['channel', 'provider'])

// Routes where the form filler widget should be disabled
const disabledRoutes = ['/chat']

// Check if current route is disabled
function isRouteDisabled(): boolean {
  const path = window.location.pathname
  return disabledRoutes.some((route) => path.startsWith(route))
}

function shouldIgnoreField(element: Element): boolean {
  if (element.closest('.formfiller-widget')) return true
  return Boolean(element.closest('[data-form-filler-ignore]'))
}

function getAllowedScopeContainer(element: Element | null): HTMLElement | null {
  if (!element) return null

  const scopeContainer = element.closest<HTMLElement>(FORM_FILLER_SCOPE_SELECTOR)
  if (!scopeContainer) return null

  const scopeName = scopeContainer.getAttribute(FORM_FILLER_SCOPE_ATTRIBUTE)
  if (!scopeName || !allowedFormFillerScopes.has(scopeName)) {
    return null
  }

  return scopeContainer
}

function isElementInAllowedScope(element: Element | null): boolean {
  return !!getAllowedScopeContainer(element)
}

export function useFormFillerWidget() {
  // Computed
  const canUndo = computed(() => globalState.fillHistory.length > 0)
  const hasClipboardData = computed(() => globalState.clipboardData.trim().length > 0)
  const parsedFieldCount = computed(
    () =>
      Object.keys(globalState.parsedClipboardFields).length ||
      globalState.parsedClipboardTokens.length
  )

  // Load configuration and templates
  async function initialize() {
    if (globalState.isInitialized) return

    // Skip initialization if user is not logged in
    const token = getStoredAccessToken()
    if (!token) {
      return
    }

    globalState.isLoading = true
    globalState.error = null
    try {
      const [configRes, templatesRes] = await Promise.all([configApi.get(), templateApi.list()])
      globalState.config = configRes.data
      globalState.templates = templatesRes.data ?? []

      // Select default template
      const defaultTemplate = globalState.templates.find((t) => t.is_default)
      if (defaultTemplate) {
        globalState.selectedTemplate = defaultTemplate
      } else if (globalState.templates.length > 0) {
        globalState.selectedTemplate = globalState.templates[0] ?? null
      }

      globalState.isInitialized = true
    } catch (e) {
      globalState.error = 'Failed to initialize form filler'
      console.error(e)
    } finally {
      globalState.isLoading = false
    }
  }

  // Check if element is a fillable form field
  function isFillableField(
    el: Element
  ): el is FillableField {
    if (shouldIgnoreField(el)) return false

    if (
      (el instanceof HTMLInputElement || el instanceof HTMLSelectElement) &&
      el.disabled
    ) {
      return false
    }

    if (
      (el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement) &&
      (el.disabled || el.readOnly)
    ) {
      return false
    }

    const tagName = el.tagName.toLowerCase()
    if (tagName === 'select' || tagName === 'textarea') return true
    if (tagName === 'input') {
      const type = (el as HTMLInputElement).type?.toLowerCase()
      // Exclude non-fillable input types
      const excludedTypes = [
        'hidden',
        'submit',
        'button',
        'file',
        'image',
        'reset',
        'checkbox',
        'radio',
      ]
      return !excludedTypes.includes(type)
    }
    return false
  }

  // Check if element should count toward form context (excludes select/dropdown)
  function isTextInputField(el: Element): boolean {
    const tagName = el.tagName.toLowerCase()
    if (tagName === 'textarea') return true
    if (tagName === 'input') {
      const type = (el as HTMLInputElement).type?.toLowerCase()
      const excludedTypes = [
        'hidden',
        'submit',
        'button',
        'file',
        'image',
        'reset',
        'checkbox',
        'radio',
      ]
      return !excludedTypes.includes(type)
    }
    return false
  }

  // Calculate widget position near the focused element (to the left)
  function calculatePosition(element: HTMLElement): WidgetPosition {
    const rect = element.getBoundingClientRect()
    const widgetWidth = 320
    const widgetHeight = 200
    const padding = 8

    // Position to the left of the input
    let x = rect.left - widgetWidth - padding
    let y = rect.top

    // If widget would go off-screen to the left, show to the right instead
    if (x < padding) {
      x = rect.right + padding
      // If still off-screen (no space on either side), position at left edge
      if (x + widgetWidth > window.innerWidth) {
        x = padding
      }
    }

    // Adjust if widget would go off-screen at the bottom
    if (y + widgetHeight > window.innerHeight) {
      y = window.innerHeight - widgetHeight - padding
    }

    // Ensure y is not negative
    if (y < padding) {
      y = padding
    }

    return { x, y }
  }

  // Find the nearest form container (form, div, section, etc.) that contains the element
  function findFormContainer(element: Element): Element | null {
    // First try to find a form element
    const form = element.closest('form')
    if (form) return form

    // Otherwise, find the nearest container with multiple text inputs (not select/dropdown)
    let current: Element | null = element.parentElement
    while (current) {
      // Check common container elements
      if (['div', 'section', 'article', 'fieldset', 'p'].includes(current.tagName.toLowerCase())) {
        const inputs = current.querySelectorAll('input, textarea')
        const textInputs = Array.from(inputs).filter((el) => isTextInputField(el))
        if (textInputs.length >= 2) {
          return current
        }
      }
      current = current.parentElement
    }
    return null
  }

  function getFillContextRoot(element: Element): Element | null {
    const scopeContainer = getAllowedScopeContainer(element)
    if (!scopeContainer) return null

    const formContainer = findFormContainer(element)
    if (formContainer && scopeContainer.contains(formContainer)) {
      return formContainer
    }

    return scopeContainer
  }

  function getScopedFillableFields(element: Element): FillableField[] {
    const root = getFillContextRoot(element)
    if (!root) return []

    const inputs = root.querySelectorAll('input, select, textarea')
    return Array.from(inputs).filter((field): field is FillableField => isFillableField(field))
  }

  // Check if the focused element is in a form-like context (2+ text input fields, not counting select)
  function isInFormContext(element: Element): boolean {
    const scopeContainer = getAllowedScopeContainer(element)
    if (!scopeContainer) return false

    const container = findFormContainer(element)
    if (!container || !scopeContainer.contains(container)) return false

    const inputs = container.querySelectorAll('input, textarea')
    const textInputs = Array.from(inputs).filter((el) => isTextInputField(el))
    return textInputs.length >= 2
  }

  function showWidgetForElement(element: FillableField): boolean {
    if (!isElementInAllowedScope(element)) return false

    // Select elements can still be filled via "Fill All" but shouldn't trigger the widget
    if (element.tagName.toLowerCase() === 'select') return false

    if (!isInFormContext(element)) return false

    globalState.focusedElement = element
    globalState.position = calculatePosition(element)
    globalState.isVisible = true
    return true
  }

  // Handle focus on form fields
  function handleFocus(event: FocusEvent) {
    const target = event.target as Element
    if (!target || !isFillableField(target)) return

    // Don't show widget on disabled routes (e.g., chat page)
    if (isRouteDisabled()) return

    showWidgetForElement(target)
  }

  // Handle blur - hide widget after a delay (to allow clicking on widget)
  function handleBlur(_event: FocusEvent) {
    // Use setTimeout to allow click events on widget to fire first
    setTimeout(() => {
      const activeElement = document.activeElement
      // Don't hide if focus moved to another form field or to the widget
      if (
        activeElement &&
        (
          (isFillableField(activeElement) && isElementInAllowedScope(activeElement)) ||
          activeElement.closest('.formfiller-widget')
        )
      ) {
        return
      }
      // Don't hide if there's clipboard data being edited
      if (globalState.clipboardData.trim()) {
        return
      }
      // Don't hide if paste area is expanded (user is interacting with it)
      // Check both the DOM element and the tracked state
      const pasteArea = document.querySelector('.formfiller-widget textarea')
      if (pasteArea || pasteAreaExpanded) {
        return
      }
      // Don't hide if there's an error being displayed
      if (globalState.error) {
        return
      }
      globalState.isVisible = false
      globalState.focusedElement = null
    }, 200)
  }

  // Update clipboard data and parse it
  function setClipboardData(data: string) {
    globalState.clipboardData = data
    globalState.parsedClipboardFields = parseClipboardData(data)
    globalState.parsedClipboardTokens = parseClipboardFields(data)
  }

  // Read from system clipboard
  // Returns true if successful, false if failed (caller should show paste area)
  async function readFromClipboard(): Promise<boolean> {
    try {
      const text = await navigator.clipboard.readText()
      setClipboardData(text)
      return true
    } catch (e) {
      console.error('Failed to read clipboard:', e)
      // Don't show error - caller will expand paste area for manual input
      return false
    }
  }

  // Clear clipboard data
  function clearClipboardData() {
    globalState.clipboardData = ''
    globalState.parsedClipboardFields = {}
    globalState.parsedClipboardTokens = []
  }

  // Reveal password field (change type to text)
  function revealPassword(element: HTMLInputElement) {
    if (element.type === 'password') {
      element.type = 'text'
      globalState.revealedPasswordFields.add(element)
    }
  }

  // Hide password field (change type back to password)
  function hidePassword(element: HTMLInputElement) {
    if (globalState.revealedPasswordFields.has(element)) {
      element.type = 'password'
      globalState.revealedPasswordFields.delete(element)
    }
  }

  // Toggle password visibility
  function togglePasswordVisibility(element: HTMLInputElement) {
    if (globalState.revealedPasswordFields.has(element)) {
      hidePassword(element)
    } else {
      revealPassword(element)
    }
  }

  // Check if a password field is revealed
  function isPasswordRevealed(element: HTMLInputElement): boolean {
    return globalState.revealedPasswordFields.has(element)
  }

  // Extract meaningful parts from a key (split by _ or -)
  function extractKeyParts(key: string): string[] {
    return key
      .toLowerCase()
      .split(/[_\-\s]+/)
      .filter((p) => p.length > 0)
  }

  // Calculate match score between a clipboard key and field identifiers
  // More strict matching: the last part of the key should match the last part of the identifier
  function calculateMatchScore(clipboardKey: string, fieldIdentifiers: string[]): number {
    const keyParts = extractKeyParts(clipboardKey)
    if (keyParts.length === 0) return 0

    // The last part is the most important (e.g., "id" in "app_id", "secret" in "app_secret")
    const keyLastPart = keyParts[keyParts.length - 1] || ''
    // Second to last part is also important for context (e.g., "app" in "app_id")
    const keySecondLastPart = keyParts.length > 1 ? keyParts[keyParts.length - 2] : null

    let bestScore = 0

    for (const identifier of fieldIdentifiers) {
      if (!identifier) continue

      const idParts = extractKeyParts(identifier)
      if (idParts.length === 0) continue

      const idLastPart = idParts[idParts.length - 1] || ''

      // Exact full match (highest priority)
      if (keyParts.join('') === idParts.join('')) {
        return 100
      }

      // Last part exact match (e.g., "app_id" matches field "id" or "user_id")
      if (keyLastPart === idLastPart) {
        let score = 80

        // Bonus if second-to-last parts also match (e.g., "app_id" matches "app_id" better than "user_id")
        if (keySecondLastPart && idParts.length > 1) {
          const idSecondLastPart = idParts[idParts.length - 2]
          if (keySecondLastPart === idSecondLastPart) {
            score += 15
          }
        }

        bestScore = Math.max(bestScore, score)
        continue
      }

      // Check if key's last part is contained in identifier's last part or vice versa
      // e.g., "webhook_url" matches "url" or "webhook"
      if (keyLastPart.includes(idLastPart) || idLastPart.includes(keyLastPart)) {
        const matchLength = Math.min(keyLastPart.length, idLastPart.length)
        if (matchLength >= 2) {
          bestScore = Math.max(bestScore, 50 + matchLength * 3)
          continue
        }
      }

      // Check if any key part matches any identifier part exactly
      for (const kp of keyParts) {
        for (const ip of idParts) {
          if (kp === ip && kp.length >= 3) {
            bestScore = Math.max(bestScore, 40 + kp.length * 2)
          }
        }
      }
    }

    return bestScore
  }

  // Find the best matching clipboard key for a field
  function findBestMatchForField(
    element: HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement,
    usedKeys: Set<string>
  ): { key: string; value: string } | null {
    const name = element.name || ''
    const id = element.id || ''
    const placeholder = 'placeholder' in element ? element.placeholder || '' : ''
    const autocomplete = element.autocomplete || ''

    // Get label text
    let labelText = ''
    if (element.id) {
      const label = document.querySelector(`label[for="${element.id}"]`)
      if (label) {
        labelText = label.textContent?.trim() || ''
      }
    }
    if (!labelText) {
      const parentLabel = element.closest('label')
      if (parentLabel) {
        // Get only direct text, not nested input values
        const clone = parentLabel.cloneNode(true) as HTMLElement
        clone.querySelectorAll('input, select, textarea').forEach((el) => el.remove())
        labelText = clone.textContent?.trim() || ''
      }
    }

    const fieldIdentifiers = [name, id, placeholder, autocomplete, labelText].filter(Boolean)

    if (fieldIdentifiers.length === 0) {
      return null
    }

    const clipboardFields = globalState.parsedClipboardFields
    let bestMatch: { key: string; value: string; score: number } | null = null

    for (const [key, value] of Object.entries(clipboardFields)) {
      // Skip already used keys (case-insensitive comparison)
      if (usedKeys.has(key.toLowerCase())) continue

      const score = calculateMatchScore(key, fieldIdentifiers)

      if (score > 0 && (!bestMatch || score > bestMatch.score)) {
        bestMatch = { key, value, score }
      }
    }

    return bestMatch ? { key: bestMatch.key, value: bestMatch.value } : null
  }

  // Find matching value for a field from parsed clipboard data or template
  function findValueForField(
    element: HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement,
    usedKeys?: Set<string>
  ): string | null {
    // First, try to match from parsed clipboard data
    if (Object.keys(globalState.parsedClipboardFields).length > 0) {
      const match = findBestMatchForField(element, usedKeys || new Set())
      if (match) {
        // Add lowercase key to usedKeys for case-insensitive tracking
        usedKeys?.add(match.key.toLowerCase())
        return match.value
      }
    }

    // Fall back to template matching
    const name = element.name?.toLowerCase() || ''
    const id = element.id?.toLowerCase() || ''
    const placeholder = 'placeholder' in element ? element.placeholder?.toLowerCase() || '' : ''
    const type = element.type?.toLowerCase() || ''
    const autocomplete = element.autocomplete?.toLowerCase() || ''

    // Get label text
    let labelText = ''
    if (element.id) {
      const label = document.querySelector(`label[for="${element.id}"]`)
      if (label) {
        labelText = label.textContent?.trim().toLowerCase() || ''
      }
    }
    if (!labelText) {
      const parentLabel = element.closest('label')
      if (parentLabel) {
        labelText = parentLabel.textContent?.trim().toLowerCase() || ''
      }
    }

    // Common field name mappings
    const fieldMappings: Record<string, string[]> = {
      email: ['email', 'mail', 'e-mail', '邮箱', '电子邮件', 'メール'],
      name: ['name', 'fullname', 'full_name', 'full-name', '姓名', '名前', 'username', 'user_name'],
      firstName: ['firstname', 'first_name', 'first-name', 'fname', '名', '名字'],
      lastName: ['lastname', 'last_name', 'last-name', 'lname', '姓', '姓氏'],
      phone: ['phone', 'tel', 'telephone', 'mobile', 'cell', '电话', '手机', '電話'],
      address: ['address', 'addr', 'street', '地址', '住所'],
      city: ['city', '城市', '市'],
      state: ['state', 'province', '省', '州'],
      zip: ['zip', 'zipcode', 'postal', 'postcode', '邮编', '郵便番号'],
      country: ['country', '国家', '国'],
      company: ['company', 'organization', 'org', '公司', '组织', '会社'],
      password: ['password', 'pass', 'pwd', '密码', 'パスワード'],
    }

    // Try to match field type and get value from template
    if (globalState.selectedTemplate) {
      for (const [fieldType, patterns] of Object.entries(fieldMappings)) {
        const allPatterns = [...patterns, fieldType]
        for (const pattern of allPatterns) {
          if (
            name.includes(pattern) ||
            pattern.includes(name) ||
            id.includes(pattern) ||
            pattern.includes(id) ||
            placeholder.includes(pattern) ||
            labelText.includes(pattern) ||
            autocomplete.includes(pattern) ||
            type === pattern
          ) {
            const templateValue = globalState.selectedTemplate.fields[fieldType]
            if (templateValue) {
              return templateValue
            }
          }
        }
      }
    }

    return null
  }

  // Fill the currently focused field
  function fillCurrentField() {
    if (!globalState.focusedElement) return

    let value: string | null = null

    // Positional mode: use the field's DOM index to pick the corresponding line
    if (globalState.parsedClipboardTokens.length > 0) {
      const fillableFields = getScopedFillableFields(globalState.focusedElement)
      const idx = fillableFields.indexOf(globalState.focusedElement)
      if (idx >= 0 && idx < globalState.parsedClipboardTokens.length) {
        value = globalState.parsedClipboardTokens[idx]!
      }
    }

    // Fall back to key-value matching
    if (!value) {
      value = findValueForField(globalState.focusedElement)
    }

    if (!value) {
      globalState.error = 'No matching value found for this field'
      setTimeout(() => {
        globalState.error = null
      }, 2000)
      return
    }

    const oldValue = globalState.focusedElement.value
    globalState.focusedElement.value = value

    // Trigger events
    globalState.focusedElement.dispatchEvent(new Event('input', { bubbles: true }))
    globalState.focusedElement.dispatchEvent(new Event('change', { bubbles: true }))

    // Auto-reveal password fields after filling
    if (
      globalState.focusedElement instanceof HTMLInputElement &&
      globalState.focusedElement.type === 'password'
    ) {
      revealPassword(globalState.focusedElement)
    }

    // Save to history
    globalState.fillHistory.push({
      timestamp: Date.now(),
      fields: [
        {
          element: globalState.focusedElement,
          oldValue,
          newValue: value,
        },
      ],
    })

    // Keep only last 20 history entries
    if (globalState.fillHistory.length > 20) {
      globalState.fillHistory.shift()
    }
  }

  // Fill all form fields on the page
  function fillAllFields() {
    if (!globalState.focusedElement) return 0

    const fillableFields = getScopedFillableFields(globalState.focusedElement)
    if (fillableFields.length === 0) return 0

    const historyEntry: FillHistoryEntry = {
      timestamp: Date.now(),
      fields: [],
    }

    // Positional mode: plain lines without keys → fill fields in DOM order
    if (globalState.parsedClipboardTokens.length > 0) {
      const lines = globalState.parsedClipboardTokens
      const count = Math.min(lines.length, fillableFields.length)
      for (let i = 0; i < count; i++) {
        const element = fillableFields[i]!
        const value = lines[i]!
        const wasPasswordField = element instanceof HTMLInputElement && element.type === 'password'
        const oldValue = element.value
        element.value = value
        element.dispatchEvent(new Event('input', { bubbles: true }))
        element.dispatchEvent(new Event('change', { bubbles: true }))
        if (wasPasswordField && element instanceof HTMLInputElement) {
          revealPassword(element)
        }
        historyEntry.fields.push({ element, oldValue, newValue: value })
      }

      if (historyEntry.fields.length > 0) {
        globalState.fillHistory.push(historyEntry)
        if (globalState.fillHistory.length > 20) globalState.fillHistory.shift()
      }
      return historyEntry.fields.length
    }

    // Key-value mode: match clipboard keys to field identifiers
    // Track used clipboard keys to prevent duplicate fills
    const usedKeys = new Set<string>()

    fillableFields.forEach((element) => {
      const value = findValueForField(element, usedKeys)
      if (!value) return

      // Remember if this was a password field before filling
      const wasPasswordField = element instanceof HTMLInputElement && element.type === 'password'

      const oldValue = element.value
      element.value = value

      // Trigger events
      element.dispatchEvent(new Event('input', { bubbles: true }))
      element.dispatchEvent(new Event('change', { bubbles: true }))

      // Auto-reveal password fields after filling
      if (wasPasswordField && element instanceof HTMLInputElement) {
        revealPassword(element)
      }

      historyEntry.fields.push({
        element,
        oldValue,
        newValue: value,
      })
    })

    if (historyEntry.fields.length > 0) {
      globalState.fillHistory.push(historyEntry)
      if (globalState.fillHistory.length > 20) {
        globalState.fillHistory.shift()
      }
    }

    return historyEntry.fields.length
  }

  // Undo last fill operation
  function undoLastFill() {
    const lastEntry = globalState.fillHistory.pop()
    if (!lastEntry) return

    for (const field of lastEntry.fields) {
      if (field.element && document.contains(field.element)) {
        field.element.value = field.oldValue
        field.element.dispatchEvent(new Event('input', { bubbles: true }))
        field.element.dispatchEvent(new Event('change', { bubbles: true }))
      }
    }
  }

  // Toggle widget visibility
  function toggleWidget() {
    if (globalState.isVisible) {
      hideWidget()
      return
    }

    const activeElement = document.activeElement
    if (!activeElement || !isFillableField(activeElement)) return

    showWidgetForElement(activeElement)
  }

  // Hide widget
  function hideWidget() {
    globalState.isVisible = false
    globalState.focusedElement = null
  }

  // Select template
  function selectTemplate(template: FillTemplate) {
    globalState.selectedTemplate = template
  }

  // Keyboard shortcut handler
  function handleKeyboardShortcut(event: KeyboardEvent) {
    // Don't handle shortcuts on disabled routes
    if (isRouteDisabled()) return

    const shortcut = globalState.config?.widget.keyboard_shortcut || 'Ctrl+Shift+F'
    const keys = shortcut.split('+').map((k) => k.toLowerCase())

    const ctrlRequired = keys.includes('ctrl')
    const shiftRequired = keys.includes('shift')
    const altRequired = keys.includes('alt')
    const keyRequired = keys.find((k) => !['ctrl', 'shift', 'alt'].includes(k))

    if (
      event.ctrlKey !== ctrlRequired ||
      event.shiftKey !== shiftRequired ||
      event.altKey !== altRequired ||
      event.key.toLowerCase() !== keyRequired
    ) {
      return
    }

    const activeElement = document.activeElement
    if (
      !(activeElement instanceof Element) ||
      (!activeElement.closest('.formfiller-widget') && !isElementInAllowedScope(activeElement))
    ) {
      return
    }

    event.preventDefault()
    toggleWidget()
  }

  // Setup focus/blur listeners
  function setupFocusListeners() {
    if (focusListenerAdded) return

    document.addEventListener('focusin', handleFocus, true)
    document.addEventListener('focusout', handleBlur, true)
    focusListenerAdded = true
  }

  // Setup and cleanup
  function setup() {
    currentDomain.value = window.location.hostname

    // Initialize
    initialize().then(() => {
      // Setup keyboard shortcut listener (only once)
      if (!keyboardListenerAdded) {
        document.addEventListener('keydown', handleKeyboardShortcut)
        keyboardListenerAdded = true
      }

      // Setup focus listeners for auto-show
      setupFocusListeners()
    })
  }

  function cleanup() {
    if (keyboardListenerAdded) {
      document.removeEventListener('keydown', handleKeyboardShortcut)
      keyboardListenerAdded = false
    }

    if (focusListenerAdded) {
      document.removeEventListener('focusin', handleFocus, true)
      document.removeEventListener('focusout', handleBlur, true)
      focusListenerAdded = false
    }
  }

  // For SPA navigation - no longer needed but keep for compatibility
  function autoScanAndShow() {
    // No-op - widget now shows on focus
  }

  return {
    state: globalState,
    canUndo,
    hasClipboardData,
    parsedFieldCount,
    initialize,
    setClipboardData,
    readFromClipboard,
    clearClipboardData,
    fillCurrentField,
    fillAllFields,
    undoLastFill,
    toggleWidget,
    hideWidget,
    selectTemplate,
    togglePasswordVisibility,
    isPasswordRevealed,
    setup,
    cleanup,
    autoScanAndShow,
  }
}
