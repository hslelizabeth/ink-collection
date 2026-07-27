<script setup>
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { checkAdminAuth, loginAdmin } from '../auth.js'

const route = useRoute()
const router = useRouter()
const password = ref('')
const loading = ref(false)
const checking = ref(true)
const error = ref('')

function destination() {
  const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/admin'
  return redirect.startsWith('/') && !redirect.startsWith('//') ? redirect : '/admin'
}

onMounted(async () => {
  if (await checkAdminAuth()) {
    router.replace(destination())
    return
  }
  checking.value = false
})

async function submit() {
  if (!password.value) {
    error.value = '请输入访问密码'
    return
  }
  loading.value = true
  error.value = ''
  try {
    await loginAdmin(password.value)
    router.replace(destination())
  } catch (e) {
    error.value = e.message || '验证失败'
    password.value = ''
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="admin-login">
    <div class="panel login-panel">
      <div class="login-mark">管</div>
      <h3>管理访问验证</h3>
      <p>请输入管理访问密码。验证后 30 分钟内保持有效，管理操作会自动续期。</p>
      <form v-if="!checking" @submit.prevent="submit">
        <div class="form-item">
          <label for="admin-password">访问密码</label>
          <input
            id="admin-password"
            v-model="password"
            type="password"
            autocomplete="current-password"
            autofocus
            placeholder="请输入访问密码"
          >
        </div>
        <div v-if="error" class="login-error">{{ error }}</div>
        <button class="btn login-submit" type="submit" :disabled="loading">
          {{ loading ? '验证中…' : '进入管理' }}
        </button>
      </form>
      <div v-else class="login-checking">正在确认管理权限…</div>
    </div>
  </div>
</template>
