<script setup>
// 藏品表单页：新增 /item/new?category=key 与编辑 /item/:id/edit 共用
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api.js'
import { store, loadCategories, catById, catByKey, showToast } from '../store.js'
import RelationPicker from '../components/RelationPicker.vue'
import StateBlock from '../components/StateBlock.vue'

const route = useRoute()
const router = useRouter()
const isEdit = computed(() => route.name === 'item-edit')
const itemId = computed(() => (isEdit.value ? Number(route.params.id) : null))

const loading = ref(true)
const error = ref('')
const saving = ref(false)

// 表单数据
const form = ref({
  category_id: null,
  name: '',
  brand: '',
  status: 'collecting',
  purchase_date: '',
  purchase_price: '',
  parted_date: '',
  parted_price: '',
  note: '',
  fields: {}
})
const relations = ref({}) // { [target_key]: [{id, name}] }
const existingImages = ref([]) // 编辑模式已有图片
const coverUrl = ref('')
const newFiles = ref([])       // [{file, preview}]

const cat = computed(() => catById(form.value.category_id))

onMounted(async () => {
  await loadCategories()
  try {
    if (isEdit.value) {
      const it = await api.getItem(itemId.value)
      form.value = {
        category_id: it.category_id,
        name: it.name || '',
        brand: it.brand || '',
        status: it.status || 'collecting',
        purchase_date: (it.purchase_date || '').slice(0, 10),
        purchase_price: it.purchase_price ?? '',
        parted_date: (it.parted_date || '').slice(0, 10),
        parted_price: it.parted_price ?? '',
        note: it.note || '',
        fields: { ...(it.fields || {}) }
      }
      // 已有关联按目标品类分组
      const rels = {}
      for (const r of it.relations || []) {
        ;(rels[r.category_key] ||= []).push({ id: r.item_id, name: r.name })
      }
      relations.value = rels
      existingImages.value = it.images || []
      coverUrl.value = it.cover_url || ''
    } else {
      const key = route.query.category
      const c = key ? catByKey(key) : store.categories[0]
      if (c) form.value.category_id = c.id
      if (!store.categories.length) error.value = '尚未配置品类，请先到「管理」页新增品类'
    }
  } catch (e) {
    error.value = e.message || '加载失败'
  } finally {
    loading.value = false
  }
})

function onPickImages(e) {
  for (const f of e.target.files || []) {
    newFiles.value.push({ file: f, preview: URL.createObjectURL(f) })
  }
  e.target.value = ''
}

function removeNewFile(i) {
  URL.revokeObjectURL(newFiles.value[i].preview)
  newFiles.value.splice(i, 1)
}

async function onDeleteImage(img) {
  if (!confirm('确定删除这张图片吗？')) return
  try {
    await api.deleteImage(img.id)
    existingImages.value = existingImages.value.filter(x => x.id !== img.id)
    showToast('图片已删除')
  } catch (e) {
    showToast(e.message || '删除失败')
  }
}

async function onSetCover(img) {
  try {
    await api.setCover(itemId.value, img.id)
    coverUrl.value = img.url
    showToast('已设为封面')
  } catch (e) {
    showToast(e.message || '设置失败')
  }
}

function numOrNull(v) {
  if (v === '' || v === null || v === undefined) return null
  const n = Number(v)
  return isNaN(n) ? null : n
}

async function save() {
  if (!form.value.category_id) { showToast('请选择品类'); return }
  if (!form.value.name.trim()) { showToast('请填写物品名称'); return }
  saving.value = true
  const body = {
    category_id: form.value.category_id,
    name: form.value.name.trim(),
    brand: form.value.brand.trim(),
    status: form.value.status,
    purchase_date: form.value.purchase_date || null,
    purchase_price: numOrNull(form.value.purchase_price),
    parted_date: form.value.status === 'parted' ? (form.value.parted_date || null) : null,
    parted_price: form.value.status === 'parted' ? numOrNull(form.value.parted_price) : null,
    note: form.value.note,
    fields: { ...form.value.fields },
    related_ids: Object.values(relations.value).flat().map(r => r.id)
  }
  try {
    let id = itemId.value
    if (isEdit.value) {
      await api.updateItem(id, body)
    } else {
      const created = await api.createItem(body)
      id = created.id
    }
    // 保存成功后上传新选择的图片
    if (newFiles.value.length && id) {
      try {
        await api.uploadImages(id, newFiles.value.map(x => x.file))
      } catch {
        showToast('藏品已保存，但部分图片上传失败')
        router.push(`/item/${id}`)
        return
      }
    }
    showToast(isEdit.value ? '已保存' : '已入库')
    router.push(`/item/${id}`)
  } catch (e) {
    showToast(e.message || '保存失败')
  } finally {
    saving.value = false
  }
}

async function remove() {
  if (!confirm(`确定删除「${form.value.name}」吗？此操作不可恢复。`)) return
  try {
    await api.deleteItem(itemId.value)
    showToast('已删除')
    router.push(cat.value ? `/c/${cat.value.key}` : '/')
  } catch (e) {
    showToast(e.message || '删除失败')
  }
}
</script>

<template>
  <StateBlock :loading="loading" :error="error" @retry="() => {}">
    <div class="crumb">
      <router-link to="/">总览</router-link> /
      <template v-if="cat"><router-link :to="`/c/${cat.key}`">{{ cat.name }}</router-link> /</template>
      <b>{{ isEdit ? '编辑藏品' : '新增藏品' }}</b>
    </div>

    <div class="panel">
      <h3>{{ isEdit ? `编辑：${form.name || ''}` : '新增藏品' }}</h3>
      <div class="form-grid">
        <div v-if="!isEdit" class="form-item">
          <label>品类</label>
          <select v-model="form.category_id">
            <option v-for="c in store.categories" :key="c.id" :value="c.id">{{ c.name }}</option>
          </select>
        </div>
        <div class="form-item">
          <label>物品名称</label>
          <input v-model="form.name" placeholder="如：写乐 长刀研 21K">
        </div>
        <div class="form-item">
          <label>品牌</label>
          <input v-model="form.brand" placeholder="如：Sailor">
        </div>
        <div class="form-item">
          <label>状态</label>
          <select v-model="form.status">
            <option value="collecting">收藏</option>
            <option value="parted">已结缘</option>
          </select>
        </div>
        <div v-for="f in cat?.fields || []" :key="f.key" class="form-item">
          <label>{{ f.label }}</label>
          <select v-if="f.type === 'select'" v-model="form.fields[f.key]">
            <option value="">未设置</option>
            <option v-for="o in f.options || []" :key="o" :value="o">{{ o }}</option>
          </select>
          <input v-else v-model="form.fields[f.key]" :placeholder="`请输入${f.label}`">
        </div>
        <div class="form-item">
          <label>购入时间</label>
          <input type="date" v-model="form.purchase_date">
        </div>
        <div class="form-item">
          <label>购入价格</label>
          <input type="number" min="0" step="0.01" v-model="form.purchase_price" placeholder="¥">
        </div>
        <template v-if="form.status === 'parted'">
          <div class="form-item">
            <label>结缘时间</label>
            <input type="date" v-model="form.parted_date">
          </div>
          <div class="form-item">
            <label>结缘价格</label>
            <input type="number" min="0" step="0.01" v-model="form.parted_price" placeholder="¥">
          </div>
        </template>
        <div v-for="r in cat?.relations || []" :key="r.target_key" class="form-item full">
          <label>{{ r.label }}（可多选）</label>
          <RelationPicker
            :target-key="r.target_key"
            :target-name="catByKey(r.target_key)?.name || ''"
            :category-id="catByKey(r.target_key)?.id"
            :exclude-id="itemId"
            :selected="relations[r.target_key] || []"
            @update:selected="relations[r.target_key] = $event"
          />
        </div>

        <div class="form-item full">
          <label>图片（可多张）</label>
          <div v-if="existingImages.length" class="img-list" style="margin-bottom:12px">
            <div v-for="img in existingImages" :key="img.id" class="img-cell">
              <div class="box">
                <img :src="img.thumb_url" alt="藏品图片">
                <span v-if="coverUrl === img.url" class="cover-flag">封面</span>
              </div>
              <div class="ops">
                <button type="button" @click="onSetCover(img)">设封面</button>
                <button type="button" @click="onDeleteImage(img)">删除</button>
              </div>
            </div>
          </div>
          <div v-if="newFiles.length" class="img-list" style="margin-bottom:12px">
            <div v-for="(f, i) in newFiles" :key="i" class="img-cell">
              <div class="box"><img :src="f.preview" alt="待上传预览"></div>
              <div class="ops"><button type="button" @click="removeNewFile(i)">移除</button></div>
            </div>
          </div>
          <label class="upload">
            {{ isEdit ? '点击添加图片（选择后立即保存时上传）' : '点击选择图片（保存后自动上传）' }}
            <input type="file" accept="image/*" multiple hidden @change="onPickImages">
          </label>
        </div>

        <div class="form-item full">
          <label>备注</label>
          <textarea rows="3" v-model="form.note"></textarea>
        </div>
      </div>

      <div class="form-actions">
        <button class="btn" :disabled="saving" @click="save">{{ saving ? '保存中…' : '保存' }}</button>
        <button class="btn ghost" @click="router.back()">取消</button>
        <span class="spacer"></span>
        <button v-if="isEdit" class="btn danger" @click="remove">删除藏品</button>
      </div>
    </div>
  </StateBlock>
</template>
