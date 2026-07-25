<script setup>
import { onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { store, loadCategories } from './store.js'
import CatIcon from './components/CatIcon.vue'
import SealLogo from './components/SealLogo.vue'

const route = useRoute()
onMounted(() => loadCategories())
</script>

<template>
  <header class="site-header">
    <router-link to="/" class="seal"><SealLogo /></router-link>
    <div>
      <div class="site-title">文房藏珍</div>
      <div class="site-sub">笔 · 墨 · 纸 · 砚</div>
    </div>
    <nav class="site-nav">
      <router-link to="/" exact-active-class="active">总览</router-link>
      <router-link
        v-for="c in store.categories" :key="c.id"
        :to="`/c/${c.key}`" active-class="active"
      >{{ c.name }}</router-link>
      <router-link to="/admin" active-class="active">管理</router-link>
    </nav>
  </header>

  <main>
    <router-view :key="route.fullPath" />
  </main>

  <!-- 移动端底部 Tab 栏（仅窄屏显示） -->
  <nav class="tabbar">
    <router-link to="/" exact-active-class="active">
      <CatIcon name="home" />
      <span>总览</span>
    </router-link>
    <router-link
      v-for="c in store.categories" :key="c.id"
      :to="`/c/${c.key}`" active-class="active"
    >
      <CatIcon :name="c.icon || 'generic'" />
      <span>{{ c.name }}</span>
    </router-link>
  </nav>

  <footer><span>文房藏珍 © 2026</span><span>藏器于身 · 静待知音</span></footer>

  <div v-if="store.toast" class="toast">{{ store.toast }}</div>
</template>
