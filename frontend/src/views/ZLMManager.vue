<template>
  <div class="zlm-manager">
    <!-- 统计卡片 -->
    <el-row :gutter="20" class="stats-row">
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-content">
            <div class="stat-icon" :class="processStatus.running ? 'running' : 'stopped'">
              {{ processStatus.running ? '✓' : '✗' }}
            </div>
            <div class="stat-info">
              <div class="stat-value" :class="processStatus.running ? 'success' : 'danger'">
                {{ processStatus.running ? '运行中' : '已停止' }}
              </div>
              <div class="stat-label">运行状态</div>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-content">
            <div class="stat-icon pid">🔢</div>
            <div class="stat-info">
              <div class="stat-value">{{ processStatus.pid || '-' }}</div>
              <div class="stat-label">进程 PID</div>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-content">
            <div class="stat-icon time">⏱</div>
            <div class="stat-info">
              <div class="stat-value">{{ formatUptime(processStatus.uptime) }}</div>
              <div class="stat-label">运行时间</div>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-content">
            <div class="stat-icon restart">🔄</div>
            <div class="stat-info">
              <div class="stat-value">{{ processStatus.restartCount }} 次</div>
              <div class="stat-label">重启次数</div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 服务器配置信息 -->
    <el-card shadow="hover" class="config-card">
      <template #header>
        <div class="card-header">
          <span class="title">🖥️ ZLM 媒体服务器配置</span>
          <div class="header-actions">
            <el-tag :type="processStatus.healthy ? 'success' : 'warning'" size="small">
              {{ processStatus.healthy ? '健康' : '检查中' }}
            </el-tag>
            <el-button type="primary" :icon="Refresh" @click="refreshStatus" :loading="loading" size="small">
              刷新状态
            </el-button>
          </div>
        </div>
      </template>
      
      <el-descriptions :column="4" border size="small">
        <el-descriptions-item label="主机地址">
          <span style="font-family: monospace;">{{ serverStats.host || '-' }}</span>
        </el-descriptions-item>
        <el-descriptions-item label="HTTP 端口">
          <el-tag type="primary" size="small">{{ processStatus.httpPort || 8080 }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="RTSP 端口">
          <el-tag type="success" size="small">{{ serverStats.port || 8554 }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="最大流数">
          {{ serverStats.maxStreams || '-' }}
        </el-descriptions-item>
        <el-descriptions-item label="录像路径" :span="4">
          <span style="font-family: monospace;">{{ serverStats.recordingPath || '-' }}</span>
        </el-descriptions-item>
      </el-descriptions>
    </el-card>

    <!-- 进程控制 -->
    <el-card shadow="hover" class="control-card">
      <template #header>
        <div class="card-header">
          <span class="title">⚙️ 进程控制</span>
        </div>
      </template>
      
      <div class="control-buttons">
        <el-button 
          type="success" 
          :icon="VideoPlay" 
          @click="startProcess"
          :loading="actionLoading === 'start'"
          :disabled="processStatus.running"
          size="large"
        >
          启动 ZLM
        </el-button>
        <el-button 
          type="danger" 
          :icon="VideoPause" 
          @click="stopProcess"
          :loading="actionLoading === 'stop'"
          :disabled="!processStatus.running"
          size="large"
        >
          停止 ZLM
        </el-button>
        <el-button 
          type="warning" 
          :icon="RefreshRight" 
          @click="restartProcess"
          :loading="actionLoading === 'restart'"
          size="large"
        >
          重启 ZLM
        </el-button>
      </div>
      
      <el-divider />
      
      <el-alert
        :title="processStatus.running ? 'ZLM 媒体服务器正在运行' : 'ZLM 媒体服务器已停止'"
        :type="processStatus.running ? 'success' : 'warning'"
        :description="processStatus.running ? `当前已运行 ${formatUptime(processStatus.uptime)}，HTTP API 端口 ${processStatus.httpPort}` : '请点击上方按钮启动服务器'"
        show-icon
        :closable="false"
      />
    </el-card>

    <!-- 流统计信息 -->
    <el-card shadow="hover" class="streams-stats-card">
      <template #header>
        <div class="card-header">
          <span class="title">📊 流统计</span>
          <el-button type="primary" link @click="goToStreamManagement">
            查看详细列表 →
          </el-button>
        </div>
      </template>
      
      <el-row :gutter="20">
        <el-col :span="6">
          <div class="mini-stat">
            <el-statistic title="总流数" :value="serverStats.totalStreams || 0">
              <template #suffix>
                <span class="stat-suffix">个</span>
              </template>
            </el-statistic>
          </div>
        </el-col>
        <el-col :span="6">
          <div class="mini-stat">
            <el-statistic title="运行中" :value="serverStats.runningStreams || 0">
              <template #suffix>
                <span class="stat-suffix success">个</span>
              </template>
            </el-statistic>
          </div>
        </el-col>
        <el-col :span="6">
          <div class="mini-stat">
            <el-statistic title="已停止" :value="serverStats.stoppedStreams || 0">
              <template #suffix>
                <span class="stat-suffix warning">个</span>
              </template>
            </el-statistic>
          </div>
        </el-col>
        <el-col :span="6">
          <div class="mini-stat">
            <el-statistic title="错误" :value="serverStats.errorStreams || 0">
              <template #suffix>
                <span class="stat-suffix danger">个</span>
              </template>
            </el-statistic>
          </div>
        </el-col>
      </el-row>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { 
  Refresh, 
  VideoPlay, 
  VideoPause, 
  RefreshRight
} from '@element-plus/icons-vue'
import axios from 'axios'

const router = useRouter()

// 状态
const loading = ref(false)
const actionLoading = ref<string | null>(null)

// 进程状态
const processStatus = reactive({
  running: false,
  pid: 0,
  uptime: '',
  healthy: false,
  restartCount: 0,
  httpPort: 0,
  available: false
})

// 服务器统计
const serverStats = reactive({
  host: '',
  port: 0,
  maxStreams: 0,
  recordingPath: '',
  totalStreams: 0,
  runningStreams: 0,
  stoppedStreams: 0,
  errorStreams: 0
})

// 定时刷新
let refreshTimer: number | null = null

// 格式化运行时间
const formatUptime = (uptime: string): string => {
  if (!uptime || uptime === '') return '-'
  
  const match = uptime.match(/(?:(\d+)h)?(?:(\d+)m)?(?:(\d+(?:\.\d+)?)s)?/)
  if (!match) return uptime
  
  const hours = parseInt(match[1] || '0')
  const minutes = parseInt(match[2] || '0')
  const seconds = parseFloat(match[3] || '0')
  
  const parts: string[] = []
  if (hours > 0) parts.push(`${hours}小时`)
  if (minutes > 0) parts.push(`${minutes}分钟`)
  if (seconds > 0 || parts.length === 0) parts.push(`${Math.floor(seconds)}秒`)
  
  return parts.join(' ')
}

// 获取 ZLM 状态
const refreshStatus = async () => {
  loading.value = true
  try {
    const response = await axios.get('/api/zlm/status')
    const data = response.data
    
    if (data.success) {
      if (data.process) {
        processStatus.running = data.process.running || false
        processStatus.pid = data.process.pid || 0
        processStatus.uptime = data.process.uptime || ''
        processStatus.healthy = data.process.healthy || false
        processStatus.restartCount = data.process.restartCount || 0
        processStatus.httpPort = data.process.httpPort || 0
      }
      if (data.server) {
        serverStats.host = data.server.host || ''
        serverStats.port = data.server.port || 0
        serverStats.maxStreams = data.server.maxStreams || 0
        serverStats.recordingPath = data.server.recordingPath || ''
        serverStats.totalStreams = data.server.totalStreams || 0
        serverStats.runningStreams = data.server.runningStreams || 0
        serverStats.stoppedStreams = data.server.stoppedStreams || 0
        serverStats.errorStreams = data.server.errorStreams || 0
      }
    }
  } catch (error) {
    console.error('获取ZLM状态失败:', error)
  } finally {
    loading.value = false
  }
}

// 启动进程
const startProcess = async () => {
  actionLoading.value = 'start'
  try {
    const response = await axios.post('/api/zlm/process/start')
    const data = response.data
    
    if (data.success) {
      ElMessage.success(data.message || 'ZLM 启动成功')
      await refreshStatus()
    } else {
      ElMessage.error(data.error || 'ZLM 启动失败')
    }
  } catch (error: any) {
    console.error('启动ZLM失败:', error)
    ElMessage.error(error.response?.data?.error || '启动ZLM失败')
  } finally {
    actionLoading.value = null
  }
}

// 停止进程
const stopProcess = async () => {
  try {
    await ElMessageBox.confirm('确定要停止 ZLM 媒体服务器吗？', '确认停止', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
  } catch {
    return
  }

  actionLoading.value = 'stop'
  try {
    const response = await axios.post('/api/zlm/process/stop')
    const data = response.data
    
    if (data.success) {
      ElMessage.success(data.message || 'ZLM 已停止')
      await refreshStatus()
    } else {
      ElMessage.error(data.error || 'ZLM 停止失败')
    }
  } catch (error: any) {
    console.error('停止ZLM失败:', error)
    ElMessage.error(error.response?.data?.error || '停止ZLM失败')
  } finally {
    actionLoading.value = null
  }
}

// 重启进程
const restartProcess = async () => {
  try {
    await ElMessageBox.confirm('确定要重启 ZLM 媒体服务器吗？', '确认重启', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
  } catch {
    return
  }

  actionLoading.value = 'restart'
  try {
    const response = await axios.post('/api/zlm/process/restart')
    const data = response.data
    
    if (data.success) {
      ElMessage.success(data.message || 'ZLM 重启成功')
      await refreshStatus()
    } else {
      ElMessage.error(data.error || 'ZLM 重启失败')
    }
  } catch (error: any) {
    console.error('重启ZLM失败:', error)
    ElMessage.error(error.response?.data?.error || '重启ZLM失败')
  } finally {
    actionLoading.value = null
  }
}

// 跳转到流管理页面
const goToStreamManagement = () => {
  router.push('/streams')
}

onMounted(() => {
  refreshStatus()
  refreshTimer = window.setInterval(refreshStatus, 30000)
})

onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
  }
})
</script>

<style scoped>
.zlm-manager {
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
  color: #fff;
}

.stat-icon.running {
  background: linear-gradient(135deg, #11998e 0%, #38ef7d 100%);
}

.stat-icon.stopped {
  background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%);
}

.stat-icon.pid {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}

.stat-icon.time {
  background: linear-gradient(135deg, #4facfe 0%, #00f2fe 100%);
}

.stat-icon.restart {
  background: linear-gradient(135deg, #fa709a 0%, #fee140 100%);
}

.stat-info {
  flex: 1;
}

.stat-value {
  font-size: 24px;
  font-weight: 600;
  color: #303133;
  line-height: 1.2;
}

.stat-value.success {
  color: #67c23a;
}

.stat-value.danger {
  color: #f56c6c;
}

.stat-label {
  font-size: 14px;
  color: #909399;
  margin-top: 4px;
}

/* 配置卡片 */
.config-card {
  margin-bottom: 20px;
}

/* 控制卡片 */
.control-card {
  margin-bottom: 20px;
}

.control-buttons {
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
}

/* 流统计卡片 */
.streams-stats-card {
  margin-bottom: 20px;
}

.mini-stat {
  text-align: center;
  padding: 10px;
}

.stat-suffix {
  font-size: 14px;
  color: #909399;
  margin-left: 4px;
}

.stat-suffix.success {
  color: #67c23a;
}

.stat-suffix.warning {
  color: #e6a23c;
}

.stat-suffix.danger {
  color: #f56c6c;
}

/* 通用样式 */
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
  align-items: center;
}

:deep(.el-statistic__content) {
  font-size: 28px;
  font-weight: 600;
}

:deep(.el-statistic__head) {
  font-size: 14px;
  color: #909399;
}
</style>
