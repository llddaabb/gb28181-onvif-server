package gb28181

import (
	"fmt"
	"log"
	"net"
	"reflect"
	"strings"
	"sync"
	"time"

	"gb28181-onvif-server/internal/config"
	"gb28181-onvif-server/internal/debug"
)

// Server GB28181服务器结构体
type Server struct {
	config     *config.GB28181Config
	listener   net.Listener // TCP 监听器
	udpConn    *net.UDPConn // UDP 连接
	devices    map[string]*Device
	channels   map[string]*Channel // 通道列表
	devicesMux sync.RWMutex
	stopChan   chan struct{}
	apiServer  interface{} // API服务器引用，用于通道同步
}

// Device GB28181设备结构体
type Device struct {
	DeviceID        string     `json:"deviceId"`
	Name            string     `json:"name"`
	Manufacturer    string     `json:"manufacturer"`
	Model           string     `json:"model"`
	Firmware        string     `json:"firmware"`
	Status          string     `json:"status"`
	SipIP           string     `json:"sipIP"`
	SipPort         int        `json:"sipPort"`
	Transport       string     `json:"transport"` // TCP/UDP
	RegisterTime    int64      `json:"registerTime"`
	LastKeepAlive   int64      `json:"lastKeepAlive"`
	Expires         int        `json:"expires"`
	Channels        []*Channel `json:"channels"`
	ChannelCount    int        `json:"channelCount"`
	OnlineChannels  int        `json:"onlineChannels"`
	PTZSupported    bool       `json:"ptzSupported"`
	RecordSupported bool       `json:"recordSupported"`
	StreamMode      string     `json:"streamMode"` // TCP-Active, TCP-Passive, UDP
}

// Channel GB28181通道结构体
type Channel struct {
	ChannelID     string `json:"channelId"`
	DeviceID      string `json:"deviceId"`
	Name          string `json:"name"`
	Manufacturer  string `json:"manufacturer"`
	Model         string `json:"model"`
	Status        string `json:"status"`
	PTZType       int    `json:"ptzType"`      // 0-未知, 1-球机, 2-半球, 3-固定枪机, 4-遥控枪机
	PTZSupported  bool   `json:"ptzSupported"` // 是否支持PTZ (ptzType=1或4时为true)
	Longitude     string `json:"longitude"`
	Latitude      string `json:"latitude"`
	StreamURL     string `json:"streamURL"`
	SubStreamURL  string `json:"subStreamURL"`
	RecordingPath string `json:"recordingPath"`
	CreateTime    int64  `json:"createTime"`
}

// NewServer 创建GB28181服务器实例
func NewServer(cfg *config.GB28181Config) *Server {
	return &Server{
		config:   cfg,
		devices:  make(map[string]*Device),
		channels: make(map[string]*Channel),
		stopChan: make(chan struct{}),
	}
}

// SetAPIServer 设置API服务器引用
func (s *Server) SetAPIServer(apiServer interface{}) {
	s.apiServer = apiServer
}

// Start 启动GB28181服务器
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.SipIP, s.config.SipPort)

	// 启动 UDP 监听 (GB28181 标准主要使用 UDP)
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		debug.Error("gb28181", "解析UDP地址失败: %v", err)
		return fmt.Errorf("解析UDP地址失败: %w", err)
	}

	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		debug.Error("gb28181", "UDP监听失败: %v", err)
		return fmt.Errorf("UDP监听失败: %w", err)
	}
	s.udpConn = udpConn

	// 启动 TCP 监听 (同时支持 TCP)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		debug.Warn("gb28181", "TCP监听失败(可忽略): %v", err)
		// TCP 监听失败不影响 UDP
	} else {
		s.listener = listener
	}

	log.Printf("[GB28181] ✓ SIP服务器启动成功 (UDP+TCP监听: %s)", addr)
	debug.Info("gb28181", "服务器启动成功，监听地址: %s (UDP+TCP)", addr)
	debug.Debug("gb28181", "配置信息: SIP IP=%s, SIP Port=%d, Realm=%s, ServerID=%s",
		s.config.SipIP, s.config.SipPort, s.config.Realm, s.config.ServerID)

	// 启动 UDP 处理协程
	go s.handleUDPConnections()

	// 启动 TCP 处理协程 (如果监听成功)
	if s.listener != nil {
		go s.acceptConnections()
	}

	// 启动心跳检查协程
	go s.heartbeatChecker()

	return nil
}

// Stop 停止GB28181服务器
func (s *Server) Stop() error {
	close(s.stopChan)
	if s.udpConn != nil {
		s.udpConn.Close()
	}
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

// handleUDPConnections 处理 UDP 连接
func (s *Server) handleUDPConnections() {
	debug.Info("gb28181", "开始接受UDP消息")
	log.Println("[GB28181] 等待UDP消息...")

	buffer := make([]byte, 8192)

	for {
		select {
		case <-s.stopChan:
			debug.Info("gb28181", "停止UDP处理")
			return
		default:
			// 设置读取超时
			s.udpConn.SetReadDeadline(time.Now().Add(1 * time.Second))

			n, remoteAddr, err := s.udpConn.ReadFromUDP(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue // 超时，继续等待
				}
				debug.Warn("gb28181", "UDP读取失败: %v", err)
				continue
			}

			if n > 0 {
				data := make([]byte, n)
				copy(data, buffer[:n])
				debug.Debug("gb28181", "收到UDP消息，来自: %s, 长度: %d字节", remoteAddr, n)
				go s.handleUDPMessage(data, remoteAddr)
			}
		}
	}
}

// handleUDPMessage 处理单个 UDP 消息
func (s *Server) handleUDPMessage(data []byte, remoteAddr *net.UDPAddr) {
	// 解析SIP消息
	message, err := ParseSIPMessage(data)
	if err != nil {
		log.Printf("[ERROR] 解析SIP消息失败: %v", err)
		debug.Error("gb28181", "解析SIP消息失败: %v\n原始数据:\n%s", err, string(data))
		return
	}

	// 如果是响应，进行响应处理
	if message.IsResponse {
		debug.Debug("gb28181", "收到状态响应: %d %s 来自: %s", message.StatusCode, message.Reason, remoteAddr)
		// 对于响应，我们需要向设备发送 ACK（如果是 INVITE 的2xx响应）
		// 使用UDP连接发送 ACK
		remoteUDP := &net.UDPAddr{
			IP:   remoteAddr.IP,
			Port: remoteAddr.Port,
		}
		s.sendACKUDP(remoteUDP, message)
		return
	}

	// 根据消息类型进行处理
	debug.Debug("gb28181", "UDP SIP消息: 类型=%s, 来自=%s", message.Type, remoteAddr)

	switch message.Type {
	case "REGISTER":
		s.handleRegisterUDP(remoteAddr, message)
	case "MESSAGE":
		s.handleMessageUDP(remoteAddr, message)
	case "INVITE":
		s.handleInviteUDP(remoteAddr, message)
	case "ACK":
		debug.Debug("gb28181", "收到ACK: %s", remoteAddr)
	case "BYE":
		s.handleByeUDP(remoteAddr, message)
	case "OPTIONS":
		s.handleOptionsUDP(remoteAddr, message)
	default:
		// SIP/2.0 响应消息也可能包含目录数据
		if strings.HasPrefix(message.Type, "SIP/2.0") {
			s.handleSIPResponseUDP(remoteAddr, message)
		} else {
			debug.Warn("gb28181", "未知的SIP消息类型: %s", message.Type)
		}
	}
}

// acceptConnections 处理客户端连接
func (s *Server) acceptConnections() {
	debug.Info("gb28181", "开始接受客户端连接")

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.stopChan:
				debug.Info("gb28181", "停止接受连接")
				log.Println("[GB28181] 停止接受客户端连接")
				return
			default:
				log.Printf("[WARN] 接受连接失败: %v", err)
				debug.Warn("gb28181", "接受连接失败: %v", err)
				continue
			}
		}

		debug.Info("gb28181", "新的客户端连接: %s", conn.RemoteAddr())
		// 为每个连接创建一个会话处理协程
		go s.handleConnection(conn)
	}
}

// handleConnection 处理单个连接
func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	debug.Debug("gb28181", "处理连接: %s", conn.RemoteAddr())

	// 创建一个缓冲区来接收SIP消息
	buffer := make([]byte, 4096)

	for {
		// 接收数据
		n, err := conn.Read(buffer)
		if err != nil {
			select {
			case <-s.stopChan:
				debug.Info("gb28181", "连接处理停止: %s", conn.RemoteAddr())
				return
			default:
				debug.Warn("gb28181", "读取连接数据失败: %s - %v", conn.RemoteAddr(), err)
				return
			}
		}

		// 处理接收到的SIP消息
		if n > 0 {
			data := buffer[:n]
			debug.Debug("gb28181", "收到SIP消息，长度: %d 字节", n)
			s.HandleSIPMessage(conn, data)
		}
	}
}

// heartbeatChecker 心跳检查
func (s *Server) heartbeatChecker() {
	// 每隔10秒检查一次设备状态
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now().Unix()
			s.devicesMux.Lock()
			// 遍历所有设备，检查是否过期
			expiredDevices := []string{}
			for deviceID, device := range s.devices {
				// 使用最后心跳时间判断过期，如果心跳时间加上过期时间小于当前时间，则设备过期
				lastActive := device.LastKeepAlive
				if lastActive == 0 {
					lastActive = device.RegisterTime
				}
				if lastActive+int64(device.Expires) < now {
					expiredDevices = append(expiredDevices, deviceID)
					delete(s.devices, deviceID)
				}
			}
			if len(expiredDevices) > 0 {
				debug.Info("gb28181", "移除已过期设备: %v", expiredDevices)
			}
			s.devicesMux.Unlock()
		case <-s.stopChan:
			return
		}
	}
}

// RegisterDevice 注册设备
func (s *Server) RegisterDevice(deviceID, name, sipIP string, sipPort int, expires int) {
	s.devicesMux.Lock()
	defer s.devicesMux.Unlock()

	now := time.Now().Unix()

	// 检查设备是否已存在
	if existing, ok := s.devices[deviceID]; ok {
		// 更新现有设备
		existing.Status = "online"
		existing.SipIP = sipIP
		existing.SipPort = sipPort
		existing.RegisterTime = now
		existing.LastKeepAlive = now
		existing.Expires = expires
		debug.Info("gb28181", "设备重新注册: ID=%s | 地址=%s:%d | 有效期=%d秒", deviceID, sipIP, sipPort, expires)
		return
	}

	device := &Device{
		DeviceID:      deviceID,
		Name:          name,
		Status:        "online",
		SipIP:         sipIP,
		SipPort:       sipPort,
		Transport:     "TCP",
		RegisterTime:  now,
		LastKeepAlive: now,
		Expires:       expires,
		Channels:      make([]*Channel, 0),
		StreamMode:    "TCP-Passive",
	}

	s.devices[deviceID] = device
	log.Printf("[GB28181] ✓ 设备注册: %s (%s:%d)", deviceID, sipIP, sipPort)
}

// UpdateDeviceInfo 更新设备信息
func (s *Server) UpdateDeviceInfo(deviceID, manufacturer, model, firmware string) {
	s.devicesMux.Lock()
	defer s.devicesMux.Unlock()

	if device, ok := s.devices[deviceID]; ok {
		device.Manufacturer = manufacturer
		device.Model = model
		device.Firmware = firmware
		debug.Debug("gb28181", "设备信息更新: ID=%s | 厂商=%s | 型号=%s", deviceID, manufacturer, model)
	}
}

// UpdateKeepAlive 更新设备心跳
func (s *Server) UpdateKeepAlive(deviceID string) {
	s.devicesMux.Lock()
	defer s.devicesMux.Unlock()

	if device, ok := s.devices[deviceID]; ok {
		device.LastKeepAlive = time.Now().Unix()
		device.Status = "online"
	}
}

// AddChannel 添加或更新通道
func (s *Server) AddChannel(deviceID string, channel *Channel) {
	s.devicesMux.Lock()
	defer s.devicesMux.Unlock()

	channel.DeviceID = deviceID

	// 检查通道是否已存在
	existingChannel, exists := s.channels[channel.ChannelID]
	if exists {
		// 更新现有通道信息
		existingChannel.Name = channel.Name
		existingChannel.Manufacturer = channel.Manufacturer
		existingChannel.Model = channel.Model
		existingChannel.Status = channel.Status
		existingChannel.PTZType = channel.PTZType
		existingChannel.PTZSupported = channel.PTZType == 1 || channel.PTZType == 4
		existingChannel.Longitude = channel.Longitude
		existingChannel.Latitude = channel.Latitude
		log.Printf("[GB28181] 📺 通道更新: 设备=%s | 通道=%s | 名称=%s", deviceID, channel.ChannelID, channel.Name)
		return
	}

	// 新通道，设置创建时间
	channel.CreateTime = time.Now().Unix()
	// 设置通道的 PTZSupported: 1-球机, 4-遥控枪机 支持PTZ
	channel.PTZSupported = channel.PTZType == 1 || channel.PTZType == 4

	// 添加到通道映射
	s.channels[channel.ChannelID] = channel

	// 添加到设备的通道列表
	if device, ok := s.devices[deviceID]; ok {
		device.Channels = append(device.Channels, channel)
		device.ChannelCount = len(device.Channels)
		if channel.Status == "ON" || channel.Status == "online" {
			device.OnlineChannels++
		}
		// 设备有任何支持PTZ的通道则设备支持PTZ
		if channel.PTZSupported {
			device.PTZSupported = true
		}
		log.Printf("[GB28181] 📺 通道添加: 设备=%s | 通道=%s | 名称=%s | PTZType=%d | PTZSupported=%v", deviceID, channel.ChannelID, channel.Name, channel.PTZType, channel.PTZSupported)
	}

	// 同步到API服务器的通道管理器
	if s.apiServer != nil {
		// 使用反射调用SyncGB28181Channel方法
		apiServerValue := reflect.ValueOf(s.apiServer)
		method := apiServerValue.MethodByName("SyncGB28181Channel")
		if method.IsValid() {
			// 调用方法
			result := method.Call([]reflect.Value{reflect.ValueOf(channel)})
			if len(result) > 0 && !result[0].IsNil() {
				if err, ok := result[0].Interface().(error); ok && err != nil {
					log.Printf("[GB28181] ⚠ 同步通道到API失败: %v", err)
				} else {
					log.Printf("[GB28181] ✓ 通道同步到API成功: %s", channel.ChannelID)
				}
			}
		} else {
			log.Printf("[GB28181] ⚠ API服务器未提供SyncGB28181Channel方法")
		}
	}
}

// GetChannels 获取设备的所有通道
func (s *Server) GetChannels(deviceID string) []*Channel {
	s.devicesMux.RLock()
	defer s.devicesMux.RUnlock()

	if device, ok := s.devices[deviceID]; ok {
		return device.Channels
	}
	return nil
}

// GetChannelByID 根据ID获取通道
func (s *Server) GetChannelByID(channelID string) (*Channel, bool) {
	s.devicesMux.RLock()
	defer s.devicesMux.RUnlock()

	channel, exists := s.channels[channelID]
	return channel, exists
}

// RemoveDevice 移除设备
func (s *Server) RemoveDevice(deviceID string) bool {
	s.devicesMux.Lock()
	defer s.devicesMux.Unlock()

	if device, ok := s.devices[deviceID]; ok {
		// 移除设备的所有通道
		for _, ch := range device.Channels {
			delete(s.channels, ch.ChannelID)
		}
		delete(s.devices, deviceID)
		log.Printf("[GB28181] 🗑️ 设备移除: ID=%s", deviceID)
		return true
	}
	return false
}

// GetStatistics 获取统计信息
func (s *Server) GetStatistics() map[string]interface{} {
	s.devicesMux.RLock()
	defer s.devicesMux.RUnlock()

	total := len(s.devices)
	online := 0
	offline := 0
	ptzDevices := 0
	totalChannels := 0
	onlineChannels := 0

	for _, device := range s.devices {
		if device.Status == "online" {
			online++
		} else {
			offline++
		}
		if device.PTZSupported {
			ptzDevices++
		}
		totalChannels += device.ChannelCount
		onlineChannels += device.OnlineChannels
	}

	return map[string]interface{}{
		"total":          total,
		"online":         online,
		"offline":        offline,
		"ptzDevices":     ptzDevices,
		"totalChannels":  totalChannels,
		"onlineChannels": onlineChannels,
	}
}

// GetDevices 获取所有设备
func (s *Server) GetDevices() []*Device {
	s.devicesMux.RLock()
	defer s.devicesMux.RUnlock()

	devices := make([]*Device, 0, len(s.devices))
	for _, device := range s.devices {
		devices = append(devices, device)
	}

	return devices
}

// GetDeviceByID 根据ID获取设备
func (s *Server) GetDeviceByID(deviceID string) (*Device, bool) {
	s.devicesMux.RLock()
	defer s.devicesMux.RUnlock()

	device, exists := s.devices[deviceID]
	return device, exists
}

// QueryCatalog 查询设备目录（获取通道列表）
func (s *Server) QueryCatalog(deviceID string) error {
	s.devicesMux.RLock()
	device, exists := s.devices[deviceID]
	s.devicesMux.RUnlock()

	if !exists {
		return fmt.Errorf("设备 %s 不存在", deviceID)
	}

	// 生成目录查询 SIP MESSAGE
	sn := time.Now().UnixNano() % 1000000
	catalogXML := fmt.Sprintf(`<?xml version="1.0" encoding="GB2312"?>
<Query>
<CmdType>Catalog</CmdType>
<SN>%d</SN>
<DeviceID>%s</DeviceID>
</Query>`, sn, deviceID)

	// 构建 SIP MESSAGE 请求
	callID := fmt.Sprintf("%d@%s", time.Now().UnixNano(), s.config.SipIP)
	branch := fmt.Sprintf("z9hG4bK%d", time.Now().UnixNano())
	tag := fmt.Sprintf("%d", time.Now().UnixNano()%100000000)

	sipMessage := fmt.Sprintf("MESSAGE sip:%s@%s:%d SIP/2.0\r\n"+
		"Via: SIP/2.0/UDP %s:%d;rport;branch=%s\r\n"+
		"From: <sip:%s@%s>;tag=%s\r\n"+
		"To: <sip:%s@%s:%d>\r\n"+
		"Call-ID: %s\r\n"+
		"CSeq: 1 MESSAGE\r\n"+
		"Content-Type: Application/MANSCDP+xml\r\n"+
		"Max-Forwards: 70\r\n"+
		"Content-Length: %d\r\n\r\n%s",
		deviceID, device.SipIP, device.SipPort,
		s.config.SipIP, s.config.SipPort, branch,
		s.config.ServerID, s.config.Realm, tag,
		deviceID, device.SipIP, device.SipPort,
		callID,
		len(catalogXML), catalogXML)

	// 发送到设备
	remoteAddr := &net.UDPAddr{
		IP:   net.ParseIP(device.SipIP),
		Port: device.SipPort,
	}

	if s.udpConn != nil {
		_, err := s.udpConn.WriteToUDP([]byte(sipMessage), remoteAddr)
		if err != nil {
			log.Printf("[GB28181] 发送目录查询失败: %v", err)
			return err
		}
		log.Printf("[GB28181] ✓ 已向设备 %s 发送目录查询请求", deviceID)
	}

	return nil
}

// QueryDeviceInfo 查询设备信息
func (s *Server) QueryDeviceInfo(deviceID string) error {
	s.devicesMux.RLock()
	device, exists := s.devices[deviceID]
	s.devicesMux.RUnlock()

	if !exists {
		return fmt.Errorf("设备 %s 不存在", deviceID)
	}

	// 生成设备信息查询 XML
	sn := time.Now().UnixNano() % 1000000
	queryXML := fmt.Sprintf(`<?xml version="1.0" encoding="GB2312"?>
<Query>
<CmdType>DeviceInfo</CmdType>
<SN>%d</SN>
<DeviceID>%s</DeviceID>
</Query>`, sn, deviceID)

	// 构建 SIP MESSAGE 请求
	callID := fmt.Sprintf("%d@%s", time.Now().UnixNano(), s.config.SipIP)
	branch := fmt.Sprintf("z9hG4bK%d", time.Now().UnixNano())
	tag := fmt.Sprintf("%d", time.Now().UnixNano()%100000000)

	sipMessage := fmt.Sprintf("MESSAGE sip:%s@%s:%d SIP/2.0\r\n"+
		"Via: SIP/2.0/UDP %s:%d;rport;branch=%s\r\n"+
		"From: <sip:%s@%s>;tag=%s\r\n"+
		"To: <sip:%s@%s:%d>\r\n"+
		"Call-ID: %s\r\n"+
		"CSeq: 1 MESSAGE\r\n"+
		"Content-Type: Application/MANSCDP+xml\r\n"+
		"Max-Forwards: 70\r\n"+
		"Content-Length: %d\r\n\r\n%s",
		deviceID, device.SipIP, device.SipPort,
		s.config.SipIP, s.config.SipPort, branch,
		s.config.ServerID, s.config.Realm, tag,
		deviceID, device.SipIP, device.SipPort,
		callID,
		len(queryXML), queryXML)

	// 发送到设备
	remoteAddr := &net.UDPAddr{
		IP:   net.ParseIP(device.SipIP),
		Port: device.SipPort,
	}

	if s.udpConn != nil {
		_, err := s.udpConn.WriteToUDP([]byte(sipMessage), remoteAddr)
		if err != nil {
			log.Printf("[GB28181] 发送设备信息查询失败: %v", err)
			return err
		}
		log.Printf("[GB28181] ✓ 已向设备 %s 发送设备信息查询请求", deviceID)
	}

	return nil
}
