package ai

import (
	"context"
	"fmt"
	"image"
	"sync"
	"time"

	"gb28181-onvif-server/internal/debug"
)

// RecordingMode AI录像模式
type RecordingMode string

const (
	RecordingModeManual     RecordingMode = "manual"     // 手动录像
	RecordingModeMotion     RecordingMode = "motion"     // 移动检测录像
	RecordingModePerson     RecordingMode = "person"     // 人形检测录像
	RecordingModeContinuous RecordingMode = "continuous" // 连续录像
)

// StreamRecorder 流录像控制器
type StreamRecorder struct {
	channelID     string
	streamURL     string
	mode          RecordingMode
	detector      *DetectorPool
	frameGrabber  *FrameGrabber
	recordControl RecordControlFunc

	// 检测参数
	detectInterval time.Duration // 检测间隔
	recordDelay    time.Duration // 录像延迟（检测到人后继续录多久）
	minRecordTime  time.Duration // 最小录像时长

	// 状态
	isRecording     bool
	lastDetectTime  time.Time
	lastPersonTime  time.Time
	recordStartTime time.Time

	// 控制
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.RWMutex

	// 统计
	stats RecordingStats
}

// RecordingStats 录像统计
type RecordingStats struct {
	TotalDetections   int64         // 总检测次数
	PersonDetections  int64         // 检测到人的次数
	RecordingSessions int64         // 录像会话数
	TotalRecordTime   time.Duration // 总录像时长
	LastDetectTime    time.Time     // 最后检测时间
	LastPersonTime    time.Time     // 最后检测到人的时间
}

// RecordControlFunc 录像控制函数
type RecordControlFunc func(channelID string, start bool) error

// RecorderConfig 录像器配置
type RecorderConfig struct {
	ChannelID      string         `yaml:"ChannelID"`
	StreamURL      string         `yaml:"StreamURL"` // 流地址
	Mode           RecordingMode  `yaml:"Mode"`
	DetectInterval time.Duration  `yaml:"DetectInterval"` // 检测间隔(秒)
	RecordDelay    time.Duration  `yaml:"RecordDelay"`    // 录像延迟(秒)
	MinRecordTime  time.Duration  `yaml:"MinRecordTime"`  // 最小录像时长(秒)
	DetectorConfig DetectorConfig `yaml:"DetectorConfig"`
	FFmpegBin      string         `yaml:"FFmpegBin"` // FFmpeg路径
}

// DefaultRecorderConfig 默认录像器配置
func DefaultRecorderConfig(channelID string) RecorderConfig {
	return RecorderConfig{
		ChannelID:      channelID,
		Mode:           RecordingModePerson,
		DetectInterval: 2 * time.Second,  // 每2秒检测一次
		RecordDelay:    10 * time.Second, // 检测到人后继续录10秒
		MinRecordTime:  5 * time.Second,  // 最小录像5秒
		DetectorConfig: DefaultDetectorConfig(),
	}
}

// NewStreamRecorder 创建流录像控制器
func NewStreamRecorder(config RecorderConfig, recordControl RecordControlFunc) (*StreamRecorder, error) {
	// 创建检测器池（使用单个检测器以节省资源）
	pool, err := NewDetectorPool(config.DetectorConfig, 1)
	if err != nil {
		return nil, fmt.Errorf("创建检测器池失败: %w", err)
	}

	// 创建帧捕获器
	frameGrabber := NewFrameGrabber(config.StreamURL, config.FFmpegBin)

	ctx, cancel := context.WithCancel(context.Background())

	recorder := &StreamRecorder{
		channelID:      config.ChannelID,
		streamURL:      config.StreamURL,
		mode:           config.Mode,
		detector:       pool,
		frameGrabber:   frameGrabber,
		recordControl:  recordControl,
		detectInterval: config.DetectInterval,
		recordDelay:    config.RecordDelay,
		minRecordTime:  config.MinRecordTime,
		ctx:            ctx,
		cancel:         cancel,
	}

	return recorder, nil
}

// Start 启动录像控制器
func (r *StreamRecorder) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	debug.Info("ai", "启动AI录像控制器: channelID=%s, mode=%s", r.channelID, r.mode)

	// 启动检测循环
	r.wg.Add(1)
	go r.detectionLoop()

	return nil
}

// Stop 停止录像控制器
func (r *StreamRecorder) Stop() error {
	r.cancel()
	r.wg.Wait()

	// 停止录像
	r.mu.Lock()
	if r.isRecording {
		r.stopRecording()
	}
	r.mu.Unlock()

	// 关闭检测器
	if err := r.detector.Close(); err != nil {
		debug.Error("ai", "关闭检测器失败: %v", err)
	}

	debug.Info("ai", "AI录像控制器已停止: channelID=%s", r.channelID)
	return nil
}

// detectionLoop 检测循环
func (r *StreamRecorder) detectionLoop() {
	defer r.wg.Done()

	ticker := time.NewTicker(r.detectInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.performDetection()
		}
	}
}

// performDetection 执行检测
func (r *StreamRecorder) performDetection() {
	// 获取当前帧（需要从流中抓取）
	frame, err := r.captureFrame()
	if err != nil {
		debug.Error("ai", "捕获帧失败: %v", err)
		return
	}

	// 获取检测器
	detectCtx, cancel := context.WithTimeout(r.ctx, 5*time.Second)
	detectCtx = context.WithValue(detectCtx, "timestamp", time.Now())
	defer cancel()

	detector, err := r.detector.Get(detectCtx)
	if err != nil {
		debug.Error("ai", "获取检测器失败: %v", err)
		return
	}
	defer r.detector.Put(detector)

	// 执行检测
	result, err := detector.Detect(detectCtx, frame)
	if err != nil {
		debug.Error("ai", "检测失败: %v", err)
		return
	}

	// 更新统计
	r.mu.Lock()
	r.stats.TotalDetections++
	r.stats.LastDetectTime = time.Now()

	if result.HasPerson {
		r.stats.PersonDetections++
		r.stats.LastPersonTime = time.Now()
		r.lastPersonTime = time.Now()

		debug.Info("ai", "检测到 %d 个人: channelID=%s, confidence=%.2f",
			result.PersonCount, r.channelID, result.Confidence)
	}

	// 判断是否需要录像
	shouldRecord := r.shouldRecord(result)
	currentlyRecording := r.isRecording

	r.mu.Unlock()

	// 控制录像
	if shouldRecord && !currentlyRecording {
		r.startRecording()
	} else if !shouldRecord && currentlyRecording {
		// 检查是否应该停止（需要考虑延迟和最小录像时长）
		if r.shouldStopRecording() {
			r.stopRecording()
		}
	}
}

// shouldRecord 判断是否应该录像
func (r *StreamRecorder) shouldRecord(result *DetectionResult) bool {
	switch r.mode {
	case RecordingModeManual:
		return false // 手动模式不自动录像
	case RecordingModeContinuous:
		return true // 连续录像
	case RecordingModePerson:
		return result.HasPerson // 检测到人就录像
	case RecordingModeMotion:
		// TODO: 实现移动检测
		return false
	default:
		return false
	}
}

// shouldStopRecording 判断是否应该停止录像
func (r *StreamRecorder) shouldStopRecording() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	now := time.Now()

	// 检查最小录像时长
	if now.Sub(r.recordStartTime) < r.minRecordTime {
		return false
	}

	// 检查延迟时间
	if now.Sub(r.lastPersonTime) < r.recordDelay {
		return false
	}

	return true
}

// startRecording 开始录像
func (r *StreamRecorder) startRecording() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.isRecording {
		return
	}

	if err := r.recordControl(r.channelID, true); err != nil {
		debug.Error("ai", "启动录像失败: %v", err)
		return
	}

	r.isRecording = true
	r.recordStartTime = time.Now()
	r.stats.RecordingSessions++

	debug.Info("ai", "AI录像已启动: channelID=%s", r.channelID)
}

// stopRecording 停止录像
func (r *StreamRecorder) stopRecording() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.isRecording {
		return
	}

	if err := r.recordControl(r.channelID, false); err != nil {
		debug.Error("ai", "停止录像失败: %v", err)
		return
	}

	recordDuration := time.Since(r.recordStartTime)
	r.stats.TotalRecordTime += recordDuration
	r.isRecording = false

	debug.Info("ai", "AI录像已停止: channelID=%s, duration=%v", r.channelID, recordDuration)
}

// captureFrame 捕获当前帧（从流中抓取）
func (r *StreamRecorder) captureFrame() (image.Image, error) {
	// 使用FrameGrabber捕获帧（已缩放到检测模型输入大小）
	ctx, cancel := context.WithTimeout(r.ctx, 10*time.Second)
	defer cancel()

	img, err := r.frameGrabber.CaptureFrameScaled(ctx, 320, 320)
	if err != nil {
		return nil, fmt.Errorf("捕获帧失败: %w", err)
	}

	return img, nil
}

// GetStats 获取统计信息
func (r *StreamRecorder) GetStats() RecordingStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.stats
}

// GetStatus 获取当前状态
func (r *StreamRecorder) GetStatus() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return map[string]interface{}{
		"channel_id":       r.channelID,
		"mode":             r.mode,
		"is_recording":     r.isRecording,
		"last_detect_time": r.stats.LastDetectTime,
		"last_person_time": r.stats.LastPersonTime,
		"stats":            r.stats,
	}
}
