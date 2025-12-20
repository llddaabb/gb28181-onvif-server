package ai

import (
	"fmt"
	"os"
	"path/filepath"

	"gb28181-onvif-server/internal/debug"
)

// DetectorType 检测器类型
type DetectorType string

const (
	DetectorTypeHTTP     DetectorType = "http"     // HTTP API 检测器（调用外部服务）
	DetectorTypeEmbedded DetectorType = "embedded" // 嵌入式检测器（纯 Go）
	DetectorTypeONNX     DetectorType = "onnx"     // ONNX Runtime 检测器
	DetectorTypeAuto     DetectorType = "auto"     // 自动选择
)

// DetectorFactory 检测器工厂配置
type DetectorFactoryConfig struct {
	Type        DetectorType   // 检测器类型
	Config      DetectorConfig // 检测器配置
	APIEndpoint string         // HTTP API 端点（仅用于 HTTP 类型）
}

// CreateDetector 创建检测器
func CreateDetector(factoryConfig DetectorFactoryConfig) (Detector, error) {
	switch factoryConfig.Type {
	case DetectorTypeHTTP:
		return createHTTPDetector(factoryConfig)

	case DetectorTypeEmbedded:
		return createEmbeddedDetector(factoryConfig)

	case DetectorTypeONNX:
		return createONNXDetector(factoryConfig)

	case DetectorTypeAuto:
		return createAutoDetector(factoryConfig)

	default:
		return nil, fmt.Errorf("未知的检测器类型: %s", factoryConfig.Type)
	}
}

// createHTTPDetector 创建 HTTP 检测器
func createHTTPDetector(cfg DetectorFactoryConfig) (Detector, error) {
	debug.Info("ai", "创建 HTTP API 检测器: endpoint=%s", cfg.APIEndpoint)
	return NewHTTPDetector(cfg.Config, cfg.APIEndpoint)
}

// createEmbeddedDetector 创建嵌入式检测器
func createEmbeddedDetector(cfg DetectorFactoryConfig) (Detector, error) {
	debug.Info("ai", "创建嵌入式检测器 (纯 Go)")
	return NewEmbeddedDetector(cfg.Config)
}

// createONNXDetector 创建 ONNX Runtime 检测器
func createONNXDetector(cfg DetectorFactoryConfig) (Detector, error) {
	// 检查模型文件是否存在
	if cfg.Config.ModelPath == "" {
		cfg.Config.ModelPath = "./models/yolov8n.onnx"
	}

	modelPath := cfg.Config.ModelPath
	if !filepath.IsAbs(modelPath) {
		if absPath, err := filepath.Abs(modelPath); err == nil {
			modelPath = absPath
		}
	}

	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		debug.Warn("ai", "模型文件不存在: %s，回退到嵌入式检测器", modelPath)
		return createEmbeddedDetector(cfg)
	}

	debug.Info("ai", "创建 ONNX Runtime 检测器: model=%s", modelPath)
	cfg.Config.ModelPath = modelPath
	return NewONNXRuntimeDetector(cfg.Config)
}

// createAutoDetector 自动选择检测器
func createAutoDetector(cfg DetectorFactoryConfig) (Detector, error) {
	debug.Info("ai", "自动选择检测器...")

	// 1. 优先尝试 ONNX Runtime（如果模型存在）
	modelPaths := []string{
		cfg.Config.ModelPath,
		"./models/yolov8n.onnx",
		"./models/yolov8s.onnx",
		"third-party/zlm/bin/models/yolov8s.onnx",
	}

	for _, path := range modelPaths {
		if path == "" {
			continue
		}

		absPath := path
		if !filepath.IsAbs(path) {
			if ap, err := filepath.Abs(path); err == nil {
				absPath = ap
			}
		}

		if _, err := os.Stat(absPath); err == nil {
			debug.Info("ai", "找到模型文件: %s", absPath)
			cfg.Config.ModelPath = absPath
			detector, err := NewONNXRuntimeDetector(cfg.Config)
			if err == nil {
				return detector, nil
			}
			debug.Warn("ai", "创建 ONNX 检测器失败: %v", err)
		}
	}

	// 2. 尝试 HTTP API（如果配置了端点）
	if cfg.APIEndpoint != "" {
		debug.Info("ai", "尝试使用 HTTP API 检测器")
		detector, err := NewHTTPDetector(cfg.Config, cfg.APIEndpoint)
		if err == nil {
			return detector, nil
		}
		debug.Warn("ai", "创建 HTTP 检测器失败: %v", err)
	}

	// 3. 回退到嵌入式检测器
	debug.Info("ai", "回退到嵌入式检测器")
	return NewEmbeddedDetector(cfg.Config)
}

// DetectorInfo 检测器信息
type DetectorInfo struct {
	Type     DetectorType `json:"type"`
	Name     string       `json:"name"`
	Backend  string       `json:"backend"`
	Model    string       `json:"model"`
	Status   string       `json:"status"`
	Features []string     `json:"features"`
}

// GetDetectorInfo 获取检测器信息
func GetDetectorInfo(detector Detector) DetectorInfo {
	info := detector.GetModelInfo()

	detectorInfo := DetectorInfo{
		Name:    info.Name,
		Backend: info.Backend,
		Status:  "running",
	}

	// 根据名称确定类型
	switch info.Name {
	case "HTTP-AI-Service":
		detectorInfo.Type = DetectorTypeHTTP
		detectorInfo.Features = []string{"remote", "scalable", "full-model"}
	case "Embedded-YOLOv8":
		detectorInfo.Type = DetectorTypeEmbedded
		detectorInfo.Features = []string{"offline", "no-dependency", "lightweight"}
	case "ONNX-YOLOv8":
		detectorInfo.Type = DetectorTypeONNX
		detectorInfo.Features = []string{"offline", "full-model", "hardware-acceleration"}
	case "Fallback-Detector":
		detectorInfo.Type = DetectorTypeEmbedded
		detectorInfo.Features = []string{"fallback", "basic-detection"}
	default:
		detectorInfo.Type = DetectorTypeAuto
	}

	return detectorInfo
}

// ListAvailableDetectors 列出可用的检测器
func ListAvailableDetectors() []DetectorInfo {
	var detectors []DetectorInfo

	// HTTP 检测器总是可用（只要有网络）
	detectors = append(detectors, DetectorInfo{
		Type:     DetectorTypeHTTP,
		Name:     "HTTP API 检测器",
		Backend:  "remote",
		Status:   "available",
		Features: []string{"remote", "scalable", "full-model"},
	})

	// 嵌入式检测器总是可用
	detectors = append(detectors, DetectorInfo{
		Type:     DetectorTypeEmbedded,
		Name:     "嵌入式检测器",
		Backend:  "go-native",
		Status:   "available",
		Features: []string{"offline", "no-dependency", "lightweight"},
	})

	// 检查 ONNX 模型是否存在
	modelPaths := []string{
		"./models/yolov8n.onnx",
		"./models/yolov8s.onnx",
		"third-party/zlm/bin/models/yolov8s.onnx",
	}

	for _, path := range modelPaths {
		if _, err := os.Stat(path); err == nil {
			detectors = append(detectors, DetectorInfo{
				Type:     DetectorTypeONNX,
				Name:     "ONNX Runtime 检测器",
				Backend:  "onnxruntime",
				Model:    path,
				Status:   "available",
				Features: []string{"offline", "full-model", "hardware-acceleration"},
			})
			break
		}
	}

	return detectors
}
