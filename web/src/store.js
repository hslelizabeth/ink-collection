import { reactive } from 'vue'
import { api } from './api.js'

// 简单的全局 store：品类配置 + 全局提示
export const store = reactive({
  categories: [],
  loaded: false,
  loadError: '',
  toast: '' // 轻量提示
})

let toastTimer = null
export function showToast(msg) {
  store.toast = msg
  clearTimeout(toastTimer)
  toastTimer = setTimeout(() => { store.toast = '' }, 3000)
}

export async function loadCategories(force = false) {
  if (store.loaded && !force) return
  store.loadError = ''
  try {
    store.categories = await api.listCategories()
    store.loaded = true
  } catch (e) {
    store.loadError = e.message || '品类加载失败'
  }
}

export function catByKey(key) {
  return store.categories.find(c => c.key === key) || null
}

export function catById(id) {
  return store.categories.find(c => c.id === id) || null
}
