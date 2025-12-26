<template>
  <div class="preview-player-root" ref="rootRef" :class="{ 'fullscreen': isFullscreen }">
    <div class="video-player-wrapper" v-loading="loading" :class="{ 'fullscreen': isFullscreen }" @dblclick="toggleFullscreen">
      
      <div :id="containerId" ref="playerContainer" class="video-player-container"></div>
      
      <div v-if="error" class="video-error">
        <el-icon size="48"><VideoCamera /></el-icon>
        <p>{{ error }}</p>
        <el-button type="primary" @click="retry">重试</el-button>
      </div>

      <!-- PTZ控制面板 - 可收起的浮动面板 -->
      <div v-if="shouldShowPtz" class="ptz-wrapper">
        <!-- 收起状态：显示浮动按钮 -->
        <div v-if="!ptzExpanded" class="ptz-toggle-btn" @click="ptzExpanded = true">
          🎮
        </div>
        <!-- 展开状态：显示完整控制面板 -->
        <div v-else class="ptz-controls">
          <!-- 收起按钮 -->
          <div class="ptz-header">
            <span class="ptz-title">云台控制</span>
            <el-button class="ptz-close-btn" size="small" @click="ptzExpanded = false">✕</el-button>
          </div>
          <!-- 速度调节 -->
          <div class="ptz-speed">
            <span class="speed-label">速度</span>
            <el-slider v-model="ptzSpeed" :min="10" :max="100" :step="10" :show-tooltip="false" size="small" />
            <span class="speed-value">{{ ptzSpeed }}%</span>
          </div>
          <!-- 方向控制 - 十字形布局 -->
          <div class="ptz-direction">
            <div class="ptz-row">
              <div class="ptz-btn-placeholder"></div>
              <el-button 
                class="ptz-btn" 
                size="small" 
                @mousedown="startPtz('up')"
                @mouseup="stopPtz"
                @mouseleave="handlePtzMouseLeave"
                @touchstart.prevent="startPtz('up')"
                @touchend.prevent="stopPtz">
                ▲
              </el-button>
              <div class="ptz-btn-placeholder"></div>
            </div>
            <div class="ptz-row">
              <el-button 
                class="ptz-btn" 
                size="small" 
                @mousedown="startPtz('left')"
                @mouseup="stopPtz"
                @mouseleave="handlePtzMouseLeave"
                @touchstart.prevent="startPtz('left')"
                @touchend.prevent="stopPtz">
                ◀
              </el-button>
              <el-button 
                class="ptz-btn ptz-stop" 
                size="small" 
                type="danger"
                @click="forceStopPtz">
                ■
              </el-button>
              <el-button 
                class="ptz-btn" 
                size="small" 
                @mousedown="startPtz('right')"
                @mouseup="stopPtz"
                @mouseleave="handlePtzMouseLeave"
                @touchstart.prevent="startPtz('right')"
                @touchend.prevent="stopPtz">
                ▶
              </el-button>
            </div>
            <div class="ptz-row">
              <div class="ptz-btn-placeholder"></div>
              <el-button 
                class="ptz-btn" 
                size="small" 
                @mousedown="startPtz('down')"
                @mouseup="stopPtz"
                @mouseleave="handlePtzMouseLeave"
                @touchstart.prevent="startPtz('down')"
                @touchend.prevent="stopPtz">
                ▼
              </el-button>
              <div class="ptz-btn-placeholder"></div>
            </div>
          </div>
          <!-- 缩放控制 -->
          <div class="ptz-zoom-controls">
            <el-button 
              class="ptz-zoom-btn" 
              size="small" 
              @mousedown="startPtz('zoomin')"
              @mouseup="stopPtz"
              @mouseleave="handlePtzMouseLeave"
              @touchstart.prevent="startPtz('zoomin')"
              @touchend.prevent="stopPtz">
              🔍+
            </el-button>
            <el-button 
              class="ptz-zoom-btn" 
              size="small" 
              @mousedown="startPtz('zoomout')"
              @mouseup="stopPtz"
              @mouseleave="handlePtzMouseLeave"
              @touchstart.prevent="startPtz('zoomout')"
              @touchend.prevent="stopPtz">
              🔍-
            </el-button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onUnmounted, onMounted, nextTick, computed } from 'vue'
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
  showPtz: { type: Boolean, default: false },  // 改为默认 false，需要明确传入才显示
  ptzDeviceId: { type: [String, Number], required: false },
  ptzChannelId: { type: [String, Number], required: false },
  // 设备类型: 'onvif' 或 'gb28181'，用于区分 PTZ API
  deviceType: { type: String, default: 'gb28181' },
  // ONVIF 专用: profileToken
  profileToken: { type: String, required: false, default: '' },
  // 新增：默认高度（可以是 number 表示 px，或字符串如 '50vh'）
  defaultHeight: { type: [Number, String], required: false, default: 600 },
  // 新增：设备是否支持PTZ
  ptzSupported: { type: Boolean, default: undefined },
})

const emit = defineEmits(['update:show', 'update:selectedChannelId', 'playing', 'error', 'loading', 'fullscreenChange'])

const playerContainer = ref<HTMLElement | null>(null)
const rootRef = ref<HTMLElement | null>(null)
// 每个实例使用唯一 container id，避免多个实例共用同一 id 导致冲突
const containerId = `play-container-${Math.random().toString(36).slice(2,9)}`
const loading = ref(false)
const error = ref('')
const isFullscreen = ref(false)
// 保存原始的默认高度，用于全屏退出时恢复
const getOriginalHeight = () => {
  const h = typeof props.defaultHeight === 'number' ? `${props.defaultHeight}px` : props.defaultHeight
  return h as string
}

const streamInfoRaw = ref<any>(null)
let h265PlayerInstance: any = null // 保持名称 h265PlayerInstance，但实际是 Jessibuca 实例

// 计算是否应该显示 PTZ 面板
const shouldShowPtz = computed(() => {
  // 如果明确设置了 showPtz 为 false，则不显示
  if (props.showPtz === false) return false
  
  // 如果传入了 ptzSupported，使用该值
  if (props.ptzSupported !== undefined) return props.ptzSupported
  
  // 如果设备对象中有 ptzSupported 字段，使用该值
  if (props.device && 'ptzSupported' in props.device) {
    return (props.device as any).ptzSupported !== false
  }
  
  // 如果 showPtz 明确为 true，则显示
  if (props.showPtz === true) return true
  
  // 默认不显示
  return false
})

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

/**
 * 全屏切换
 */
const toggleFullscreen = async () => {
  if (!rootRef.value) return
  
  try {
    if (!isFullscreen.value) {
      // 进入全屏
      console.debug('[PreviewPlayer] toggleFullscreen: Entering fullscreen')
      const element = rootRef.value as any
      
      // 先设置全屏标志，这样 ResizeObserver 会跳过更新
      isFullscreen.value = true
      console.debug('[PreviewPlayer] toggleFullscreen: Set isFullscreen.value to true')
      
      // 清除 wrapper 的内联 height 样式，让 CSS 的 100% 接管
      const wrapperEl = playerContainer.value?.parentElement
      if (wrapperEl) {
        wrapperEl.style.height = ''
        console.debug('[PreviewPlayer] toggleFullscreen: Cleared wrapper inline height style')
      }
      
      // 尝试使用浏览器全屏 API
      if (element.requestFullscreen) {
        try {
          await element.requestFullscreen()
          console.debug('[PreviewPlayer] toggleFullscreen: requestFullscreen succeeded')
        } catch (e) {
          console.debug('toggleFullscreen: requestFullscreen failed, will use CSS fullscreen', e)
        }
      } else if (element.webkitRequestFullscreen) {
        await element.webkitRequestFullscreen()
        console.debug('[PreviewPlayer] toggleFullscreen: webkitRequestFullscreen succeeded')
      } else if (element.mozRequestFullScreen) {
        await element.mozRequestFullScreen()
        console.debug('[PreviewPlayer] toggleFullscreen: mozRequestFullScreen succeeded')
      } else if (element.msRequestFullscreen) {
        await element.msRequestFullscreen()
        console.debug('[PreviewPlayer] toggleFullscreen: msRequestFullscreen succeeded')
      }
      
      // 隐藏页面滚动条
      document.body.style.overflow = 'hidden'
      document.documentElement.style.overflow = 'hidden'
    } else {
      // 退出全屏
      console.debug('[PreviewPlayer] toggleFullscreen: Exiting fullscreen, isFullscreen.value=', isFullscreen.value)
      const doc = document as any
      // 尝试退出浏览器全屏
      if (doc.fullscreenElement || doc.webkitFullscreenElement || doc.mozFullScreenElement || doc.msFullscreenElement) {
        try {
          if (doc.exitFullscreen) {
            await doc.exitFullscreen()
            console.debug('[PreviewPlayer] toggleFullscreen: exitFullscreen succeeded')
          } else if (doc.webkitExitFullscreen) {
            await doc.webkitExitFullscreen()
            console.debug('[PreviewPlayer] toggleFullscreen: webkitExitFullscreen succeeded')
          } else if (doc.mozCancelFullScreen) {
            await doc.mozCancelFullScreen()
            console.debug('[PreviewPlayer] toggleFullscreen: mozCancelFullScreen succeeded')
          } else if (doc.msExitFullscreen) {
            await doc.msExitFullscreen()
            console.debug('[PreviewPlayer] toggleFullscreen: msExitFullscreen succeeded')
          }
        } catch (e) {
          console.debug('toggleFullscreen: exit fullscreen failed', e)
        }
      }
      
      // 恢复页面滚动
      document.body.style.overflow = ''
      document.documentElement.style.overflow = ''
      
      // 恢复原始高度（必须在设置 isFullscreen 之前）
      const wrapperEl = playerContainer.value?.parentElement
      if (wrapperEl) {
        const originalHeight = getOriginalHeight()
        console.debug('[PreviewPlayer] toggleFullscreen: Restoring height to', originalHeight)
        wrapperEl.style.height = originalHeight
      }
      
      // 最后设置 isFullscreen = false，这样 ResizeObserver 会恢复工作
      isFullscreen.value = false
      console.debug('[PreviewPlayer] toggleFullscreen: Set isFullscreen.value to false')
    }
  } catch (e) {
    console.error('Fullscreen toggle failed:', e)
    // 如果浏览器 API 失败，仍使用 CSS 全屏
    isFullscreen.value = !isFullscreen.value
    if (isFullscreen.value) {
      document.body.style.overflow = 'hidden'
      document.documentElement.style.overflow = 'hidden'
    } else {
      document.body.style.overflow = ''
      document.documentElement.style.overflow = ''
    }
  }
}

/**
 * 监听全屏变化事件
 */
const handleFullscreenChange = () => {
  const doc = document as any
  const fullscreenEl = doc.fullscreenElement || doc.webkitFullscreenElement || doc.mozFullScreenElement || doc.msFullscreenElement
  
  // 只有当全屏元素是自己的 rootRef 时才认为是全屏状态
  const wasFullscreen = isFullscreen.value
  const isMyFullscreen = fullscreenEl === rootRef.value
  isFullscreen.value = isMyFullscreen
  
  // 通知父组件全屏状态变化
  if (wasFullscreen !== isFullscreen.value) {
    emit('fullscreenChange', isFullscreen.value)
  }
  
  // 如果从全屏退出，需要恢复高度
  if (wasFullscreen && !isFullscreen.value) {
    // 延迟处理，确保 DOM 已完全更新
    nextTick(() => {
      setTimeout(() => {
        // 重置包装器样式
        const wrapperEl = playerContainer.value?.parentElement
        if (wrapperEl) {
          const originalHeight = getOriginalHeight()
          console.debug('[PreviewPlayer] handleFullscreenChange: Restoring height to:', originalHeight)
          wrapperEl.style.height = originalHeight
          // 注意：不要调用 adjustPlayerSize()，ResizeObserver 会自动处理
        }
      }, 150)
    })
  }
}

/**
 * 监听 ESC 键退出全屏
 */
const handleKeydown = (e: KeyboardEvent) => {
  if (e.key === 'Escape' && isFullscreen.value) {
    toggleFullscreen()
  }
}

onUnmounted(() => {
  cleanup()
  if (resizeObserver) resizeObserver.disconnect()
  if (videoSizeCheckTimeout) clearTimeout(videoSizeCheckTimeout)
  document.removeEventListener('fullscreenchange', handleFullscreenChange)
  document.removeEventListener('webkitfullscreenchange', handleFullscreenChange)
  document.removeEventListener('mozfullscreenchange', handleFullscreenChange)
  document.removeEventListener('MSFullscreenChange', handleFullscreenChange)
  document.removeEventListener('keydown', handleKeydown)
})

// 初始化：设置初始 wrapper 高度并启动 ResizeObserver
nextTick(() => {
  const wrapperEl = playerContainer.value?.parentElement
  if (wrapperEl) {
    const h = getOriginalHeight()
    wrapperEl.style.height = h
    console.debug('[PreviewPlayer] Init: Set wrapper height to:', h, 'defaultHeight:', props.defaultHeight)
  } else {
    console.warn('[PreviewPlayer] Init: wrapper element not found')
  }
  ensureResizeObserver()
  
  // 添加全屏事件监听
  document.addEventListener('fullscreenchange', handleFullscreenChange)
  document.addEventListener('webkitfullscreenchange', handleFullscreenChange)
  document.addEventListener('mozfullscreenchange', handleFullscreenChange)
  document.addEventListener('MSFullscreenChange', handleFullscreenChange)
  document.addEventListener('keydown', handleKeydown)
})

// ResizeObserver: 监听外部容器大小变化，调整 wrapper 高度
let resizeObserver: ResizeObserver | null = null
function ensureResizeObserver() {
  if (typeof window === 'undefined') return
  if (resizeObserver) return
  resizeObserver = new ResizeObserver((entries) => {
    // 全屏时，让 CSS 处理高度，不进行任何更新
    if (isFullscreen.value) {
      console.debug('[PreviewPlayer] ResizeObserver: Skipping during fullscreen')
      return
    }
    
    for (const entry of entries) {
      const cr = entry.contentRect
      const wrapperEl = playerContainer.value?.parentElement
      if (!wrapperEl) continue
      // 默认为外部容器高度的一部分或全部
      const height = cr.height
      if (height && height > 0) {
        console.debug('[PreviewPlayer] ResizeObserver: Updating wrapper height to', height)
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
let videoSizeCheckTimeout: ReturnType<typeof setTimeout> | null = null
const adjustPlayerSize = () => {
    console.debug('[PreviewPlayer] adjustPlayerSize called')
    if (!h265PlayerInstance) {
      console.debug('[PreviewPlayer] adjustPlayerSize: h265PlayerInstance not available')
      return
    }

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
    } else if (h265PlayerInstance.width && h265PlayerInstance.height) {
        // 尝试其他属性名
        videoWidth = h265PlayerInstance.width
        videoHeight = h265PlayerInstance.height
    }

    if (videoWidth > 0 && videoHeight > 0) {
        console.debug(`[PreviewPlayer] adjustPlayerSize: Detected video resolution: ${videoWidth}x${videoHeight}`)
        // 清除超时定时器
        if (videoSizeCheckTimeout) {
            clearTimeout(videoSizeCheckTimeout)
            videoSizeCheckTimeout = null
        }
        applyVideoSize(videoWidth, videoHeight)
    } else {
        console.debug('[PreviewPlayer] adjustPlayerSize: Video dimensions not yet available, will retry...')
        // 清除旧的超时定时器
        if (videoSizeCheckTimeout) clearTimeout(videoSizeCheckTimeout)
        
        // 延迟重试（最多 3 秒后使用默认比例）
        videoSizeCheckTimeout = setTimeout(() => {
            console.debug('[PreviewPlayer] adjustPlayerSize: Failed to get video dimensions after timeout, using default 16:10 aspect ratio')
            applyVideoSize(16, 10) // 使用 16:10 作为默认比例
            videoSizeCheckTimeout = null
        }, 3000)
    }
}

/**
 * 应用视频尺寸到容器
 */
const applyVideoSize = (videoWidth: number, videoHeight: number) => {
    const wrapperEl = playerContainer.value?.parentElement
    if (!wrapperEl) {
      console.warn('[PreviewPlayer] applyVideoSize: wrapper element not found')
      return
    }

    const aspectRatio = videoWidth / videoHeight
    const containerWidth = wrapperEl.clientWidth
    
    if (containerWidth <= 0) {
        // 容器宽度还未准备好，延迟重试
        console.debug('[PreviewPlayer] applyVideoSize: container width not ready, retrying...')
        setTimeout(() => applyVideoSize(videoWidth, videoHeight), 500)
        return
    }

    // 根据视频宽高比计算高度
    const calculatedHeight = containerWidth / aspectRatio

    // 应用高度样式（最小100px，最大90vh）
    const finalHeight = Math.max(100, Math.min(calculatedHeight, window.innerHeight * 0.9))
    const oldHeight = wrapperEl.style.height
    wrapperEl.style.height = `${finalHeight}px`

    console.debug(`[PreviewPlayer] applyVideoSize: Adjusted height from "${oldHeight}" to "${finalHeight}px" (video: ${videoWidth}x${videoHeight}, aspect: ${aspectRatio.toFixed(2)})`)
}

function extractStreamUrl(data: any, schema: string) {
  if (!data) return '';

  // FLV 优先（默认推荐格式，延迟低）
  if (schema === 'flv' || schema === 'default') {
    // 支持驼峰和蛇形命名
    const flvUrl = data.FlvURL || data.flvUrl || data.flv_url || '';
    if (flvUrl) return flvUrl;
  }

  // WebSocket FLV 次选
  if (schema === 'ws' || schema === 'default') {
    const wsFlvUrl = (
      data.WsFlvURL ||
      data.WSFlvURL ||
      data.wsFlvUrl ||
      data.ws_flv_url ||
      ''
    );
    if (wsFlvUrl) return wsFlvUrl;
  }

  // HLS 作为最后备选（兼容性好但延迟高）
  if (schema === 'hls' || schema === 'default') {
    const hlsUrl = data.HlsURL || data.hlsUrl || data.hls_url || '';
    if (hlsUrl) return hlsUrl;
  }

  return '';
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
      // 根据URL类型决定是否使用MSE
      useMSE: finalUrl.includes('.flv') ? true : false,
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
      // 监听视频信息事件，用于自适应视频尺寸
      try {
        h265PlayerInstance.on('videoInfo', (info: any) => {
          console.debug('[PreviewPlayer] Video info event:', info)
          if (info && info.videoWidth && info.videoHeight) {
            applyVideoSize(info.videoWidth, info.videoHeight)
          }
        })
      } catch (e) {
        console.debug('[PreviewPlayer] videoInfo event not supported')
      }
      // 可以在这里添加 audioInfo 等其他事件监听，参考案例
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
    let url = ''
    
    // 根据设备类型选择不同的API路径
    if (props.deviceType === 'onvif') {
      // ONVIF设备预览启动接口 (不需要 channelId 路径参数)
      url = `/api/onvif/devices/${deviceId}/preview/start`
    } else {
      // GB28181设备预览启动接口 (需要 channelId 路径参数)
      url = `/api/gb28181/devices/${deviceId}/channels/${ch}/preview/start`
    }
    
    const resp = await fetch(url, { 
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
        // 构建基础API路径，根据设备类型选择
        const baseApi = props.deviceType === 'onvif' ? '/api/onvif' : '/api/gb28181'
        
        // 如果有 channelId，调用通道停止接口，否则调用设备级停止
        if (channelId) {
          await fetch(`${baseApi}/devices/${deviceId}/channels/${channelId}/preview/stop`, { 
            method: 'POST', 
            headers: { 'Content-Type': 'application/json' }, 
            body: JSON.stringify({ channelId }) 
          }).then(r => r.json().catch(() => ({}))).catch(() => {})
        } else {
          await fetch(`${baseApi}/devices/${deviceId}/preview/stop`, { 
            method: 'POST', 
            headers: { 'Content-Type': 'application/json' }, 
            body: JSON.stringify({ channelId: deviceId }) 
          }).then(r => r.json().catch(() => ({}))).catch(() => {})
        }
      }
      // 注意：移除了错误的回退到 /api/gb28181/stop 的逻辑
      // 该接口是停止整个GB28181服务，不应该在停止预览时调用
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

// ===================== PTZ 控制逻辑 =====================
const ptzSpeed = ref(50) // PTZ 速度 (10-100)
const ptzExpanded = ref(false) // PTZ 面板是否展开

// PTZ 状态管理 - 解决快速点击和交错请求问题
let ptzState = {
  moving: false,           // 是否正在移动
  direction: '',           // 当前移动方向
  moveStartTime: 0,        // 移动开始时间
  pendingStop: false,      // 是否有待处理的停止请求
  requestInFlight: false,  // 是否有请求正在进行中
  lastStopTime: 0,         // 上次停止时间
}

// 最小移动时间(ms) - 确保move命令有足够时间被摄像头执行
const MIN_MOVE_DURATION = 150

// 方向映射 - 保持与后端一致的命令名称
const directionMap: Record<string, string> = {
  'up': 'up',
  'down': 'down', 
  'left': 'left',
  'right': 'right',
  'zoomin': 'zoomin',
  'zoomout': 'zoomout'
}

/**
 * 开始 PTZ 移动（长按触发）
 * 优化：添加请求队列和状态锁，防止交错请求
 */
async function startPtz(direction: string) {
  // 如果已经在同方向移动，忽略
  if (ptzState.moving && ptzState.direction === direction) {
    console.debug('[PTZ] Already moving in direction:', direction)
    return
  }
  
  // 如果有请求正在进行，等待
  if (ptzState.requestInFlight) {
    console.debug('[PTZ] Request in flight, queuing move:', direction)
    // 标记新的移动方向，等当前请求完成后处理
    ptzState.direction = direction
    ptzState.pendingStop = false
    return
  }
  
  const deviceId = props.ptzDeviceId ?? (props.device as any)?.deviceId
  const deviceType = props.deviceType || 'gb28181'
  
  if (!deviceId) {
    ElMessage.error('缺少 PTZ 目标设备信息')
    return
  }

  // 设置状态
  ptzState.moving = true
  ptzState.direction = direction
  ptzState.moveStartTime = Date.now()
  ptzState.pendingStop = false
  ptzState.requestInFlight = true

  try {
    console.debug('[PTZ] Starting move:', direction)
    if (deviceType === 'onvif') {
      await sendOnvifPtz('move', direction)
    } else {
      await sendGb28181Ptz(direction)
    }
    console.debug('[PTZ] Move command sent successfully')
  } catch (e: any) {
    console.error('PTZ start failed', e)
    ElMessage.error('PTZ 操作失败: ' + (e.message || '未知错误'))
    ptzState.moving = false
    ptzState.direction = ''
  } finally {
    ptzState.requestInFlight = false
    
    // 检查是否有待处理的停止请求
    if (ptzState.pendingStop) {
      console.debug('[PTZ] Processing pending stop')
      ptzState.pendingStop = false
      await doStopPtz()
    }
  }
}

/**
 * 停止 PTZ 移动
 * 优化：添加最小移动时间保证，防止过快停止
 */
async function stopPtz() {
  // 如果没有在移动，忽略
  if (!ptzState.moving) {
    console.debug('[PTZ] Not moving, ignoring stop')
    return
  }
  
  // 防止重复停止（300ms内的重复stop调用忽略）
  const now = Date.now()
  if (now - ptzState.lastStopTime < 300) {
    console.debug('[PTZ] Stop called too soon, ignoring')
    return
  }
  
  // 如果有请求正在进行，标记待停止
  if (ptzState.requestInFlight) {
    console.debug('[PTZ] Request in flight, marking pending stop')
    ptzState.pendingStop = true
    return
  }
  
  // 计算已移动时间
  const moveDuration = now - ptzState.moveStartTime
  
  // 如果移动时间太短，延迟停止
  if (moveDuration < MIN_MOVE_DURATION) {
    const delay = MIN_MOVE_DURATION - moveDuration
    console.debug('[PTZ] Move duration too short, delaying stop by', delay, 'ms')
    setTimeout(() => {
      if (ptzState.moving && !ptzState.requestInFlight) {
        doStopPtz()
      }
    }, delay)
    return
  }
  
  await doStopPtz()
}

/**
 * 实际执行停止 PTZ
 */
async function doStopPtz() {
  if (!ptzState.moving) return
  
  const deviceId = props.ptzDeviceId ?? (props.device as any)?.deviceId
  const deviceType = props.deviceType || 'gb28181'
  
  if (!deviceId) return

  ptzState.requestInFlight = true
  ptzState.lastStopTime = Date.now()
  
  try {
    console.debug('[PTZ] Sending stop command')
    if (deviceType === 'onvif') {
      await sendOnvifPtz('stop')
    } else {
      await sendGb28181Ptz('stop')
    }
    console.debug('[PTZ] Stop command sent successfully')
  } catch (e: any) {
    console.error('PTZ stop failed', e)
  } finally {
    ptzState.moving = false
    ptzState.direction = ''
    ptzState.requestInFlight = false
  }
}

/**
 * 处理鼠标离开按钮事件
 * 只有当鼠标按键仍然按下时才停止（防止误触发）
 */
function handlePtzMouseLeave(event: MouseEvent) {
  // 检查是否有鼠标按键被按下（buttons > 0 表示有按键按下）
  if (event.buttons > 0 && ptzState.moving) {
    console.debug('[PTZ] Mouse left button while pressed, stopping')
    stopPtz()
  }
}

/**
 * 强制停止 PTZ（点击停止按钮时使用）
 */
async function forceStopPtz() {
  console.debug('[PTZ] Force stop requested')
  // 重置所有状态
  ptzState.pendingStop = false
  ptzState.moving = true // 临时设为true以便doStopPtz执行
  await doStopPtz()
}

/**
 * ONVIF PTZ 控制
 */
async function sendOnvifPtz(command: string, direction?: string) {
  const deviceId = props.ptzDeviceId ?? (props.device as any)?.deviceId
  const profileToken = props.profileToken || 'PROFILE_000'
  
  const payload: Record<string, any> = {
    profileToken: profileToken,
    command: command,
    speed: ptzSpeed.value
  }
  
  if (direction) {
    payload.direction = directionMap[direction] || direction
  }
  
  // 对设备ID进行URL编码（处理冒号等特殊字符）
  const encodedDeviceId = encodeURIComponent(String(deviceId))
  const url = `/api/onvif/devices/${encodedDeviceId}/ptz-control`
  console.log('[PTZ] ONVIF request:', url, payload)
  
  const res = await fetch(url, { 
    method: 'POST', 
    headers: { 'Content-Type': 'application/json' }, 
    body: JSON.stringify(payload) 
  })
  
  const data = await res.json().catch(() => ({}))
  if (!res.ok || (data && !data.success && data.code !== 0)) {
    throw new Error(data?.message || data?.msg || `HTTP ${res.status}`)
  }
  return data
}

/**
 * GB28181 PTZ 控制
 */
async function sendGb28181Ptz(action: string) {
  const deviceId = props.ptzDeviceId ?? (props.device as any)?.deviceId
  const channelId = props.ptzChannelId ?? props.selectedChannelId
  
  if (!channelId) {
    throw new Error('缺少通道信息')
  }
  
  const payload = {
    command: String(action),
    channelId: String(channelId),
    deviceId: String(deviceId),
    speed: Math.round(ptzSpeed.value * 2.55) // 转换为 0-255
  }
  
  const url = `/api/gb28181/devices/${deviceId}/ptz`
  console.log('[PTZ] GB28181 request:', url, payload)
  
  const res = await fetch(url, { 
    method: 'POST', 
    headers: { 'Content-Type': 'application/json' }, 
    body: JSON.stringify(payload) 
  })
  
  const data = await res.json().catch(() => ({}))
  if (!res.ok || (data && !data.success && data.code !== 0)) {
    throw new Error(data?.message || data?.msg || `HTTP ${res.status}`)
  }
  return data
}

/**
 * 兼容旧版调用方式
 */
async function ptz(action: string, opts?: { deviceId?: string | number; channelId?: string | number, speed?: number }) {
  if (action === 'stop') {
    await forceStopPtz()
  } else {
    if (opts?.speed) ptzSpeed.value = opts.speed
    await startPtz(action)
    // 对于单击模式，短暂移动后停止（增加延时确保命令执行）
    setTimeout(() => stopPtz(), 350)
  }
}

defineExpose({ startPreview, startWithStreamInfo, stopPlaybackOnly, stopPreview, ptz, startPtz, stopPtz, forceStopPtz })
</script>

<style scoped>
.preview-player-root { 
  width: 100%; 
  height: 100%;
  transition: all 0.3s ease;
}

.preview-player-root.fullscreen {
  position: fixed !important;
  top: 0 !important;
  left: 0 !important;
  right: 0 !important;
  bottom: 0 !important;
  z-index: 99999 !important;
  width: 100vw !important;
  height: 100vh !important;
  max-width: none !important;
  max-height: none !important;
  margin: 0 !important;
  padding: 0 !important;
  border: none !important;
  border-radius: 0 !important;
  background: #000 !important;
}

/* 进入浏览器全屏时的样式 */
.preview-player-root.fullscreen:fullscreen,
.preview-player-root.fullscreen:-webkit-full-screen,
.preview-player-root.fullscreen:-moz-full-screen,
.preview-player-root.fullscreen:-ms-fullscreen {
  width: 100% !important;
  height: 100% !important;
}

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

.preview-player-root.fullscreen .video-player-wrapper {
  width: 100%;
  height: 100%;
  cursor: pointer;
}

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

/* PTZ 控制面板样式 */
.ptz-wrapper {
  position: absolute;
  right: 12px;
  bottom: 12px;
  z-index: 120;
}

/* 收起状态的浮动按钮 */
.ptz-toggle-btn {
  width: 44px;
  height: 44px;
  background: rgba(0,0,0,0.6);
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  cursor: pointer;
  box-shadow: 0 4px 12px rgba(0,0,0,0.4);
  transition: all 0.2s ease;
  backdrop-filter: blur(4px);
}

.ptz-toggle-btn:hover {
  background: rgba(64,158,255,0.5);
  transform: scale(1.1);
}

/* 展开状态的控制面板 */
.ptz-controls { 
  background: rgba(0,0,0,0.75); 
  padding: 12px; 
  border-radius: 12px; 
  display: flex; 
  flex-direction: column; 
  gap: 8px;
  box-shadow: 0 6px 24px rgba(0,0,0,0.5);
  backdrop-filter: blur(8px);
  min-width: 140px;
  animation: ptz-expand 0.2s ease-out;
}

@keyframes ptz-expand {
  from {
    opacity: 0;
    transform: scale(0.8) translateY(10px);
  }
  to {
    opacity: 1;
    transform: scale(1) translateY(0);
  }
}

/* 面板头部 */
.ptz-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-bottom: 6px;
  border-bottom: 1px solid rgba(255,255,255,0.1);
  margin-bottom: 4px;
}

.ptz-title {
  color: rgba(255,255,255,0.9);
  font-size: 12px;
  font-weight: 500;
}

.ptz-close-btn {
  width: 20px !important;
  height: 20px !important;
  min-width: 20px !important;
  padding: 0 !important;
  font-size: 12px !important;
  background: transparent !important;
  border: none !important;
  color: rgba(255,255,255,0.6) !important;
}

.ptz-close-btn:hover {
  color: #fff !important;
  background: rgba(255,255,255,0.1) !important;
}

/* 速度调节 */
.ptz-speed {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 0;
  border-bottom: 1px solid rgba(255,255,255,0.1);
  margin-bottom: 4px;
}
.ptz-speed .speed-label {
  color: rgba(255,255,255,0.7);
  font-size: 11px;
  white-space: nowrap;
}
.ptz-speed .speed-value {
  color: #fff;
  font-size: 11px;
  min-width: 32px;
  text-align: right;
}
.ptz-speed :deep(.el-slider) {
  flex: 1;
  min-width: 60px;
}
.ptz-speed :deep(.el-slider__runway) {
  background: rgba(255,255,255,0.2);
}
.ptz-speed :deep(.el-slider__bar) {
  background: #409eff;
}
.ptz-speed :deep(.el-slider__button) {
  width: 12px;
  height: 12px;
  border-color: #409eff;
}

/* 方向控制区域 */
.ptz-direction {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.ptz-row { 
  display: flex; 
  justify-content: center; 
  gap: 2px;
}

.ptz-btn-placeholder {
  width: 36px;
  height: 36px;
}

.ptz-btn {
  width: 36px !important;
  height: 36px !important;
  padding: 0 !important;
  font-size: 14px !important;
  background: rgba(255,255,255,0.1) !important;
  border: 1px solid rgba(255,255,255,0.2) !important;
  color: #fff !important;
  border-radius: 6px !important;
  transition: all 0.15s ease !important;
  display: flex !important;
  align-items: center !important;
  justify-content: center !important;
}

.ptz-btn:hover {
  background: rgba(64,158,255,0.3) !important;
  border-color: rgba(64,158,255,0.5) !important;
  transform: scale(1.05);
}

.ptz-btn:active {
  background: rgba(64,158,255,0.5) !important;
  transform: scale(0.95);
}

.ptz-stop {
  background: rgba(245,108,108,0.3) !important;
  border-color: rgba(245,108,108,0.5) !important;
}

.ptz-stop:hover {
  background: rgba(245,108,108,0.5) !important;
}

/* 缩放控制 */
.ptz-zoom-controls {
  display: flex;
  justify-content: center;
  gap: 8px;
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px solid rgba(255,255,255,0.1);
}

.ptz-zoom-btn {
  flex: 1;
  height: 32px !important;
  padding: 0 8px !important;
  font-size: 12px !important;
  background: rgba(255,255,255,0.1) !important;
  border: 1px solid rgba(255,255,255,0.2) !important;
  color: #fff !important;
  border-radius: 6px !important;
  transition: all 0.15s ease !important;
}

.ptz-zoom-btn:hover {
  background: rgba(103,194,58,0.3) !important;
  border-color: rgba(103,194,58,0.5) !important;
}

.ptz-zoom-btn:active {
  background: rgba(103,194,58,0.5) !important;
}

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

/* PTZ 控制面板样式 */
.ptz-wrapper {
  position: absolute;
  right: 12px;
  bottom: 12px;
  z-index: 120;
}

.video-player-wrapper {
  cursor: pointer;
}

.video-player-wrapper:hover::after {
  content: '双击全屏（ESC 退出）';
  position: absolute;
  bottom: 16px;
  left: 50%;
  transform: translateX(-50%);
  background: rgba(0,0,0,0.7);
  color: #fff;
  padding: 8px 16px;
  border-radius: 6px;
  font-size: 12px;
  pointer-events: none;
  white-space: nowrap;
}
</style>