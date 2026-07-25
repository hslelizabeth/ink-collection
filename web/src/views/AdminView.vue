<script setup>
// 管理页：藏品管理入口 / 品类配置 / 数据备份
import { ref, computed, onMounted } from 'vue'
import { api } from '../api.js'
import { store, loadCategories, catByKey, showToast } from '../store.js'
import CatIcon from '../components/CatIcon.vue'
import StateBlock from '../components/StateBlock.vue'

const tab = ref('cats') // items | cats | backup
const loading = ref(true)
const error = ref('')

async function load() {
  loading.value = true
  error.value = ''
  try {
    await loadCategories(true)
  } catch (e) {
    error.value = store.loadError || e.message || '加载失败'
  } finally {
    loading.value = false
  }
}
onMounted(load)

// ---- 品类编辑弹层 ----
const ICONS = [
  { key: 'pen', label: '钢笔' },
  { key: 'ink', label: '墨水' },
  { key: 'inkstone', label: '砚台' },
  { key: 'inkstick', label: '墨条' },
  { key: 'generic', label: '通用' }
]

const editing = ref(null) // 编辑中的品类草稿，null 表示关闭弹层
const savingCat = ref(false)

function blankCategory() {
  return {
    id: null, key: '', name: '', icon: 'generic', color: '#3a4a5a',
    fields: [], relations: [], sort: (store.categories.length + 1) * 10
  }
}

function openNew() {
  editing.value = blankCategory()
}

function openEdit(c) {
  editing.value = {
    id: c.id, key: c.key, name: c.name, icon: c.icon || 'generic',
    color: c.color || '#3a4a5a', sort: c.sort ?? 0,
    fields: (c.fields || []).map(f => ({ ...f, optionsText: (f.options || []).join('，') })),
    relations: (c.relations || []).map(r => ({ ...r }))
  }
}

function addField() {
  editing.value.fields.push({ label: '', key: '', type: 'text', optionsText: '' })
}
function removeField(i) {
  editing.value.fields.splice(i, 1)
}
function addRelation() {
  editing.value.relations.push({ target_key: store.categories[0]?.key || '', label: '' })
}
function removeRelation(i) {
  editing.value.relations.splice(i, 1)
}

// 关联可选目标品类（排除自身）
const relTargets = computed(() =>
  store.categories.filter(c => c.key !== editing.value?.key)
)

async function saveCategory() {
  const e = editing.value
  if (!e.name.trim()) { showToast('请填写品类名称'); return }
  if (!e.id && !/^[a-z][a-z0-9_]*$/.test(e.key)) {
    showToast('标识需为小写字母开头的英文/数字/下划线'); return
  }
  for (const f of e.fields) {
    if (!f.label.trim() || !/^[a-z][a-z0-9_]*$/.test(f.key)) {
      showToast('专属字段需填写名称，且标识为小写英文'); return
    }
  }
  savingCat.value = true
  const body = {
    key: e.key,
    name: e.name.trim(),
    icon: e.icon,
    color: e.color,
    sort: e.sort,
    fields: e.fields.map(f => ({
      key: f.key.trim(),
      label: f.label.trim(),
      type: f.type,
      ...(f.type === 'select'
        ? { options: f.optionsText.split(/[,，]/).map(s => s.trim()).filter(Boolean) }
        : {})
    })),
    relations: e.relations
      .filter(r => r.target_key && r.label.trim())
      .map(r => ({ target_key: r.target_key, label: r.label.trim() }))
  }
  try {
    if (e.id) await api.updateCategory(e.id, body)
    else await api.createCategory(body)
    await loadCategories(true)
    editing.value = null
    showToast('品类已保存')
  } catch (err) {
    showToast(err.message || '保存失败')
  } finally {
    savingCat.value = false
  }
}

async function removeCategory(c) {
  if (!confirm(`确定删除品类「${c.name}」吗？`)) return
  try {
    await api.deleteCategory(c.id)
    await loadCategories(true)
    showToast('品类已删除')
  } catch (e) {
    if (e.status === 409) showToast('该品类下仍有藏品，请先移走或删除藏品')
    else showToast(e.message || '删除失败')
  }
}

function relText(c) {
  if (!c.relations?.length) return '—'
  return c.relations
    .map(r => `↔ ${catByKey(r.target_key)?.name || r.target_key}（${r.label}）`)
    .join('、')
}
</script>

<template>
  <div class="admin">
    <aside class="admin-side">
      <a :class="{ active: tab === 'items' }" @click="tab = 'items'">藏品管理</a>
      <a :class="{ active: tab === 'cats' }" @click="tab = 'cats'">品类配置</a>
      <a :class="{ active: tab === 'backup' }" @click="tab = 'backup'">数据备份</a>
    </aside>

    <StateBlock :loading="loading" :error="error" @retry="load">
      <!-- 藏品管理：入口链接 -->
      <div v-if="tab === 'items'">
        <div class="panel">
          <h3>藏品管理</h3>
          <p style="font-size:13px;color:var(--ink-soft);line-height:2;margin-bottom:14px">
            按品类浏览与管理藏品，或在品类列表页点击「+ 新增」入库新藏品。
          </p>
          <div style="display:flex;gap:10px;flex-wrap:wrap">
            <router-link v-for="c in store.categories" :key="c.id" class="btn ghost" :to="`/c/${c.key}`">
              {{ c.name }}（{{ c.item_count ?? 0 }}）
            </router-link>
          </div>
        </div>
      </div>

      <!-- 品类配置 -->
      <div v-else-if="tab === 'cats'">
        <div class="panel">
          <h3>品类配置 <button class="btn" style="margin-left:auto" @click="openNew">+ 新增品类</button></h3>
          <table class="config-table">
            <tr><th>品类</th><th>通用字段</th><th>专属字段</th><th>关联</th><th>操作</th></tr>
            <tr v-for="c in store.categories" :key="c.id">
              <td>{{ c.name }}</td>
              <td>名称/品牌/状态/图片/购入结缘信息</td>
              <td>
                <span v-for="f in c.fields || []" :key="f.key" class="tag" style="margin-right:4px">{{ f.label }}</span>
                <template v-if="!c.fields?.length">—</template>
              </td>
              <td>{{ relText(c) }}</td>
              <td style="white-space:nowrap">
                <button class="btn ghost" @click="openEdit(c)">编辑</button>
                <button class="btn danger" style="margin-left:6px" @click="removeCategory(c)">删除</button>
              </td>
            </tr>
          </table>
          <!-- 移动端品类卡片 -->
          <div class="cat-cards">
            <div v-for="c in store.categories" :key="c.id" class="cat-card">
              <div class="cc-head">
                <b>{{ c.name }}</b>
                <span>
                  <button class="btn ghost" @click="openEdit(c)">编辑</button>
                  <button class="btn danger" style="margin-left:6px" @click="removeCategory(c)">删除</button>
                </span>
              </div>
              <div class="cc-row"><span>通用字段</span>名称 / 品牌 / 状态 / 图片 / 购入结缘信息</div>
              <div class="cc-row">
                <span>专属字段</span>
                <span>
                  <span v-for="f in c.fields || []" :key="f.key" class="tag" style="margin-right:4px">{{ f.label }}</span>
                  <template v-if="!c.fields?.length">—</template>
                </span>
              </div>
              <div class="cc-row"><span>关联</span>{{ relText(c) }}</div>
            </div>
          </div>
        </div>
      </div>

      <!-- 数据备份 -->
      <div v-else>
        <div class="panel">
          <h3>数据备份</h3>
          <p style="font-size:13px;color:var(--ink-soft);line-height:2;margin-bottom:16px">
            点击下方按钮下载完整数据库文件（含全部品类、藏品与关联配置）。<br>
            藏品图片保存在服务器的 uploads 目录中，请另行定期备份该目录。
          </p>
          <a class="btn" href="/api/backup" download>下载数据库备份</a>
        </div>
      </div>
    </StateBlock>
  </div>

  <!-- 品类编辑弹层 -->
  <div v-if="editing" class="modal-mask" @click.self="editing = null">
    <div class="modal">
      <h3>{{ editing.id ? '编辑品类' : '新增品类' }}</h3>
      <div class="form-grid">
        <div class="form-item">
          <label>品类名称</label>
          <input v-model="editing.name" placeholder="如：钢笔">
        </div>
        <div class="form-item">
          <label>标识（英文，用于网址）</label>
          <input v-model="editing.key" :disabled="!!editing.id" placeholder="如：pen">
        </div>
        <div class="form-item full">
          <label>图标</label>
          <div class="icon-pick">
            <span
              v-for="ic in ICONS" :key="ic.key"
              class="opt" :class="{ active: editing.icon === ic.key }"
              :title="ic.label"
              @click="editing.icon = ic.key"
            ><CatIcon :name="ic.key" /></span>
          </div>
        </div>
        <div class="form-item">
          <label>占位色</label>
          <input type="color" v-model="editing.color" style="padding:2px;height:40px">
        </div>
      </div>

      <div style="margin-top:18px">
        <label class="form-item" style="margin-bottom:6px"><label>专属字段</label></label>
        <div class="editor-hint">标识为小写英文（如 nib），类型为下拉时用逗号分隔选项。</div>
        <div v-for="(f, i) in editing.fields" :key="i" class="editor-row">
          <input v-model="f.label" placeholder="名称，如：笔尖类型" style="flex:1;min-width:110px">
          <input v-model="f.key" placeholder="标识，如：nib" style="width:110px">
          <select v-model="f.type">
            <option value="text">文本</option>
            <option value="select">下拉</option>
          </select>
          <input v-if="f.type === 'select'" v-model="f.optionsText" placeholder="选项，逗号分隔" style="flex:1;min-width:120px">
          <button class="mini-btn" @click="removeField(i)">删除</button>
        </div>
        <button class="mini-btn" @click="addField">+ 添加字段</button>
      </div>

      <div style="margin-top:18px">
        <label class="form-item" style="margin-bottom:6px"><label>关联配置</label></label>
        <div class="editor-hint">选择目标品类并命名关系，如：钢笔 ↔ 墨水（搭配墨水）。</div>
        <div v-for="(r, i) in editing.relations" :key="i" class="editor-row">
          <select v-model="r.target_key">
            <option v-for="c in relTargets" :key="c.key" :value="c.key">{{ c.name }}</option>
          </select>
          <input v-model="r.label" placeholder="关系名称，如：搭配墨水" style="flex:1;min-width:140px">
          <button class="mini-btn" @click="removeRelation(i)">删除</button>
        </div>
        <button class="mini-btn" :disabled="!relTargets.length" @click="addRelation">+ 添加关联</button>
      </div>

      <div class="form-actions">
        <button class="btn" :disabled="savingCat" @click="saveCategory">{{ savingCat ? '保存中…' : '保存' }}</button>
        <button class="btn ghost" @click="editing = null">取消</button>
      </div>
    </div>
  </div>
</template>
