package gb28181

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// MediaSession 媒体会话
type MediaSession struct {
	SessionID  string `json:"session_id"`  // 会话ID
	DeviceID   string `json:"device_id"`   // 设备ID
	ChannelID  string `json:"channel_id"`  // 通道ID
	CallID     string `json:"call_id"`     // SIP Call-ID
	FromTag    string `json:"from_tag"`    // From Tag
	ToTag      string `json:"to_tag"`      // To Tag
	StreamID   string `json:"stream_id"`   // ZLM 流ID
	RTPPort    int    `json:"rtp_port"`    // RTP 接收端口
	SSRC       string `json:"ssrc"`        // SSRC
	Status     string `json:"status"`      // 状态: inviting, playing, stopped
	CreateTime int64  `json:"create_time"` // 创建时间
	StartTime  int64  `json:"start_time"`  // 开始播放时间
	MediaIP    string `json:"media_ip"`    // 媒体服务器IP
	MediaPort  int    `json:"media_port"`  // 媒体服务器端口
	Transport  string `json:"transport"`   // 传输协议 TCP/UDP
}

// MediaSessionManager 媒体会话管理器
type MediaSessionManager struct {
	sessions map[string]*MediaSession // key: deviceID_channelID
	mutex    sync.RWMutex
}

var sessionManager = &MediaSessionManager{
	sessions: make(map[string]*MediaSession),
}

// GetSessionManager 获取会话管理器
func GetSessionManager() *MediaSessionManager {
	return sessionManager
}

// GetSession 获取会话
func (m *MediaSessionManager) GetSession(deviceID, channelID string) *MediaSession {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	key := fmt.Sprintf("%s_%s", deviceID, channelID)
	return m.sessions[key]
}

// AddSession 添加会话
func (m *MediaSessionManager) AddSession(session *MediaSession) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	key := fmt.Sprintf("%s_%s", session.DeviceID, session.ChannelID)
	m.sessions[key] = session
}

// RemoveSession 移除会话
func (m *MediaSessionManager) RemoveSession(deviceID, channelID string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	key := fmt.Sprintf("%s_%s", deviceID, channelID)
	delete(m.sessions, key)
}

// GetAllSessions 获取所有会话
func (m *MediaSessionManager) GetAllSessions() []*MediaSession {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	sessions := make([]*MediaSession, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

// generateSSRC 生成 SSRC
// GB28181 规定 SSRC 为10位，格式: 1位(实时/历史) + 4位域 + 5位序号
func generateSSRC(realm string, isPlayback bool) string {
	prefix := "0" // 0: 实时, 1: 历史
	if isPlayback {
		prefix = "1"
	}

	// 取域的后4位
	domainPart := realm
	if len(domainPart) > 4 {
		domainPart = domainPart[len(domainPart)-4:]
	} else {
		domainPart = fmt.Sprintf("%04s", domainPart)
	}

	// 生成5位序号
	seq := fmt.Sprintf("%05d", time.Now().UnixNano()%100000)

	return prefix + domainPart + seq
}

// generateStreamID 生成流ID
func generateStreamID(deviceID, channelID string) string {
	// 使用设备ID和通道ID组合，移除特殊字符
	streamID := strings.ReplaceAll(deviceID, "-", "")
	if channelID != "" && channelID != deviceID {
		streamID = streamID + "_" + strings.ReplaceAll(channelID, "-", "")
	}
	return streamID
}

// InviteRequest 发起实时视频请求
// 向设备发送 INVITE 请求，请求设备推送 PS 流
func (s *Server) InviteRequest(deviceID, channelID string, rtpPort int, mediaIP string) (*MediaSession, error) {
	// 获取设备信息
	device, exists := s.GetDeviceByID(deviceID)
	if !exists {
		return nil, fmt.Errorf("设备不存在: %s", deviceID)
	}

	// 如果没有指定通道ID，使用设备ID
	if channelID == "" {
		channelID = deviceID
	}

	// 检查是否已有会话
	if session := sessionManager.GetSession(deviceID, channelID); session != nil {
		if session.Status == "playing" {
			return session, nil // 返回现有会话
		}
		// 清理旧会话
		sessionManager.RemoveSession(deviceID, channelID)
	}

	// 确保 mediaIP 有效且设备可以访问
	if mediaIP == "" || mediaIP == "0.0.0.0" || mediaIP == "::" || mediaIP == "localhost" {
		// 使用设备地址所在的网络获取本机IP
		mediaIP = device.SipIP
		if mediaIP == "" || mediaIP == "0.0.0.0" || mediaIP == "::" {
			mediaIP = "127.0.0.1"
		}
	}

	// 尝试获取与设备在同一网段的本机IP（总是探测一次，避免选到不可达接口）
	// 通过与设备通信的本地地址来获取IP
	conn, err := net.Dial("udp", device.SipIP+":"+strconv.Itoa(device.SipPort))
	if err == nil {
		defer conn.Close()
		localAddr := conn.LocalAddr().String()
		if idx := strings.LastIndex(localAddr, ":"); idx > 0 {
			candidateIP := localAddr[:idx]
			// 检查是否是有效的IP且不是回环地址
			if candidateIP != "" && candidateIP != "127.0.0.1" && candidateIP != "::1" {
				mediaIP = candidateIP
			}
		}
	}

	log.Printf("[GB28181] INVITE 媒体IP最终决定: %s (设备IP: %s)", mediaIP, device.SipIP)

	// 生成会话参数
	callID := generateCallID()
	fromTag := generateTag()
	ssrc := generateSSRC(s.config.Realm, false)
	streamID := generateStreamID(deviceID, channelID)

	// 创建会话
	session := &MediaSession{
		SessionID:  fmt.Sprintf("session_%d", time.Now().UnixNano()),
		DeviceID:   deviceID,
		ChannelID:  channelID,
		CallID:     callID,
		FromTag:    fromTag,
		StreamID:   streamID,
		RTPPort:    rtpPort,
		SSRC:       ssrc,
		Status:     "inviting",
		CreateTime: time.Now().Unix(),
		MediaIP:    mediaIP,
		MediaPort:  rtpPort,
		Transport:  device.Transport,
	}

	// 构建 SDP
	sdp := s.buildInviteSDP(session, mediaIP)

	// 构建 INVITE 消息，优先使用 mediaIP 作为本地 SIP 地址
	inviteMsg := s.buildInviteMessage(device, channelID, callID, fromTag, sdp, mediaIP)

	log.Printf("[GB28181] 发送 INVITE 请求: deviceID=%s, channelID=%s, rtpPort=%d, ssrc=%s, mediaIP=%s",
		deviceID, channelID, rtpPort, ssrc, mediaIP)

	// 发送 INVITE
	err = s.sendSIPMessageUDP(device, inviteMsg)
	if err != nil {
		return nil, fmt.Errorf("发送 INVITE 失败: %v", err)
	}

	// 保存会话
	sessionManager.AddSession(session)

	return session, nil
}

// buildInviteSDP 构建 INVITE SDP
func (s *Server) buildInviteSDP(session *MediaSession, mediaIP string) string {
	// SDP 内容
	// GB28181 使用 PS 流封装
	sdp := fmt.Sprintf(`v=0
o=%s 0 0 IN IP4 %s
s=Play
c=IN IP4 %s
t=0 0
m=video %d RTP/AVP 96
a=recvonly
a=rtpmap:96 PS/90000
y=%s
`, s.config.ServerID, mediaIP, mediaIP, session.RTPPort, session.SSRC)

	return sdp
}

// buildInviteMessage 构建 INVITE SIP 消息
func (s *Server) buildInviteMessage(device *Device, channelID, callID, fromTag, sdp string, localIP string) string {
	// 请求URI
	requestURI := fmt.Sprintf("sip:%s@%s:%d", channelID, device.SipIP, device.SipPort)

	// Via
	// 如果提供了 localIP 且非空，则使用它作为本地地址，否则回退到配置
	localAddrIP := s.config.SipIP
	if localIP != "" && localIP != "0.0.0.0" && localIP != "::" {
		localAddrIP = localIP
	}

	via := fmt.Sprintf("SIP/2.0/%s %s:%d;rport;branch=z9hG4bK%d",
		device.Transport, localAddrIP, s.config.SipPort, time.Now().UnixNano())

	// From
	from := fmt.Sprintf("<sip:%s@%s:%d>;tag=%s",
		s.config.ServerID, localAddrIP, s.config.SipPort, fromTag)

	// To
	to := fmt.Sprintf("<sip:%s@%s:%d>", channelID, device.SipIP, device.SipPort)

	// Contact
	contact := fmt.Sprintf("<sip:%s@%s:%d>", s.config.ServerID, localAddrIP, s.config.SipPort)

	// 构建消息
	msg := fmt.Sprintf("INVITE %s SIP/2.0\r\n", requestURI)
	msg += fmt.Sprintf("Via: %s\r\n", via)
	msg += fmt.Sprintf("From: %s\r\n", from)
	msg += fmt.Sprintf("To: %s\r\n", to)
	msg += fmt.Sprintf("Call-ID: %s\r\n", callID)
	msg += "CSeq: 1 INVITE\r\n"
	msg += fmt.Sprintf("Contact: %s\r\n", contact)
	msg += "Content-Type: APPLICATION/SDP\r\n"
	msg += "Max-Forwards: 70\r\n"
	msg += fmt.Sprintf("Content-Length: %d\r\n", len(sdp))
	msg += "\r\n"
	msg += sdp

	return msg
}

// sendSIPMessageUDP 通过 UDP 发送 SIP 消息
func (s *Server) sendSIPMessageUDP(device *Device, message string) error {
	addr := net.JoinHostPort(device.SipIP, strconv.Itoa(device.SipPort))

	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return fmt.Errorf("解析地址失败: %v", err)
	}

	// 使用现有的 UDP 连接发送
	if s.udpConn != nil {
		_, err = s.udpConn.WriteToUDP([]byte(message), udpAddr)
		if err != nil {
			return fmt.Errorf("UDP发送失败: %v", err)
		}
		log.Printf("[GB28181] ✓ INVITE 已发送到 %s", addr)
		return nil
	}

	return fmt.Errorf("UDP连接不可用")
}

// ByeRequest 发送 BYE 请求停止流
func (s *Server) ByeRequest(deviceID, channelID string) error {
	session := sessionManager.GetSession(deviceID, channelID)
	if session == nil {
		return fmt.Errorf("会话不存在")
	}

	device, exists := s.GetDeviceByID(deviceID)
	if !exists {
		// 设备不存在，直接清理会话
		sessionManager.RemoveSession(deviceID, channelID)
		return nil
	}

	// 构建 BYE 消息
	byeMsg := s.buildByeMessage(device, session)

	log.Printf("[GB28181] 发送 BYE 请求: deviceID=%s, channelID=%s", deviceID, channelID)

	// 发送 BYE
	err := s.sendSIPMessageUDP(device, byeMsg)
	if err != nil {
		log.Printf("[GB28181] 发送 BYE 失败: %v", err)
	}

	// 清理会话
	session.Status = "stopped"
	sessionManager.RemoveSession(deviceID, channelID)

	return nil
}

// buildByeMessage 构建 BYE SIP 消息
func (s *Server) buildByeMessage(device *Device, session *MediaSession) string {
	// 请求URI
	requestURI := fmt.Sprintf("sip:%s@%s:%d", session.ChannelID, device.SipIP, device.SipPort)

	// Via
	via := fmt.Sprintf("SIP/2.0/%s %s:%d;rport;branch=z9hG4bK%d",
		device.Transport, s.config.SipIP, s.config.SipPort, time.Now().UnixNano())

	// From (与 INVITE 相同)
	from := fmt.Sprintf("<sip:%s@%s:%d>;tag=%s",
		s.config.ServerID, s.config.SipIP, s.config.SipPort, session.FromTag)

	// To
	to := fmt.Sprintf("<sip:%s@%s:%d>", session.ChannelID, device.SipIP, device.SipPort)
	if session.ToTag != "" {
		to += fmt.Sprintf(";tag=%s", session.ToTag)
	}

	// 构建消息
	msg := fmt.Sprintf("BYE %s SIP/2.0\r\n", requestURI)
	msg += fmt.Sprintf("Via: %s\r\n", via)
	msg += fmt.Sprintf("From: %s\r\n", from)
	msg += fmt.Sprintf("To: %s\r\n", to)
	msg += fmt.Sprintf("Call-ID: %s\r\n", session.CallID)
	msg += "CSeq: 2 BYE\r\n"
	msg += "Max-Forwards: 70\r\n"
	msg += "Content-Length: 0\r\n"
	msg += "\r\n"

	return msg
}

// HandleInviteResponse 处理 INVITE 响应
func (s *Server) HandleInviteResponse(statusCode int, callID, toTag string) {
	// 查找对应的会话
	sessions := sessionManager.GetAllSessions()
	for _, session := range sessions {
		if session.CallID == callID {
			session.ToTag = toTag
			if statusCode == 200 {
				session.Status = "playing"
				session.StartTime = time.Now().Unix()
				log.Printf("[GB28181] ✓ INVITE 成功: deviceID=%s, channelID=%s",
					session.DeviceID, session.ChannelID)
			} else {
				session.Status = "failed"
				log.Printf("[GB28181] ✗ INVITE 失败: statusCode=%d, deviceID=%s",
					statusCode, session.DeviceID)
			}
			return
		}
	}
}

// GetMediaSession 获取媒体会话
func (s *Server) GetMediaSession(deviceID, channelID string) *MediaSession {
	return sessionManager.GetSession(deviceID, channelID)
}

// GetAllMediaSessions 获取所有媒体会话
func (s *Server) GetAllMediaSessions() []*MediaSession {
	return sessionManager.GetAllSessions()
}
