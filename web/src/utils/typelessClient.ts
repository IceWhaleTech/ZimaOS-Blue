export function initTypelessCopyHandler(): void {
  ;(window as unknown as { __typelessCopyCode?: (codeId: string) => void }).__typelessCopyCode =
    async (codeId: string) => {
      const codeElement = document.getElementById(codeId)
      if (!codeElement) return

      try {
        const code = codeElement.textContent || ''
        await navigator.clipboard.writeText(code)

        const btn = document.querySelector<HTMLElement>(`[data-code-id="${codeId}"]`)
        if (btn) {
          const originalHtml = btn.innerHTML
          btn.innerHTML =
            '<svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 text-green-400" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" /></svg>'
          setTimeout(() => {
            btn.innerHTML = originalHtml
          }, 2000)
        }
      } catch (err) {
        console.error('Failed to copy code:', err)
      }
    }
  ;(
    window as unknown as { __typelessToggleCodeCollapse?: (toggleButtonId: string) => void }
  ).__typelessToggleCodeCollapse = (toggleButtonId: string) => {
    const button = document.getElementById(toggleButtonId)
    if (!button) return

    const containerId = button.getAttribute('data-code-container-id')
    const fadeId = button.getAttribute('data-code-fade-id')
    if (!containerId) return

    const container = document.getElementById(containerId)
    if (!container) return

    const fade = fadeId ? document.getElementById(fadeId) : null
    const expandLabel = button.getAttribute('data-expand-label') || 'Show more'
    const collapseLabel = button.getAttribute('data-collapse-label') || 'Collapse'
    const collapsedMaxHeight = Number(button.getAttribute('data-collapsed-max-height') || 0)
    const isCollapsed = container.getAttribute('data-collapsed') !== 'false'

    if (isCollapsed) {
      container.style.maxHeight = 'none'
      container.style.overflowY = 'auto'
      container.setAttribute('data-collapsed', 'false')
      fade?.classList.add('hidden')
      button.textContent = collapseLabel
      return
    }

    container.style.maxHeight = collapsedMaxHeight > 0 ? `${collapsedMaxHeight}px` : ''
    container.style.overflowY = 'hidden'
    container.setAttribute('data-collapsed', 'true')
    fade?.classList.remove('hidden')
    button.textContent = expandLabel
  }
  ;(
    window as unknown as { __typelessCopyTerminal?: (terminalId: string) => void }
  ).__typelessCopyTerminal = async (terminalId: string) => {
    const btn = document.querySelector<HTMLElement>(`[data-terminal-id="${terminalId}"]`)
    if (!btn) return

    try {
      const content = btn.getAttribute('data-terminal-content') || ''
      const textarea = document.createElement('textarea')
      textarea.innerHTML = content
      const plainText = textarea.value

      await navigator.clipboard.writeText(plainText)

      const originalHtml = btn.innerHTML
      btn.innerHTML =
        '<svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 text-green-400" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" /></svg>'
      setTimeout(() => {
        btn.innerHTML = originalHtml
      }, 2000)
    } catch (err) {
      console.error('Failed to copy terminal content:', err)
    }
  }
  ;(
    window as unknown as {
      __typelessOpenFullscreen?: (type: string, dataJson: string, isBase64?: boolean) => void
    }
  ).__typelessOpenFullscreen = (type: string, dataJson: string, isBase64?: boolean) => {
    const json = isBase64 ? decodeURIComponent(escape(atob(dataJson))) : dataJson
    const event = new CustomEvent('typeless-fullscreen', {
      detail: { type, dataJson: json },
    })
    window.dispatchEvent(event)
  }
}
