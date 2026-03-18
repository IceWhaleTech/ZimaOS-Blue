import { createApp } from 'vue'
import { createPinia } from 'pinia'
import router from './router'
import { i18n, initLocale } from './i18n'
import App from './App.vue'
import { initTypelessCopyHandler } from './utils/typelessRenderers'
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

async function bootstrap() {
  // Wait for the initial route (including auth guards) before mounting.
  // This avoids mounting protected layout components on public routes first.
  await router.isReady()

  // Load locale in background after mount.
  // The minimal fallback messages in i18n/index.ts cover the brief gap.
  app.mount('#app')

  initLocale().then(() => {
    // Sync locale store after browser language detection completes,
    // so the settings UI shows the correct language.
    useLocaleStore(pinia).syncFromI18n()
  })
}

void bootstrap()
