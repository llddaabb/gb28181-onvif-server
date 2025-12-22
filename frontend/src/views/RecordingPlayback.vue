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

        <!-- AI智能录像 -->
        <el-tab-pane label="AI录像" name="ai">
          <!-- AI状态概览 -->
          <el-row :gutter="20" style="margin-bottom: 20px;">
            <el-col :span="6">
              <el-card shadow="hover" class="stat-card">
                <div class="stat-title">检测器状态</div>
                <div class="stat-value">
                  <el-tag :type="aiDetectorInfo.available ? 'success' : 'danger'" size="large">
                    {{ aiDetectorInfo.available ? '已就绪' : '未启用' }}
                  </el-tag>
                </div>
                <div v-if="aiDetectorInfo.available" class="stat-desc">
                  {{ aiDetectorInfo.backend }} / {{ aiDetectorInfo.name }}
                </div>
              </el-card>
            </el-col>
            <el-col :span="6">
              <el-card shadow="hover" class="stat-card">
                <div class="stat-title">运行中任务</div>
                <div class="stat-value">{{ Object.keys(aiRecordingStatus).length }}</div>
                <div class="stat-desc">个通道正在AI检测</div>
              </el-card>
            </el-col>
            <el-col :span="6">
              <el-card shadow="hover" class="stat-card">
                <div class="stat-title">今日检测</div>
                <div class="stat-value">{{ aiTodayStats.detections }}</div>
                <div class="stat-desc">共检测到 {{ aiTodayStats.persons }} 次人形</div>
              </el-card>
            </el-col>
            <el-col :span="6">
              <el-card shadow="hover" class="stat-card">
                <div class="stat-title">AI录像时长</div>
                <div class="stat-value">{{ aiTodayStats.recordTime }}</div>
                <div class="stat-desc">今日AI触发录像</div>
              </el-card>
            </el-col>
          </el-row>

          <!-- AI录像配置 -->
          <el-card style="margin-bottom: 20px;">
            <template #header>
              <div class="card-header">
                <span>启动AI录像</span>
                <el-button type="primary" size="small" @click="refreshAIStatus">
                  <el-icon><Refresh /></el-icon> 刷新状态
                </el-button>
              </div>
            </template>
            <el-form :inline="true" :model="aiRecordingForm">
              <el-form-item label="选择通道">
                <el-select v-model="aiRecordingForm.channelId" placeholder="请选择通道" style="width: 200px;">
                  <el-option 
                    v-for="channel in channels" 
                    :key="channel.channelId" 
                    :label="channel.channelName || channel.channelId" 
                    :value="channel.channelId"
                  />
                </el-select>
              </el-form-item>
              <el-form-item label="检测模式">
                <el-select v-model="aiRecordingForm.mode" placeholder="选择模式" style="width: 150px;">
                  <el-option label="人形检测" value="person" />
                  <el-option label="移动检测" value="motion" />
                  <el-option label="连续录像" value="continuous" />
                </el-select>
              </el-form-item>
              <el-form-item>
                <el-button type="success" @click="startAIRecording" :loading="aiLoading">
                  <el-icon><VideoPlay /></el-icon> 启动AI录像
                </el-button>
              </el-form-item>
            </el-form>
          </el-card>

          <!-- 运行中的AI录像任务 -->
          <el-table :data="aiRecordingList" style="width: 100%;">
            <el-table-column prop="channel_id" label="通道ID" width="180" />
            <el-table-column label="检测模式" width="120">
              <template #default="scope">
                <el-tag :type="getModeTagType(scope.row.mode)">
                  {{ getModeLabel(scope.row.mode) }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="录像状态" width="120">
              <template #default="scope">
                <el-tag :type="scope.row.is_recording ? 'danger' : 'info'">
                  {{ scope.row.is_recording ? '录制中' : '待检测' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="检测统计" width="180">
              <template #default="scope">
                <span>检测: {{ scope.row.stats?.TotalDetections || 0 }} 次</span><br>
                <span>人形: {{ scope.row.stats?.PersonDetections || 0 }} 次</span>
              </template>
            </el-table-column>
            <el-table-column label="录像统计" width="180">
              <template #default="scope">
                <span>会话: {{ scope.row.stats?.RecordingSessions || 0 }} 次</span><br>
                <span>时长: {{ formatDuration(scope.row.stats?.TotalRecordTime || 0) }}</span>
              </template>
            </el-table-column>
            <el-table-column label="最后检测" width="180">
              <template #default="scope">
                {{ formatDateTime(scope.row.last_detect_time) || '-' }}
              </template>
            </el-table-column>
            <el-table-column label="操作" width="150" fixed="right">
              <template #default="scope">
                <el-button type="danger" size="small" @click="stopAIRecording(scope.row.channel_id)">
                  <el-icon><VideoPause /></el-icon> 停止
                </el-button>
              </template>
            </el-table-column>
          </el-table>

          <!-- AI配置面板 -->
          <el-card style="margin-top: 20px;">
            <template #header>
              <div class="card-header">
                <span>AI检测配置</span>
                <el-button type="primary" size="small" @click="saveAIConfig" :loading="aiConfigLoading">
                  保存配置
                </el-button>
              </div>
            </template>
            <el-form :model="aiConfig" label-width="120px">
              <el-row :gutter="20">
                <el-col :span="8">
                  <el-form-item label="置信度阈值">
                    <el-slider v-model="aiConfig.confidence" :min="0.1" :max="1" :step="0.05" show-input />
                  </el-form-item>
                </el-col>
                <el-col :span="8">
                  <el-form-item label="IoU阈值">
                    <el-slider v-model="aiConfig.iouThreshold" :min="0.1" :max="1" :step="0.05" show-input />
                  </el-form-item>
                </el-col>
                <el-col :span="8">
                  <el-form-item label="检测线程数">
                    <el-input-number v-model="aiConfig.numThreads" :min="1" :max="16" />
                  </el-form-item>
                </el-col>
              </el-row>
              <el-row :gutter="20">
                <el-col :span="8">
                  <el-form-item label="检测间隔(秒)">
                    <el-input-number v-model="aiConfig.detectInterval" :min="1" :max="60" />
                  </el-form-item>
                </el-col>
                <el-col :span="8">
                  <el-form-item label="录像延迟(秒)">
                    <el-input-number v-model="aiConfig.recordDelay" :min="5" :max="300" />
                  </el-form-item>
                </el-col>
                <el-col :span="8">
                  <el-form-item label="自动启动">
                    <el-switch v-model="aiConfig.autoStart" />
                  </el-form-item>
                </el-col>
              </el-row>
            </el-form>
          </el-card>
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
import { ref, computed, onMounted, onUnmounted } from 'vue'
import axios from 'axios'
import { ElMessage } from 'element-plus'
import { Refresh, VideoPlay, VideoPause, Download, CopyDocument } from '@element-plus/icons-vue'

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
  streamURL?: string
}

interface Device {
  deviceId: string
  name: string
  status: string
}

interface AIRecordingStatus {
  channel_id: string
  mode: string
  is_recording: boolean
  last_detect_time: string
  last_person_time: string
  stats: {
    TotalDetections: number
    PersonDetections: number
    RecordingSessions: number
    TotalRecordTime: number
  }
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

// AI录像相关
const aiLoading = ref(false)
const aiConfigLoading = ref(false)
const aiRecordingStatus = ref<Record<string, AIRecordingStatus>>({})
const aiDetectorInfo = ref({
  available: false,
  name: '',
  backend: '',
  inputSize: 0,
  confidence: 0,
  iouThreshold: 0
})
const aiRecordingForm = ref({
  channelId: '',
  mode: 'person'
})
const aiConfig = ref({
  confidence: 0.5,
  iouThreshold: 0.45,
  numThreads: 4,
  detectInterval: 2,
  recordDelay: 10,
  autoStart: false
})
const aiTodayStats = ref({
  detections: 0,
  persons: 0,
  recordTime: '0分钟'
})
let aiStatusTimer: number | null = null

// 计算AI录像列表
const aiRecordingList = computed(() => {
  return Object.values(aiRecordingStatus.value)
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

// AI录像相关方法
const fetchAIDetectorInfo = async () => {
  try {
    const response = await axios.get('http://localhost:9080/api/ai/detector/info')
    if (response.data.success) {
      aiDetectorInfo.value = response.data.info || { available: false }
    }
  } catch (error) {
    console.error('获取AI检测器信息失败:', error)
    aiDetectorInfo.value = { available: false, name: '', backend: '', inputSize: 0, confidence: 0, iouThreshold: 0 }
  }
}

const fetchAIConfig = async () => {
  try {
    const response = await axios.get('http://localhost:9080/api/ai/config')
    if (response.data.success && response.data.config) {
      const cfg = response.data.config
      aiConfig.value = {
        confidence: cfg.Confidence || 0.5,
        iouThreshold: cfg.IoUThreshold || 0.45,
        numThreads: cfg.NumThreads || 4,
        detectInterval: cfg.DetectInterval || 2,
        recordDelay: cfg.RecordDelay || 10,
        autoStart: cfg.AutoStart || false
      }
    }
  } catch (error) {
    console.error('获取AI配置失败:', error)
  }
}

const saveAIConfig = async () => {
  aiConfigLoading.value = true
  try {
    const response = await axios.put('http://localhost:9080/api/ai/config', {
      Enable: true,
      Confidence: aiConfig.value.confidence,
      IoUThreshold: aiConfig.value.iouThreshold,
      NumThreads: aiConfig.value.numThreads,
      DetectInterval: aiConfig.value.detectInterval,
      RecordDelay: aiConfig.value.recordDelay,
      AutoStart: aiConfig.value.autoStart
    })
    if (response.data.success) {
      ElMessage.success('AI配置保存成功')
    } else {
      ElMessage.error('保存失败')
    }
  } catch (error) {
    ElMessage.error('保存AI配置失败')
    console.error('保存AI配置失败:', error)
  } finally {
    aiConfigLoading.value = false
  }
}

const refreshAIStatus = async () => {
  try {
    const response = await axios.get('http://localhost:9080/api/ai/recording/status/all')
    if (response.data.success) {
      aiRecordingStatus.value = response.data.status || {}
      
      // 计算今日统计
      let totalDetections = 0
      let totalPersons = 0
      let totalRecordTime = 0
      
      Object.values(aiRecordingStatus.value).forEach((status: any) => {
        if (status.stats) {
          totalDetections += status.stats.TotalDetections || 0
          totalPersons += status.stats.PersonDetections || 0
          totalRecordTime += status.stats.TotalRecordTime || 0
        }
      })
      
      aiTodayStats.value = {
        detections: totalDetections,
        persons: totalPersons,
        recordTime: formatDuration(totalRecordTime)
      }
    }
  } catch (error) {
    console.error('获取AI录像状态失败:', error)
  }
}

const startAIRecording = async () => {
  if (!aiRecordingForm.value.channelId) {
    ElMessage.warning('请选择通道')
    return
  }

  aiLoading.value = true
  try {
    const response = await axios.post('http://localhost:9080/api/ai/recording/start', {
      channel_id: aiRecordingForm.value.channelId,
      mode: aiRecordingForm.value.mode
    })
    
    if (response.data.success) {
      ElMessage.success(`AI录像已启动: ${aiRecordingForm.value.channelId}`)
      aiRecordingForm.value.channelId = ''
      await refreshAIStatus()
    } else {
      ElMessage.error(response.data.error || '启动失败')
    }
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '启动AI录像失败')
    console.error('启动AI录像失败:', error)
  } finally {
    aiLoading.value = false
  }
}

const stopAIRecording = async (channelId: string) => {
  try {
    const response = await axios.post('http://localhost:9080/api/ai/recording/stop', {
      channel_id: channelId
    })
    
    if (response.data.success) {
      ElMessage.success(`AI录像已停止: ${channelId}`)
      await refreshAIStatus()
    } else {
      ElMessage.error(response.data.error || '停止失败')
    }
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '停止AI录像失败')
    console.error('停止AI录像失败:', error)
  }
}

const getModeLabel = (mode: string) => {
  const labels: Record<string, string> = {
    person: '人形检测',
    motion: '移动检测',
    continuous: '连续录像',
    manual: '手动模式'
  }
  return labels[mode] || mode
}

const getModeTagType = (mode: string) => {
  const types: Record<string, string> = {
    person: 'success',
    motion: 'warning',
    continuous: 'primary',
    manual: 'info'
  }
  return types[mode] || 'info'
}

const formatDuration = (nanoseconds: number) => {
  // Go的time.Duration是纳秒
  const seconds = Math.floor(nanoseconds / 1e9)
  if (seconds < 60) return `${seconds}秒`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}分${seconds % 60}秒`
  const hours = Math.floor(minutes / 60)
  return `${hours}时${minutes % 60}分`
}

const formatDateTime = (dateStr: string) => {
  if (!dateStr || dateStr === '0001-01-01T00:00:00Z') return ''
  try {
    const date = new Date(dateStr)
    return date.toLocaleString('zh-CN')
  } catch {
    return dateStr
  }
}

onMounted(() => {
  fetchDevices()
  fetchChannels()
  fetchAIDetectorInfo()
  fetchAIConfig()
  refreshAIStatus()
  
  // 定时刷新AI状态
  aiStatusTimer = window.setInterval(() => {
    if (activeTab.value === 'ai') {
      refreshAIStatus()
    }
  }, 5000)
})

onUnmounted(() => {
  if (aiStatusTimer) {
    clearInterval(aiStatusTimer)
  }
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

/* AI录像相关样式 */
.stat-card {
  text-align: center;
  padding: 10px;
}

.stat-card .stat-title {
  font-size: 14px;
  color: #909399;
  margin-bottom: 10px;
}

.stat-card .stat-value {
  font-size: 28px;
  font-weight: bold;
  color: #303133;
  margin-bottom: 5px;
}

.stat-card .stat-desc {
  font-size: 12px;
  color: #909399;
}
</style>
