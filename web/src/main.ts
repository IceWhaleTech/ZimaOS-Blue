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

// Initialize locale (lazy load if needed) then mount
initLocale().then(() => {
  app.mount('#app')
})
