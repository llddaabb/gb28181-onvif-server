package gb28181

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// nonceStore 保存已发送的 nonce 用于认证验证
var nonceStore = struct {
	sync.RWMutex
	nonces map[string]int64 // nonce -> 创建时间
}{
	nonces: make(map[string]int64),
}

// CatalogResponse GB28181 目录响应结构
type CatalogResponse struct {
	XMLName    xml.Name       `xml:"Response"`
	CmdType    string         `xml:"CmdType"`
	SN         int            `xml:"SN"`
	DeviceID   string         `xml:"DeviceID"`
	SumNum     int            `xml:"SumNum"`
	DeviceList CatalogDevices `xml:"DeviceList"`
}

type CatalogDevices struct {
	Num     int             `xml:"Num,attr"`
	Devices []CatalogDevice `xml:"Item"`
}

type CatalogDevice struct {
	DeviceID     string `xml:"DeviceID"`
	Name         string `xml:"Name"`
	Manufacturer string `xml:"Manufacturer"`
	Model        string `xml:"Model"`
	Owner        string `xml:"Owner"`
	CivilCode    string `xml:"CivilCode"`
	Address      string `xml:"Address"`
	Parental     int    `xml:"Parental"`
	ParentID     string `xml:"ParentID"`
	SafetyWay    int    `xml:"SafetyWay"`
	RegisterWay  int    `xml:"RegisterWay"`
	Secrecy      int    `xml:"Secrecy"`
	Status       string `xml:"Status"`
	Longitude    string `xml:"Longitude"`
	Latitude     string `xml:"Latitude"`
	PTZType      int    `xml:"PTZType"`
}

// DeviceInfoResponse 设备信息响应结构
type DeviceInfoResponse struct {
	XMLName      xml.Name `xml:"Response"`
	CmdType      string   `xml:"CmdType"`
	SN           int      `xml:"SN"`
	DeviceID     string   `xml:"DeviceID"`
	DeviceName   string   `xml:"DeviceName"`
	Result       string   `xml:"Result"`
	Manufacturer string   `xml:"Manufacturer"`
	Model        string   `xml:"Model"`
	Firmware     string   `xml:"Firmware"`
	Channel      int      `xml:"Channel"`
}

// SIPMessage SIP消息结构体
type SIPMessage struct {
	Type    string            // 请求类型: REGISTER, INVITE, ACK, BYE, MESSAGE 等
	Headers map[string]string // SIP头字段
	Body    string            // 消息体
}

// ParseSIPMessage 解析SIP消息
func ParseSIPMessage(data []byte) (*SIPMessage, error) {
	reader := bufio.NewReader(strings.NewReader(string(data)))

	// 解析请求行
	requestLine, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("解析请求行失败: %w", err)
	}
	requestLine = strings.TrimSpace(requestLine)
	parts := strings.SplitN(requestLine, " ", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("无效的请求行: %s", requestLine)
	}

	message := &SIPMessage{
		Type:    parts[0],
		Headers: make(map[string]string),
	}

	// 解析头字段
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("解析头字段失败: %w", err)
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break // 头字段结束
		}

		// 处理折叠行
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			// 这是一个折叠行，添加到上一个头字段
			lastKey := ""
			for k := range message.Headers {
				lastKey = k
			}
			if lastKey != "" {
				message.Headers[lastKey] += " " + strings.TrimSpace(line)
			}
		} else {
			// 普通头字段
			colonIndex := strings.Index(line, ":")
			if colonIndex == -1 {
				return nil, fmt.Errorf("无效的头字段: %s", line)
			}
			key := line[:colonIndex]
			value := strings.TrimSpace(line[colonIndex+1:])
			message.Headers[key] = value
		}
	}

	// 解析消息体 - 读取剩余所有内容
	var bodyBuilder strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			// EOF或其他错误，将已读取的部分作为body
			if line != "" {
				bodyBuilder.WriteString(line)
			}
			break
		}
		bodyBuilder.WriteString(line)
	}
	message.Body = strings.TrimSpace(bodyBuilder.String())

	return message, nil
}

// BuildSIPResponse 构建SIP响应消息
func BuildSIPResponse(request *SIPMessage, statusCode int, reasonPhrase string) []byte {
	// 获取CSeq头
	cseq := request.Headers["CSeq"]
	if cseq == "" {
		cseq = "1 REGISTER" // 默认值
	}

	// 构建响应行
	responseLine := fmt.Sprintf("SIP/2.0 %d %s\r\n", statusCode, reasonPhrase)

	// 构建头字段
	headers := ""
	headers += fmt.Sprintf("Via: %s\r\n", request.Headers["Via"])
	headers += fmt.Sprintf("From: %s\r\n", request.Headers["From"])
	headers += fmt.Sprintf("To: %s\r\n", request.Headers["To"])
	headers += fmt.Sprintf("Call-ID: %s\r\n", request.Headers["Call-ID"])
	headers += fmt.Sprintf("CSeq: %s\r\n", cseq)
	headers += fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123))
	headers += "Content-Length: 0\r\n"

	// 组合完整响应
	response := responseLine + headers + "\r\n"

	return []byte(response)
}

// HandleSIPMessage 处理SIP消息
func (s *Server) HandleSIPMessage(conn net.Conn, data []byte) {
	// 解析SIP消息
	message, err := ParseSIPMessage(data)
	if err != nil {
		log.Printf("[ERROR] 解析SIP消息失败: %v", err)
		return
	}

	// 根据消息类型进行处理
	log.Printf("[SIP] 收到消息类型: %s 来自: %s", message.Type, conn.RemoteAddr())
	switch message.Type {
	case "REGISTER":
		s.handleRegister(conn, message)
	case "INVITE":
		s.handleInvite(conn, message)
	case "ACK":
		s.handleAck(conn, message)
	case "BYE":
		s.handleBye(conn, message)
	case "MESSAGE":
		s.handleMessage(conn, message)
	case "OPTIONS":
		s.handleOptions(conn, message)
	default:
		log.Printf("[WARN] 未知的SIP消息类型: %s", message.Type)
	}
}

// handleRegister 处理注册请求
func (s *Server) handleRegister(conn net.Conn, message *SIPMessage) {
	// 解析From头获取设备信息
	fromHeader := message.Headers["From"]
	if fromHeader == "" {
		log.Println("REGISTER消息缺少From头")
		response := BuildSIPResponse(message, 400, "Bad Request")
		conn.Write(response)
		return
	}

	// 从From头中提取设备ID
	// From: <sip:34020000001320000001@3402000000>;tag=123456
	deviceID := extractDeviceID(fromHeader)
	if deviceID == "" {
		log.Println("[WARN] 无法从From头提取设备ID")
		response := BuildSIPResponse(message, 400, "Bad Request")
		conn.Write(response)
		return
	}

	log.Printf("[REGISTER] 设备ID: %s", deviceID)

	// 认证检查
	if !s.authenticateMessage(message) {
		// 发送401未授权响应
		log.Printf("[WARN] 设备 %s 认证失败", deviceID)
		realm := s.config.Realm
		response := []byte(fmt.Sprintf("SIP/2.0 401 Unauthorized\r\n"+"Via: %s\r\n"+"From: %s\r\n"+"To: %s\r\n"+"Call-ID: %s\r\n"+"CSeq: %s\r\n"+"WWW-Authenticate: Digest realm=\"%s\", nonce=\"%s\", algorithm=MD5\r\n"+"Content-Length: 0\r\n\r\n",
			message.Headers["Via"],
			message.Headers["From"],
			message.Headers["To"],
			message.Headers["Call-ID"],
			message.Headers["CSeq"],
			realm,
			generateNonce(),
		))
		conn.Write(response)
		return
	}

	// 解析Contact头获取设备IP和端口
	contactHeader := message.Headers["Contact"]
	if contactHeader == "" {
		log.Println("[WARN] REGISTER消息缺少Contact头")
		response := BuildSIPResponse(message, 400, "Bad Request")
		conn.Write(response)
		return
	}

	ip, port := extractIPPortFromContact(contactHeader)
	if ip == "" {
		log.Println("[WARN] 无法从Contact头提取IP和端口")
		response := BuildSIPResponse(message, 400, "Bad Request")
		conn.Write(response)
		return
	}

	log.Printf("[REGISTER] 设备地址: %s:%d", ip, port)

	// 解析Expires头
	expires := 3600 // 默认3600秒
	expiresHeader := message.Headers["Expires"]
	if expiresHeader != "" {
		if e, err := strconv.Atoi(expiresHeader); err == nil {
			expires = e
		}
	}

	// 注册设备
	s.RegisterDevice(deviceID, "", ip, port, expires)

	// 发送200 OK响应
	log.Printf("[REGISTER] ✓ 设备 %s 注册成功，有效期: %d秒", deviceID, expires)
	response := BuildSIPResponse(message, 200, "OK")
	conn.Write(response)
}

// authenticateMessage 认证SIP消息
func (s *Server) authenticateMessage(message *SIPMessage) bool {
	// 如果配置中没有设置密码，则跳过认证
	if s.config.Password == "" {
		log.Println("[AUTH] 未配置密码，跳过认证")
		return true
	}

	// 获取Authorization头
	authHeader := message.Headers["Authorization"]
	if authHeader == "" {
		log.Println("[AUTH] 未找到Authorization头，需要认证")
		return false
	}

	log.Printf("[AUTH] 收到Authorization头: %s", authHeader)

	// 解析Authorization头
	// Authorization: Digest username="34020000001320000001", realm="3402000000", nonce="123456", uri="sip:...", response="..."
	params := parseAuthParams(authHeader)

	// GB28181 使用设备ID作为用户名
	username, ok := params["username"]
	if !ok {
		log.Println("[AUTH] Authorization头缺少username")
		return false
	}
	log.Printf("[AUTH] 用户名(设备ID): %s", username)

	realm, ok := params["realm"]
	if !ok {
		log.Println("[AUTH] Authorization头缺少realm")
		return false
	}
	log.Printf("[AUTH] Realm: %s", realm)

	nonce, ok := params["nonce"]
	if !ok {
		log.Println("[AUTH] Authorization头缺少nonce")
		return false
	}

	// 验证 nonce 是否有效（是我们之前发送的）
	if !isValidNonce(nonce) {
		log.Printf("[AUTH] nonce 无效或已过期: %s", nonce)
		return false
	}

	uri, ok := params["uri"]
	if !ok {
		log.Println("[AUTH] Authorization头缺少uri")
		return false
	}

	response, ok := params["response"]
	if !ok {
		log.Println("[AUTH] Authorization头缺少response")
		return false
	}

	log.Printf("[AUTH] 认证参数: nonce=%s, uri=%s", nonce, uri)
	log.Printf("[AUTH] 客户端response: %s", response)

	// 计算期望的响应值
	// A1 = username:realm:password
	A1 := fmt.Sprintf("%s:%s:%s", username, realm, s.config.Password)
	md5A1 := md5.Sum([]byte(A1))

	// A2 = method:uri
	A2 := fmt.Sprintf("%s:%s", message.Type, uri)
	md5A2 := md5.Sum([]byte(A2))

	// response = md5(md5(A1):nonce:md5(A2))
	expectedResponse := fmt.Sprintf("%s:%s:%s", hex.EncodeToString(md5A1[:]), nonce, hex.EncodeToString(md5A2[:]))
	md5ExpectedResponse := md5.Sum([]byte(expectedResponse))
	expectedResponseHex := hex.EncodeToString(md5ExpectedResponse[:])

	log.Printf("[AUTH] 计算response: %s", expectedResponseHex)

	// 比较计算出的响应值和客户端提供的响应值
	if expectedResponseHex == response {
		log.Println("[AUTH] ✓ 认证成功")
		return true
	}

	log.Printf("[AUTH] ✗ 认证失败: 响应值不匹配")
	log.Printf("[AUTH] A1=%s, md5(A1)=%s", A1, hex.EncodeToString(md5A1[:]))
	log.Printf("[AUTH] A2=%s, md5(A2)=%s", A2, hex.EncodeToString(md5A2[:]))
	return false
}

// parseAuthParams 解析认证头参数
func parseAuthParams(authHeader string) map[string]string {
	params := make(map[string]string)

	// 跳过 "Digest " 前缀
	authHeader = strings.TrimPrefix(authHeader, "Digest ")

	// 分割参数
	pairs := strings.Split(authHeader, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			// 移除引号
			value = strings.Trim(value, "\"")
			params[key] = value
		}
	}

	return params
}

// generateNonce 生成随机的nonce值并保存
func generateNonce() string {
	// 使用当前时间戳和随机数生成nonce
	timestamp := time.Now().UnixNano()
	random := fmt.Sprintf("%d", timestamp)
	hash := md5.Sum([]byte(random))
	nonce := hex.EncodeToString(hash[:])

	// 保存 nonce
	nonceStore.Lock()
	nonceStore.nonces[nonce] = time.Now().Unix()
	// 清理过期的 nonce（超过5分钟）
	now := time.Now().Unix()
	for n, t := range nonceStore.nonces {
		if now-t > 300 {
			delete(nonceStore.nonces, n)
		}
	}
	nonceStore.Unlock()

	return nonce
}

// isValidNonce 检查 nonce 是否有效
func isValidNonce(nonce string) bool {
	nonceStore.RLock()
	defer nonceStore.RUnlock()
	_, exists := nonceStore.nonces[nonce]
	return exists
}

// handleInvite 处理INVITE请求（实时流请求）
func (s *Server) handleInvite(conn net.Conn, message *SIPMessage) {
	// 解析设备ID
	fromHeader := message.Headers["From"]
	deviceID := extractDeviceID(fromHeader)
	if deviceID == "" {
		log.Println("[WARN] INVITE消息缺少有效设备ID")
		response := BuildSIPResponse(message, 400, "Bad Request")
		conn.Write(response)
		return
	}

	log.Printf("[INVITE] 设备 %s 请求媒体流", deviceID)

	// 这里需要处理媒体流协商（SDP）
	// 简化处理，直接返回200 OK
	response := BuildSIPResponse(message, 200, "OK")
	conn.Write(response)
	log.Printf("[INVITE] ✓ 已接受设备 %s 的媒体流请求", deviceID)
}

// handleAck 处理ACK请求
func (s *Server) handleAck(conn net.Conn, message *SIPMessage) {
	// ACK不需要响应
	log.Printf("[ACK] 收到确认请求")
}

// handleBye 处理BYE请求（结束会话）
func (s *Server) handleBye(conn net.Conn, message *SIPMessage) {
	// 发送200 OK响应
	response := BuildSIPResponse(message, 200, "OK")
	conn.Write(response)
	log.Printf("[BYE] 会话已结束")
}

// handleMessage 处理MESSAGE请求（GB28181中的设备信息查询等）
func (s *Server) handleMessage(conn net.Conn, message *SIPMessage) {
	// 简化处理，返回200 OK
	response := BuildSIPResponse(message, 200, "OK")
	conn.Write(response)
	log.Printf("[MESSAGE] 设备消息请求已处理")
}

// handleOptions 处理OPTIONS请求
func (s *Server) handleOptions(conn net.Conn, message *SIPMessage) {
	// 发送200 OK响应
	response := BuildSIPResponse(message, 200, "OK")
	conn.Write(response)
	log.Printf("[OPTIONS] 收到心跳检测请求")
}

// extractDeviceID 从From头中提取设备ID
func extractDeviceID(fromHeader string) string {
	// From: <sip:34020000001320000001@3402000000>;tag=123456
	start := strings.Index(fromHeader, "<sip:")
	if start == -1 {
		return ""
	}
	start += 5 // 跳过 "<sip:"

	end := strings.Index(fromHeader[start:], "@")
	if end == -1 {
		return ""
	}

	return fromHeader[start : start+end]
}

// extractIPPortFromContact 从Contact头中提取IP和端口
func extractIPPortFromContact(contactHeader string) (string, int) {
	// Contact: <sip:34020000001320000001@192.168.1.100:5060>;expires=3600
	start := strings.Index(contactHeader, "@")
	if start == -1 {
		return "", 0
	}
	start += 1

	end := strings.Index(contactHeader[start:], ">")
	if end == -1 {
		return "", 0
	}

	sipAddr := contactHeader[start : start+end]
	ipPort := strings.SplitN(sipAddr, ":", 2)
	if len(ipPort) < 2 {
		return ipPort[0], 5060 // 默认端口
	}

	port, err := strconv.Atoi(ipPort[1])
	if err != nil {
		return ipPort[0], 5060
	}

	return ipPort[0], port
}

// ==================== UDP 版本的消息处理函数 ====================

// handleRegisterUDP 处理 UDP 注册请求
func (s *Server) handleRegisterUDP(remoteAddr *net.UDPAddr, message *SIPMessage) {
	// 解析From头获取设备信息
	fromHeader := message.Headers["From"]
	if fromHeader == "" {
		log.Println("[WARN] UDP REGISTER消息缺少From头")
		response := BuildSIPResponse(message, 400, "Bad Request")
		s.udpConn.WriteToUDP(response, remoteAddr)
		return
	}

	// 从From头中提取设备ID
	deviceID := extractDeviceID(fromHeader)
	if deviceID == "" {
		log.Println("[WARN] UDP 无法从From头提取设备ID")
		response := BuildSIPResponse(message, 400, "Bad Request")
		s.udpConn.WriteToUDP(response, remoteAddr)
		return
	}

	log.Printf("[UDP-REGISTER] 设备ID: %s 来自: %s", deviceID, remoteAddr)

	// 认证检查
	if !s.authenticateMessage(message) {
		// 发送401未授权响应
		log.Printf("[UDP-REGISTER] 设备 %s 认证失败，发送401挑战", deviceID)
		realm := s.config.Realm
		response := []byte(fmt.Sprintf("SIP/2.0 401 Unauthorized\r\n"+
			"Via: %s\r\n"+
			"From: %s\r\n"+
			"To: %s\r\n"+
			"Call-ID: %s\r\n"+
			"CSeq: %s\r\n"+
			"WWW-Authenticate: Digest realm=\"%s\", nonce=\"%s\", algorithm=MD5\r\n"+
			"Content-Length: 0\r\n\r\n",
			message.Headers["Via"],
			message.Headers["From"],
			message.Headers["To"],
			message.Headers["Call-ID"],
			message.Headers["CSeq"],
			realm,
			generateNonce(),
		))
		s.udpConn.WriteToUDP(response, remoteAddr)
		return
	}

	// 解析Contact头获取设备IP和端口
	contactHeader := message.Headers["Contact"]
	ip := remoteAddr.IP.String()
	port := remoteAddr.Port

	if contactHeader != "" {
		extractedIP, extractedPort := extractIPPortFromContact(contactHeader)
		if extractedIP != "" {
			ip = extractedIP
			port = extractedPort
		}
	}

	log.Printf("[UDP-REGISTER] 设备地址: %s:%d", ip, port)

	// 解析Expires头
	expires := 3600 // 默认3600秒
	expiresHeader := message.Headers["Expires"]
	if expiresHeader != "" {
		if e, err := strconv.Atoi(expiresHeader); err == nil {
			expires = e
		}
	}

	// 注册设备
	s.RegisterDevice(deviceID, "", ip, port, expires)

	// 发送200 OK响应
	log.Printf("[UDP-REGISTER] ✓ 设备 %s 注册成功，有效期: %d秒", deviceID, expires)
	response := BuildSIPResponse(message, 200, "OK")
	s.udpConn.WriteToUDP(response, remoteAddr)
}

// handleMessageUDP 处理 UDP MESSAGE 请求
func (s *Server) handleMessageUDP(remoteAddr *net.UDPAddr, message *SIPMessage) {
	log.Printf("[UDP-MESSAGE] 收到消息请求来自: %s", remoteAddr)

	// 从 From 头提取设备ID
	fromHeader := message.Headers["From"]
	deviceID := extractDeviceID(fromHeader)
	log.Printf("[UDP-MESSAGE] 设备ID: %s", deviceID)

	// 发送200 OK响应
	response := BuildSIPResponse(message, 200, "OK")
	s.udpConn.WriteToUDP(response, remoteAddr)

	// 如果设备ID有效，检查设备是否已注册
	if deviceID != "" {
		s.devicesMux.RLock()
		_, exists := s.devices[deviceID]
		s.devicesMux.RUnlock()

		if !exists {
			// 设备未注册，自动注册（从 MESSAGE 消息中提取信息）
			device := &Device{
				DeviceID:      deviceID,
				SipIP:         remoteAddr.IP.String(),
				SipPort:       remoteAddr.Port,
				Transport:     "UDP",
				Status:        "online",
				RegisterTime:  time.Now().Unix(),
				LastKeepAlive: time.Now().Unix(),
				Expires:       3600, // 默认1小时有效期
				Channels:      make([]*Channel, 0),
			}
			s.devicesMux.Lock()
			s.devices[deviceID] = device
			s.devicesMux.Unlock()
			log.Printf("[UDP-MESSAGE] ✓ 设备 %s 自动注册成功 (通过MESSAGE)", deviceID)

			// 自动查询设备信息和目录
			go func() {
				time.Sleep(500 * time.Millisecond)
				s.QueryDeviceInfo(deviceID)
				time.Sleep(500 * time.Millisecond)
				s.QueryCatalog(deviceID)
			}()
		} else {
			// 设备已注册，更新心跳时间
			s.UpdateKeepAlive(deviceID)
		}
	} // 解析消息体（可能是设备目录、状态等XML数据）
	if len(message.Body) > 0 {
		log.Printf("[UDP-MESSAGE] 消息体内容:\n%s", message.Body)
		if strings.Contains(message.Body, "Catalog") && strings.Contains(message.Body, "Response") {
			log.Printf("[UDP-MESSAGE] 收到设备目录响应")
			s.parseCatalogResponse(deviceID, message.Body)
		} else if strings.Contains(message.Body, "DeviceInfo") && strings.Contains(message.Body, "Response") {
			log.Printf("[UDP-MESSAGE] 收到设备信息响应")
			s.parseDeviceInfoResponse(deviceID, message.Body)
		} else if strings.Contains(message.Body, "Keepalive") {
			if deviceID != "" {
				s.UpdateKeepAlive(deviceID)
				log.Printf("[UDP-MESSAGE] 收到设备 %s 心跳", deviceID)
			}
		}
	}
}

// parseCatalogResponse 解析目录响应
func (s *Server) parseCatalogResponse(deviceID string, body string) {
	// 替换 GB2312 编码声明为 UTF-8，因为 Go 标准库不支持 GB2312
	body = strings.Replace(body, `encoding="GB2312"`, `encoding="UTF-8"`, 1)
	body = strings.Replace(body, `encoding='GB2312'`, `encoding='UTF-8'`, 1)

	var catalog CatalogResponse
	if err := xml.Unmarshal([]byte(body), &catalog); err != nil {
		log.Printf("[GB28181] 解析目录响应失败: %v", err)
		return
	}

	log.Printf("[GB28181] 目录响应: 设备=%s, 总数=%d, 本次=%d", catalog.DeviceID, catalog.SumNum, catalog.DeviceList.Num)

	// 使用响应中的设备ID（如果有）
	if catalog.DeviceID != "" {
		deviceID = catalog.DeviceID
	}

	// 解析通道信息
	for _, item := range catalog.DeviceList.Devices {
		// 跳过非通道设备（如NVR本身）
		// GB28181 通道ID 一般以 132 或 134 开头（摄像头或报警设备）
		channelID := item.DeviceID

		// 判断是否是通道（通道ID后缀通常不同于设备ID）
		if channelID == deviceID {
			// 这是设备本身，不是通道，更新设备信息
			s.UpdateDeviceInfo(deviceID, item.Manufacturer, item.Model, "")
			continue
		}

		channel := &Channel{
			ChannelID:    channelID,
			DeviceID:     deviceID,
			Name:         item.Name,
			Manufacturer: item.Manufacturer,
			Model:        item.Model,
			Status:       item.Status,
			PTZType:      item.PTZType,
			Longitude:    item.Longitude,
			Latitude:     item.Latitude,
		}

		// 添加到设备
		s.AddChannel(deviceID, channel)
		log.Printf("[GB28181] ✓ 解析通道: ID=%s, 名称=%s, 状态=%s, PTZ=%d",
			channelID, item.Name, item.Status, item.PTZType)
	}
}

// parseDeviceInfoResponse 解析设备信息响应
func (s *Server) parseDeviceInfoResponse(deviceID string, body string) {
	// 替换 GB2312 编码声明为 UTF-8，因为 Go 标准库不支持 GB2312
	body = strings.Replace(body, `encoding="GB2312"`, `encoding="UTF-8"`, 1)
	body = strings.Replace(body, `encoding='GB2312'`, `encoding='UTF-8'`, 1)

	var info DeviceInfoResponse
	if err := xml.Unmarshal([]byte(body), &info); err != nil {
		log.Printf("[GB28181] 解析设备信息响应失败: %v", err)
		return
	}

	// 使用响应中的设备ID（如果有）
	if info.DeviceID != "" {
		deviceID = info.DeviceID
	}

	s.UpdateDeviceInfo(deviceID, info.Manufacturer, info.Model, info.Firmware)

	// 更新设备名称
	s.devicesMux.Lock()
	if device, ok := s.devices[deviceID]; ok {
		device.Name = info.DeviceName
	}
	s.devicesMux.Unlock()

	log.Printf("[GB28181] ✓ 设备信息更新: ID=%s, 名称=%s, 厂商=%s, 型号=%s, 通道数=%d",
		deviceID, info.DeviceName, info.Manufacturer, info.Model, info.Channel)
}

// handleInviteUDP 处理 UDP INVITE 请求
func (s *Server) handleInviteUDP(remoteAddr *net.UDPAddr, message *SIPMessage) {
	fromHeader := message.Headers["From"]
	deviceID := extractDeviceID(fromHeader)
	if deviceID == "" {
		log.Println("[WARN] UDP INVITE消息缺少有效设备ID")
		response := BuildSIPResponse(message, 400, "Bad Request")
		s.udpConn.WriteToUDP(response, remoteAddr)
		return
	}

	log.Printf("[UDP-INVITE] 设备 %s 请求媒体流", deviceID)

	// 简化处理，直接返回200 OK
	response := BuildSIPResponse(message, 200, "OK")
	s.udpConn.WriteToUDP(response, remoteAddr)
	log.Printf("[UDP-INVITE] ✓ 已接受设备 %s 的媒体流请求", deviceID)
}

// handleByeUDP 处理 UDP BYE 请求
func (s *Server) handleByeUDP(remoteAddr *net.UDPAddr, message *SIPMessage) {
	response := BuildSIPResponse(message, 200, "OK")
	s.udpConn.WriteToUDP(response, remoteAddr)
	log.Printf("[UDP-BYE] 会话已结束: %s", remoteAddr)
}

// handleOptionsUDP 处理 UDP OPTIONS 请求（心跳）
func (s *Server) handleOptionsUDP(remoteAddr *net.UDPAddr, message *SIPMessage) {
	// 发送200 OK响应
	response := BuildSIPResponse(message, 200, "OK")
	s.udpConn.WriteToUDP(response, remoteAddr)
	log.Printf("[UDP-OPTIONS] 收到心跳检测请求: %s", remoteAddr)
}

// handleSIPResponseUDP 处理 UDP SIP 响应消息
func (s *Server) handleSIPResponseUDP(remoteAddr *net.UDPAddr, message *SIPMessage) {
	log.Printf("[SIP-RESPONSE] 收到SIP响应来自: %s, 类型: %s", remoteAddr, message.Type)

	// 从 From 头提取设备ID
	fromHeader := message.Headers["From"]
	deviceID := extractDeviceID(fromHeader)

	// 解析消息体（可能是设备目录、状态等XML数据）
	if len(message.Body) > 0 {
		log.Printf("[SIP-RESPONSE] 消息体长度: %d", len(message.Body))
		if strings.Contains(message.Body, "Catalog") && strings.Contains(message.Body, "Response") {
			log.Printf("[SIP-RESPONSE] 收到设备目录响应")
			s.parseCatalogResponse(deviceID, message.Body)
		} else if strings.Contains(message.Body, "DeviceInfo") && strings.Contains(message.Body, "Response") {
			log.Printf("[SIP-RESPONSE] 收到设备信息响应")
			s.parseDeviceInfoResponse(deviceID, message.Body)
		}
	}
}
