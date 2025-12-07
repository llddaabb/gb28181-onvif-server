package zlm

import (
	"fmt"
	"log"
)

// EnhancedZLMManager 增强的 ZLM 管理器 - 结合 API 客户端和本地管理
type EnhancedZLMManager struct {
	apiClient *ZLMAPIClient
	localServer *ZLMServer
}

// NewEnhancedZLMManager 创建增强的 ZLM 管理器
func NewEnhancedZLMManager(apiURL string, cfg *Config) *EnhancedZLMManager {
	return &EnhancedZLMManager{
		apiClient: NewZLMAPIClient(apiURL, WithTimeout(10)),
		localServer: NewZLMServer(cfg),
	}
}

// Start 启动管理器
func (m *EnhancedZLMManager) Start() error {
	// 检查 API 连接
	if !m.apiClient.Health() {
		return fmt.Errorf("ZLM API 服务不可用")
	}

	// 启动本地服务器
	if err := m.localServer.Start(); err != nil {
		return fmt.Errorf("启动本地服务器失败: %w", err)
	}

	// 获取并记录版本信息
	version, err := m.apiClient.GetVersion()
	if err != nil {
		log.Printf("警告: 无法获取版本信息: %v", err)
	} else {
		log.Printf("ZLM Pro 版本: %s", version.Version)
	}

	return nil
}

// Stop 停止管理器
func (m *EnhancedZLMManager) Stop() error {
	return m.localServer.Stop()
}

// GetStreamList 获取所有流列表 (从 ZLM API)
func (m *EnhancedZLMManager) GetStreamList() ([]*StreamInfo, error) {
	return m.apiClient.GetMediaList()
}

// GetServerStats 获取服务器统计信息
func (m *EnhancedZLMManager) GetServerStats() (map[string]interface{}, error) {
	return m.apiClient.GetStatistic()
}

// IsStreamOnline 检查流是否在线
func (m *EnhancedZLMManager) IsStreamOnline(app, stream string) (bool, error) {
	streams, err := m.apiClient.GetMediaList()
	if err != nil {
		return false, err
	}

	for _, s := range streams {
		if s.App == app && s.Stream == stream && s.Online == 1 {
			return true, nil
		}
	}

	return false, nil
}

// AddRTPStream 添加 RTP 流
func (m *EnhancedZLMManager) AddRTPStream(app, stream string) (*RTPInfo, error) {
	return m.apiClient.OpenRTP(app, stream)
}

// RemoveStream 移除流
func (m *EnhancedZLMManager) RemoveStream(app, stream string) error {
	return m.apiClient.CloseStream(app, stream)
}

// Example_BasicUsage 基本使用示例
func Example_BasicUsage() {
	// 1. 创建 API 客户端
	client := NewZLMAPIClient("http://127.0.0.1:16379")

	// 2. 获取版本信息
	version, err := client.GetVersion()
	if err != nil {
		log.Printf("获取版本失败: %v", err)
		return
	}
	fmt.Printf("ZLM 版本: %s\n", version.Version)

	// 3. 获取所有媒体流
	streams, err := client.GetMediaList()
	if err != nil {
		log.Printf("获取流列表失败: %v", err)
		return
	}

	fmt.Printf("当前流数: %d\n", len(streams))
	for _, stream := range streams {
		fmt.Printf("  应用: %s, 流: %s, 在线: %v, 观众: %d\n",
			stream.App, stream.Stream, stream.Online == 1, stream.ReaderCount)
	}

	// 4. 获取服务器统计
	stats, err := client.GetStatistic()
	if err != nil {
		log.Printf("获取统计失败: %v", err)
		return
	}
	fmt.Printf("服务器统计: %+v\n", stats)
}

// Example_EnhancedManager 增强管理器使用示例
func Example_EnhancedManager() {
	// 1. 创建增强管理器
	cfg := &Config{
		Host:          "127.0.0.1",
		Port:          8554,
		RecordingPath: "./recordings",
		MaxStreams:    100,
	}

	manager := NewEnhancedZLMManager("http://127.0.0.1:16379", cfg)

	// 2. 启动管理器
	if err := manager.Start(); err != nil {
		log.Printf("启动失败: %v", err)
		return
	}
	defer manager.Stop()

	// 3. 添加 RTP 流
	rtpInfo, err := manager.AddRTPStream("live", "camera1")
	if err != nil {
		log.Printf("添加 RTP 流失败: %v", err)
	} else {
		fmt.Printf("RTP 端口: %d\n", rtpInfo.Port)
	}

	// 4. 检查流状态
	online, err := manager.IsStreamOnline("live", "camera1")
	if err != nil {
		log.Printf("检查流状态失败: %v", err)
	} else {
		fmt.Printf("流在线: %v\n", online)
	}

	// 5. 获取流列表
	streams, err := manager.GetStreamList()
	if err != nil {
		log.Printf("获取流列表失败: %v", err)
	} else {
		fmt.Printf("当前流数: %d\n", len(streams))
	}

	// 6. 移除流
	if err := manager.RemoveStream("live", "camera1"); err != nil {
		log.Printf("移除流失败: %v", err)
	}
}

// Example_HTTPRequests 直接 HTTP 请求示例
func Example_HTTPRequests() {
	// 也可以直接使用 curl 命令:

	// 1. 获取版本
	// curl http://127.0.0.1:16379/api/version

	// 2. 获取流列表
	// curl http://127.0.0.1:16379/api/getMediaList

	// 3. 打开 RTP 推流
	// curl -X POST http://127.0.0.1:16379/api/openRtp \
	//   -H "Content-Type: application/json" \
	//   -d '{"app":"live","stream":"test"}'

	// 4. 关闭流
	// curl -X POST http://127.0.0.1:16379/api/closeStream \
	//   -H "Content-Type: application/json" \
	//   -d '{"app":"live","stream":"test"}'

	// 5. 获取统计
	// curl http://127.0.0.1:16379/api/getStatistic

	// 6. 获取服务器配置
	// curl http://127.0.0.1:16379/api/getServerConfig

	log.Println("查看注释中的 curl 命令示例")
}
