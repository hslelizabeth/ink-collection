<script setup>
// 品类列表页：面包屑 + 筛选栏 + 卡片栅格 + 分页
import { ref, computed, watch, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { api } from '../api.js'
import { store, loadCategories, catByKey } from '../store.js'
import ItemCard from '../components/ItemCard.vue'
import StateBlock from '../components/StateBlock.vue'

const route = useRoute()
const cat = computed(() => catByKey(route.params.key))

const items = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 24
const loading = ref(true)
const error = ref('')

// 筛选条件
const status = ref('')       // '' | collecting | parted
const brand = ref('')
const fieldFilters = ref({}) // { [fieldKey]: value }
const q = ref('')
const sort = ref('purchase_date') // purchase_date | brand | price_desc | price_asc
const filterOptions = ref({ brands: [], fields: {} })

const totalPages = computed(() => Math.max(Math.ceil(total.value / pageSize), 1))

let qTimer = null
watch(q, () => {
  clearTimeout(qTimer)
  qTimer = setTimeout(() => { page.value = 1; loadItems() }, 350)
})

function setStatus(v) { status.value = v; page.value = 1; loadItems() }
function setBrand(v) { brand.value = v; page.value = 1; loadItems() }
function setField(k, v) { fieldFilters.value[k] = v; page.value = 1; loadItems() }
function setSort(v) { sort.value = v; page.value = 1; loadItems() }
function goPage(p) {
  if (p < 1 || p > totalPages.value) return
  page.value = p
  loadItems()
}

async function loadItems() {
  if (!cat.value) return
  loading.value = true
  error.value = ''
  const params = { category_id: cat.value.id, page: page.value, page_size: pageSize, sort: sort.value }
  if (status.value) params.status = status.value
  if (brand.value) params.brand = brand.value
  if (q.value.trim()) params.q = q.value.trim()
  for (const [k, v] of Object.entries(fieldFilters.value)) {
    if (v) params[`f_${k}`] = v
  }
  try {
    const data = await api.listItems(params)
    items.value = data.list || []
    total.value = data.total || 0
  } catch (e) {
    error.value = e.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function loadFilters() {
  if (!cat.value) return
  try {
    filterOptions.value = await api.getFilters(cat.value.id)
  } catch { /* 筛选项加载失败不阻塞列表 */ }
}

onMounted(async () => {
  await loadCategories()
  if (!cat.value) {
    error.value = '品类不存在'
    loading.value = false
    return
  }
  loadFilters()
  loadItems()
})

// 有可选值的专属字段才渲染筛选组
const fieldGroups = computed(() => {
  const fields = cat.value?.fields || []
  const opts = filterOptions.value.fields || {}
  return fields
    .map(f => ({ ...f, values: (opts[f.key] || []).filter(v => v !== '') }))
    .filter(f => f.values.length)
})

const brands = computed(() => (filterOptions.value.brands || []).filter(b => b))
</script>

<template>
  <div v-if="!cat" class="state-block">
    <template v-if="error"><span class="err">{{ error }}</span></template>
    <template v-else>载入中…</template>
  </div>
  <template v-else>
    <div class="crumb list-head">
      <span><router-link to="/">总览</router-link> / <b>{{ cat.name }}</b>（{{ total }} 件）</span>
      <router-link class="btn" :to="`/item/new?category=${cat.key}`">+ 新增</router-link>
    </div>

    <div class="filters">
      <div class="filter-group">
        <span class="fname">状态</span>
        <span class="chip" :class="{ active: status === '' }" @click="setStatus('')">全部</span>
        <span class="chip" :class="{ active: status === 'collecting' }" @click="setStatus('collecting')">收藏</span>
        <span class="chip" :class="{ active: status === 'parted' }" @click="setStatus('parted')">已结缘</span>
      </div>
      <div v-for="g in fieldGroups" :key="g.key" class="filter-group">
        <span class="fname">{{ g.label }}</span>
        <span class="chip" :class="{ active: !fieldFilters[g.key] }" @click="setField(g.key, '')">全部</span>
        <span
          v-for="v in g.values" :key="v"
          class="chip" :class="{ active: fieldFilters[g.key] === v }"
          @click="setField(g.key, v)"
        >{{ v }}</span>
      </div>
      <div v-if="brands.length" class="filter-group">
        <span class="fname">品牌</span>
        <span class="chip" :class="{ active: brand === '' }" @click="setBrand('')">全部</span>
        <span
          v-for="b in brands" :key="b"
          class="chip" :class="{ active: brand === b }"
          @click="setBrand(b)"
        >{{ b }}</span>
      </div>
      <input class="search" v-model="q" placeholder="搜索名称…">
      <div class="filter-group sort-group">
        <span class="fname">排序</span>
        <span class="chip" :class="{ active: sort === 'purchase_date' }" @click="setSort('purchase_date')">最新购入</span>
        <span class="chip" :class="{ active: sort === 'brand' }" @click="setSort('brand')">品牌</span>
        <span class="chip" :class="{ active: sort === 'price_desc' }" @click="setSort('price_desc')">价格高→低</span>
        <span class="chip" :class="{ active: sort === 'price_asc' }" @click="setSort('price_asc')">价格低→高</span>
      </div>
    </div>

    <StateBlock :loading="loading" :error="error" :empty="!items.length" @retry="loadItems">
      <template #empty>暂无藏品，去添加一件吧</template>
      <div class="grid">
        <ItemCard v-for="it in items" :key="it.id" :item="it" :category="cat" />
      </div>
      <div v-if="totalPages > 1" class="pager">
        <button class="btn ghost" :disabled="page <= 1" @click="goPage(page - 1)">上一页</button>
        <span>{{ page }} / {{ totalPages }}</span>
        <button class="btn ghost" :disabled="page >= totalPages" @click="goPage(page + 1)">下一页</button>
      </div>
    </StateBlock>
  </template>
</template>
