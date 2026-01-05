<template>
  <div class="onvif-device-manager">
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
            <i class="el-icon-video-camera"></i> ONVIF设备管理
          </span>
          <div class="button-group">
            <el-button 
              type="primary" 
              @click="showAddModal = true"
              size="default">
              ➕ 手动添加
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
          placeholder="搜索设备名称、IP地址..."
          style="width: 300px"
          clearable
          @clear="searchText = ''">
          <template #prefix>🔍</template>
        </el-input>
        <el-select v-model="statusFilter" placeholder="状态筛选" style="width: 120px; margin-left: 10px;" clearable>
          <el-option label="全部" value="" />
          <el-option label="在线" value="online" />
          <el-option label="离线" value="offline" />
          <el-option label="未知" value="unknown" />
        </el-select>
      </div>

      <!-- 设备列表 -->
      <el-table
        :data="filteredDevices"
        stripe
        style="width: 100%"
        v-loading="loading"
        empty-text="暂无设备"
        @row-click="handleRowClick">
        <el-table-column type="expand">
          <template #default="{ row }">
            <div class="device-expand">
              <el-descriptions :column="3" border size="small">
                <el-descriptions-item label="设备ID">{{ row.deviceId }}</el-descriptions-item>
                <el-descriptions-item label="固件版本">{{ row.firmwareVersion || '-' }}</el-descriptions-item>
                <el-descriptions-item label="序列号">{{ row.serialNumber || '-' }}</el-descriptions-item>
                <el-descriptions-item label="发现时间">{{ formatTime(row.discoveryTime) }}</el-descriptions-item>
                <el-descriptions-item label="最后在线">{{ formatTime(row.lastSeenTime) }}</el-descriptions-item>
                <el-descriptions-item label="检查间隔">{{ row.checkInterval || 60 }}秒</el-descriptions-item>
                <el-descriptions-item label="PTZ支持">
                  <el-tag :type="row.ptzSupported ? 'success' : 'info'" size="small">
                    {{ row.ptzSupported ? '支持' : '不支持' }}
                  </el-tag>
                </el-descriptions-item>
                <el-descriptions-item label="音频支持">
                  <el-tag :type="row.audioSupported ? 'success' : 'info'" size="small">
                    {{ row.audioSupported ? '支持' : '不支持' }}
                  </el-tag>
                </el-descriptions-item>
                <el-descriptions-item label="服务列表">
                  <div v-if="row.services && row.services.length">
                    <el-tag v-for="service in row.services.slice(0, 3)" :key="service" size="small" style="margin: 2px;">
                      {{ getServiceName(service) }}
                    </el-tag>
                    <span v-if="row.services.length > 3" style="color: #909399;">+{{ row.services.length - 3 }}</span>
                  </div>
                  <span v-else>-</span>
                </el-descriptions-item>
              </el-descriptions>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="name" label="设备名称" width="180">
          <template #default="{ row }">
            <div style="display: flex; align-items: center; gap: 6px;">
              <span>{{ row.name }}</span>
              <el-tag v-if="row.ptzSupported" type="warning" size="small" effect="plain">PTZ</el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="ip" label="IP地址" width="130"></el-table-column>
        <el-table-column prop="port" label="端口" width="70"></el-table-column>
        <el-table-column prop="manufacturer" label="制造商" width="120"></el-table-column>
        <el-table-column prop="model" label="型号" width="120"></el-table-column>
        <el-table-column label="状态" width="140">
          <template #default="{ row }">
            <div style="display: flex; align-items: center; gap: 8px;">
              <el-tag 
                :type="row.status === 'online' ? 'success' : row.status === 'offline' ? 'danger' : 'warning'"
                effect="plain">
                {{ getStatusText(row.status) }}
              </el-tag>
              <span v-if="row.responseTime > 0" style="font-size: 12px; color: #909399;">
                {{ row.responseTime }}ms
              </span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="450" fixed="right">
          <template #default="{ row }">
            <el-button-group>
              <el-tooltip content="添加通道" placement="top">
                <el-button 
                  type="success" 
                  size="small"
                  @click.stop="showAddChannelDialog(row)">
                  ➕
                </el-button>
              </el-tooltip>
              <el-tooltip content="预览流地址" placement="top">
                <el-button 
                  type="success" 
                  size="small"
                  :disabled="!row.previewURL"
                  @click.stop="showPreview(row)">
                  🎬
                </el-button>
              </el-tooltip>
              <el-tooltip content="配置文件" placement="top">
                <el-button 
                  type="info" 
                  size="small"
                  @click.stop="showProfiles(row)">
                  📋
                </el-button>
              </el-tooltip>
              <el-tooltip content="编辑凭证" placement="top">
                <el-button 
                  type="warning" 
                  size="small"
                  @click.stop="showEditCredentials(row)">
                  🔐
                </el-button>
              </el-tooltip>
              <el-tooltip content="更新IP" placement="top">
                <el-button 
                  size="small"
                  @click.stop="showUpdateIPModal_func(row)">
                  🔄
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

    <!-- 手动添加设备对话框 -->
    <el-dialog 
      v-model="showAddModal" 
      title="手动添加ONVIF设备"
      width="500px"
      @close="resetAddForm">
      <el-form 
        :model="addForm"
        ref="addFormRef"
        :rules="addFormRules"
        label-width="120px">
        <el-form-item label="添加方式" prop="method">
          <el-radio-group v-model="addForm.method">
            <el-radio label="ip">IP:Port方式</el-radio>
            <el-radio label="xaddr">XADDR方式</el-radio>
          </el-radio-group>
        </el-form-item>

        <!-- IP:Port方式 -->
        <template v-if="addForm.method === 'ip'">
          <el-form-item label="IP地址" prop="ip">
            <el-input 
              v-model="addForm.ip" 
              placeholder="例: 192.168.1.100"
              clearable></el-input>
          </el-form-item>
          <el-form-item label="端口" prop="port">
            <el-input-number 
              v-model="addForm.port" 
              :min="1" 
              :max="65535"
              placeholder="例: 8080"></el-input-number>
          </el-form-item>
        </template>

        <!-- XADDR方式 -->
        <template v-if="addForm.method === 'xaddr'">
          <el-form-item label="XADDR" prop="xaddr">
            <el-input 
              v-model="addForm.xaddr" 
              placeholder="例: http://192.168.1.100:8080/onvif/device_service"
              clearable></el-input>
          </el-form-item>
        </template>

        <el-form-item label="用户名" prop="username">
          <el-input 
            v-model="addForm.username" 
            placeholder="默认: admin"
            clearable></el-input>
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input 
            v-model="addForm.password" 
            type="password"
            placeholder="设备密码"
            clearable></el-input>
        </el-form-item>
        <el-form-item label="设备名称" prop="name">
          <el-input 
            v-model="addForm.name" 
            placeholder="可选：自定义设备名称"
            clearable></el-input>
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="showAddModal = false">取消</el-button>
        <el-button type="primary" @click="addDevice" :loading="addLoading">
          添加设备
        </el-button>
      </template>
    </el-dialog>



    <!-- 更新IP对话框 -->
    <el-dialog 
      v-model="showUpdateIPModal" 
      title="更新设备IP地址"
      width="400px"
      @close="resetUpdateIPForm">
      <el-form 
        :model="updateIPForm"
        label-width="100px">
        <el-form-item label="当前IP">
          <el-input 
            v-model="updateIPForm.oldIP" 
            disabled></el-input>
        </el-form-item>
        <el-form-item label="新IP地址">
          <el-input 
            v-model="updateIPForm.newIP" 
            placeholder="输入新的IP地址"
            clearable></el-input>
        </el-form-item>
        <el-form-item label="新端口">
          <el-input-number 
            v-model="updateIPForm.newPort" 
            :min="1" 
            :max="65535"></el-input-number>
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="showUpdateIPModal = false">取消</el-button>
        <el-button type="primary" @click="updateDeviceIP" :loading="updateIPLoading">
          更新IP
        </el-button>
      </template>
    </el-dialog>

    <!-- 设备预览对话框 -->
    <el-dialog 
      v-model="previewData.showDialog" 
      :title="`设备预览 - ${previewData.device?.name}`"
      width="900px"
      draggable
      :modal="false"
      @close="stopPreview"
      @open="onPreviewDialogOpen">
      <div class="preview-container">
        <!-- 凭证输入区域 -->
        <div class="credentials-form" v-if="!previewData.streamInfo && !previewData.loading">
          <el-alert 
            v-if="previewData.error && previewData.error.includes('401')"
            title="RTSP 认证失败，请输入正确的用户名和密码"
            type="warning"
            :closable="false"
            show-icon
            style="margin-bottom: 16px">
          </el-alert>
          <el-form :inline="true" class="credentials-inline-form">
            <el-form-item label="用户名">
              <el-input v-model="previewData.credentials.username" placeholder="admin" style="width: 150px" />
            </el-form-item>
            <el-form-item label="密码">
              <el-input v-model="previewData.credentials.password" type="password" placeholder="密码" style="width: 150px" show-password />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="startPreviewWithCredentials" :loading="previewData.loading">
                开始预览
              </el-button>
            </el-form-item>
          </el-form>
        </div>

        <!-- 视频播放区域 (使用 PreviewPlayer) -->
        <div class="video-player-wrapper">
          <PreviewPlayer 
            ref="previewPlayerRef" 
            :show="previewData.showDialog" 
            :device="previewData.device ? { deviceId: previewData.device.deviceId || previewData.device.id } : null" 
            :channels="previewData.streamInfo ? [{ channelId: previewData.streamInfo.stream_key || previewData.streamInfo.channel_id }] : []" 
            :selectedChannelId="previewData.streamInfo ? (previewData.streamInfo.stream_key || previewData.streamInfo.channel_id) : ''"
            :showPtz="previewData.device?.ptzSupported === true"
            :ptzDeviceId="previewData.device?.deviceId || previewData.device?.id"
            :profileToken="previewData.selectedProfile || 'PROFILE_000'"
            deviceType="onvif"
          />
        </div>

        <!-- 播放信息 显示由 PreviewPlayer 组件处理 -->

        <!-- 设备信息 -->
        <div class="preview-info">
          <el-descriptions :column="3" border size="small">
            <el-descriptions-item label="设备名称">
              {{ previewData.device?.name }}
            </el-descriptions-item>
            <el-descriptions-item label="设备地址">
              {{ previewData.device?.ip }}:{{ previewData.device?.port }}
            </el-descriptions-item>
            <el-descriptions-item label="在线状态">
              <el-tag 
                :type="previewData.device?.status === 'online' ? 'success' : 'danger'"
                effect="plain" size="small">
                {{ getStatusText(previewData.device?.status) }}
              </el-tag>
            </el-descriptions-item>
          </el-descriptions>
        </div>
      </div>

      <template #footer>
        <el-button @click="copyPreviewURL">📋 复制RTSP地址</el-button>
        <el-button type="danger" @click="stopPreviewAndClose">停止预览</el-button>
      </template>
    </el-dialog>

    <!-- 配置文件对话框 -->
    <el-dialog 
      v-model="profilesData.showDialog" 
      :title="`媒体配置 - ${profilesData.device?.name}`"
      width="900px">
      <el-table :data="profilesData.profiles" v-loading="profilesData.loading" stripe>
        <el-table-column prop="name" label="配置名称" width="120"></el-table-column>
        <el-table-column prop="token" label="Token" width="120"></el-table-column>
        <el-table-column prop="encoding" label="编码" width="80"></el-table-column>
        <el-table-column prop="resolution" label="分辨率" width="120"></el-table-column>
        <el-table-column prop="fps" label="帧率" width="60"></el-table-column>
        <el-table-column prop="bitrate" label="码率(kbps)" width="100"></el-table-column>
        <el-table-column label="操作" width="120">
          <template #default="{ row }">
            <el-button size="small" type="primary" @click="getStreamByProfile(row.token)">
              获取流
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <template #footer>
        <el-button @click="profilesData.showDialog = false">关闭</el-button>
      </template>
    </el-dialog>

    <!-- 添加通道选择对话框 -->
    <el-dialog 
      v-model="addChannelData.showDialog" 
      :title="`添加通道 - ${addChannelData.device?.name}`"
      width="800px">
      <el-alert 
        type="info" 
        :closable="false"
        style="margin-bottom: 16px;">
        <template #title>
          选择要添加到通道管理的Profile配置
        </template>
      </el-alert>
      
      <el-table 
        :data="addChannelData.profiles" 
        v-loading="addChannelData.loading"
        @selection-change="handleChannelSelectionChange"
        stripe>
        <el-table-column type="selection" width="55"></el-table-column>
        <el-table-column prop="name" label="配置名称" width="120"></el-table-column>
        <el-table-column prop="token" label="Token" width="120"></el-table-column>
        <el-table-column prop="encoding" label="编码" width="80"></el-table-column>
        <el-table-column prop="resolution" label="分辨率" width="120"></el-table-column>
        <el-table-column prop="fps" label="帧率" width="70"></el-table-column>
        <el-table-column prop="bitrate" label="码率" width="100">
          <template #default="{ row }">
            {{ row.bitrate }} kbps
          </template>
        </el-table-column>
      </el-table>

      <template #footer>
        <el-button @click="addChannelData.showDialog = false">取消</el-button>
        <el-button 
          type="primary" 
          @click="confirmAddChannels"
          :disabled="addChannelData.selectedProfiles.length === 0"
          :loading="addChannelData.adding">
          添加选中通道 ({{ addChannelData.selectedProfiles.length }})
        </el-button>
      </template>
    </el-dialog>

    <!-- 编辑凭证对话框 -->
    <el-dialog 
      v-model="credentialsData.showDialog" 
      :title="`编辑凭证 - ${credentialsData.device?.name}`"
      width="500px">
      <el-form 
        ref="credentialsFormRef"
        :model="credentialsForm"
        :rules="credentialsFormRules"
        label-width="120px">
        <el-form-item label="设备地址">
          <el-input 
            v-model="credentialsForm.ip" 
            :placeholder="`${credentialsData.device?.ip}:${credentialsData.device?.port}`"
            disabled />
        </el-form-item>
        <el-form-item label="用户名" prop="username">
          <el-input 
            v-model="credentialsForm.username" 
            :placeholder="credentialsData.device?.username || 'admin'"
            clearable />
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input 
            v-model="credentialsForm.password" 
            type="password"
            :placeholder="credentialsData.device?.password || '默认密码'"
            show-password
            clearable />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="credentialsData.showDialog = false">取消</el-button>
        <el-button 
          type="primary" 
          @click="updateCredentials"
          :loading="credentialsData.loading">
          更新凭证
        </el-button>
      </template>
    </el-dialog>


  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { VideoCamera } from '@element-plus/icons-vue'
import PreviewPlayer from '../components/PreviewPlayer.vue'

interface Device {
  deviceId: string
  name: string
  ip: string
  port: number
  manufacturer: string
  model: string
  firmwareVersion?: string
  serialNumber?: string
  status: string
  username: string
  password: string
  previewURL?: string
  snapshotURL?: string
  responseTime?: number
  lastCheckTime?: string
  discoveryTime?: string
  lastSeenTime?: string
  checkInterval?: number
  ptzSupported?: boolean
  audioSupported?: boolean
  services?: string[]
  failureCount?: number
}

interface PTZPreset {
  token: string
  name: string
}

interface MediaProfile {
  token: string
  name: string
  encoding: string
  resolution: string
  width: number
  height: number
  fps: number
  bitrate: number
}

const devices = ref<Device[]>([])
const loading = ref(false)
const searchText = ref('')
const statusFilter = ref('')

// 统计数据
const statistics = computed(() => {
  const total = devices.value.length
  const online = devices.value.filter(d => d.status === 'online').length
  const offline = devices.value.filter(d => d.status === 'offline').length
  const ptzDevices = devices.value.filter(d => d.ptzSupported).length
  return { total, online, offline, ptzDevices }
})

// 过滤后的设备列表
const filteredDevices = computed(() => {
  return devices.value.filter(device => {
    const matchSearch = !searchText.value || 
      device.name?.toLowerCase().includes(searchText.value.toLowerCase()) ||
      device.ip?.includes(searchText.value) ||
      device.manufacturer?.toLowerCase().includes(searchText.value.toLowerCase())
    const matchStatus = !statusFilter.value || device.status === statusFilter.value
    return matchSearch && matchStatus
  })
})

// 流信息接口
interface StreamInfo {
  device_id: string
  stream_key: string
  app: string
  stream: string
  source_url: string
  flv_url: string
  ws_flv_url: string
  hls_url: string
  rtmp_url: string
  rtsp_url: string
  create_time: number
}

// 预览数据
const previewData = reactive({
  showDialog: false,
  device: null as Device | null,
  loading: false,
  error: '',
  streamInfo: null as StreamInfo | null,
  flvPlayer: null as any,
  // 凭证信息 - 用于 RTSP 认证
  credentials: {
    username: '',
    password: ''
  },
  // 当前使用的 profile token (用于 PTZ 控制)
  selectedProfile: 'PROFILE_000'
})

// Preview player ref
const previewPlayerRef = ref<any>(null)

// 配置文件数据
const profilesData = reactive({
  showDialog: false,
  device: null as Device | null,
  profiles: [] as MediaProfile[],
  loading: false
})

// 添加通道数据
const addChannelData = reactive({
  showDialog: false,
  device: null as Device | null,
  profiles: [] as MediaProfile[],
  selectedProfiles: [] as MediaProfile[],
  loading: false,
  adding: false
})

// 编辑凭证数据
const credentialsFormRef = ref()
const credentialsData = reactive({
  showDialog: false,
  device: null as Device | null,
  loading: false
})

const credentialsForm = reactive({
  ip: '',
  username: '',
  password: ''
})

const credentialsFormRules = {
  username: [{ required: true, message: '用户名必填', trigger: 'change' }],
  password: [{ required: true, message: '密码必填', trigger: 'change' }]
}

// 手动添加表单
const showAddModal = ref(false)
const addLoading = ref(false)
const addFormRef = ref()
const addForm = reactive({
  method: 'ip',
  ip: '',
  port: 8080,
  xaddr: '',
  username: 'admin',
  password: '',
  name: ''
})
const addFormRules = {
  ip: [{ required: true, message: 'IP地址必填', trigger: 'change' }],
  port: [{ required: true, message: '端口必填', trigger: 'change' }],
  xaddr: [{ required: true, message: 'XADDR必填', trigger: 'change' }],
  username: [{ required: true, message: '用户名必填', trigger: 'change' }],
  password: [{ required: true, message: '密码必填', trigger: 'change' }]
}



// 更新IP表单
const showUpdateIPModal = ref(false)
const updateIPLoading = ref(false)
const updateIPForm = reactive({
  deviceID: '',
  oldIP: '',
  newIP: '',
  newPort: 8080
})



// 自动刷新定时器
let refreshTimer: ReturnType<typeof setInterval> | null = null

// 获取设备列表
const refreshDevices = async () => {
  loading.value = true
  try {
    const response = await fetch('/api/onvif/devices')
    if (!response.ok) throw new Error('获取设备列表失败')
    const data = await response.json()
    devices.value = data.devices || []
  } catch (error) {
    ElMessage.error(`加载失败: ${error}`)
  } finally {
    loading.value = false
  }
}



// 添加设备
const addDevice = async () => {
  if (!addFormRef.value) return
  await addFormRef.value.validate()

  addLoading.value = true
  try {
    const payload: any = {
      username: addForm.username || 'admin',
      password: addForm.password
    }

    if (addForm.method === 'ip') {
      payload.ip = addForm.ip
      payload.port = addForm.port
    } else {
      payload.xaddr = addForm.xaddr
    }

    if (addForm.name) {
      payload.name = addForm.name
    }

    const response = await fetch('/api/onvif/devices', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    })

    if (!response.ok) throw new Error('添加失败')
    const data = await response.json()
    
    ElMessage.success('设备添加成功')
    showAddModal.value = false
    resetAddForm()
    refreshDevices()
  } catch (error) {
    ElMessage.error(`添加失败: ${error}`)
  } finally {
    addLoading.value = false
  }
}



// 显示更新IP对话框
const showUpdateIPModal_func = (row: Device) => {
  updateIPForm.deviceID = row.deviceId
  updateIPForm.oldIP = row.ip
  updateIPForm.newIP = row.ip
  updateIPForm.newPort = row.port
  showUpdateIPModal.value = true
}

// 更新设备IP
const updateDeviceIP = async () => {
  if (!updateIPForm.newIP) {
    ElMessage.error('请输入新IP地址')
    return
  }

  updateIPLoading.value = true
  try {
    const response = await fetch(`/api/onvif/devices/${updateIPForm.deviceID}/refresh`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        new_ip: updateIPForm.newIP,
        new_port: updateIPForm.newPort
      })
    })

    if (!response.ok) throw new Error('更新失败')
    
    ElMessage.success('设备已刷新')
    showUpdateIPModal.value = false
    resetUpdateIPForm()
    refreshDevices()
  } catch (error) {
    ElMessage.error(`更新失败: ${error}`)
  } finally {
    updateIPLoading.value = false
  }
}

// 显示设备预览（自动启动播放）
const showPreview = async (row: Device) => {
  console.log('[ONVIFDeviceManager] showPreview - row:', row, 'ptzSupported:', row.ptzSupported)
  previewData.device = row
  previewData.error = ''
  previewData.streamInfo = null
  // 初始化凭证 - 使用设备保存的凭证或默认值
  previewData.credentials.username = row.username || 'admin'
  previewData.credentials.password = row.password || 'a123456'
  // 初始化 profile token - 默认使用 PROFILE_000
  previewData.selectedProfile = 'PROFILE_000'
  previewData.showDialog = true
  
  console.log('[ONVIFDeviceManager] previewData.device.ptzSupported:', previewData.device?.ptzSupported)
  
  // 自动启动预览
  await nextTick()
  startPreviewWithCredentials()
}

// 表格行点击处理（兼容模板绑定）
const handleRowClick = (row: Device) => {
  // 简单切换选中状态或展开行，当前实现为打开详情（可根据需要调整）
  // 这里保持行为与之前的 handleRowClick 预期一致：设置当前选中设备并展开（如果需要）
  // 暂时将其行为设为：将设备设为选中（用于未来扩展）
  // 如果你期望点击行打开某个侧边栏或详情页，请告知我以实现。
  console.debug('row clicked', row)
}

// 将内部状态码转成人类可读文本
const getStatusText = (status: string | undefined) => {
  if (!status) return '未知'
  if (status === 'online') return '在线'
  if (status === 'offline') return '离线'
  return status
}

// 由 PreviewPlayer 组件处理播放逻辑与错误
const onPreviewDialogOpen = () => {
  // 对话框打开时重置错误状态（预览已在 showPreview 中自动启动）
  previewData.error = ''
}

// 在进行关键操作前，统一验证设备凭证并在验证成功后同步通道到通道管理
const ensureDeviceAuth = async (device: Device) => {
  if (!device) return false

  try {
    const profilesResp = await fetch(`/api/onvif/devices/${encodeURIComponent(device.deviceId)}/profiles`)
    if (profilesResp.ok) {
      return true
    }

    // 如果返回 JSON，展示错误信息
    const err = await profilesResp.json().catch(() => ({}))
    const msg = err.error || err.message || '设备可能需要重新认证或不在线'
    ElMessage.warning(msg)
    return false
  } catch (e: any) {
    console.warn('获取设备配置文件失败，继续使用默认参数', e?.message)
    return true
  }
}

// 在用户输入凭据后启动预览（带重试和错误诊断）
const startPreviewWithCredentials = async () => {
  if (!previewData.device) return
  
  previewData.loading = true
  previewData.error = ''
  const maxRetries = 2
  let lastError = ''

  for (let attempt = 1; attempt <= maxRetries; attempt++) {
    try {
      console.log(`[尝试 ${attempt}/${maxRetries}] 启动ONVIF设备预览 (Profile: ${previewData.selectedProfile})`)
      
      const response = await fetch(`/api/onvif/devices/${previewData.device.deviceId}/preview/start`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          profileToken: previewData.selectedProfile || 'PROFILE_000',
          username: previewData.credentials.username || previewData.device.username || '',
          password: previewData.credentials.password || previewData.device.password || ''
        })
      })
      
      if (!response.ok) {
        const errData = await response.json().catch(() => ({}))
        throw new Error(errData.error || `HTTP ${response.status}: ${response.statusText}`)
      }
      
      const data = await response.json()
      if (!data.success) {
        throw new Error(data.error || '启动预览失败')
      }
      
      previewData.streamInfo = data.data
      previewData.loading = false
      
      // 显示成功提示，告知用户流已添加到媒体流管理
      ElMessage.success({
        message: '预览已启动，流已添加到媒体流管理',
        duration: 3000
      })
      
      // 通知 PreviewPlayer 开始播放
      await nextTick()
      if (previewPlayerRef.value && previewData.streamInfo) {
        const p = (typeof previewPlayerRef.value.startWithStreamInfo === 'function') 
          ? previewPlayerRef.value 
          : (previewPlayerRef.value.value && typeof previewPlayerRef.value.value.startWithStreamInfo === 'function') 
            ? previewPlayerRef.value.value 
            : (previewPlayerRef.value.$ && previewPlayerRef.value.$.exposed && typeof previewPlayerRef.value.$.exposed.startWithStreamInfo === 'function') 
              ? previewPlayerRef.value.$.exposed 
              : null
        if (p) {
          await p.startWithStreamInfo(previewData.streamInfo)
        } else {
          try { if (typeof previewPlayerRef.value.startPreview === 'function') await previewPlayerRef.value.startPreview() } catch (_) {}
        }
      }
      
      return // 成功，退出循环
    } catch (e: any) {
      lastError = e.message || String(e)
      console.warn(`[失败 ${attempt}/${maxRetries}] 启动预览失败: ${lastError}`)
      
      if (attempt < maxRetries) {
        await new Promise(resolve => setTimeout(resolve, 1500))
      }
    }
  }

  // 所有重试都失败了
  previewData.loading = false
  previewData.error = lastError
  
  // 解析错误信息，提供诊断建议
  const showDetailedError = () => {
    let title = '启动预览失败'
    let message = lastError
    
    // 检查特定的错误类型
    if (lastError.includes('RTSP')) {
      title = 'RTSP 地址不可用'
      message = `${lastError}\n\n排查步骤：\n1. 检查设备是否在线（检查设备管理中的状态）\n2. 尝试在编辑凭证中修改凭证后重试\n3. 检查网络连接\n4. 如果问题持续，请查看服务器日志`
    } else if (lastError.includes('认证') || lastError.includes('401')) {
      title = 'RTSP 认证失败'
      message = `${lastError}\n\n请检查：\n1. 用户名和密码是否正确\n2. 点击"编辑凭证"更新设备凭据\n3. 重试启动预览`
    } else if (lastError.includes('Connection') || lastError.includes('dial')) {
      title = '无法连接到设备'
      message = `${lastError}\n\n请检查：\n1. 设备是否在线\n2. 网络连接是否正常\n3. 防火墙是否阻止了连接`
    } else if (lastError.includes('500') || lastError.includes('Internal')) {
      title = '服务器错误'
      message = `${lastError}\n\n请查看服务器日志获取更多信息`
    }
    
    ElMessageBox.alert(message, title, {
      confirmButtonText: '关闭',
      type: 'error',
      dangerouslyUseHTMLString: false
    })
  }
  
  ElMessage.error(`启动预览失败: ${lastError.substring(0, 100)}...`)
  
  // 延迟显示详细信息，避免与错误消息冲突
  setTimeout(() => {
    showDetailedError()
  }, 500)
}

// 停止预览并关闭对话框
const stopPreviewAndClose = async () => {
  // 仅停止播放并调用后端停止代理
  if (previewPlayerRef.value) await previewPlayerRef.value.stopPlaybackOnly()
  if (previewData.device && previewData.streamInfo) {
    try {
      await fetch(`/api/onvif/devices/${previewData.device.deviceId}/preview/stop`, { method: 'POST', headers: { 'Content-Type': 'application/json' } })
    } catch (e) { console.warn('stop preview api', e) }
  }
  previewData.streamInfo = null
  previewData.error = ''
  previewData.showDialog = false
}

// 兼容模板中 @close="stopPreview" 的调用，调用 stopPreviewAndClose
const stopPreview = async () => {
  await stopPreviewAndClose()
}

// 复制到剪贴板
const copyToClipboard = async (text: string) => {
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success('已复制到剪贴板')
  } catch (e) {
    ElMessage.error('复制失败')
  }
}

// (重复的 showProfiles 已删除，使用文件后部定义的带认证版本)

// 根据配置获取流并播放（带重试机制）
const getStreamByProfile = async (profileToken: string) => {
  if (!profilesData.device) return

  const maxRetries = 2
  let lastError = ''

  // 关闭配置文件对话框，打开预览对话框
  profilesData.showDialog = false
  previewData.device = profilesData.device
  previewData.error = ''
  previewData.streamInfo = null
  previewData.credentials.username = profilesData.device.username || 'admin'
  previewData.credentials.password = profilesData.device.password || 'a123456'
  previewData.selectedProfile = profileToken // 保存当前使用的 profile token
  previewData.showDialog = true
  previewData.loading = true

  for (let attempt = 1; attempt <= maxRetries; attempt++) {
    try {
      console.log(`[尝试 ${attempt}/${maxRetries}] 获取流地址并播放 (Profile: ${profileToken})`)
      
      const response = await fetch(`/api/onvif/devices/${profilesData.device.deviceId}/preview/start`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ 
          profileToken,
          username: previewData.credentials.username,
          password: previewData.credentials.password
        })
      })
      
      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}))
        throw new Error(errorData.error || `HTTP ${response.status}`)
      }
      
      const data = await response.json()

      if (data && data.success && data.data) {
        previewData.streamInfo = data.data
        previewData.loading = false
        
        // 通知 PreviewPlayer 开始播放
        await nextTick()
        if (previewPlayerRef.value && previewData.streamInfo) {
          const p = (typeof previewPlayerRef.value.startWithStreamInfo === 'function') 
            ? previewPlayerRef.value 
            : (previewPlayerRef.value.value && typeof previewPlayerRef.value.value.startWithStreamInfo === 'function') 
              ? previewPlayerRef.value.value 
              : (previewPlayerRef.value.$ && previewPlayerRef.value.$.exposed && typeof previewPlayerRef.value.$.exposed.startWithStreamInfo === 'function') 
                ? previewPlayerRef.value.$.exposed 
                : null
          if (p) {
            await p.startWithStreamInfo(previewData.streamInfo)
          } else {
            try { if (typeof previewPlayerRef.value.startPreview === 'function') await previewPlayerRef.value.startPreview() } catch (_) {}
          }
        }
        
        ElMessage.success({
          message: `使用配置 ${profileToken} 启动播放成功，流已添加到媒体流管理`,
          duration: 3000
        })
        return
      } else {
        throw new Error(data?.message || '启动预览失败')
      }
    } catch (error: any) {
      lastError = error.message
      console.warn(`[失败 ${attempt}/${maxRetries}] ${lastError}`)
      
      if (attempt < maxRetries) {
        await new Promise(resolve => setTimeout(resolve, 800))
      }
    }
  }

  // 所有重试都失败了
  previewData.loading = false
  previewData.error = lastError
  ElMessage.error(`获取流地址失败: ${lastError}`)
}

// 显示添加通道对话框
const showAddChannelDialog = async (row: Device) => {
  addChannelData.device = row
  addChannelData.showDialog = true
  addChannelData.loading = true
  addChannelData.selectedProfiles = []
  
  try {
    // 获取设备的Profile列表（使用GET方法）
    const response = await fetch(`/api/onvif/devices/${row.deviceId}/profiles`, {
      method: 'GET',
      headers: { 'Content-Type': 'application/json' }
    })
    
    if (!response.ok) {
      const errorData = await response.json().catch(() => ({}))
      throw new Error(errorData.message || `HTTP ${response.status}`)
    }
    
    const data = await response.json()
    if (data && data.profiles) {
      addChannelData.profiles = data.profiles
      if (data.profiles.length === 0) {
        ElMessage.warning('设备没有可用的配置文件')
      }
    } else {
      throw new Error(data?.error || '获取配置失败')
    }
  } catch (error: any) {
    console.error('获取配置列表失败:', error)
    ElMessage.error('获取配置列表失败: ' + error.message)
    addChannelData.showDialog = false
  } finally {
    addChannelData.loading = false
  }
}

// 处理通道选择变化
const handleChannelSelectionChange = (selection: MediaProfile[]) => {
  addChannelData.selectedProfiles = selection
}

// 确认添加选中的通道
const confirmAddChannels = async () => {
  if (addChannelData.selectedProfiles.length === 0) {
    ElMessage.warning('请至少选择一个配置')
    return
  }
  
  addChannelData.adding = true
  const device = addChannelData.device
  let successCount = 0
  let failCount = 0
  
  try {
    for (const profile of addChannelData.selectedProfiles) {
      try {
        const channelData = {
          // ONVIF设备不提供channelId，让后端自动生成
          channelName: `${device?.name}-${profile.name}`,
          deviceId: device?.deviceId,
          deviceType: 'onvif',
          status: device?.status,
          manufacturer: device?.manufacturer,
          model: device?.model,
          profileToken: profile.token,
          resolution: profile.resolution,
          encoding: profile.encoding,
          fps: profile.fps,
          bitrate: profile.bitrate,
          streamUrl: '',
        }
        
        const response = await fetch('/api/channel/add', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(channelData)
        })
        
        const result = await response.json()
        
        // 后端返回 status: "ok" 表示成功
        if (result.status === 'ok' || result.success) {
          successCount++
        } else {
          failCount++
          console.error(`添加配置 ${profile.name} 失败:`, result.message || result.error)
        }
      } catch (error) {
        failCount++
        console.error(`添加配置 ${profile.name} 失败:`, error)
      }
    }
    
    if (successCount > 0) {
      ElMessage.success(`成功添加 ${successCount} 个通道${failCount > 0 ? `，失败 ${failCount} 个` : ''}`)
      addChannelData.showDialog = false
    } else {
      ElMessage.error('所有通道添加失败')
    }
  } finally {
    addChannelData.adding = false
  }
}

// 显示编辑凭证对话框
const showEditCredentials = (row: Device) => {
  credentialsData.device = row
  credentialsForm.ip = `${row.ip}:${row.port}`
  credentialsForm.username = row.username || ''
  credentialsForm.password = row.password || ''
  credentialsData.showDialog = true
}

// 重置凭证表单
const resetCredentialsForm = () => {
  credentialsForm.ip = ''
  credentialsForm.username = ''
  credentialsForm.password = ''
}

// 更新设备凭证
const updateCredentials = async () => {
  if (!credentialsFormRef.value) return
  
  try {
    await credentialsFormRef.value.validate()
  } catch {
    return
  }
  
  if (!credentialsData.device) {
    ElMessage.error('设备信息丢失')
    return
  }
  
  credentialsData.loading = true
  try {
    const deviceId = credentialsData.device?.deviceId || credentialsData.device?.id
    const response = await fetch(`/api/onvif/devices/${encodeURIComponent(deviceId)}/credentials`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        username: credentialsForm.username,
        password: credentialsForm.password
      })
    })
    
    if (!response.ok) {
      const errData = await response.json().catch(() => ({}))
      throw new Error(errData.error || '更新凭证失败')
    }
    
    const result = await response.json()
    
    // 更新本地设备列表
    const device = devices.value.find(d => d.deviceId === credentialsData.device?.deviceId)
    if (device) {
      device.username = credentialsForm.username
      device.password = credentialsForm.password
    }
    
    ElMessage.success('凭证已更新')
    credentialsData.showDialog = false
    resetCredentialsForm()
  } catch (error: any) {
    ElMessage.error(`更新失败: ${error.message}`)
  } finally {
    credentialsData.loading = false
  }
}

// 显示配置文件（带重试机制和详细错误处理）
const showProfiles = async (row: Device) => {
  profilesData.device = row
  profilesData.showDialog = true
  profilesData.loading = true
  
  const maxRetries = 3
  let lastError = ''
  
  // 重试机制：最多重试 3 次
  for (let attempt = 1; attempt <= maxRetries; attempt++) {
    try {
      console.log(`[尝试 ${attempt}/${maxRetries}] 获取设备 ${row.deviceId} 的配置文件`)
      
      const response = await fetch(`/api/onvif/devices/${row.deviceId}/profiles`, {
        method: 'GET',
        headers: { 'Content-Type': 'application/json' },
        timeout: 15000 // 设置 15 秒超时
      })
      
      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}))
        throw new Error(errorData.message || `HTTP ${response.status}: ${response.statusText}`)
      }
      
      const data = await response.json()
      
      // 成功获取配置文件
      if (data.profiles && data.profiles.length > 0) {
        profilesData.profiles = data.profiles
        ElMessage.success(`成功获取 ${data.profiles.length} 个媒体配置`)
        profilesData.loading = false
        return
      } else if (data.profiles) {
        profilesData.profiles = []
        ElMessage.warning('设备没有可用的媒体配置文件')
        profilesData.loading = false
        return
      }
      
      throw new Error('响应数据格式错误')
    } catch (error: any) {
      lastError = error.message || String(error)
      console.warn(`[失败 ${attempt}/${maxRetries}] ${lastError}`)
      
      // 如果还有重试次数，等待 1 秒后重试
      if (attempt < maxRetries) {
        await new Promise(resolve => setTimeout(resolve, 1000))
      }
    }
  }
  
  // 所有重试都失败了
  profilesData.loading = false
  profilesData.profiles = []
  
  // 提供更详细的错误信息和诊断建议
  const errorMessage = `获取配置文件失败: ${lastError}`
  ElMessageBox.confirm(
    `${errorMessage}\n\n可能原因：\n1. 设备凭证不正确或已过期\n2. 设备暂时离线\n3. 网络连接不稳定\n4. 设备不支持该操作\n\n建议：\n- 检查凭证是否正确（点击编辑凭证按钮）\n- 尝试刷新设备列表\n- 检查网络连接\n- 稍后重试`,
    '配置文件获取失败',
    {
      confirmButtonText: '编辑凭证',
      cancelButtonText: '关闭',
      type: 'warning'
    }
  ).then(() => {
    // 用户点击编辑凭证
    showEditCredentials(row)
    profilesData.showDialog = false
  }).catch(() => {
    // 用户点击关闭
  })
}

// 根据服务类型友好展示服务名
const getServiceName = (service: string) => {
  if (!service) return ''
  if (service.includes('Media')) return 'Media'
  if (service.includes('PTZ')) return 'PTZ'
  if (service.includes('Event')) return 'Events'
  if (service.includes('Device')) return 'Device'
  if (service.includes('Imaging')) return 'Imaging'
  if (service.includes('Recording')) return 'Recording'
  return service.split('/').pop() || service
}

// 格式化时间
const formatTime = (timeStr: string | undefined) => {
  if (!timeStr) return '-'
  try {
    const date = new Date(timeStr)
    return date.toLocaleString('zh-CN')
  } catch {
    return timeStr
  }
}

// 删除设备
const deleteDevice = (row: Device) => {
  ElMessageBox.confirm(
    `确定删除设备"${row.name}"吗？`,
    '删除确认',
    { confirmButtonText: '确定', cancelButtonText: '取消', type: 'warning' }
  )
    .then(async () => {
      try {
        const response = await fetch(`/api/onvif/devices/${encodeURIComponent(row.deviceId)}`, {
          method: 'DELETE'
        })

        if (!response.ok) throw new Error('删除失败')
        
        ElMessage.success('设备已删除')
        refreshDevices()
      } catch (error) {
        ElMessage.error(`删除失败: ${error}`)
      }
    })
    .catch(() => {})
}

// 复制预览URL到剪贴板
const copyPreviewURL = async () => {
  if (!previewData.device?.previewURL) {
    ElMessage.error('没有预览地址可复制')
    return
  }

  try {
    await navigator.clipboard.writeText(previewData.device.previewURL)
    ElMessage.success('预览地址已复制到剪贴板')
  } catch (error) {
    ElMessage.error('复制失败，请手动复制')
  }
}

// 重置表单
const resetAddForm = () => {
  addForm.method = 'ip'
  addForm.ip = ''
  addForm.port = 8080
  addForm.xaddr = ''
  addForm.username = 'admin'
  addForm.password = ''
  addForm.name = ''
}



const resetUpdateIPForm = () => {
  updateIPForm.deviceID = ''
  updateIPForm.oldIP = ''
  updateIPForm.newIP = ''
  updateIPForm.newPort = 8080
}

// 组件挂载
onMounted(() => {
  refreshDevices()
  // 设置自动刷新（每30秒）
  refreshTimer = setInterval(refreshDevices, 30000)
})

// 组件卸载
onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
})
</script>

<style scoped lang="css">
.onvif-device-manager {
  padding: 20px;
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

:deep(.el-button) {
  display: flex;
  align-items: center;
  gap: 4px;
}

.preview-container {
  padding: 20px 0;
}

.credentials-form {
  background: #f5f7fa;
  padding: 16px;
  border-radius: 8px;
  margin-bottom: 16px;
}

.credentials-inline-form {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.credentials-inline-form .el-form-item {
  margin-bottom: 0;
}

.preview-url {
  margin-bottom: 20px;
  word-break: break-all;
}

.preview-info {
  margin: 20px 0;
}

/* 视频播放器样式 */
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

:deep(.el-descriptions) {
  margin-bottom: 20px;
}

:deep(.el-descriptions-item__label) {
  font-weight: 600;
}

/* 发现设备对话框样式 */

</style>
