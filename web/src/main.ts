import { createApp } from 'vue'
import { createPinia } from 'pinia'
import router from './router'
import { i18n, initLocale } from './i18n'
import App from './App.vue'
import './style.css'

const app = createApp(App)

app.use(createPinia())
app.use(router)
app.use(i18n)

// Initialize locale (lazy load if needed) then mount
initLocale().then(() => {
  app.mount('#app')
})
