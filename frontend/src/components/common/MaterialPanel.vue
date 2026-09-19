<template>
  <div v-loading="loading" class="material-panel">
    <el-alert
      v-if="!stage && detail"
      :type="detail.complete ? 'success' : 'warning'"
      :closable="false"
      show-icon
      class="material-panel__alert"
    >
      <template #title>
        <span v-if="detail.complete">必要现场材料已齐全, 可正常闭环</span>
        <span v-else>缺失必要材料: {{ detail.missing.join('、') }} (缺失时不允许闭环)</span>
      </template>
    </el-alert>

    <div v-for="group in visibleGroups" :key="group.stage" class="material-group">
      <div class="material-group__header">
        <div>
          <span class="material-group__title">{{ group.label }}</span>
          <el-tag size="small" effect="plain" class="material-group__count">{{ group.items.length }} 个文件</el-tag>
          <span v-if="group.requirement" class="material-group__requirement text-muted">
            必要材料: {{ group.requirement }}
          </span>
        </div>
        <el-upload
          v-if="!readonly"
          :show-file-list="false"
          accept="image/jpeg,image/png,image/webp,image/gif,video/mp4,video/quicktime,video/webm"
          :before-upload="(file) => checkFile(file)"
          :http-request="(options) => handleUpload(group.stage, options)"
        >
          <el-button type="primary" size="small" :icon="Upload" :loading="uploadingStage === group.stage">
            上传照片 / 视频
          </el-button>
        </el-upload>
      </div>

      <div v-if="group.items.length" class="material-grid">
        <div v-for="item in group.items" :key="item.id" class="material-card">
          <el-image
            v-if="item.kind === 'photo'"
            :src="materialApi.fileUrl(item.id)"
            :preview-src-list="[materialApi.fileUrl(item.id)]"
            fit="cover"
            preview-teleported
            class="material-card__media"
          />
          <video v-else :src="materialApi.fileUrl(item.id)" controls preload="metadata" class="material-card__media" />
          <div class="material-card__meta">
            <div class="material-card__title" :title="item.title">{{ item.title || item.original_name || '-' }}</div>
            <div class="text-muted material-card__sub">
              {{ dictLabel(MATERIAL_KIND, item.kind) }} · {{ formatFileSize(item.size) }} · {{ formatDateTime(item.created_at) }}
            </div>
            <div class="material-card__actions">
              <el-button link type="primary" size="small" :icon="Download" @click="handleDownload(item)">下载</el-button>
              <el-button v-if="!readonly" link type="danger" size="small" :icon="Delete" @click="handleRemove(item)">
                删除
              </el-button>
            </div>
          </div>
        </div>
      </div>
      <el-empty v-else description="暂无材料" :image-size="48" class="material-group__empty" />
    </div>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Delete, Download, Upload } from '@element-plus/icons-vue'
import { materialApi } from '@/api/material'
import { MATERIAL_KIND, dictLabel } from '@/constants/dict'
import { formatDateTime, formatFileSize } from '@/utils/format'

const props = defineProps({
  faultId: { type: [Number, String], default: null },
  // 固定展示某个阶段(registration / repair / acceptance), 为空时展示全部阶段
  stage: { type: String, default: '' },
  // 只读模式(故障已关闭): 隐藏上传与删除
  readonly: { type: Boolean, default: false },
})

const emit = defineEmits(['change'])

const loading = ref(false)
const uploadingStage = ref('')
const detail = ref(null)

const visibleGroups = computed(() => {
  const groups = detail.value?.stages ?? []
  if (!props.stage) return groups
  return groups.filter((group) => group.stage === props.stage)
})

async function load() {
  if (!props.faultId) {
    detail.value = null
    return
  }
  loading.value = true
  try {
    detail.value = await materialApi.listByFault(props.faultId)
  } catch (error) {
    detail.value = null
  } finally {
    loading.value = false
  }
}

// 上传前做体积预检, 与后端限制保持一致(照片 10MB / 视频 200MB)。
function checkFile(file) {
  const isVideo = file.type?.startsWith('video/')
  const limit = isVideo ? 200 : 10
  if (file.size > limit * 1024 * 1024) {
    ElMessage.error(`${isVideo ? '视频' : '照片'}大小不能超过 ${limit} MB`)
    return false
  }
  return true
}

async function handleUpload(stage, options) {
  uploadingStage.value = stage
  try {
    await materialApi.upload(props.faultId, { file: options.file, stage })
    ElMessage.success('材料已上传')
    await load()
    emit('change')
  } catch (error) {
    // 错误提示由请求拦截器统一处理
  } finally {
    uploadingStage.value = ''
  }
}

function handleDownload(item) {
  const link = document.createElement('a')
  link.href = materialApi.downloadUrl(item.id)
  link.download = item.original_name || item.title || 'material'
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
}

async function handleRemove(item) {
  try {
    await ElMessageBox.confirm(`确认删除材料 "${item.title || item.original_name}" ?`, '删除确认', {
      type: 'warning',
      confirmButtonText: '确认删除',
      cancelButtonText: '取消',
    })
  } catch (error) {
    return
  }
  try {
    await materialApi.remove(item.id)
    ElMessage.success('材料已删除')
    await load()
    emit('change')
  } catch (error) {
    // 错误提示由请求拦截器统一处理
  }
}

watch(() => props.faultId, load, { immediate: true })

defineExpose({ reload: load })
</script>

<style scoped>
.material-panel__alert {
  margin-bottom: 16px;
}

.material-group + .material-group {
  margin-top: 16px;
}

.material-group__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}

.material-group__title {
  font-weight: 600;
}

.material-group__count {
  margin-left: 8px;
}

.material-group__requirement {
  margin-left: 8px;
  font-size: 12px;
}

.material-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 12px;
}

.material-card {
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 6px;
  overflow: hidden;
}

.material-card__media {
  width: 100%;
  height: 110px;
  display: block;
  background: #000;
  object-fit: cover;
}

.material-card__meta {
  padding: 8px;
}

.material-card__title {
  font-size: 13px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.material-card__sub {
  font-size: 12px;
  margin-top: 2px;
}

.material-card__actions {
  margin-top: 4px;
}

.material-group__empty {
  padding: 8px 0;
}
</style>
