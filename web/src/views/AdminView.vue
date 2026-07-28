<script setup>
// 管理页：藏品管理入口 / 品类配置 / 数据备份
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api.js'
import { logoutAdmin, invalidateAdminAuth } from '../auth.js'
import { store, loadCategories, catByKey, showToast } from '../store.js'
import CatIcon from '../components/CatIcon.vue'
import StateBlock from '../components/StateBlock.vue'

const router = useRouter()
const tab = ref('cats') // items | cats | backup | security
const loading = ref(true)
const error = ref('')
const downloading = ref(false)
const changingPassword = ref(false)
const passwordForm = ref({ current: '', next: '', confirm: '' })

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

async function downloadBackup() {
  downloading.value = true
  try {
    await api.downloadBackup()
    showToast('数据库备份已开始下载')
  } catch (e) {
    showToast(e.message || '备份下载失败')
  } finally {
    downloading.value = false
  }
}

async function changePassword() {
  const form = passwordForm.value
  if (!form.current) { showToast('请输入当前密码'); return }
  if (Array.from(form.next).length < 12) { showToast('新密码至少需要 12 个字符'); return }
  if (form.next !== form.confirm) { showToast('两次输入的新密码不一致'); return }
  changingPassword.value = true
  try {
    await api.changePassword({
      current_password: form.current,
      new_password: form.next,
      confirm_password: form.confirm
    })
    passwordForm.value = { current: '', next: '', confirm: '' }
    invalidateAdminAuth()
    showToast('密码已修改，请使用新密码重新验证')
    router.replace({ name: 'admin-login', query: { redirect: '/admin' } })
  } catch (e) {
    showToast(e.message || '修改密码失败')
  } finally {
    changingPassword.value = false
  }
}

async function logout() {
  await logoutAdmin()
  showToast('已退出管理')
  router.replace({ name: 'admin-login', query: { redirect: '/admin' } })
}

// ---- 品类编辑弹层 ----
const ICONS = [
  { key: 'pen', label: '钢笔' },
  { key: 'ink', label: '墨水' },
  { key: 'inkstone', label: '砚台' },
  { key: 'inkstick', label: '墨条' },
  { key: 'brush', label: '毛笔' },
  { key: 'paper', label: '宣纸' },
  { key: 'notebook', label: '本子' },
  { key: 'seal', label: '印章' },
  { key: 'inkpad', label: '印泥' },
  { key: 'paperweight', label: '镇纸' },
  { key: 'scroll', label: '卷轴' },
  { key: 'penholder', label: '笔筒' },
  { key: 'palette', label: '颜料' },
  { key: 'pencil', label: '铅笔' },
  { key: 'ruler', label: '尺子' },
  { key: 'scissors', label: '剪刀' },
  { key: 'bookmark', label: '书签' },
  { key: 'letter', label: '信封' },
  { key: 'eraser', label: '橡皮' },
  { key: 'tape', label: '胶带' },
  { key: 'glue', label: '胶棒' },
  { key: 'generic', label: '通用' }
]

const editing = ref(null) // 编辑中的品类草稿，null 表示关闭弹层
const savingCat = ref(false)
const reordering = ref(false)

// 品类自定义排序：与相邻品类交换位置，顺序同步到导航与首页统计
async function moveCategory(c, dir) {
  const list = store.categories
  const i = list.findIndex(x => x.id === c.id)
  const j = i + dir
  if (i < 0 || j < 0 || j >= list.length) return
  reordering.value = true
  try {
    const newOrder = [...list]
    ;[newOrder[i], newOrder[j]] = [newOrder[j], newOrder[i]]
    const bodyOf = (cat, sort) => ({
      key: cat.key, name: cat.name, icon: cat.icon, color: cat.color,
      sort, fields: cat.fields || [], relations: cat.relations || []
    })
    // 统一按新位置重排序号（历史数据可能使用 1,2,3… 旧刻度），只提交有变化的品类
    const updates = newOrder
      .map((cat, p) => ({ cat, sort: (p + 1) * 10 }))
      .filter(u => u.cat.sort !== u.sort)
    for (const u of updates) {
      await api.updateCategory(u.cat.id, bodyOf(u.cat, u.sort))
    }
    await loadCategories(true)
    showToast('顺序已更新')
  } catch (e) {
    showToast(e.message || '调整顺序失败')
  } finally {
    reordering.value = false
  }
}

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
      <a :class="{ active: tab === 'security' }" @click="tab = 'security'">访问安全</a>
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
            <tr v-for="(c, i) in store.categories" :key="c.id">
              <td>{{ c.name }}</td>
              <td>名称/品牌/状态/图片/购入结缘信息</td>
              <td>
                <span v-for="f in c.fields || []" :key="f.key" class="tag" style="margin-right:4px">{{ f.label }}</span>
                <template v-if="!c.fields?.length">—</template>
              </td>
              <td>{{ relText(c) }}</td>
              <td style="white-space:nowrap">
                <button class="btn ghost" :disabled="reordering || i === 0" title="上移" @click="moveCategory(c, -1)">↑</button>
                <button class="btn ghost" style="margin-left:6px" :disabled="reordering || i === store.categories.length - 1" title="下移" @click="moveCategory(c, 1)">↓</button>
                <button class="btn ghost" style="margin-left:6px" @click="openEdit(c)">编辑</button>
                <button class="btn danger" style="margin-left:6px" @click="removeCategory(c)">删除</button>
              </td>
            </tr>
          </table>
          <!-- 移动端品类卡片 -->
          <div class="cat-cards">
            <div v-for="(c, i) in store.categories" :key="c.id" class="cat-card">
              <div class="cc-head">
                <b>{{ c.name }}</b>
                <span>
                  <button class="btn ghost" :disabled="reordering || i === 0" title="上移" @click="moveCategory(c, -1)">↑</button>
                  <button class="btn ghost" style="margin-left:6px" :disabled="reordering || i === store.categories.length - 1" title="下移" @click="moveCategory(c, 1)">↓</button>
                  <button class="btn ghost" style="margin-left:6px" @click="openEdit(c)">编辑</button>
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
      <div v-else-if="tab === 'backup'">
        <div class="panel">
          <h3>数据备份</h3>
          <p style="font-size:13px;color:var(--ink-soft);line-height:2;margin-bottom:16px">
            点击下方按钮下载完整数据库文件（含全部品类、藏品与关联配置）。<br>
            藏品图片保存在服务器的 uploads 目录中，请另行定期备份该目录。
          </p>
          <button class="btn" :disabled="downloading" @click="downloadBackup">
            {{ downloading ? '正在生成…' : '下载数据库备份' }}
          </button>
        </div>
      </div>

      <!-- 访问安全 -->
      <div v-else>
        <div class="panel security-panel">
          <h3>访问安全</h3>
          <p class="security-hint">
            当前管理权限有效。管理会话在最后一次管理操作后保持 30 分钟；修改密码会让所有已登录设备立即退出。
          </p>
          <div class="form-grid security-form">
            <div class="form-item full">
              <label>当前密码</label>
              <input v-model="passwordForm.current" type="password" autocomplete="current-password">
            </div>
            <div class="form-item">
              <label>新密码（至少 12 个字符）</label>
              <input v-model="passwordForm.next" type="password" autocomplete="new-password">
            </div>
            <div class="form-item">
              <label>确认新密码</label>
              <input v-model="passwordForm.confirm" type="password" autocomplete="new-password">
            </div>
          </div>
          <div class="form-actions">
            <button class="btn" :disabled="changingPassword" @click="changePassword">
              {{ changingPassword ? '修改中…' : '修改访问密码' }}
            </button>
            <span class="spacer"></span>
            <button class="btn danger" @click="logout">退出管理</button>
          </div>
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
