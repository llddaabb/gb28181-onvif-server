//go:build linux && cgo
// +build linux,cgo

package ai

/*
#cgo LDFLAGS: -L${SRCDIR}/lib -lonnxruntime -Wl,-rpath,${SRCDIR}/lib
#cgo CFLAGS: -I${SRCDIR}/include

#include <stdlib.h>
#include <onnxruntime_c_api.h>

// 错误处理辅助函数
static const char* GetErrorMessage(OrtStatus* status, const OrtApi* api) {
    return api->GetErrorMessage(status);
}
*/
import "C"

import (
	"context"
	"fmt"
	"image"
	"math"
	"sort"
	"sync"
	"time"
	"unsafe"

	"gb28181-onvif-server/internal/debug"
)

// ONNXRuntimeDetector 使用 ONNX Runtime 的检测器
type ONNXRuntimeDetector struct {
	config    DetectorConfig
	modelInfo ModelInfo
	mu        sync.Mutex

	// ONNX Runtime 对象
	api         *C.OrtApi
	env         *C.OrtEnv
	session     *C.OrtSession
	sessionOpts *C.OrtSessionOptions
	memoryInfo  *C.OrtMemoryInfo

	// 模型参数
	inputName   string
	outputName  string
	inputWidth  int
	inputHeight int
	numClasses  int

	// 预分配的缓冲区
	inputBuffer  []float32
	outputBuffer []float32
}

// NewONNXRuntimeDetector 创建 ONNX Runtime 检测器
func NewONNXRuntimeDetector(config DetectorConfig) (*ONNXRuntimeDetector, error) {
	detector := &ONNXRuntimeDetector{
		config: config,
		modelInfo: ModelInfo{
			Name:         "ONNX-YOLOv8",
			Backend:      config.Backend,
			InputSize:    config.InputSize,
			Confidence:   config.Confidence,
			IoUThreshold: config.IoUThreshold,
		},
		inputWidth:  config.InputSize,
		inputHeight: config.InputSize,
		numClasses:  80, // COCO
		inputName:   "images",
		outputName:  "output0",
	}

	// 获取 API
	detector.api = C.OrtGetApiBase().GetApi(C.ORT_API_VERSION)
	if detector.api == nil {
		return nil, fmt.Errorf("无法获取 ONNX Runtime API")
	}

	// 创建环境
	var status *C.OrtStatus
	envName := C.CString("yolov8_detector")
	defer C.free(unsafe.Pointer(envName))

	status = C.OrtApi_CreateEnv(detector.api, C.ORT_LOGGING_LEVEL_WARNING, envName, &detector.env)
	if status != nil {
		return nil, fmt.Errorf("创建 ONNX 环境失败: %s", C.GoString(C.GetErrorMessage(status, detector.api)))
	}

	// 创建会话选项
	status = C.OrtApi_CreateSessionOptions(detector.api, &detector.sessionOpts)
	if status != nil {
		detector.Close()
		return nil, fmt.Errorf("创建会话选项失败")
	}

	// 设置线程数
	if config.NumThreads > 0 {
		C.OrtApi_SetIntraOpNumThreads(detector.api, detector.sessionOpts, C.int(config.NumThreads))
	}

	// 设置优化级别
	C.OrtApi_SetSessionGraphOptimizationLevel(detector.api, detector.sessionOpts, C.ORT_ENABLE_ALL)

	// 加载模型
	modelPath := C.CString(config.ModelPath)
	defer C.free(unsafe.Pointer(modelPath))

	status = C.OrtApi_CreateSession(detector.api, detector.env, modelPath, detector.sessionOpts, &detector.session)
	if status != nil {
		detector.Close()
		return nil, fmt.Errorf("加载模型失败: %s", C.GoString(C.GetErrorMessage(status, detector.api)))
	}

	// 创建内存信息
	cpuStr := C.CString("Cpu")
	defer C.free(unsafe.Pointer(cpuStr))
	status = C.OrtApi_CreateCpuMemoryInfo(detector.api, C.OrtArenaAllocator, C.OrtMemTypeDefault, &detector.memoryInfo)
	if status != nil {
		detector.Close()
		return nil, fmt.Errorf("创建内存信息失败")
	}

	// 预分配缓冲区
	detector.inputBuffer = make([]float32, 3*config.InputSize*config.InputSize)
	// YOLOv8 输出: [1, 84, 8400] (84 = 4 bbox + 80 classes)
	detector.outputBuffer = make([]float32, 84*8400)

	debug.Info("ai", "ONNX Runtime 检测器已创建: model=%s, backend=%s",
		config.ModelPath, config.Backend)

	return detector, nil
}

// Detect 检测图像中的目标
func (d *ONNXRuntimeDetector) Detect(ctx context.Context, img image.Image) (*DetectionResult, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	startTime := time.Now()

	// 预处理
	scaleX, scaleY := d.preprocess(img)

	// 创建输入张量
	inputShape := []C.int64_t{1, 3, C.int64_t(d.inputHeight), C.int64_t(d.inputWidth)}
	var inputTensor *C.OrtValue

	status := C.OrtApi_CreateTensorWithDataAsOrtValue(
		d.api,
		d.memoryInfo,
		unsafe.Pointer(&d.inputBuffer[0]),
		C.size_t(len(d.inputBuffer)*4),
		&inputShape[0],
		C.size_t(len(inputShape)),
		C.ONNX_TENSOR_ELEMENT_DATA_TYPE_FLOAT,
		&inputTensor,
	)
	if status != nil {
		return nil, fmt.Errorf("创建输入张量失败")
	}
	defer C.OrtApi_ReleaseValue(d.api, inputTensor)

	// 运行推理
	inputNameC := C.CString(d.inputName)
	outputNameC := C.CString(d.outputName)
	defer C.free(unsafe.Pointer(inputNameC))
	defer C.free(unsafe.Pointer(outputNameC))

	inputNames := []*C.char{inputNameC}
	outputNames := []*C.char{outputNameC}
	inputs := []*C.OrtValue{inputTensor}
	var outputTensor *C.OrtValue

	status = C.OrtApi_Run(
		d.api,
		d.session,
		nil,
		&inputNames[0],
		&inputs[0],
		1,
		&outputNames[0],
		1,
		&outputTensor,
	)
	if status != nil {
		return nil, fmt.Errorf("推理失败: %s", C.GoString(C.GetErrorMessage(status, d.api)))
	}
	defer C.OrtApi_ReleaseValue(d.api, outputTensor)

	// 获取输出数据
	var outputData *C.float
	status = C.OrtApi_GetTensorMutableData(d.api, outputTensor, (*unsafe.Pointer)(unsafe.Pointer(&outputData)))
	if status != nil {
		return nil, fmt.Errorf("获取输出数据失败")
	}

	// 复制输出数据
	outputSize := 84 * 8400
	copy(d.outputBuffer, (*[84 * 8400]float32)(unsafe.Pointer(outputData))[:outputSize:outputSize])

	// 后处理
	boxes := d.postprocess(scaleX, scaleY)

	// 统计结果
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

	debug.Debug("ai", "ONNX推理完成: hasPerson=%v, count=%d, time=%v",
		result.HasPerson, result.PersonCount, time.Since(startTime))

	return result, nil
}

// preprocess 预处理图像
func (d *ONNXRuntimeDetector) preprocess(img image.Image) (scaleX, scaleY float32) {
	bounds := img.Bounds()
	origWidth := bounds.Dx()
	origHeight := bounds.Dy()

	scaleX = float32(d.inputWidth) / float32(origWidth)
	scaleY = float32(d.inputHeight) / float32(origHeight)

	// 填充输入缓冲区 (CHW格式, RGB)
	for y := 0; y < d.inputHeight; y++ {
		for x := 0; x < d.inputWidth; x++ {
			srcX := int(float32(x) / scaleX)
			srcY := int(float32(y) / scaleY)

			if srcX >= origWidth {
				srcX = origWidth - 1
			}
			if srcY >= origHeight {
				srcY = origHeight - 1
			}

			r, g, b, _ := img.At(srcX+bounds.Min.X, srcY+bounds.Min.Y).RGBA()

			idx := y*d.inputWidth + x
			d.inputBuffer[idx] = float32(r>>8) / 255.0
			d.inputBuffer[d.inputHeight*d.inputWidth+idx] = float32(g>>8) / 255.0
			d.inputBuffer[2*d.inputHeight*d.inputWidth+idx] = float32(b>>8) / 255.0
		}
	}

	return
}

// postprocess 后处理 YOLOv8 输出
func (d *ONNXRuntimeDetector) postprocess(scaleX, scaleY float32) []BBox {
	var boxes []BBox

	// YOLOv8 输出格式: [1, 84, 8400]
	// 84 = 4 (xywh) + 80 (class scores)
	// 8400 = 总检测数

	numDetections := 8400
	numFeatures := 84

	for i := 0; i < numDetections; i++ {
		// 获取边界框坐标
		cx := d.outputBuffer[0*numDetections+i]
		cy := d.outputBuffer[1*numDetections+i]
		w := d.outputBuffer[2*numDetections+i]
		h := d.outputBuffer[3*numDetections+i]

		// 找到最高置信度的类别
		maxScore := float32(0)
		maxClass := 0
		for c := 0; c < d.numClasses; c++ {
			score := d.outputBuffer[(4+c)*numDetections+i]
			if score > maxScore {
				maxScore = score
				maxClass = c
			}
		}

		// 过滤低置信度
		if maxScore < d.config.Confidence {
			continue
		}

		// 只保留person类别 (class 0)
		if maxClass != 0 {
			continue
		}

		// 转换坐标 (中心点+宽高 -> 左上右下)
		x1 := (cx - w/2) / scaleX
		y1 := (cy - h/2) / scaleY
		x2 := (cx + w/2) / scaleX
		y2 := (cy + h/2) / scaleY

		boxes = append(boxes, BBox{
			X1:         x1,
			Y1:         y1,
			X2:         x2,
			Y2:         y2,
			Confidence: maxScore,
			Class:      cocoClassNames[maxClass],
		})
	}

	// NMS
	boxes = d.nms(boxes)

	return boxes
}

// nms 非极大值抑制
func (d *ONNXRuntimeDetector) nms(boxes []BBox) []BBox {
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
			if d.iou(boxes[i], boxes[j]) > d.config.IoUThreshold {
				used[j] = true
			}
		}
	}

	return result
}

// iou 计算IoU
func (d *ONNXRuntimeDetector) iou(a, b BBox) float32 {
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

// GetModelInfo 获取模型信息
func (d *ONNXRuntimeDetector) GetModelInfo() ModelInfo {
	return d.modelInfo
}

// Close 关闭检测器
func (d *ONNXRuntimeDetector) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.session != nil {
		C.OrtApi_ReleaseSession(d.api, d.session)
		d.session = nil
	}
	if d.sessionOpts != nil {
		C.OrtApi_ReleaseSessionOptions(d.api, d.sessionOpts)
		d.sessionOpts = nil
	}
	if d.memoryInfo != nil {
		C.OrtApi_ReleaseMemoryInfo(d.api, d.memoryInfo)
		d.memoryInfo = nil
	}
	if d.env != nil {
		C.OrtApi_ReleaseEnv(d.api, d.env)
		d.env = nil
	}

	debug.Info("ai", "ONNX Runtime 检测器已关闭")
	return nil
}
