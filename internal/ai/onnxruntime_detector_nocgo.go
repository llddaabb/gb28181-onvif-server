//go:build !cgo
// +build !cgo

package ai

import (
	"context"
	"image"
	"math"
	"sort"
	"sync"
	"time"

	"gb28181-onvif-server/internal/debug"
)

// ONNXRuntimeDetector 纯 Go 实现的检测器（非 CGO 版本）
// 使用简化的人体检测算法，在没有 ONNX Runtime 时作为备用
type ONNXRuntimeDetector struct {
	config    DetectorConfig
	modelInfo ModelInfo
	mu        sync.RWMutex

	inputWidth  int
	inputHeight int
}

// NewONNXRuntimeDetector 创建检测器
func NewONNXRuntimeDetector(config DetectorConfig) (*ONNXRuntimeDetector, error) {
	detector := &ONNXRuntimeDetector{
		config: config,
		modelInfo: ModelInfo{
			Name:         "Fallback-Detector",
			Backend:      "go-native",
			InputSize:    config.InputSize,
			Confidence:   config.Confidence,
			IoUThreshold: config.IoUThreshold,
		},
		inputWidth:  config.InputSize,
		inputHeight: config.InputSize,
	}

	debug.Warn("ai", "CGO 未启用，使用纯 Go 备用检测器（功能受限）")
	debug.Info("ai", "如需完整 ONNX 推理，请使用 CGO_ENABLED=1 编译")

	return detector, nil
}

// Detect 检测图像
func (d *ONNXRuntimeDetector) Detect(ctx context.Context, img image.Image) (*DetectionResult, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	startTime := time.Now()

	// 使用简化的检测算法
	boxes := d.simpleDetect(img)

	// 统计结果
	personCount := len(boxes)
	var maxConfidence float32 = 0
	for _, box := range boxes {
		if box.Confidence > maxConfidence {
			maxConfidence = box.Confidence
		}
	}

	result := &DetectionResult{
		HasPerson:   personCount > 0,
		PersonCount: personCount,
		Confidence:  maxConfidence,
		Boxes:       boxes,
		Timestamp:   time.Now(),
	}

	debug.Debug("ai", "备用检测完成: hasPerson=%v, count=%d, time=%v",
		result.HasPerson, result.PersonCount, time.Since(startTime))

	return result, nil
}

// simpleDetect 简化检测（基于颜色特征）
func (d *ONNXRuntimeDetector) simpleDetect(img image.Image) []BBox {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// 分析图像中的肤色区域
	skinMap := make([][]bool, height)
	for y := 0; y < height; y++ {
		skinMap[y] = make([]bool, width)
		for x := 0; x < width; x++ {
			r, g, b, _ := img.At(x+bounds.Min.X, y+bounds.Min.Y).RGBA()
			skinMap[y][x] = isSkinPixel(float32(r>>8), float32(g>>8), float32(b>>8))
		}
	}

	// 连通域分析找到可能的人体区域
	regions := findConnectedRegions(skinMap, width, height)

	// 转换为检测框
	var boxes []BBox
	for _, region := range regions {
		// 过滤太小或太大的区域
		regionWidth := region.maxX - region.minX
		regionHeight := region.maxY - region.minY
		area := regionWidth * regionHeight

		// 人体区域的面积应该在合理范围内
		imageArea := width * height
		if float64(area) < 0.01*float64(imageArea) || float64(area) > 0.8*float64(imageArea) {
			continue
		}

		// 检查长宽比（人体通常是竖直的）
		aspectRatio := float64(regionHeight) / float64(regionWidth)
		if aspectRatio < 0.5 || aspectRatio > 4.0 {
			continue
		}

		// 计算置信度（基于区域大小和肤色像素密度）
		density := float32(region.pixelCount) / float32(area)
		confidence := density * 0.8 // 最高0.8的置信度

		if confidence >= d.config.Confidence {
			boxes = append(boxes, BBox{
				X1:         float32(region.minX),
				Y1:         float32(region.minY),
				X2:         float32(region.maxX),
				Y2:         float32(region.maxY),
				Confidence: confidence,
				Class:      "person",
			})
		}
	}

	// NMS
	boxes = nmsBoxes(boxes, d.config.IoUThreshold)

	return boxes
}

// GetModelInfo 获取模型信息
func (d *ONNXRuntimeDetector) GetModelInfo() ModelInfo {
	return d.modelInfo
}

// Close 关闭检测器
func (d *ONNXRuntimeDetector) Close() error {
	return nil
}

// 辅助结构和函数

type skinRegion struct {
	minX, minY, maxX, maxY int
	pixelCount             int
}

func isSkinPixel(r, g, b float32) bool {
	// YCbCr 肤色检测
	cb := 128 - 0.169*r - 0.331*g + 0.5*b
	cr := 128 + 0.5*r - 0.419*g - 0.081*b
	return cb >= 77 && cb <= 127 && cr >= 133 && cr <= 173
}

func findConnectedRegions(skinMap [][]bool, width, height int) []skinRegion {
	visited := make([][]bool, height)
	for y := 0; y < height; y++ {
		visited[y] = make([]bool, width)
	}

	var regions []skinRegion

	// 使用洪水填充找连通域
	var floodFill func(x, y int, region *skinRegion)
	floodFill = func(x, y int, region *skinRegion) {
		if x < 0 || x >= width || y < 0 || y >= height {
			return
		}
		if visited[y][x] || !skinMap[y][x] {
			return
		}

		visited[y][x] = true
		region.pixelCount++

		if x < region.minX {
			region.minX = x
		}
		if x > region.maxX {
			region.maxX = x
		}
		if y < region.minY {
			region.minY = y
		}
		if y > region.maxY {
			region.maxY = y
		}

		// 8连通
		floodFill(x-1, y, region)
		floodFill(x+1, y, region)
		floodFill(x, y-1, region)
		floodFill(x, y+1, region)
	}

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if skinMap[y][x] && !visited[y][x] {
				region := skinRegion{
					minX: x, maxX: x,
					minY: y, maxY: y,
				}
				floodFill(x, y, &region)

				// 只保留有一定大小的区域
				if region.pixelCount > 100 {
					regions = append(regions, region)
				}
			}
		}
	}

	return regions
}

func nmsBoxes(boxes []BBox, iouThreshold float32) []BBox {
	if len(boxes) == 0 {
		return boxes
	}

	sort.Slice(boxes, func(i, j int) bool {
		return boxes[i].Confidence > boxes[j].Confidence
	})

	var result []BBox
	used := make([]bool, len(boxes))

	for i := 0; i < len(boxes); i++ {
		if used[i] {
			continue
		}
		result = append(result, boxes[i])
		used[i] = true

		for j := i + 1; j < len(boxes); j++ {
			if used[j] {
				continue
			}
			if boxIoU(boxes[i], boxes[j]) > iouThreshold {
				used[j] = true
			}
		}
	}

	return result
}

func boxIoU(a, b BBox) float32 {
	x1 := math.Max(float64(a.X1), float64(b.X1))
	y1 := math.Max(float64(a.Y1), float64(b.Y1))
	x2 := math.Min(float64(a.X2), float64(b.X2))
	y2 := math.Min(float64(a.Y2), float64(b.Y2))

	if x2 <= x1 || y2 <= y1 {
		return 0
	}

	intersection := (x2 - x1) * (y2 - y1)
	areaA := float64((a.X2 - a.X1) * (a.Y2 - a.Y1))
	areaB := float64((b.X2 - b.X1) * (b.Y2 - b.Y1))

	return float32(intersection / (areaA + areaB - intersection))
}
