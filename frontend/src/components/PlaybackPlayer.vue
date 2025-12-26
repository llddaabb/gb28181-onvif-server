<template>
  <div class="playback-player-root" ref="rootRef" :class="{ 'fullscreen': isFullscreen }">
    <div class="video-player-wrapper" v-loading="loading" :class="{ 'fullscreen': isFullscreen }">
      
      <div :id="containerId" ref="playerContainer" class="video-player-container"></div>
      
      <div v-if="error" class="video-error">
        <el-icon size="48"><VideoCamera /></el-icon>
        <p>{{ error }}</p>
        <el-button type="primary" @click="retry">重试</el-button>
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
import { VideoCamera } from '@element-plus/icons-vue'

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
})

const emit = defineEmits(['playing', 'paused', 'ended', 'error', 'timeupdate', 'fullscreenChange'])

const playerContainer = ref<HTMLElement | null>(null)
const rootRef = ref<HTMLElement | null>(null)
const containerId = `playback-container-${Math.random().toString(36).slice(2,9)}`
const loading = ref(false)
const error = ref('')
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


// 判断是否使用原生 video 元素（MP4 格式）
const useNativeVideo = computed(() => {
  const url = effectivePlayUrl.value
  return url && (url.endsWith('.mp4') || url.includes('.mp4?') || !url.includes('.flv'))
})

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
  
  // 事件监听
  video.onloadedmetadata = () => {
    duration.value = video.duration || props.videoDuration || 0
    console.log('[PlaybackPlayer] Video duration:', duration.value)
  }
  
  video.ontimeupdate = () => {
    currentProgress.value = video.currentTime
    emit('timeupdate', video.currentTime)
  }
  
  video.onplay = () => {
    isPaused.value = false
    emit('playing')
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
    console.error('[PlaybackPlayer] Video error:', e)
    error.value = '视频加载失败，可能是编码格式不支持'
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
 * 使用 Jessibuca 播放 FLV 流
 */
const initJessibucaPlayer = async (container: HTMLElement, url: string) => {
  const JessibucaClass = await getJessibuca()
  
  const player = new JessibucaClass({
    container: container,
    videoBuffer: 1,      // 增加缓冲区，给设备端录像更多时间
    isResize: true,
    text: '',
    loadingText: '正在连接设备...',
    debug: false,
    showBandwidth: false,
    operateBtns: {
      fullscreen: false,
      screenshot: false,
      play: false,
      audio: false,
    },
    forceNoOffscreen: true,
    isNotMute: !isMuted.value,
    hasAudio: true,
    // 关键配置：decoder 和 wasm 路径
    decoder: '/jessibuca/decoder.js',
    wasmPath: '/jessibuca/decoder.wasm',
    useMSE: false,  // 关闭 MSE，使用 HTTP FLV
    useWCS: false,  // 关闭 WebCodec
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
  
  player.on('pause', () => {
    isPaused.value = true
    emit('paused')
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
      error.value = '播放出错: ' + (e?.message || e || '未知错误')
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
      error.value = '播放超时，设备可能未推流或网络问题'
      emit('error', new Error('timeout'))
    }
  })
  
  // 开始播放
  await player.play(url)
  playerInstance = player
  
  // FLV 流通常是直播/实时，时长需要从外部获取
  duration.value = props.videoDuration || 0
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
}

/**
 * 重试播放
 */
const retry = () => {
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
  }
}

/**
 * 快进 10 秒
 */
const seekForward = () => {
  if (nativeVideoElement) {
    nativeVideoElement.currentTime = Math.min(duration.value, nativeVideoElement.currentTime + 10)
  }
}

/**
 * 改变播放速度
 */
const changeSpeed = (speed: number) => {
  playbackSpeed.value = speed
  if (nativeVideoElement) {
    nativeVideoElement.playbackRate = speed
  }
}

/**
 * 切换静音
 */
const toggleMute = () => {
  isMuted.value = !isMuted.value
  if (nativeVideoElement) {
    nativeVideoElement.muted = isMuted.value
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
}

.video-error p {
  margin: 10px 0;
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
