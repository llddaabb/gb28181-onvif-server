<template>
  <div class="multi-preview">
    <!-- 左侧通道树 -->
    <div class="left-panel">
      <el-card shadow="hover" class="tree-card">
        <template #header>
          <div class="panel-header">
            <span class="panel-title">📺 通道列表</span>
            <el-button type="primary" link :icon="Refresh" @click="fetchChannels" :loading="loadingChannels">
              刷新
            </el-button>
          </div>
        </template>
        
        <el-input
          v-model="channelSearchKeyword"
          placeholder="搜索通道..."
          :prefix-icon="Search"
          clearable
          style="margin-bottom: 12px;"
        />
        
        <div class="channel-tree" v-loading="loadingChannels">
          <el-tree
            ref="treeRef"
            :data="channelTreeData"
            :props="treeProps"
            node-key="id"
            :filter-node-method="filterNode"
            highlight-current
            default-expand-all
            @node-click="handleNodeClick"
          >
            <template #default="{ node, data }">
              <div class="tree-node">
                <span class="node-icon">{{ data.icon }}</span>
                <span class="node-label" :title="node.label">{{ node.label }}</span>
                <el-tag v-if="data.status" size="small" :type="data.status === 'ON' ? 'success' : 'info'">
                  {{ data.status === 'ON' ? '在线' : '离线' }}
                </el-tag>
              </div>
            </template>
          </el-tree>
          
          <el-empty v-if="channelTreeData.length === 0 && !loadingChannels" description="暂无通道数据" :image-size="80" />
        </div>
      </el-card>
    </div>

    <!-- 右侧预览区 -->
    <div class="right-panel">
      <!-- 工具栏 -->
      <el-card shadow="hover" class="toolbar-card">
        <div class="toolbar">
          <div class="toolbar-left">
            <span class="title">🖥️ 多画面预览</span>
            <el-divider direction="vertical" />
            <el-select v-model="currentLayout" placeholder="选择布局" style="width: 140px;" @change="changeLayout">
              <el-option label="4 画面 (2x2)" :value="4" />
              <el-option label="6 画面 (3x2)" :value="6" />
              <el-option label="9 画面 (3x3)" :value="9" />
              <el-option label="16 画面 (4x4)" :value="16" />
              <el-option label="25 画面 (5x5)" :value="25" />
              <el-option label="32 画面 (8x4)" :value="32" />
              <el-option label="自定义" value="custom" />
            </el-select>
            
            <!-- 自定义布局 -->
            <template v-if="currentLayout === 'custom'">
              <el-input-number v-model="customCols" :min="1" :max="10" size="small" style="width: 80px;" />
              <span style="margin: 0 4px;">x</span>
              <el-input-number v-model="customRows" :min="1" :max="10" size="small" style="width: 80px;" />
              <el-button type="primary" size="small" @click="applyCustomLayout">应用</el-button>
            </template>
          </div>
          
          <div class="toolbar-right">
            <el-dropdown trigger="click" @command="handlePresetCommand">
              <el-button type="primary">
                预设方案 <el-icon class="el-icon--right"><ArrowDown /></el-icon>
              </el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="save">💾 保存当前方案</el-dropdown-item>
                  <el-dropdown-item divided disabled v-if="savedPresets.length === 0">暂无保存的方案</el-dropdown-item>
                  <el-dropdown-item 
                    v-for="preset in savedPresets" 
                    :key="preset.id" 
                    :command="`load:${preset.id}`"
                  >
                    📋 {{ preset.name }}
                  </el-dropdown-item>
                  <el-dropdown-item divided command="manage" v-if="savedPresets.length > 0">
                    ⚙️ 管理方案
                  </el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
            
            <el-button type="success" :icon="Refresh" @click="refreshAllStreams">刷新</el-button>
            <el-button type="warning" @click="clearAllStreams">清空</el-button>
            <el-button :icon="FullScreen" @click="toggleFullscreen">{{ isFullscreen ? '退出' : '全屏' }}</el-button>
          </div>
        </div>
      </el-card>

      <!-- 多画面网格 -->
      <div 
        ref="previewContainer" 
        class="preview-grid-container" 
        :class="{ 'fullscreen': isFullscreen }"
      >
        <div class="preview-grid" :style="gridStyle">
          <div 
            v-for="(slot, index) in slots" 
            :key="index" 
            class="preview-slot"
            :class="{ 'active': selectedSlot === index, 'has-stream': slot.streamKey }"
            @click="selectSlot(index)"
          >
            <!-- 视频播放器 -->
            <div class="video-wrapper" v-if="slot.streamKey">
              <PreviewPlayer
                :ref="el => setPlayerRef(index, el)"
                :show="true"
                :device="null"
                :channels="[]"
                :selectedChannelId="''"
                :defaultHeight="'100%'"
                :showPtz="slot.ptzSupported === true && fullscreenSlotIndex === index"
                :ptzSupported="slot.ptzSupported"
                :ptzDeviceId="slot.ptzDeviceId"
                :ptzChannelId="slot.ptzChannelId"
                :deviceType="slot.streamType"
                @fullscreenChange="(isFs: boolean) => onSlotFullscreenChange(index, isFs)"
              />
              <div class="video-overlay">
                <div class="stream-info">
                  <span class="stream-name">{{ slot.streamName || slot.streamKey }}</span>
                </div>
                <div class="video-controls">
                  <el-button type="primary" size="small" circle :icon="Refresh" @click.stop="playStream(index)" />
                  <el-button type="danger" size="small" circle :icon="Close" @click.stop="removeStream(index)" />
                </div>
              </div>
              <div v-if="slot.loading" class="video-loading">
                <el-icon class="is-loading"><Refresh /></el-icon>
                <span>加载中...</span>
              </div>
              <div v-if="slot.error" class="video-error">
                <el-icon><WarningFilled /></el-icon>
                <span>{{ slot.error }}</span>
              </div>
            </div>
            
            <!-- 空槽位 -->
            <div class="empty-slot" v-else>
              <el-icon><VideoCamera /></el-icon>
              <span>点击左侧通道添加</span>
              <span class="slot-number">{{ index + 1 }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 保存方案对话框 -->
    <el-dialog v-model="savePresetVisible" title="保存预览方案" width="400px">
      <el-form label-width="80px">
        <el-form-item label="方案名称" required>
          <el-input v-model="newPresetName" placeholder="请输入方案名称" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="savePresetVisible = false">取消</el-button>
        <el-button type="primary" @click="savePreset" :disabled="!newPresetName">保存</el-button>
      </template>
    </el-dialog>

    <!-- 管理方案对话框 -->
    <el-dialog v-model="managePresetsVisible" title="管理预览方案" width="500px">
      <el-table :data="savedPresets" style="width: 100%">
        <el-table-column prop="name" label="方案名称" />
        <el-table-column prop="layout" label="布局" width="100">
          <template #default="{ row }">
            {{ row.cols }}x{{ row.rows }}
          </template>
        </el-table-column>
        <el-table-column prop="createdAt" label="创建时间" width="160">
          <template #default="{ row }">
            {{ formatDate(row.createdAt) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="120">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click="loadPreset(row.id)">加载</el-button>
            <el-button type="danger" link size="small" @click="deletePreset(row.id)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { ElTree } from 'element-plus'
import { 
  Plus, 
  Close, 
  Refresh, 
  FullScreen, 
  ArrowDown, 
  Search, 
  WarningFilled,
  VideoCamera
} from '@element-plus/icons-vue'
import PreviewPlayer from '../components/PreviewPlayer.vue'

interface PreviewSlot {
  streamKey: string
  streamName: string
  streamUrl: string
  flvUrl?: string
  hlsUrl?: string
  streamType: 'gb28181' | 'onvif' | 'custom'
  loading: boolean
  error: string
  player: any
  // PTZ 支持
  ptzSupported?: boolean
  ptzDeviceId?: string
  ptzChannelId?: string
}

interface TreeNode {
  id: string
  label: string
  icon: string
  type: 'group' | 'gb28181' | 'onvif'
  status?: string
  data?: any
  children?: TreeNode[]
}

interface Preset {
  id: string
  name: string
  cols: number
  rows: number
  slots: Array<{
    streamKey: string
    streamName: string
    streamUrl: string
    streamType: string
  }>
  createdAt: number
}

// 布局配置
const currentLayout = ref<number | 'custom'>(4)
const customCols = ref(3)
const customRows = ref(3)
const cols = ref(2)
const rows = ref(2)

// 槽位数据
const slots = ref<PreviewSlot[]>([])
const selectedSlot = ref<number | null>(null)
const fullscreenSlotIndex = ref<number | null>(null)  // 当前全屏的槽位索引
// per-slot PreviewPlayer refs
const playerRefs = ref<Record<number, any>>({})

const setPlayerRef = (index: number, el: any) => {
  if (!el) {
    delete playerRefs.value[index]
  } else {
    playerRefs.value[index] = el
  }
}

// 处理单个 PreviewPlayer 全屏状态变化
const onSlotFullscreenChange = (index: number, isFullscreen: boolean) => {
  if (isFullscreen) {
    fullscreenSlotIndex.value = index
  } else {
    fullscreenSlotIndex.value = null
  }
}

// Resolve the actual component instance from various possible ref shapes
const resolvePlayer = (el: any) => {
  if (!el) return null
  // already a component public instance
  if (typeof el.startWithStreamInfo === 'function') return el
  // maybe a ref object
  if (el.value && typeof el.value.startWithStreamInfo === 'function') return el.value
  // try internal exposed (Vue internals) - best-effort
  // @ts-ignore
  if (el.$ && el.$.exposed && typeof el.$.exposed.startWithStreamInfo === 'function') return el.$.exposed
  // no usable API found
  return null
}

// 通道树
const treeRef = ref<InstanceType<typeof ElTree>>()
const channelTreeData = ref<TreeNode[]>([])
const channelSearchKeyword = ref('')
const loadingChannels = ref(false)

const treeProps = {
  children: 'children',
  label: 'label'
}

// 预设方案
const savedPresets = ref<Preset[]>([])
const savePresetVisible = ref(false)
const managePresetsVisible = ref(false)
const newPresetName = ref('')

// 全屏
const isFullscreen = ref(false)
const previewContainer = ref<HTMLElement | null>(null)

// 计算网格样式
const gridStyle = computed(() => ({
  gridTemplateColumns: `repeat(${cols.value}, 1fr)`,
  gridTemplateRows: `repeat(${rows.value}, 1fr)`
}))

// 监听搜索关键词变化
watch(channelSearchKeyword, (val) => {
  treeRef.value?.filter(val)
})

// 过滤树节点
const filterNode = (value: string, data: TreeNode) => {
  if (!value) return true
  return data.label.toLowerCase().includes(value.toLowerCase())
}

// 获取通道列表
const fetchChannels = async () => {
  loadingChannels.value = true
  const treeData: TreeNode[] = []
  
  try {
    // 优先从通道列表 API 获取
    let channelsFromApi: any[] = []
    try {
      const channelResponse = await fetch('/api/channel/list')
      const channelData = await channelResponse.json()
      channelsFromApi = channelData.channels || []
    } catch (e) {
      console.log('通道列表 API 不可用，从设备获取')
    }
    
    // 获取 GB28181 设备
    const gb28181Response = await fetch('/api/gb28181/devices')
    const gb28181Data = await gb28181Response.json()
    
    const devices = gb28181Data.devices || []
    if (devices.length > 0) {
      const gb28181Node: TreeNode = {
        id: 'gb28181-root',
        label: 'GB28181 通道',
        icon: '📡',
        type: 'group',
        children: []
      }
      
      for (const device of devices) {
        const deviceNode: TreeNode = {
          id: `gb28181-device-${device.deviceId || device.id}`,
          label: device.deviceId || device.id,
          icon: '📷',
          type: 'group',
          status: device.status,
          children: []
        }
        
        if (device.channels) {
          for (const ch of device.channels) {
            deviceNode.children!.push({
              id: `gb28181-${ch.channelId || ch.id}`,
              label: ch.channelId || ch.id,
              icon: '🎥',
              type: 'gb28181',
              status: ch.status,
              data: { 
                ...ch, 
                channelId: ch.channelId || ch.id,
                name: ch.name || ch.channelName,
                deviceId: device.deviceId || device.id 
              }
            })
          }
        }
        
        if (deviceNode.children!.length > 0) {
          gb28181Node.children!.push(deviceNode)
        }
      }
      
      if (gb28181Node.children!.length > 0) {
        treeData.push(gb28181Node)
      }
    }
    
    // 获取 ONVIF 设备
    const onvifResponse = await fetch('/api/onvif/devices')
    const onvifData = await onvifResponse.json()
    
    const onvifDevices = onvifData.devices || []
    if (onvifDevices.length > 0) {
      const onvifNode: TreeNode = {
        id: 'onvif-root',
        label: 'ONVIF 通道',
        icon: '🌐',
        type: 'group',
        children: onvifDevices.map((d: any) => ({
          id: `onvif-${d.id}`,
          label: d.ip || d.id,
          icon: '📹',
          type: 'onvif' as const,
          status: d.status === 'online' ? 'ON' : d.status,
          data: d
        }))
      }
      treeData.push(onvifNode)
    }
    
    // 如果有从通道 API 获取的数据，添加为独立节点
    if (channelsFromApi.length > 0) {
      // 按设备类型分组
      const gb28181Channels = channelsFromApi.filter(c => c.deviceType === 'gb28181')
      const onvifChannels = channelsFromApi.filter(c => c.deviceType === 'onvif')
      const otherChannels = channelsFromApi.filter(c => !c.deviceType || (c.deviceType !== 'gb28181' && c.deviceType !== 'onvif'))
      
      if (otherChannels.length > 0) {
        const otherNode: TreeNode = {
          id: 'other-root',
          label: '其他通道',
          icon: '📺',
          type: 'group',
          children: otherChannels.map((c: any) => ({
            id: `channel-${c.channelId || c.id}`,
            label: c.channelName || c.name || c.channelId,
            icon: '🎬',
            type: 'gb28181' as const,
            status: c.status === 'online' ? 'ON' : c.status,
            data: c
          }))
        }
        treeData.push(otherNode)
      }
    }
    
  } catch (error) {
    console.error('获取通道列表失败:', error)
    ElMessage.error('获取通道列表失败')
  } finally {
    loadingChannels.value = false
  }
  
  channelTreeData.value = treeData
}

// 处理树节点点击
const handleNodeClick = (data: TreeNode) => {
  // 只处理叶子节点（实际通道）
  if (data.type === 'group') return
  
  addStreamToNextSlot(data)
}

const normalizeStreamId = (channelId?: string) => {
  if (!channelId) return ''
  const sanitized = channelId.replace(/-/g, '')
  return sanitized || channelId
}

const isStreamOnline = async (app: string, streamId: string) => {
  if (!streamId) return false
  try {
    const response = await fetch('/api/zlm/streams')
    const data = await response.json()
    const streams = Array.isArray(data.streams) ? data.streams : []
    return streams.some((stream: any) => stream.app === app && stream.stream === streamId)
  } catch (error) {
    console.error('检查流状态失败:', error)
    return false
  }
}

// 添加流到下一个空槽位
const addStreamToNextSlot = async (data: TreeNode) => {
  let targetIndex = -1
  
  // 优先使用选中的槽位
  if (selectedSlot.value !== null) {
    targetIndex = selectedSlot.value
  } else {
    // 否则找第一个空槽位
    targetIndex = slots.value.findIndex(s => !s.streamKey)
  }
  
  // 如果没有空槽位且没有选中槽位
  if (targetIndex === -1) {
    ElMessage.warning('所有窗口已被占用，请先选择一个窗口或清空')
    return
  }
  
  const slot = slots.value[targetIndex]
  const host = window.location.hostname
  
  // 停止当前播放
  stopStream(targetIndex)
  
  // 标记为加载中
  slot.loading = true
  slot.error = ''
  
  if (data.type === 'gb28181' && data.data) {
    const channel = data.data
    const channelId = channel.channelId || channel.id
    const streamId = normalizeStreamId(channelId)
    const appName = 'live'
    const deviceId = channel.deviceId || data.deviceId
    
    slot.streamKey = channelId
    slot.streamName = channel.name || channel.channelName || channelId
    slot.streamType = 'gb28181'
    // 保存 PTZ 信息
    slot.ptzSupported = channel.ptzSupported === true
    slot.ptzDeviceId = deviceId
    slot.ptzChannelId = channelId
    
    // 先检查流是否已存在（直接尝试播放地址）
    const liveStreamUrl = streamId ? `http://${host}:8080/${appName}/${streamId}.live.flv` : `http://${host}:8080/live/${channelId}.live.flv`
    const rtpStreamUrl = streamId ? `http://${host}:8080/rtp/${streamId}.live.flv` : `http://${host}:8080/rtp/${channelId}.live.flv`
    let streamReady = false
    if (streamId) {
      const online = await isStreamOnline(appName, streamId)
      if (online) {
        slot.streamUrl = liveStreamUrl
        streamReady = true
        console.log('流已存在，直接播放:', liveStreamUrl)
      }
    }
    
    if (!streamReady) {
      try {
        const response = await fetch(`/api/gb28181/devices/${deviceId}/channels/${channelId}/preview/start`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' }
        })
        const result = await response.json()
        if (result.success && result.data) {
          slot.flvUrl = result.data.flv_url ? result.data.flv_url.replace('127.0.0.1', host).replace('localhost', host) : undefined
          slot.hlsUrl = result.data.hls_url ? result.data.hls_url.replace('127.0.0.1', host).replace('localhost', host) : undefined
          slot.streamUrl = slot.hlsUrl || slot.flvUrl || liveStreamUrl
          streamReady = true
          console.log('API 返回的 URLs:', { flv: slot.flvUrl, hls: slot.hlsUrl }, '选用:', slot.streamUrl)
          await new Promise(resolve => setTimeout(resolve, 1000))
        } else if (result.error && result.error.includes('already exists')) {
          slot.streamUrl = liveStreamUrl
          streamReady = true
        }
      } catch (error) {
        console.error('测试预览请求失败:', error)
      }
    }

    if (!streamReady) {
      try {
        const startResponse = await fetch(`/api/gb28181/devices/${deviceId}/channels/${channelId}/preview/start`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' }
        })
        const startResult = await startResponse.json()
        if (startResult.success && startResult.data) {
          slot.flvUrl = startResult.data.flv_url ? startResult.data.flv_url.replace('127.0.0.1', host).replace('localhost', host) : undefined
          slot.hlsUrl = startResult.data.hls_url ? startResult.data.hls_url.replace('127.0.0.1', host).replace('localhost', host) : undefined
          slot.streamUrl = slot.hlsUrl || slot.flvUrl || rtpStreamUrl
          streamReady = true
          await new Promise(resolve => setTimeout(resolve, 1500))
        }
      } catch (error) {
        console.error('启动预览失败:', error)
      }
    }

    if (!streamReady) {
      slot.flvUrl = liveStreamUrl
      slot.streamUrl = liveStreamUrl
    }
  } else if (data.type === 'onvif' && data.data) {
    const device = data.data
    const deviceId = device.id || device.deviceId  // 使用实际的设备 ID，不是树节点 ID
    
    slot.streamKey = deviceId
    slot.streamName = device.name || device.ip
    slot.streamType = 'onvif'
    // 保存 PTZ 信息
    slot.ptzSupported = device.ptzSupported === true
    slot.ptzDeviceId = deviceId
    slot.ptzChannelId = ''
    
    // ONVIF 需要先调用后端 API 来启动预览并获取真实的流地址
    try {
      const response = await fetch(`/api/onvif/devices/${encodeURIComponent(deviceId)}/preview/start`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          username: device.username || 'admin',
          password: device.password || ''
        })
      })
      const result = await response.json()
      
      if (result.success && result.data) {
        const host = window.location.hostname
        // 使用 API 返回的 FLV 或 HLS 地址
        slot.flvUrl = result.data.flv_url ? result.data.flv_url.replace('127.0.0.1', host).replace('localhost', host) : undefined
        slot.hlsUrl = result.data.hls_url ? result.data.hls_url.replace('127.0.0.1', host).replace('localhost', host) : undefined
        slot.streamUrl = slot.hlsUrl || slot.flvUrl
        console.log('ONVIF API 返回的 URLs:', { flv: slot.flvUrl, hls: slot.hlsUrl }, '选用:', slot.streamUrl)
        await new Promise(resolve => setTimeout(resolve, 1000))
      } else {
        throw new Error(result.error || '启动 ONVIF 预览失败')
      }
    } catch (error) {
      console.error('启动 ONVIF 预览失败:', error)
      slot.error = `ONVIF 预览启动失败: ${error}`
      slot.loading = false
      ElMessage.error(`ONVIF 设备添加失败: ${error}`)
      return
    }
  }
  
  slot.loading = false
  
  // 选中并播放
  selectedSlot.value = targetIndex
  
  nextTick(() => {
    playStream(targetIndex)
  })
  
  ElMessage.success(`已添加到窗口 ${targetIndex + 1}`)
  
  // 自动聚焦到下一个空窗口
  nextTick(() => {
    const nextEmptyIndex = slots.value.findIndex(s => !s.streamKey)
    if (nextEmptyIndex !== -1) {
      selectedSlot.value = nextEmptyIndex
    }
  })
}

// 初始化槽位
const initSlots = (count: number, keepStreams = false) => {
  // 保存当前流数据
  const existingStreams = keepStreams ? slots.value.filter(s => s.streamKey) : []
  
  // 停止所有现有播放器（只停止本地播放，保留后端流以便快速恢复）
  Object.keys(playerRefs.value).forEach(key => {
    try { playerRefs.value[Number(key)]?.stopPlaybackOnly() } catch (e) {}
  })
  
  // 创建新槽位
  const newSlots = Array(count).fill(null).map(() => ({
    streamKey: '',
    streamName: '',
    streamUrl: '',
    streamType: 'gb28181' as const,
    loading: false,
    error: '',
    player: null,
    ptzSupported: false,
    ptzDeviceId: '',
    ptzChannelId: ''
  }))
  
  // 如果保留流，则将现有流复制到新槽位（尽可能多）
  if (keepStreams && existingStreams.length > 0) {
    const copyCount = Math.min(existingStreams.length, count)
    for (let i = 0; i < copyCount; i++) {
      newSlots[i] = {
        ...existingStreams[i],
        loading: false,
        error: '',
        player: null
      }
    }
  }
  
  slots.value = newSlots
  selectedSlot.value = null
  
  // 延迟重新播放保留的流
  if (keepStreams && existingStreams.length > 0) {
    nextTick(() => {
      const copyCount = Math.min(existingStreams.length, count)
      for (let i = 0; i < copyCount; i++) {
        if (slots.value[i].streamKey) {
          playStream(i)
        }
      }
    })
  }
}

// 改变布局
const changeLayout = (layout: number | 'custom') => {
  if (layout === 'custom') return
  
  const layoutConfig: Record<number, [number, number]> = {
    4: [2, 2],
    6: [3, 2],
    9: [3, 3],
    16: [4, 4],
    25: [5, 5],
    32: [8, 4]
  }
  
  const [c, r] = layoutConfig[layout] || [2, 2]
  cols.value = c
  rows.value = r
  initSlots(c * r, true) // 保留现有流
}

// 应用自定义布局
const applyCustomLayout = () => {
  cols.value = customCols.value
  rows.value = customRows.value
  initSlots(customCols.value * customRows.value, true) // 保留现有流
}

// playerRefs are set via template ref bindings

// 选择槽位
const selectSlot = (index: number) => {
  selectedSlot.value = index
}

// 播放流
const playStream = async (index: number) => {
  const slot = slots.value[index]
  if (!slot.streamUrl && !slot.flvUrl && !slot.hlsUrl) return

  slot.loading = true
  slot.error = ''

  // stop existing player
  try { const p = resolvePlayer(playerRefs.value[index]); if (p) await p.stopPreview(); } catch (e) {}

  try {
    // 使用蛇形命名以匹配 API 返回格式，PreviewPlayer 现在支持两种命名
    const info: any = { flv_url: slot.flvUrl || slot.streamUrl, hls_url: slot.hlsUrl || slot.streamUrl }
    await nextTick()
    const player = resolvePlayer(playerRefs.value[index])
    if (!player) {
      slot.error = '播放器未就绪'
      slot.loading = false
      return
    }
    await player.startWithStreamInfo(info)
    slot.loading = false
  } catch (e: any) {
    console.error('播放失败:', e)
    slot.error = e.message || '播放失败'
    slot.loading = false
  }
}

// 停止单个流
const stopStream = (index: number) => {
  try { const p = resolvePlayer(playerRefs.value[index]); if (p) p.stopPlaybackOnly() } catch (e) {}
  if (slots.value[index]) slots.value[index].player = null
}

// 移除流
const removeStream = (index: number) => {
  // 停止本地播放并请求后端停止预览（清理ZLM端口）
  try { const p = resolvePlayer(playerRefs.value[index]); if (p) p.stopPlaybackOnly() } catch (e) {}
  try { const p2 = resolvePlayer(playerRefs.value[index]); if (p2) p2.stopPreview() } catch (e) {}
  slots.value[index] = {
    streamKey: '',
    streamName: '',
    streamUrl: '',
    streamType: 'gb28181',
    loading: false,
    error: '',
    player: null
  }
}

// 刷新所有流
const refreshAllStreams = () => {
  slots.value.forEach((slot, index) => {
    if (slot.streamKey) {
      playStream(index)
    }
  })
}

// 清空所有流
const clearAllStreams = () => {
  Object.keys(playerRefs.value).forEach(key => {
    // stop both local playback and backend preview
    try { playerRefs.value[parseInt(key)]?.stopPlaybackOnly() } catch (e) {}
    try { playerRefs.value[parseInt(key)]?.stopPreview() } catch (e) {}
  })
  
  slots.value = slots.value.map(() => ({
    streamKey: '',
    streamName: '',
    streamUrl: '',
    streamType: 'gb28181' as const,
    loading: false,
    error: '',
    player: null
  }))
}

// 全屏切换
const toggleFullscreen = () => {
  if (!previewContainer.value) return
  
  const elem = previewContainer.value as any
  
  if (!isFullscreen.value) {
    // 进入全屏 - 兼容多种浏览器
    if (elem.requestFullscreen) {
      elem.requestFullscreen()
    } else if (elem.webkitRequestFullscreen) {
      elem.webkitRequestFullscreen()
    } else if (elem.mozRequestFullScreen) {
      elem.mozRequestFullScreen()
    } else if (elem.msRequestFullscreen) {
      elem.msRequestFullscreen()
    }
  } else {
    // 退出全屏 - 兼容多种浏览器
    const doc = document as any
    if (doc.exitFullscreen) {
      doc.exitFullscreen()
    } else if (doc.webkitExitFullscreen) {
      doc.webkitExitFullscreen()
    } else if (doc.mozCancelFullScreen) {
      doc.mozCancelFullScreen()
    } else if (doc.msExitFullscreen) {
      doc.msExitFullscreen()
    }
  }
}

// 监听全屏变化
const handleFullscreenChange = () => {
  const doc = document as any
  isFullscreen.value = !!(
    doc.fullscreenElement || 
    doc.webkitFullscreenElement || 
    doc.mozFullScreenElement || 
    doc.msFullscreenElement
  )
}

// 保存预设
const savePreset = () => {
  if (!newPresetName.value) return
  
  const preset: Preset = {
    id: Date.now().toString(),
    name: newPresetName.value,
    cols: cols.value,
    rows: rows.value,
    slots: slots.value.map(s => ({
      streamKey: s.streamKey,
      streamName: s.streamName,
      streamUrl: s.streamUrl,
      streamType: s.streamType
    })),
    createdAt: Date.now()
  }
  
  savedPresets.value.push(preset)
  localStorage.setItem('multiPreviewPresets', JSON.stringify(savedPresets.value))
  
  ElMessage.success('方案保存成功')
  savePresetVisible.value = false
  newPresetName.value = ''
}

// 加载预设
const loadPreset = (id: string) => {
  const preset = savedPresets.value.find(p => p.id === id)
  if (!preset) return
  
  clearAllStreams()
  
  cols.value = preset.cols
  rows.value = preset.rows
  currentLayout.value = 'custom'
  customCols.value = preset.cols
  customRows.value = preset.rows
  
  slots.value = preset.slots.map(s => ({
    ...s,
    streamType: s.streamType as 'gb28181' | 'onvif' | 'custom',
    loading: false,
    error: '',
    player: null
  }))
  
  // 延迟播放
  nextTick(() => {
    slots.value.forEach((slot, index) => {
      if (slot.streamKey) {
        playStream(index)
      }
    })
  })
  
  managePresetsVisible.value = false
  ElMessage.success(`已加载方案: ${preset.name}`)
}

// 删除预设
const deletePreset = async (id: string) => {
  try {
    await ElMessageBox.confirm('确定删除该方案吗？', '确认删除', {
      type: 'warning'
    })
  } catch {
    return
  }
  
  savedPresets.value = savedPresets.value.filter(p => p.id !== id)
  localStorage.setItem('multiPreviewPresets', JSON.stringify(savedPresets.value))
  ElMessage.success('方案已删除')
}

// 处理预设菜单命令
const handlePresetCommand = (command: string) => {
  if (command === 'save') {
    savePresetVisible.value = true
  } else if (command === 'manage') {
    managePresetsVisible.value = true
  } else if (command.startsWith('load:')) {
    const id = command.replace('load:', '')
    loadPreset(id)
  }
}

// 格式化日期
const formatDate = (timestamp: number) => {
  return new Date(timestamp).toLocaleString()
}

// 加载保存的预设
const loadSavedPresets = () => {
  const saved = localStorage.getItem('multiPreviewPresets')
  if (saved) {
    try {
      savedPresets.value = JSON.parse(saved)
    } catch (e) {}
  }
}

onMounted(() => {
  initSlots(4)
  loadSavedPresets()
  fetchChannels()
  document.addEventListener('fullscreenchange', handleFullscreenChange)
  document.addEventListener('webkitfullscreenchange', handleFullscreenChange)
  document.addEventListener('mozfullscreenchange', handleFullscreenChange)
  document.addEventListener('MSFullscreenChange', handleFullscreenChange)
})

onUnmounted(() => {
  // ensure backend previews are cleaned up
  Object.keys(playerRefs.value).forEach(key => {
    try { playerRefs.value[parseInt(key)]?.stopPreview() } catch (e) {}
  })
  clearAllStreams()
  document.removeEventListener('fullscreenchange', handleFullscreenChange)
  document.removeEventListener('webkitfullscreenchange', handleFullscreenChange)
  document.removeEventListener('mozfullscreenchange', handleFullscreenChange)
  document.removeEventListener('MSFullscreenChange', handleFullscreenChange)
})
</script>

<style scoped>
.multi-preview {
  display: flex;
  height: calc(100vh - 100px);
  gap: 16px;
}

/* 左侧面板 */
.left-panel {
  width: 280px;
  flex-shrink: 0;
}

.tree-card {
  height: 100%;
  display: flex;
  flex-direction: column;
}

.tree-card :deep(.el-card__body) {
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.panel-title {
  font-weight: 600;
  font-size: 14px;
}

.channel-tree {
  flex: 1;
  overflow-y: auto;
}

.tree-node {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 0;
  width: 100%;
}

.node-icon {
  font-size: 14px;
}

.node-label {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
}

/* 右侧面板 */
.right-panel {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.toolbar-card {
  margin-bottom: 16px;
  flex-shrink: 0;
}

.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.toolbar-left .title {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
}

.toolbar-right {
  display: flex;
  gap: 10px;
}

.preview-grid-container {
  flex: 1;
  background: #1a1a1a;
  border-radius: 8px;
  padding: 8px;
  overflow: hidden;
}

.preview-grid-container.fullscreen {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 9999;
  border-radius: 0;
  padding: 4px;
}

.preview-grid {
  display: grid;
  gap: 4px;
  height: 100%;
}

.preview-slot {
  background: #2a2a2a;
  border-radius: 4px;
  overflow: hidden;
  position: relative;
  cursor: pointer;
  transition: all 0.2s;
  border: 2px solid transparent;
}

.preview-slot:hover {
  border-color: #409eff;
}

.preview-slot.active {
  border-color: #67c23a;
}

.preview-slot.has-stream {
  cursor: default;
}

.video-wrapper {
  width: 100%;
  height: 100%;
  position: relative;
}

 

/* 确保播放器填充整个容器 */
.video-wrapper :deep(.preview-player-root),
.video-wrapper :deep(.video-player-wrapper) {
  width: 100% !important;
  height: 100% !important;
}

.video-player {
  width: 100%;
  height: 100%;
  object-fit: contain;
  background: #000;
}

.video-overlay {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  padding: 8px;
  background: linear-gradient(to bottom, rgba(0,0,0,0.7), transparent);
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  opacity: 0;
  transition: opacity 0.2s;
  z-index: 2;
  pointer-events: none;
}

.video-wrapper:hover .video-overlay {
  opacity: 1;
}

.stream-info {
  color: #fff;
  pointer-events: none;
}

.stream-name {
  font-size: 12px;
  font-weight: 500;
}

.video-controls {
  display: flex;
  gap: 4px;
  pointer-events: auto;
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
  gap: 8px;
  font-size: 12px;
}

.video-error {
  color: #f56c6c;
}

.empty-slot {
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  color: #666;
  gap: 8px;
}

.empty-slot .el-icon {
  font-size: 32px;
  color: #409eff;
}

.empty-slot span {
  font-size: 12px;
}

.slot-number {
  position: absolute;
  bottom: 8px;
  right: 8px;
  background: rgba(0,0,0,0.5);
  color: #fff;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
}
</style>
