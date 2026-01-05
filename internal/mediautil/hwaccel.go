package mediautil

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// HWAccelType 硬件加速类型
type HWAccelType string

const (
	HWAccelNone         HWAccelType = "none"
	HWAccelCUDA         HWAccelType = "cuda"
	HWAccelQSV          HWAccelType = "qsv"
	HWAccelVAAPI        HWAccelType = "vaapi"
	HWAccelVideoToolbox HWAccelType = "videotoolbox"
)

// HWAccelDetector 硬件加速检测器
type HWAccelDetector struct {
	mu              sync.RWMutex
	availableAccels []HWAccelType
	bestAccel       HWAccelType
	detected        bool
	lastCheck       time.Time
}

// NewHWAccelDetector 创建硬件加速检测器
func NewHWAccelDetector() *HWAccelDetector {
	return &HWAccelDetector{
		availableAccels: []HWAccelType{},
		bestAccel:       HWAccelNone,
		detected:        false,
	}
}

// Detect 检测可用的硬件加速
func (d *HWAccelDetector) Detect() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	log.Println("[硬件加速] 开始检测可用的硬件加速...")

	// 检查 ffmpeg 是否可用
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		log.Printf("[硬件加速] ffmpeg 不可用: %v", err)
		return fmt.Errorf("ffmpeg not found: %w", err)
	}

	// 获取 ffmpeg 支持的硬件加速列表
	cmd := exec.Command("ffmpeg", "-hwaccels")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		log.Printf("[硬件加速] 执行 ffmpeg -hwaccels 失败: %v", err)
		return fmt.Errorf("failed to detect hwaccels: %w", err)
	}

	output := out.String()
	log.Printf("[硬件加速] ffmpeg -hwaccels 输出:\n%s", output)

	// 解析输出
	lines := strings.Split(output, "\n")
	availableTypes := make(map[HWAccelType]bool)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Hardware") {
			continue
		}

		// 检查各种硬件加速类型
		switch {
		case strings.Contains(line, "cuda"):
			availableTypes[HWAccelCUDA] = true
		case strings.Contains(line, "qsv"):
			availableTypes[HWAccelQSV] = true
		case strings.Contains(line, "vaapi"):
			availableTypes[HWAccelVAAPI] = true
		case strings.Contains(line, "videotoolbox"):
			availableTypes[HWAccelVideoToolbox] = true
		}
	}

	// 测试实际可用性（按优先级顺序）
	testOrder := []HWAccelType{HWAccelCUDA, HWAccelQSV, HWAccelVAAPI, HWAccelVideoToolbox}
	d.availableAccels = []HWAccelType{}

	for _, accelType := range testOrder {
		if availableTypes[accelType] {
			if d.testHWAccel(accelType) {
				d.availableAccels = append(d.availableAccels, accelType)
				log.Printf("[硬件加速] ✓ %s 可用", accelType)
			} else {
				log.Printf("[硬件加速] ✗ %s 列出但测试失败", accelType)
			}
		}
	}

	// 设置最佳加速类型
	if len(d.availableAccels) > 0 {
		d.bestAccel = d.availableAccels[0]
		log.Printf("[硬件加速] 选择最佳加速: %s", d.bestAccel)
	} else {
		d.bestAccel = HWAccelNone
		log.Println("[硬件加速] 无可用硬件加速，将使用软件解码")
	}

	d.detected = true
	d.lastCheck = time.Now()

	return nil
}

// testHWAccel 测试硬件加速是否真正可用
func (d *HWAccelDetector) testHWAccel(accelType HWAccelType) bool {
	// 使用 testsrc 测试硬件加速
	args := []string{
		"-f", "lavfi",
		"-i", "testsrc=duration=1:size=640x480:rate=1",
		"-hwaccel", string(accelType),
		"-f", "null",
		"-",
	}

	cmd := exec.Command("ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	// 设置超时
	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	select {
	case err := <-done:
		if err != nil {
			log.Printf("[硬件加速] 测试 %s 失败: %v, stderr: %s", accelType, err, stderr.String())
			return false
		}
		return true
	case <-time.After(5 * time.Second):
		cmd.Process.Kill()
		log.Printf("[硬件加速] 测试 %s 超时", accelType)
		return false
	}
}

// GetBestAccel 获取最佳硬件加速类型
func (d *HWAccelDetector) GetBestAccel() HWAccelType {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// 如果未检测过，先检测
	if !d.detected {
		d.mu.RUnlock()
		d.Detect()
		d.mu.RLock()
	}

	return d.bestAccel
}

// GetAvailableAccels 获取所有可用的硬件加速类型
func (d *HWAccelDetector) GetAvailableAccels() []HWAccelType {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if !d.detected {
		d.mu.RUnlock()
		d.Detect()
		d.mu.RLock()
	}

	result := make([]HWAccelType, len(d.availableAccels))
	copy(result, d.availableAccels)
	return result
}

// IsDetected 是否已检测
func (d *HWAccelDetector) IsDetected() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.detected
}

// GetFFmpegHWAccelArgs 获取 ffmpeg 硬件加速参数
func (d *HWAccelDetector) GetFFmpegHWAccelArgs() []string {
	accel := d.GetBestAccel()
	if accel == HWAccelNone {
		return []string{}
	}

	args := []string{"-hwaccel", string(accel)}

	// 针对不同的硬件加速添加特定参数
	switch accel {
	case HWAccelCUDA:
		// CUDA 加速需要指定输出格式
		args = append(args, "-hwaccel_output_format", "cuda")
	case HWAccelQSV:
		// QSV 加速
		args = append(args, "-hwaccel_output_format", "qsv")
	case HWAccelVAAPI:
		// VAAPI 加速（Linux）
		args = append(args, "-hwaccel_device", "/dev/dri/renderD128")
		args = append(args, "-hwaccel_output_format", "vaapi")
	case HWAccelVideoToolbox:
		// macOS VideoToolbox
		args = append(args, "-hwaccel_output_format", "videotoolbox")
	}

	return args
}

// GetDecoderName 获取特定编码格式的硬件解码器名称
func (d *HWAccelDetector) GetDecoderName(codec string) string {
	accel := d.GetBestAccel()
	if accel == HWAccelNone {
		return "" // 使用默认解码器
	}

	// 根据硬件加速类型和编码格式返回相应的解码器
	switch accel {
	case HWAccelCUDA:
		switch codec {
		case "h264":
			return "h264_cuvid"
		case "hevc", "h265":
			return "hevc_cuvid"
		case "vp9":
			return "vp9_cuvid"
		}
	case HWAccelQSV:
		switch codec {
		case "h264":
			return "h264_qsv"
		case "hevc", "h265":
			return "hevc_qsv"
		case "vp9":
			return "vp9_qsv"
		}
	case HWAccelVAAPI:
		// VAAPI 通常使用通用解码器 + hwaccel
		return ""
	case HWAccelVideoToolbox:
		// VideoToolbox 通常使用通用解码器 + hwaccel
		return ""
	}

	return ""
}

// 全局硬件加速检测器实例
var globalDetector *HWAccelDetector
var once sync.Once

// GetGlobalDetector 获取全局硬件加速检测器
func GetGlobalDetector() *HWAccelDetector {
	once.Do(func() {
		globalDetector = NewHWAccelDetector()
		// 异步检测，不阻塞启动
		go func() {
			if err := globalDetector.Detect(); err != nil {
				log.Printf("[硬件加速] 检测失败: %v", err)
			}
		}()
	})
	return globalDetector
}
