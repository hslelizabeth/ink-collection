<script setup>
// 藏品详情页：画廊 + 标题/状态印章 + 字段网格 + 关联藏品 + 备注
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { api } from '../api.js'
import { loadCategories, catById, catByKey } from '../store.js'
import { formatMoney, formatDate, isCssColor, statusLabel } from '../utils.js'
import CatIcon from '../components/CatIcon.vue'
import StateBlock from '../components/StateBlock.vue'

const route = useRoute()
const item = ref(null)
const loading = ref(true)
const error = ref('')
const activeImg = ref(0)

const cat = computed(() => (item.value ? catById(item.value.category_id) : null))

async function load() {
  loading.value = true
  error.value = ''
  try {
    await loadCategories()
    item.value = await api.getItem(route.params.id)
    activeImg.value = 0
  } catch (e) {
    error.value = e.message || '加载失败'
  } finally {
    loading.value = false
  }
}
onMounted(load)

const images = computed(() => item.value?.images || [])

// 字段网格：品牌 + 各专属字段 + 购入/结缘信息
const fieldCells = computed(() => {
  if (!item.value) return []
  const cells = [{ label: '品牌', value: item.value.brand || '—' }]
  for (const f of cat.value?.fields || []) {
    const v = item.value.fields?.[f.key]
    cells.push({ label: f.label, value: v ? String(v) : '—' })
  }
  cells.push(
    { label: '购入时间', value: formatDate(item.value.purchase_date) },
    { label: '购入价格', value: formatMoney(item.value.purchase_price) },
    { label: '结缘时间', value: formatDate(item.value.parted_date) },
    { label: '结缘价格', value: formatMoney(item.value.parted_price) }
  )
  return cells
})

// 关联藏品按关系名称分组
const relGroups = computed(() => {
  const groups = []
  for (const r of item.value?.relations || []) {
    let g = groups.find(g => g.label === r.label)
    if (!g) { g = { label: r.label, items: [] }; groups.push(g) }
    g.items.push(r)
  }
  return groups
})

// 色点：relation 带 color 字段且为合法 CSS 颜色时使用，否则用品类色
function relDotColor(r) {
  if (r.color && isCssColor(r.color)) return r.color
  return catByKey(r.category_key)?.color || '#a9853c'
}
</script>

<template>
  <StateBlock :loading="loading" :error="error" @retry="load">
    <template v-if="item">
      <div class="crumb">
        <router-link to="/">总览</router-link> /
        <router-link v-if="cat" :to="`/c/${cat.key}`">{{ cat.name }}</router-link>
        <template v-else>{{ item.category?.name || '' }}</template> /
        <b>{{ item.name }}</b>
      </div>
      <div class="detail">
        <div>
          <div class="gallery-main" :style="images.length ? {} : { background: cat?.color || '#3a4a5a' }">
            <img v-if="images.length" :src="images[activeImg].url" :alt="item.name">
            <CatIcon v-else :name="cat?.icon || 'generic'" />
          </div>
          <div v-if="images.length > 1" class="thumbs">
            <button
              v-for="(img, i) in images" :key="img.id"
              class="thumb" :class="{ active: i === activeImg }"
              @click="activeImg = i"
            ><img :src="img.thumb_url" :alt="`${item.name} 图 ${i + 1}`"></button>
          </div>
        </div>
        <div>
          <div class="d-title">
            <h2>{{ item.name }}</h2>
            <span class="stamp static" :class="{ sold: item.status === 'parted' }">{{ statusLabel(item.status) }}</span>
            <router-link class="btn ghost" :to="`/item/${item.id}/edit`">编辑</router-link>
          </div>
          <div class="d-fields">
            <div v-for="c in fieldCells" :key="c.label" class="d-field">
              <span>{{ c.label }}</span><b>{{ c.value }}</b>
            </div>
          </div>
          <div v-for="g in relGroups" :key="g.label" class="rel-block">
            <h4>{{ g.label }}</h4>
            <div class="rel-chips">
              <router-link v-for="r in g.items" :key="r.item_id" class="rel-chip" :to="`/item/${r.item_id}`">
                <img v-if="r.cover_thumb_url" :src="r.cover_thumb_url" :alt="r.name">
                <span v-else class="dot" :style="{ background: relDotColor(r) }"></span>
                {{ r.name }}
              </router-link>
            </div>
          </div>
          <div v-if="item.note" class="d-note">{{ item.note }}</div>
        </div>
      </div>
    </template>
  </StateBlock>
</template>
