package ai

import (
	"fmt"
	"sync"

	"gb28181-onvif-server/internal/debug"
)

// AIRecordingManager AI录像管理器
type AIRecordingManager struct {
	recorders     map[string]*StreamRecorder
	recordControl RecordControlFunc
	defaultConfig DetectorConfig
	mu            sync.RWMutex
}

// NewAIRecordingManager 创建AI录像管理器
func NewAIRecordingManager(recordControl RecordControlFunc) *AIRecordingManager {
	return &AIRecordingManager{
		recorders:     make(map[string]*StreamRecorder),
		recordControl: recordControl,
		defaultConfig: DefaultDetectorConfig(),
	}
}

// StartChannelRecording 启动通道AI录像
func (m *AIRecordingManager) StartChannelRecording(channelID string, mode RecordingMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已存在
	if _, exists := m.recorders[channelID]; exists {
		return fmt.Errorf("通道 %s 的AI录像已启动", channelID)
	}

	// 创建录像器配置
	config := DefaultRecorderConfig(channelID)
	config.Mode = mode

	// 创建录像器
	recorder, err := NewStreamRecorder(config, m.recordControl)
	if err != nil {
		return fmt.Errorf("创建录像器失败: %w", err)
	}

	// 启动录像器
	if err := recorder.Start(); err != nil {
		return fmt.Errorf("启动录像器失败: %w", err)
	}

	m.recorders[channelID] = recorder
	debug.Info("ai", "通道AI录像已启动: channelID=%s, mode=%s", channelID, mode)

	return nil
}

// StopChannelRecording 停止通道AI录像
func (m *AIRecordingManager) StopChannelRecording(channelID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	recorder, exists := m.recorders[channelID]
	if !exists {
		return fmt.Errorf("通道 %s 的AI录像未启动", channelID)
	}

	// 停止录像器
	if err := recorder.Stop(); err != nil {
		debug.Error("ai", "停止录像器失败: %v", err)
	}

	delete(m.recorders, channelID)
	debug.Info("ai", "通道AI录像已停止: channelID=%s", channelID)

	return nil
}

// GetChannelStatus 获取通道状态
func (m *AIRecordingManager) GetChannelStatus(channelID string) (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	recorder, exists := m.recorders[channelID]
	if !exists {
		return nil, fmt.Errorf("通道 %s 的AI录像未启动", channelID)
	}

	return recorder.GetStatus(), nil
}

// GetAllStatus 获取所有通道状态
func (m *AIRecordingManager) GetAllStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := make(map[string]interface{})
	for channelID, recorder := range m.recorders {
		status[channelID] = recorder.GetStatus()
	}

	return status
}

// StopAll 停止所有AI录像
func (m *AIRecordingManager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for channelID, recorder := range m.recorders {
		if err := recorder.Stop(); err != nil {
			debug.Error("ai", "停止通道 %s 录像失败: %v", channelID, err)
		}
	}

	m.recorders = make(map[string]*StreamRecorder)
	debug.Info("ai", "所有AI录像已停止")
}
