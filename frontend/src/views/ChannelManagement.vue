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
            <el-button type="warning" :icon="Download" @click="importChannelsFromDevices" :loading="importLoading">
              从设备导入
            </el-button>
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
        <el-table-column label="操作" width="500" fixed="right">
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
              type="success" 
              link 
              size="small" 
              @click="showPushDialog(row)"
            >
              推流
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
          <el-select v-model="newChannel.deviceId" placeholder="请选择设备" style="width: 100%;" @change="onDeviceSelected">
            <el-option 
              v-for="device in availableDevices" 
              :key="device.deviceId || device.id" 
              :label="device.name || device.deviceId || device.id" 
              :value="device.deviceId || device.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item v-if="newChannel.deviceType === 'gb28181'" label="通道号">
          <template v-if="availableDeviceChannels.length > 0">
            <el-select v-model="newChannel.channel" placeholder="请选择通道号">
              <el-option v-for="ch in availableDeviceChannels" :key="ch" :label="ch" :value="ch" />
            </el-select>
          </template>
          <template v-else>
            <el-input v-model="newChannel.channel" placeholder="请输入通道号" />
          </template>
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

    <!-- 预览对话框 (使用 PreviewPlayer) -->
    <el-dialog 
      v-model="previewDialogVisible" 
      :title="`预览: ${selectedChannel?.channelName || selectedChannel?.channelId}`" 
      width="900px"
      @close="() => { if (previewPlayerRef.value) previewPlayerRef.value.stopPreview() }"
    >
      <div class="preview-container">
        <div class="video-player-wrapper">
          <PreviewPlayer 
            ref="previewPlayerRef" 
            :show="previewDialogVisible" 
            :device="{ deviceId: selectedChannel?.deviceId }" 
            :channels="[{ channelId: selectedChannel?.channelId }]" 
            :selectedChannelId="selectedChannel?.channelId || ''" 
            :deviceType="selectedChannel?.deviceType || 'gb28181'"
            :profileToken="selectedChannel?.profileToken || ''"
          />
        </div>

        <div class="preview-controls">
          <el-button type="danger" @click="() => { if (previewPlayerRef.value) previewPlayerRef.value.stopPreview(); previewDialogVisible = false }">停止播放</el-button>
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

    <!-- 推流到直播平台对话框 -->
    <el-dialog v-model="pushDialogVisible" title="推流到直播平台" width="650px">
      <div v-if="pushChannel">
        <el-descriptions :column="2" border size="small" style="margin-bottom: 20px;">
          <el-descriptions-item label="通道ID">{{ pushChannel.channelId }}</el-descriptions-item>
          <el-descriptions-item label="通道名称">{{ pushChannel.channelName || pushChannel.name || '-' }}</el-descriptions-item>
        </el-descriptions>

        <!-- 已有推流任务列表 -->
        <div v-if="channelPushTargets.length > 0" style="margin-bottom: 20px;">
          <div style="font-weight: 600; margin-bottom: 10px;">当前推流任务</div>
          <el-table :data="channelPushTargets" size="small" border>
            <el-table-column prop="name" label="名称" width="120" />
            <el-table-column prop="platform" label="平台" width="100">
              <template #default="{ row }">
                <el-tag size="small">{{ getPlatformLabel(row.platform) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="status" label="状态" width="90">
              <template #default="{ row }">
                <el-tag :type="row.status === 'pushing' ? 'success' : row.status === 'error' ? 'danger' : 'info'" size="small">
                  {{ row.status === 'pushing' ? '推流中' : row.status === 'error' ? '错误' : '已停止' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="140">
              <template #default="{ row }">
                <el-button 
                  v-if="row.status !== 'pushing'" 
                  type="success" 
                  link 
                  size="small" 
                  @click="startPush(row.id)"
                  :loading="row.loading"
                >
                  开始
                </el-button>
                <el-button 
                  v-else 
                  type="danger" 
                  link 
                  size="small" 
                  @click="stopPush(row.id)"
                  :loading="row.loading"
                >
                  停止
                </el-button>
                <el-button 
                  type="danger" 
                  link 
                  size="small" 
                  @click="deletePushTarget(row.id)"
                >
                  删除
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </div>

        <!-- 添加新推流任务 -->
        <el-divider content-position="left">添加新推流任务</el-divider>
        <el-form :model="newPushTarget" label-width="100px" size="default">
          <el-form-item label="任务名称" required>
            <el-input v-model="newPushTarget.name" placeholder="例如：抖音直播" />
          </el-form-item>
          <el-form-item label="直播平台" required>
            <el-select v-model="newPushTarget.platform" placeholder="请选择直播平台" style="width: 100%;" @change="onPlatformChange">
              <el-option 
                v-for="platform in pushPlatforms" 
                :key="platform.id" 
                :label="platform.name" 
                :value="platform.id"
              >
                <span>{{ platform.name }}</span>
                <span style="color: #999; font-size: 12px; margin-left: 10px;">{{ platform.url_template }}</span>
              </el-option>
            </el-select>
          </el-form-item>
          <el-form-item label="推流地址" required>
            <el-input v-model="newPushTarget.pushUrl" placeholder="rtmp://live-push.xxx.com/live/">
              <template #prepend v-if="selectedPlatformTemplate">
                <el-tooltip :content="selectedPlatformTemplate" placement="top">
                  <el-icon><InfoFilled /></el-icon>
                </el-tooltip>
              </template>
            </el-input>
          </el-form-item>
          <el-form-item label="推流码" required>
            <el-input v-model="newPushTarget.streamKey" placeholder="请输入推流码/串流密钥" show-password />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" @click="addPushTarget" :loading="pushLoading">
              添加推流任务
            </el-button>
          </el-form-item>
        </el-form>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Refresh, WarningFilled, InfoFilled, Download } from '@element-plus/icons-vue'
import PreviewPlayer from '../components/PreviewPlayer.vue'

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

interface PushPlatform {
  id: string
  name: string
  url_template: string
}

interface PushTarget {
  id: string
  name: string
  platform: string
  push_url: string
  stream_key: string
  channel_id: string
  channel_name: string
  source_url: string
  status: string
  zlm_key?: string
  error?: string
  created_at?: string
  updated_at?: string
  loading?: boolean
}

interface ZLMConfig {
  http: { port: number }
  rtsp: { port: number }
  rtmp: { port: number }
}

const channels = ref<Channel[]>([])
const selectedChannel = ref<Channel | null>(null)

// ZLM配置
const zlmConfig = ref<ZLMConfig>({
  http: { port: 8081 },
  rtsp: { port: 8554 },
  rtmp: { port: 1935 }
})
const loading = ref(false)
const addLoading = ref(false)
const addChannelDialogVisible = ref(false)
const previewDialogVisible = ref(false)
const previewLoading = ref(false)
const previewError = ref('')
const playType = ref<'flv' | 'hls'>('flv')
const importLoading = ref(false)

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

// 可供选择的设备内部通道（当选择 GB28181 设备时填充）
const availableDeviceChannels = ref<string[]>([])

// 当用户在添加通道对话中选择设备时，自动填充通道或 ONVIF profile
const onDeviceSelected = async (deviceId: string) => {
  availableDeviceChannels.value = []
  newChannel.value.channel = ''
  newChannel.value.profileToken = ''
  newChannel.value.streamUrl = ''

  if (!deviceId) return

  if (newChannel.value.deviceType === 'gb28181') {
    // 尝试调用后端获取该 GB28181 设备的通道列表
    try {
      const resp = await fetch(`/api/gb28181/devices/${encodeURIComponent(deviceId)}/channels`)
      if (resp.ok) {
        const d = await resp.json()
        if (d && Array.isArray(d.channels)) {
          availableDeviceChannels.value = d.channels.map((c: any) => c.channelId || c.id || String(c))
        }
      }
    } catch (e) {
      console.warn('获取设备通道失败', e)
    }
  } else if (newChannel.value.deviceType === 'onvif') {
    // 拉取 ONVIF profiles，自动选择第一个 profile token，并尝试构造 RTSP 地址
    try {
      const resp = await fetch(`/api/onvif/devices/${encodeURIComponent(deviceId)}/profiles`)
      if (resp.ok) {
        const d = await resp.json()
        if (d && Array.isArray(d.profiles) && d.profiles.length > 0) {
          newChannel.value.profileToken = d.profiles[0].token || d.profiles[0].profileToken || ''
          // 如果返回了 streamUri 或 rtsp 地址，优先使用
          const uri = d.profiles[0].streamUri || d.profiles[0].rtsp || d.profiles[0].source || ''
          if (uri) newChannel.value.streamUrl = uri
        }
      }
    } catch (e) {
      console.warn('获取 ONVIF profiles 失败', e)
    }
  }
}

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

// Preview player ref
const previewPlayerRef = ref<any>(null)

// 推流相关状态
const pushDialogVisible = ref(false)
const pushChannel = ref<Channel | null>(null)
const pushPlatforms = ref<PushPlatform[]>([])
const channelPushTargets = ref<PushTarget[]>([])
const pushLoading = ref(false)
const newPushTarget = ref({
  name: '',
  platform: '',
  pushUrl: '',
  streamKey: ''
})

// 获取选中平台的模板
const selectedPlatformTemplate = computed(() => {
  const platform = pushPlatforms.value.find(p => p.id === newPushTarget.value.platform)
  return platform?.url_template || ''
})

// 定时刷新
let refreshTimer: number | null = null

// 获取 ZLM 配置
const fetchZLMConfig = async () => {
  try {
    const response = await fetch('/api/zlm/config')
    const data = await response.json()
    if (data.success && data.config) {
      zlmConfig.value = data.config
      console.log('获取到ZLM配置:', data.config)
    }
  } catch (error) {
    console.error('获取ZLM配置失败:', error)
  }
}

// 获取通道列表
const fetchChannels = async () => {
  loading.value = true
  try {
    const response = await fetch('/api/channel/list')
    const data = await response.json()
    channels.value = data.channels || []
  } catch (error) {
    console.error('获取通道列表失败:', error)
    // 不自动从设备导入通道，通道应由用户手动添加
    channels.value = []
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

// 从设备导入通道到数据库
const importChannelsFromDevices = async () => {
  importLoading.value = true
  try {
    // 获取GB28181设备的通道
    const gb28181Response = await fetch('/api/gb28181/devices')
    const gb28181Data = await gb28181Response.json()
    
    const channelsToImport: any[] = []
    
    // 收集GB28181通道
    if (gb28181Data.devices) {
      for (const device of gb28181Data.devices) {
        if (device.channels && device.channels.length > 0) {
          for (const ch of device.channels) {
            channelsToImport.push({
              channelId: ch.channelId,
              name: ch.name,
              deviceId: device.deviceId,
              deviceType: 'gb28181',
              manufacturer: ch.manufacturer || '',
              model: ch.model || '',
              status: ch.status || 'ON',
              streamUrl: ch.streamURL || '',
              longitude: ch.longitude || '',
              latitude: ch.latitude || ''
            })
          }
        }
      }
    }
    
    // 获取ONVIF设备的通道
    try {
      const onvifResponse = await fetch('/api/onvif/devices')
      const onvifData = await onvifResponse.json()
      
      if (onvifData.devices) {
        for (const device of onvifData.devices) {
          // ONVIF设备每个profile作为一个通道
          const profilesResp = await fetch(`/api/onvif/devices/${encodeURIComponent(device.uuid)}/profiles`)
          if (profilesResp.ok) {
            const profilesData = await profilesResp.json()
            if (profilesData.profiles) {
              for (const profile of profilesData.profiles) {
                channelsToImport.push({
                  channelId: `${device.uuid}_${profile.token}`,
                  name: profile.name || device.name,
                  deviceId: device.uuid,
                  deviceType: 'onvif',
                  manufacturer: device.manufacturer || '',
                  model: device.model || '',
                  status: device.status === 'online' ? 'ON' : 'OFF',
                  streamUrl: profile.streamUri || '',
                  profileToken: profile.token
                })
              }
            }
          }
        }
      }
    } catch (onvifError) {
      console.warn('获取ONVIF设备失败:', onvifError)
    }
    
    if (channelsToImport.length === 0) {
      ElMessage.warning('没有找到可导入的通道')
      return
    }
    
    // 调用导入API
    const importResponse = await fetch('/api/channel/import', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({ channels: channelsToImport })
    })
    
    const importResult = await importResponse.json()
    
    if (importResult.success) {
      ElMessage.success(`导入成功：添加 ${importResult.addedCount} 个通道${importResult.failedCount > 0 ? `，失败 ${importResult.failedCount} 个` : ''}`)
      // 刷新通道列表
      await fetchChannels()
    } else {
      ElMessage.error('导入失败：' + (importResult.error || importResult.message))
    }
  } catch (error) {
    console.error('导入通道失败:', error)
    ElMessage.error('导入通道失败，请查看控制台')
  } finally {
    importLoading.value = false
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
  availableDeviceChannels.value = []
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

    // 支持多种后端返回格式：{ success: true } 或 { status: 'ok' } 或 包含 channel 对象
    const ok = data && (data.success === true || data.status === 'ok' || (data.channel && data.channel.channelId))
    if (ok) {
      ElMessage.success(data.message || '通道添加成功')
      addChannelDialogVisible.value = false
      fetchChannels()
    } else {
      ElMessage.error(data.error || data.message || '通道添加失败')
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
    
    const rtspPort = zlmConfig.value.rtsp.port
    const response = await fetch(endpoint, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        channel_id: channelId,
        stream_url: channel.streamUrl || `rtsp://localhost:${rtspPort}/live/${channelId}`,
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

// 预览通道（委托 PreviewPlayer 处理启动/停止）
const previewChannel = async (channel: Channel) => {
  selectedChannel.value = channel
  previewError.value = ''
  previewLoading.value = false
  previewDialogVisible.value = true
  // 等待对话框和子组件渲染完成，然后通过 ref 调用 startPreview 启动流
  await nextTick()
  try {
    if (previewPlayerRef.value && typeof previewPlayerRef.value.startPreview === 'function') {
      await previewPlayerRef.value.startPreview(channel.channelId)
    }
  } catch (e) {
    console.error('启动预览失败:', e)
  }
}

// 停止预览
const stopPreview = () => {
  if (previewPlayerRef.value) previewPlayerRef.value.stopPreview()
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
  const rtspPort = zlmConfig.value.rtsp.port
  const url = `rtsp://${host}:${rtspPort}/rtp/${channel.channelId}`
  copyUrl(url)
}

// ========== 推流相关方法 ==========

// 获取推流平台列表
const fetchPushPlatforms = async () => {
  try {
    const response = await fetch('/api/push/platforms')
    const data = await response.json()
    if (data.success && data.platforms) {
      pushPlatforms.value = data.platforms
    }
  } catch (error) {
    console.error('获取推流平台列表失败:', error)
  }
}

// 获取通道的推流任务
const fetchChannelPushTargets = async (channelId: string) => {
  try {
    const response = await fetch(`/api/push/channel/${encodeURIComponent(channelId)}`)
    const data = await response.json()
    if (data.success && data.targets) {
      channelPushTargets.value = data.targets
    } else {
      channelPushTargets.value = []
    }
  } catch (error) {
    console.error('获取通道推流任务失败:', error)
    channelPushTargets.value = []
  }
}

// 显示推流对话框
const showPushDialog = async (channel: Channel) => {
  pushChannel.value = channel
  newPushTarget.value = {
    name: '',
    platform: '',
    pushUrl: '',
    streamKey: ''
  }
  pushDialogVisible.value = true
  
  // 加载平台列表和通道已有推流任务
  await Promise.all([
    fetchPushPlatforms(),
    fetchChannelPushTargets(channel.channelId)
  ])
}

// 平台选择变化时自动填充推流地址模板
const onPlatformChange = (platformId: string) => {
  const platform = pushPlatforms.value.find(p => p.id === platformId)
  if (platform && platform.url_template) {
    newPushTarget.value.pushUrl = platform.url_template
  }
}

// 获取平台显示名称
const getPlatformLabel = (platformId: string) => {
  const platform = pushPlatforms.value.find(p => p.id === platformId)
  return platform?.name || platformId
}

// 添加推流任务
const addPushTarget = async () => {
  if (!pushChannel.value) return
  
  if (!newPushTarget.value.name || !newPushTarget.value.platform || !newPushTarget.value.pushUrl || !newPushTarget.value.streamKey) {
    ElMessage.warning('请填写完整的推流信息')
    return
  }
  
  pushLoading.value = true
  try {
    const host = window.location.hostname
    const rtspPort = zlmConfig.value.rtsp.port
    // 构建源流地址 - 使用 RTSP 地址
    const sourceUrl = pushChannel.value.streamUrl || `rtsp://${host}:${rtspPort}/rtp/${pushChannel.value.channelId}`
    
    const response = await fetch('/api/push/targets', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name: newPushTarget.value.name,
        platform: newPushTarget.value.platform,
        push_url: newPushTarget.value.pushUrl,
        stream_key: newPushTarget.value.streamKey,
        channel_id: pushChannel.value.channelId,
        channel_name: pushChannel.value.channelName || pushChannel.value.name || '',
        source_url: sourceUrl
      })
    })
    
    const data = await response.json()
    if (data.success) {
      ElMessage.success('推流任务添加成功')
      // 重置表单
      newPushTarget.value = {
        name: '',
        platform: '',
        pushUrl: '',
        streamKey: ''
      }
      // 刷新任务列表
      await fetchChannelPushTargets(pushChannel.value.channelId)
    } else {
      ElMessage.error(data.error || '添加推流任务失败')
    }
  } catch (error) {
    console.error('添加推流任务失败:', error)
    ElMessage.error('添加推流任务失败')
  } finally {
    pushLoading.value = false
  }
}

// 开始推流
const startPush = async (targetId: string) => {
  const target = channelPushTargets.value.find(t => t.id === targetId)
  if (target) target.loading = true
  
  try {
    const response = await fetch(`/api/push/targets/${encodeURIComponent(targetId)}/start`, {
      method: 'POST'
    })
    const data = await response.json()
    if (data.success) {
      ElMessage.success('推流已开始')
      if (pushChannel.value) {
        await fetchChannelPushTargets(pushChannel.value.channelId)
      }
    } else {
      ElMessage.error(data.error || '开始推流失败')
    }
  } catch (error) {
    console.error('开始推流失败:', error)
    ElMessage.error('开始推流失败')
  } finally {
    if (target) target.loading = false
  }
}

// 停止推流
const stopPush = async (targetId: string) => {
  const target = channelPushTargets.value.find(t => t.id === targetId)
  if (target) target.loading = true
  
  try {
    const response = await fetch(`/api/push/targets/${encodeURIComponent(targetId)}/stop`, {
      method: 'POST'
    })
    const data = await response.json()
    if (data.success) {
      ElMessage.success('推流已停止')
      if (pushChannel.value) {
        await fetchChannelPushTargets(pushChannel.value.channelId)
      }
    } else {
      ElMessage.error(data.error || '停止推流失败')
    }
  } catch (error) {
    console.error('停止推流失败:', error)
    ElMessage.error('停止推流失败')
  } finally {
    if (target) target.loading = false
  }
}

// 删除推流任务
const deletePushTarget = async (targetId: string) => {
  try {
    await ElMessageBox.confirm('确定删除该推流任务吗？', '确认删除', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
  } catch {
    return
  }
  
  try {
    const response = await fetch(`/api/push/targets/${encodeURIComponent(targetId)}`, {
      method: 'DELETE'
    })
    const data = await response.json()
    if (data.success) {
      ElMessage.success('推流任务已删除')
      if (pushChannel.value) {
        await fetchChannelPushTargets(pushChannel.value.channelId)
      }
    } else {
      ElMessage.error(data.error || '删除推流任务失败')
    }
  } catch (error) {
    console.error('删除推流任务失败:', error)
    ElMessage.error('删除推流任务失败')
  }
}

// ========== 推流方法结束 ==========

onMounted(async () => {
  await fetchZLMConfig()
  await fetchChannels()
  fetchDevices()
  // 获取录像状态
  fetchRecordingStatus()
  // 每30秒刷新
  refreshTimer = window.setInterval(fetchChannels, 30000)
})

// 当设备类型发生变化时清理设备相关的选择
watch(() => newChannel.value.deviceType, (val) => {
  availableDeviceChannels.value = []
  newChannel.value.channel = ''
  newChannel.value.profileToken = ''
  newChannel.value.streamUrl = ''
})

onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
  }
  if (previewPlayerRef.value) {
    try { previewPlayerRef.value.stopPreview() } catch (e) {}
  }
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
