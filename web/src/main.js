import { createApp } from 'vue'
import App from './App.vue'
import { router } from './router.js'
import { invalidateAdminAuth } from './auth.js'
import './styles.css'

window.addEventListener('admin-auth-required', () => {
  invalidateAdminAuth()
  const current = router.currentRoute.value
  if (current.name !== 'admin-login') {
    router.push({ name: 'admin-login', query: { redirect: current.fullPath } })
  }
})

createApp(App).use(router).mount('#app')
