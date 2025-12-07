package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"time"
)

// HTTPDetector HTTP API检测器（调用外部AI服务或本地服务）
type HTTPDetector struct {
	apiURL    string
	config    DetectorConfig
	modelInfo ModelInfo
	client    *http.Client
}

// NewHTTPDetector 创建HTTP检测器
func NewHTTPDetector(config DetectorConfig, apiURL string) (*HTTPDetector, error) {
	if apiURL == "" {
		apiURL = "http://localhost:8000/detect" // 默认本地AI服务
	}

	detector := &HTTPDetector{
		apiURL: apiURL,
		config: config,
		modelInfo: ModelInfo{
			Name:         "HTTP-AI-Service",
			Backend:      "remote",
			InputSize:    config.InputSize,
			Confidence:   config.Confidence,
			IoUThreshold: config.IoUThreshold,
		},
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	return detector, nil
}

// APIDetectionResponse AI服务响应
type APIDetectionResponse struct {
	Success     bool    `json:"success"`
	HasPerson   bool    `json:"has_person"`
	PersonCount int     `json:"person_count"`
	Confidence  float32 `json:"confidence"`
	Boxes       []struct {
		X1         float32 `json:"x1"`
		Y1         float32 `json:"y1"`
		X2         float32 `json:"x2"`
		Y2         float32 `json:"y2"`
		Confidence float32 `json:"confidence"`
		Class      string  `json:"class"`
	} `json:"boxes"`
	Error string `json:"error,omitempty"`
}

// Detect 检测图像中是否有人
func (d *HTTPDetector) Detect(ctx context.Context, img image.Image) (*DetectionResult, error) {
	// 编码图像为JPEG
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return nil, fmt.Errorf("编码图像失败: %w", err)
	}

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "POST", d.apiURL, &buf)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "image/jpeg")
	req.Header.Set("X-Confidence-Threshold", fmt.Sprintf("%.2f", d.config.Confidence))

	// 发送请求
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API返回错误: %s - %s", resp.Status, string(body))
	}

	// 解析响应
	var apiResp APIDetectionResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("检测失败: %s", apiResp.Error)
	}

	// 转换为DetectionResult
	result := &DetectionResult{
		HasPerson:   apiResp.HasPerson,
		PersonCount: apiResp.PersonCount,
		Confidence:  apiResp.Confidence,
		Boxes:       make([]BBox, len(apiResp.Boxes)),
		Timestamp:   time.Now(),
	}

	for i, box := range apiResp.Boxes {
		result.Boxes[i] = BBox{
			X1:         box.X1,
			Y1:         box.Y1,
			X2:         box.X2,
			Y2:         box.Y2,
			Confidence: box.Confidence,
			Class:      box.Class,
		}
	}

	return result, nil
}

// GetModelInfo 获取模型信息
func (d *HTTPDetector) GetModelInfo() ModelInfo {
	return d.modelInfo
}

// Close 关闭检测器
func (d *HTTPDetector) Close() error {
	d.client.CloseIdleConnections()
	return nil
}
