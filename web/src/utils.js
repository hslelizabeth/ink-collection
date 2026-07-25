// 通用格式化工具
export function formatMoney(v) {
  if (v === null || v === undefined || v === '' || isNaN(Number(v))) return '—'
  const n = Number(v)
  const s = n.toLocaleString('en-US', { maximumFractionDigits: 2 })
  return `¥ ${s}`
}

export function formatDate(v) {
  if (!v) return '—'
  return String(v).slice(0, 10)
}

// 判断是否为合法 CSS 颜色值（用于墨水色点）
export function isCssColor(v) {
  if (!v || typeof v !== 'string') return false
  return typeof CSS !== 'undefined' && CSS.supports && CSS.supports('color', v.trim())
}

export function statusLabel(status) {
  return status === 'parted' ? '已结缘' : '收藏'
}
