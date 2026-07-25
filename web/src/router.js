import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  { path: '/', name: 'home', component: () => import('./views/HomeView.vue') },
  { path: '/c/:key', name: 'category', component: () => import('./views/CategoryView.vue') },
  { path: '/item/new', name: 'item-new', component: () => import('./views/ItemFormView.vue') },
  { path: '/item/:id(\\d+)', name: 'item-detail', component: () => import('./views/ItemDetailView.vue') },
  { path: '/item/:id(\\d+)/edit', name: 'item-edit', component: () => import('./views/ItemFormView.vue') },
  { path: '/admin', name: 'admin', component: () => import('./views/AdminView.vue') },
  { path: '/:pathMatch(.*)*', redirect: '/' }
]

export const router = createRouter({
  history: createWebHistory(),
  routes,
  scrollBehavior: () => ({ top: 0 })
})
