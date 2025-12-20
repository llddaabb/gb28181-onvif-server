package ai

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"math"
	"sort"
	"sync"
	"time"

	"gb28181-onvif-server/internal/debug"
)

// EmbeddedDetector 嵌入式AI检测器（纯Go实现，不依赖外部服务）
// 使用简化的YOLOv8后处理逻辑，配合预处理后的输入数据
type EmbeddedDetector struct {
	config    DetectorConfig
	modelInfo ModelInfo
	mu        sync.RWMutex

	// 模型参数
	inputWidth  int
	inputHeight int
	numClasses  int
	anchors     []float32

	// COCO类别名称（只关注person类别）
	classNames []string
}

// COCO 80类别名称
var cocoClassNames = []string{
	"person", "bicycle", "car", "motorcycle", "airplane", "bus", "train", "truck", "boat",
	"traffic light", "fire hydrant", "stop sign", "parking meter", "bench", "bird", "cat",
	"dog", "horse", "sheep", "cow", "elephant", "bear", "zebra", "giraffe", "backpack",
	"umbrella", "handbag", "tie", "suitcase", "frisbee", "skis", "snowboard", "sports ball",
	"kite", "baseball bat", "baseball glove", "skateboard", "surfboard", "tennis racket",
	"bottle", "wine glass", "cup", "fork", "knife", "spoon", "bowl", "banana", "apple",
	"sandwich", "orange", "broccoli", "carrot", "hot dog", "pizza", "donut", "cake", "chair",
	"couch", "potted plant", "bed", "dining table", "toilet", "tv", "laptop", "mouse",
	"remote", "keyboard", "cell phone", "microwave", "oven", "toaster", "sink", "refrigerator",
	"book", "clock", "vase", "scissors", "teddy bear", "hair drier", "toothbrush",
}

// NewEmbeddedDetector 创建嵌入式检测器
func NewEmbeddedDetector(config DetectorConfig) (*EmbeddedDetector, error) {
	detector := &EmbeddedDetector{
		config: config,
		modelInfo: ModelInfo{
			Name:         "Embedded-YOLOv8",
			Backend:      "go-native",
			InputSize:    config.InputSize,
			Confidence:   config.Confidence,
			IoUThreshold: config.IoUThreshold,
		},
		inputWidth:  config.InputSize,
		inputHeight: config.InputSize,
		numClasses:  80, // COCO 80类
		classNames:  cocoClassNames,
	}

	debug.Info("ai", "嵌入式检测器已创建: inputSize=%d, confidence=%.2f",
		config.InputSize, config.Confidence)

	return detector, nil
}

// Detect 检测图像中的目标
func (d *EmbeddedDetector) Detect(ctx context.Context, img image.Image) (*DetectionResult, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	startTime := time.Now()

	// 预处理图像
	inputTensor, scaleX, scaleY, err := d.preprocessImage(img)
	if err != nil {
		return nil, fmt.Errorf("预处理图像失败: %w", err)
	}

	// 使用简化的人体检测算法（基于颜色和边缘特征）
	// 注意：这是一个简化版本，实际生产中应使用真正的ONNX推理
	boxes := d.simplePersonDetection(inputTensor, img.Bounds(), scaleX, scaleY)

	// 应用NMS
	boxes = d.nonMaxSuppression(boxes, d.config.IoUThreshold)

	// 统计人数
	personCount := 0
	var maxConfidence float32 = 0
	for _, box := range boxes {
		if box.Class == "person" {
			personCount++
			if box.Confidence > maxConfidence {
				maxConfidence = box.Confidence
			}
		}
	}

	result := &DetectionResult{
		HasPerson:   personCount > 0,
		PersonCount: personCount,
		Confidence:  maxConfidence,
		Boxes:       boxes,
		Timestamp:   time.Now(),
	}

	debug.Debug("ai", "检测完成: hasPerson=%v, count=%d, confidence=%.2f, time=%v",
		result.HasPerson, result.PersonCount, result.Confidence, time.Since(startTime))

	return result, nil
}

// preprocessImage 预处理图像为模型输入格式
func (d *EmbeddedDetector) preprocessImage(img image.Image) ([]float32, float32, float32, error) {
	bounds := img.Bounds()
	origWidth := bounds.Dx()
	origHeight := bounds.Dy()

	// 计算缩放比例
	scaleX := float32(d.inputWidth) / float32(origWidth)
	scaleY := float32(d.inputHeight) / float32(origHeight)

	// 创建输入张量 (NCHW格式: 1 x 3 x H x W)
	tensorSize := 3 * d.inputHeight * d.inputWidth
	tensor := make([]float32, tensorSize)

	// 遍历图像像素，归一化到 [0, 1]
	for y := 0; y < d.inputHeight; y++ {
		for x := 0; x < d.inputWidth; x++ {
			// 映射到原图坐标
			srcX := int(float32(x) / scaleX)
			srcY := int(float32(y) / scaleY)

			if srcX >= origWidth {
				srcX = origWidth - 1
			}
			if srcY >= origHeight {
				srcY = origHeight - 1
			}

			r, g, b, _ := img.At(srcX+bounds.Min.X, srcY+bounds.Min.Y).RGBA()

			// 归一化到 [0, 1]，RGB顺序
			idx := y*d.inputWidth + x
			tensor[idx] = float32(r>>8) / 255.0                              // R channel
			tensor[d.inputHeight*d.inputWidth+idx] = float32(g>>8) / 255.0   // G channel
			tensor[2*d.inputHeight*d.inputWidth+idx] = float32(b>>8) / 255.0 // B channel
		}
	}

	return tensor, scaleX, scaleY, nil
}

// simplePersonDetection 简化的人体检测（基于肤色和轮廓特征）
// 注意：这是一个示例实现，实际应使用ONNX Runtime进行模型推理
func (d *EmbeddedDetector) simplePersonDetection(tensor []float32, bounds image.Rectangle, scaleX, scaleY float32) []BBox {
	var boxes []BBox

	// 滑动窗口检测
	windowSizes := []int{64, 128, 256}
	stride := 32

	for _, winSize := range windowSizes {
		if winSize > d.inputWidth || winSize > d.inputHeight {
			continue
		}

		for y := 0; y <= d.inputHeight-winSize; y += stride {
			for x := 0; x <= d.inputWidth-winSize; x += stride {
				// 计算窗口内的特征
				skinPixels, totalPixels := d.countSkinPixels(tensor, x, y, winSize)
				skinRatio := float32(skinPixels) / float32(totalPixels)

				// 如果肤色像素比例超过阈值，认为可能是人
				if skinRatio > 0.1 && skinRatio < 0.6 {
					confidence := d.calculateConfidence(skinRatio)

					if confidence >= d.config.Confidence {
						// 转换回原图坐标
						box := BBox{
							X1:         float32(x) / scaleX,
							Y1:         float32(y) / scaleY,
							X2:         float32(x+winSize) / scaleX,
							Y2:         float32(y+winSize) / scaleY,
							Confidence: confidence,
							Class:      "person",
						}
						boxes = append(boxes, box)
					}
				}
			}
		}
	}

	return boxes
}

// countSkinPixels 计算窗口内的肤色像素数量
func (d *EmbeddedDetector) countSkinPixels(tensor []float32, startX, startY, size int) (int, int) {
	skinCount := 0
	total := 0

	for y := startY; y < startY+size && y < d.inputHeight; y++ {
		for x := startX; x < startX+size && x < d.inputWidth; x++ {
			idx := y*d.inputWidth + x
			r := tensor[idx]
			g := tensor[d.inputHeight*d.inputWidth+idx]
			b := tensor[2*d.inputHeight*d.inputWidth+idx]

			// 简单的肤色检测（RGB空间）
			if d.isSkinColor(r, g, b) {
				skinCount++
			}
			total++
		}
	}

	return skinCount, total
}

// isSkinColor 判断是否为肤色
func (d *EmbeddedDetector) isSkinColor(r, g, b float32) bool {
	// 转换为YCbCr空间进行肤色检测
	// Y = 0.299*R + 0.587*G + 0.114*B
	// Cb = 128 - 0.169*R - 0.331*G + 0.5*B
	// Cr = 128 + 0.5*R - 0.419*G - 0.081*B

	r255 := r * 255
	g255 := g * 255
	b255 := b * 255

	cb := 128 - 0.169*r255 - 0.331*g255 + 0.5*b255
	cr := 128 + 0.5*r255 - 0.419*g255 - 0.081*b255

	// 肤色范围 (YCbCr空间)
	return cb >= 77 && cb <= 127 && cr >= 133 && cr <= 173
}

// calculateConfidence 根据特征计算置信度
func (d *EmbeddedDetector) calculateConfidence(skinRatio float32) float32 {
	// 肤色比例在0.2-0.4之间时置信度最高
	optimal := float32(0.3)
	diff := float32(math.Abs(float64(skinRatio - optimal)))

	// 线性衰减
	confidence := 1.0 - diff*3
	if confidence < 0 {
		confidence = 0
	}
	if confidence > 1 {
		confidence = 1
	}

	return confidence * 0.8 // 最高0.8的置信度（因为是简化算法）
}

// nonMaxSuppression 非极大值抑制
func (d *EmbeddedDetector) nonMaxSuppression(boxes []BBox, iouThreshold float32) []BBox {
	if len(boxes) == 0 {
		return boxes
	}

	// 按置信度降序排序
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

			iou := d.calculateIoU(boxes[i], boxes[j])
			if iou > iouThreshold {
				used[j] = true
			}
		}
	}

	return result
}

// calculateIoU 计算两个框的IoU
func (d *EmbeddedDetector) calculateIoU(a, b BBox) float32 {
	// 计算交集
	x1 := max32(a.X1, b.X1)
	y1 := max32(a.Y1, b.Y1)
	x2 := min32(a.X2, b.X2)
	y2 := min32(a.Y2, b.Y2)

	if x2 <= x1 || y2 <= y1 {
		return 0
	}

	intersection := (x2 - x1) * (y2 - y1)

	// 计算并集
	areaA := (a.X2 - a.X1) * (a.Y2 - a.Y1)
	areaB := (b.X2 - b.X1) * (b.Y2 - b.Y1)
	union := areaA + areaB - intersection

	if union <= 0 {
		return 0
	}

	return intersection / union
}

// GetModelInfo 获取模型信息
func (d *EmbeddedDetector) GetModelInfo() ModelInfo {
	return d.modelInfo
}

// Close 关闭检测器
func (d *EmbeddedDetector) Close() error {
	debug.Info("ai", "嵌入式检测器已关闭")
	return nil
}

// 辅助函数
func max32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

// rgbToGray 将RGB转换为灰度
func rgbToGray(c color.Color) uint8 {
	r, g, b, _ := c.RGBA()
	// 使用标准公式: Y = 0.299*R + 0.587*G + 0.114*B
	gray := (19595*r + 38470*g + 7471*b + 1<<15) >> 24
	return uint8(gray)
}
