package onvif

import (
	"encoding/xml"
	"fmt"
	"gb28181-onvif-server/internal/config"
	"gb28181-onvif-server/internal/debug"
	"log"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Manager ONVIF管理器结构体
type Manager struct {
	config        *config.ONVIFConfig
	devices       map[string]*Device
	devicesMux    sync.RWMutex
	stopChan      chan struct{}
	wsDiscovery   *WSDiscoveryService
	eventHandlers map[string][]EventHandler
	handlersMux   sync.RWMutex
}

// Device ONVIF设备结构体
type Device struct {
	DeviceID        string              // 设备ID (IP:Port)
	Name            string              // 设备名称
	Model           string              // 设备型号
	Manufacturer    string              // 制造商
	FirmwareVersion string              // 固件版本
	SerialNumber    string              // 序列号
	HardwareID      string              // 硬件ID
	IP              string              // IP地址
	Port            int                 // ONVIF端口
	ONVIFAddr       string              // ONVIF服务端点地址 (完整URL)
	SipPort         int                 // GB28181 SIP端口
	Username        string              // 用户名
	Password        string              // 密码
	Status          string              // 在线状态: online/offline/unknown
	DiscoveryTime   time.Time           // 发现时间
	LastSeenTime    time.Time           // 最后在线时间
	Services        []string            // ONVIF服务列表
	Capabilities    *DeviceCapabilities // 设备能力
	PreviewURL      string              // 预览流地址(RTSP)
	SnapshotURL     string              // 快照地址
	LastCheckTime   time.Time           // 最后状态检查时间
	CheckInterval   int                 // 检查间隔(秒)
	FailureCount    int                 // 连续失败次数
	ResponseTime    int64               // 响应时间(毫秒)
	Profiles        []MediaProfile      // 媒体配置文件列表
	PTZSupported    bool                // 是否支持PTZ
	AudioSupported  bool                // 是否支持音频
	Metadata        map[string]string   // 扩展元数据
}

// EventHandler 事件处理器
type EventHandler func(event DeviceEvent)

// DeviceEvent 设备事件
type DeviceEvent struct {
	Type      string      // 事件类型: online, offline, discovered, removed
	DeviceID  string      // 设备ID
	Device    *Device     // 设备信息
	Timestamp time.Time   // 事件时间
	Data      interface{} // 附加数据
}

// NewManager 创建ONVIF管理器实例
func NewManager(cfg *config.ONVIFConfig) *Manager {
	m := &Manager{
		config:        cfg,
		devices:       make(map[string]*Device),
		stopChan:      make(chan struct{}),
		eventHandlers: make(map[string][]EventHandler),
	}

	// 初始化WS-Discovery服务
	m.wsDiscovery = NewWSDiscoveryService(m)

	return m
}

// Start 启动ONVIF管理器
func (m *Manager) Start() error {
	log.Println("═══════════════════════════════════════════════════════════")
	log.Println("[ONVIF] ✓ ONVIF管理器启动成功")
	log.Printf("[ONVIF] 配置 - 发现间隔: %d秒", m.config.DiscoveryInterval)
	log.Println("═══════════════════════════════════════════════════════════")
	debug.Info("onvif", "ONVIF管理器启动")
	debug.Debug("onvif", "配置信息: 发现间隔=%d秒", m.config.DiscoveryInterval)

	// 启动WS-Discovery服务
	if err := m.wsDiscovery.Start(); err != nil {
		log.Printf("[ONVIF] [WARN] WS-Discovery服务启动失败: %v", err)
	}

	// 启动设备发现协程
	go m.deviceDiscoveryLoop()

	// 启动设备状态监控协程
	go m.deviceStatusMonitor()

	return nil
}

// Stop 停止ONVIF管理器
func (m *Manager) Stop() error {
	log.Println("[ONVIF] 正在停止ONVIF管理器...")

	// 停止WS-Discovery服务
	if m.wsDiscovery != nil {
		m.wsDiscovery.Stop()
	}

	close(m.stopChan)

	log.Println("[ONVIF] ✓ ONVIF管理器已停止")
	return nil
}

// RegisterEventHandler 注册事件处理器
func (m *Manager) RegisterEventHandler(eventType string, handler EventHandler) {
	m.handlersMux.Lock()
	defer m.handlersMux.Unlock()

	m.eventHandlers[eventType] = append(m.eventHandlers[eventType], handler)
}

// emitEvent 触发事件
func (m *Manager) emitEvent(event DeviceEvent) {
	m.handlersMux.RLock()
	handlers := m.eventHandlers[event.Type]
	allHandlers := m.eventHandlers["*"] // 通配符处理器
	m.handlersMux.RUnlock()

	for _, handler := range handlers {
		go handler(event)
	}
	for _, handler := range allHandlers {
		go handler(event)
	}
}

// deviceDiscoveryLoop 设备发现循环
func (m *Manager) deviceDiscoveryLoop() {
	log.Println("[ONVIF] 启动设备发现循环...")
	ticker := time.NewTicker(time.Duration(m.config.DiscoveryInterval) * time.Second)
	defer ticker.Stop()

	// 立即执行一次发现
	m.discoverDevices()

	for {
		select {
		case <-ticker.C:
			m.discoverDevices()
		case <-m.stopChan:
			log.Println("[ONVIF] 停止设备发现循环")
			return
		}
	}
}

// deviceStatusMonitor 设备状态监控
func (m *Manager) deviceStatusMonitor() {
	log.Println("[ONVIF] 启动设备状态监控...")
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.UpdateDeviceStatus()
		case <-m.stopChan:
			log.Println("[ONVIF] 停止设备状态监控")
			return
		}
	}
}

// DiscoverDevices 手动触发设备发现并返回结果
func (m *Manager) DiscoverDevices() ([]DeviceDiscoveryResult, error) {
	log.Println("[ONVIF] 正在执行手动设备发现...")

	if m.wsDiscovery == nil {
		return nil, fmt.Errorf("WS-Discovery服务未初始化")
	}

	discoveredDevices, err := m.wsDiscovery.Probe()
	if err != nil {
		return nil, fmt.Errorf("WS-Discovery探测失败: %w", err)
	}

	log.Printf("[ONVIF] ✓ 手动发现完成，找到 %d 个设备", len(discoveredDevices))
	return discoveredDevices, nil
}

// discoverDevices 发现ONVIF设备（内部定时调用）
func (m *Manager) discoverDevices() {
	log.Println("[ONVIF] 正在执行设备发现...")
	debug.Debug("onvif", "开始设备发现过程")

	// 使用WS-Discovery进行设备发现
	if m.wsDiscovery != nil {
		discoveredDevices, err := m.wsDiscovery.Probe()
		if err != nil {
			debug.Warn("onvif", "WS-Discovery探测失败: %v", err)
		} else {
			log.Printf("[ONVIF] WS-Discovery 发现了 %d 个设备", len(discoveredDevices))
			for _, result := range discoveredDevices {
				// 尝试自动添加发现的设备
				log.Printf("[ONVIF] 处理发现的设备: %s (XAddr: %s)", result.Name, result.XAddr)

				m.tryAutoAddDevice(result)
			}
		}
	}

	// 同时更新现有设备状态
	m.UpdateDeviceStatus()

	m.devicesMux.RLock()
	deviceCount := len(m.devices)
	m.devicesMux.RUnlock()

	if deviceCount > 0 {
		log.Printf("[ONVIF] ✓ 当前管理 %d 个ONVIF设备", deviceCount)
	}
	debug.Debug("onvif", "设备发现完成，设备数: %d", deviceCount)
}

// tryAutoAddDevice 尝试自动添加发现的设备
func (m *Manager) tryAutoAddDevice(result DeviceDiscoveryResult) {
	// 解析设备地址
	host, port, err := ParseXAddr(result.XAddr)
	if err != nil {
		log.Printf("[ONVIF] [ERROR] 解析发现的设备地址失败: %v (XAddr: %s)", err, result.XAddr)
		debug.Debug("onvif", "解析发现的设备地址失败: %v", err)
		return
	}

	deviceID := fmt.Sprintf("%s:%d", host, port)

	// 检查设备是否已存在
	m.devicesMux.RLock()
	_, exists := m.devices[deviceID]
	m.devicesMux.RUnlock()

	if exists {
		log.Printf("[ONVIF] 设备已存在，跳过: %s", deviceID)
		return // 设备已存在，跳过
	}

	// 创建新设备记录（未验证状态）
	device := &Device{
		DeviceID:      deviceID,
		Name:          result.Name,
		Model:         result.Model,
		Manufacturer:  result.Manufacturer,
		IP:            host,
		Port:          port, // ONVIF Port
		SipPort:       5060, // 默认SIP端口
		Status:        "discovered",
		DiscoveryTime: time.Now(),
		Services:      result.Types,
		Metadata:      result.Extras,
		CheckInterval: 60,
	}

	if device.Name == "" {
		device.Name = fmt.Sprintf("ONVIF Camera (%s)", host)
	}

	log.Printf("[ONVIF] 开始处理发现的设备: %s (%s:%d)", device.Name, host, port)

	// 立即添加基本设备信息
	m.devicesMux.Lock()
	m.devices[deviceID] = device
	m.devicesMux.Unlock()

	log.Printf("[ONVIF] ✅ 已将发现的设备添加到列表: %s (%s) | 状态: discovered", device.Name, device.DeviceID)

	// 异步获取设备详细信息（用于完善设备信息）
	go func() {
		// 使用 WS-Discovery 返回的原始 XAddr，如果无法连接再尝试备选端点
		xaddr := result.XAddr

		if xaddr == "" {
			// 备选：如果没有 XAddr，使用默认路径
			xaddr = fmt.Sprintf("http://%s:%d/onvif/device_service", host, port)
		}
		// 如果没有提供认证信息，尝试空认证和常见默认凭证
		detailedDevice, err := m.getDeviceDetails(xaddr, "", "")
		if err == nil && detailedDevice != nil {
			// 使用详细信息更新设备
			m.devicesMux.Lock()
			detailedDevice.DiscoveryTime = device.DiscoveryTime
			if detailedDevice.Name == "" {
				detailedDevice.Name = device.Name
			}
			m.devices[deviceID] = detailedDevice
			m.devicesMux.Unlock()
			log.Printf("[ONVIF] ✅ 已获取设备详细信息: %s (%s) | 状态: %s", detailedDevice.Name, detailedDevice.DeviceID, detailedDevice.Status)
		} else {
			// 即使获取详细信息失败，设备仍然已被添加到列表
			// 打印真实错误原因（包括已尝试的端点和凭证数量）
			log.Printf("[ONVIF] ⚠️ 获取设备详细信息失败: %s (%s) | 原因: %v | 但基本信息已添加", device.Name, device.DeviceID, err)
		}

		// 触发设备发现事件
		m.emitEvent(DeviceEvent{
			Type:      "discovered",
			DeviceID:  deviceID,
			Device:    device,
			Timestamp: time.Now(),
		})
	}()
}

// getDeviceDetails 获取设备详细信息，带有重试逻辑
func (m *Manager) getDeviceDetails(xaddr, username, password string) (*Device, error) {
	// 如果没有提供认证信息，尝试常见的默认凭证
	credentialsList := []struct {
		username string
		password string
	}{
		{username, password}, // 首先尝试提供的凭证
	}

	// 如果提供的用户名为空，尝试常见默认凭证
	if username == "" {
		credentialsList = append(credentialsList,
			struct {
				username string
				password string
			}{"admin", "admin"},
			struct {
				username string
				password string
			}{"admin", "12345"},
			struct {
				username string
				password string
			}{"admin", "123456"},
			struct {
				username string
				password string
			}{"root", "root"},
			struct {
				username string
				password string
			}{"root", "12345"},
		)
	}

	// 创建备选ONVIF端点列表
	var xaddrs []string

	// 第一优先级：使用传入的 xaddr (WS-Discovery 返回的)
	if xaddr != "" {
		xaddrs = append(xaddrs, xaddr)
	}

	// 从 xaddr 中解析 IP 和端口，用于生成备用端点
	ip := "127.0.0.1"
	port := 80

	parsedURL, err := url.Parse(xaddr)
	if err == nil {
		ip = parsedURL.Hostname()
		port = 80
		if parsedURL.Port() != "" {
			p, err := strconv.Atoi(parsedURL.Port())
			if err == nil {
				port = p
			}
		}
	}

	// 只有当WS-Discovery返回的XAddr失败时，才尝试这些备用端点
	// 这些是常见的ONVIF路径和端口组合
	backupXaddrs := []string{
		// 常见 HTTP 路径（优先级从高到低）
		fmt.Sprintf("http://%s:%d/onvif/device_service", ip, port),
		fmt.Sprintf("http://%s/onvif/device_service", ip),
		fmt.Sprintf("http://%s:%d/ONVIF/device_service", ip, port),
		fmt.Sprintf("http://%s:%d/onvif/Device", ip, port),
		fmt.Sprintf("http://%s:%d/onvif", ip, port),
		fmt.Sprintf("http://%s:80/onvif/device_service", ip),
		fmt.Sprintf("http://%s:8080/onvif/device_service", ip),
		fmt.Sprintf("http://%s:8000/onvif/device_service", ip),
	}

	// 只添加不重复的备用地址
	for _, addr := range backupXaddrs {
		if addr != xaddr && !contains(xaddrs, addr) {
			xaddrs = append(xaddrs, addr)
		}
	}

	// 尝试连接到每个备选端点，使用多个凭证
	var deviceClient *ONVIFDeviceClient
	var successAddr string

	log.Printf("[ONVIF] 🔍 开始连接设备，待尝试的 xaddr 列表 (%d 个):", len(xaddrs))
	for i, addr := range xaddrs {
		log.Printf("[ONVIF]   [%d] %s", i+1, addr)
	}

	for _, cred := range credentialsList {
		for _, tryAddr := range xaddrs {
			// 创建ONVIF设备客户端
			d, err := NewDevice(DeviceParams{
				Xaddr:    tryAddr,
				Username: cred.username,
				Password: cred.password,
				Timeout:  10 * time.Second,
			})
			if err != nil {
				continue
			}

			// 测试设备连接
			if err := d.TestConnection(); err != nil {
				continue
			}

			// 成功连接
			log.Printf("[ONVIF] ✅ 成功连接到设备: %s (使用认证: %s)", tryAddr, cred.username)
			deviceClient = d
			successAddr = tryAddr
			break
		}

		// 如果此凭证成功，不再尝试其他凭证
		if successAddr != "" {
			break
		}
	}

	// 如果所有尝试都失败
	if successAddr == "" {
		return nil, fmt.Errorf("无法连接到ONVIF设备 (已尝试 %d 个端点和 %d 组凭证)", len(xaddrs), len(credentialsList))
	}

	// 获取设备服务列表
	var services []string
	servicesMap := deviceClient.GetServices()
	for serviceName, serviceAddr := range servicesMap {
		log.Printf("[ONVIF]   - %s: %s", serviceName, serviceAddr)
		services = append(services, serviceAddr)
	}

	// 获取设备信息
	deviceInfo, _ := deviceClient.GetDeviceInfo()

	// 获取设备能力
	capabilities := deviceClient.GetCapabilities()

	// 获取媒体配置文件
	profiles, _ := deviceClient.GetMediaProfiles()

	// 获取主码流URL
	var previewURL string
	if len(profiles) > 0 {
		previewURL, _ = deviceClient.GetStreamURI(profiles[0].Token)
	}

	// 获取快照URL
	var snapshotURL string
	if len(profiles) > 0 {
		snapshotURL, _ = deviceClient.GetSnapshotURI(profiles[0].Token)
	}

	device := &Device{
		DeviceID:        fmt.Sprintf("%s:%d", ip, port),
		Name:            fmt.Sprintf("ONVIF Camera (%s)", ip),
		Model:           deviceInfo["Model"],
		Manufacturer:    deviceInfo["Manufacturer"],
		FirmwareVersion: deviceInfo["FirmwareVersion"],
		SerialNumber:    deviceInfo["SerialNumber"],
		HardwareID:      deviceInfo["HardwareId"],
		IP:              ip,
		Port:            port,
		SipPort:         5060, // 默认SIP端口
		Username:        username,
		Password:        password,
		Status:          "online",
		DiscoveryTime:   time.Now(),
		LastSeenTime:    time.Now(),
		Services:        services,
		Capabilities:    capabilities,
		PreviewURL:      previewURL,
		SnapshotURL:     snapshotURL,
		LastCheckTime:   time.Now(),
		CheckInterval:   60,
		FailureCount:    0,
		ResponseTime:    0,
		Profiles:        profiles,
		PTZSupported:    capabilities != nil && capabilities.HasPTZ,
		AudioSupported:  false,
		Metadata:        make(map[string]string),
		ONVIFAddr:       successAddr,
	}

	return device, nil
}

// GetDevices 获取所有ONVIF设备
func (m *Manager) GetDevices() []*Device {
	m.devicesMux.RLock()
	defer m.devicesMux.RUnlock()

	devices := make([]*Device, 0, len(m.devices))
	for _, device := range m.devices {
		devices = append(devices, device)
	}

	return devices
}

// GetDeviceByID 根据ID获取ONVIF设备
func (m *Manager) GetDeviceByID(deviceID string) (*Device, bool) {
	m.devicesMux.RLock()
	defer m.devicesMux.RUnlock()

	device, exists := m.devices[deviceID]
	return device, exists
}

// StartStream 启动媒体流
func (m *Manager) StartStream(deviceID, profileToken string) (string, error) {
	device, exists := m.GetDeviceByID(deviceID)
	if !exists {
		return "", fmt.Errorf("设备不存在: %s", deviceID)
	}

	// 创建ONVIF设备客户端
	xaddr := m.getONVIFAddr(device)
	d, err := NewDevice(DeviceParams{
		Xaddr:    xaddr,
		Username: device.Username,
		Password: device.Password,
	})
	if err != nil {
		return "", fmt.Errorf("创建设备客户端失败: %w", err)
	}

	// 如果没有指定profileToken，使用第一个配置文件
	if profileToken == "" && len(device.Profiles) > 0 {
		profileToken = device.Profiles[0].Token
	}

	// 获取流URI
	streamURL, err := d.GetStreamURI(profileToken)
	if err != nil {
		// 回退到简化格式
		streamURL = fmt.Sprintf("rtsp://%s:%s@%s:554/Streaming/Channels/101",
			device.Username, device.Password, device.IP)
	}

	debug.Info("onvif", "启动媒体流: 设备=%s, Profile=%s, URL=%s", deviceID, profileToken, streamURL)
	return streamURL, nil
}

// StartDiscovery 启动设备发现
func (m *Manager) StartDiscovery(duration time.Duration) {
	go func() {
		time.Sleep(duration)
	}()
	m.discoverDevices()
}

// GetStreamURI 别名：获取设备流地址
func (m *Manager) GetStreamURI(deviceID, profileToken string) (string, error) {
	return m.StartStream(deviceID, profileToken)
}

// UpdateDevicePreview 更新设备预览信息
func (m *Manager) UpdateDevicePreview(deviceID, previewURL, snapshotURL string) error {
	device, exists := m.GetDeviceByID(deviceID)
	if !exists {
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	if previewURL != "" {
		device.PreviewURL = previewURL
	} else {
		previewURL, _ := m.getDevicePreviewURL(device)
		if previewURL != "" {
			device.PreviewURL = previewURL
		}
	}

	if snapshotURL != "" {
		device.SnapshotURL = snapshotURL
	} else {
		snapshotURL, _ := m.getDeviceSnapshotURL(device)
		if snapshotURL != "" {
			device.SnapshotURL = snapshotURL
		}
	}

	return nil
}

// ContinuousMove PTZ连续移动
func (m *Manager) ContinuousMove(deviceID, profileToken, command string, speed float64) error {
	return m.PTZControl(deviceID, command, speed)
}

// StopPTZ 停止PTZ
func (m *Manager) StopPTZ(deviceID, profileToken string) error {
	return m.PTZControl(deviceID, "stop", 0)
}

// SetPreset 设置预置位
func (m *Manager) SetPreset(deviceID, profileToken, presetName, presetToken string) (string, error) {
	return m.SetPTZPreset(deviceID, presetName)
}

// GotoPreset 移动到预置位
func (m *Manager) GotoPreset(deviceID, profileToken, presetToken string, speed float64) error {
	return m.PTZGotoPreset(deviceID, presetToken)
}

// RemovePreset 删除预置位
func (m *Manager) RemovePreset(deviceID, profileToken, presetToken string) error {
	device, exists := m.GetDeviceByID(deviceID)
	if !exists {
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	xaddr := fmt.Sprintf("http://%s:%d/onvif/device_service", device.IP, device.Port)
	d, err := NewDevice(DeviceParams{
		Xaddr:    xaddr,
		Username: device.Username,
		Password: device.Password,
	})
	if err != nil {
		return fmt.Errorf("创建设备客户端失败: %w", err)
	}

	if profileToken == "" && len(device.Profiles) > 0 {
		profileToken = device.Profiles[0].Token
	}

	debug.Info("onvif", "删除预置位: 设备=%s, 预置位=%s", deviceID, presetToken)
	// 删除预置位功能实现
	if d.client != nil {
		return d.client.RemovePreset(profileToken, presetToken)
	}
	return nil
}

// GetPresets 获取预置位列表
func (m *Manager) GetPresets(deviceID, profileToken string) ([]PTZPreset, error) {
	device, exists := m.GetDeviceByID(deviceID)
	if !exists {
		return nil, fmt.Errorf("设备不存在: %s", deviceID)
	}

	if profileToken == "" && len(device.Profiles) > 0 {
		profileToken = device.Profiles[0].Token
	}

	return m.GetPTZPresets(deviceID)
}

// GetSnapshotURI 获取设备快照地址
func (m *Manager) GetSnapshotURI(deviceID, profileToken string) (string, error) {
	device, exists := m.GetDeviceByID(deviceID)
	if !exists {
		return "", fmt.Errorf("设备不存在: %s", deviceID)
	}

	// 如果没有profileToken，使用第一个配置文件
	if profileToken == "" && len(device.Profiles) > 0 {
		profileToken = device.Profiles[0].Token
	}

	// 创建ONVIF设备客户端
	xaddr := fmt.Sprintf("http://%s:%d/onvif/device_service", device.IP, device.Port)
	d, err := NewDevice(DeviceParams{
		Xaddr:    xaddr,
		Username: device.Username,
		Password: device.Password,
	})
	if err != nil {
		return "", fmt.Errorf("创建设备客户端失败: %w", err)
	}

	return d.GetSnapshotURI(profileToken)
}

// UpdateDeviceCredentials 更新设备凭据
func (m *Manager) UpdateDeviceCredentials(deviceID, username, password string) error {
	device, exists := m.GetDeviceByID(deviceID)
	if !exists {
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	// 如果提供了用户名或密码，更新它们
	if username != "" {
		device.Username = username
	}
	if password != "" {
		device.Password = password
	}

	// 验证新的凭据
	xaddr := fmt.Sprintf("http://%s:%d/onvif/device_service", device.IP, device.Port)
	d, err := NewDevice(DeviceParams{
		Xaddr:    xaddr,
		Username: device.Username,
		Password: device.Password,
	})
	if err != nil {
		// 恢复原来的凭据
		return fmt.Errorf("新凭据验证失败: %w", err)
	}

	// 验证连接
	if err := d.TestConnection(); err != nil {
		return fmt.Errorf("设备连接测试失败: %w", err)
	}

	log.Printf("[ONVIF] ✓ 设备凭据已更新: %s", deviceID)
	return nil
}

// GetDeviceStatus 获取设备状态
func (m *Manager) GetDeviceStatus(deviceID string) (string, error) {
	device, exists := m.GetDeviceByID(deviceID)
	if !exists {
		return "", fmt.Errorf("设备不存在: %s", deviceID)
	}
	return device.Status, nil
}

// GetStats 获取统计信息
func (m *Manager) GetStats() map[string]interface{} {
	return m.GetDeviceStatistics()
}

// GetVideoEncoderConfigurations 获取视频编码配置
func (m *Manager) GetVideoEncoderConfigurations(deviceID, profileToken string) ([]map[string]interface{}, error) {
	device, exists := m.GetDeviceByID(deviceID)
	if !exists {
		return nil, fmt.Errorf("设备不存在: %s", deviceID)
	}

	// 如果没有指定profileToken，使用第一个配置文件
	if profileToken == "" && len(device.Profiles) > 0 {
		profileToken = device.Profiles[0].Token
	}

	// 创建ONVIF设备客户端
	xaddr := fmt.Sprintf("http://%s:%d/onvif/device_service", device.IP, device.Port)
	d, err := NewDevice(DeviceParams{
		Xaddr:    xaddr,
		Username: device.Username,
		Password: device.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("创建设备客户端失败: %w", err)
	}

	// 获取视频编码配置
	configs, err := d.client.GetVideoEncoderConfigurations(profileToken)
	if err != nil {
		return nil, fmt.Errorf("获取视频编码配置失败: %w", err)
	}

	// 转换为map格式
	result := make([]map[string]interface{}, len(configs))
	for i, cfg := range configs {
		result[i] = map[string]interface{}{
			"token":        cfg.Token,
			"name":         cfg.Name,
			"encoding":     cfg.Encoding,
			"width":        cfg.Width,
			"height":       cfg.Height,
			"quality":      cfg.Quality,
			"frameRate":    cfg.FrameRate,
			"bitrateLimit": cfg.BitrateLimit,
			"h264Profile":  cfg.H264Profile,
		}
	}

	return result, nil
}

// GetSnapshotURL 获取设备快照地址
func (m *Manager) GetSnapshotURL(deviceID, profileToken string) (string, error) {
	device, exists := m.GetDeviceByID(deviceID)
	if !exists {
		return "", fmt.Errorf("设备不存在: %s", deviceID)
	}

	// 如果已有快照URL，直接返回
	if device.SnapshotURL != "" && profileToken == "" {
		return device.SnapshotURL, nil
	}

	// 创建ONVIF设备客户端获取快照URL
	xaddr := fmt.Sprintf("http://%s:%d/onvif/device_service", device.IP, device.Port)
	d, err := NewDevice(DeviceParams{
		Xaddr:    xaddr,
		Username: device.Username,
		Password: device.Password,
	})
	if err != nil {
		return "", fmt.Errorf("创建设备客户端失败: %w", err)
	}

	if profileToken == "" && len(device.Profiles) > 0 {
		profileToken = device.Profiles[0].Token
	}

	return d.GetSnapshotURI(profileToken)
}

// GetSnapshot 获取设备快照数据
func (m *Manager) GetSnapshot(deviceID, profileToken string) ([]byte, string, error) {
	device, exists := m.GetDeviceByID(deviceID)
	if !exists {
		return nil, "", fmt.Errorf("设备不存在: %s", deviceID)
	}

	xaddr := fmt.Sprintf("http://%s:%d/onvif/device_service", device.IP, device.Port)
	d, err := NewDevice(DeviceParams{
		Xaddr:    xaddr,
		Username: device.Username,
		Password: device.Password,
	})
	if err != nil {
		return nil, "", fmt.Errorf("创建设备客户端失败: %w", err)
	}

	if profileToken == "" && len(device.Profiles) > 0 {
		profileToken = device.Profiles[0].Token
	}

	return d.GetSnapshot(profileToken)
}

// AddDevice 添加ONVIF设备（支持多种地址格式）
func (m *Manager) AddDevice(xaddr, username, password string) (*Device, error) {
	// 规范化地址格式
	xaddr = normalizeXAddr(xaddr)

	log.Printf("[ONVIF] 📝 手动添加设备: %s", xaddr)

	// 验证地址格式
	if xaddr == "" {
		return nil, fmt.Errorf("设备地址不能为空")
	}

	// 获取设备详细信息
	device, err := m.getDeviceDetails(xaddr, username, password)
	if err != nil {
		log.Printf("[ONVIF] [ERROR] 添加设备失败: %v", err)
		return nil, fmt.Errorf("获取设备信息失败: %w", err)
	}

	// 保存设备到设备列表
	m.devicesMux.Lock()
	defer m.devicesMux.Unlock()

	// 检查是否已存在
	if existingDevice, exists := m.devices[device.DeviceID]; exists {
		// 更新现有设备信息
		existingDevice.Username = username
		existingDevice.Password = password
		existingDevice.Status = "online"
		existingDevice.LastSeenTime = time.Now()
		log.Printf("[ONVIF] ✓ 设备信息已更新: %s", device.DeviceID)
		return existingDevice, nil
	}

	m.devices[device.DeviceID] = device

	log.Printf("[ONVIF] ✓ 设备添加成功: ID=%s | 名称=%s | 地址=%s:%d | 型号=%s",
		device.DeviceID, device.Name, device.IP, device.Port, device.Model)
	debug.Info("onvif", "添加ONVIF设备成功: %s", device.DeviceID)

	// 触发设备添加事件
	m.emitEvent(DeviceEvent{
		Type:      "added",
		DeviceID:  device.DeviceID,
		Device:    device,
		Timestamp: time.Now(),
	})

	return device, nil
}

// AddDeviceWithIP 通过 IP 和端口添加设备（支持多网卡场景）
func (m *Manager) AddDeviceWithIP(ip string, port int, username, password string) (*Device, error) {
	// 验证 IP 地址有效性
	if !ValidateIPAddress(ip) {
		return nil, fmt.Errorf("无效的IP地址: %s", ip)
	}

	if !ValidatePort(port) {
		return nil, fmt.Errorf("无效的端口: %d", port)
	}

	// 构建 XADDR
	xaddr := fmt.Sprintf("http://%s:%d/onvif/device_service", ip, port)

	log.Printf("[ONVIF] 📝 通过IP添加设备: %s:%d", ip, port)

	return m.AddDevice(xaddr, username, password)
}

// VerifyDeviceCredentials 验证设备的用户名和密码是否正确
func (m *Manager) VerifyDeviceCredentials(ip string, port int, username, password string) error {
	xaddr := fmt.Sprintf("http://%s:%d/onvif/device_service", ip, port)
	log.Printf("[ONVIF] 🔐 正在验证设备凭据: %s", xaddr)

	// 创建一个临时的ONVIF设备客户端进行测试
	d, err := NewDevice(DeviceParams{
		Xaddr:    xaddr,
		Username: username,
		Password: password,
		Timeout:  10 * time.Second, // 使用一个合理的超时时间
	})
	if err != nil {
		return fmt.Errorf("创建设备客户端失败: %w", err)
	}

	// 调用一个需要认证的简单方法来测试凭据。
	// GetSystemDateAndTime 是一个很好的选择，因为它很轻量。
	_, err = d.GetSystemDateAndTime()
	if err != nil {
		// 检查返回的错误是否明确指示认证失败
		errMsg := strings.ToLower(err.Error())
		if strings.Contains(errMsg, "auth") || strings.Contains(errMsg, "not authorized") || strings.Contains(errMsg, "unauthorized") {
			return fmt.Errorf("凭据无效：用户名或密码错误")
		}

		return fmt.Errorf("无法验证设备凭据: %w", err)
	}
	return nil // err 为 nil，表示验证成功
}

// RemoveDevice 移除ONVIF设备
func (m *Manager) RemoveDevice(deviceID string) error {
	m.devicesMux.Lock()
	defer m.devicesMux.Unlock()

	device, exists := m.devices[deviceID]
	if !exists {
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	delete(m.devices, deviceID)
	debug.Info("onvif", "移除ONVIF设备成功: %s", deviceID)
	log.Printf("[ONVIF] ✓ 设备已移除: %s", deviceID)

	// 触发设备移除事件
	m.emitEvent(DeviceEvent{
		Type:      "removed",
		DeviceID:  deviceID,
		Device:    device,
		Timestamp: time.Now(),
	})

	return nil
}

// GetProfiles 获取设备媒体配置文件
func (m *Manager) GetProfiles(deviceID string) ([]map[string]interface{}, error) {
	device, exists := m.GetDeviceByID(deviceID)
	if !exists {
		return nil, fmt.Errorf("设备不存在: %s", deviceID)
	}

	// 创建ONVIF设备客户端
	xaddr := fmt.Sprintf("http://%s:%d/onvif/device_service", device.IP, device.Port)
	d, err := NewDevice(DeviceParams{
		Xaddr:    xaddr,
		Username: device.Username,
		Password: device.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("创建设备客户端失败: %w", err)
	}

	// 获取媒体配置文件
	mediaProfiles, err := d.GetMediaProfiles()
	if err != nil {
		return nil, fmt.Errorf("获取媒体配置文件失败: %w", err)
	}

	// 转换为map格式
	profiles := make([]map[string]interface{}, len(mediaProfiles))
	for i, profile := range mediaProfiles {
		profileMap := map[string]interface{}{
			"token":      profile.Token,
			"name":       profile.Name,
			"encoding":   profile.Encoding,
			"resolution": profile.Resolution,
			"width":      profile.Width,
			"height":     profile.Height,
			"fps":        profile.FPS,
			"bitrate":    profile.Bitrate,
		}

		if profile.VideoEncoder != nil {
			profileMap["videoEncoder"] = map[string]interface{}{
				"token":        profile.VideoEncoder.Token,
				"name":         profile.VideoEncoder.Name,
				"encoding":     profile.VideoEncoder.Encoding,
				"width":        profile.VideoEncoder.Width,
				"height":       profile.VideoEncoder.Height,
				"quality":      profile.VideoEncoder.Quality,
				"frameRate":    profile.VideoEncoder.FrameRate,
				"bitrateLimit": profile.VideoEncoder.BitrateLimit,
				"h264Profile":  profile.VideoEncoder.H264Profile,
			}
		}

		if profile.PTZConfig != nil {
			profileMap["ptzConfig"] = map[string]interface{}{
				"token":     profile.PTZConfig.Token,
				"name":      profile.PTZConfig.Name,
				"nodeToken": profile.PTZConfig.NodeToken,
			}
		}

		profiles[i] = profileMap
	}

	// 更新设备的配置文件缓存
	m.devicesMux.Lock()
	if dev, ok := m.devices[deviceID]; ok {
		dev.Profiles = mediaProfiles
	}
	m.devicesMux.Unlock()

	return profiles, nil
}

// PTZControl 控制设备PTZ
func (m *Manager) PTZControl(deviceID, command string, speed float64) error {
	device, exists := m.GetDeviceByID(deviceID)
	if !exists {
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	// 创建ONVIF设备客户端
	xaddr := fmt.Sprintf("http://%s:%d/onvif/device_service", device.IP, device.Port)
	d, err := NewDevice(DeviceParams{
		Xaddr:    xaddr,
		Username: device.Username,
		Password: device.Password,
	})
	if err != nil {
		return fmt.Errorf("创建设备客户端失败: %w", err)
	}

	// 获取默认配置文件Token
	profileToken := "main_profile"
	if len(device.Profiles) > 0 {
		profileToken = device.Profiles[0].Token
	}

	// 根据命令执行不同的PTZ操作
	var velocity *PTZVector
	switch strings.ToLower(command) {
	case "up":
		velocity = &PTZVector{PanTilt: &Vector2D{X: 0, Y: speed}}
	case "down":
		velocity = &PTZVector{PanTilt: &Vector2D{X: 0, Y: -speed}}
	case "left":
		velocity = &PTZVector{PanTilt: &Vector2D{X: -speed, Y: 0}}
	case "right":
		velocity = &PTZVector{PanTilt: &Vector2D{X: speed, Y: 0}}
	case "zoomin":
		velocity = &PTZVector{Zoom: &Vector1D{X: speed}}
	case "zoomout":
		velocity = &PTZVector{Zoom: &Vector1D{X: -speed}}
	case "stop":
		return d.PTZStop(profileToken, true, true)
	case "home":
		return d.GotoHomePosition(profileToken, nil)
	default:
		return fmt.Errorf("未知的PTZ命令: %s", command)
	}

	debug.Info("onvif", "PTZ控制: 设备=%s, 命令=%s, 速度=%.2f", deviceID, command, speed)
	return d.PTZContinuousMove(profileToken, velocity, 0)
}

// PTZGotoPreset 移动到预置位
func (m *Manager) PTZGotoPreset(deviceID, presetToken string) error {
	device, exists := m.GetDeviceByID(deviceID)
	if !exists {
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	xaddr := fmt.Sprintf("http://%s:%d/onvif/device_service", device.IP, device.Port)
	d, err := NewDevice(DeviceParams{
		Xaddr:    xaddr,
		Username: device.Username,
		Password: device.Password,
	})
	if err != nil {
		return fmt.Errorf("创建设备客户端失败: %w", err)
	}

	profileToken := "main_profile"
	if len(device.Profiles) > 0 {
		profileToken = device.Profiles[0].Token
	}

	debug.Info("onvif", "PTZ移动到预置位: 设备=%s, 预置位=%s", deviceID, presetToken)
	return d.GotoPreset(profileToken, presetToken, nil)
}

// GetPTZPresets 获取PTZ预置位列表
func (m *Manager) GetPTZPresets(deviceID string) ([]PTZPreset, error) {
	device, exists := m.GetDeviceByID(deviceID)
	if !exists {
		return nil, fmt.Errorf("设备不存在: %s", deviceID)
	}

	xaddr := fmt.Sprintf("http://%s:%d/onvif/device_service", device.IP, device.Port)
	d, err := NewDevice(DeviceParams{
		Xaddr:    xaddr,
		Username: device.Username,
		Password: device.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("创建设备客户端失败: %w", err)
	}

	profileToken := "main_profile"
	if len(device.Profiles) > 0 {
		profileToken = device.Profiles[0].Token
	}

	return d.GetPTZPresets(profileToken)
}

// SetPTZPreset 设置PTZ预置位
func (m *Manager) SetPTZPreset(deviceID, presetName string) (string, error) {
	device, exists := m.GetDeviceByID(deviceID)
	if !exists {
		return "", fmt.Errorf("设备不存在: %s", deviceID)
	}

	xaddr := fmt.Sprintf("http://%s:%d/onvif/device_service", device.IP, device.Port)
	d, err := NewDevice(DeviceParams{
		Xaddr:    xaddr,
		Username: device.Username,
		Password: device.Password,
	})
	if err != nil {
		return "", fmt.Errorf("创建设备客户端失败: %w", err)
	}

	profileToken := "main_profile"
	if len(device.Profiles) > 0 {
		profileToken = device.Profiles[0].Token
	}

	debug.Info("onvif", "设置PTZ预置位: 设备=%s, 名称=%s", deviceID, presetName)
	return d.SetPreset(profileToken, presetName, "")
}

// StopStream 停止媒体流
func (m *Manager) StopStream(deviceID string) error {
	_, exists := m.GetDeviceByID(deviceID)
	if !exists {
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	debug.Info("onvif", "停止ONVIF设备流: %s", deviceID)
	// 实际实现应该通过媒体服务停止流
	return nil
}

// UpdateDeviceStatus 更新设备状态 - 周期性检查所有设备的在线状态
func (m *Manager) UpdateDeviceStatus() {
	m.devicesMux.RLock()
	deviceList := make([]*Device, 0, len(m.devices))
	for _, device := range m.devices {
		deviceList = append(deviceList, device)
	}
	m.devicesMux.RUnlock()

	if len(deviceList) == 0 {
		return
	}

	debug.Debug("onvif", "开始检查 %d 个设备的状态", len(deviceList))

	// 使用WaitGroup等待所有设备检查完成
	var wg sync.WaitGroup
	for _, device := range deviceList {
		wg.Add(1)
		go func(d *Device) {
			defer wg.Done()
			m.checkDeviceStatus(d)
		}(device)
	}
	wg.Wait()
}

// checkDeviceStatus 检查单个设备的在线状态和获取预览URL
func (m *Manager) checkDeviceStatus(device *Device) {
	now := time.Now()

	// 设置默认检查间隔为60秒
	if device.CheckInterval <= 0 {
		device.CheckInterval = 60
	}

	// 检查间隔未到，跳过
	if !device.LastCheckTime.IsZero() &&
		device.LastCheckTime.Add(time.Duration(device.CheckInterval)*time.Second).After(now) {
		return
	}

	// 记录检查开始时间
	start := time.Now()
	xaddr := fmt.Sprintf("http://%s:%d/onvif/device_service", device.IP, device.Port)

	// 尝试连接设备ONVIF服务
	onvifDev, err := NewDevice(DeviceParams{
		Xaddr:    xaddr,
		Username: device.Username,
		Password: device.Password,
		Timeout:  5 * time.Second,
	})

	// 记录响应时间
	device.ResponseTime = time.Since(start).Milliseconds()
	device.LastCheckTime = now

	if err != nil {
		m.handleDeviceOffline(device, err)
		return
	}

	// 测试连接
	if err := onvifDev.TestConnection(); err != nil {
		m.handleDeviceOffline(device, err)
		return
	}

	// 设备在线
	previousStatus := device.Status
	device.Status = "online"
	device.FailureCount = 0
	device.LastSeenTime = now

	// 更新设备信息
	if info, err := onvifDev.GetDeviceInfo(); err == nil {
		if mfr, ok := info["Manufacturer"]; ok && mfr != "" {
			device.Manufacturer = mfr
		}
		if model, ok := info["Model"]; ok && model != "" {
			device.Model = model
		}
		if fw, ok := info["FirmwareVersion"]; ok && fw != "" {
			device.FirmwareVersion = fw
		}
		if sn, ok := info["SerialNumber"]; ok && sn != "" {
			device.SerialNumber = sn
		}
	}

	// 更新设备能力
	device.Capabilities = onvifDev.GetCapabilities()
	if device.Capabilities != nil {
		device.PTZSupported = device.Capabilities.HasPTZ
	}

	// 获取预览URL（异步，不阻塞主流程）
	go func() {
		if previewURL, err := m.getDevicePreviewURL(device); err == nil {
			device.PreviewURL = previewURL
		}
		if snapshotURL, err := m.getDeviceSnapshotURL(device); err == nil {
			device.SnapshotURL = snapshotURL
		}
	}()

	// 如果设备刚刚上线，触发事件
	if previousStatus != "online" {
		log.Printf("[ONVIF] ✓ 设备上线: %s (%s:%d) | 响应时间: %dms",
			device.Name, device.IP, device.Port, device.ResponseTime)
		m.emitEvent(DeviceEvent{
			Type:      "online",
			DeviceID:  device.DeviceID,
			Device:    device,
			Timestamp: now,
		})
	}

	debug.Debug("onvif", "设备状态检查成功: %s - 在线, 响应时间%dms",
		device.Name, device.ResponseTime)
}

// handleDeviceOffline 处理设备离线
func (m *Manager) handleDeviceOffline(device *Device, err error) {
	device.FailureCount++
	previousStatus := device.Status

	if device.FailureCount >= 3 {
		device.Status = "offline"
		if previousStatus != "offline" {
			log.Printf("[ONVIF] 📵 设备离线: %s (%s:%d) | 错误: %v",
				device.Name, device.IP, device.Port, err)
			m.emitEvent(DeviceEvent{
				Type:      "offline",
				DeviceID:  device.DeviceID,
				Device:    device,
				Timestamp: time.Now(),
				Data:      err.Error(),
			})
		}
	} else {
		device.Status = "unknown"
	}

	debug.Warn("onvif", "检查设备失败[%d次]: %s (%s:%d) - %v",
		device.FailureCount, device.Name, device.IP, device.Port, err)
}

// getDevicePreviewURL 获取设备RTSP预览地址
func (m *Manager) getDevicePreviewURL(device *Device) (string, error) {
	xaddr := fmt.Sprintf("http://%s:%d/onvif/device_service", device.IP, device.Port)

	// 连接设备获取详细信息
	onvifDev, err := NewDevice(DeviceParams{
		Xaddr:    xaddr,
		Username: device.Username,
		Password: device.Password,
	})
	if err != nil {
		return "", fmt.Errorf("连接设备失败: %w", err)
	}

	// 获取默认配置文件Token
	profileToken := "main_profile"
	if len(device.Profiles) > 0 {
		profileToken = device.Profiles[0].Token
	}

	// 获取流URI
	previewURL, err := onvifDev.GetStreamURI(profileToken)
	if err != nil {
		// 回退到构建默认URL
		previewURL = fmt.Sprintf("rtsp://%s:%s@%s:554/Streaming/Channels/101",
			device.Username, device.Password, device.IP)
	}

	debug.Debug("onvif", "生成预览URL: %s -> %s", device.Name, previewURL)
	return previewURL, nil
}

// getDeviceSnapshotURL 获取设备快照地址
func (m *Manager) getDeviceSnapshotURL(device *Device) (string, error) {
	xaddr := fmt.Sprintf("http://%s:%d/onvif/device_service", device.IP, device.Port)

	onvifDev, err := NewDevice(DeviceParams{
		Xaddr:    xaddr,
		Username: device.Username,
		Password: device.Password,
	})
	if err != nil {
		return "", fmt.Errorf("连接设备失败: %w", err)
	}

	profileToken := "main_profile"
	if len(device.Profiles) > 0 {
		profileToken = device.Profiles[0].Token
	}

	return onvifDev.GetSnapshotURI(profileToken)
}

// GetDeviceStatistics 获取设备统计信息
func (m *Manager) GetDeviceStatistics() map[string]interface{} {
	m.devicesMux.RLock()
	defer m.devicesMux.RUnlock()

	total := len(m.devices)
	online := 0
	offline := 0
	unknown := 0
	ptzDevices := 0

	for _, device := range m.devices {
		switch device.Status {
		case "online":
			online++
		case "offline":
			offline++
		default:
			unknown++
		}
		if device.PTZSupported {
			ptzDevices++
		}
	}

	return map[string]interface{}{
		"total":      total,
		"online":     online,
		"offline":    offline,
		"unknown":    unknown,
		"ptzDevices": ptzDevices,
	}
}

// ExportDevices 导出设备列表（用于备份）
func (m *Manager) ExportDevices() []map[string]interface{} {
	m.devicesMux.RLock()
	defer m.devicesMux.RUnlock()

	result := make([]map[string]interface{}, 0, len(m.devices))
	for _, device := range m.devices {
		result = append(result, map[string]interface{}{
			"ip":       device.IP,
			"port":     device.Port,
			"username": device.Username,
			"password": device.Password,
			"name":     device.Name,
		})
	}

	return result
}

// ImportDevices 导入设备列表
func (m *Manager) ImportDevices(deviceList []map[string]interface{}) (added int, failed int, errors []string) {
	for _, deviceInfo := range deviceList {
		ip, _ := deviceInfo["ip"].(string)
		port, _ := deviceInfo["port"].(float64)
		username, _ := deviceInfo["username"].(string)
		password, _ := deviceInfo["password"].(string)

		if ip == "" || port == 0 {
			failed++
			errors = append(errors, fmt.Sprintf("无效的设备信息: ip=%s, port=%.0f", ip, port))
			continue
		}

		_, err := m.AddDeviceWithIP(ip, int(port), username, password)
		if err != nil {
			failed++
			errors = append(errors, fmt.Sprintf("%s:%d - %v", ip, int(port), err))
		} else {
			added++
		}
	}

	return added, failed, errors
}

// WSDiscoveryService WS-Discovery服务
type WSDiscoveryService struct {
	manager    *Manager
	stopChan   chan struct{}
	running    bool
	interfaces []net.Interface // 网络接口列表
	localIPs   []net.IP        // 所有本地IPv4地址
}

// NewWSDiscoveryService 创建WS-Discovery服务
func NewWSDiscoveryService(manager *Manager) *WSDiscoveryService {
	return &WSDiscoveryService{
		manager:  manager,
		stopChan: make(chan struct{}),
	}
}

// Start 启动WS-Discovery服务
func (s *WSDiscoveryService) Start() error {
	// 获取所有可用的网络接口
	interfaces, err := net.Interfaces()
	if err != nil {
		return fmt.Errorf("获取网络接口失败: %w", err)
	}

	// 收集所有有效的IPv4地址
	for _, iface := range interfaces {
		// 跳过不活动和回环接口
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		// 检查是否有IPv4地址
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		hasIPv4 := false
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				hasIPv4 = true
				// 收集每个IPv4地址
				s.localIPs = append(s.localIPs, ipnet.IP)
			}
		}

		if hasIPv4 {
			s.interfaces = append(s.interfaces, iface)
		}
	}

	if len(s.localIPs) == 0 {
		log.Println("[ONVIF] [WARN] 未找到可用的IPv4地址")
	}

	s.running = true
	log.Printf("[ONVIF] ✓ WS-Discovery服务启动 (发现 %d 个网络接口, %d 个IP地址)", len(s.interfaces), len(s.localIPs))
	for _, ip := range s.localIPs {
		log.Printf("[ONVIF]   - %s", ip.String())
	}

	return nil
}

// Stop 停止WS-Discovery服务
func (s *WSDiscoveryService) Stop() {
	if s.running {
		close(s.stopChan)
		s.running = false
		log.Println("[ONVIF] WS-Discovery服务已停止")
	}
}

// Probe 发送WS-Discovery探测消息（支持多网卡多IP）
func (s *WSDiscoveryService) Probe() ([]DeviceDiscoveryResult, error) {
	var allResults []DeviceDiscoveryResult
	resultMap := make(map[string]bool) // 用于去重

	// WS-Discovery多播地址
	multicastAddr, err := net.ResolveUDPAddr("udp4", "239.255.255.250:3702")
	if err != nil {
		return nil, fmt.Errorf("解析多播地址失败: %w", err)
	}

	// 在每个IP地址上发送探测
	for _, localIP := range s.localIPs {
		results, err := s.probeOnIP(localIP, multicastAddr)
		if err != nil {
			debug.Debug("onvif", "IP %s 探测失败: %v", localIP.String(), err)
			continue
		}

		// 合并结果（去重）
		for _, result := range results {
			if result.XAddr != "" && !resultMap[result.XAddr] {
				resultMap[result.XAddr] = true
				allResults = append(allResults, result)
			}
		}
	}

	// 如果没有本地IP，使用默认接口
	if len(s.localIPs) == 0 {
		results, err := s.probeDefault(multicastAddr)
		if err != nil {
			debug.Debug("onvif", "默认接口探测失败: %v", err)
		} else {
			for _, result := range results {
				if result.XAddr != "" && !resultMap[result.XAddr] {
					resultMap[result.XAddr] = true
					allResults = append(allResults, result)
				}
			}
		}
	}

	log.Printf("[ONVIF] WS-Discovery发现 %d 个设备", len(allResults))
	return allResults, nil
}

// probeOnIP 在指定IP地址上发送探测
func (s *WSDiscoveryService) probeOnIP(localIP net.IP, multicastAddr *net.UDPAddr) ([]DeviceDiscoveryResult, error) {
	// 创建UDP连接，绑定到特定IP
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: localIP, Port: 0})
	if err != nil {
		return nil, fmt.Errorf("创建UDP连接失败: %w", err)
	}
	defer conn.Close()

	return s.sendProbeAndCollect(conn, multicastAddr, localIP.String())
}

// probeDefault 使用默认接口发送探测
func (s *WSDiscoveryService) probeDefault(multicastAddr *net.UDPAddr) ([]DeviceDiscoveryResult, error) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{Port: 0})
	if err != nil {
		return nil, fmt.Errorf("创建UDP连接失败: %w", err)
	}
	defer conn.Close()

	return s.sendProbeAndCollect(conn, multicastAddr, "default")
}

// sendProbeAndCollect 发送探测并收集响应
func (s *WSDiscoveryService) sendProbeAndCollect(conn *net.UDPConn, multicastAddr *net.UDPAddr, ifaceName string) ([]DeviceDiscoveryResult, error) {
	// 构建探测消息
	messageID := GenerateUUID()
	probeMessage := BuildWSDiscoveryProbe(messageID)

	// 发送探测消息（发送多次以提高可靠性）
	for i := 0; i < 2; i++ {
		_, err := conn.WriteToUDP([]byte(probeMessage), multicastAddr)
		if err != nil {
			return nil, fmt.Errorf("发送探测消息失败: %w", err)
		}
		time.Sleep(100 * time.Millisecond)
	}

	debug.Debug("onvif", "已在接口 %s 发送WS-Discovery探测 (MessageID: %s)", ifaceName, messageID)

	// 收集响应
	var results []DeviceDiscoveryResult
	buffer := make([]byte, 16384) // 增大缓冲区

	// 设置接收超时
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	for {
		n, remoteAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				break // 超时，结束收集
			}
			continue
		}

		// 解析响应
		result := s.parseProbeResponse(buffer[:n])
		if result != nil {
			result.SourceIP = remoteAddr.IP.String()
			results = append(results, *result)
			debug.Debug("onvif", "收到设备响应: %s (来自: %s)", result.XAddr, remoteAddr.String())
		}
	}

	return results, nil
}

// parseProbeResponse 解析探测响应
func (s *WSDiscoveryService) parseProbeResponse(data []byte) *DeviceDiscoveryResult {
	// 尝试标准XML解析
	var response DiscoveryResponse
	if err := xml.Unmarshal(data, &response); err != nil {
		// 尝试使用正则表达式解析
		return s.parseProbeResponseFallback(data)
	}

	if len(response.Body.ProbeMatches.ProbeMatch) == 0 {
		return s.parseProbeResponseFallback(data)
	}

	match := response.Body.ProbeMatches.ProbeMatch[0]

	// 处理多个XAddrs（取第一个有效的）
	xaddrs := strings.Fields(match.XAddrs)
	var primaryXAddr string
	for _, xaddr := range xaddrs {
		if strings.HasPrefix(xaddr, "http://") {
			primaryXAddr = xaddr
			break
		}
	}
	if primaryXAddr == "" && len(xaddrs) > 0 {
		primaryXAddr = xaddrs[0]
	}

	result := &DeviceDiscoveryResult{
		XAddr: primaryXAddr,
		Types: strings.Fields(match.Types),
	}

	// 解析Scopes
	scopeInfo := ParseDiscoveryScopes(match.Scopes)
	result.Scopes = strings.Fields(match.Scopes)
	result.Manufacturer = scopeInfo.Manufacturer
	result.Model = scopeInfo.Model
	result.Name = scopeInfo.Name
	result.Location = scopeInfo.Location
	result.Hardware = scopeInfo.Hardware
	result.Extras = scopeInfo.Extras

	// 从EndpointReference获取UUID
	if match.EndpointReference.Address != "" {
		result.Extras["uuid"] = match.EndpointReference.Address
	}

	return result
}

// parseProbeResponseFallback 使用正则表达式解析响应（备用方案）
func (s *WSDiscoveryService) parseProbeResponseFallback(data []byte) *DeviceDiscoveryResult {
	dataStr := string(data)

	// 提取XAddrs
	xaddrPattern := regexp.MustCompile(`<[^:]*:?XAddrs>([^<]+)</[^:]*:?XAddrs>`)
	xaddrMatch := xaddrPattern.FindStringSubmatch(dataStr)
	if len(xaddrMatch) < 2 {
		return nil
	}

	xaddrs := strings.Fields(xaddrMatch[1])
	var primaryXAddr string
	for _, xaddr := range xaddrs {
		if strings.HasPrefix(xaddr, "http://") {
			primaryXAddr = xaddr
			break
		}
	}
	if primaryXAddr == "" && len(xaddrs) > 0 {
		primaryXAddr = xaddrs[0]
	}

	if primaryXAddr == "" {
		return nil
	}

	result := &DeviceDiscoveryResult{
		XAddr:  primaryXAddr,
		Extras: make(map[string]string),
	}

	// 提取Types
	typesPattern := regexp.MustCompile(`<[^:]*:?Types>([^<]+)</[^:]*:?Types>`)
	if typesMatch := typesPattern.FindStringSubmatch(dataStr); len(typesMatch) > 1 {
		result.Types = strings.Fields(typesMatch[1])
	}

	// 提取Scopes
	scopesPattern := regexp.MustCompile(`<[^:]*:?Scopes>([^<]+)</[^:]*:?Scopes>`)
	if scopesMatch := scopesPattern.FindStringSubmatch(dataStr); len(scopesMatch) > 1 {
		scopeInfo := ParseDiscoveryScopes(scopesMatch[1])
		result.Scopes = strings.Fields(scopesMatch[1])
		result.Manufacturer = scopeInfo.Manufacturer
		result.Model = scopeInfo.Model
		result.Name = scopeInfo.Name
		result.Location = scopeInfo.Location
		result.Hardware = scopeInfo.Hardware
		if scopeInfo.Extras != nil {
			for k, v := range scopeInfo.Extras {
				result.Extras[k] = v
			}
		}
	}

	return result
}

// contains 检查字符串切片中是否包含指定字符串
func contains(slice []string, item string) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}

// getONVIFAddr 获取设备的ONVIF端点地址，优先使用已保存的地址
func (m *Manager) getONVIFAddr(device *Device) string {
	if device.ONVIFAddr != "" {
		return device.ONVIFAddr
	}
	// 备选：构造默认地址
	return fmt.Sprintf("http://%s:%d/onvif/device_service", device.IP, device.Port)
}
