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

// Mount immediately for fast first paint, load locale in background.
// The minimal fallback messages in i18n/index.ts cover the brief gap.
app.mount('#app')
initLocale()
