package api

import (
	"fmt"
	"sync"
	"time"
)

// ChannelManager 通道管理器
type ChannelManager struct {
	channels map[string]*Channel
	mutex    sync.RWMutex
}

// NewChannelManager 创建通道管理器
func NewChannelManager() *ChannelManager {
	return &ChannelManager{
		channels: make(map[string]*Channel),
	}
}

// AddChannel 添加通道
func (cm *ChannelManager) AddChannel(channel *Channel) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if _, exists := cm.channels[channel.ChannelID]; exists {
		return fmt.Errorf("通道已存在: %s", channel.ChannelID)
	}

	channel.Status = "online"
	cm.channels[channel.ChannelID] = channel
	return nil
}

// GetChannel 获取通道
func (cm *ChannelManager) GetChannel(channelID string) (*Channel, bool) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	channel, exists := cm.channels[channelID]
	return channel, exists
}

// GetChannels 获取所有通道
func (cm *ChannelManager) GetChannels() []*Channel {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	channels := make([]*Channel, 0, len(cm.channels))
	for _, channel := range cm.channels {
		channels = append(channels, channel)
	}
	return channels
}

// GetChannelsByDevice 根据设备ID获取通道
func (cm *ChannelManager) GetChannelsByDevice(deviceID string) []*Channel {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	channels := make([]*Channel, 0)
	for _, channel := range cm.channels {
		if channel.DeviceID == deviceID {
			channels = append(channels, channel)
		}
	}
	return channels
}

// DeleteChannel 删除通道
func (cm *ChannelManager) DeleteChannel(channelID string) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if _, exists := cm.channels[channelID]; !exists {
		return fmt.Errorf("通道不存在: %s", channelID)
	}

	delete(cm.channels, channelID)
	return nil
}

// UpdateChannel 更新通道
func (cm *ChannelManager) UpdateChannel(channel *Channel) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if _, exists := cm.channels[channel.ChannelID]; !exists {
		return fmt.Errorf("通道不存在: %s", channel.ChannelID)
	}

	cm.channels[channel.ChannelID] = channel
	return nil
}

// RecordingManager 录像管理器
type RecordingManager struct {
	recordings               map[string]*Recording
	persistentRecordings     map[string]bool // 持久录像状态：channelID -> 是否启用持久录像
	persistentRecordingsMutex sync.RWMutex
	mutex                    sync.RWMutex
}

// NewRecordingManager 创建录像管理器
func NewRecordingManager() *RecordingManager {
	return &RecordingManager{
		recordings:           make(map[string]*Recording),
		persistentRecordings: make(map[string]bool),
	}
}

// AddRecording 添加录像
func (rm *RecordingManager) AddRecording(recording *Recording) error {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	if _, exists := rm.recordings[recording.RecordingID]; exists {
		return fmt.Errorf("录像已存在: %s", recording.RecordingID)
	}

	rm.recordings[recording.RecordingID] = recording
	return nil
}

// GetRecording 获取录像
func (rm *RecordingManager) GetRecording(recordingID string) (*Recording, bool) {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	recording, exists := rm.recordings[recordingID]
	return recording, exists
}

// GetRecordingsByDate 根据日期获取录像
func (rm *RecordingManager) GetRecordingsByDate(channelID string, date time.Time) []*Recording {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	recordings := make([]*Recording, 0)
	for _, recording := range rm.recordings {
		if recording.ChannelID == channelID &&
			recording.StartTime.After(startOfDay) &&
			recording.StartTime.Before(endOfDay) {
			recordings = append(recordings, recording)
		}
	}
	return recordings
}

// SetPersistentRecording 设置持久录像
func (rm *RecordingManager) SetPersistentRecording(channelID string, enable bool) {
	rm.persistentRecordingsMutex.Lock()
	defer rm.persistentRecordingsMutex.Unlock()
	
	if enable {
		rm.persistentRecordings[channelID] = true
	} else {
		delete(rm.persistentRecordings, channelID)
	}
}

// IsPersistentRecording 检查是否启用持久录像
func (rm *RecordingManager) IsPersistentRecording(channelID string) bool {
	rm.persistentRecordingsMutex.RLock()
	defer rm.persistentRecordingsMutex.RUnlock()
	
	return rm.persistentRecordings[channelID]
}

// GetPersistentRecordings 获取所有需要持久录像的通道
func (rm *RecordingManager) GetPersistentRecordings() []string {
	rm.persistentRecordingsMutex.RLock()
	defer rm.persistentRecordingsMutex.RUnlock()
	
	channels := make([]string, 0, len(rm.persistentRecordings))
	for channelID, enabled := range rm.persistentRecordings {
		if enabled {
			channels = append(channels, channelID)
		}
	}
	return channels
}

// StreamManager 流管理器
type StreamManager struct {
	streams map[string]*StreamInfo
	mutex   sync.RWMutex
}

// NewStreamManager 创建流管理器
func NewStreamManager() *StreamManager {
	return &StreamManager{
		streams: make(map[string]*StreamInfo),
	}
}

// StartStream 启动流
func (sm *StreamManager) StartStream(streamInfo *StreamInfo) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if _, exists := sm.streams[streamInfo.StreamID]; exists {
		return fmt.Errorf("流已存在: %s", streamInfo.StreamID)
	}

	streamInfo.Status = "running"
	streamInfo.StartTime = time.Now()
	sm.streams[streamInfo.StreamID] = streamInfo
	return nil
}

// StopStream 停止流
func (sm *StreamManager) StopStream(streamID string) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	stream, exists := sm.streams[streamID]
	if !exists {
		return fmt.Errorf("流不存在: %s", streamID)
	}

	stream.Status = "stopped"
	stream.EndTime = time.Now()
	return nil
}

// GetStreams 获取所有运行中的流
func (sm *StreamManager) GetStreams() []*StreamInfo {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	streams := make([]*StreamInfo, 0, len(sm.streams))
	for _, stream := range sm.streams {
		if stream.Status == "running" {
			streams = append(streams, stream)
		}
	}
	return streams
}
