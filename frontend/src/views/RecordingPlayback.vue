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
                  :key="channel.channelId || channel.ChannelID" 
                  :label="channel.channelName || channel.name || channel.Name || channel.channelId || channel.ChannelID" 
                  :value="channel.channelId || channel.ChannelID"
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
              <el-button type="primary" @click="queryRecordings" :loading="deviceRecordingLoading">
                查询录像
              </el-button>
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
              <el-select v-model="zlmSelectedChannel" placeholder="请选择通道" @change="onZlmChannelChange">
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
                :disabled-date="disabledDate"
                :cell-class-name="getDateCellClass"
                @panel-change="onCalendarPanelChange"
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
      :title="`录像回放 - ${currentPlayback.fileName || selectedRecording?.channelName || '未知'}`" 
      width="80%"
      :close-on-click-modal="false"
      @close="onPlaybackDialogClose"
    >
      <div class="playback-content">
        <!-- 使用 PlaybackPlayer 组件 -->
        <PlaybackPlayer
          v-if="playbackDialogVisible && (currentPlayback.playUrl || currentPlayback.flvUrl)"
          :play-url="currentPlayback.playUrl"
          :flv-url="currentPlayback.flvUrl"
          :mp4-url="currentPlayback.mp4Url"
          :download-url="currentPlayback.downloadUrl"
          :recording-info="currentPlayback"
          :default-height="480"
          :autoplay="true"
          @playing="onPlaybackPlaying"
          @ended="onPlaybackEnded"
          @error="onPlaybackError"
        />
        
        <!-- 录像信息 -->
        <div v-if="currentPlayback.fileName" class="playback-info" style="margin-top: 20px;">
          <el-alert 
            v-if="currentPlayback.note"
            :title="currentPlayback.note" 
            type="info" 
            :closable="false"
            style="margin-bottom: 15px;"
          />
          <el-descriptions :column="3" border>
            <el-descriptions-item label="文件名">{{ currentPlayback.fileName }}</el-descriptions-item>
            <el-descriptions-item label="应用">{{ currentPlayback.app }}</el-descriptions-item>
            <el-descriptions-item label="流ID">{{ currentPlayback.stream }}</el-descriptions-item>
            <el-descriptions-item label="文件大小">{{ currentPlayback.fileSize || currentPlayback.size }}</el-descriptions-item>
            <el-descriptions-item label="修改时间">{{ currentPlayback.modTime }}</el-descriptions-item>
            <el-descriptions-item label="播放模式">
              <el-tag :type="currentPlayback.flvUrl ? 'success' : 'info'">
                {{ currentPlayback.flvUrl ? 'FLV 流' : 'MP4 直播' }}
              </el-tag>
            </el-descriptions-item>
          </el-descriptions>
          <div style="margin-top: 15px; text-align: center;">
            <el-button type="success" @click="downloadFile(currentPlayback.downloadUrl || currentPlayback.mp4Url, currentPlayback.fileName)">
              <el-icon><Download /></el-icon> 下载录像文件
            </el-button>
            <el-button @click="copyToClipboard(currentPlayback.playUrl)">
              <el-icon><CopyDocument /></el-icon> 复制播放地址
            </el-button>
          </div>
        </div>
        
        <!-- 设备端录像信息 -->
        <div v-if="selectedRecording && !currentPlayback.fileName" class="playback-info" style="margin-top: 20px;">
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
            <el-descriptions-item label="类型">{{ selectedRecording?.type || '-' }}</el-descriptions-item>
          </el-descriptions>
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
import PlaybackPlayer from '../components/PlaybackPlayer.vue'

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
const zlmRecordingDates = ref<string[]>([]) // 有录像的日期列表
const zlmCalendarYear = ref(new Date().getFullYear())
const zlmCalendarMonth = ref(new Date().getMonth() + 1)
const currentPlayback = ref({
  fileName: '',
  app: '',
  stream: '',
  size: '',
  fileSize: '',
  modTime: '',
  playUrl: '',
  flvUrl: '',
  mp4Url: '',
  downloadUrl: '',
  streamKey: '',
  gb28181StreamId: '', // GB28181 回放流 ID
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
  
  // 检查是否是 GB28181 设备
  const gb28181Device = gb28181Devices.value.find(d => d.deviceId === selectedDevice.value)
  if (gb28181Device) {
    // GB28181 设备的通道 - 兼容不同字段名
    const deviceChannels = (gb28181Device as any).channels || (gb28181Device as any).Channels || []
    return deviceChannels.map((ch: any) => ({
      channelId: ch.channelId || ch.ChannelID,
      channelName: ch.name || ch.Name || ch.channelId || ch.ChannelID
    }))
  }
  
  // 检查是否是 ONVIF 设备（ONVIF 设备通常只有一个默认通道）
  const onvifDevice = onvifDevices.value.find(d => d.deviceId === selectedDevice.value)
  if (onvifDevice) {
    // 为 ONVIF 设备创建一个虚拟通道
    return [{
      channelId: selectedDevice.value,
      channelName: (onvifDevice as any).name || (onvifDevice as any).Name || selectedDevice.value
    }]
  }
  
  return []
})

// 判断设备类型
const isGB28181Device = computed(() => {
  return gb28181Devices.value.some(d => d.deviceId === selectedDevice.value)
})

const isONVIFDevice = computed(() => {
  return onvifDevices.value.some(d => d.deviceId === selectedDevice.value)
})

const fetchDevices = async () => {
  try {
    const gb28181Response = await axios.get('/api/gb28181/devices')
    gb28181Devices.value = gb28181Response.data.devices || []
    
    const onvifResponse = await axios.get('/api/onvif/devices')
    onvifDevices.value = onvifResponse.data.devices || []
  } catch (error) {
    console.error('获取设备列表失败:', error)
  }
}

const fetchChannels = async () => {
  try {
    const response = await axios.get('/api/channel/list')
    channels.value = response.data.channels || []
  } catch (error) {
    console.error('获取通道列表失败:', error)
  }
}

// 设备录像查询加载状态
const deviceRecordingLoading = ref(false)

const queryRecordings = async () => {
  if (!selectedDevice.value || !selectedChannel.value) {
    ElMessage.warning('请选择设备和通道')
    return
  }

  deviceRecordingLoading.value = true
  recordings.value = []

  try {
    const dateStr = queryDate.value instanceof Date 
      ? queryDate.value.toISOString().split('T')[0]
      : queryDate.value
    
    // 转换为 GB28181 要求的时间格式
    const startTime = `${dateStr}T00:00:00`
    const endTime = `${dateStr}T23:59:59`

    if (isGB28181Device.value) {
      // GB28181 设备录像查询 - 先发送查询请求
      await axios.get('/api/gb28181/record/query', {
        params: {
          channelId: selectedChannel.value,
          startTime: startTime,
          endTime: endTime,
          type: 'all'
        }
      })
      
      ElMessage.info('录像查询请求已发送，正在等待设备响应...')
      
      // 等待设备响应，然后获取结果（轮询最多10次，每次间隔1秒）
      let attempts = 0
      const maxAttempts = 10
      
      const pollResults = async () => {
        attempts++
        try {
          const resultResponse = await axios.get('/api/gb28181/record/list', {
            params: { channelId: selectedChannel.value }
          })
          
          if (resultResponse.data.success && resultResponse.data.count > 0) {
            // 获取到结果，转换格式
            const deviceRecords = resultResponse.data.records || []
            recordings.value = deviceRecords.map((rec: any, index: number) => ({
              recordingId: `gb28181_${selectedChannel.value}_${index}`,
              channelId: rec.channelId,
              channelName: rec.name || selectedChannel.value,
              startTime: rec.startTime,
              endTime: rec.endTime,
              duration: calculateDuration(rec.startTime, rec.endTime),
              fileSize: rec.fileSize ? formatFileSize(rec.fileSize) : '-',
              status: 'complete',
              type: rec.type || 'all',
              filePath: rec.filePath
            }))
            
            ElMessage.success(`查询到 ${recordings.value.length} 条录像`)
            deviceRecordingLoading.value = false
          } else if (attempts < maxAttempts) {
            // 继续轮询
            setTimeout(pollResults, 1000)
          } else {
            // 超时
            ElMessage.warning('未查询到录像或设备响应超时')
            deviceRecordingLoading.value = false
          }
        } catch (error) {
          if (attempts < maxAttempts) {
            setTimeout(pollResults, 1000)
          } else {
            deviceRecordingLoading.value = false
          }
        }
      }
      
      // 开始轮询
      setTimeout(pollResults, 1500)
      
    } else if (isONVIFDevice.value) {
      // ONVIF 设备录像查询
      const response = await axios.get(`/api/onvif/devices/${encodeURIComponent(selectedDevice.value)}/recordings`, {
        params: {
          startTime: startTime,
          endTime: endTime
        }
      })
      
      if (response.data.success) {
        const onvifRecords = response.data.recordings || []
        recordings.value = onvifRecords.map((rec: any, index: number) => ({
          recordingId: rec.recordingToken || `onvif_${selectedDevice.value}_${index}`,
          channelId: selectedDevice.value,
          channelName: rec.name || selectedDevice.value,
          startTime: rec.startTime ? new Date(rec.startTime).toLocaleString('zh-CN') : '-',
          endTime: rec.endTime ? new Date(rec.endTime).toLocaleString('zh-CN') : '-',
          duration: calculateDuration(rec.startTime, rec.endTime),
          fileSize: '-',
          status: 'complete',
          recordingToken: rec.recordingToken
        }))
        
        ElMessage.success(`查询到 ${recordings.value.length} 条录像`)
      } else {
        ElMessage.warning('未查询到录像')
      }
      deviceRecordingLoading.value = false
    } else {
      ElMessage.error('未知的设备类型')
      deviceRecordingLoading.value = false
    }
  } catch (error: any) {
    ElMessage.error('查询录像失败: ' + (error.response?.data?.error || error.message))
    console.error('查询录像失败:', error)
    deviceRecordingLoading.value = false
  }
}

// 计算时长
const calculateDuration = (startTime: string, endTime: string): string => {
  try {
    const start = new Date(startTime)
    const end = new Date(endTime)
    const diffMs = end.getTime() - start.getTime()
    if (isNaN(diffMs) || diffMs < 0) return '-'
    
    const hours = Math.floor(diffMs / 3600000)
    const minutes = Math.floor((diffMs % 3600000) / 60000)
    const seconds = Math.floor((diffMs % 60000) / 1000)
    
    if (hours > 0) {
      return `${hours}时${minutes}分${seconds}秒`
    } else if (minutes > 0) {
      return `${minutes}分${seconds}秒`
    } else {
      return `${seconds}秒`
    }
  } catch {
    return '-'
  }
}

// 格式化文件大小
const formatFileSize = (bytes: number): string => {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  if (bytes < 1024 * 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
  return (bytes / (1024 * 1024 * 1024)).toFixed(2) + ' GB'
}

const playbackRecording = async (recording: Recording) => {
  selectedRecording.value = recording
  
  // 重置当前回放状态
  currentPlayback.value = {
    fileName: '',
    app: '',
    stream: '',
    size: '',
    fileSize: recording.fileSize,
    modTime: '',
    playUrl: '',
    flvUrl: '',
    mp4Url: '',
    downloadUrl: '',
    streamKey: '',
    gb28181StreamId: '',
    note: ''
  }
  
  // 根据设备类型获取回放地址
  if (isGB28181Device.value) {
    // GB28181 设备录像回放 - 需要通过 INVITE 建立会话
    try {
      ElMessage.info('正在请求设备端录像回放...')
      
      const response = await axios.post('/api/gb28181/record/playback', {
        channelId: recording.channelId,
        startTime: recording.startTime,
        endTime: recording.endTime
      })
      
      if (response.data.success) {
        currentPlayback.value.gb28181StreamId = response.data.streamId || ''
        currentPlayback.value.note = `GB28181 设备端录像回放 - 正在等待设备推流（约3-5秒）`
        
        // 先打开对话框显示等待状态
        playbackDialogVisible.value = true
        playbackProgress.value = 0
        playbackDuration.value = 3600
        
        // 延迟 3 秒后再设置播放地址，给设备时间建立 RTP 连接
        ElMessage.info('等待设备推流中，请稍候...')
        await new Promise(resolve => setTimeout(resolve, 3000))
        
        // 设置播放地址（延迟后）
        currentPlayback.value.playUrl = response.data.playUrl || ''
        currentPlayback.value.flvUrl = response.data.flvUrl || response.data.wsFlvUrl || ''
        currentPlayback.value.note = `GB28181 设备端录像回放 (流ID: ${response.data.streamId || ''}, SSRC: ${response.data.ssrc || ''})`
        
        ElMessage.success('设备推流已建立，开始播放')
        return // 提前返回，避免重复打开对话框
      } else {
        ElMessage.warning('GB28181 设备端录像回放暂不支持，请直接在设备端查看')
      }
    } catch (error: any) {
      console.error('GB28181 录像回放失败:', error)
      ElMessage.warning(error.response?.data?.error || 'GB28181 设备端录像回放功能开发中')
    }
  } else if (isONVIFDevice.value) {
    // ONVIF 设备录像回放 - 通过 GetReplayUri
    if ((recording as any).recordingToken) {
      try {
        const response = await axios.get(`/api/onvif/devices/${encodeURIComponent(selectedDevice.value)}/replay-uri`, {
          params: { recordingToken: (recording as any).recordingToken }
        })
        
        if (response.data.success && response.data.replayUri) {
          // 通过 ZLM 代理 RTSP 回放流
          const proxyResponse = await axios.post('/api/stream/proxy', {
            url: response.data.replayUri,
            app: 'playback',
            stream: `onvif_${Date.now()}`
          })
          
          if (proxyResponse.data.success) {
            currentPlayback.value.playUrl = proxyResponse.data.flvUrl || proxyResponse.data.playUrl
            currentPlayback.value.flvUrl = proxyResponse.data.flvUrl || ''
            currentPlayback.value.streamKey = proxyResponse.data.key || ''
            currentPlayback.value.note = 'ONVIF 设备端录像回放'
          }
        } else {
          ElMessage.warning('无法获取 ONVIF 设备回放地址')
        }
      } catch (error) {
        console.error('ONVIF 录像回放失败:', error)
        ElMessage.warning('ONVIF 设备端录像回放功能开发中')
      }
    }
  }
  
  playbackDialogVisible.value = true
  playbackProgress.value = 0
  playbackDuration.value = 3600
}

const downloadRecording = async (recording: Recording) => {
  try {
    const response = await axios.get(`/api/recording/${recording.recordingId}/download`, {
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

    const response = await axios.get('/api/recording/zlm/list', {
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
  zlmRecordingDates.value = []
}

// 获取通道有录像的日期列表
const fetchRecordingDates = async (year?: number, month?: number) => {
  if (!zlmSelectedChannel.value) {
    zlmRecordingDates.value = []
    return
  }

  try {
    const y = year || zlmCalendarYear.value
    const m = month || zlmCalendarMonth.value
    
    const response = await axios.get('/api/recording/zlm/dates', {
      params: {
        channelId: zlmSelectedChannel.value,
        year: y,
        month: m,
        app: 'live'
      }
    })
    
    if (response.data.success) {
      zlmRecordingDates.value = response.data.dates || []
    }
  } catch (error) {
    console.error('获取录像日期失败:', error)
  }
}

// 通道选择变化时获取录像日期
const onZlmChannelChange = () => {
  fetchRecordingDates()
}

// 日历面板切换时获取新月份的录像日期
const onCalendarPanelChange = (date: Date) => {
  zlmCalendarYear.value = date.getFullYear()
  zlmCalendarMonth.value = date.getMonth() + 1
  fetchRecordingDates()
}

// 获取日期单元格的样式类
const getDateCellClass = (date: Date) => {
  const dateStr = formatDateToString(date)
  if (zlmRecordingDates.value.includes(dateStr)) {
    return 'has-recording'
  }
  return ''
}

// 格式化日期为字符串 (YYYY-MM-DD)
const formatDateToString = (date: Date): string => {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

// 禁用没有录像的日期（可选功能，目前不启用）
const disabledDate = (_date: Date) => {
  // 返回 false 表示不禁用任何日期
  // 如需禁用没有录像的日期，取消下面注释：
  // const dateStr = formatDateToString(date)
  // return !zlmRecordingDates.value.includes(dateStr)
  return false
}

const playZLMRecording = async (recording: any) => {
  try {
    const response = await axios.get(
      `/api/recording/zlm/play/${recording.app}/${recording.stream}/${recording.fileName}`
    )
    
    if (response.data.success) {
      currentPlayback.value = {
        fileName: recording.fileName,
        app: recording.app,
        stream: recording.stream,
        size: recording.fileSize,
        fileSize: response.data.fileSize || recording.fileSize,
        modTime: recording.modTime,
        playUrl: response.data.playUrl || response.data.mp4Url,
        flvUrl: response.data.flvUrl || '',
        mp4Url: response.data.mp4Url || '',
        downloadUrl: response.data.downloadUrl || response.data.mp4Url,
        streamKey: response.data.streamKey || '',
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

// 关闭回放对话框时清理资源
const onPlaybackDialogClose = async () => {
  // 如果有 ZLM 流代理，停止它
  if (currentPlayback.value.streamKey) {
    try {
      await axios.post('/api/recording/zlm/stop', null, {
        params: { key: currentPlayback.value.streamKey }
      })
    } catch (error) {
      console.warn('停止回放流失败:', error)
    }
  }
  
  // 如果有 GB28181 回放会话，停止它
  if (currentPlayback.value.gb28181StreamId && selectedRecording.value) {
    try {
      await axios.post('/api/gb28181/record/playback/stop', {
        channelId: selectedRecording.value.channelId,
        streamId: currentPlayback.value.gb28181StreamId
      })
    } catch (error) {
      console.warn('停止 GB28181 回放会话失败:', error)
    }
  }
  
  // 重置状态
  currentPlayback.value = {
    fileName: '',
    app: '',
    stream: '',
    size: '',
    fileSize: '',
    modTime: '',
    playUrl: '',
    flvUrl: '',
    mp4Url: '',
    downloadUrl: '',
    streamKey: '',
    gb28181StreamId: '',
    note: ''
  }
  selectedRecording.value = null
}

// 播放事件处理
const onPlaybackPlaying = () => {
  console.log('[Playback] 开始播放')
}

const onPlaybackEnded = () => {
  console.log('[Playback] 播放结束')
}

const onPlaybackError = (error: any) => {
  console.error('[Playback] 播放错误:', error)
}

const downloadZLMRecording = async (recording: any) => {
  try {
    const response = await axios.get(
      `/api/recording/zlm/play/${recording.app}/${recording.stream}/${recording.fileName}`
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
    const response = await axios.get('/api/ai/detector/info')
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
    const response = await axios.get('/api/ai/config')
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
    const response = await axios.put('/api/ai/config', {
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
    const response = await axios.get('/api/ai/recording/status/all')
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
    const response = await axios.post('/api/ai/recording/start', {
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
    const response = await axios.post('/api/ai/recording/stop', {
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

<!-- 全局样式用于日期选择器标记 -->
<style>
/* 有录像的日期标记样式 */
.el-date-table td.has-recording {
  position: relative;
}

.el-date-table td.has-recording .el-date-table-cell__text {
  background-color: #409eff;
  color: #fff;
  border-radius: 50%;
}

.el-date-table td.has-recording::after {
  content: '';
  position: absolute;
  bottom: 2px;
  left: 50%;
  transform: translateX(-50%);
  width: 4px;
  height: 4px;
  background-color: #67c23a;
  border-radius: 50%;
}

/* 当前选中日期的有录像标记 */
.el-date-table td.current.has-recording .el-date-table-cell__text {
  background-color: #409eff;
}

/* 今天+有录像 */
.el-date-table td.today.has-recording .el-date-table-cell__text {
  background-color: #409eff;
  color: #fff;
}
</style>
