<script setup>
// 关联藏品选择器：按目标品类搜索名称，下拉选择，已选为可移除 chip
import { ref, watch, onMounted, onBeforeUnmount } from 'vue'
import { api } from '../api.js'

const props = defineProps({
  targetKey: { type: String, required: true },   // 目标品类 key
  targetName: { type: String, default: '' },     // 目标品类名（提示用）
  categoryId: { type: [Number, String], default: null }, // 目标品类 id
  excludeId: { type: [Number, String], default: null },  // 编辑时排除自己
  selected: { type: Array, default: () => [] }   // [{id, name}]
})
const emit = defineEmits(['update:selected'])

const keyword = ref('')
const options = ref([])
const open = ref(false)
const searching = ref(false)
let timer = null

async function search() {
  if (!props.categoryId) return
  searching.value = true
  try {
    const data = await api.listItems({ category_id: props.categoryId, q: keyword.value, page_size: 20 })
    const picked = new Set(props.selected.map(s => s.id))
    options.value = (data.list || []).filter(it => it.id !== props.excludeId && !picked.has(it.id))
  } catch {
    options.value = []
  } finally {
    searching.value = false
  }
}

watch(keyword, () => {
  clearTimeout(timer)
  timer = setTimeout(search, 300)
})

function onFocus() {
  open.value = true
  search()
}

function pick(item) {
  emit('update:selected', [...props.selected, { id: item.id, name: item.name }])
  keyword.value = ''
  options.value = []
  open.value = false
}

function remove(id) {
  emit('update:selected', props.selected.filter(s => s.id !== id))
}

function onDocClick(e) {
  if (!rootEl.value?.contains(e.target)) open.value = false
}
const rootEl = ref(null)
onMounted(() => document.addEventListener('click', onDocClick))
onBeforeUnmount(() => document.removeEventListener('click', onDocClick))
</script>

<template>
  <div class="rel-picker" ref="rootEl">
    <input
      v-model="keyword"
      :placeholder="`搜索${targetName || '藏品'}名称…`"
      @focus="onFocus"
      @input="open = true"
    >
    <div v-if="open" class="opts">
      <div v-if="searching" class="empty-opt">搜索中…</div>
      <template v-else>
        <div v-for="o in options" :key="o.id" class="opt" @click="pick(o)">
          <span>{{ o.name }}</span>
          <span v-if="o.brand" class="meta">{{ o.brand }}</span>
        </div>
        <div v-if="!options.length" class="empty-opt">无可选藏品</div>
      </template>
    </div>
    <div v-if="selected.length" class="sel-chips">
      <span v-for="s in selected" :key="s.id" class="sel-chip">
        {{ s.name }}
        <button type="button" title="移除" @click="remove(s.id)">×</button>
      </span>
    </div>
  </div>
</template>
