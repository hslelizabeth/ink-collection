import { reactive } from 'vue'
import { api } from './api.js'

export const adminAuth = reactive({
  authenticated: false,
  checked: false
})

export async function checkAdminAuth() {
  try {
    const data = await api.authStatus()
    adminAuth.authenticated = !!data.authenticated
  } catch {
    adminAuth.authenticated = false
  } finally {
    adminAuth.checked = true
  }
  return adminAuth.authenticated
}

export async function loginAdmin(password) {
  await api.login({ password })
  adminAuth.authenticated = true
  adminAuth.checked = true
}

export async function logoutAdmin() {
  try {
    await api.logout()
  } finally {
    invalidateAdminAuth()
  }
}

export function invalidateAdminAuth() {
  adminAuth.authenticated = false
  adminAuth.checked = true
}
