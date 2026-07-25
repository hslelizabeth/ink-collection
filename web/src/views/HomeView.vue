<script setup>
// 总览页：hero + 统计卡 + 分类统计 + 品类入口 + 最新入库
import { ref, computed, onMounted } from 'vue'
import { api } from '../api.js'
import { store, loadCategories, catById } from '../store.js'
import { formatMoney } from '../utils.js'
import CatIcon from '../components/CatIcon.vue'
import ItemCard from '../components/ItemCard.vue'
import StateBlock from '../components/StateBlock.vue'

const stats = ref(null)
const recent = ref([])
const loading = ref(true)
const error = ref('')

async function load() {
  loading.value = true
  error.value = ''
  try {
    await loadCategories()
    const [s, r] = await Promise.all([
      api.getStats(),
      api.listItems({ page: 1, page_size: 4 })
    ])
    stats.value = s
    recent.value = r.list || []
  } catch (e) {
    error.value = e.message || '加载失败'
  } finally {
    loading.value = false
  }
}
onMounted(load)

const catRows = computed(() => {
  if (!stats.value?.categories?.length) return []
  const maxVal = Math.max(...stats.value.categories.map(c => c.total_value || 0), 1)
  const totalCount = stats.value.categories.reduce((s, c) => s + (c.count || 0), 0) || 1
  return stats.value.categories.map(c => ({
    ...c,
    pct: Math.max(Math.round(((c.total_value || 0) / maxVal) * 100), 2),
    share: Math.round(((c.count || 0) / totalCount) * 100)
  }))
})

function catInfo(c) {
  // 品类入口卡：件数与金额来自 stats，图标/颜色来自品类配置
  const st = stats.value?.categories?.find(s => s.id === c.id)
  return {
    count: st?.count ?? c.item_count ?? 0,
    value: st?.total_value ?? 0
  }
}
</script>

<template>
  <div class="hero">
    <div class="hero-text">
      <h2>藏器于身，<em>静待知音</em></h2>
      <p>记录每一支笔、每一锭墨的来处与归处。<br>收藏流转，皆有痕迹。</p>
    </div>
    <div class="hero-verse">笔墨纸砚，文房四宝</div>
  </div>

  <StateBlock :loading="loading" :error="error" @retry="load">
    <div class="stats">
      <div class="stat accent"><b>{{ stats?.total_collecting ?? 0 }}</b><span>当前收藏</span></div>
      <div class="stat gold"><b>{{ formatMoney(stats?.total_value ?? 0) }}</b><span>收藏总值</span></div>
    </div>

    <template v-if="catRows.length">
      <div class="section-title"><h3>分类统计</h3><span class="more">按收藏金额</span></div>
      <div class="catstats">
        <div v-for="c in catRows" :key="c.id" class="catstats-row">
          <span class="cname">{{ c.name }}<small>{{ c.count }} 件</small></span>
          <span class="catstats-bar"><i :style="{ width: c.pct + '%', background: c.color || '#3a4a5a' }"></i></span>
          <span class="cnum">{{ c.count }} 件 · 占比 {{ c.share }}%</span>
          <span class="cprice">{{ formatMoney(c.total_value) }}</span>
        </div>
      </div>
    </template>

    <div class="section-title"><h3>藏品品类</h3></div>
    <div v-if="store.categories.length" class="cats">
      <router-link v-for="c in store.categories" :key="c.id" class="cat" :to="`/c/${c.key}`">
        <CatIcon :name="c.icon || 'generic'" />
        <div>
          <h3>{{ c.name }}</h3>
          <p>{{ catInfo(c).count }} 件 · {{ formatMoney(catInfo(c).value) }}</p>
        </div>
        <span class="count">{{ catInfo(c).count }} 件</span>
      </router-link>
    </div>
    <div v-else class="state-block">尚未配置品类，请先到「管理」页新增</div>

    <template v-if="recent.length">
      <div class="section-title"><h3>最新入库</h3></div>
      <div class="recent">
        <ItemCard v-for="it in recent" :key="it.id" :item="it" :category="catById(it.category_id)" />
      </div>
    </template>
  </StateBlock>
</template>
