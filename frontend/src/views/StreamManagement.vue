<template>
  <div class="stream-management">
    <!-- 统计卡片 -->
    <el-row :gutter="20" class="stats-row">
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-content">
            <div class="stat-icon total">📺</div>
            <div class="stat-info">
              <div class="stat-value">{{ statistics.total }}</div>
              <div class="stat-label">媒体流总数</div>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-content">
            <div class="stat-icon online">▶</div>
            <div class="stat-info">
              <div class="stat-value success">{{ statistics.online }}</div>
              <div class="stat-label">在线流</div>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-content">
            <div class="stat-icon offline">⏸</div>
            <div class="stat-info">
              <div class="stat-value warning">{{ statistics.offline }}</div>
              <div class="stat-label">离线流</div>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-content">
            <div class="stat-icon viewers">👥</div>
            <div class="stat-info">
              <div class="stat-value">{{ statistics.totalViewers }}</div>
              <div class="stat-label">观众总数</div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 媒体流列表 -->
    <el-card shadow="hover" class="streams-card">
      <template #header>
        <div class="card-header">
          <span class="title">📡 媒体流列表</span>
          <div class="header-actions">
            <el-button type="primary" :icon="Plus" @click="showAddStreamDialog = true" :disabled="!zlmRunning">
              添加流
            </el-button>
            <el-button type="success" :icon="Refresh" @click="fetchStreams" :loading="loading">
              刷新
            </el-button>
          </div>
        </div>
      </template>

      <el-table :data="streams" style="width: 100%" v-loading="loading" empty-text="暂无媒体流">
        <el-table-column prop="app" label="应用" width="100">
          <template #default="{ row }">
            <el-tag type="info" size="small">{{ row.app || 'live' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="stream" label="流名称" width="160">
          <template #default="{ row }">
            <span style="font-family: monospace;">{{ row.stream || row.ID || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="isStreamOnline(row) ? 'success' : 'info'" size="small">
              {{ isStreamOnline(row) ? '在线' : '离线' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="类型" width="80">
          <template #default="{ row }">
            <el-tag type="warning" size="small">{{ row.schema || row.streamType || 'RTSP' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="观众" width="70" align="center">
          <template #default="{ row }">
            <span class="viewer-count">{{ row.readerCount || 0 }}</span>
          </template>
        </el-table-column>
        <el-table-column label="源地址" show-overflow-tooltip>
          <template #default="{ row }">
            <span class="source-url">{{ row.originUrl || row.URL || row.streamUrl || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <el-button 
              type="primary" 
              link 
              size="small" 
              @click="previewStream(row)"
              :disabled="!isStreamOnline(row)"
            >
              预览
            </el-button>
            <el-button type="success" link size="small" @click="copyStreamUrl(row)">
              复制地址
            </el-button>
            <el-button type="danger" link size="small" @click="removeStream(row)">
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 添加流对话框 -->
    <el-dialog v-model="showAddStreamDialog" title="添加媒体流" width="500px">
      <el-form :model="newStreamForm" label-width="100px">
        <el-form-item label="应用名称" required>
          <el-input v-model="newStreamForm.app" placeholder="例如: live" />
        </el-form-item>
        <el-form-item label="流名称" required>
          <el-input v-model="newStreamForm.stream" placeholder="例如: camera1" />
        </el-form-item>
        <el-form-item label="源地址">
          <el-input v-model="newStreamForm.url" placeholder="rtsp://... 或 rtmp://..." />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showAddStreamDialog = false">取消</el-button>
        <el-button type="primary" @click="addStream" :loading="addStreamLoading">添加</el-button>
      </template>
    </el-dialog>

    <!-- 预览对话框 -->
    <el-dialog v-model="showPreviewDialog" :title="`预览: ${previewInfo.stream}`" width="900px" @close="stopPreview">
      <div class="preview-container">
        <!-- 视频播放器 -->
        <div class="video-player-wrapper">
          <PreviewPlayer ref="previewPlayerRef" :show="true" :device="null" :channels="[]" :selectedChannelId="''"
            @playing="() => { previewLoading = false }"
            @error="(msg) => { previewLoading = false; ElMessage.error(msg || '播放失败') }"
            @loading="(val) => { previewLoading = val }"
          />
          <div v-if="previewLoading" class="video-loading">
            <el-icon class="is-loading"><Refresh /></el-icon>
            <span>正在加载...</span>
          </div>
        </div>
        
        <!-- 播放地址列表 -->
        <div class="preview-urls">
          <el-descriptions :column="2" border size="small">
            <el-descriptions-item label="HTTP-FLV">
              <div class="url-item">
                <el-link :href="previewInfo.httpFlv" target="_blank" :underline="false">
                  <span class="url-text">{{ previewInfo.httpFlv }}</span>
                </el-link>
                <el-button type="primary" link size="small" @click="copyUrl(previewInfo.httpFlv)">复制</el-button>
                <el-button type="success" link size="small" @click="playStream('flv')">播放</el-button>
              </div>
            </el-descriptions-item>
            <el-descriptions-item label="HLS">
              <div class="url-item">
                <el-link :href="previewInfo.hls" target="_blank" :underline="false">
                  <span class="url-text">{{ previewInfo.hls }}</span>
                </el-link>
                <el-button type="primary" link size="small" @click="copyUrl(previewInfo.hls)">复制</el-button>
                <el-button type="success" link size="small" @click="playStream('hls')">播放</el-button>
              </div>
            </el-descriptions-item>
            <el-descriptions-item label="RTSP">
              <div class="url-item">
                <span class="url-text">{{ previewInfo.rtsp }}</span>
                <el-button type="primary" link size="small" @click="copyUrl(previewInfo.rtsp)">复制</el-button>
              </div>
            </el-descriptions-item>
            <el-descriptions-item label="RTMP">
              <div class="url-item">
                <span class="url-text">{{ previewInfo.rtmp }}</span>
                <el-button type="primary" link size="small" @click="copyUrl(previewInfo.rtmp)">复制</el-button>
              </div>
            </el-descriptions-item>
          </el-descriptions>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted, computed, nextTick } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Refresh } from '@element-plus/icons-vue'
import PreviewPlayer from '../components/PreviewPlayer.vue'

interface Stream {
  app?: string
  stream?: string
  ID?: string
  streamID?: string
  deviceID?: string
  deviceName?: string
  streamType?: string
  streamUrl?: string
  URL?: string
  originUrl?: string
  startTime?: string
  status?: string
  online?: number
  Status?: string
  readerCount?: number
  schema?: string
}

const streams = ref<Stream[]>([])
const loading = ref(false)
const addStreamLoading = ref(false)
const showAddStreamDialog = ref(false)
const showPreviewDialog = ref(false)
const zlmRunning = ref(false)

// 统计信息
const statistics = computed(() => {
  const total = streams.value.length
  const online = streams.value.filter(s => isStreamOnline(s)).length
  const offline = total - online
  const totalViewers = streams.value.reduce((sum, s) => sum + (s.readerCount || 0), 0)
  return { total, online, offline, totalViewers }
})

// 判断流是否在线
const isStreamOnline = (row: Stream): boolean => {
  return row.online === 1 || row.Status === 'running' || row.status === 'running'
}

// 新建流表单
const newStreamForm = reactive({
  app: 'live',
  stream: '',
  url: ''
})

// 预览信息
const previewInfo = reactive({
  stream: '',
  url: '',
  httpFlv: '',
  hls: '',
  rtsp: '',
  rtmp: ''
})

// 播放器引用
const previewPlayerRef = ref<any>(null)
const previewLoading = ref(false)

// 定时刷新
let refreshTimer: number | null = null

// 获取 ZLM 状态
const checkZlmStatus = async () => {
  try {
    const response = await fetch('/api/zlm/status')
    const data = await response.json()
    zlmRunning.value = data.success && data.process?.running
  } catch {
    zlmRunning.value = false
  }
}

// 获取流列表
const fetchStreams = async () => {
  loading.value = true
  try {
    // 先获取 ZLM 流列表
    const zlmResponse = await fetch('/api/zlm/streams')
    const zlmData = await zlmResponse.json()
    
    let allStreams: Stream[] = []
    
    if (zlmData.streams) {
      allStreams = [...zlmData.streams]
    }
    
    // 也尝试获取本地流列表（如果有）
    try {
      const localResponse = await fetch('/api/stream/list')
      const localData = await localResponse.json()
      if (localData.streams) {
        // 合并本地流（避免重复）
        for (const stream of localData.streams) {
          const exists = allStreams.some(s => 
            (s.stream === stream.streamID) || (s.ID === stream.streamID)
          )
          if (!exists) {
            allStreams.push({
              ...stream,
              stream: stream.streamID,
              originUrl: stream.streamUrl
            })
          }
        }
      }
    } catch {
      // 忽略本地流列表获取失败
    }
    
    streams.value = allStreams
    await checkZlmStatus()
  } catch (error) {
    console.error('获取流列表失败:', error)
    ElMessage.error('获取流列表失败')
  } finally {
    loading.value = false
  }
}

// 添加流
const addStream = async () => {
  if (!newStreamForm.app || !newStreamForm.stream) {
    ElMessage.warning('请填写应用名称和流名称')
    return
  }

  addStreamLoading.value = true
  try {
    const response = await fetch('/api/zlm/streams/add', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(newStreamForm)
    })
    const data = await response.json()
    
    if (data.success || data.status === 'ok') {
      ElMessage.success('添加流成功')
      showAddStreamDialog.value = false
      newStreamForm.stream = ''
      newStreamForm.url = ''
      await fetchStreams()
    } else {
      ElMessage.error(data.error || data.msg || '添加流失败')
    }
  } catch (error) {
    console.error('添加流失败:', error)
    ElMessage.error('添加流失败')
  } finally {
    addStreamLoading.value = false
  }
}

// 预览流 - 从后端API获取正确的播放地址
const previewStream = async (row: Stream) => {
  const app = row.app || 'live'
  const stream = row.stream || row.ID || row.streamID || 'stream'
  
  previewInfo.stream = `${app}/${stream}`
  previewInfo.url = row.originUrl || row.URL || row.streamUrl || ''
  
  // 从后端API获取流的播放地址（包含正确的端口配置）
  try {
    const response = await fetch(`/api/zlm/streams/${app}/${stream}/urls`)
    if (response.ok) {
      const data = await response.json()
      // 使用后端返回的URL
      previewInfo.httpFlv = data.flv_url || data.httpFlv || ''
      previewInfo.hls = data.hls_url || data.hls || ''
      previewInfo.rtsp = data.rtsp_url || data.rtsp || ''
      previewInfo.rtmp = data.rtmp_url || data.rtmp || ''
    } else {
      ElMessage.error('获取流地址失败')
      return
    }
  } catch (error) {
    console.error('获取流地址失败:', error) 
    
  }
  
  showPreviewDialog.value = true
  // 打开对话框后使用 nextTick 启动播放并监听播放器事件
  previewLoading.value = true
  nextTick(() => {
    try {
      const candidate = previewPlayerRef.value
      const p = (candidate && typeof candidate.startWithStreamInfo === 'function') ? candidate : (candidate && candidate.value && typeof candidate.value.startWithStreamInfo === 'function') ? candidate.value : (candidate && candidate.$ && candidate.$.exposed && typeof candidate.$.exposed.startWithStreamInfo === 'function') ? candidate.$.exposed : null
      if (!p) {
        previewLoading.value = false
        return
      }
      // 启动播放
      p.startWithStreamInfo({ flv_url: previewInfo.httpFlv, hls_url: previewInfo.hls })
    } catch (e) { previewLoading.value = false }
  })
}

// 播放流
const playStream = async (type: 'flv' | 'hls') => {
  // 使用 PreviewPlayer 控制播放；优先 hls
  previewLoading.value = true
  try {
    const player = previewPlayerRef.value
    if (!player) {
      ElMessage.error('播放器未就绪')
      return
    }
    if (type === 'hls') {
      await player.startWithStreamInfo({ hls_url: previewInfo.hls })
    } else if (type === 'flv') {
      await player.startWithStreamInfo({ flv_url: previewInfo.httpFlv })
    } else {
      await player.startWithStreamInfo({ flv_url: previewInfo.httpFlv, hls_url: previewInfo.hls })
    }
  } catch (error) {
    console.error('播放失败:', error)
    ElMessage.error('播放失败，请检查流是否在线')
  } finally {
    previewLoading.value = false
  }
}

// 停止预览
const stopPreview = () => {
  try { previewPlayerRef.value?.stopPlaybackOnly() } catch (e) {}
  try { previewPlayerRef.value?.stopPreview() } catch (e) {}
}

// 复制 URL
const copyUrl = (url: string) => {
  navigator.clipboard.writeText(url).then(() => {
    ElMessage.success('已复制到剪贴板')
  }).catch(() => {
    ElMessage.error('复制失败')
  })
}

// 复制流地址 - 使用已获取的正确地址
const copyStreamUrl = (row: Stream) => {
  // 优先使用RTSP地址（最通用）
  const app = row.app || 'live'
  const stream = row.stream || row.ID || row.streamID || 'stream'
  // 使用相对路径，让后端代理处理
  const url = `http://${window.location.host}/zlm/${app}/${stream}.live.flv`
  copyUrl(url)
}

// 删除流
const removeStream = async (row: Stream) => {
  const app = row.app || 'live'
  const stream = row.stream || row.ID || row.streamID || ''
  
  try {
    await ElMessageBox.confirm(`确定要删除流 ${app}/${stream} 吗？`, '确认删除', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
  } catch {
    return
  }

  try {
    const response = await fetch(`/api/zlm/streams/${app}_${stream}/remove`, {
      method: 'DELETE'
    })
    const data = await response.json()
    
    if (data.success || data.status === 'ok') {
      ElMessage.success('删除成功')
      await fetchStreams()
    } else {
      ElMessage.error(data.error || data.msg || '删除失败')
    }
  } catch (error) {
    console.error('删除流失败:', error)
    ElMessage.error('删除流失败')
  }
}

onMounted(() => {
  fetchStreams()
  // 每10秒自动刷新
  refreshTimer = window.setInterval(fetchStreams, 10000)
})

onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
  }
  stopPreview()
})
</script>

<style scoped>
.stream-management {
  padding: 20px;
}

/* 统计卡片样式 */
.stats-row {
  margin-bottom: 20px;
}

.stat-card {
  transition: all 0.3s;
}

.stat-card:hover {
  transform: translateY(-3px);
}

.stat-content {
  display: flex;
  align-items: center;
  gap: 16px;
}

.stat-icon {
  width: 48px;
  height: 48px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
}

.stat-icon.total {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}

.stat-icon.online {
  background: linear-gradient(135deg, #11998e 0%, #38ef7d 100%);
}

.stat-icon.offline {
  background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%);
}

.stat-icon.viewers {
  background: linear-gradient(135deg, #4facfe 0%, #00f2fe 100%);
}

.stat-info {
  flex: 1;
}

.stat-value {
  font-size: 28px;
  font-weight: 600;
  color: #303133;
  line-height: 1.2;
}

.stat-value.success {
  color: #67c23a;
}

.stat-value.warning {
  color: #e6a23c;
}

.stat-value.danger {
  color: #f56c6c;
}

.stat-label {
  font-size: 14px;
  color: #909399;
  margin-top: 4px;
}

/* 流列表卡片 */
.streams-card {
  margin-bottom: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-header .title {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
}

.header-actions {
  display: flex;
  gap: 10px;
}

.viewer-count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 24px;
  height: 20px;
  padding: 0 6px;
  background: #ecf5ff;
  border-radius: 10px;
  color: #409eff;
  font-size: 12px;
  font-weight: 500;
}

.source-url {
  font-family: monospace;
  font-size: 12px;
  color: #606266;
}

/* 预览相关样式 */
.preview-container {
  min-height: 200px;
}

.video-player-wrapper {
  position: relative;
  background: #000;
  border-radius: 8px;
  overflow: hidden;
  margin-bottom: 16px;
}

.video-player {
  width: 100%;
  height: 400px;
  display: block;
}

.video-loading {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  color: #fff;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  font-size: 14px;
}

.video-loading .el-icon {
  font-size: 32px;
}

.preview-urls {
  margin-top: 16px;
}

.url-item {
  display: flex;
  align-items: center;
  gap: 8px;
}

.url-text {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 280px;
  font-size: 12px;
  font-family: monospace;
}
</style>