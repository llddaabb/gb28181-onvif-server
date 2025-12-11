<template>
  <div class="preview-player-root" ref="rootRef">
    <div class="video-player-wrapper" v-loading="loading">
      
      <div :id="containerId" ref="playerContainer" class="video-player-container"></div>
      
      <div v-if="error" class="video-error">
        <el-icon size="48"><VideoCamera /></el-icon>
        <p>{{ error }}</p>
        <el-button type="primary" @click="retry">重试</el-button>
      </div>

      <div v-if="showPtz" class="ptz-controls">
        <div class="ptz-row">
          <el-button size="small" @click="ptz('up')">上</el-button>
        </div>
        <div class="ptz-row">
          <el-button size="small" @click="ptz('left')">左</el-button>
          <el-button size="small" type="primary" @click="ptz('stop')">停止</el-button>
          <el-button size="small" @click="ptz('right')">右</el-button>
        </div>
        <div class="ptz-row">
          <el-button size="small" @click="ptz('down')">下</el-button>
        </div>
        <div class="ptz-row ptz-zoom">
          <el-button size="small" @click="ptz('zoomin')">放大</el-button>
          <el-button size="small" @click="ptz('zoomout')">缩小</el-button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onUnmounted, nextTick } from 'vue'
import { ElMessage } from 'element-plus'
import { VideoCamera } from '@element-plus/icons-vue'

/**
 * 动态获取 Jessibuca 构造函数：
 * 1) 优先使用全局变量（例如通过在 public 引入 jessibuca 脚本后会暴露到 window）
 * 2) 若全局不存在，则尝试通过在 /jessibuca/jessibuca.js 的静态路径加载脚本（适用于将库放到 public 目录的情况）
 */
async function getJessibuca() {
  const w = (window as any)
  // 常见的全局命名
  if (w && (w.Jessibuca || w.jessibuca || w.JB)) {
    return w.Jessibuca || w.jessibuca || w.JB
  }

  // 尝试动态加载 public 中的脚本文件（避免在编译时静态 import 导致找不到类型）
  const scriptUrl = '/jessibuca/jessibuca.js'
  await new Promise<void>((resolve, reject) => {
    const existing = document.querySelector(`script[src="${scriptUrl}"]`)
    if (existing) {
      // 如果已有 script 元素，等待其加载或立即继续
      if ((existing as HTMLScriptElement).getAttribute('data-loaded') === '1') {
        resolve()
      } else {
        existing.addEventListener('load', () => resolve())
        existing.addEventListener('error', () => reject(new Error('Failed to load jessibuca script')))
      }
      return
    }
    const s = document.createElement('script')
    s.src = scriptUrl
    s.async = true
    s.onload = () => {
      s.setAttribute('data-loaded', '1')
      resolve()
    }
    s.onerror = () => reject(new Error('Failed to load jessibuca script'))
    document.head.appendChild(s)
  })

  if (w && (w.Jessibuca || w.jessibuca || w.JB)) {
    return w.Jessibuca || w.jessibuca || w.JB
  }
  throw new Error('Jessibuca not found on window after loading script')
}


// 假设 Device 和 Channel 接口已在别处定义或就是这样
interface Device { deviceId: string }
interface Channel { channelId: string }

const props = defineProps({
  show: { type: Boolean, required: false, default: false },
  device: { type: Object as () => Device | null, required: false },
  channels: { type: Array as () => Channel[], required: false, default: () => [] },
  selectedChannelId: { type: [String, Number], required: false },
  showPtz: { type: Boolean, default: true },
  ptzDeviceId: { type: [String, Number], required: false },
  ptzChannelId: { type: [String, Number], required: false },
  // 新增：默认高度（可以是 number 表示 px，或字符串如 '50vh'）
  defaultHeight: { type: [Number, String], required: false, default: 600 },
})

const emit = defineEmits(['update:show', 'update:selectedChannelId', 'playing', 'error', 'loading'])

const playerContainer = ref<HTMLElement | null>(null)
const rootRef = ref<HTMLElement | null>(null)
// 每个实例使用唯一 container id，避免多个实例共用同一 id 导致冲突
const containerId = `play-container-${Math.random().toString(36).slice(2,9)}`
const loading = ref(false)
const error = ref('')

const streamInfoRaw = ref<any>(null)
let h265PlayerInstance: any = null // 保持名称 h265PlayerInstance，但实际是 Jessibuca 实例

// --- 函数定义区 ---

/**
 * 清理并销毁播放器实例
 */
const cleanup = async () => {
  console.debug('[PreviewPlayer] cleanup called')
  try {
    if (h265PlayerInstance) {
      // 优先使用 destroy，与 jessibuca 推荐的销毁方式保持一致
      try {
        if (typeof h265PlayerInstance.destroy === 'function') {
          const maybe = h265PlayerInstance.destroy()
          if (maybe && typeof maybe.then === 'function') {
            await maybe.catch(() => {})
          }
        } else if (typeof h265PlayerInstance.release === 'function') {
          const maybe = h265PlayerInstance.release()
          if (maybe && typeof maybe.then === 'function') {
            await maybe.catch(() => {})
          }
        }
      } catch (inner) {
        console.warn('[PreviewPlayer] error while destroying player instance', inner)
      }
      h265PlayerInstance = null
    }

    // 清空播放容器内的 DOM，确保下次可以重新创建播放器实例
    const containerEl = playerContainer.value
    if (containerEl) {
      try {
        containerEl.innerHTML = ''
        // 恢复原始 class，避免 jessibuca 添加的类影响下一次创建
        containerEl.className = 'video-player-container'
      } catch (e) {
        console.warn('[PreviewPlayer] failed to clear container', e)
      }
    }

    // 重置父容器高度为默认高度（使用属性或 CSS 变量）
    const wrapperEl = playerContainer.value?.parentElement
    if (wrapperEl) {
      const h = typeof props.defaultHeight === 'number' ? `${props.defaultHeight}px` : props.defaultHeight
      wrapperEl.style.height = h as string
    }
  } catch (e) {
    console.warn('[PreviewPlayer] cleanup error', e)
  }
}

onUnmounted(() => {
  cleanup()
  if (resizeObserver) resizeObserver.disconnect()
})

// 初始化：设置初始 wrapper 高度并启动 ResizeObserver
nextTick(() => {
  const wrapperEl = playerContainer.value?.parentElement
  if (wrapperEl) {
    const h = typeof props.defaultHeight === 'number' ? `${props.defaultHeight}px` : props.defaultHeight
    wrapperEl.style.height = h as string
  }
  ensureResizeObserver()
})

// ResizeObserver: 监听外部容器大小变化，调整 wrapper 高度
let resizeObserver: ResizeObserver | null = null
function ensureResizeObserver() {
  if (typeof window === 'undefined') return
  if (resizeObserver) return
  resizeObserver = new ResizeObserver((entries) => {
    for (const entry of entries) {
      const cr = entry.contentRect
      const wrapperEl = playerContainer.value?.parentElement
      if (!wrapperEl) continue
      // 默认为外部容器高度的一部分或全部
      const height = cr.height
      if (height && height > 0) {
        wrapperEl.style.height = `${height}px`
      }
    }
  })
  if (rootRef.value) resizeObserver.observe(rootRef.value)
}

/**
 * 将相对 URL 转换为绝对 URL
 */
function normalizeStreamUrl(url: string): string {
  if (!url) return ''
  // 如果 URL 已经是完整 URL (http, https, rtmp, rtsp)，直接返回
  if (url.startsWith('http://') || url.startsWith('https://') || url.startsWith('rtmp://') || url.startsWith('rtsp://')) {
    return url
  }
  // 如果是相对路径 (以 / 开头)，转换为完整 URL
  if (url.startsWith('/')) {
    // 使用当前页面的协议和域名/端口来拼接
    return `${window.location.protocol}//${window.location.host}${url}`
  }
  // 其他情况返回原值，但可能会失败
  return url
}

/**
 * 尝试获取视频分辨率并调整容器尺寸
 */
const adjustPlayerSize = () => {
    if (!h265PlayerInstance) return

    let videoWidth = 0
    let videoHeight = 0

    // 尝试获取视频的原始分辨率
    if (h265PlayerInstance.getVideoWidth && h265PlayerInstance.getVideoHeight) {
        videoWidth = h265PlayerInstance.getVideoWidth()
        videoHeight = h265PlayerInstance.getVideoHeight()
    } else if (h265PlayerInstance.videoWidth && h265PlayerInstance.videoHeight) {
        // 有些版本可能直接是属性
        videoWidth = h265PlayerInstance.videoWidth
        videoHeight = h265PlayerInstance.videoHeight
    } 

    if (videoWidth > 0 && videoHeight > 0) {
        console.debug(`[PreviewPlayer] Detected video resolution: ${videoWidth}x${videoHeight}`)
        
        // 目标：调整 video-player-wrapper 的高度
        const wrapperEl = playerContainer.value?.parentElement
        if (!wrapperEl) return

        const aspectRatio = videoWidth / videoHeight
        
        // 固定宽度，根据比例计算高度
        const containerWidth = wrapperEl.clientWidth
        const calculatedHeight = containerWidth / aspectRatio

        // 应用高度样式
        wrapperEl.style.height = `${calculatedHeight}px`
        wrapperEl.style.maxHeight = '100vh'; // 可选：限制最大高度

        console.log(`[PreviewPlayer] Adjusted wrapper height to ${calculatedHeight}px`)
    } else {
         console.warn('[PreviewPlayer] Failed to get valid video dimensions, skipping size adjustment.')
    }
}
function extractStreamUrl(data: any, schema: string) {
  if (!data) return '';

  // WebSocket（ws-flv 最优）
  if (schema === 'ws') {
    return (
      data.WsFlvURL ||
      data.WSFlvURL ||
      data.wsFlvUrl ||
      ''
    );
  }

  // HLS 优先
  if (schema === 'hls') {
    return data.HlsURL || data.hlsUrl || '';
  }

  // 默认：FLV / WS-FLV 自动选择
  return (
    data.WsFlvURL ||
    data.FlvURL ||
    data.HlsURL ||
    ''
  );
}

/**
 * 初始化和启动播放器
 */
const initPlayer = async () => {
  await cleanup()
  loading.value = true
  emit('loading', true) // 显式通知外部组件开始加载
  error.value = ''

  const base = window.location.origin;
const rawUrl = extractStreamUrl(streamInfoRaw.value, 'default');
const flvUrl = rawUrl.startsWith('/') ? base + rawUrl : rawUrl;
  
  
  const finalUrl = normalizeStreamUrl(flvUrl)
  console.log('[PreviewPlayer] Original stream URLs:', {flvUrl: normalizeStreamUrl(flvUrl) })
  console.debug('[PreviewPlayer] Normalized play URL:', finalUrl)
  if (!finalUrl) {
    error.value = '未获取到流地址'
    loading.value = false
    emit('loading', false)
    emit('error', error.value)
    return
  }

  // 2. 动态获取 Jessibuca 构造函数（优先尝试 npm 动态 import，失败则回退到 public 脚本）
  await nextTick()
  let JB: any = null
  try {
    JB = await getJessibuca()
  } catch (e: any) {
    console.error('[PreviewPlayer] getJessibuca failed', e)
    error.value = 'Jessibuca 播放器库未找到，无法播放视频'
    loading.value = false
    emit('loading', false)
    emit('error', error.value)
    return
  }

  // 3. 获取 DOM 元素（直接使用 ref，避免依赖固定 id）
  await nextTick()
  const containerEl = playerContainer.value
  if (!containerEl) {
    error.value = '播放容器元素未找到'
    loading.value = false
    emit('loading', false)
    return
  }
  // 给容器设置唯一 id（部分播放器实现可能会读取 id）
  try { containerEl.id = containerId } catch (_) {}

  try {
    // jessibuca 配置，参考案例和最佳实践
    const cfg = {
      container: containerEl, // jessibuca 的推荐配置方式
      id: containerId, // 也可以保留 ID
      url: finalUrl,
      isLive: true,
      autoplay: true,
      // autodestroy: false, // 默认或根据需要设置
      videoBuffer: 0.2, // 缓存时长，从案例中借鉴
      isResize: false,
      useMSE: flvUrl ? true : false, // FLV 流通常使用 MSE
      debug: false,
      // 隐藏自带的操作按钮
      operateBtns: {
          fullscreen: false,
          screenshot: false,
          play: false,
          audio: false,
          recorder: false
      },
      wasmPath:'/jessibuca/decoder.wasm',
      decoder:'/jessibuca/decoder.js',
      // ... 其他配置，例如：showBandwidth: false
    }

    // 实例化播放器
    h265PlayerInstance = new (JB as any)(cfg)
    console.debug('[PreviewPlayer] Jessibuca instance created', h265PlayerInstance)

    // 绑定事件：jessibuca 提供 on 方法
    if (h265PlayerInstance && typeof h265PlayerInstance.on === 'function') {
      h265PlayerInstance.on('loadfinish', () => { 
          // loadfinish 替代 ready/play 作为加载完成的标志
          loading.value = false
          emit('loading', false)
          emit('playing')
          nextTick(() => adjustPlayerSize())
      })
      h265PlayerInstance.on('error', (e: any) => { 
          console.error('[PreviewPlayer] Jessibuca error', e)
          error.value = '播放器错误'
          loading.value = false
          emit('loading', false)
          emit('error', error.value) 
      })
      h265PlayerInstance.on('start', () => { 
          // 确保 play 后加载状态解除
          if (loading.value) { 
            loading.value = false; 
            emit('loading', false);
          }
      })
      // 可以在这里添加 videoInfo/audioInfo 等事件监听，参考案例
    }

    // jessibuca 会自动 play（autoplay:true），但确保 url 是通过配置传入的
    // 如果播放器没有自动播放，可能需要手动调用 play() 或 start()
    if (h265PlayerInstance && typeof h265PlayerInstance.play === 'function') {
      h265PlayerInstance.play(finalUrl) // 显式调用 play 并传入 url
    }

    // 兜底：20s 后如果仍在 loading，关闭 loading
    setTimeout(() => { if (loading.value && !error.value) { loading.value = false; emit('loading', false) } }, 2000)

  } catch (e: any) {
    console.error('[PreviewPlayer] Init failed (Jessibuca)', e)
    error.value = e.message || '播放器初始化异常'
    loading.value = false
    emit('loading', false)
    emit('error', error.value)
  }
}

// ---------------- 业务逻辑保持不变 ----------------

// 后端启动预览
const startPreview = async (channelId?: string) => {
  if (!props.device) return
  const ch = channelId || props.selectedChannelId || (props.device as any)?.deviceId
  if (!ch) return
  
  loading.value = true
  emit('loading', true)
  error.value = ''
  
  try {
    const deviceId = (props.device as any).deviceId
    // 假设这是一个标准的 GB28181 预览启动接口
    const resp = await fetch(`/api/gb28181/devices/${deviceId}/channels/${ch}/preview/start`, { 
      method: 'POST', 
      headers: { 'Content-Type': 'application/json' } 
    })
    const data = await resp.json()
    if (!data || !data.success) throw new Error(data?.error || '启动预览失败')
    
    streamInfoRaw.value = data.data
    await initPlayer()
    
  } catch (e: any) {
    console.error('startPreview error', e)
    error.value = e.message || '启动预览失败'
    loading.value = false
    emit('loading', false)
    emit('error', error.value)
  }
}

// 直接使用已有流信息播放
async function startWithStreamInfo(info: { hls_url?: string; flv_url?: string } | null) {
  if (!info) return
  // 保留 device_id / channel_id 字段（如果上游提供）以便停止时调用设备相关接口
  streamInfoRaw.value = Object.assign({}, info)
  await initPlayer()
}

// 停止播放（含后端调用）
const stopPreview = async () => {
  // 先销毁播放器，释放资源
  await cleanup()
  // 然后通知后端停止流，优先调用设备/通道相关的停止接口
  if (streamInfoRaw.value) {
    try {
      const deviceId = streamInfoRaw.value.device_id ?? (props.device as any)?.deviceId
      const channelId = streamInfoRaw.value.channel_id ?? streamInfoRaw.value.channelId ?? props.selectedChannelId
      if (deviceId) {
        // 如果有 channelId，调用通道停止接口，否则调用设备级停止
        if (channelId) {
          await fetch(`/api/gb28181/devices/${deviceId}/channels/${channelId}/preview/stop`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ channelId }) }).then(r => r.json().catch(() => ({}))).catch(() => {})
        } else {
          await fetch(`/api/gb28181/devices/${deviceId}/preview/stop`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ channelId: deviceId }) }).then(r => r.json().catch(() => ({}))).catch(() => {})
        }
      } else {
        // 回退到通用 stop
        await fetch(`/api/gb28181/stop`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(streamInfoRaw.value) }).then(r => r.json().catch(() => ({}))).catch(() => {})
      }
    } catch (e) {
      console.warn('stopPreview backend failed', e)
    }
  }
  streamInfoRaw.value = null
}

const stopPlaybackOnly = () => cleanup()

const retry = () => {
  // 重新执行启动预览逻辑，如果 streamInfoRaw 还在，则直接 initPlayer
  if (streamInfoRaw.value) {
     initPlayer()
  } else {
     // 否则重新调用 startPreview
     startPreview()
  }
}

// PTZ 控制逻辑
async function ptz(action: string, opts?: { deviceId?: string | number; channelId?: string | number, speed?: number }) {
  const deviceId = opts?.deviceId ?? props.ptzDeviceId ?? (props.device as any)?.deviceId
  const channelId = opts?.channelId ?? props.ptzChannelId ?? props.selectedChannelId
  const speed = opts?.speed ?? 128
  if (!deviceId || !channelId) {
    ElMessage.error('缺少 PTZ 目标设备或通道信息')
    return
  }

  try {
    // 后端期望的接口：POST /api/gb28181/devices/{id}/ptz
    // 请求体字段：{ command: string, channelId: string, speed: int }
    const payload = {
      command: String(action),
      channelId: String(channelId),
      deviceId: String(deviceId),
      speed: Number(speed)
    }
    const url = `/api/gb28181/devices/${deviceId}/ptz`
    const res = await fetch(url, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) })
    const data = await res.json().catch(() => ({}))
    if (data && (data.success || data.code === 0)) {
      ElMessage.success('PTZ 指令已发送')
    } else {
      ElMessage.error(data?.message || data?.msg || 'PTZ 发送失败')
    }
  } catch (e: any) {
    console.error('ptz request failed', e)
    ElMessage.error('PTZ 操作失败')
  }
}

defineExpose({ startPreview, startWithStreamInfo, stopPlaybackOnly, stopPreview, ptz })
</script>

<style scoped>
.preview-player-root { width: 100%; height: 100%; }
.video-player-wrapper { 
  position: relative; 
  width: 100%; 
  /* 使用 CSS 变量设置默认高度（可通过 prop 反映到内联样式），并保证最小高度 */
  min-height: var(--preview-default-height, 400px);
  height: auto; 
  height: 100%; 
  background: #000; 
  /* 确保过渡平滑 */
  transition: height 0.3s ease; 
}

/* 容器样式：必须设置为块级且有宽高 */
/* jessibuca 会在这个容器内创建 canvas/video 元素 */
.video-player-container { 
  width: 100%; 
  height: 100%; 
  background: #000; 
  display: block;
}

.video-error { 
  position: absolute; 
  left: 50%; 
  top: 50%; 
  transform: translate(-50%, -50%); 
  text-align: center; 
  color: #fff; 
  z-index: 10;
}
.ptz-controls { 
  position: absolute; 
  right: 12px; 
  bottom: 12px; 
  background: rgba(0,0,0,0.55); 
  padding: 8px; 
  border-radius: 8px; 
  display:flex; 
  flex-direction:column; 
  gap:8px;
  z-index: 120; 
  box-shadow: 0 6px 18px rgba(0,0,0,0.45);
}
.ptz-controls .el-button { background: rgba(255,255,255,0.04); color: #fff }
.ptz-row { display:flex; justify-content:center; gap:6px }
.ptz-zoom { margin-top:4px }
</style>