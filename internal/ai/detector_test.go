//go:build !cgo || test
// +build !cgo test

package ai

import (
	"context"
	"image"
	"image/color"
	"testing"
	"time"
)

// 创建测试图像
func createTestImage(width, height int, skinColor bool) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	var c color.RGBA
	if skinColor {
		// 肤色区域 (符合 YCbCr 肤色检测范围)
		c = color.RGBA{R: 200, G: 150, B: 130, A: 255}
	} else {
		// 非肤色区域
		c = color.RGBA{R: 50, G: 100, B: 150, A: 255}
	}

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, c)
		}
	}

	return img
}

// 创建混合测试图像（部分区域有肤色）
func createMixedTestImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// 背景色
	bg := color.RGBA{R: 50, G: 100, B: 150, A: 255}
	// 肤色
	skin := color.RGBA{R: 200, G: 150, B: 130, A: 255}

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// 在中间区域放置肤色块
			if x > width/4 && x < width*3/4 && y > height/4 && y < height*3/4 {
				img.Set(x, y, skin)
			} else {
				img.Set(x, y, bg)
			}
		}
	}

	return img
}

func TestEmbeddedDetector_Create(t *testing.T) {
	config := DefaultDetectorConfig()
	detector, err := NewEmbeddedDetector(config)

	if err != nil {
		t.Fatalf("创建嵌入式检测器失败: %v", err)
	}

	defer detector.Close()

	info := detector.GetModelInfo()
	if info.Name != "Embedded-YOLOv8" {
		t.Errorf("模型名称不正确: got %s, want Embedded-YOLOv8", info.Name)
	}
}

func TestEmbeddedDetector_Detect_NoSkin(t *testing.T) {
	config := DefaultDetectorConfig()
	config.Confidence = 0.3
	detector, err := NewEmbeddedDetector(config)
	if err != nil {
		t.Fatalf("创建检测器失败: %v", err)
	}
	defer detector.Close()

	// 创建无肤色的图像
	img := createTestImage(320, 320, false)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := detector.Detect(ctx, img)
	if err != nil {
		t.Fatalf("检测失败: %v", err)
	}

	// 无肤色图像不应检测到人
	if result.HasPerson {
		t.Logf("警告: 无肤色图像检测到 %d 人 (置信度: %.2f)", result.PersonCount, result.Confidence)
	}
}

func TestEmbeddedDetector_Detect_WithSkin(t *testing.T) {
	config := DefaultDetectorConfig()
	config.Confidence = 0.1 // 降低阈值以便检测
	detector, err := NewEmbeddedDetector(config)
	if err != nil {
		t.Fatalf("创建检测器失败: %v", err)
	}
	defer detector.Close()

	// 创建有肤色的图像
	img := createMixedTestImage(320, 320)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := detector.Detect(ctx, img)
	if err != nil {
		t.Fatalf("检测失败: %v", err)
	}

	t.Logf("检测结果: hasPerson=%v, count=%d, confidence=%.2f, boxes=%d",
		result.HasPerson, result.PersonCount, result.Confidence, len(result.Boxes))
}

func TestDetectorFactory_Auto(t *testing.T) {
	config := DefaultDetectorConfig()
	factoryConfig := DetectorFactoryConfig{
		Type:   DetectorTypeAuto,
		Config: config,
	}

	detector, err := CreateDetector(factoryConfig)
	if err != nil {
		t.Fatalf("创建检测器失败: %v", err)
	}
	defer detector.Close()

	info := detector.GetModelInfo()
	t.Logf("自动选择的检测器: name=%s, backend=%s", info.Name, info.Backend)
}

func TestDetectorFactory_Embedded(t *testing.T) {
	config := DefaultDetectorConfig()
	factoryConfig := DetectorFactoryConfig{
		Type:   DetectorTypeEmbedded,
		Config: config,
	}

	detector, err := CreateDetector(factoryConfig)
	if err != nil {
		t.Fatalf("创建嵌入式检测器失败: %v", err)
	}
	defer detector.Close()

	info := detector.GetModelInfo()
	if info.Backend != "go-native" {
		t.Errorf("后端不正确: got %s, want go-native", info.Backend)
	}
}

func TestListAvailableDetectors(t *testing.T) {
	detectors := ListAvailableDetectors()

	if len(detectors) == 0 {
		t.Fatal("没有可用的检测器")
	}

	for _, d := range detectors {
		t.Logf("检测器: type=%s, name=%s, status=%s, features=%v",
			d.Type, d.Name, d.Status, d.Features)
	}

	// 至少应该有 HTTP 和 Embedded 检测器
	hasHTTP := false
	hasEmbedded := false
	for _, d := range detectors {
		if d.Type == DetectorTypeHTTP {
			hasHTTP = true
		}
		if d.Type == DetectorTypeEmbedded {
			hasEmbedded = true
		}
	}

	if !hasHTTP {
		t.Error("缺少 HTTP 检测器")
	}
	if !hasEmbedded {
		t.Error("缺少嵌入式检测器")
	}
}

func TestDetectorPool(t *testing.T) {
	config := DefaultDetectorConfig()
	factoryConfig := DetectorFactoryConfig{
		Type:   DetectorTypeEmbedded,
		Config: config,
	}

	pool, err := NewDetectorPoolWithFactory(factoryConfig, 2)
	if err != nil {
		t.Fatalf("创建检测器池失败: %v", err)
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 获取检测器
	detector1, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("获取检测器失败: %v", err)
	}

	detector2, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("获取第二个检测器失败: %v", err)
	}

	// 归还检测器
	pool.Put(detector1)
	pool.Put(detector2)

	t.Log("检测器池测试通过")
}

func BenchmarkEmbeddedDetector_Detect(b *testing.B) {
	config := DefaultDetectorConfig()
	config.InputSize = 320
	detector, err := NewEmbeddedDetector(config)
	if err != nil {
		b.Fatalf("创建检测器失败: %v", err)
	}
	defer detector.Close()

	img := createMixedTestImage(640, 480)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := detector.Detect(ctx, img)
		if err != nil {
			b.Fatalf("检测失败: %v", err)
		}
	}
}
