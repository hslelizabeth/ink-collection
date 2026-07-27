// 后端 API 封装（契约见项目说明）
async function request(path, options = {}) {
  let res
  try {
    res = await fetch(path, options)
  } catch (e) {
    throw new Error('网络连接失败，请检查服务是否在线')
  }
  if (!res.ok) {
    let msg = `请求失败（${res.status}）`
    try {
      const data = await res.json()
      if (data && data.error) msg = data.error
    } catch { /* 非 JSON 响应 */ }
    const err = new Error(msg)
    err.status = res.status
    throw err
  }
  if (res.status === 204) return null
  return res.json()
}

function jsonOptions(method, body) {
  return {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body)
  }
}

export const api = {
  // 品类
  listCategories: () => request('/api/categories'),
  createCategory: (data) => request('/api/categories', jsonOptions('POST', data)),
  updateCategory: (id, data) => request(`/api/categories/${id}`, jsonOptions('PUT', data)),
  deleteCategory: (id) => request(`/api/categories/${id}`, { method: 'DELETE' }),

  // 藏品
  listItems: (params = {}) => {
    const qs = new URLSearchParams()
    for (const [k, v] of Object.entries(params)) {
      if (Array.isArray(v)) {
        for (const item of v) qs.append(k, item)
      } else if (v !== undefined && v !== null && v !== '') {
        qs.set(k, v)
      }
    }
    const s = qs.toString()
    return request(`/api/items${s ? '?' + s : ''}`)
  },
  getItem: (id) => request(`/api/items/${id}`),
  createItem: (data) => request('/api/items', jsonOptions('POST', data)),
  updateItem: (id, data) => request(`/api/items/${id}`, jsonOptions('PUT', data)),
  deleteItem: (id) => request(`/api/items/${id}`, { method: 'DELETE' }),

  // 图片
  uploadImages: (itemId, files) => {
    const fd = new FormData()
    for (const f of files) fd.append('files', f)
    return request(`/api/items/${itemId}/images`, { method: 'POST', body: fd })
  },
  deleteImage: (id) => request(`/api/images/${id}`, { method: 'DELETE' }),
  setCover: (itemId, imageId) =>
    request(`/api/items/${itemId}/cover`, jsonOptions('PUT', { image_id: imageId })),

  // 筛选 / 统计
  getFilters: (categoryId) => request(`/api/filters?category_id=${categoryId}`),
  getStats: () => request('/api/stats')
}
