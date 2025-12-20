package push

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"gb28181-onvif-server/internal/debug"
	"gb28181-onvif-server/internal/zlm"
)

// PushTarget 推流目标平台
type PushTarget struct {
	ID          string    `json:"id"`           // 唯一标识
	Name        string    `json:"name"`         // 名称（如：抖音直播）
	Platform    string    `json:"platform"`     // 平台类型: douyin, bilibili, kuaishou, custom
	PushURL     string    `json:"push_url"`     // 推流地址
	StreamKey   string    `json:"stream_key"`   // 推流密钥
	ChannelID   string    `json:"channel_id"`   // 关联的通道ID
	ChannelName string    `json:"channel_name"` // 通道名称
	SourceURL   string    `json:"source_url"`   // 源流地址
	Status      string    `json:"status"`       // 状态: stopped, pushing, error
	FFmpegKey   string    `json:"ffmpeg_key"`   // ZLM FFmpeg 任务 Key
	StartTime   time.Time `json:"start_time"`   // 开始时间
	ErrorMsg    string    `json:"error_msg"`    // 错误信息
	CreatedAt   time.Time `json:"created_at"`   // 创建时间
	UpdatedAt   time.Time `json:"updated_at"`   // 更新时间
}

// PlatformInfo 直播平台信息
type PlatformInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	PushURLTpl  string `json:"push_url_template"` // 推流地址模板
	Description string `json:"description"`
}

// 预定义的直播平台
var SupportedPlatforms = []PlatformInfo{
	{
		ID:          "douyin",
		Name:        "抖音直播",
		PushURLTpl:  "rtmp://live-push.douyin.com/live/{stream_key}",
		Description: "抖音直播平台",
	},
	{
		ID:          "bilibili",
		Name:        "B站直播",
		PushURLTpl:  "rtmp://live-push.bilivideo.com/live-bvc/{stream_key}",
		Description: "哔哩哔哩直播",
	},
	{
		ID:          "kuaishou",
		Name:        "快手直播",
		PushURLTpl:  "rtmp://live-push.kuaishou.com/push/{stream_key}",
		Description: "快手直播平台",
	},
	{
		ID:          "huya",
		Name:        "虎牙直播",
		PushURLTpl:  "rtmp://al.live-push.huya.com/huyalive/{stream_key}",
		Description: "虎牙直播平台",
	},
	{
		ID:          "douyu",
		Name:        "斗鱼直播",
		PushURLTpl:  "rtmp://send.douyu.com/live/{stream_key}",
		Description: "斗鱼直播平台",
	},
	{
		ID:          "custom",
		Name:        "自定义平台",
		PushURLTpl:  "{push_url}",
		Description: "自定义 RTMP 推流地址",
	},
}

// Manager 推流管理器
type Manager struct {
	mutex     sync.RWMutex
	targets   map[string]*PushTarget // key: target ID
	zlmClient *zlm.ZLMAPIClient
	dataFile  string
	httpPort  int // ZLM HTTP 端口
}

// NewManager 创建推流管理器
func NewManager(zlmClient *zlm.ZLMAPIClient, dataFile string, httpPort int) *Manager {
	m := &Manager{
		targets:   make(map[string]*PushTarget),
		zlmClient: zlmClient,
		dataFile:  dataFile,
		httpPort:  httpPort,
	}
	m.loadTargets()
	return m
}

// GetPlatforms 获取支持的直播平台列表
func (m *Manager) GetPlatforms() []PlatformInfo {
	return SupportedPlatforms
}

// GetTargets 获取所有推流目标
func (m *Manager) GetTargets() []*PushTarget {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := make([]*PushTarget, 0, len(m.targets))
	for _, target := range m.targets {
		result = append(result, target)
	}
	return result
}

// GetTarget 获取单个推流目标
func (m *Manager) GetTarget(id string) (*PushTarget, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	target, exists := m.targets[id]
	if !exists {
		return nil, errors.New("push target not found")
	}
	return target, nil
}

// GetTargetsByChannel 获取通道的所有推流任务
func (m *Manager) GetTargetsByChannel(channelID string) []*PushTarget {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var result []*PushTarget
	for _, target := range m.targets {
		if target.ChannelID == channelID {
			result = append(result, target)
		}
	}
	return result
}

// AddTarget 添加推流目标
func (m *Manager) AddTarget(target *PushTarget) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if target.ID == "" {
		target.ID = fmt.Sprintf("push_%d", time.Now().UnixNano())
	}

	target.Status = "stopped"
	target.CreatedAt = time.Now()
	target.UpdatedAt = time.Now()

	m.targets[target.ID] = target
	m.saveTargets()

	debug.Info("push", "Added push target: %s -> %s", target.Name, target.Platform)
	return nil
}

// UpdateTarget 更新推流目标
func (m *Manager) UpdateTarget(id string, updates map[string]interface{}) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	target, exists := m.targets[id]
	if !exists {
		return errors.New("push target not found")
	}

	if name, ok := updates["name"].(string); ok {
		target.Name = name
	}
	if platform, ok := updates["platform"].(string); ok {
		target.Platform = platform
	}
	if pushURL, ok := updates["push_url"].(string); ok {
		target.PushURL = pushURL
	}
	if streamKey, ok := updates["stream_key"].(string); ok {
		target.StreamKey = streamKey
	}

	target.UpdatedAt = time.Now()
	m.saveTargets()

	return nil
}

// DeleteTarget 删除推流目标
func (m *Manager) DeleteTarget(id string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	target, exists := m.targets[id]
	if !exists {
		return errors.New("push target not found")
	}

	// 如果正在推流，先停止
	if target.Status == "pushing" && target.FFmpegKey != "" {
		m.zlmClient.DelFFmpegSource(target.FFmpegKey)
	}

	delete(m.targets, id)
	m.saveTargets()

	debug.Info("push", "Deleted push target: %s", id)
	return nil
}

// StartPush 开始推流
func (m *Manager) StartPush(id string) error {
	m.mutex.Lock()
	target, exists := m.targets[id]
	if !exists {
		m.mutex.Unlock()
		return errors.New("push target not found")
	}

	if target.Status == "pushing" {
		m.mutex.Unlock()
		return errors.New("already pushing")
	}
	m.mutex.Unlock()

	// 构建完整的推流地址
	dstURL := m.buildPushURL(target)
	if dstURL == "" {
		return errors.New("invalid push URL")
	}

	// 源流地址（ZLM 内部流）
	srcURL := target.SourceURL
	if srcURL == "" {
		return errors.New("source URL is empty")
	}

	debug.Info("push", "Starting push: %s -> %s", srcURL, dstURL)

	// 调用 ZLM 添加 FFmpeg 推流任务
	result, err := m.zlmClient.AddFFmpegSource(srcURL, dstURL, 10000, true)
	if err != nil {
		m.mutex.Lock()
		target.Status = "error"
		target.ErrorMsg = err.Error()
		target.UpdatedAt = time.Now()
		m.saveTargets()
		m.mutex.Unlock()
		return fmt.Errorf("failed to start push: %w", err)
	}

	m.mutex.Lock()
	target.FFmpegKey = result.Key
	target.Status = "pushing"
	target.StartTime = time.Now()
	target.ErrorMsg = ""
	target.UpdatedAt = time.Now()
	m.saveTargets()
	m.mutex.Unlock()

	debug.Info("push", "Push started successfully: %s (key: %s)", target.Name, result.Key)
	return nil
}

// StopPush 停止推流
func (m *Manager) StopPush(id string) error {
	m.mutex.Lock()
	target, exists := m.targets[id]
	if !exists {
		m.mutex.Unlock()
		return errors.New("push target not found")
	}

	if target.Status != "pushing" {
		m.mutex.Unlock()
		return nil
	}

	ffmpegKey := target.FFmpegKey
	m.mutex.Unlock()

	// 停止 FFmpeg 推流
	if ffmpegKey != "" {
		if err := m.zlmClient.DelFFmpegSource(ffmpegKey); err != nil {
			debug.Warn("push", "Failed to delete ffmpeg source: %v", err)
		}
	}

	m.mutex.Lock()
	target.Status = "stopped"
	target.FFmpegKey = ""
	target.UpdatedAt = time.Now()
	m.saveTargets()
	m.mutex.Unlock()

	debug.Info("push", "Push stopped: %s", target.Name)
	return nil
}

// RefreshStatus 刷新推流状态
func (m *Manager) RefreshStatus() error {
	// 获取当前所有 FFmpeg 任务
	ffmpegList, err := m.zlmClient.ListFFmpegSource()
	if err != nil {
		return err
	}

	// 创建 FFmpeg key 集合
	activeKeys := make(map[string]bool)
	for _, item := range ffmpegList {
		if key, ok := item["key"].(string); ok {
			activeKeys[key] = true
		}
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 更新状态
	for _, target := range m.targets {
		if target.Status == "pushing" {
			if target.FFmpegKey == "" || !activeKeys[target.FFmpegKey] {
				target.Status = "error"
				target.ErrorMsg = "Push task not found"
				target.UpdatedAt = time.Now()
			}
		}
	}

	m.saveTargets()
	return nil
}

// buildPushURL 构建推流地址
func (m *Manager) buildPushURL(target *PushTarget) string {
	if target.Platform == "custom" {
		return target.PushURL
	}

	// 查找平台模板
	for _, platform := range SupportedPlatforms {
		if platform.ID == target.Platform {
			if target.PushURL != "" && target.StreamKey != "" {
				return target.PushURL + "/" + target.StreamKey
			}
			return ""
		}
	}

	return ""
}

// loadTargets 加载推流目标
func (m *Manager) loadTargets() {
	if m.dataFile == "" {
		return
	}

	data, err := os.ReadFile(m.dataFile)
	if err != nil {
		if !os.IsNotExist(err) {
			debug.Warn("push", "Failed to load push targets: %v", err)
		}
		return
	}

	var targets []*PushTarget
	if err := json.Unmarshal(data, &targets); err != nil {
		debug.Warn("push", "Failed to parse push targets: %v", err)
		return
	}

	for _, target := range targets {
		// 重置运行状态
		target.Status = "stopped"
		target.FFmpegKey = ""
		m.targets[target.ID] = target
	}

	debug.Info("push", "Loaded %d push targets", len(targets))
}

// saveTargets 保存推流目标
func (m *Manager) saveTargets() {
	if m.dataFile == "" {
		return
	}

	targets := make([]*PushTarget, 0, len(m.targets))
	for _, target := range m.targets {
		targets = append(targets, target)
	}

	data, err := json.MarshalIndent(targets, "", "  ")
	if err != nil {
		debug.Warn("push", "Failed to marshal push targets: %v", err)
		return
	}

	if err := os.WriteFile(m.dataFile, data, 0644); err != nil {
		debug.Warn("push", "Failed to save push targets: %v", err)
	}
}
