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

// StreamProxyCallback 流代理添加回调函数类型
// deviceID: 设备ID, rtspURL: RTSP流地址, username/password: 设备凭据
type StreamProxyCallback func(deviceID, rtspURL, username, password string) error

// Manager ONVIF管理器结构体
type Manager struct {
	config        *config.ONVIFConfig
	devices       map[string]*Device
	devicesMux    sync.RWMutex
	stopChan      chan struct{}
	wsDiscovery   *WSDiscoveryService
	eventHandlers map[string][]EventHandler
	handlersMux   sync.RWMutex
	// SOAP客户端缓存，避免每次操作都重新创建客户端
	soapClients   map[string]*SOAPClient
	soapClientMux sync.RWMutex
	// PTZ客户端缓存（保持向后兼容）
	ptzClients   map[string]*SOAPClient
	ptzClientMux sync.RWMutex
	// 流代理回调：设备发现后自动添加流代理
	streamProxyCallback StreamProxyCallback
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
	// 缓存相关
	InfoFetchedAt time.Time // 设备详情获取时间（避免频繁获取）
	InfoCacheTTL  int       // 设备信息缓存有效期(秒)，默认300秒
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
		soapClients:   make(map[string]*SOAPClient),
		ptzClients:    make(map[string]*SOAPClient),
	}

	// 初始化WS-Discovery服务
	m.wsDiscovery = NewWSDiscoveryService(m)

	return m
}

// SetStreamProxyCallback 设置流代理回调函数
// 当设备发现完成并获取到流地址后，会调用此回调自动添加流代理
func (m *Manager) SetStreamProxyCallback(callback StreamProxyCallback) {
	m.streamProxyCallback = callback
}

// getOrCreateSOAPClient 获取或创建通用SOAP客户端缓存
func (m *Manager) getOrCreateSOAPClient(device *Device) (*SOAPClient, error) {
	m.soapClientMux.RLock()
	client, exists := m.soapClients[device.DeviceID]
	m.soapClientMux.RUnlock()

	if exists && client != nil {
		return client, nil
	}

	xaddr := m.getONVIFAddr(device)
	if xaddr == "" {
		return nil, fmt.Errorf("设备未提供 XAddr")
	}

	client = NewSOAPClient(xaddr, device.Username, device.Password)

	// 预先获取服务地址
	_, _ = client.GetCapabilities()

	m.soapClientMux.Lock()
	m.soapClients[device.DeviceID] = client
	m.soapClientMux.Unlock()

	debug.Debug("onvif", "创建SOAP客户端缓存: 设备=%s", device.DeviceID)
	return client, nil
}

// initDeviceSOAPClient 预创建并缓存设备的SOAP客户端
// 在设备发现完成后调用，提前获取 PTZ/Media 服务地址并缓存
func (m *Manager) initDeviceSOAPClient(device *Device) {
	if device == nil {
		return
	}

	xaddr := m.getONVIFAddr(device)
	if xaddr == "" {
		return
	}

	// 检查是否已经有缓存的客户端
	m.soapClientMux.RLock()
	existingClient := m.soapClients[device.DeviceID]
	m.soapClientMux.RUnlock()

	if existingClient != nil {
		// 已有缓存，检查是否有 PTZ 地址
		if existingClient.GetPTZAddr() != "" {
			return
		}
	}

	// 创建新的 SOAP 客户端
	client := NewSOAPClient(xaddr, device.Username, device.Password)

	// 获取服务地址（GetCapabilities 会设置 mediaAddr 和 ptzAddr）
	client.GetCapabilities()

	// 缓存客户端
	m.soapClientMux.Lock()
	m.soapClients[device.DeviceID] = client
	m.soapClientMux.Unlock()
}

// ClearSOAPClientCache 清除指定设备的SOAP客户端缓存
func (m *Manager) ClearSOAPClientCache(deviceID string) {
	m.soapClientMux.Lock()
	delete(m.soapClients, deviceID)
	m.soapClientMux.Unlock()

	m.ptzClientMux.Lock()
	delete(m.ptzClients, deviceID)
	m.ptzClientMux.Unlock()

	debug.Debug("onvif", "清除SOAP客户端缓存: 设备=%s", deviceID)
}

// Start 启动ONVIF管理器
func (m *Manager) Start() error {
	log.Println("[ONVIF] ✓ ONVIF管理器启动成功")
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
	// 状态监控间隔设为60秒，与设备检查间隔一致
	ticker := time.NewTicker(60 * time.Second)
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
	debug.Debug("onvif", "开始设备发现过程")

	// 使用WS-Discovery进行设备发现
	if m.wsDiscovery != nil {
		discoveredDevices, err := m.wsDiscovery.Probe()
		if err != nil {
			debug.Warn("onvif", "WS-Discovery探测失败: %v", err)
		} else {
			for _, result := range discoveredDevices {
				m.tryAutoAddDevice(result)
			}
		}
	}

	debug.Debug("onvif", "设备发现完成")
}

// tryAutoAddDevice 尝试自动添加发现的设备
// tryAutoAddDevice 尝试自动添加发现的设备
func (m *Manager) tryAutoAddDevice(result DeviceDiscoveryResult) {
	// 解析设备地址
	host, port, err := ParseXAddr(result.XAddr)
	if err != nil {
		debug.Debug("onvif", "解析发现的设备地址失败: %v", err)
		return
	}

	deviceID := fmt.Sprintf("%s:%d", host, port)

	// 检查设备是否已存在
	m.devicesMux.RLock()
	_, exists := m.devices[deviceID]
	m.devicesMux.RUnlock()

	if exists {
		return
	}

	// 创建新设备记录（未验证状态）
	device := &Device{
		DeviceID:      deviceID,
		Name:          result.Name,
		Model:         result.Model,
		Manufacturer:  result.Manufacturer,
		IP:            host,
		Port:          port,
		SipPort:       5060,
		Status:        "discovered",
		DiscoveryTime: time.Now(),
		Services:      result.Types,
		Metadata:      result.Extras,
		CheckInterval: 60,
		ONVIFAddr:     result.XAddr,
	}

	if device.Name == "" {
		device.Name = fmt.Sprintf("ONVIF Camera (%s)", host)
	}

	// 立即添加基本设备信息
	m.devicesMux.Lock()
	m.devices[deviceID] = device
	m.devicesMux.Unlock()

	log.Printf("[ONVIF] ✓ 发现新设备: %s (%s)", device.Name, device.DeviceID)

	// 异步获取设备详细信息
	go func() {
		xaddr := result.XAddr
		if xaddr == "" {
			return
		}

		// 尝试多组凭据（按优先级）
		credentialsList := []struct {
			username string
			password string
		}{
			{"admin", "a123456789"}, // 先尝试 admin 用户
			{"test", "a123456789"},  // 再尝试 test 用户
			{"admin", "admin"},      // 常见默认凭据
			{"", ""},                // 匿名访问
		}

		var detailedDevice *Device
		var workingUsername, workingPassword string

		for _, cred := range credentialsList {
			dev, err := m.getDeviceDetails(xaddr, cred.username, cred.password)
			if err == nil && dev != nil {
				detailedDevice = dev
				workingUsername = cred.username
				workingPassword = cred.password
				break
			}
		}

		if detailedDevice != nil {
			// 使用详细信息更新设备
			m.devicesMux.Lock()
			detailedDevice.DiscoveryTime = device.DiscoveryTime
			if detailedDevice.Name == "" {
				detailedDevice.Name = device.Name
			}
			detailedDevice.ONVIFAddr = xaddr
			detailedDevice.Username = workingUsername
			detailedDevice.Password = workingPassword
			m.devices[deviceID] = detailedDevice
			m.devicesMux.Unlock()

			// 预创建并缓存 SOAP 客户端，提前获取 PTZ/Media 服务地址
			go m.initDeviceSOAPClient(detailedDevice)

			// 自动添加流代理（如果设置了回调且有流地址）
			if m.streamProxyCallback != nil && detailedDevice.PreviewURL != "" {
				go func(dev *Device) {
					if err := m.streamProxyCallback(dev.DeviceID, dev.PreviewURL, dev.Username, dev.Password); err != nil {
						debug.Warn("onvif", "自动添加流代理失败: %s | %v", dev.DeviceID, err)
					}
				}(detailedDevice)
			}
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

// 辅助：判断是否为认证错误或TLS错误
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unauthorized") ||
		strings.Contains(msg, "not authorized") ||
		strings.Contains(msg, "auth") ||
		strings.Contains(msg, "401")
}
func isTLSError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "x509") ||
		strings.Contains(msg, "certificate") ||
		strings.Contains(msg, "tls")
}

// 简单的 TCP 连通性检查（避免 SOAP 超时浪费）
func checkXAddrReachable(xaddr string, timeout time.Duration) error {
	u, err := url.Parse(xaddr)
	if err != nil {
		return fmt.Errorf("XAddr解析失败: %w", err)
	}
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	addr := net.JoinHostPort(host, port)
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return fmt.Errorf("端点不可达: %s (%w)", addr, err)
	}
	_ = conn.Close()
	return nil
}

// getDeviceDetails 获取设备详细信息，带分类处理（认证/可达性/TLS）
func (m *Manager) getDeviceDetails(xaddr, username, password string) (*Device, error) {
	if xaddr == "" {
		return nil, fmt.Errorf("未提供 XAddr（请使用 WS-Discovery 获取设备端点）")
	}

	// 先做连通性检查
	if err := checkXAddrReachable(xaddr, 3*time.Second); err != nil {
		return nil, fmt.Errorf("端点不可达: %w", err)
	}

	// 创建客户端并测试，增加超时时间
	d, err := NewDevice(DeviceParams{
		Xaddr:    xaddr,
		Username: username,
		Password: password,
		Timeout:  30 * time.Second, // 增加超时时间到 30 秒
	})
	if err != nil {
		if isTLSError(err) {
			return nil, fmt.Errorf("TLS错误（可能是自签名证书或HTTPS端点）：%w", err)
		}
		return nil, fmt.Errorf("创建设备客户端失败: %w", err)
	}

	// 连接成功后，获取设备信息
	u, _ := url.Parse(xaddr)
	ip := u.Hostname()
	port := 80
	if u != nil && u.Port() != "" {
		if p, perr := strconv.Atoi(u.Port()); perr == nil {
			port = p
		}
	}

	var services []string
	for _, serviceAddr := range d.GetServices() {
		services = append(services, serviceAddr)
	}

	deviceInfo, _ := d.GetDeviceInfo()
	capabilities := d.GetCapabilities()
	profiles, _ := d.GetMediaProfiles()

	var previewURL string
	if len(profiles) > 0 {
		if url, err := d.GetStreamURI(profiles[0].Token); err == nil {
			previewURL = url
		}
	}
	var snapshotURL string
	if len(profiles) > 0 {
		if url, err := d.GetSnapshotURI(profiles[0].Token); err == nil {
			snapshotURL = url
		}
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
		SipPort:         5060,
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
		ONVIFAddr:       xaddr,
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

// GetDeviceByID 根据ID获取ONVIF设备（支持端口自适应）
func (m *Manager) GetDeviceByID(deviceID string) (*Device, bool) {
	m.devicesMux.RLock()
	defer m.devicesMux.RUnlock()

	// 尝试精确匹配
	if device, exists := m.devices[deviceID]; exists {
		return device, true
	}

	// 如果精确匹配失败，尝试基于IP的模糊匹配（自适应端口）
	// 用于处理端口差异（如 192.168.1.232:80 vs 192.168.1.232:443）
	parts := strings.Split(deviceID, ":")
	if len(parts) == 2 {
		requestIP := parts[0]

		for existingID, device := range m.devices {
			existingParts := strings.Split(existingID, ":")
			if len(existingParts) == 2 && existingParts[0] == requestIP {
				return device, true
			}
		}
	}

	return nil, false
}

// GetDeviceList 获取所有设备ID列表（用于调试）
func (m *Manager) GetDeviceList() []string {
	m.devicesMux.RLock()
	defer m.devicesMux.RUnlock()

	deviceIDs := make([]string, 0, len(m.devices))
	for id := range m.devices {
		deviceIDs = append(deviceIDs, id)
	}

	return deviceIDs
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
		Timeout:  10 * time.Second,
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
func (m *Manager) RemovePreset(deviceID, profileToken string, presetToken string) error {
	device, exists := m.GetDeviceByID(deviceID)
	if !exists {
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	xaddr := m.getONVIFAddr(device)
	if xaddr == "" {
		return fmt.Errorf("设备未提供 XAddr（WS-Discovery 未返回端点）")
	}

	d, err := NewDevice(DeviceParams{
		Xaddr:    xaddr,
		Username: device.Username,
		Password: device.Password,
		Timeout:  10 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("创建设备客户端失败: %w", err)
	}

	// 🔧 测试连接
	if err := d.TestConnection(); err != nil {
		return fmt.Errorf("设备连接失败: %w", err)
	}

	if profileToken == "" && len(device.Profiles) > 0 {
		profileToken = device.Profiles[0].Token
	}

	if profileToken == "" {
		return fmt.Errorf("未指定 profileToken 且设备无可用配置文件")
	}

	debug.Info("onvif", "删除预置位: 设备=%s, 预置位=%s", deviceID, presetToken)
	// 🔧 直接调用 d 的方法
	return d.RemovePreset(profileToken, presetToken)
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
	xaddr := m.getONVIFAddr(device)
	d, err := NewDevice(DeviceParams{
		Xaddr:    xaddr,
		Username: device.Username,
		Password: device.Password,
		Timeout:  10 * time.Second,
	})
	if err != nil {
		return "", fmt.Errorf("创建设备客户端失败: %w", err)
	}

	return d.GetSnapshotURI(profileToken)
}

// UpdateDeviceIP 更新设备IP地址和端口
func (m *Manager) UpdateDeviceIP(deviceID, newIP string, newPort int) error {
	m.devicesMux.Lock()
	defer m.devicesMux.Unlock()

	// 尝试精确匹配
	device, exists := m.devices[deviceID]
	actualDeviceID := deviceID

	// 如果精确匹配失败，尝试基于IP的模糊匹配（自适应端口）
	if !exists {
		parts := strings.Split(deviceID, ":")
		if len(parts) == 2 {
			requestIP := parts[0]
			for existingID, d := range m.devices {
				existingParts := strings.Split(existingID, ":")
				if len(existingParts) == 2 && existingParts[0] == requestIP {
					device = d
					actualDeviceID = existingID
					exists = true
					break
				}
			}
		}
	}

	if !exists {
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	// 验证新IP地址的有效性
	if newIP != "" && net.ParseIP(newIP) == nil {
		return fmt.Errorf("无效的IP地址: %s", newIP)
	}

	// 验证新端口的有效性
	if newPort > 0 && (newPort < 1 || newPort > 65535) {
		return fmt.Errorf("无效的端口号: %d", newPort)
	}

	// 更新IP和端口
	if newIP != "" {
		device.IP = newIP
		// 重新生成设备ID (IP:Port 格式)
		oldDeviceID := actualDeviceID
		if newPort == 0 {
			newPort = device.Port // 保持原端口
		}
		device.Port = newPort
		newDeviceID := fmt.Sprintf("%s:%d", newIP, newPort)

		// 更新设备ID
		if oldDeviceID != newDeviceID {
			m.devices[newDeviceID] = device
			delete(m.devices, oldDeviceID)
			device.DeviceID = newDeviceID
		}

		// 重新生成ONVIF地址
		device.ONVIFAddr = m.getONVIFAddr(device)

		// 验证新地址的连接
		xaddr := m.getONVIFAddr(device)
		d, err := NewDevice(DeviceParams{
			Xaddr:    xaddr,
			Username: device.Username,
			Password: device.Password,
			Timeout:  10 * time.Second,
		})
		if err != nil {
			// 如果验证失败，回复原值
			device.IP = strings.Split(oldDeviceID, ":")[0]
			device.Port, _ = strconv.Atoi(strings.Split(oldDeviceID, ":")[1])
			device.ONVIFAddr = m.getONVIFAddr(device)
			return fmt.Errorf("新IP地址验证失败: %w", err)
		}

		// 验证连接
		if err := d.TestConnection(); err != nil {
			// 如果连接失败，回复原值
			device.IP = strings.Split(oldDeviceID, ":")[0]
			device.Port, _ = strconv.Atoi(strings.Split(oldDeviceID, ":")[1])
			device.ONVIFAddr = m.getONVIFAddr(device)
			return fmt.Errorf("设备连接测试失败: %w", err)
		}

		log.Printf("[ONVIF] ✓ 设备IP已更新: %s -> %s:%d", oldDeviceID, newIP, newPort)
	} else if newPort > 0 {
		// 只更新端口
		device.Port = newPort
		newDeviceID := fmt.Sprintf("%s:%d", device.IP, newPort)
		oldDeviceID := actualDeviceID

		if oldDeviceID != newDeviceID {
			m.devices[newDeviceID] = device
			delete(m.devices, oldDeviceID)
			device.DeviceID = newDeviceID
		}

		device.ONVIFAddr = m.getONVIFAddr(device)
		log.Printf("[ONVIF] ✓ 设备端口已更新: %s -> %s", oldDeviceID, newDeviceID)
	}

	return nil
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
	xaddr := m.getONVIFAddr(device)
	d, err := NewDevice(DeviceParams{
		Xaddr:    xaddr,
		Username: device.Username,
		Password: device.Password,
		Timeout:  10 * time.Second,
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

	xaddr := m.getONVIFAddr(device)
	if xaddr == "" {
		return nil, fmt.Errorf("设备未提供 XAddr（WS-Discovery 未返回端点）")
	}

	d, err := NewDevice(DeviceParams{
		Xaddr:    xaddr,
		Username: device.Username,
		Password: device.Password,
		Timeout:  10 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("创建设备客户端失败: %w", err)
	}

	// 🔧 测试连接
	if err := d.TestConnection(); err != nil {
		return nil, fmt.Errorf("设备连接失败: %w", err)
	}

	if profileToken == "" && len(device.Profiles) > 0 {
		profileToken = device.Profiles[0].Token
	}

	if profileToken == "" {
		return nil, fmt.Errorf("未指定 profileToken 且设备无可用配置文件")
	}

	// 获取媒体配置文件以获取视频编码信息
	mediaProfiles, err := d.GetMediaProfiles()
	if err != nil {
		return nil, fmt.Errorf("获取媒体配置文件失败: %w", err)
	}

	result := make([]map[string]interface{}, 0)
	for _, profile := range mediaProfiles {
		if profile.VideoEncoder != nil {
			result = append(result, map[string]interface{}{
				"token":        profile.VideoEncoder.Token,
				"name":         profile.VideoEncoder.Name,
				"encoding":     profile.VideoEncoder.Encoding,
				"width":        profile.VideoEncoder.Width,
				"height":       profile.VideoEncoder.Height,
				"quality":      profile.VideoEncoder.Quality,
				"frameRate":    profile.VideoEncoder.FrameRate,
				"bitrateLimit": profile.VideoEncoder.BitrateLimit,
				"h264Profile":  profile.VideoEncoder.H264Profile,
			})
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
	xaddr := m.getONVIFAddr(device)
	d, err := NewDevice(DeviceParams{
		Xaddr:    xaddr,
		Username: device.Username,
		Password: device.Password,
		Timeout:  10 * time.Second,
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

	xaddr := m.getONVIFAddr(device)
	d, err := NewDevice(DeviceParams{
		Xaddr:    xaddr,
		Username: device.Username,
		Password: device.Password,
		Timeout:  10 * time.Second,
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

	// 验证地址格式
	if xaddr == "" {
		return nil, fmt.Errorf("设备地址不能为空")
	}

	// 获取设备详细信息
	device, err := m.getDeviceDetails(xaddr, username, password)
	if err != nil {
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
		return existingDevice, nil
	}

	m.devices[device.DeviceID] = device

	log.Printf("[ONVIF] ✓ 设备添加成功: %s (%s)", device.DeviceID, device.Name)
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

// AddDeviceWithIP 通过 IP 和端口添加设备（禁用：不再构造端点）
func (m *Manager) AddDeviceWithIP(ip string, port int, username, password string) (*Device, error) {
	return nil, fmt.Errorf("已禁用基于 IP/端口构造端点：请使用 WS-Discovery 或调用 AddDevice(xaddr, ...) 提供完整 XAddr")
}

// VerifyDeviceCredentials 验证设备的用户名和密码是否正确
func (m *Manager) VerifyDeviceCredentials(ip string, port int, username, password string) error {
	xaddr := fmt.Sprintf("http://%s:%d/onvif/device_service", ip, port)

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
		allDevices := m.GetDeviceList()
		return nil, fmt.Errorf("设备不存在: %s (已知设备: %v)", deviceID, allDevices)
	}

	// 创建ONVIF设备客户端
	xaddr := m.getONVIFAddr(device)
	if xaddr == "" {
		return nil, fmt.Errorf("设备未提供 XAddr（WS-Discovery 未返回端点）")
	}

	d, err := NewDevice(DeviceParams{
		Xaddr:    xaddr,
		Username: device.Username,
		Password: device.Password,
		Timeout:  10 * time.Second,
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

// GetProfilesWithCredentials 使用指定的凭据获取设备的媒体配置文件
func (m *Manager) GetProfilesWithCredentials(deviceID, username, password string) ([]map[string]interface{}, error) {
	device, exists := m.GetDeviceByID(deviceID)
	if !exists {
		allDevices := m.GetDeviceList()
		return nil, fmt.Errorf("设备不存在: %s (已知设备: %v)", deviceID, allDevices)
	}

	// 使用传入的凭据，如果为空则使用设备存储的凭据，最后回退到默认凭据
	if username == "" {
		username = device.Username
	}
	if password == "" {
		password = device.Password
	}
	if username == "" {
		username = "admin"
	}
	if password == "" {
		password = "a123456789"
	}

	// 创建ONVIF设备客户端
	xaddr := m.getONVIFAddr(device)
	if xaddr == "" {
		return nil, fmt.Errorf("设备未提供 XAddr（WS-Discovery 未返回端点）")
	}

	d, err := NewDevice(DeviceParams{
		Xaddr:    xaddr,
		Username: username,
		Password: password,
		Timeout:  10 * time.Second,
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
		// 如果认证成功，更新设备的凭据
		if dev.Username == "" || dev.Password == "" {
			dev.Username = username
			dev.Password = password
		}
	}
	m.devicesMux.Unlock()

	return profiles, nil
}

// getOrCreatePTZClient 获取或创建PTZ客户端缓存（复用通用SOAP客户端）
func (m *Manager) getOrCreatePTZClient(device *Device) (*SOAPClient, error) {
	// 直接复用通用的 SOAP 客户端缓存
	return m.getOrCreateSOAPClient(device)
}

// ClearPTZClientCache 清除指定设备的PTZ客户端缓存
func (m *Manager) ClearPTZClientCache(deviceID string) {
	// 直接调用通用的缓存清除方法
	m.ClearSOAPClientCache(deviceID)
}

// PTZControl 控制设备PTZ (优化版：使用缓存的SOAP客户端)
func (m *Manager) PTZControl(deviceID, command string, speed float64) error {
	device, exists := m.GetDeviceByID(deviceID)
	if !exists {
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	// 使用缓存的PTZ客户端
	client, err := m.getOrCreatePTZClient(device)
	if err != nil {
		return fmt.Errorf("获取PTZ客户端失败: %w", err)
	}

	// 获取默认配置文件Token
	profileToken := "main_profile"
	if len(device.Profiles) > 0 {
		profileToken = device.Profiles[0].Token
	}

	// 根据命令执行不同的PTZ操作
	switch strings.ToLower(command) {
	case "up":
		return client.ContinuousMove(profileToken, 0, speed, 0, 5.0)
	case "down":
		return client.ContinuousMove(profileToken, 0, -speed, 0, 5.0)
	case "left":
		return client.ContinuousMove(profileToken, -speed, 0, 0, 5.0)
	case "right":
		return client.ContinuousMove(profileToken, speed, 0, 0, 5.0)
	case "zoomin":
		return client.ContinuousMove(profileToken, 0, 0, speed, 5.0)
	case "zoomout":
		return client.ContinuousMove(profileToken, 0, 0, -speed, 5.0)
	case "stop":
		return client.StopPTZ(profileToken)
	case "home":
		return client.GotoPreset(profileToken, "1")
	default:
		return fmt.Errorf("未知的PTZ命令: %s", command)
	}
}

// PTZGotoPreset 移动到预置位
func (m *Manager) PTZGotoPreset(deviceID, presetToken string) error {
	device, exists := m.GetDeviceByID(deviceID)
	if !exists {
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	xaddr := m.getONVIFAddr(device)
	if xaddr == "" {
		return fmt.Errorf("设备未提供 XAddr（WS-Discovery 未返回端点）")
	}

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
	return d.GotoPreset(profileToken, presetToken)
}

// GetPTZPresets 获取PTZ预置位列表
func (m *Manager) GetPTZPresets(deviceID string) ([]PTZPreset, error) {
	device, exists := m.GetDeviceByID(deviceID)
	if !exists {
		return nil, fmt.Errorf("设备不存在: %s", deviceID)
	}

	xaddr := m.getONVIFAddr(device)
	if xaddr == "" {
		return nil, fmt.Errorf("设备未提供 XAddr（WS-Discovery 未返回端点）")
	}

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

	xaddr := m.getONVIFAddr(device)
	if xaddr == "" {
		return "", fmt.Errorf("设备未提供 XAddr（WS-Discovery 未返回端点）")
	}

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

// checkDeviceStatus 检查单个设备的在线状态（优化版：减少SOAP请求）
func (m *Manager) checkDeviceStatus(device *Device) {
	now := time.Now()

	// 设置默认检查间隔为60秒
	if device.CheckInterval <= 0 {
		device.CheckInterval = 60
	}

	// 设置默认缓存有效期为300秒（5分钟）
	if device.InfoCacheTTL <= 0 {
		device.InfoCacheTTL = 300
	}

	// 检查间隔未到，跳过
	if !device.LastCheckTime.IsZero() &&
		device.LastCheckTime.Add(time.Duration(device.CheckInterval)*time.Second).After(now) {
		return
	}

	// 获取或创建缓存的SOAP客户端
	client, err := m.getOrCreateSOAPClient(device)
	if err != nil {
		m.handleDeviceOffline(device, err)
		return
	}

	// 记录检查开始时间
	start := time.Now()

	// 仅用 GetSystemDateAndTime 测试连接（最轻量的请求）
	_, err = client.GetSystemDateAndTime()

	// 记录响应时间
	device.ResponseTime = time.Since(start).Milliseconds()
	device.LastCheckTime = now

	if err != nil {
		// 连接失败，清除缓存，下次重新创建
		m.ClearSOAPClientCache(device.DeviceID)
		m.handleDeviceOffline(device, err)
		return
	}

	// 设备在线
	previousStatus := device.Status
	device.Status = "online"
	device.FailureCount = 0
	device.LastSeenTime = now

	// 仅在缓存过期时才获取详细信息（避免频繁请求）
	needFetchInfo := device.InfoFetchedAt.IsZero() ||
		device.InfoFetchedAt.Add(time.Duration(device.InfoCacheTTL)*time.Second).Before(now)

	if needFetchInfo {
		device.InfoFetchedAt = now
		// 异步获取详细信息，不阻塞状态检查
		go m.fetchDeviceDetails(device, client)
	}

	// 如果设备刚刚上线，触发事件
	if previousStatus != "online" {
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

// fetchDeviceDetails 异步获取设备详细信息（使用缓存的SOAP客户端）
func (m *Manager) fetchDeviceDetails(device *Device, client *SOAPClient) {
	// 获取设备信息
	if info, err := client.GetDeviceInformation(); err == nil {
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

	// 获取设备能力（客户端已缓存，不会再次请求）
	if client.ptzAddr != "" {
		device.PTZSupported = true
		if device.Capabilities == nil {
			device.Capabilities = &DeviceCapabilities{HasPTZ: true}
		} else {
			device.Capabilities.HasPTZ = true
		}
	}

	// 获取预览URL（仅在没有时获取）
	if device.PreviewURL == "" {
		profileToken := "main_profile"
		if len(device.Profiles) > 0 {
			profileToken = device.Profiles[0].Token
		}
		if url, err := client.GetStreamURI(profileToken); err == nil && url != "" {
			device.PreviewURL = url
		}
	}

	// 获取快照URL（仅在没有时获取）
	if device.SnapshotURL == "" {
		profileToken := "main_profile"
		if len(device.Profiles) > 0 {
			profileToken = device.Profiles[0].Token
		}
		if url, err := client.GetSnapshotURI(profileToken); err == nil && url != "" {
			device.SnapshotURL = url
		}
	}

	debug.Debug("onvif", "设备详情更新完成: %s", device.DeviceID)
}

// getDevicePreviewURL 获取设备RTSP预览地址（优化版：使用缓存的客户端）
func (m *Manager) getDevicePreviewURL(device *Device) (string, error) {
	// 如果凭据未设置，跳过
	if device.Username == "" && device.Password == "" {
		return "", fmt.Errorf("设备凭据未设置，跳过获取预览URL")
	}

	// 使用缓存的SOAP客户端
	client, err := m.getOrCreateSOAPClient(device)
	if err != nil {
		return "", fmt.Errorf("获取SOAP客户端失败: %w", err)
	}

	// 获取默认配置文件Token
	profileToken := "main_profile"
	if len(device.Profiles) > 0 {
		profileToken = device.Profiles[0].Token
	}

	// 获取流URI
	previewURL, err := client.GetStreamURI(profileToken)
	if err != nil {
		// 回退到构建默认URL
		previewURL = fmt.Sprintf("rtsp://%s:%s@%s:554/Streaming/Channels/101",
			device.Username, device.Password, device.IP)
	}

	debug.Debug("onvif", "生成预览URL: %s -> %s", device.Name, previewURL)
	return previewURL, nil
}

// getDeviceSnapshotURL 获取设备快照地址（优化版：使用缓存的客户端）
func (m *Manager) getDeviceSnapshotURL(device *Device) (string, error) {
	// 如果凭据未设置，跳过
	if device.Username == "" && device.Password == "" {
		return "", fmt.Errorf("设备凭据未设置，跳过获取快照URL")
	}

	// 使用缓存的SOAP客户端
	client, err := m.getOrCreateSOAPClient(device)
	if err != nil {
		return "", fmt.Errorf("获取SOAP客户端失败: %w", err)
	}

	profileToken := "main_profile"
	if len(device.Profiles) > 0 {
		profileToken = device.Profiles[0].Token
	}

	return client.GetSnapshotURI(profileToken)
}
func (m *Manager) getONVIFAddr(device *Device) string {
	// 仅使用 WS-Discovery 获取到的端点，不再构造默认路径
	if device.ONVIFAddr == "" {
		log.Printf("[ONVIF] [WARN] 设备 %s 未提供 XAddr（WS-Discovery 未返回端点）", device.DeviceID)
	}
	return device.ONVIFAddr
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
	debug.Info("onvif", "WS-Discovery服务启动 (发现 %d 个网络接口)", len(s.interfaces))

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

// parseProbeResponse 解析探测响应（改进版）
func (s *WSDiscoveryService) parseProbeResponse(data []byte) *DeviceDiscoveryResult {
	var response DiscoveryResponse
	if err := xml.Unmarshal(data, &response); err != nil {
		return s.parseProbeResponseFallback(data)
	}

	if len(response.Body.ProbeMatches.ProbeMatch) == 0 {
		return s.parseProbeResponseFallback(data)
	}

	match := response.Body.ProbeMatches.ProbeMatch[0]

	// 处理多个XAddrs(取第一个有效的)
	xaddrs := strings.Fields(match.XAddrs)

	var primaryXAddr string

	// 优先选择 HTTP 地址
	for _, xaddr := range xaddrs {
		normalized := normalizeXAddr(xaddr)

		if strings.HasPrefix(normalized, "http://") {
			primaryXAddr = normalized
			break
		}
	}

	// 如果没有 HTTP 地址，取第一个并规范化
	if primaryXAddr == "" && len(xaddrs) > 0 {
		primaryXAddr = normalizeXAddr(xaddrs[0])
	}

	if primaryXAddr == "" {
		return nil
	}

	// 验证并提取端口信息
	_, _, err := ParseXAddr(primaryXAddr)
	_ = err

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

// parseProbeResponseFallback 使用正则表达式解析响应(备用方案) - 改进版
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

	// 优先选择 HTTP 地址并规范化
	for _, xaddr := range xaddrs {
		normalized := normalizeXAddr(xaddr)

		if strings.HasPrefix(normalized, "http://") {
			primaryXAddr = normalized
			break
		}
	}

	if primaryXAddr == "" && len(xaddrs) > 0 {
		primaryXAddr = normalizeXAddr(xaddrs[0])
	}

	if primaryXAddr == "" {
		return nil
	}

	// 验证地址
	_, _, err := ParseXAddr(primaryXAddr)
	_ = err

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
