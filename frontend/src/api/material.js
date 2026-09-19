import request from './request'

const BASE = import.meta.env.VITE_API_BASE_URL || '/api/v1'

// 现场材料接口: 故障各阶段照片 / 视频的上传、分组查看与下载。
export const materialApi = {
  // 按阶段分组查询故障的现场材料与完整性结论
  listByFault: (faultId) => request.get(`/faults/${faultId}/materials`),
  // 上传材料(multipart 表单), stage: registration / repair / acceptance
  upload: (faultId, { file, stage, title }) => {
    const form = new FormData()
    form.append('file', file)
    form.append('stage', stage)
    if (title) form.append('title', title)
    return request.post(`/faults/${faultId}/materials`, form, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 120000,
    })
  },
  remove: (id) => request.delete(`/materials/${id}`),
  // 内联查看地址(供 <img> / <video> 直接引用)
  fileUrl: (id) => `${BASE}/materials/${id}/file`,
  // 附件下载地址
  downloadUrl: (id) => `${BASE}/materials/${id}/file?download=1`,
}
