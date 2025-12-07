<template>
  <div class="playback-container">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>录像回放</span>
        </div>
      </template>

      <el-tabs v-model="activeTab" type="card">
        <!-- 设备录像查询 -->
        <el-tab-pane label="设备录像" name="device">
          <el-row :gutter="20" class="query-section">
            <el-col :span="6">
              <label>选择设备</label>
              <el-select v-model="selectedDevice" placeholder="请选择设备">
                <el-option-group label="GB28181设备">
                  <el-option 
                    v-for="device in gb28181Devices" 
                    :key="device.deviceId" 
                    :label="device.name || device.deviceId" 
                    :value="device.deviceId"
                  />
                </el-option-group>
                <el-option-group label="ONVIF设备">
                  <el-option 
                    v-for="device in onvifDevices" 
                    :key="device.deviceId" 
                    :label="device.name || device.deviceId" 
                    :value="device.deviceId"
                  />
                </el-option-group>
              </el-select>
            </el-col>
            <el-col :span="6">
              <label>选择通道</label>
              <el-select v-model="selectedChannel" placeholder="请选择通道">
                <el-option 
                  v-for="channel in availableChannels" 
                  :key="channel.channelId" 
                  :label="channel.channelName" 
                  :value="channel.channelId"
                />
              </el-select>
            </el-col>
            <el-col :span="6">
              <label>查询日期</label>
              <el-date-picker 
                v-model="queryDate" 
                type="date" 
                placeholder="选择日期"
                style="width: 100%"
              />
            </el-col>
            <el-col :span="6">
              <el-button type="primary" @click="queryRecordings">查询录像</el-button>
            </el-col>
          </el-row>

          <!-- 录像列表 -->
          <el-table :data="recordings" style="width: 100%; margin-top: 20px;">
            <el-table-column prop="recordingId" label="录像ID" width="150" />
            <el-table-column prop="channelName" label="通道" width="150" />
            <el-table-column prop="startTime" label="开始时间" width="180" />
            <el-table-column prop="endTime" label="结束时间" width="180" />
            <el-table-column prop="duration" label="时长" width="100" />
            <el-table-column prop="fileSize" label="文件大小" width="120" />
            <el-table-column prop="status" label="状态" width="100">
              <template #default="scope">
                <el-tag :type="scope.row.status === 'complete' ? 'success' : 'warning'">
                  {{ scope.row.status === 'complete' ? '完整' : '进行中' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="200" fixed="right">
              <template #default="scope">
                <el-button 
                  type="primary" 
                  size="small" 
                  @click="playbackRecording(scope.row)"
                >
                  回放
                </el-button>
                <el-button 
                  type="info" 
                  size="small" 
                  @click="downloadRecording(scope.row)"
                >
                  下载
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>

        <!-- ZLM录像列表 -->
        <el-tab-pane label="ZLM录像" name="zlm">
          <el-row :gutter="20" class="query-section">
            <el-col :span="6">
              <label>选择通道</label>
              <el-select v-model="zlmSelectedChannel" placeholder="请选择通道">
                <el-option 
                  v-for="channel in channels" 
                  :key="channel.channelId" 
                  :label="channel.channelName || channel.channelId" 
                  :value="channel.channelId"
                />
              </el-select>
            </el-col>
            <el-col :span="6">
              <label>查询日期</label>
              <el-date-picker 
                v-model="zlmQueryDate" 
                type="date" 
                placeholder="选择日期"
                style="width: 100%"
              />
            </el-col>
            <el-col :span="6">
              <el-button type="primary" @click="queryZLMRecordings" :loading="zlmLoading">
                查询录像
              </el-button>
              <el-button @click="clearZLMQuery">清空</el-button>
            </el-col>
          </el-row>

          <el-table :data="zlmRecordings" style="width: 100%; margin-top: 20px;">
            <el-table-column prop="recordingId" label="录像ID" width="180" show-overflow-tooltip />
            <el-table-column prop="startTime" label="开始时间" width="180" />
            <el-table-column prop="endTime" label="结束时间" width="180" />
            <el-table-column prop="duration" label="时长" width="100" />
            <el-table-column prop="fileSize" label="文件大小" width="120" />
            <el-table-column prop="status" label="状态" width="100">
              <template #default="scope">
                <el-tag :type="scope.row.status === 'complete' ? 'success' : 'warning'">
                  {{ scope.row.status === 'complete' ? '完整' : scope.row.status === 'recording' ? '录制中' : '进行中' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="200" fixed="right">
              <template #default="scope">
                <el-button 
                  type="primary" 
                  size="small" 
                  @click="playZLMRecording(scope.row)"
                >
                  回放
                </el-button>
                <el-button 
                  type="success" 
                  size="small" 
                  @click="downloadZLMRecording(scope.row)"
                >
                  下载
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>
      </el-tabs>
    </el-card>

    <!-- 回放窗口 -->
    <el-dialog 
      v-model="playbackDialogVisible" 
      :title="`录像回放 - ${selectedRecording?.channelName || currentPlayback.fileName}`" 
      width="80%"
    >
      <div class="playback-content">
        <div class="video-player">
          <video 
            v-if="currentPlayback.playUrl"
            ref="videoPlayer"
            :src="currentPlayback.playUrl" 
            controls 
            autoplay
            style="width: 100%; max-height: 500px; background: #000;"
          />
          <video 
            v-else-if="selectedRecording?.playbackUrl"
            ref="videoPlayer"
            :src="selectedRecording?.playbackUrl" 
            controls 
            style="width: 100%; max-height: 500px; background: #000;"
          />
        </div>
        <div v-if="selectedRecording" class="playback-info" style="margin-top: 20px;">
          <el-descriptions :column="4" border>
            <el-descriptions-item label="录像ID">{{ selectedRecording?.recordingId }}</el-descriptions-item>
            <el-descriptions-item label="通道">{{ selectedRecording?.channelName }}</el-descriptions-item>
            <el-descriptions-item label="开始时间">{{ selectedRecording?.startTime }}</el-descriptions-item>
            <el-descriptions-item label="结束时间">{{ selectedRecording?.endTime }}</el-descriptions-item>
            <el-descriptions-item label="时长">{{ selectedRecording?.duration }}</el-descriptions-item>
            <el-descriptions-item label="文件大小">{{ selectedRecording?.fileSize }}</el-descriptions-item>
            <el-descriptions-item label="状态">
              <el-tag :type="selectedRecording?.status === 'complete' ? 'success' : 'warning'">
                {{ selectedRecording?.status === 'complete' ? '完整' : '进行中' }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="帧率">{{ selectedRecording?.frameRate || '-' }}</el-descriptions-item>
          </el-descriptions>
        </div>
        <div v-if="currentPlayback.fileName" class="playback-info" style="margin-top: 20px;">
          <el-alert 
            v-if="currentPlayback.note"
            :title="currentPlayback.note" 
            type="warning" 
            :closable="false"
            style="margin-bottom: 15px;"
          />
          <el-descriptions :column="3" border>
            <el-descriptions-item label="文件名">{{ currentPlayback.fileName }}</el-descriptions-item>
            <el-descriptions-item label="应用">{{ currentPlayback.app }}</el-descriptions-item>
            <el-descriptions-item label="流ID">{{ currentPlayback.stream }}</el-descriptions-item>
            <el-descriptions-item label="文件大小">{{ currentPlayback.size }}</el-descriptions-item>
            <el-descriptions-item label="修改时间">{{ currentPlayback.modTime }}</el-descriptions-item>
            <el-descriptions-item label="播放地址">
              <el-link :href="currentPlayback.playUrl" target="_blank" type="primary">
                打开播放地址
              </el-link>
            </el-descriptions-item>
          </el-descriptions>
          <div style="margin-top: 15px; text-align: center;">
            <el-button type="success" @click="downloadFile(currentPlayback.playUrl, currentPlayback.fileName)">
              <el-icon><Download /></el-icon> 下载录像文件
            </el-button>
            <el-button @click="copyToClipboard(currentPlayback.playUrl)">
              <el-icon><CopyDocument /></el-icon> 复制播放地址
            </el-button>
          </div>
        </div>
        <div v-if="!currentPlayback.playUrl && selectedRecording" class="timeline-info" style="margin-top: 20px;">
          <el-slider 
            v-model="playbackProgress" 
            :max="playbackDuration" 
            range 
            marks
            style="margin-bottom: 10px;"
          />
          <div style="text-align: center;">
            <span>{{ formatTime(playbackProgress) }} / {{ formatTime(playbackDuration) }}</span>
          </div>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import axios from 'axios'
import { ElMessage } from 'element-plus'

interface Recording {
  recordingId: string
  channelId: string
  channelName: string
  startTime: string
  endTime: string
  duration: string
  fileSize: string
  status: string
  playbackUrl: string
  frameRate?: string
}

interface ZLMRecording {
  fileName: string
  app: string
  stream: string
  size: string
  modTime: string
}

interface Channel {
  channelId: string
  channelName: string
  deviceId: string
}

interface Device {
  deviceId: string
  name: string
  status: string
}

const activeTab = ref('device')
const selectedDevice = ref('')
const selectedChannel = ref('')
const queryDate = ref(new Date())
const recordings = ref<Recording[]>([])
const selectedRecording = ref<Recording | null>(null)
const playbackDialogVisible = ref(false)
const playbackProgress = ref(0)
const playbackDuration = ref(0)

const gb28181Devices = ref<Device[]>([])
const onvifDevices = ref<Device[]>([])
const channels = ref<Channel[]>([])

// ZLM录像相关
const zlmRecordings = ref<ZLMRecording[]>([])
const zlmRecordPath = ref('')
const zlmLoading = ref(false)
const zlmSelectedChannel = ref('')
const zlmQueryDate = ref(new Date())
const currentPlayback = ref({
  fileName: '',
  app: '',
  stream: '',
  size: '',
  modTime: '',
  playUrl: '',
  note: ''
})

const availableChannels = computed(() => {
  if (!selectedDevice.value) return []
  return channels.value.filter(ch => ch.deviceId === selectedDevice.value)
})

const fetchDevices = async () => {
  try {
    const gb28181Response = await axios.get('http://localhost:9080/api/gb28181/devices')
    gb28181Devices.value = gb28181Response.data.devices || []
    
    const onvifResponse = await axios.get('http://localhost:9080/api/onvif/devices')
    onvifDevices.value = onvifResponse.data.devices || []
  } catch (error) {
    console.error('获取设备列表失败:', error)
  }
}

const fetchChannels = async () => {
  try {
    const response = await axios.get('http://localhost:9080/api/channel/list')
    channels.value = response.data.channels || []
  } catch (error) {
    console.error('获取通道列表失败:', error)
  }
}

const queryRecordings = async () => {
  if (!selectedDevice.value || !selectedChannel.value) {
    ElMessage.warning('请选择设备和通道')
    return
  }

  try {
    const dateStr = queryDate.value instanceof Date 
      ? queryDate.value.toISOString().split('T')[0]
      : queryDate.value

    const response = await axios.get('http://localhost:9080/api/recording/query', {
      params: {
        deviceId: selectedDevice.value,
        channelId: selectedChannel.value,
        date: dateStr
      }
    })
    
    recordings.value = response.data.recordings || []
  } catch (error) {
    ElMessage.error('查询录像失败')
    console.error('查询录像失败:', error)
  }
}

const playbackRecording = (recording: Recording) => {
  selectedRecording.value = recording
  playbackDialogVisible.value = true
  playbackProgress.value = 0
  // 简单估算时长（应该从后端获取）
  playbackDuration.value = 3600 // 假设1小时
}

const downloadRecording = async (recording: Recording) => {
  try {
    const response = await axios.get(`http://localhost:9080/api/recording/${recording.recordingId}/download`, {
      responseType: 'blob'
    })
    
    const url = window.URL.createObjectURL(response.data)
    const link = document.createElement('a')
    link.href = url
    link.download = `recording_${recording.recordingId}.mp4`
    link.click()
    window.URL.revokeObjectURL(url)
    
    ElMessage.success('下载开始')
  } catch (error) {
    ElMessage.error('下载失败')
    console.error('下载失败:', error)
  }
}

const formatTime = (seconds: number) => {
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const secs = Math.floor(seconds % 60)
  return `${hours.toString().padStart(2, '0')}:${minutes.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`
}

// ZLM录像功能
const queryZLMRecordings = async () => {
  if (!zlmSelectedChannel.value) {
    ElMessage.warning('请选择通道')
    return
  }

  zlmLoading.value = true
  try {
    const dateStr = zlmQueryDate.value instanceof Date 
      ? zlmQueryDate.value.toISOString().split('T')[0]
      : zlmQueryDate.value

    const response = await axios.get('http://localhost:9080/api/recording/zlm/list', {
      params: {
        channelId: zlmSelectedChannel.value,
        date: dateStr,
        app: 'live'
      }
    })
    
    if (response.data.success) {
      zlmRecordings.value = response.data.recordings || []
      zlmRecordPath.value = response.data.recordPath || ''
      ElMessage.success(`查询到 ${response.data.total} 个录像片段`)
    } else {
      ElMessage.error('查询录像失败')
    }
  } catch (error) {
    ElMessage.error('查询ZLM录像失败')
    console.error('查询ZLM录像失败:', error)
  } finally {
    zlmLoading.value = false
  }
}

const clearZLMQuery = () => {
  zlmSelectedChannel.value = ''
  zlmQueryDate.value = new Date()
  zlmRecordings.value = []
}

const playZLMRecording = async (recording: any) => {
  try {
    const response = await axios.get(
      `http://localhost:9080/api/recording/zlm/play/${recording.app}/${recording.stream}/${recording.fileName}`
    )
    
    if (response.data.success) {
      currentPlayback.value = {
        fileName: recording.fileName,
        app: recording.app,
        stream: recording.stream,
        size: recording.fileSize,
        modTime: recording.modTime,
        playUrl: response.data.playUrl || response.data.mp4Url,
        note: response.data.note || ''
      }
      selectedRecording.value = null
      playbackDialogVisible.value = true
    } else {
      ElMessage.error('获取播放地址失败')
    }
  } catch (error) {
    ElMessage.error('播放失败')
    console.error('播放失败:', error)
  }
}

const downloadZLMRecording = async (recording: any) => {
  try {
    const response = await axios.get(
      `http://localhost:9080/api/recording/zlm/play/${recording.app}/${recording.stream}/${recording.fileName}`
    )
    
    if (response.data.success) {
      const downloadUrl = response.data.playUrl || response.data.mp4Url
      downloadFile(downloadUrl, recording.fileName)
    } else {
      ElMessage.error('获取下载地址失败')
    }
  } catch (error) {
    ElMessage.error('下载失败')
    console.error('下载失败:', error)
  }
}

const downloadFile = (url: string, fileName: string) => {
  const link = document.createElement('a')
  link.href = url
  link.download = fileName
  link.target = '_blank'
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  ElMessage.success('开始下载')
}

const copyToClipboard = async (text: string) => {
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success('已复制到剪贴板')
  } catch (error) {
    ElMessage.error('复制失败')
  }
}

onMounted(() => {
  fetchDevices()
  fetchChannels()
})
</script>

<style scoped>
.playback-container {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.query-section {
  padding: 15px;
  background-color: #f5f7fa;
  border-radius: 4px;
  margin-bottom: 20px;
}

.query-section label {
  display: block;
  margin-bottom: 8px;
  font-weight: 500;
}

.playback-content {
  padding: 20px 0;
}

.video-player {
  width: 100%;
  background: #000;
  border-radius: 4px;
  overflow: hidden;
}

.playback-info {
  background-color: #f5f7fa;
  padding: 15px;
  border-radius: 4px;
}

.timeline-info {
  background-color: #f5f7fa;
  padding: 15px;
  border-radius: 4px;
}
</style>
