<script setup>
// 藏品卡片：桌面竖向 / 移动端横向（样式控制）
import { computed } from 'vue'
import CatIcon from './CatIcon.vue'
import { formatMoney, statusLabel } from '../utils.js'

const props = defineProps({
  item: { type: Object, required: true },
  category: { type: Object, default: null } // 品类配置（用于占位色/图标/字段标签）
})

const icon = computed(() => props.category?.icon || 'generic')
const color = computed(() => props.category?.color || '#3a4a5a')

// 卡片上的信息标签：品牌 + 第一个有值的专属字段
const tags = computed(() => {
  const t = []
  if (props.item.brand) t.push(props.item.brand)
  const fields = props.category?.fields || []
  for (const f of fields) {
    const v = props.item.fields?.[f.key]
    if (v) { t.push(String(v)); break }
  }
  return t
})

const priceText = computed(() => {
  if (props.item.status === 'parted' && props.item.parted_price != null) {
    return `结缘 ${formatMoney(props.item.parted_price)}`
  }
  if (props.item.purchase_price != null) return `购入 ${formatMoney(props.item.purchase_price)}`
  return ''
})
</script>

<template>
  <router-link class="item-card" :to="`/item/${item.id}`">
    <div class="item-img" :class="{ parted: item.status === 'parted' }" :style="item.cover_thumb_url ? {} : { background: color }">
      <img v-if="item.cover_thumb_url" :src="item.cover_thumb_url" :alt="item.name" loading="lazy">
      <CatIcon v-else :name="icon" />
      <span v-if="item.status === 'parted'" class="stamp sold">{{ statusLabel(item.status) }}</span>
    </div>
    <div class="item-body">
      <h4>{{ item.name }}</h4>
      <div class="item-meta">
        <span v-for="(t, i) in tags" :key="i" class="tag">{{ t }}</span>
      </div>
      <div v-if="priceText" class="item-price">{{ priceText }}</div>
    </div>
  </router-link>
</template>
