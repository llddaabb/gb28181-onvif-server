<template>
  <div class="playback-player-root" ref="rootRef" :class="{ 'fullscreen': isFullscreen }">
    <div class="video-player-wrapper" v-loading="loading" :class="{ 'fullscreen': isFullscreen }">
      
      <div :id="containerId" ref="playerContainer" class="video-player-container"></div>
      
      <div v-if="error" class="video-error">
        <el-icon size="48"><VideoCamera /></el-icon>
        <p>{{ error }}</p>
        <p v-if="isH265EncodingError" class="error-hint">
          您的浏览器不支持 H.265 视频编码格式。
          <br>请下载视频文件后使用 VLC 或其他播放器观看。
        </p>
        <div class="error-actions">
          <el-button type="primary" @click="retry">重试</el-button>
          <el-button v-if="downloadUrl" type="success" @click="downloadVideo">
            <el-icon><Download /></el-icon> 下载视频
          </el-button>
        </div>
      </div>

      <!-- 回放控制栏 -->
      <div class="playback-controls" v-if="isPlaying && !error">
        <!-- 进度条 -->
        <div class="progress-bar-wrapper">
          <el-slider 
            v-model="currentProgress"
            :min="0"
            :max="duration"
            :format-tooltip="formatProgressTooltip"
            @change="onSeek"
            @input="onSeeking"
            class="progress-slider"
          />
        </div>
        
        <!-- 控制按钮 -->
        <div class="controls-row">
          <div class="controls-left">
            <el-button 
              :icon="isPaused ? 'VideoPlay' : 'VideoPause'" 
              circle 
              size="small"
              @click="togglePause"
            >
              {{ isPaused ? '▶' : '⏸' }}
            </el-button>
            <el-button icon="RefreshLeft" circle size="small" @click="seekBackward">
              ⏪
            </el-button>
            <el-button icon="RefreshRight" circle size="small" @click="seekForward">
              ⏩
            </el-button>
            <span class="time-display">
              {{ formatTime(currentProgress) }} / {{ formatTime(duration) }}
            </span>
          </div>
          
          <div class="controls-center">
            <span class="recording-info" v-if="recordingInfo">
              {{ recordingInfo.fileName || recordingInfo.name || '' }}
            </span>
          </div>
          
          <div class="controls-right">
            <!-- 倍速选择 -->
            <el-dropdown @command="changeSpeed" trigger="click">
              <span class="speed-btn">{{ playbackSpeed }}x</span>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item :command="0.5">0.5x</el-dropdown-item>
                  <el-dropdown-item :command="1">1x</el-dropdown-item>
                  <el-dropdown-item :command="1.5">1.5x</el-dropdown-item>
                  <el-dropdown-item :command="2">2x</el-dropdown-item>
                  <el-dropdown-item :command="4">4x</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
            
            <!-- 音量控制 -->
            <div class="volume-control">
              <el-button circle size="small" @click="toggleMute">
                {{ isMuted ? '🔇' : '🔊' }}
              </el-button>
              <el-slider 
                v-model="volume" 
                :min="0" 
                :max="100" 
                size="small"
                class="volume-slider"
                @input="onVolumeChange"
              />
            </div>
            
            <!-- 全屏 -->
            <el-button circle size="small" @click="toggleFullscreen">
              {{ isFullscreen ? '⛶' : '⛶' }}
            </el-button>
            
            <!-- 下载 -->
            <el-button circle size="small" @click="downloadRecording" v-if="downloadUrl">
              ⬇
            </el-button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onUnmounted, onMounted, nextTick, watch, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { VideoCamera, Download } from '@element-plus/icons-vue'

/**
 * 动态获取 Jessibuca 构造函数
 */
async function getJessibuca() {
  const w = (window as any)
  if (w && (w.Jessibuca || w.jessibuca || w.JB)) {
    return w.Jessibuca || w.jessibuca || w.JB
  }

  const scriptUrl = '/jessibuca/jessibuca.js'
  await new Promise<void>((resolve, reject) => {
    const existing = document.querySelector(`script[src="${scriptUrl}"]`)
    if (existing) {
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

/**
 * 动态加载 EasyPlayerPro (支持 H.265 软解码)
 */
async function getEasyPlayerPro() {
  const w = (window as any)
  if (w && w.EasyPlayerPro) {
    return w.EasyPlayerPro
  }

  const scriptUrl = '/easyplayer/EasyPlayer-pro.js'
  await new Promise<void>((resolve, reject) => {
    const existing = document.querySelector(`script[src="${scriptUrl}"]`)
    if (existing) {
      if ((existing as HTMLScriptElement).getAttribute('data-loaded') === '1') {
        resolve()
      } else {
        existing.addEventListener('load', () => resolve())
        existing.addEventListener('error', () => reject(new Error('Failed to load EasyPlayer script')))
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
    s.onerror = () => reject(new Error('Failed to load EasyPlayer script'))
    document.head.appendChild(s)
  })

  if (w && w.EasyPlayerPro) {
    return w.EasyPlayerPro
  }
  throw new Error('EasyPlayerPro not found on window after loading script')
}

interface RecordingInfo {
  fileName?: string
  name?: string
  startTime?: string
  endTime?: string
  duration?: number
  app?: string
  stream?: string
  playUrl?: string
  flvUrl?: string
  mp4Url?: string
  downloadUrl?: string
}

const props = defineProps({
  // 播放URL（优先使用 flvUrl，回退到 mp4Url）
  playUrl: { type: String, required: false, default: '' },
  // FLV流地址（推荐用于 Jessibuca）
  flvUrl: { type: String, required: false, default: '' },
  // MP4直接地址（备选）
  mp4Url: { type: String, required: false, default: '' },
  // 下载地址
  downloadUrl: { type: String, required: false, default: '' },
  // 录像信息
  recordingInfo: { type: Object as () => RecordingInfo | null, required: false, default: null },
  // 视频时长（秒）
  videoDuration: { type: Number, required: false, default: 0 },
  // 自动播放
  autoplay: { type: Boolean, default: true },
  // 默认高度
  defaultHeight: { type: [Number, String], required: false, default: 480 },
  // 强制按 H.265 软解（后端检测到 HEVC 时传入）
  forceH265: { type: Boolean, default: false },
  // 可选的编码提示
  codec: { type: String, required: false, default: '' },
})

const emit = defineEmits(['playing', 'paused', 'ended', 'error', 'timeupdate', 'fullscreenChange'])

const playerContainer = ref<HTMLElement | null>(null)
const rootRef = ref<HTMLElement | null>(null)
const containerId = `playback-container-${Math.random().toString(36).slice(2,9)}`
const loading = ref(false)
const error = ref('')
const isH265EncodingError = ref(false)
const isFullscreen = ref(false)
const isPlaying = ref(false)
const isPaused = ref(false)
const isMuted = ref(false)

// 重试机制
const retryCount = ref(0)
const maxRetries = 3

// 播放控制状态
const currentProgress = ref(0)
const duration = ref(0)
const playbackSpeed = ref(1)
const volume = ref(80)

let playerInstance: any = null
let nativeVideoElement: HTMLVideoElement | null = null
let easyPlayerInstance: any = null  // EasyPlayerPro 实例
let progressTimer: number | null = null

// 计算实际使用的播放URL
const effectivePlayUrl = computed(() => {
  // 优先使用 FLV 流（Jessibuca 播放）
  if (props.flvUrl) return props.flvUrl
  // 其次使用传入的 playUrl
  if (props.playUrl) return props.playUrl
  // 最后使用 MP4 地址
  if (props.mp4Url) return props.mp4Url
  return ''
})

// 下载URL
const downloadUrl = computed(() => {
  return props.downloadUrl || props.mp4Url || props.playUrl || ''
})

// 是否为 MP4 源
const isMp4Source = computed(() => {
  const url = effectivePlayUrl.value
  if (!url) return false
  return url.endsWith('.mp4') || url.includes('.mp4?')
})

// 判断是否使用原生 video 元素（MP4 格式）
const useNativeVideo = computed(() => {
  // 完全禁用原生 video 标签，所有播放都使用软解
  return false
})

// H.265 检测状态 - 不使用 h265webjs，直接提供下载
const detectedH265 = ref(false)

/**
 * 初始化播放器
 */
const initPlayer = async () => {
  const url = effectivePlayUrl.value
  if (!url) {
    error.value = '无效的播放地址'
    return
  }

  loading.value = true
  error.value = ''

  try {
    await nextTick()
    
    const container = document.getElementById(containerId)
    if (!container) {
      throw new Error('播放器容器不存在')
    }

    // 清理旧实例
    await cleanup()

    if (useNativeVideo.value) {
      // 使用原生 HTML5 Video 播放 MP4
      await initNativeVideoPlayer(container, url)
    } else if (isMp4Source.value) {
      // MP4 且标记为 H.265 时，直接使用 EasyPlayerPro 软解
      await initEasyPlayer(container, url)
    } else {
      // 使用 Jessibuca 播放 FLV 流
      await initJessibucaPlayer(container, url)
    }

    isPlaying.value = true
    loading.value = false
    
    // 启动进度更新定时器
    startProgressTimer()
    
  } catch (e: any) {
    console.error('[PlaybackPlayer] Init failed:', e)
    error.value = e.message || '播放器初始化失败'
    loading.value = false
    emit('error', e)
  }
}

/**
 * 使用原生 Video 元素播放 MP4
 */
const initNativeVideoPlayer = async (container: HTMLElement, url: string) => {
  const video = document.createElement('video')
  video.src = url
  video.controls = false // 使用自定义控制栏
  video.autoplay = props.autoplay
  video.style.width = '100%'
  video.style.height = '100%'
  video.style.objectFit = 'contain'
  video.style.backgroundColor = '#000'
  
  // H.265 检测
  let hasVideoFrame = false
  let checkCount = 0
  const maxChecks = 6
  let switchedToEasyPlayer = false  // 防止重复切换
  
  const switchToEasyPlayer = async () => {
    if (switchedToEasyPlayer) return  // 防止重复调用
    switchedToEasyPlayer = true
    
    console.log('[PlaybackPlayer] H.265 detected, switching to EasyPlayerPro for soft decoding')
    
    // 先移除所有事件监听器，防止触发错误
    video.onloadedmetadata = null
    video.onloadeddata = null
    video.ontimeupdate = null
    video.onplay = null
    video.onpause = null
    video.onended = null
    video.onerror = null
    
    video.pause()
    video.src = ''
    
    // 从 DOM 移除 video 元素
    if (video.parentNode) {
      video.parentNode.removeChild(video)
    }
    
    nativeVideoElement = null
    detectedH265.value = true
    loading.value = true
    
    // 检查容器是否仍然有效
    const currentContainer = document.getElementById(containerId)
    if (!currentContainer) {
      console.error('[PlaybackPlayer] Container not found, cannot switch to EasyPlayer')
      error.value = '播放器容器不存在'
      loading.value = false
      return
    }
    
    try {
      // 使用 EasyPlayerPro 播放 H.265
      await initEasyPlayer(currentContainer, url)
    } catch (e: any) {
      console.error('[PlaybackPlayer] EasyPlayerPro init failed:', e)
      // EasyPlayer 初始化失败，显示下载提示
      error.value = '您的浏览器不支持 H.265 视频编码格式'
      isH265EncodingError.value = true
      loading.value = false
    }
  }
  
  const checkVideoDecoding = () => {
    if (switchedToEasyPlayer) return  // 已切换，停止检查
    checkCount++
    // 如果视频已在播放但宽高为0，说明解码失败（H.265）
    if (video.readyState >= 2 && !video.paused && video.currentTime > 0) {
      if (video.videoWidth === 0 || video.videoHeight === 0) {
        if (checkCount >= maxChecks && !hasVideoFrame) {
          switchToEasyPlayer()
          return
        }
      } else {
        hasVideoFrame = true
      }
    }
    if (checkCount < maxChecks && !hasVideoFrame && !error.value && !switchedToEasyPlayer) {
      setTimeout(checkVideoDecoding, 500)
    }
  }
  
  // 事件监听
  video.onloadedmetadata = () => {
    duration.value = video.duration || props.videoDuration || 0
    console.log('[PlaybackPlayer] Video duration:', duration.value, 'videoWidth:', video.videoWidth, 'videoHeight:', video.videoHeight)
    
    // 如果元数据加载完但视频尺寸为0，可能是H.265
    if (video.videoWidth === 0 || video.videoHeight === 0) {
      console.warn('[PlaybackPlayer] Video dimensions are 0, checking decoding...')
      setTimeout(checkVideoDecoding, 1000)
    }
  }
  
  video.onloadeddata = () => {
    console.log('[PlaybackPlayer] Video data loaded, videoWidth:', video.videoWidth)
    if (video.videoWidth > 0 && video.videoHeight > 0) {
      hasVideoFrame = true
    }
  }
  
  video.ontimeupdate = () => {
    if (switchedToEasyPlayer) return
    currentProgress.value = video.currentTime
    emit('timeupdate', video.currentTime)
    
    // 持续检测：如果播放了一段时间但仍没有视频帧
    if (!hasVideoFrame && video.currentTime > 2 && (video.videoWidth === 0 || video.videoHeight === 0)) {
      console.warn('[PlaybackPlayer] Playing but no video frames detected, likely H.265')
      switchToEasyPlayer()
    }
  }
  
  video.onplay = () => {
    isPaused.value = false
    emit('playing')
    // 开始检测视频解码
    setTimeout(checkVideoDecoding, 1000)
  }
  
  video.onpause = () => {
    isPaused.value = true
    emit('paused')
  }
  
  video.onended = () => {
    isPlaying.value = false
    emit('ended')
  }
  
  video.onerror = (e) => {
    // 如果已切换到 EasyPlayer，忽略这个错误
    if (switchedToEasyPlayer) return
    
    console.error('[PlaybackPlayer] Video error:', e)
    // 检查是否是编码格式不支持（通常是H.265）
    const videoError = (video as any).error
    if (videoError) {
      console.log('[PlaybackPlayer] Video error code:', videoError.code, 'message:', videoError.message)
      // MEDIA_ERR_SRC_NOT_SUPPORTED (4) 或 MEDIA_ERR_DECODE (3) 通常表示编码不支持
      if (videoError.code === 3 || videoError.code === 4) {
        error.value = '浏览器不支持此视频编码格式'
        isH265EncodingError.value = true
      } else {
        error.value = '视频加载失败'
        isH265EncodingError.value = false
      }
    } else {
      error.value = '视频加载失败'
      isH265EncodingError.value = false
    }
    emit('error', e)
  }
  
  container.innerHTML = ''
  container.appendChild(video)
  nativeVideoElement = video
  
  // 设置音量
  video.volume = volume.value / 100
  video.muted = isMuted.value
}

/**
 * 使用 EasyPlayerPro 播放 H.265 视频 (WASM 软解码)
 */
const initEasyPlayer = async (container: HTMLElement, url: string) => {
  // 验证容器
  if (!container || !container.parentNode) {
    throw new Error('EasyPlayer container is invalid')
  }
  
  const EasyPlayerProClass = await getEasyPlayerPro()
  
  // 清理容器内容
  container.innerHTML = ''
  
  // 创建播放器容器 div（EasyPlayer 需要一个空的容器）
  const playerDiv = document.createElement('div')
  playerDiv.style.width = '100%'
  playerDiv.style.height = '100%'
  playerDiv.style.position = 'relative'
  playerDiv.style.backgroundColor = '#000'
  container.appendChild(playerDiv)
  
  let fetchErrorCount = 0  // 记录 fetchError 次数
  
  const player = new EasyPlayerProClass(playerDiv, {
    isLive: false,  // 点播模式
    hasAudio: true,
    isMute: isMuted.value,
    stretch: true,   // 拉伸填充容器
    bufferTime: 0.5,
    loadTimeOut: 30,
    loadTimeReplay: 3,
    // 强制 WASM 软解码 H.265
    isH265: true,     // 明确告诉播放器这是 H.265
    MSE: false,       // 禁用 MSE（浏览器 MSE 不支持 H.265）
    useMSE: false,    // 禁用 MSE
    WCS: false,       // 关闭 WebCodec
    useWCS: false,
    WASM: true,       // 启用 WASM 软解码
    useWasm: true,
    useSIMD: true,    // 使用 SIMD 加速
    autoWasm: false,  // 不自动切换，强制 WASM
    hardDecodingNotSupportAutoWasm: false,
    decoderErrorAutoWasm: false,  // 禁止错误时自动切换
    // 关键：强制使用 WASM 解码 MP4（而不是 MSE）
    isWasmMp4: true,
    // 禁止所有自动重播行为（重播会切换到 MSE）
    streamErrorReplay: false,
    streamEndReplay: false,
    loadingTimeoutReplay: false,
    heartTimeoutReplay: false,
    mseDecodeErrorReplay: false,
    wcsDecodeErrorReplay: false,
    wasmDecodeErrorReplay: false,
    simdDecodeErrorReplay: false,
    playFailedAndReplay: false,
    gpuDecoder: false,
    canvasRender: true,  // 使用 Canvas 渲染
    useCanvasRender: true,
    useVideoRender: false,
    mseUseCanvasRender: false,  // 即使 MSE 也禁用
    // playbackConfig 中也要禁用 MSE
    playbackConfig: {
      useMSE: false,
      useWCS: false,
      isH265: true,
      isMp4: true,
      isWasmMp4: true,
      hasLive: false,
    },
    debug: true,   // 开启调试
    isBand: false,
    btns: {
      fullscreen: false,
      screenshot: false,
      play: false,
      audio: false,
      record: false,
      stretch: false,
      zoom: false,
      ptz: false,
      quality: false,
    }
  })
  
  // 事件监听
  player.on('play', () => {
    console.log('[PlaybackPlayer] EasyPlayer play')
    isPaused.value = false
    loading.value = false
    emit('playing')
  })
  
  player.on('pause', () => {
    isPaused.value = true
    emit('paused')
  })
  
  player.on('videoInfo', (info: any) => {
    console.log('[PlaybackPlayer] EasyPlayer videoInfo:', info)
    if (info && info.duration) {
      duration.value = info.duration
    }
  })
  
  player.on('timestamps', (ts: number) => {
    // 播放时间回调（秒）
    currentProgress.value = ts
    emit('timeupdate', ts)
  })
  
  player.on('liveEnd', () => {
    console.log('[PlaybackPlayer] EasyPlayer playback ended')
    isPlaying.value = false
    emit('ended')
  })
  
  player.on('error', (e: any) => {
    // fetchError 是初始加载时的常见错误，手动重试最多3次
    if (e === 'fetchError' || (e && e.message && e.message.includes('fetch'))) {
      fetchErrorCount++
      console.warn(`[PlaybackPlayer] EasyPlayer fetchError (${fetchErrorCount}/3)`)
      if (fetchErrorCount < 3) {
        // 手动重试
        setTimeout(() => {
          if (easyPlayerInstance) {
            console.log(`[PlaybackPlayer] EasyPlayer 重试播放 (${fetchErrorCount}/3)`)
            easyPlayerInstance.play(url).catch(() => {})
          }
        }, 1000)
        return  // 不报错，等待重试
      }
    }
    console.error('[PlaybackPlayer] EasyPlayer error:', e)
    error.value = 'H.265 视频播放出错'
    isH265EncodingError.value = true
    loading.value = false
    emit('error', e)
  })
  
  player.on('timeout', () => {
    console.warn('[PlaybackPlayer] EasyPlayer timeout')
    if (retryCount.value < maxRetries) {
      retryCount.value++
      console.log(`[PlaybackPlayer] EasyPlayer timeout retry (${retryCount.value}/${maxRetries})`)
      setTimeout(() => {
        if (easyPlayerInstance) {
          easyPlayerInstance.play(url).catch(() => {})
        }
      }, 2000)
    } else {
      error.value = '视频播放超时，请下载视频文件观看'
      isH265EncodingError.value = true
      loading.value = false
      emit('error', new Error('timeout'))
    }
  })
  
  // 开始播放 - MP4 点播使用 playback()
  // 由于我们已经在配置中强制 WASM: true, MSE: false, isWasmMp4: true
  // playback() 会尊重这些配置使用 WASM 软解而不是 MSE
  console.log('[PlaybackPlayer] Starting EasyPlayer playback with URL:', url)
  await player.playback(url)
  easyPlayerInstance = player
  
  // 设置时长
  if (props.videoDuration) {
    duration.value = props.videoDuration
  } else if (props.recordingInfo?.duration) {
    duration.value = props.recordingInfo.duration
  }
  
  loading.value = false
  console.log('[PlaybackPlayer] EasyPlayer initialized')
}

/**
 * 使用 Jessibuca 播放（支持H.265的MP4文件和FLV流）
 */
const initJessibucaPlayer = async (container: HTMLElement, url: string) => {
  const JessibucaClass = await getJessibuca()
  
  const player = new JessibucaClass({
    container: container,
    videoBuffer: 0.5,    // 录像回放使用较小的缓冲
    isResize: true,
    text: '',
    loadingText: '正在加载录像...',
    debug: false,
    showBandwidth: false,
    operateBtns: {
      fullscreen: false,
      screenshot: false,
      play: false,
      audio: false,
      recorder: false
    },
    forceNoOffscreen: true,
    isNotMute: !isMuted.value,
    hasAudio: true,
    // 关键配置：decoder 和 wasm 路径
    decoder: '/jessibuca/decoder.js',
    wasmPath: '/jessibuca/decoder.wasm',
    useMSE: url.includes('.flv'),  // FLV使用MSE，MP4不使用
    useWCS: false,  // 关闭 WebCodec
    // MP4录像需要的配置
    isLive: false,  // 录像回放不是直播
    // 增加超时时间（设备端录像推流可能需要更长时间）
    loadingTimeout: 20,   // 20秒加载超时
    heartTimeout: 15,     // 15秒心跳超时
  })
  
  // 事件监听
  player.on('play', () => {
    isPaused.value = false
    retryCount.value = 0  // 播放成功，重置重试计数
    emit('playing')
  })
  
  player.on('loadfinish', () => {
    console.log('[PlaybackPlayer] Jessibuca loadfinish')
    loading.value = false
  })
  
  player.on('videoInfo', (info: any) => {
    console.log('[PlaybackPlayer] Video info:', info)
    if (info && info.width && info.height) {
      console.log(`[PlaybackPlayer] Video resolution: ${info.width}x${info.height}`)
    }
  })
  
  player.on('timeUpdate', (ts: number) => {
    // Jessibuca的timeUpdate返回的是毫秒
    currentProgress.value = ts / 1000
    emit('timeupdate', currentProgress.value)
  })
  
  player.on('pause', () => {
    isPaused.value = true
    emit('paused')
  })
  
  player.on('playbackEnded', () => {
    console.log('[PlaybackPlayer] Playback ended')
    isPlaying.value = false
    emit('ended')
  })
  
  player.on('error', (e: any) => {
    console.error('[PlaybackPlayer] Jessibuca error:', e)
    // 尝试重试
    if (retryCount.value < maxRetries) {
      retryCount.value++
      console.log(`[PlaybackPlayer] 播放出错，尝试重试 (${retryCount.value}/${maxRetries})...`)
      setTimeout(() => {
        if (playerInstance) {
          playerInstance.play(url).catch(() => {})
        }
      }, 2000)
    } else {
      error.value = '视频播放出错，可能是网络问题或视频格式不支持'
      isH265EncodingError.value = true
      emit('error', e)
    }
  })
  
  player.on('timeout', () => {
    console.log('[PlaybackPlayer] 播放超时, retryCount:', retryCount.value)
    // 超时时尝试重试
    if (retryCount.value < maxRetries) {
      retryCount.value++
      console.log(`[PlaybackPlayer] 超时重试 (${retryCount.value}/${maxRetries})...`)
      setTimeout(() => {
        if (playerInstance) {
          playerInstance.play(url).catch(() => {})
        }
      }, 2000)
    } else {
      error.value = '播放超时，请检查网络连接或下载录像文件观看'
      isH265EncodingError.value = true
      emit('error', new Error('timeout'))
    }
  })
  
  // 开始播放
  console.log('[PlaybackPlayer] Starting playback with URL:', url)
  await player.play(url)
  playerInstance = player
  
  // 尝试获取时长（Jessibuca可能没有直接的时长API，使用props传入的）
  if (props.videoDuration) {
    duration.value = props.videoDuration
  } else if (props.recordingInfo?.duration) {
    duration.value = props.recordingInfo.duration
  }
  
  console.log('[PlaybackPlayer] Jessibuca player initialized, duration:', duration.value)
}

/**
 * 清理播放器
 */
const cleanup = async () => {
  stopProgressTimer()
  
  try {
    if (playerInstance) {
      if (typeof playerInstance.destroy === 'function') {
        await playerInstance.destroy().catch(() => {})
      }
      playerInstance = null
    }
    
    if (easyPlayerInstance) {
      if (typeof easyPlayerInstance.destroy === 'function') {
        easyPlayerInstance.destroy()
      }
      easyPlayerInstance = null
    }
    
    if (nativeVideoElement) {
      nativeVideoElement.pause()
      nativeVideoElement.src = ''
      nativeVideoElement.load()
      nativeVideoElement = null
    }
    
    const container = document.getElementById(containerId)
    if (container) {
      container.innerHTML = ''
    }
  } catch (e) {
    console.warn('[PlaybackPlayer] cleanup error:', e)
  }
  
  isPlaying.value = false
  isPaused.value = false
  currentProgress.value = 0
  detectedH265.value = false
}

/**
 * 重试播放
 */
const retry = () => {
  error.value = ''
  isH265EncodingError.value = false
  detectedH265.value = false
  initPlayer()
}

/**
 * 切换暂停/播放
 */
const togglePause = () => {
  if (nativeVideoElement) {
    if (nativeVideoElement.paused) {
      nativeVideoElement.play()
    } else {
      nativeVideoElement.pause()
    }
  } else if (easyPlayerInstance) {
    if (isPaused.value) {
      // EasyPlayer 没有 resume，需要重新 play
      const url = effectivePlayUrl.value
      easyPlayerInstance.playback(url).catch(() => {})
    } else {
      easyPlayerInstance.pause()
    }
  } else if (playerInstance) {
    if (isPaused.value) {
      playerInstance.play()
    } else {
      playerInstance.pause()
    }
  }
}

/**
 * 跳转进度
 */
const onSeek = (value: number) => {
  if (nativeVideoElement) {
    nativeVideoElement.currentTime = value
  } else if (easyPlayerInstance) {
    // EasyPlayer 使用 seekTime 方法（单位：秒）
    easyPlayerInstance.seekTime(value)
  }
  // Jessibuca 暂不支持 seek
}

const onSeeking = (value: number) => {
  // 拖拽时的预览（可选）
}

/**
 * 快退 10 秒
 */
const seekBackward = () => {
  if (nativeVideoElement) {
    nativeVideoElement.currentTime = Math.max(0, nativeVideoElement.currentTime - 10)
  } else if (easyPlayerInstance) {
    easyPlayerInstance.seekTime(Math.max(0, currentProgress.value - 10))
  }
}

/**
 * 快进 10 秒
 */
const seekForward = () => {
  if (nativeVideoElement) {
    nativeVideoElement.currentTime = Math.min(duration.value, nativeVideoElement.currentTime + 10)
  } else if (easyPlayerInstance) {
    easyPlayerInstance.seekTime(Math.min(duration.value, currentProgress.value + 10))
  }
}

/**
 * 改变播放速度
 */
const changeSpeed = (speed: number) => {
  playbackSpeed.value = speed
  if (nativeVideoElement) {
    nativeVideoElement.playbackRate = speed
  } else if (easyPlayerInstance) {
    // EasyPlayer 使用 setRate 方法
    easyPlayerInstance.setRate(speed)
  }
}

/**
 * 切换静音
 */
const toggleMute = () => {
  isMuted.value = !isMuted.value
  if (nativeVideoElement) {
    nativeVideoElement.muted = isMuted.value
  } else if (easyPlayerInstance) {
    easyPlayerInstance.setMute(isMuted.value)
  } else if (playerInstance) {
    if (isMuted.value) {
      playerInstance.mute()
    } else {
      playerInstance.cancelMute()
    }
  }
}

/**
 * 音量变化
 */
const onVolumeChange = (val: number) => {
  if (nativeVideoElement) {
    nativeVideoElement.volume = val / 100
  } else if (easyPlayerInstance) {
    // EasyPlayer 音量控制 (0-1)
    // 暂无直接的 setVolume API，通过 setMute 控制
    if (val === 0) {
      easyPlayerInstance.setMute(true)
      isMuted.value = true
    } else if (isMuted.value) {
      easyPlayerInstance.setMute(false)
      isMuted.value = false
    }
  } else if (playerInstance) {
    playerInstance.setVolume(val / 100)
  }
}

/**
 * 全屏切换
 */
const toggleFullscreen = async () => {
  if (!rootRef.value) return
  
  try {
    if (!isFullscreen.value) {
      const element = rootRef.value as any
      if (element.requestFullscreen) {
        await element.requestFullscreen()
      } else if (element.webkitRequestFullscreen) {
        await element.webkitRequestFullscreen()
      }
      isFullscreen.value = true
    } else {
      const doc = document as any
      if (doc.exitFullscreen) {
        await doc.exitFullscreen()
      } else if (doc.webkitExitFullscreen) {
        await doc.webkitExitFullscreen()
      }
      isFullscreen.value = false
    }
    emit('fullscreenChange', isFullscreen.value)
  } catch (e) {
    console.warn('[PlaybackPlayer] Fullscreen toggle failed:', e)
  }
}

/**
 * 下载视频（用于错误提示中的下载按钮）
 */
const downloadVideo = () => {
  const url = downloadUrl.value
  if (!url) {
    ElMessage.warning('无可用的下载地址')
    return
  }
  
  const a = document.createElement('a')
  a.href = url
  a.download = props.recordingInfo?.fileName || 'recording.mp4'
  a.target = '_blank'
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  ElMessage.success('开始下载视频')
}

/**
 * 下载录像
 */
const downloadRecording = () => {
  const url = props.downloadUrl || props.mp4Url || effectivePlayUrl.value
  if (!url) {
    ElMessage.warning('无可下载的地址')
    return
  }
  
  const a = document.createElement('a')
  a.href = url
  a.download = props.recordingInfo?.fileName || 'recording.mp4'
  a.target = '_blank'
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
}

/**
 * 格式化时间
 */
const formatTime = (seconds: number): string => {
  if (!seconds || isNaN(seconds)) return '00:00'
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = Math.floor(seconds % 60)
  if (h > 0) {
    return `${h.toString().padStart(2, '0')}:${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`
  }
  return `${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`
}

const formatProgressTooltip = (value: number): string => {
  return formatTime(value)
}

/**
 * 进度更新定时器
 */
const startProgressTimer = () => {
  stopProgressTimer()
  progressTimer = window.setInterval(() => {
    if (nativeVideoElement && !nativeVideoElement.paused) {
      currentProgress.value = nativeVideoElement.currentTime
    }
  }, 500)
}

const stopProgressTimer = () => {
  if (progressTimer) {
    clearInterval(progressTimer)
    progressTimer = null
  }
}

// 监听 playUrl 变化
watch(() => [props.playUrl, props.flvUrl, props.mp4Url], () => {
  if (effectivePlayUrl.value) {
    initPlayer()
  }
}, { immediate: true })

// 监听全屏状态变化（处理 ESC 退出）
onMounted(() => {
  const handleFullscreenChange = () => {
    const doc = document as any
    if (!doc.fullscreenElement && !doc.webkitFullscreenElement) {
      isFullscreen.value = false
    }
  }
  document.addEventListener('fullscreenchange', handleFullscreenChange)
  document.addEventListener('webkitfullscreenchange', handleFullscreenChange)
})

onUnmounted(() => {
  cleanup()
})

// 暴露方法供父组件调用
defineExpose({
  play: initPlayer,
  stop: cleanup,
  pause: togglePause,
  seek: onSeek,
})
</script>

<style scoped>
.playback-player-root {
  position: relative;
  width: 100%;
  background: #000;
  border-radius: 4px;
  overflow: hidden;
}

.playback-player-root.fullscreen {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 9999;
  border-radius: 0;
}

.video-player-wrapper {
  position: relative;
  width: 100%;
  height: v-bind('typeof defaultHeight === "number" ? defaultHeight + "px" : defaultHeight');
  background: #000;
}

.video-player-wrapper.fullscreen {
  height: 100vh;
}

.video-player-container {
  width: 100%;
  height: 100%;
}

.video-error {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  text-align: center;
  color: #909399;
  max-width: 500px;
  padding: 20px;
}

.video-error p {
  margin: 10px 0;
  font-size: 14px;
}

.error-hint {
  color: #e6a23c;
  font-size: 13px;
  line-height: 1.6;
  margin-top: 15px;
}

.error-actions {
  margin-top: 20px;
  display: flex;
  gap: 10px;
  justify-content: center;
}

/* 回放控制栏 */
.playback-controls {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  background: linear-gradient(transparent, rgba(0, 0, 0, 0.8));
  padding: 10px 15px;
  transition: opacity 0.3s;
}

.playback-player-root:not(:hover) .playback-controls {
  opacity: 0;
}

.playback-player-root:hover .playback-controls {
  opacity: 1;
}

.progress-bar-wrapper {
  margin-bottom: 8px;
}

.progress-slider {
  width: 100%;
}

.progress-slider :deep(.el-slider__runway) {
  height: 4px;
  background: rgba(255, 255, 255, 0.3);
}

.progress-slider :deep(.el-slider__bar) {
  background: #409eff;
  height: 4px;
}

.progress-slider :deep(.el-slider__button) {
  width: 12px;
  height: 12px;
  border: 2px solid #409eff;
}

.controls-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: #fff;
}

.controls-left,
.controls-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.controls-center {
  flex: 1;
  text-align: center;
  overflow: hidden;
}

.recording-info {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.8);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.time-display {
  font-size: 12px;
  color: #fff;
  margin-left: 10px;
}

.speed-btn {
  color: #fff;
  font-size: 12px;
  cursor: pointer;
  padding: 4px 8px;
  background: rgba(255, 255, 255, 0.2);
  border-radius: 4px;
}

.speed-btn:hover {
  background: rgba(255, 255, 255, 0.3);
}

.volume-control {
  display: flex;
  align-items: center;
  gap: 4px;
}

.volume-slider {
  width: 60px;
}

.volume-slider :deep(.el-slider__runway) {
  height: 3px;
  background: rgba(255, 255, 255, 0.3);
}

.volume-slider :deep(.el-slider__bar) {
  background: #fff;
  height: 3px;
}

.volume-slider :deep(.el-slider__button) {
  width: 10px;
  height: 10px;
  border: none;
  background: #fff;
}

/* 按钮样式覆盖 */
.playback-controls :deep(.el-button) {
  background: rgba(255, 255, 255, 0.2);
  border: none;
  color: #fff;
}

.playback-controls :deep(.el-button:hover) {
  background: rgba(255, 255, 255, 0.3);
}
</style>
