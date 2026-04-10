import ExecutionEnvironment from '@docusaurus/ExecutionEnvironment'
import siteConfig from '@generated/docusaurus.config'

const CHINESE_LOCALE = 'zh-CN'

function trimTrailingSlash(value: string): string {
  if (value.length > 1 && value.endsWith('/')) {
    return value.slice(0, -1)
  }
  return value
}

function isHomePath(pathname: string, baseUrl: string): boolean {
  const normalizedPathname = trimTrailingSlash(pathname)
  const normalizedBaseUrl = trimTrailingSlash(baseUrl)
  return normalizedPathname === normalizedBaseUrl
}

function resolvePreferredLocale(): string {
  const browserLocales = Array.isArray(navigator.languages) && navigator.languages.length > 0
    ? navigator.languages
    : [navigator.language]

  for (const locale of browserLocales) {
    if (typeof locale === 'string' && locale.toLowerCase().startsWith('zh')) {
      return CHINESE_LOCALE
    }
  }

  return 'en'
}

function redirectToPreferredLocale(): void {
  if (!ExecutionEnvironment.canUseDOM) {
    return
  }

  const baseUrl = siteConfig.baseUrl || '/'
  const {pathname, search, hash} = window.location

  if (!isHomePath(pathname, baseUrl)) {
    return
  }

  if (resolvePreferredLocale() !== CHINESE_LOCALE) {
    return
  }

  const targetPath = `${baseUrl}${CHINESE_LOCALE}/`
  const normalizedTargetPath = targetPath.replace(/\/{2,}/g, '/')

  if (trimTrailingSlash(pathname) === trimTrailingSlash(normalizedTargetPath)) {
    return
  }

  window.location.replace(`${normalizedTargetPath}${search}${hash}`)
}

redirectToPreferredLocale()
