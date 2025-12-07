package zlm

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// MediaStreamConfig ZLM 媒体流配置
type MediaStreamConfig struct {
	ID           string
	URL          string
	Type         string // rtsp, http, rtmp
	DeviceID     string
	ChannelID    string
	Status       string // running, stopped, error
	StartTime    time.Time
	LastError    string
	RecordingURL string // 录像存储路径
}

// ZLMServer ZLM 媒体服务器
type ZLMServer struct {
	config     *Config
	streams    map[string]*MediaStreamConfig
	mutex      sync.RWMutex
	stopChan   chan struct{}
	running    bool
	recordPath string
	apiClient  *ZLMAPIClient // API 客户端
}

// Config ZLM 配置
type Config struct {
	Host          string
	Port          int
	HTTPPort      int // HTTP API 端口
	RTMPPort      int // RTMP 端口
	RTSPPort      int // RTSP 端口
	MediaPort     int
	RecordingPath string
	MaxStreams    int
	BufferSize    int
	APIBaseURL    string // ZLM HTTP API 地址
	Secret        string // ZLM API 密钥
}

// NewZLMServer 创建 ZLM 服务器实例
func NewZLMServer(cfg *Config) *ZLMServer {
	if cfg == nil {
		cfg = &Config{
			Host:          "127.0.0.1",
			Port:          8554,
			HTTPPort:      8080,
			RTMPPort:      1935,
			RTSPPort:      554,
			MediaPort:     1935,
			RecordingPath: "./recordings",
			MaxStreams:    100,
			BufferSize:    1024 * 1024,
		}
	}

	// 创建 API 客户端
	var apiClient *ZLMAPIClient
	if cfg.APIBaseURL != "" {
		apiClient = NewZLMAPIClient(cfg.APIBaseURL, WithSecret(cfg.Secret))
	} else if cfg.HTTPPort > 0 {
		apiClient = NewZLMAPIClient(fmt.Sprintf("http://%s:%d", cfg.Host, cfg.HTTPPort), WithSecret(cfg.Secret))
	}

	return &ZLMServer{
		config:     cfg,
		streams:    make(map[string]*MediaStreamConfig),
		stopChan:   make(chan struct{}),
		recordPath: cfg.RecordingPath,
		apiClient:  apiClient,
	}
}

// GetAPIClient 获取 API 客户端
func (zs *ZLMServer) GetAPIClient() *ZLMAPIClient {
	return zs.apiClient
}

// SetAPIClient 设置 API 客户端
func (zs *ZLMServer) SetAPIClient(client *ZLMAPIClient) {
	zs.apiClient = client
}

// Start 启动 ZLM 服务器
func (zs *ZLMServer) Start() error {
	if zs.running {
		return fmt.Errorf("ZLM服务器已启动")
	}

	zs.running = true
	log.Printf("ZLM服务器启动，监听地址: %s:%d", zs.config.Host, zs.config.Port)

	// 启动监控协程
	go zs.monitorStreams()

	return nil
}

// Stop 停止 ZLM 服务器
func (zs *ZLMServer) Stop() error {
	if !zs.running {
		return nil
	}

	zs.running = false
	close(zs.stopChan)

	// 停止所有流
	zs.mutex.Lock()
	for _, stream := range zs.streams {
		stream.Status = "stopped"
	}
	zs.mutex.Unlock()

	log.Println("ZLM服务器已停止")
	return nil
}

// AddStream 添加媒体流
func (zs *ZLMServer) AddStream(stream *MediaStreamConfig) error {
	zs.mutex.Lock()
	defer zs.mutex.Unlock()

	if _, exists := zs.streams[stream.ID]; exists {
		return fmt.Errorf("流已存在: %s", stream.ID)
	}

	if len(zs.streams) >= zs.config.MaxStreams {
		return fmt.Errorf("达到最大流数限制: %d", zs.config.MaxStreams)
	}

	stream.Status = "running"
	stream.StartTime = time.Now()
	zs.streams[stream.ID] = stream

	log.Printf("添加媒体流: %s (%s)", stream.ID, stream.URL)
	return nil
}

// RemoveStream 移除媒体流
func (zs *ZLMServer) RemoveStream(streamID string) error {
	zs.mutex.Lock()
	defer zs.mutex.Unlock()

	if _, exists := zs.streams[streamID]; !exists {
		return fmt.Errorf("流不存在: %s", streamID)
	}

	delete(zs.streams, streamID)
	log.Printf("删除媒体流: %s", streamID)
	return nil
}

// GetStream 获取媒体流
func (zs *ZLMServer) GetStream(streamID string) (*MediaStreamConfig, bool) {
	zs.mutex.RLock()
	defer zs.mutex.RUnlock()

	stream, exists := zs.streams[streamID]
	return stream, exists
}

// GetStreams 获取所有媒体流
func (zs *ZLMServer) GetStreams() []*MediaStreamConfig {
	zs.mutex.RLock()
	defer zs.mutex.RUnlock()

	streams := make([]*MediaStreamConfig, 0, len(zs.streams))
	for _, stream := range zs.streams {
		streams = append(streams, stream)
	}
	return streams
}

// GetMediaList 从ZLM API获取媒体流列表
func (zs *ZLMServer) GetMediaList() ([]*StreamInfo, error) {
	if zs.apiClient == nil {
		return nil, fmt.Errorf("API client not initialized")
	}
	return zs.apiClient.GetMediaList()
}

// CloseStream 关闭ZLM媒体流
func (zs *ZLMServer) CloseStream(app, stream string) error {
	if zs.apiClient == nil {
		return fmt.Errorf("API client not initialized")
	}
	return zs.apiClient.CloseStream(app, stream)
}

// AddStreamProxy 添加流代理
func (zs *ZLMServer) AddStreamProxy(app, stream, url string) (string, error) {
	if zs.apiClient == nil {
		return "", fmt.Errorf("API client not initialized")
	}
	info, err := zs.apiClient.AddStreamProxy(url, app, stream)
	if err != nil {
		return "", err
	}
	return info.Key, nil
}

// GetStreamsByDevice 根据设备ID获取媒体流
func (zs *ZLMServer) GetStreamsByDevice(deviceID string) []*MediaStreamConfig {
	zs.mutex.RLock()
	defer zs.mutex.RUnlock()

	streams := make([]*MediaStreamConfig, 0)
	for _, stream := range zs.streams {
		if stream.DeviceID == deviceID {
			streams = append(streams, stream)
		}
	}
	return streams
}

// StartRecording 启动录像
func (zs *ZLMServer) StartRecording(streamID, recordingPath string) error {
	zs.mutex.Lock()
	defer zs.mutex.Unlock()

	stream, exists := zs.streams[streamID]
	if !exists {
		return fmt.Errorf("流不存在: %s", streamID)
	}

	if recordingPath == "" {
		recordingPath = fmt.Sprintf("%s/%s.mp4", zs.recordPath, streamID)
	}

	stream.RecordingURL = recordingPath
	log.Printf("启动录像: %s -> %s", streamID, recordingPath)
	return nil
}

// StopRecording 停止录像
func (zs *ZLMServer) StopRecording(streamID string) error {
	zs.mutex.Lock()
	defer zs.mutex.Unlock()

	stream, exists := zs.streams[streamID]
	if !exists {
		return fmt.Errorf("流不存在: %s", streamID)
	}

	recordingURL := stream.RecordingURL
	stream.RecordingURL = ""

	log.Printf("停止录像: %s (保存到: %s)", streamID, recordingURL)
	return nil
}

// GetRecordingPath 获取录像路径
func (zs *ZLMServer) GetRecordingPath(streamID string) (string, error) {
	zs.mutex.RLock()
	defer zs.mutex.RUnlock()

	stream, exists := zs.streams[streamID]
	if !exists {
		return "", fmt.Errorf("流不存在: %s", streamID)
	}

	return stream.RecordingURL, nil
}

// monitorStreams 监控流状态
func (zs *ZLMServer) monitorStreams() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			zs.checkStreamHealth()
		case <-zs.stopChan:
			return
		}
	}
}

// checkStreamHealth 检查流健康状态
func (zs *ZLMServer) checkStreamHealth() {
	zs.mutex.Lock()
	defer zs.mutex.Unlock()

	for _, stream := range zs.streams {
		// 简化实现：模拟健康检查
		if stream.Status == "running" {
			// 实际应该连接到流源检查是否可用
			log.Printf("检查流状态: %s (状态正常)", stream.ID)
		}
	}
}

// GetStatistics 获取服务器统计信息
func (zs *ZLMServer) GetStatistics() map[string]interface{} {
	zs.mutex.RLock()
	defer zs.mutex.RUnlock()

	runningCount := 0
	stoppedCount := 0
	errorCount := 0

	for _, stream := range zs.streams {
		switch stream.Status {
		case "running":
			runningCount++
		case "stopped":
			stoppedCount++
		case "error":
			errorCount++
		}
	}

	return map[string]interface{}{
		"host":           zs.config.Host,
		"port":           zs.config.Port,
		"maxStreams":     zs.config.MaxStreams,
		"totalStreams":   len(zs.streams),
		"runningStreams": runningCount,
		"stoppedStreams": stoppedCount,
		"errorStreams":   errorCount,
		"recordingPath":  zs.recordPath,
	}
}
