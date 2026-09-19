import request from './request'

// 媒体文件直接通过 URL 访问(图片预览 / 视频播放 / 下载), 与接口同源无需鉴权。
const base = import.meta.env.VITE_API_BASE_URL || '/api/v1'

// 媒体在线访问地址(浏览器内联展示, 支持图片预览与视频 Range 播放)。
export const mediaInlineUrl = (id) => `${base}/materials/media/${id}?download=0`

// 媒体下载地址(附件方式下载)。
export const mediaDownloadUrl = (id) => `${base}/materials/media/${id}`

// 现场材料管理接口。
export const materialApi = {
  // 材料目录
  listCatalog: (params) => request.get('/materials/catalog', { params }),
  createCatalog: (data) => request.post('/materials/catalog', data),
  updateCatalog: (id, data) => request.put(`/materials/catalog/${id}`, data),
  removeCatalog: (id) => request.delete(`/materials/catalog/${id}`),
  catalogMeta: () => request.get('/materials/catalog/meta'),

  // 故障材料清单
  listItems: (params) => request.get('/materials/items', { params }),
  listFaultItems: (faultId) => request.get(`/materials/faults/${faultId}/items`),
  createFaultItem: (faultId, data) => request.post(`/materials/faults/${faultId}/items`, data),
  updateItem: (id, data) => request.put(`/materials/items/${id}`, data),
  receiveItem: (id, received) => request.post(`/materials/items/${id}/receive`, { received }),
  removeItem: (id) => request.delete(`/materials/items/${id}`),
  completeness: (faultId) => request.get(`/materials/faults/${faultId}/completeness`),
  summary: () => request.get('/materials/summary'),

  // 现场照片/视频
  listMedia: (faultId) => request.get(`/materials/faults/${faultId}/media`),
  uploadMedia: (faultId, formData, onUploadProgress) =>
    request.post(`/materials/faults/${faultId}/media`, formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 300000,
      onUploadProgress,
    }),
  removeMedia: (id) => request.delete(`/materials/media/${id}`),
}
