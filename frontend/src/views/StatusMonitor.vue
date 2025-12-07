<template>
  <div class="status-monitor-container">
    <div class="page-header">
      <h1>系统状态监控</h1>
      <el-button type="primary" size="small" @click="refreshAll">
        <el-icon><Refresh /></el-icon>
        刷新数据
      </el-button>
    </div>
    
    <el-row :gutter="20" class="status-grid">
      <!-- 服务器状态 -->
      <el-col :xs="24" :sm="12" :lg="6">
        <el-card class="status-card" shadow="hover">
          <template #header>
            <div class="card-header">
              <div class="card-left">
                <el-icon><Monitor /></el-icon>
                <span>服务器状态</span>
              </div>
              <el-tag :type="serverStatus === 'running' ? 'success' : 'danger'">
                {{ serverStatus === 'running' ? '运行中' : '停止' }}
              </el-tag>
            </div>
          </template>
          <div class="status-info">
            <div class="info-item">
              <el-icon><Clock /></el-icon>
              <div class="info-content">
                <span class="info-label">启动时间</span>
                <span class="info-value">{{ serverInfo.startTime }}</span>
              </div>
            </div>
            <div class="info-item">
              <el-icon><Timer /></el-icon>
              <div class="info-content">
                <span class="info-label">运行时长</span>
                <span class="info-value">{{ serverInfo.uptime }}</span>
              </div>
            </div>
            <div class="info-item">
              <el-icon><Memory /></el-icon>
              <div class="info-content">
                <span class="info-label">内存使用</span>
                <span class="info-value">{{ serverInfo.memoryUsage }}</span>
              </div>
            </div>
            <div class="info-item">
              <el-icon><Cpu /></el-icon>
              <div class="info-content">
                <span class="info-label">CPU使用率</span>
                <span class="info-value">{{ serverInfo.cpuUsage }}</span>
              </div>
            </div>
          </div>
        </el-card>
      </el-col>

      <!-- 服务状态 -->
      <el-col :xs="24" :sm="12" :lg="6">
        <el-card class="status-card" shadow="hover">
          <template #header>
            <div class="card-header">
              <div class="card-left">
                <el-icon><Service /></el-icon>
                <span>服务状态</span>
              </div>
            </div>
          </template>
          <div class="service-list">
            <div v-for="service in services" :key="service.name" class="service-item">
              <div class="service-info">
                <span class="service-name">{{ service.name }}</span>
                <span class="service-port">端口: {{ service.port }}</span>
              </div>
              <el-tag :type="service.status === 'running' ? 'success' : 'danger'">
                {{ service.status === 'running' ? '正常' : '异常' }}
              </el-tag>
            </div>
          </div>
        </el-card>
      </el-col>

      <!-- 设备统计 -->
      <el-col :xs="24" :sm="12" :lg="6">
        <el-card class="status-card" shadow="hover">
          <template #header>
            <div class="card-header">
              <div class="card-left">
                <el-icon><VideoCamera /></el-icon>
                <span>设备统计</span>
              </div>
            </div>
          </template>
          <div class="device-stats">
            <div class="stat-item">
              <el-icon><Camera /></el-icon>
              <div class="stat-content">
                <span class="stat-label">GB28181设备</span>
                <span class="stat-value">{{ deviceStats.gb28181.total }}</span>
                <span class="stat-online">在线: {{ deviceStats.gb28181.online }}</span>
              </div>
            </div>
            <div class="stat-item">
              <el-icon><Monitor /></el-icon>
              <div class="stat-content">
                <span class="stat-label">ONVIF设备</span>
                <span class="stat-value">{{ deviceStats.onvif.total }}</span>
                <span class="stat-online">在线: {{ deviceStats.onvif.online }}</span>
              </div>
            </div>
            <div class="stat-item">
              <el-icon><VideoPlay /></el-icon>
              <div class="stat-content">
                <span class="stat-label">活跃流</span>
                <span class="stat-value">{{ deviceStats.activeStreams }}</span>
              </div>
            </div>
          </div>
        </el-card>
      </el-col>

      <!-- 系统资源 -->
      <el-col :xs="24" :sm="12" :lg="6">
        <el-card class="status-card" shadow="hover">
          <template #header>
            <div class="card-header">
              <div class="card-left">
                <el-icon><TrendCharts /></el-icon>
                <span>系统资源</span>
              </div>
            </div>
          </template>
          <div class="resource-info">
            <div class="resource-item">
              <el-icon><HardDisk /></el-icon>
              <div class="resource-content">
                <span class="resource-label">磁盘使用率</span>
                <el-progress :percentage="systemResources.diskUsage" :color="getColor(systemResources.diskUsage)" />
              </div>
            </div>
            <div class="resource-item">
              <el-icon><Connection /></el-icon>
              <div class="resource-content">
                <span class="resource-label">网络流量</span>
                <div class="network-stats">
                  <span class="network-upload">上传: {{ systemResources.network.upload }}</span>
                  <span class="network-download">下载: {{ systemResources.network.download }}</span>
                </div>
              </div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 实时日志 -->
    <el-card class="logs-card" shadow="hover">
      <template #header>
        <div class="card-header">
          <div class="card-left">
            <el-icon><Document /></el-icon>
            <span>实时日志</span>
          </div>
          <div class="card-actions">
            <el-button size="small" @click="toggleAutoRefresh">
              {{ autoRefresh ? '停止自动刷新' : '开启自动刷新' }}
            </el-button>
            <el-button size="small" @click="clearLogs">清空日志</el-button>
          </div>
        </div>
      </template>
      <div class="logs-container">
        <div v-for="(log, index) in logs" :key="index" class="log-entry" :class="log.level">
          <span class="log-time">{{ log.time }}</span>
          <el-tag :type="getLogTagType(log.level)" size="small">{{ log.level.toUpperCase() }}</el-tag>
          <span class="log-message">{{ log.message }}</span>
        </div>
        <div v-if="logs.length === 0" class="empty-logs">
          <el-icon><Document /></el-icon>
          <span>暂无日志</span>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import axios from 'axios'

interface ServerInfo {
  startTime: string
  uptime: string
  memoryUsage: string
  cpuUsage: string
}

interface Service {
  name: string
  status: string
  port?: number
}

interface DeviceStats {
  gb28181: {
    total: number
    online: number
  }
  onvif: {
    total: number
    online: number
  }
  activeStreams: number
}

interface SystemResources {
  diskUsage: number
  network: {
    upload: string
    download: string
  }
}

interface LogEntry {
  time: string
  level: string
  message: string
}

const serverStatus = ref('running')
const serverInfo = ref<ServerInfo>({
  startTime: '--',
  uptime: '--',
  memoryUsage: '--',
  cpuUsage: '--'
})

const services = ref<Service[]>([
  { name: 'GB28181服务', status: 'running', port: 5060 },
  { name: 'ONVIF服务', status: 'running', port: 80 },
  { name: 'API服务', status: 'running', port: 9080 },
  { name: 'ZLM媒体服务', status: 'running', port: 8554 }
])

const deviceStats = ref<DeviceStats>({
  gb28181: { total: 0, online: 0 },
  onvif: { total: 0, online: 0 },
  activeStreams: 0
})

const systemResources = ref<SystemResources>({
  diskUsage: 0,
  network: { upload: '0 KB/s', download: '0 KB/s' }
})

const logs = ref<LogEntry[]>([])

const getColor = (percentage: number) => {
  if (percentage < 70) return '#67c23a'
  if (percentage < 90) return '#e6a23c'
  return '#f56c6c'
}

const clearLogs = () => {
  logs.value = []
}

const getLogTagType = (level: string) => {
  const l = (level || '').toUpperCase()
  if (l === 'INFO') return 'info'
  if (l === 'WARN' || l === 'WARNING') return 'warning'
  if (l === 'ERROR') return 'danger'
  return 'info'
}

const fetchStatus = async () => {
  try {
    // 获取服务器状态
    const statusResponse = await axios.get('http://localhost:9080/api/status')
    if (statusResponse.data.serverInfo) {
      serverInfo.value = statusResponse.data.serverInfo
    }

    // 获取设备统计
    const statsResponse = await axios.get('http://localhost:9080/api/stats')
    if (statsResponse.data.success && statsResponse.data.stats) {
      deviceStats.value = statsResponse.data.stats
    }

    // 获取系统资源
    const resourcesResponse = await axios.get('http://localhost:9080/api/resources')
    if (resourcesResponse.data.success && resourcesResponse.data.resources) {
      // 确保返回的 resources 包含 diskUsage 和 network
      const res = resourcesResponse.data.resources
      systemResources.value = {
        diskUsage: res.diskUsage || 0,
        network: res.network || { upload: '0 KB/s', download: '0 KB/s' }
      }
    }

    // 获取最新日志
    const logsResponse = await axios.get('http://localhost:9080/api/logs/latest')
    if (logsResponse.data.success && logsResponse.data.logs) {
      logs.value = [...logsResponse.data.logs, ...logs.value].slice(0, 50)
    }
  } catch (error) {
    console.error('获取状态信息失败:', error)
    // 添加错误日志
    logs.value.unshift({
      time: new Date().toLocaleTimeString(),
      level: 'ERROR',
      message: '获取系统状态失败'
    })
  }
}

let intervalId: number

onMounted(() => {
  fetchStatus()
  // 每5秒更新一次状态
  intervalId = setInterval(fetchStatus, 5000)
})

onUnmounted(() => {
  if (intervalId) {
    clearInterval(intervalId)
  }
})
</script>

<style scoped>
.status-monitor-container {
  padding: 20px;
  max-width: 1200px;
  margin: 0 auto;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.status-grid {
  /* 让 Element Plus 的 el-row/el-col 负责布局，避免覆盖 flex/grid 行为 */
  margin-bottom: 20px;
}

.el-col {
  margin-bottom: 20px;
}

.status-card {
  height: fit-content;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.card-left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.card-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.status-info .info-item {
  margin: 8px 0;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.status-info .info-item .info-content {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
}

.service-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.service-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 0;
  border-bottom: 1px solid #f0f0f0;
}

.service-item:last-child {
  border-bottom: none;
}

.device-stats {
  display: flex;
  flex-direction: column;
  gap: 15px;
}

.stat-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.stat-label {
  font-weight: 500;
}

.stat-value {
  font-size: 1.2em;
  font-weight: bold;
  color: #409eff;
}

.stat-online {
  font-size: 0.9em;
  color: #67c23a;
}

.resource-item {
  margin-bottom: 15px;
}

.resource-item:last-child {
  margin-bottom: 0;
}

.network-stats {
  display: flex;
  justify-content: space-between;
  margin-top: 5px;
}

.logs-card {
  margin-top: 20px;
}

.logs-container {
  max-height: 300px;
  overflow-y: auto;
  background: #f5f7fa;
  border-radius: 4px;
  padding: 10px;
}
.log-entry {
  display: flex;
  gap: 10px;
  padding: 5px 0;
  border-bottom: 1px solid #e0e0e0;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 0.9em;
}

.log-entry {
  display: flex;
  gap: 10px;
  padding: 5px 0;
  border-bottom: 1px solid #e0e0e0;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 0.9em;
  align-items: center;
}

.log-entry:last-child {
  border-bottom: none;
}


.log-time {
  color: #909399;
  min-width: 80px;
}

/* 日志等级通过 .log-entry.<LEVEL> 设置颜色 */
.log-entry.INFO { color: #409eff; }
.log-entry.WARN { color: #e6a23c; }
.log-entry.ERROR { color: #f56c6c; }

.log-message {
  flex: 1;
}
</style>