<script setup>
// 加载 / 错误 / 空态 统一展示
defineProps({
  loading: { type: Boolean, default: false },
  error: { type: String, default: '' },
  empty: { type: Boolean, default: false },
  emptyText: { type: String, default: '暂无数据' }
})
defineEmits(['retry'])
</script>

<template>
  <div v-if="loading" class="state-block">载入中…</div>
  <div v-else-if="error" class="state-block">
    <div class="err">{{ error }}</div>
    <button class="btn ghost" @click="$emit('retry')">重试</button>
  </div>
  <div v-else-if="empty" class="state-block"><slot name="empty">{{ emptyText }}</slot></div>
  <slot v-else />
</template>
