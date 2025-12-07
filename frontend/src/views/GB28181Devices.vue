<template>
  <div class="gb28181-device-manager">
    <!-- 服务器配置信息 -->
    <el-card class="server-config-card" shadow="hover">
      <template #header>
        <div class="card-header">
          <span class="title">🖥️ GB28181 服务器配置</span>
          <div class="header-actions">
            <el-tag :type="serverConfig.auth_enabled ? 'success' : 'warning'" size="small">
              {{ serverConfig.auth_enabled ? '已启用认证' : '未启用认证' }}
            </el-tag>
            <el-button 
              v-if="!configEditing" 
              type="primary" 
              size="small" 
              @click="startEditConfig">
              ✏️ 编辑
            </el-button>
            <template v-else>
              <el-button type="success" size="small" @click="saveConfig" :loading="configSaving">
                💾 保存
              </el-button>
              <el-button size="small" @click="cancelEditConfig">
                取消
              </el-button>
            </template>
          </div>
        </div>
      </template>
      
      <!-- 只读模式 -->
      <template v-if="!configEditing">
        <el-descriptions :column="4" border size="small">
          <el-descriptions-item label="服务器ID">
            <el-tag type="primary" effect="plain" style="font-family: monospace;">
              {{ serverConfig.server_id || '-' }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="SIP地址">
            <span style="font-family: monospace;">{{ serverConfig.sip_ip || '0.0.0.0' }}:{{ serverConfig.sip_port || 5060 }}</span>
          </el-descriptions-item>
          <el-descriptions-item label="域(Realm)">
            <span style="font-family: monospace;">{{ serverConfig.realm || '-' }}</span>
          </el-descriptions-item>
          <el-descriptions-item label="注册有效期">
            {{ serverConfig.register_expires || 3600 }} 秒
          </el-descriptions-item>
        </el-descriptions>
      </template>
      
      <!-- 编辑模式 -->
      <template v-else>
        <el-form :model="configForm" label-width="100px" size="small">
          <el-row :gutter="20">
            <el-col :span="12">
              <el-form-item label="服务器ID">
                <el-input v-model="configForm.server_id" placeholder="如: 34020000002000000001" />
              </el-form-item>
            </el-col>
            <el-col :span="6">
              <el-form-item label="SIP IP">
                <el-input v-model="configForm.sip_ip" placeholder="如: 0.0.0.0" />
              </el-form-item>
            </el-col>
            <el-col :span="6">
              <el-form-item label="SIP端口">
                <el-input-number v-model="configForm.sip_port" :min="1" :max="65535" />
              </el-form-item>
            </el-col>
          </el-row>
          <el-row :gutter="20">
            <el-col :span="8">
              <el-form-item label="域(Realm)">
                <el-input v-model="configForm.realm" placeholder="如: 3402000000" />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="认证密码">
                <el-input v-model="configForm.password" type="password" show-password placeholder="留空则不认证" />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="注册有效期">
                <el-input-number v-model="configForm.register_expires" :min="60" :max="86400" :step="60" />
              </el-form-item>
            </el-col>
          </el-row>
        </el-form>
      </template>
            
    </el-card>

    <!-- 统计卡片 -->
    <el-row :gutter="20" class="stats-row">
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-content">
            <div class="stat-icon total">📹</div>
            <div class="stat-info">
              <div class="stat-value">{{ statistics.total }}</div>
              <div class="stat-label">设备总数</div>
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
              <div class="stat-label">在线设备</div>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-content">
            <div class="stat-icon offline">✗</div>
            <div class="stat-info">
              <div class="stat-value danger">{{ statistics.offline }}</div>
              <div class="stat-label">离线设备</div>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-content">
            <div class="stat-icon ptz">🎮</div>
            <div class="stat-info">
              <div class="stat-value">{{ statistics.ptzDevices }}</div>
              <div class="stat-label">PTZ设备</div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-card class="box-card" shadow="hover">
      <template #header>
        <div class="card-header">
          <span class="title">
            📡 GB28181设备管理
          </span>
          <div class="button-group">
            <el-button 
              type="success" 
              @click="discoverDevices"
              :loading="discoverLoading"
              size="default">
              🔍 等待注册
            </el-button>
            <el-button 
              @click="refreshDevices"
              :loading="loading"
              size="default">
              🔄 刷新列表
            </el-button>
          </div>
        </div>
      </template>

      <!-- 搜索过滤 -->
      <div class="filter-bar">
        <el-input
          v-model="searchText"
          placeholder="搜索设备ID、名称、IP地址..."
          style="width: 300px"
          clearable
          @clear="searchText = ''">
          <template #prefix>🔍</template>
        </el-input>
        <el-select v-model="statusFilter" placeholder="状态筛选" style="width: 120px; margin-left: 10px;" clearable>
          <el-option label="全部" value="" />
          <el-option label="在线" value="online" />
          <el-option label="离线" value="offline" />
        </el-select>
      </div>

      <!-- 设备列表 -->
      <el-table
        :data="filteredDevices"
        stripe
        style="width: 100%"
        v-loading="loading"
        empty-text="暂无设备，请等待GB28181设备主动注册"
        @row-click="handleRowClick">
        <el-table-column type="expand">
          <template #default="{ row }">
            <div class="device-expand">
              <el-descriptions :column="3" border size="small">
                <el-descriptions-item label="设备ID">{{ row.deviceId }}</el-descriptions-item>
                <el-descriptions-item label="厂商">{{ row.manufacturer || '-' }}</el-descriptions-item>
                <el-descriptions-item label="型号">{{ row.model || '-' }}</el-descriptions-item>
                <el-descriptions-item label="固件版本">{{ row.firmware || '-' }}</el-descriptions-item>
                <el-descriptions-item label="传输协议">{{ row.transport || 'TCP' }}</el-descriptions-item>
                <el-descriptions-item label="流模式">{{ row.streamMode || '-' }}</el-descriptions-item>
                <el-descriptions-item label="注册时间">{{ formatTime(row.registerTime) }}</el-descriptions-item>
                <el-descriptions-item label="最后心跳">{{ formatTime(row.lastKeepAlive) }}</el-descriptions-item>
                <el-descriptions-item label="有效期">{{ row.expires }}秒</el-descriptions-item>
                <el-descriptions-item label="通道数">
                  <el-tag type="info" size="small">
                    {{ row.onlineChannels || 0 }} / {{ row.channelCount || 0 }}
                  </el-tag>
                </el-descriptions-item>
                <el-descriptions-item label="PTZ支持">
                  <el-tag :type="row.ptzSupported ? 'success' : 'info'" size="small">
                    {{ row.ptzSupported ? '支持' : '不支持' }}
                  </el-tag>
                </el-descriptions-item>
                <el-descriptions-item label="录像支持">
                  <el-tag :type="row.recordSupported ? 'success' : 'info'" size="small">
                    {{ row.recordSupported ? '支持' : '不支持' }}
                  </el-tag>
                </el-descriptions-item>
              </el-descriptions>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="deviceId" label="设备ID" width="220">
          <template #default="{ row }">
            <div style="display: flex; align-items: center; gap: 6px;">
              <span style="font-family: monospace; font-size: 12px;">{{ row.deviceId }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="name" label="设备名称" width="150">
          <template #default="{ row }">
            <span>{{ row.name || '未命名设备' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="sipIP" label="SIP地址" width="150">
          <template #default="{ row }">
            <span>{{ row.sipIP }}:{{ row.sipPort }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="manufacturer" label="厂商" width="100"></el-table-column>
        <el-table-column label="通道" width="80">
          <template #default="{ row }">
            <el-tag type="info" size="small">
              {{ row.channelCount || 0 }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="120">
          <template #default="{ row }">
            <div style="display: flex; align-items: center; gap: 8px;">
              <el-tag 
                :type="row.status === 'online' ? 'success' : 'danger'"
                effect="plain">
                {{ row.status === 'online' ? '在线' : '离线' }}
              </el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="260" fixed="right">
          <template #default="{ row }">
            <el-button-group>
              <el-tooltip content="视频预览" placement="top">
                <el-button 
                  type="success" 
                  size="small"
                  :disabled="row.status !== 'online'"
                  @click.stop="showPreview(row)">
                  🎬
                </el-button>
              </el-tooltip>
              <el-tooltip content="PTZ控制" placement="top">
                <el-button 
                  type="warning" 
                  size="small"
                  :disabled="!row.ptzSupported || row.status !== 'online'"
                  @click.stop="showPTZControl(row)">
                  🎮
                </el-button>
              </el-tooltip>
              <el-tooltip content="查看通道" placement="top">
                <el-button 
                  type="info" 
                  size="small"
                  @click.stop="showChannels(row)">
                  📺
                </el-button>
              </el-tooltip>
              <el-tooltip content="删除设备" placement="top">
                <el-button 
                  type="danger" 
                  size="small"
                  @click.stop="deleteDevice(row)">
                  🗑️
                </el-button>
              </el-tooltip>
            </el-button-group>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 设备预览对话框 -->
    <el-dialog 
      v-model="previewData.showDialog" 
      :title="previewDialogTitle"
      width="950px"
      @close="stopPreview"
      @open="onPreviewDialogOpen">
      <div class="preview-container">
        <!-- 通道选择器 -->
        <div class="channel-selector" v-if="previewData.channels && previewData.channels.length > 1">
          <span style="margin-right: 10px;">选择通道:</span>
          <el-select 
            v-model="previewData.selectedChannelId" 
            placeholder="请选择通道"
            style="width: 350px;"
            @change="onChannelChange">
            <el-option
              v-for="ch in previewData.channels"
              :key="ch.channelId"
              :label="`${ch.name || '通道'} (${ch.channelId})`"
              :value="ch.channelId"
              :disabled="ch.status !== 'ON' && ch.status !== 'online'">
              <div style="display: flex; justify-content: space-between; align-items: center;">
                <span>{{ ch.name || '通道' }}</span>
                <span style="font-size: 12px; color: #909399; margin-left: 10px;">{{ ch.channelId }}</span>
                <el-tag 
                  :type="ch.status === 'ON' || ch.status === 'online' ? 'success' : 'danger'" 
                  size="small"
                  style="margin-left: 10px;">
                  {{ ch.status === 'ON' || ch.status === 'online' ? '在线' : '离线' }}
                </el-tag>
              </div>
            </el-option>
          </el-select>
          <el-button 
            type="primary" 
            size="small" 
            style="margin-left: 10px;" 
            @click="startPreviewStream"
            :loading="previewData.loading"
            :disabled="!previewData.selectedChannelId">
            开始预览
          </el-button>
        </div>
        
        <!-- 视频播放区域 -->
        <div class="video-player-wrapper" v-loading="previewData.loading">
          <video 
            ref="videoRef" 
            class="video-player"
            controls
            autoplay
            muted
            @error="onVideoError">
          </video>
          <div v-if="previewData.error" class="video-error">
            <el-icon size="48"><VideoCamera /></el-icon>
            <p>{{ previewData.error }}</p>
            <el-button type="primary" @click="retryPreview">重试</el-button>
          </div>
        </div>

        <!-- 播放信息 -->
        <div class="stream-urls" v-if="previewData.streamInfo">
          <el-descriptions :column="1" border size="small">
            <el-descriptions-item label="HTTP-FLV">
              <el-link type="primary" @click="copyToClipboard(previewData.streamInfo.flv_url)">
                {{ previewData.streamInfo.flv_url }}
              </el-link>
            </el-descriptions-item>
            <el-descriptions-item label="WS-FLV">
              <el-link type="primary" @click="copyToClipboard(previewData.streamInfo.ws_flv_url)">
                {{ previewData.streamInfo.ws_flv_url }}
              </el-link>
            </el-descriptions-item>
            <el-descriptions-item label="HLS">
              <el-link type="primary" @click="copyToClipboard(previewData.streamInfo.hls_url)">
                {{ previewData.streamInfo.hls_url }}
              </el-link>
            </el-descriptions-item>
          </el-descriptions>
        </div>

        <el-alert
          title="GB28181 预览说明"
          description="GB28181设备需要主动向服务器推送媒体流。请确保设备已正确配置并处于在线状态。"
          type="info"
          show-icon
          :closable="false"
          style="margin-top: 16px;"></el-alert>
      </div>

      <template #footer>
        <el-button type="danger" @click="stopPreviewAndClose">停止预览</el-button>
      </template>
    </el-dialog>

    <!-- PTZ控制对话框 -->
    <el-dialog 
      v-model="ptzData.showDialog" 
      :title="`PTZ控制 - ${ptzData.device?.name || ptzData.device?.deviceId}`"
      width="500px">
      <div class="ptz-container">
        <div class="ptz-device-info">
          <el-tag type="success">{{ ptzData.device?.sipIP }}:{{ ptzData.device?.sipPort }}</el-tag>
          <el-tag type="info">{{ ptzData.device?.manufacturer || 'GB28181' }}</el-tag>
        </div>

        <!-- PTZ方向控制 -->
        <div class="ptz-controls">
          <div class="ptz-direction">
            <div class="ptz-row">
              <div class="ptz-cell"></div>
              <el-button 
                class="ptz-btn"
                @mousedown="startPTZ('up')"
                @mouseup="stopPTZ"
                @mouseleave="stopPTZ">
                ⬆️
              </el-button>
              <div class="ptz-cell"></div>
            </div>
            <div class="ptz-row">
              <el-button 
                class="ptz-btn"
                @mousedown="startPTZ('left')"
                @mouseup="stopPTZ"
                @mouseleave="stopPTZ">
                ⬅️
              </el-button>
              <el-button 
                class="ptz-btn center"
                @click="stopPTZ">
                ⏹️
              </el-button>
              <el-button 
                class="ptz-btn"
                @mousedown="startPTZ('right')"
                @mouseup="stopPTZ"
                @mouseleave="stopPTZ">
                ➡️
              </el-button>
            </div>
            <div class="ptz-row">
              <div class="ptz-cell"></div>
              <el-button 
                class="ptz-btn"
                @mousedown="startPTZ('down')"
                @mouseup="stopPTZ"
                @mouseleave="stopPTZ">
                ⬇️
              </el-button>
              <div class="ptz-cell"></div>
            </div>
          </div>

          <!-- 变焦控制 -->
          <div class="ptz-zoom">
            <el-button 
              class="ptz-btn zoom"
              @mousedown="startPTZ('zoomIn')"
              @mouseup="stopPTZ"
              @mouseleave="stopPTZ">
              🔍+
            </el-button>
            <el-button 
              class="ptz-btn zoom"
              @mousedown="startPTZ('zoomOut')"
              @mouseup="stopPTZ"
              @mouseleave="stopPTZ">
              🔍-
            </el-button>
          </div>
        </div>

        <!-- 速度控制 -->
        <div class="ptz-speed">
          <span>速度: {{ ptzData.speed }}</span>
          <el-slider 
            v-model="ptzData.speed" 
            :min="1" 
            :max="255"
            :step="1"
            style="width: 200px; margin-left: 10px;"></el-slider>
        </div>
      </div>

      <template #footer>
        <el-button @click="ptzData.showDialog = false">关闭</el-button>
      </template>
    </el-dialog>

    <!-- 通道列表对话框 -->
    <el-dialog 
      v-model="channelsData.showDialog" 
      :title="`设备通道 - ${channelsData.device?.name || channelsData.device?.deviceId}`"
      width="900px">
      
      <div class="channel-header">
        <el-alert 
          type="info" 
          :closable="false"
          style="margin-bottom: 16px;">
          <template #title>
            <div style="display: flex; align-items: center; gap: 10px;">
              <span>设备ID: {{ channelsData.device?.deviceId }}</span>
              <el-divider direction="vertical" />
              <span>通道总数: {{ channelsData.channels.length }}</span>
              <el-divider direction="vertical" />
              <span>在线通道: {{ channelsData.channels.filter(c => c.status === 'ON' || c.status === 'online').length }}</span>
            </div>
          </template>
        </el-alert>
        <el-button type="primary" size="small" @click="refreshChannels" :loading="channelsData.loading">
          🔄 刷新通道
        </el-button>
      </div>
      
      <el-table
        :data="channelsData.channels"
        v-loading="channelsData.loading"
        empty-text="暂无通道信息，请点击刷新通道按钮查询设备通道"
        stripe
        max-height="400">
        <el-table-column prop="channelId" label="通道ID" width="200">
          <template #default="{ row }">
            <span style="font-family: monospace; font-size: 12px;">{{ row.channelId }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="name" label="通道名称" min-width="120">
          <template #default="{ row }">
            <span>{{ row.name || '未命名通道' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="manufacturer" label="厂商" width="100">
          <template #default="{ row }">
            <span>{{ row.manufacturer || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="model" label="型号" width="100">
          <template #default="{ row }">
            <span>{{ row.model || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status === 'ON' || row.status === 'online' ? 'success' : 'danger'" size="small">
              {{ row.status === 'ON' || row.status === 'online' ? '在线' : '离线' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="PTZ" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.ptzType > 0 ? 'success' : 'info'" size="small">
              {{ row.ptzType > 0 ? '支持' : '不支持' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button-group>
              <el-button 
                type="primary" 
                size="small"
                :disabled="row.status !== 'ON' && row.status !== 'online'"
                @click="previewChannel(row)">
                🎬 预览
              </el-button>
              <el-button 
                type="warning" 
                size="small"
                :disabled="row.ptzType <= 0 || (row.status !== 'ON' && row.status !== 'online')"
                @click="ptzControlChannel(row)">
                🎮
              </el-button>
            </el-button-group>
          </template>
        </el-table-column>
      </el-table>

      <template #footer>
        <el-button @click="channelsData.showDialog = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { VideoCamera } from '@element-plus/icons-vue'

interface Device {
  deviceId: string
  name: string
  manufacturer: string
  model: string
  firmware: string
  status: string
  sipIP: string
  sipPort: number
  transport: string
  registerTime: number
  lastKeepAlive: number
  expires: number
  channelCount: number
  onlineChannels: number
  ptzSupported: boolean
  recordSupported: boolean
  streamMode: string
}

interface Channel {
  channelId: string
  deviceId: string
  name: string
  status: string
  ptzType: number
  manufacturer: string
  model: string
  longitude: string
  latitude: string
}

interface StreamInfo {
  device_id: string
  channel_id: string
  stream_key: string
  app: string
  stream: string
  flv_url: string
  ws_flv_url: string
  hls_url: string
  rtmp_url: string
  create_time: number
}

interface ServerConfig {
  sip_ip: string
  sip_port: number
  realm: string
  server_id: string
  heartbeat_interval: number
  register_expires: number
  auth_enabled: boolean
}

interface ConfigForm {
  sip_ip: string
  sip_port: number
  realm: string
  server_id: string
  password: string
  register_expires: number
}

const devices = ref<Device[]>([])
const loading = ref(false)
const discoverLoading = ref(false)
const searchText = ref('')
const statusFilter = ref('')

// 服务器配置
const serverConfig = ref<ServerConfig>({
  sip_ip: '0.0.0.0',
  sip_port: 5060,
  realm: '',
  server_id: '',
  heartbeat_interval: 60,
  register_expires: 3600,
  auth_enabled: false
})

// 配置编辑
const configEditing = ref(false)
const configSaving = ref(false)
const configForm = ref<ConfigForm>({
  sip_ip: '0.0.0.0',
  sip_port: 5060,
  realm: '',
  server_id: '',
  password: '',
  register_expires: 3600
})

// 统计数据
const statistics = computed(() => {
  const total = devices.value.length
  const online = devices.value.filter(d => d.status === 'online').length
  const offline = devices.value.filter(d => d.status !== 'online').length
  const ptzDevices = devices.value.filter(d => d.ptzSupported).length
  return { total, online, offline, ptzDevices }
})

// 过滤后的设备列表
const filteredDevices = computed(() => {
  return devices.value.filter(device => {
    const matchSearch = !searchText.value || 
      device.deviceId?.toLowerCase().includes(searchText.value.toLowerCase()) ||
      device.name?.toLowerCase().includes(searchText.value.toLowerCase()) ||
      device.sipIP?.includes(searchText.value) ||
      device.manufacturer?.toLowerCase().includes(searchText.value.toLowerCase())
    const matchStatus = !statusFilter.value || device.status === statusFilter.value
    return matchSearch && matchStatus
  })
})

// 预览数据
const previewData = reactive({
  showDialog: false,
  device: null as Device | null,
  channels: [] as Channel[],
  selectedChannelId: '' as string,
  loading: false,
  error: '',
  streamInfo: null as StreamInfo | null,
  flvPlayer: null as any
})

// 预览对话框标题
const previewDialogTitle = computed(() => {
  const device = previewData.device
  const channel = previewData.channels.find(c => c.channelId === previewData.selectedChannelId)
  if (channel) {
    return `预览 - ${channel.name || '通道'} (${channel.channelId})`
  }
  return `设备预览 - ${device?.name || device?.deviceId || ''}`
})

// PTZ控制数据
const ptzData = reactive({
  showDialog: false,
  device: null as Device | null,
  speed: 128
})

// 通道数据
const channelsData = reactive({
  showDialog: false,
  device: null as Device | null,
  channels: [] as Channel[],
  loading: false
})

// 视频播放器引用
const videoRef = ref<HTMLVideoElement | null>(null)

// 自动刷新定时器
let refreshTimer: ReturnType<typeof setInterval> | null = null

// 获取设备列表
const refreshDevices = async () => {
  loading.value = true
  try {
    const response = await fetch('/api/gb28181/devices')
    if (!response.ok) throw new Error('获取设备列表失败')
    const data = await response.json()
    devices.value = data.devices || []
  } catch (error) {
    console.error('获取设备列表失败:', error)
    ElMessage.error('获取设备列表失败')
  } finally {
    loading.value = false
  }
}

// 发现设备
const discoverDevices = async () => {
  discoverLoading.value = true
  try {
    const response = await fetch('/api/gb28181/discover', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' }
    })
    if (!response.ok) throw new Error('设备发现失败')
    const data = await response.json()
    ElMessage.success(data.message || 'GB28181设备需要主动注册到本服务器')
    // 等待设备注册后刷新
    setTimeout(refreshDevices, 3000)
  } catch (error) {
    ElMessage.error(`设备发现失败: ${error}`)
  } finally {
    discoverLoading.value = false
  }
}

// 删除设备
const deleteDevice = async (device: Device) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除设备 "${device.name || device.deviceId}" 吗？`,
      '确认删除',
      { type: 'warning' }
    )
    
    const response = await fetch(`/api/gb28181/devices/${device.deviceId}`, {
      method: 'DELETE'
    })
    
    if (!response.ok) throw new Error('删除失败')
    
    ElMessage.success('设备已删除')
    refreshDevices()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error(`删除失败: ${error.message || error}`)
    }
  }
}

// 显示预览
const showPreview = async (device: Device) => {
  previewData.device = device
  previewData.channels = []
  previewData.selectedChannelId = ''
  previewData.error = ''
  previewData.streamInfo = null
  previewData.showDialog = true
}

// 预览对话框打开时获取通道列表
const onPreviewDialogOpen = async () => {
  if (!previewData.device) return
  
  previewData.loading = true
  try {
    const response = await fetch(`/api/gb28181/devices/${previewData.device.deviceId}/channels`)
    if (response.ok) {
      const data = await response.json()
      previewData.channels = data.channels || []
      
      // 如果只有一个通道，自动选中并开始预览
      if (previewData.channels.length === 1) {
        previewData.selectedChannelId = previewData.channels[0].channelId
        startPreviewStream()
      } else if (previewData.channels.length > 1) {
        // 多个通道，选中第一个在线的通道
        const onlineChannel = previewData.channels.find(c => c.status === 'ON' || c.status === 'online')
        if (onlineChannel) {
          previewData.selectedChannelId = onlineChannel.channelId
        }
      } else {
        // 没有通道，显示错误
        previewData.error = '该设备没有可用通道，请先刷新通道列表'
      }
    }
  } catch (error) {
    console.error('获取通道列表失败:', error)
  } finally {
    previewData.loading = false
  }
}

// 通道切换
const onChannelChange = () => {
  // 停止当前播放
  if (previewData.flvPlayer) {
    try {
      previewData.flvPlayer.pause()
      previewData.flvPlayer.unload()
      previewData.flvPlayer.detachMediaElement()
      previewData.flvPlayer.destroy()
    } catch (e) {
      console.warn('清理播放器时出错:', e)
    }
    previewData.flvPlayer = null
  }
  previewData.streamInfo = null
  previewData.error = ''
}

// 启动预览流
const startPreviewStream = async () => {
  if (!previewData.device) return
  
  const channelId = previewData.selectedChannelId || previewData.device.deviceId
  
  previewData.loading = true
  previewData.error = ''
  
  try {
    const response = await fetch(`/api/gb28181/devices/${previewData.device.deviceId}/channels/${channelId}/preview/start`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' }
    })
    
    if (!response.ok) {
      const errData = await response.json().catch(() => ({}))
      throw new Error(errData.error || '启动预览失败')
    }
    
    const data = await response.json()
    if (!data.success) {
      throw new Error(data.error || '启动预览失败')
    }
    
    previewData.streamInfo = data.data
    
    // 等待 DOM 更新后初始化播放器
    await nextTick()
    initFlvPlayer()
    
  } catch (error: any) {
    console.error('启动预览失败:', error)
    previewData.error = error.message || '启动预览失败'
  } finally {
    previewData.loading = false
  }
}

// 初始化 FLV 播放器
const initFlvPlayer = async () => {
  if (!previewData.streamInfo || !videoRef.value) return
  
  try {
    const flvjs = await import('flv.js')
    
    if (!flvjs.default.isSupported()) {
      previewData.error = '浏览器不支持 FLV 播放'
      return
    }
    
    if (previewData.flvPlayer) {
      previewData.flvPlayer.destroy()
      previewData.flvPlayer = null
    }
    
    previewData.flvPlayer = flvjs.default.createPlayer({
      type: 'flv',
      url: previewData.streamInfo.flv_url,
      isLive: true,
      hasAudio: true,
      hasVideo: true,
      cors: true
    }, {
      enableStashBuffer: false,
      stashInitialSize: 128,
      enableWorker: true,
      lazyLoadMaxDuration: 3 * 60,
      seekType: 'range'
    })
    
    previewData.flvPlayer.attachMediaElement(videoRef.value)
    previewData.flvPlayer.load()
    previewData.flvPlayer.play()
    
    previewData.flvPlayer.on(flvjs.default.Events.ERROR, (errType: any, errDetail: any) => {
      console.error('FLV播放器错误:', errType, errDetail)
      previewData.error = `播放错误: ${errDetail}`
    })
    
  } catch (error: any) {
    console.error('初始化播放器失败:', error)
    previewData.error = `播放器初始化失败: ${error.message}`
  }
}

// 停止预览
const stopPreview = async () => {
  if (previewData.flvPlayer) {
    try {
      previewData.flvPlayer.pause()
      previewData.flvPlayer.unload()
      previewData.flvPlayer.detachMediaElement()
      previewData.flvPlayer.destroy()
    } catch (e) {
      console.warn('销毁播放器时出错:', e)
    }
    previewData.flvPlayer = null
  }
  
  if (previewData.device && previewData.streamInfo) {
    const channelId = previewData.selectedChannelId || previewData.device.deviceId
    try {
      await fetch(`/api/gb28181/devices/${previewData.device.deviceId}/channels/${channelId}/preview/stop`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' }
      })
    } catch (e) {
      console.warn('停止预览流时出错:', e)
    }
  }
  
  previewData.streamInfo = null
  previewData.error = ''
}

const stopPreviewAndClose = async () => {
  await stopPreview()
  previewData.showDialog = false
}

const retryPreview = () => {
  previewData.error = ''
  startPreviewStream()
}

const onVideoError = (event: Event) => {
  console.error('视频播放错误:', event)
  if (!previewData.error) {
    previewData.error = '视频加载失败，请检查设备是否正在推流'
  }
}

// PTZ 控制
const showPTZControl = (device: Device) => {
  if (!device.ptzSupported) {
    ElMessage.warning('该设备不支持PTZ控制')
    return
  }
  ptzData.device = device
  ptzData.showDialog = true
}

const startPTZ = async (command: string) => {
  if (!ptzData.device) return
  
  try {
    await fetch(`/api/gb28181/devices/${ptzData.device.deviceId}/ptz`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        command: command,
        speed: ptzData.speed
      })
    })
  } catch (error) {
    ElMessage.error(`PTZ控制失败: ${error}`)
  }
}

const stopPTZ = async () => {
  if (!ptzData.device) return
  
  try {
    await fetch(`/api/gb28181/devices/${ptzData.device.deviceId}/ptz`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        command: 'stop',
        speed: 0
      })
    })
  } catch (error) {
    console.error('停止PTZ失败:', error)
  }
}

// 通道管理
const showChannels = async (device: Device) => {
  channelsData.device = device
  channelsData.channels = []
  channelsData.loading = true
  channelsData.showDialog = true
  
  await refreshChannels()
}

// 刷新通道列表
const refreshChannels = async () => {
  if (!channelsData.device) return
  
  channelsData.loading = true
  try {
    // 先触发设备查询通道
const queryResponse = await fetch(`/api/gb28181/devices/${channelsData.device.deviceId}/catalog`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' }
    })
    if (!queryResponse.ok) {
      console.warn('触发目录查询失败')
    }
    
    // 等待一段时间让设备响应
    await new Promise(resolve => setTimeout(resolve, 1000))
    
    // 获取通道列表
    const response = await fetch(`/api/gb28181/devices/${channelsData.device.deviceId}/channels`)
    if (!response.ok) throw new Error('获取通道失败')
    const data = await response.json()
    channelsData.channels = data.channels || []
    
    if (channelsData.channels.length === 0) {
      ElMessage.info('暂无通道信息，请等待设备响应后再次刷新')
    } else {
      ElMessage.success(`获取到 ${channelsData.channels.length} 个通道`)
    }
  } catch (error) {
    console.error('获取通道列表失败:', error)
    ElMessage.error('获取通道列表失败')
  } finally {
    channelsData.loading = false
  }
}

const previewChannel = (channel: Channel) => {
  // 预览指定通道
  const device = channelsData.device
  if (device) {
    previewData.device = device
    previewData.channels = channelsData.channels
    previewData.selectedChannelId = channel.channelId
    previewData.error = ''
    previewData.streamInfo = null
    previewData.showDialog = true
    channelsData.showDialog = false
    // 直接开始预览
    startPreviewStream()
  }
}

// PTZ 控制指定通道
const ptzControlChannel = (channel: Channel) => {
  const device = channelsData.device
  if (device) {
    ptzData.device = device
    ptzData.showDialog = true
    channelsData.showDialog = false
  }
}

// 辅助函数
const handleRowClick = (row: Device) => {
  // 可以在这里添加点击行的逻辑
}

const formatTime = (timestamp: number) => {
  if (!timestamp) return '-'
  const date = new Date(timestamp * 1000)
  return date.toLocaleString('zh-CN')
}

const getPTZTypeName = (ptzType: number) => {
  const types: Record<number, string> = {
    0: '未知',
    1: '球机',
    2: '半球',
    3: '固定枪机',
    4: '遥控枪机'
  }
  return types[ptzType] || '未知'
}

const copyToClipboard = async (text: string) => {
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success('已复制到剪贴板')
  } catch (e) {
    ElMessage.error('复制失败')
  }
}

// 获取服务器配置
const fetchServerConfig = async () => {
  try {
    const response = await fetch('/api/gb28181/server-config')
    if (response.ok) {
      const data = await response.json()
      if (data.success && data.config) {
        serverConfig.value = data.config
      }
    }
  } catch (error) {
    console.error('获取服务器配置失败:', error)
  }
}

// 开始编辑配置
const startEditConfig = () => {
  configForm.value = {
    sip_ip: serverConfig.value.sip_ip,
    sip_port: serverConfig.value.sip_port,
    realm: serverConfig.value.realm,
    server_id: serverConfig.value.server_id,
    password: '',  // 密码不回显
    register_expires: serverConfig.value.register_expires
  }
  configEditing.value = true
}

// 取消编辑配置
const cancelEditConfig = () => {
  configEditing.value = false
}

// 保存配置
const saveConfig = async () => {
  configSaving.value = true
  try {
    const response = await fetch('/api/gb28181/server-config', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(configForm.value)
    })
    
    if (!response.ok) {
      const errData = await response.json().catch(() => ({}))
      throw new Error(errData.error || '保存失败')
    }
    
    const data = await response.json()
    if (data.success) {
      ElMessage.success('配置保存成功，需要重启服务器生效')
      configEditing.value = false
      fetchServerConfig()
    } else {
      throw new Error(data.error || '保存失败')
    }
  } catch (error: any) {
    ElMessage.error(`保存配置失败: ${error.message || error}`)
  } finally {
    configSaving.value = false
  }
}

// 获取本地IP（用于显示提示）
const getLocalIP = () => {
  // 如果配置的是 0.0.0.0，显示当前页面的主机名
  if (serverConfig.value.sip_ip === '0.0.0.0' || !serverConfig.value.sip_ip) {
    return window.location.hostname
  }
  return serverConfig.value.sip_ip
}

// 生命周期
onMounted(() => {
  fetchServerConfig()
  refreshDevices()
  // 每30秒自动刷新
  refreshTimer = setInterval(refreshDevices, 30000)
})

onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
  }
  stopPreview()
})
</script>

<style scoped lang="css">
.gb28181-device-manager {
  padding: 20px;
}

.server-config-card {
  margin-bottom: 20px;
}

.server-config-card .card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.server-config-card .title {
  font-size: 16px;
  font-weight: 600;
}

.stats-row {
  margin-bottom: 20px;
}

.stat-card {
  cursor: default;
}

.stat-content {
  display: flex;
  align-items: center;
  gap: 16px;
}

.stat-icon {
  font-size: 32px;
  width: 50px;
  height: 50px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 10px;
}

.stat-icon.total { background: #e8f4fd; }
.stat-icon.online { background: #e8f8e8; }
.stat-icon.offline { background: #fde8e8; }
.stat-icon.ptz { background: #fff3e0; }

.stat-info {
  flex: 1;
}

.stat-value {
  font-size: 28px;
  font-weight: bold;
  color: #303133;
}

.stat-value.success { color: #67c23a; }
.stat-value.danger { color: #f56c6c; }

.stat-label {
  font-size: 14px;
  color: #909399;
  margin-top: 4px;
}

.box-card {
  margin-bottom: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0;
}

.title {
  font-size: 16px;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 8px;
}

.button-group {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.filter-bar {
  margin-bottom: 16px;
  display: flex;
  align-items: center;
}

.device-expand {
  padding: 10px 20px;
  background: #fafafa;
}

/* 视频播放器样式 */
.preview-container {
  padding: 20px 0;
}

.video-player-wrapper {
  position: relative;
  width: 100%;
  background: #000;
  border-radius: 8px;
  overflow: hidden;
  margin-bottom: 16px;
}

.video-player {
  width: 100%;
  max-height: 480px;
  min-height: 360px;
  background: #000;
  display: block;
}

.video-error {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.8);
  color: #fff;
  gap: 16px;
}

.video-error p {
  max-width: 80%;
  text-align: center;
  color: #f56c6c;
}

.stream-urls {
  margin-bottom: 16px;
}

.stream-urls :deep(.el-link) {
  font-family: monospace;
  font-size: 12px;
  word-break: break-all;
}

/* PTZ控制样式 */
.ptz-container {
  padding: 10px;
}

.ptz-device-info {
  display: flex;
  gap: 10px;
  margin-bottom: 20px;
  justify-content: center;
}

.ptz-controls {
  display: flex;
  gap: 30px;
  justify-content: center;
  align-items: center;
  margin-bottom: 20px;
}

.ptz-direction {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.ptz-row {
  display: flex;
  gap: 4px;
  justify-content: center;
}

.ptz-cell {
  width: 50px;
  height: 50px;
}

.ptz-btn {
  width: 50px;
  height: 50px;
  font-size: 20px;
  padding: 0;
}

.ptz-btn.center {
  background: #f56c6c;
  color: white;
}

.ptz-zoom {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.ptz-btn.zoom {
  width: 60px;
  height: 40px;
  font-size: 16px;
}

.ptz-speed {
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 20px;
  padding: 10px;
  background: #f5f7fa;
  border-radius: 6px;
}

.channel-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 16px;
}

.channel-selector {
  display: flex;
  align-items: center;
  margin-bottom: 16px;
  padding: 12px;
  background: #f5f7fa;
  border-radius: 6px;
}
</style>
