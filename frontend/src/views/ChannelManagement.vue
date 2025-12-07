<template>
  <div class="channel-management">
    <!-- 统计卡片 -->
    <el-row :gutter="20" class="stats-row">
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-content">
            <div class="stat-icon total">📺</div>
            <div class="stat-info">
              <div class="stat-value">{{ statistics.total }}</div>
              <div class="stat-label">通道总数</div>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-content">
            <div class="stat-icon online">✓</div>
            <div class="stat-info">
              <div class="stat-value success">{{ statistics.online }}</div>
              <div class="stat-label">在线通道</div>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-content">
            <div class="stat-icon gb28181">📡</div>
            <div class="stat-info">
              <div class="stat-value">{{ statistics.gb28181 }}</div>
              <div class="stat-label">GB28181 通道</div>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-content">
            <div class="stat-icon onvif">🎥</div>
            <div class="stat-info">
              <div class="stat-value">{{ statistics.onvif }}</div>
              <div class="stat-label">ONVIF 通道</div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 通道列表 -->
    <el-card shadow="hover" class="channels-card">
      <template #header>
        <div class="card-header">
          <span class="title">📺 通道列表</span>
          <div class="header-actions">
            <el-button type="primary" :icon="Plus" @click="showAddChannelDialog">
              添加通道
            </el-button>
            <el-button type="success" :icon="Refresh" @click="fetchChannels" :loading="loading">
              刷新
            </el-button>
          </div>
        </div>
      </template>

      <el-table :data="channels" style="width: 100%" v-loading="loading" empty-text="暂无通道">
        <el-table-column prop="channelId" label="通道ID" width="180">
          <template #default="{ row }">
            <span style="font-family: monospace;">{{ row.channelId }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="channelName" label="通道名称" width="150">
          <template #default="{ row }">
            <span class="channel-name">{{ row.channelName || row.name || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="deviceId" label="所属设备" width="180">
          <template #default="{ row }">
            <el-tooltip :content="row.deviceId" placement="top">
              <span style="font-family: monospace; font-size: 12px;">
                {{ row.deviceId?.slice(0, 12) }}...
              </span>
            </el-tooltip>
          </template>
        </el-table-column>
        <el-table-column prop="deviceType" label="设备类型" width="120">
          <template #default="{ row }">
            <el-tag :type="row.deviceType === 'gb28181' ? 'primary' : 'success'" size="small">
              {{ row.deviceType === 'gb28181' ? 'GB28181' : 'ONVIF' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.status === 'online' || row.status === 'ON' ? 'success' : 'info'" size="small">
              {{ row.status === 'online' || row.status === 'ON' ? '在线' : '离线' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="recording" label="录像" width="90">
          <template #default="{ row }">
            <el-tag :type="isRecording(row) ? 'danger' : 'info'" size="small">
              <span v-if="isRecording(row)" class="recording-indicator">● 录像中</span>
              <span v-else>未录像</span>
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="streamUrl" label="流地址" show-overflow-tooltip>
          <template #default="{ row }">
            <span class="stream-url">{{ row.streamUrl || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="420" fixed="right">
          <template #default="{ row }">
            <el-button 
              type="primary" 
              link 
              size="small" 
              @click="previewChannel(row)"
            >
              预览
            </el-button>
            <el-button 
              :type="isRecording(row) ? 'danger' : 'success'" 
              link 
              size="small" 
              @click="toggleRecording(row)"
              :loading="row.recordingLoading"
            >
              {{ isRecording(row) ? '停止录像' : '开始录像' }}
            </el-button>
            <el-button 
              :type="row.aiRecording ? 'warning' : 'info'" 
              link 
              size="small" 
              @click="toggleAIRecording(row)"
              :loading="row.aiRecordingLoading"
            >
              {{ row.aiRecording ? '停止AI录像' : 'AI录像' }}
            </el-button>
            <el-button 
              type="warning" 
              link 
              size="small" 
              @click="copyStreamUrl(row)"
            >
              复制地址
            </el-button>
            <el-button 
              type="danger" 
              link 
              size="small" 
              @click="deleteChannel(row)"
            >
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 添加通道对话框 -->
    <el-dialog v-model="addChannelDialogVisible" title="添加通道" width="500px">
      <el-form :model="newChannel" label-width="100px">
        <el-form-item label="通道名称" required>
          <el-input v-model="newChannel.channelName" placeholder="请输入通道名称" />
        </el-form-item>
        <el-form-item label="设备类型" required>
          <el-select v-model="newChannel.deviceType" placeholder="请选择设备类型" style="width: 100%;">
            <el-option label="GB28181" value="gb28181" />
            <el-option label="ONVIF" value="onvif" />
          </el-select>
        </el-form-item>
        <el-form-item label="所属设备" required>
          <el-select v-model="newChannel.deviceId" placeholder="请选择设备" style="width: 100%;">
            <el-option 
              v-for="device in availableDevices" 
              :key="device.deviceId || device.id" 
              :label="device.name || device.deviceId || device.id" 
              :value="device.deviceId || device.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item v-if="newChannel.deviceType === 'gb28181'" label="通道号">
          <el-input v-model="newChannel.channel" placeholder="请输入通道号" />
        </el-form-item>
        <el-form-item v-if="newChannel.deviceType === 'onvif'" label="Profile">
          <el-input v-model="newChannel.profileToken" placeholder="请输入Profile Token" />
        </el-form-item>
        <el-form-item label="流地址">
          <el-input v-model="newChannel.streamUrl" placeholder="RTSP流地址（可选）" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="addChannelDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="addChannel" :loading="addLoading">添加</el-button>
      </template>
    </el-dialog>

    <!-- 预览对话框 -->
    <el-dialog 
      v-model="previewDialogVisible" 
      :title="`预览: ${selectedChannel?.channelName || selectedChannel?.channelId}`" 
      width="900px"
      @close="stopPreview"
    >
      <div class="preview-container">
        <!-- 视频播放器 -->
        <div class="video-player-wrapper">
          <video ref="videoPlayer" class="video-player" controls autoplay muted></video>
          <div v-if="previewLoading" class="video-loading">
            <el-icon class="is-loading"><Refresh /></el-icon>
            <span>正在加载...</span>
          </div>
          <div v-if="previewError" class="video-error">
            <el-icon><WarningFilled /></el-icon>
            <span>{{ previewError }}</span>
            <el-button type="primary" size="small" @click="retryPreview">重试</el-button>
          </div>
        </div>
        
        <!-- 播放控制 -->
        <div class="preview-controls">
          <el-button-group>
            <el-button :type="playType === 'flv' ? 'primary' : 'default'" @click="playStream('flv')">
              HTTP-FLV
            </el-button>
            <el-button :type="playType === 'hls' ? 'primary' : 'default'" @click="playStream('hls')">
              HLS
            </el-button>
          </el-button-group>
          <el-button type="danger" @click="stopPreview">停止播放</el-button>
        </div>
        
        <!-- 播放地址列表 -->
        <div class="preview-urls">
          <el-descriptions :column="2" border size="small">
            <el-descriptions-item label="HTTP-FLV">
              <div class="url-item">
                <span class="url-text">{{ previewInfo.httpFlv }}</span>
                <el-button type="primary" link size="small" @click="copyUrl(previewInfo.httpFlv)">复制</el-button>
              </div>
            </el-descriptions-item>
            <el-descriptions-item label="HLS">
              <div class="url-item">
                <span class="url-text">{{ previewInfo.hls }}</span>
                <el-button type="primary" link size="small" @click="copyUrl(previewInfo.hls)">复制</el-button>
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
        
        <!-- 通道信息 -->
        <div class="channel-details">
          <el-descriptions :column="3" border size="small" title="通道信息">
            <el-descriptions-item label="通道ID">
              <span style="font-family: monospace;">{{ selectedChannel?.channelId }}</span>
            </el-descriptions-item>
            <el-descriptions-item label="通道名称">{{ selectedChannel?.channelName }}</el-descriptions-item>
            <el-descriptions-item label="设备类型">
              <el-tag :type="selectedChannel?.deviceType === 'gb28181' ? 'primary' : 'success'" size="small">
                {{ selectedChannel?.deviceType === 'gb28181' ? 'GB28181' : 'ONVIF' }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="设备ID">
              <span style="font-family: monospace; font-size: 12px;">{{ selectedChannel?.deviceId }}</span>
            </el-descriptions-item>
            <el-descriptions-item label="状态">
              <el-tag :type="selectedChannel?.status === 'online' ? 'success' : 'info'" size="small">
                {{ selectedChannel?.status === 'online' ? '在线' : '离线' }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="源地址">
              <span style="font-size: 12px;">{{ selectedChannel?.streamUrl || '-' }}</span>
            </el-descriptions-item>
          </el-descriptions>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Refresh, WarningFilled } from '@element-plus/icons-vue'

interface Channel {
  channelId: string
  channelName?: string
  name?: string
  deviceId: string
  deviceType: string
  status: string
  streamUrl?: string
  channel?: string
  profileToken?: string
}

interface Device {
  deviceId?: string
  id?: string
  name?: string
  status?: string
}

const channels = ref<Channel[]>([])
const selectedChannel = ref<Channel | null>(null)
const loading = ref(false)
const addLoading = ref(false)
const addChannelDialogVisible = ref(false)
const previewDialogVisible = ref(false)
const previewLoading = ref(false)
const previewError = ref('')
const playType = ref<'flv' | 'hls'>('flv')

// 录像状态管理
const recordingChannels = ref<Set<string>>(new Set())

// 统计信息
const statistics = computed(() => {
  const total = channels.value.length
  const online = channels.value.filter(c => c.status === 'online' || c.status === 'ON').length
  const gb28181 = channels.value.filter(c => c.deviceType === 'gb28181').length
  const onvif = channels.value.filter(c => c.deviceType === 'onvif').length
  return { total, online, gb28181, onvif }
})

const newChannel = ref({
  channelName: '',
  deviceType: 'gb28181',
  deviceId: '',
  channel: '',
  profileToken: '',
  streamUrl: ''
})

const gb28181Devices = ref<Device[]>([])
const onvifDevices = ref<Device[]>([])

const availableDevices = computed(() => {
  if (newChannel.value.deviceType === 'gb28181') {
    return gb28181Devices.value
  } else {
    return onvifDevices.value
  }
})

// 预览信息
const previewInfo = reactive({
  httpFlv: '',
  hls: '',
  rtsp: '',
  rtmp: ''
})

// 视频播放器
const videoPlayer = ref<HTMLVideoElement | null>(null)
let flvPlayer: any = null

// 定时刷新
let refreshTimer: number | null = null

// 获取通道列表
const fetchChannels = async () => {
  loading.value = true
  try {
    const response = await fetch('/api/channel/list')
    const data = await response.json()
    channels.value = data.channels || []
  } catch (error) {
    console.error('获取通道列表失败:', error)
    // 尝试从 GB28181 设备获取通道
    await fetchChannelsFromDevices()
  } finally {
    loading.value = false
  }
}

// 从设备获取通道
const fetchChannelsFromDevices = async () => {
  try {
    const response = await fetch('/api/gb28181/devices')
    const data = await response.json()
    
    if (data.success && data.devices) {
      const allChannels: Channel[] = []
      for (const device of data.devices) {
        if (device.channels && device.channels.length > 0) {
          for (const ch of device.channels) {
            allChannels.push({
              channelId: ch.id || ch.channelId,
              channelName: ch.name || ch.channelName,
              deviceId: device.id || device.deviceId,
              deviceType: 'gb28181',
              status: ch.status || 'online',
              streamUrl: ch.streamUrl || ''
            })
          }
        }
      }
      channels.value = allChannels
    }
  } catch (error) {
    console.error('从设备获取通道失败:', error)
  }
}

// 获取设备列表
const fetchDevices = async () => {
  try {
    const gb28181Response = await fetch('/api/gb28181/devices')
    const gb28181Data = await gb28181Response.json()
    gb28181Devices.value = gb28181Data.devices || []
    
    const onvifResponse = await fetch('/api/onvif/devices')
    const onvifData = await onvifResponse.json()
    onvifDevices.value = onvifData.devices || []
  } catch (error) {
    console.error('获取设备列表失败:', error)
  }
}

const showAddChannelDialog = () => {
  newChannel.value = {
    channelName: '',
    deviceType: 'gb28181',
    deviceId: '',
    channel: '',
    profileToken: '',
    streamUrl: ''
  }
  addChannelDialogVisible.value = true
}

const addChannel = async () => {
  if (!newChannel.value.channelName || !newChannel.value.deviceId) {
    ElMessage.warning('请填写必要信息')
    return
  }

  addLoading.value = true
  try {
    const response = await fetch('/api/channel/add', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(newChannel.value)
    })
    const data = await response.json()
    
    if (data.success) {
      ElMessage.success('通道添加成功')
      addChannelDialogVisible.value = false
      fetchChannels()
    } else {
      ElMessage.error(data.error || '通道添加失败')
    }
  } catch (error) {
    ElMessage.error('通道添加失败')
    console.error('添加通道失败:', error)
  } finally {
    addLoading.value = false
  }
}

const deleteChannel = async (channel: Channel) => {
  try {
    await ElMessageBox.confirm(`确定删除通道 ${channel.channelName || channel.channelId} 吗?`, '确认删除', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
  } catch {
    return
  }

  try {
    const response = await fetch(`/api/channel/${channel.channelId}`, {
      method: 'DELETE'
    })
    const data = await response.json()
    
    if (data.success) {
      ElMessage.success('通道删除成功')
      fetchChannels()
    } else {
      ElMessage.error(data.error || '通道删除失败')
    }
  } catch (error) {
    ElMessage.error('通道删除失败')
  }
}

// 判断通道是否正在录像
const isRecording = (channel: Channel) => {
  return recordingChannels.value.has(channel.channelId)
}

// 切换录像状态
const toggleRecording = async (channel: Channel) => {
  const channelId = channel.channelId
  const currentlyRecording = isRecording(channel)
  
  // 设置加载状态
  ;(channel as any).recordingLoading = true
  
  try {
    const action = currentlyRecording ? 'stop' : 'start'
    const response = await fetch(`/api/channel/${channelId}/recording/${action}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' }
    })
    
    const data = await response.json()
    
    if (data.success) {
      if (currentlyRecording) {
        recordingChannels.value.delete(channelId)
        ElMessage.success('录像已停止')
      } else {
        recordingChannels.value.add(channelId)
        ElMessage.success('录像已开始')
      }
      // 触发响应式更新
      recordingChannels.value = new Set(recordingChannels.value)
    } else {
      ElMessage.error(data.error || `${currentlyRecording ? '停止' : '开始'}录像失败`)
    }
  } catch (error) {
    console.error('录像操作失败:', error)
    ElMessage.error(`${currentlyRecording ? '停止' : '开始'}录像失败`)
  } finally {
    ;(channel as any).recordingLoading = false
  }
}

// AI录像控制
const toggleAIRecording = async (channel: Channel) => {
  const channelId = channel.channelId
  const currentlyAIRecording = (channel as any).aiRecording || false
  
  // 设置加载状态
  ;(channel as any).aiRecordingLoading = true
  
  try {
    const action = currentlyAIRecording ? 'stop' : 'start'
    const endpoint = `/api/ai/recording/${action}`
    
    const response = await fetch(endpoint, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        channel_id: channelId,
        stream_url: channel.streamUrl || `rtsp://localhost:8554/live/${channelId}`,
        mode: 'person'
      })
    })
    
    // 检查HTTP状态
    if (!response.ok) {
      const errorText = await response.text()
      if (response.status === 503) {
        ElMessage.error('AI功能未启用，请在设置中开启AI录像功能')
      } else {
        ElMessage.error(errorText || `HTTP ${response.status}: ${currentlyAIRecording ? '停止' : '启动'}AI录像失败`)
      }
      return
    }
    
    const data = await response.json()
    
    if (data.success) {
      ;(channel as any).aiRecording = !currentlyAIRecording
      ElMessage.success(currentlyAIRecording ? 'AI录像已停止' : 'AI录像已启动')
    } else {
      ElMessage.error(data.error || `${currentlyAIRecording ? '停止' : '启动'}AI录像失败`)
    }
  } catch (error) {
    console.error('AI录像操作失败:', error)
    ElMessage.error(`${currentlyAIRecording ? '停止' : '启动'}AI录像失败`)
  } finally {
    ;(channel as any).aiRecordingLoading = false
  }
}

// 获取所有通道的录像状态
const fetchRecordingStatus = async () => {
  for (const channel of channels.value) {
    try {
      const response = await fetch(`/api/channel/${channel.channelId}/recording/status`)
      const data = await response.json()
      if (data.success && data.isRecording) {
        recordingChannels.value.add(channel.channelId)
      }
    } catch (error) {
      // 忽略错误
    }
  }
  // 触发响应式更新
  recordingChannels.value = new Set(recordingChannels.value)
}

// 预览通道
const previewChannel = async (channel: Channel) => {
  selectedChannel.value = channel
  previewError.value = ''
  previewLoading.value = true
  
  previewDialogVisible.value = true
  
  try {
    // 先尝试使用测试预览 API（流代理方式，使用公共测试流）
    let response = await fetch(`/api/gb28181/devices/${channel.deviceId}/channels/${channel.channelId}/preview/test`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' }
    })
    
    let data = await response.json()
    
    // 如果测试预览失败，尝试使用真实预览 API
    if (!data.success) {
      response = await fetch(`/api/gb28181/devices/${channel.deviceId}/channels/${channel.channelId}/preview/start`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' }
      })
      data = await response.json()
    }
    
    if (!data.success) {
      previewError.value = data.error || '请求预览失败'
      previewLoading.value = false
      return
    }
    
    // 使用返回的播放地址
    const host = window.location.hostname
    if (data.data) {
      previewInfo.httpFlv = data.data.flv_url?.replace('127.0.0.1', host) || ''
      previewInfo.hls = data.data.hls_url?.replace('127.0.0.1', host) || ''
      previewInfo.rtsp = data.data.rtsp_url?.replace('127.0.0.1', host) || ''
      previewInfo.rtmp = data.data.rtmp_url?.replace('127.0.0.1', host) || ''
    }
    
    // 等待流建立
    await new Promise(resolve => setTimeout(resolve, 1500))
    
    // 开始播放
    playStream('flv')
  } catch (error) {
    console.error('预览请求失败:', error)
    previewError.value = '请求预览失败，请检查设备连接'
    previewLoading.value = false
  }
}

// 播放流
const playStream = async (type: 'flv' | 'hls') => {
  stopPreviewPlayer()
  playType.value = type
  previewLoading.value = true
  previewError.value = ''
  
  try {
    if (type === 'flv' && videoPlayer.value) {
      // 动态导入 flv.js
      const flvjs = await import('flv.js')
      if (flvjs.default.isSupported()) {
        flvPlayer = flvjs.default.createPlayer({
          type: 'flv',
          url: previewInfo.httpFlv,
          isLive: true
        })
        flvPlayer.attachMediaElement(videoPlayer.value)
        flvPlayer.load()
        flvPlayer.play()
        
        flvPlayer.on('error', (err: any) => {
          console.error('FLV播放错误:', err)
          previewError.value = '播放失败，请检查流是否在线'
          previewLoading.value = false
        })
      } else {
        previewError.value = '当前浏览器不支持 FLV 播放'
      }
    } else if (type === 'hls' && videoPlayer.value) {
      videoPlayer.value.src = previewInfo.hls
      videoPlayer.value.play()
    }
  } catch (error) {
    console.error('播放失败:', error)
    previewError.value = '播放失败，请检查流是否在线'
  } finally {
    previewLoading.value = false
  }
}

// 停止播放器
const stopPreviewPlayer = () => {
  if (flvPlayer) {
    try {
      flvPlayer.pause()
      flvPlayer.unload()
      flvPlayer.detachMediaElement()
      flvPlayer.destroy()
    } catch (e) {}
    flvPlayer = null
  }
  if (videoPlayer.value) {
    videoPlayer.value.pause()
    videoPlayer.value.src = ''
  }
}

// 停止预览
const stopPreview = () => {
  stopPreviewPlayer()
}

// 重试预览
const retryPreview = () => {
  if (selectedChannel.value) {
    playStream('flv')
  }
}

// 复制 URL
const copyUrl = (url: string) => {
  navigator.clipboard.writeText(url).then(() => {
    ElMessage.success('已复制到剪贴板')
  }).catch(() => {
    ElMessage.error('复制失败')
  })
}

// 复制流地址
const copyStreamUrl = (channel: Channel) => {
  const host = window.location.hostname
  const url = `rtsp://${host}:8554/rtp/${channel.channelId}`
  copyUrl(url)
}

onMounted(async () => {
  await fetchChannels()
  fetchDevices()
  // 获取录像状态
  fetchRecordingStatus()
  // 每30秒刷新
  refreshTimer = window.setInterval(fetchChannels, 30000)
})

onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
  }
  stopPreviewPlayer()
})
</script>

<style scoped>
.channel-management {
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

.stat-icon.total {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}

.stat-icon.online {
  background: linear-gradient(135deg, #11998e 0%, #38ef7d 100%);
}

.stat-icon.gb28181 {
  background: linear-gradient(135deg, #4facfe 0%, #00f2fe 100%);
}

.stat-icon.onvif {
  background: linear-gradient(135deg, #fa709a 0%, #fee140 100%);
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

.stat-label {
  font-size: 14px;
  color: #909399;
  margin-top: 4px;
}

/* 通道列表卡片 */
.channels-card {
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

.channel-name {
  font-weight: 500;
  color: #303133;
}

.stream-url {
  font-family: monospace;
  font-size: 12px;
  color: #606266;
}

/* 预览相关样式 */
.preview-container {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.video-player-wrapper {
  position: relative;
  background: #000;
  border-radius: 8px;
  overflow: hidden;
}

.video-player {
  width: 100%;
  height: 400px;
  display: block;
}

.video-loading,
.video-error {
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
  text-align: center;
}

.video-loading .el-icon,
.video-error .el-icon {
  font-size: 32px;
}

.video-error {
  color: #f56c6c;
}

.preview-controls {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.preview-urls {
  margin-top: 8px;
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
  max-width: 300px;
  font-size: 12px;
  font-family: monospace;
}

.channel-details {
  margin-top: 8px;
}

/* 录像指示器闪烁动画 */
.recording-indicator {
  animation: blink 1s infinite;
}

@keyframes blink {
  0%, 50% { opacity: 1; }
  51%, 100% { opacity: 0.3; }
}
</style>
