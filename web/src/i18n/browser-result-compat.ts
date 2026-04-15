type BrowserResultNamedArgs = Record<string, string | number>

type BrowserResultI18nAdapter = {
  t: (key: string, named?: BrowserResultNamedArgs) => string
  te: (key: string) => boolean
}

type BrowserResultCompatOptions = {
  screenshotFallback?: string
  preserveRawEnglishScreenshot?: boolean
}

type ParsedBrowserPageHeader = {
  title: string
  url: string
  body: string
  interactiveCount: number | null
  includesScreenshot: boolean
}

const SCREENSHOT_EXACT_MESSAGE_KEYS: Record<string, string> = {
  'Screenshot captured': 'resultCard.messages.screenshot_captured',
  'Screenshot captured (interactive elements unavailable)':
    'resultCard.messages.screenshot_captured_interactive_elements_unavailable',
  'Screenshot captured for active tab': 'resultCard.messages.screenshot_captured_for_active_tab',
}

const MAIN_CONTENT_PREFIX = 'Main content:\n'
const SECTION_MARKERS = [
  {
    marker: '\n\nPage structure:\n',
    key: 'resultCard.messages.browser_page_structure_section',
    fallback: 'Page structure',
  },
  {
    marker: '\n\nInteractive elements:\n',
    key: 'resultCard.messages.browser_interactive_elements_section',
    fallback: 'Interactive elements',
  },
] as const

function resolveBrowserResultText(
  adapter: BrowserResultI18nAdapter,
  key: string,
  fallback: string,
  named?: BrowserResultNamedArgs
): string {
  if (!adapter.te(key)) return fallback
  return named ? adapter.t(key, named) : adapter.t(key)
}

function localizeBrowserScrollDirection(
  adapter: BrowserResultI18nAdapter,
  direction: string
): string {
  const normalized = direction.trim().toLowerCase()
  if (!normalized) return ''
  const key = `resultCard.messages.browser_scroll_direction_${normalized}`
  return resolveBrowserResultText(adapter, key, normalized)
}

function translateLegacyScreenshotMessage(
  raw: string,
  adapter: BrowserResultI18nAdapter,
  options: BrowserResultCompatOptions
): string {
  const exactKey = SCREENSHOT_EXACT_MESSAGE_KEYS[raw]
  if (exactKey) {
    if (adapter.te(exactKey)) return adapter.t(exactKey)
    if (options.preserveRawEnglishScreenshot) return raw
    return options.screenshotFallback ?? raw
  }

  const screenshotForTabMatch = raw.match(/^Screenshot captured for tab (.+)$/)
  if (screenshotForTabMatch?.[1]) {
    const key = 'resultCard.messageTemplates.screenshot_captured_for_tab'
    if (adapter.te(key)) return adapter.t(key, { target: screenshotForTabMatch[1] })
    if (options.preserveRawEnglishScreenshot) return raw
    return options.screenshotFallback ?? raw
  }

  const screenshotForMatch = raw.match(/^Screenshot captured for (.+)$/)
  if (screenshotForMatch?.[1]) {
    const key = 'resultCard.messageTemplates.screenshot_captured_for'
    if (adapter.te(key)) return adapter.t(key, { target: screenshotForMatch[1] })
    if (options.preserveRawEnglishScreenshot) return raw
    return options.screenshotFallback ?? raw
  }

  if (/^Screenshot captured\b/.test(raw)) {
    if (options.preserveRawEnglishScreenshot) return raw
    return options.screenshotFallback ?? raw
  }

  return raw
}

function parseBrowserPageHeader(raw: string): ParsedBrowserPageHeader | null {
  if (!raw.startsWith('Page: ')) return null

  const bodyStart = raw.indexOf('\n\n')
  if (bodyStart < 0) return null

  let header = raw.slice('Page: '.length, bodyStart)
  const body = raw.slice(bodyStart + 2)
  let interactiveCount: number | null = null
  let includesScreenshot = false

  const screenshotCountMatch = header.match(/ — screenshot \+ (\d+) interactive elements$/)
  if (screenshotCountMatch?.[1]) {
    header = header.slice(0, screenshotCountMatch.index)
    interactiveCount = Number(screenshotCountMatch[1])
    includesScreenshot = true
  } else {
    const interactiveCountMatch = header.match(/ — (\d+) interactive elements$/)
    if (interactiveCountMatch?.[1]) {
      header = header.slice(0, interactiveCountMatch.index)
      interactiveCount = Number(interactiveCountMatch[1])
    }
  }

  if (!header.endsWith(')')) return null
  const titleUrlDivider = header.lastIndexOf(' (')
  if (titleUrlDivider < 0) return null

  return {
    title: header.slice(0, titleUrlDivider),
    url: header.slice(titleUrlDivider + 2, -1),
    body,
    interactiveCount,
    includesScreenshot,
  }
}

function translateLegacyBrowserPageMessage(raw: string, adapter: BrowserResultI18nAdapter): string {
  const parsed = parseBrowserPageHeader(raw)
  if (!parsed) return raw

  const { title, url, body, interactiveCount, includesScreenshot } = parsed
  if (body.startsWith(MAIN_CONTENT_PREFIX)) {
    const contentBody = body.slice(MAIN_CONTENT_PREFIX.length)
    for (const section of SECTION_MARKERS) {
      const markerIndex = contentBody.indexOf(section.marker)
      if (markerIndex < 0) continue
      return resolveBrowserResultText(
        adapter,
        'resultCard.messageTemplates.browser_page_main_content_with_section',
        raw,
        {
          title,
          url,
          content: contentBody.slice(0, markerIndex),
          section: resolveBrowserResultText(adapter, section.key, section.fallback),
          body: contentBody.slice(markerIndex + section.marker.length),
        }
      )
    }

    return resolveBrowserResultText(
      adapter,
      'resultCard.messageTemplates.browser_page_main_content',
      raw,
      {
        title,
        url,
        content: contentBody,
      }
    )
  }

  if (interactiveCount !== null) {
    return resolveBrowserResultText(
      adapter,
      includesScreenshot
        ? 'resultCard.messageTemplates.browser_page_with_screenshot_interactive_count'
        : 'resultCard.messageTemplates.browser_page_with_interactive_count',
      raw,
      {
        title,
        url,
        count: interactiveCount,
        body,
      }
    )
  }

  return resolveBrowserResultText(adapter, 'resultCard.messageTemplates.browser_page', raw, {
    title,
    url,
    body,
  })
}

export function translateHistoricalEnglishBrowserResult(
  raw: string,
  adapter: BrowserResultI18nAdapter,
  options: BrowserResultCompatOptions = {}
): string {
  const trimmed = raw.trim()
  if (!trimmed) return ''

  const screenshotMessage = translateLegacyScreenshotMessage(trimmed, adapter, options)
  if (screenshotMessage !== trimmed) return screenshotMessage

  const actionMatch = trimmed.match(/^Performed (.+) on @(\d+)$/)
  if (actionMatch?.[1] && actionMatch[2]) {
    return resolveBrowserResultText(
      adapter,
      'resultCard.messageTemplates.browser_action_performed_on_ref',
      trimmed,
      {
        action: actionMatch[1],
        ref: Number(actionMatch[2]),
      }
    )
  }

  const scrollMatch = trimmed.match(/^Scrolled page (up|down|left|right)$/)
  if (scrollMatch?.[1]) {
    return resolveBrowserResultText(
      adapter,
      'resultCard.messageTemplates.browser_scrolled_page',
      trimmed,
      {
        direction: localizeBrowserScrollDirection(adapter, scrollMatch[1]),
      }
    )
  }

  const openTabsMatch = trimmed.match(/^(\d+) open tabs$/)
  if (openTabsMatch?.[1]) {
    return resolveBrowserResultText(
      adapter,
      'resultCard.messageTemplates.browser_open_tabs',
      trimmed,
      {
        count: Number(openTabsMatch[1]),
      }
    )
  }

  const tabClosedMatch = trimmed.match(/^Tab (.+) closed$/)
  if (tabClosedMatch?.[1]) {
    return resolveBrowserResultText(
      adapter,
      'resultCard.messageTemplates.browser_tab_closed',
      trimmed,
      {
        target: tabClosedMatch[1],
      }
    )
  }

  const recipesAvailableMatch = trimmed.match(/^(\d+) recipes available$/)
  if (recipesAvailableMatch?.[1]) {
    return resolveBrowserResultText(
      adapter,
      'resultCard.messageTemplates.browser_recipes_available',
      trimmed,
      {
        count: Number(recipesAvailableMatch[1]),
      }
    )
  }

  const largeDomNoteMatch = trimmed.match(
    /^Page has (\d+) interactive elements and a large DOM\. Using interactive elements list\. Use 'screenshot' for visual layout\.$/
  )
  if (largeDomNoteMatch?.[1]) {
    return resolveBrowserResultText(
      adapter,
      'resultCard.messageTemplates.browser_large_dom_note',
      trimmed,
      {
        count: Number(largeDomNoteMatch[1]),
      }
    )
  }

  if (trimmed.startsWith('Page: ')) {
    return translateLegacyBrowserPageMessage(trimmed, adapter)
  }

  return trimmed
}
