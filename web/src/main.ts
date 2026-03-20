import { createApp } from 'vue'
import { createPinia } from 'pinia'
import router from './router'
import { i18n, initLocale } from './i18n'
import App from './App.vue'
import { initTypelessCopyHandler } from './utils/typelessClient'
import { reportStartupMark } from './utils/startupTrace'
import './style.css'

const app = createApp(App)
const pinia = createPinia()

app.use(pinia)
app.use(router)
app.use(i18n)

// Initialize typeless card copy handler
initTypelessCopyHandler()

// Initialize theme store early to apply theme to entire page
// This must be done after pinia is installed but before mounting
import { useThemeStore } from './stores/theme'
useThemeStore(pinia)

import { useLocaleStore } from './stores/locale'

function bootstrap() {
  reportStartupMark('frontend_bootstrap_enter')

  // Mount immediately so the shell can render while the initial route component
  // and redirects finish resolving in the background.
  app.mount('#app')
  reportStartupMark('app_mounted')

  void router.isReady().then(() => {
    reportStartupMark('router_ready')
  })

  // Load locale in background after mount.
  // The minimal fallback messages in i18n/index.ts cover the brief gap.
  initLocale().then(() => {
    // Sync locale store after browser language detection completes,
    // so the settings UI shows the correct language.
    useLocaleStore(pinia).syncFromI18n()
    reportStartupMark('locale_ready')
  })
}

void bootstrap()
