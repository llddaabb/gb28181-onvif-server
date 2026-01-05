package api

import (
	"sync"
	"time"
)

// PreviewSession 预览会话信息
type PreviewSession struct {
	DeviceID   string `json:"device_id"`
	ChannelID  string `json:"channel_id,omitempty"`
	StreamKey  string `json:"stream_key"`
	App        string `json:"app"`
	Stream     string `json:"stream"`
	SourceURL  string `json:"source_url,omitempty"`
	FlvURL     string `json:"flv_url"`
	WsFlvURL   string `json:"ws_flv_url"`
	HlsURL     string `json:"hls_url"`
	RtmpURL    string `json:"rtmp_url"`
	RtspURL    string `json:"rtsp_url"`
	CreateTime int64  `json:"create_time"`
	DeviceType string `json:"device_type"` // "gb28181" or "onvif"
}

// PreviewSessionManager 预览会话管理器
type PreviewSessionManager struct {
	sessions map[string]*PreviewSession
	mu       sync.RWMutex
}

// NewPreviewSessionManager 创建预览会话管理器
func NewPreviewSessionManager() *PreviewSessionManager {
	return &PreviewSessionManager{
		sessions: make(map[string]*PreviewSession),
	}
}

// Add 添加预览会话
func (m *PreviewSessionManager) Add(session *PreviewSession) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[session.StreamKey] = session
}

// Get 获取预览会话
func (m *PreviewSessionManager) Get(key string) (*PreviewSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	session, exists := m.sessions[key]
	return session, exists
}

// Remove 移除预览会话
func (m *PreviewSessionManager) Remove(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, key)
}

// GetAll 获取所有预览会话
func (m *PreviewSessionManager) GetAll() []*PreviewSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*PreviewSession, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

// GetByDevice 根据设备ID获取预览会话
func (m *PreviewSessionManager) GetByDevice(deviceID string) []*PreviewSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*PreviewSession, 0)
	for _, session := range m.sessions {
		if session.DeviceID == deviceID {
			sessions = append(sessions, session)
		}
	}
	return sessions
}

// Clean 清理过期的预览会话（超过指定时间未使用）
func (m *PreviewSessionManager) Clean(maxAge time.Duration) []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().Unix()
	removed := []string{}

	for key, session := range m.sessions {
		if now-session.CreateTime > int64(maxAge.Seconds()) {
			delete(m.sessions, key)
			removed = append(removed, key)
		}
	}

	return removed
}

// Count 获取会话数量
func (m *PreviewSessionManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}
