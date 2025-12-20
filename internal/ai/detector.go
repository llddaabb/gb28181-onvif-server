package ai

import (
	"context"
	"fmt"
	"image"
	"sync"
	"time"
)

// DetectionResult AI检测结果
type DetectionResult struct {
	HasPerson   bool      // 是否检测到人
	PersonCount int       // 人数
	Confidence  float32   // 置信度
	Boxes       []BBox    // 检测框
	Timestamp   time.Time // 检测时间
}

// BBox 边界框
type BBox struct {
	X1, Y1, X2, Y2 float32 // 坐标
	Confidence     float32 // 置信度
	Class          string  // 类别
}

// Detector AI检测器接口
type Detector interface {
	// Detect 检测图像中是否有人
	Detect(ctx context.Context, img image.Image) (*DetectionResult, error)

	// GetModelInfo 获取模型信息
	GetModelInfo() ModelInfo

	// Close 关闭检测器
	Close() error
}

// ModelInfo 模型信息
type ModelInfo struct {
	Name         string  // 模型名称
	Backend      string  // 后端: cpu, cuda, opencl
	InputSize    int     // 输入尺寸
	Confidence   float32 // 置信度阈值
	IoUThreshold float32 // NMS IoU阈值
}

// DetectorConfig 检测器配置
type DetectorConfig struct {
	ModelPath    string  `yaml:"ModelPath"`    // 模型文件路径
	Backend      string  `yaml:"Backend"`      // cpu, cuda, opencl, auto
	InputSize    int     `yaml:"InputSize"`    // 输入图像大小 (320, 640, 1280)
	Confidence   float32 `yaml:"Confidence"`   // 置信度阈值 (0.0-1.0)
	IoUThreshold float32 `yaml:"IoUThreshold"` // NMS IoU阈值
	MaxBatchSize int     `yaml:"MaxBatchSize"` // 最大批处理大小
	NumThreads   int     `yaml:"NumThreads"`   // CPU线程数 (0=自动)
}

// DefaultDetectorConfig 默认检测器配置
func DefaultDetectorConfig() DetectorConfig {
	return DetectorConfig{
		ModelPath:    "./models/yolov8n.onnx", // 使用nano模型，最快最省资源
		Backend:      "auto",                  // 自动选择最优后端
		InputSize:    320,                     // 使用最小输入尺寸，降低计算量
		Confidence:   0.5,                     // 中等置信度
		IoUThreshold: 0.45,                    // 标准NMS阈值
		MaxBatchSize: 1,                       // 不使用批处理
		NumThreads:   2,                       // 限制CPU线程数
	}
}

// DetectorPool 检测器池
type DetectorPool struct {
	detectors []Detector
	pool      chan Detector
	config    DetectorConfig
	mu        sync.RWMutex

	// 工厂配置
	factoryConfig DetectorFactoryConfig
}

// NewDetectorPool 创建检测器池
func NewDetectorPool(config DetectorConfig, poolSize int) (*DetectorPool, error) {
	// 使用默认的自动选择类型
	return NewDetectorPoolWithFactory(DetectorFactoryConfig{
		Type:   DetectorTypeAuto,
		Config: config,
	}, poolSize)
}

// NewDetectorPoolWithFactory 使用工厂配置创建检测器池
func NewDetectorPoolWithFactory(factoryConfig DetectorFactoryConfig, poolSize int) (*DetectorPool, error) {
	if poolSize <= 0 {
		poolSize = 1
	}

	pool := &DetectorPool{
		detectors:     make([]Detector, 0, poolSize),
		pool:          make(chan Detector, poolSize),
		config:        factoryConfig.Config,
		factoryConfig: factoryConfig,
	}

	// 创建检测器实例
	for i := 0; i < poolSize; i++ {
		detector, err := CreateDetector(factoryConfig)
		if err != nil {
			// 清理已创建的检测器
			pool.Close()
			return nil, fmt.Errorf("创建检测器 %d 失败: %w", i, err)
		}
		pool.detectors = append(pool.detectors, detector)
		pool.pool <- detector
	}

	return pool, nil
}

// Get 从池中获取检测器
func (p *DetectorPool) Get(ctx context.Context) (Detector, error) {
	select {
	case detector := <-p.pool:
		return detector, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Put 归还检测器到池
func (p *DetectorPool) Put(detector Detector) {
	if detector != nil {
		select {
		case p.pool <- detector:
		default:
			// 池已满，不应该发生
		}
	}
}

// Close 关闭检测器池
func (p *DetectorPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	close(p.pool)

	var lastErr error
	for _, detector := range p.detectors {
		if err := detector.Close(); err != nil {
			lastErr = err
		}
	}

	p.detectors = nil
	return lastErr
}
