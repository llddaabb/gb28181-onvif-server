package gb28181

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"gb28181-onvif-server/internal/debug"
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

// CatalogDeviceInfo 设备信息嵌套结构（用于解析Info标签内的字段）
type CatalogDeviceInfo struct {
	PTZType       int    `xml:"PTZType"`
	DownloadSpeed string `xml:"DownloadSpeed"`
}

type CatalogDevice struct {
	DeviceID     string            `xml:"DeviceID"`
	Name         string            `xml:"Name"`
	Manufacturer string            `xml:"Manufacturer"`
	Model        string            `xml:"Model"`
	Owner        string            `xml:"Owner"`
	CivilCode    string            `xml:"CivilCode"`
	Address      string            `xml:"Address"`
	Parental     int               `xml:"Parental"`
	ParentID     string            `xml:"ParentID"`
	SafetyWay    int               `xml:"SafetyWay"`
	RegisterWay  int               `xml:"RegisterWay"`
	Secrecy      int               `xml:"Secrecy"`
	Status       string            `xml:"Status"`
	Longitude    string            `xml:"Longitude"`
	Latitude     string            `xml:"Latitude"`
	PTZType      int               `xml:"PTZType"` // 直接在Item下的PTZType（部分设备）
	Info         CatalogDeviceInfo `xml:"Info"`    // 嵌套在Info标签内的PTZType（大华等设备）
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
	Type       string            // 请求类型: REGISTER, INVITE, ACK, BYE, MESSAGE 等 (请求) / "" (响应)
	StatusCode int               // 状态码 (仅响应): 100-699
	Reason     string            // 原因短语 (仅响应)
	Headers    map[string]string // SIP头字段
	Body       string            // 消息体
	IsResponse bool              // 是否为响应
}

// ParseSIPMessage 解析SIP消息
func ParseSIPMessage(data []byte) (*SIPMessage, error) {
	reader := bufio.NewReader(strings.NewReader(string(data)))

	// 解析请求行或状态行
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
		Headers:    make(map[string]string),
		IsResponse: strings.HasPrefix(requestLine, "SIP/"),
	}

	// 判断是请求还是响应
	if message.IsResponse {
		// 解析响应：SIP/2.0 200 OK
		// parts[0] = "SIP/2.0"
		// parts[1] = 状态码
		// parts[2] = 原因短语
		statusCode := 0
		fmt.Sscanf(parts[1], "%d", &statusCode)
		message.StatusCode = statusCode
		message.Reason = parts[2]
		message.Type = "" // 响应没有Type
	} else {
		// 请求: INVITE sip:xxx SIP/2.0
		message.Type = parts[0]
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
		debug.Warn("gb28181", "解析SIP消息失败: %v", err)
		return
	}

	// 如果是响应，进行响应处理
	if message.IsResponse {
		debug.Debug("gb28181", "收到状态响应: %d %s 来自: %s", message.StatusCode, message.Reason, conn.RemoteAddr())
		s.handleSIPResponse(conn, message)
		return
	}

	// 根据请求类型进行处理
	debug.Debug("gb28181", "TCP SIP消息: 类型=%s 来自: %s", message.Type, conn.RemoteAddr())
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
		debug.Warn("gb28181", "未知的SIP消息类型: %s", message.Type)
	}
}

// handleSIPResponse 处理SIP响应（设备对我们请求的回复）
func (s *Server) handleSIPResponse(conn net.Conn, response *SIPMessage) {
	callID := response.Headers["Call-ID"]
	cseq := response.Headers["CSeq"]

	debug.Debug("gb28181", "SIP-Response Call-ID: %s, CSeq: %s, Status: %d", callID, cseq, response.StatusCode)

	// 对于 INVITE 的 2xx 响应，需要发送 ACK
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		// 这是对 INVITE 的成功响应，需要发送 ACK
		if strings.Contains(cseq, "INVITE") {
			debug.Debug("gb28181", "对INVITE响应 %d，发送ACK", response.StatusCode)
			s.sendACK(conn, response)
		}
	} else if response.StatusCode >= 300 && response.StatusCode < 400 {
		// 3xx 重定向，暂不处理
		debug.Warn("gb28181", "收到重定向响应 %d: %s", response.StatusCode, response.Reason)
	} else if response.StatusCode >= 400 {
		// 4xx 或更高的错误
		debug.Warn("gb28181", "收到错误响应 %d: %s", response.StatusCode, response.Reason)
	} else if response.StatusCode >= 100 && response.StatusCode < 200 {
		// 1xx 临时响应（如 180 Ringing, 183 Session Progress）
		debug.Debug("gb28181", "SIP临时响应 %d: %s", response.StatusCode, response.Reason)
		// 可以选择发送 PRACK（Provisional Acknowledgement）但GB28181不要求
	}
}

// sendACK 发送ACK响应
func (s *Server) sendACK(conn net.Conn, response *SIPMessage) {
	callID := response.Headers["Call-ID"]
	from := response.Headers["From"]
	to := response.Headers["To"]
	cseq := response.Headers["CSeq"]
	via := response.Headers["Via"]

	// 解析CSeq获取序列号
	cseqParts := strings.Fields(cseq)
	if len(cseqParts) < 2 {
		debug.Warn("gb28181", "无效的CSeq头: %s", cseq)
		return
	}

	// 构建ACK请求
	// 提取To头中的请求URI（设备地址）
	toHeader := to
	requestURI := extractSIPURI(toHeader)
	if requestURI == "" {
		// 降级使用From头中的URI
		requestURI = extractSIPURI(from)
	}

	if requestURI == "" {
		debug.Warn("gb28181", "无法从To/From头提取URI")
		return
	}

	// 构建ACK消息
	ackMsg := fmt.Sprintf("ACK %s SIP/2.0\r\n", requestURI)
	ackMsg += fmt.Sprintf("Via: %s\r\n", via)
	ackMsg += fmt.Sprintf("From: %s\r\n", from)
	ackMsg += fmt.Sprintf("To: %s\r\n", to)
	ackMsg += fmt.Sprintf("Call-ID: %s\r\n", callID)
	ackMsg += fmt.Sprintf("CSeq: %s ACK\r\n", cseqParts[0])
	ackMsg += "Content-Length: 0\r\n\r\n"

	debug.Debug("gb28181", "发送ACK: %s", requestURI)
	conn.Write([]byte(ackMsg))
}

// extractSIPURI 从SIP头中提取URI
func extractSIPURI(header string) string {
	// 从 <sip:xxx@xxx:5060> 格式中提取 sip:xxx@xxx:5060
	start := strings.Index(header, "<")
	end := strings.Index(header, ">")
	if start >= 0 && end > start {
		return header[start+1 : end]
	}
	// 如果没有尖括号，尝试直接作为URI
	if strings.HasPrefix(header, "sip:") {
		// 从sip:xxx@xxx:5060;tag=xxx中提取URI部分
		parts := strings.Split(header, ";")
		return parts[0]
	}
	return ""
}

// sendACKUDP 通过UDP发送ACK响应 (用于处理来自设备的响应)
func (s *Server) sendACKUDP(remoteAddr *net.UDPAddr, response *SIPMessage) {
	callID := response.Headers["Call-ID"]
	from := response.Headers["From"]
	to := response.Headers["To"]
	cseq := response.Headers["CSeq"]
	via := response.Headers["Via"]

	// 解析CSeq获取序列号
	cseqParts := strings.Fields(cseq)
	if len(cseqParts) < 2 {
		debug.Warn("gb28181", "UDP ACK无效的CSeq头: %s", cseq)
		return
	}

	// 仅对 INVITE 响应发送 ACK
	if !strings.Contains(cseq, "INVITE") {
		return
	}

	// 对于2xx响应，构建ACK请求
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return
	}

	// 提取请求URI - 使用设备的地址
	requestURI := extractSIPURI(from)
	if requestURI == "" {
		return
	}

	// 构建ACK消息
	ackMsg := fmt.Sprintf("ACK %s SIP/2.0\r\n", requestURI)
	ackMsg += fmt.Sprintf("Via: %s\r\n", via)
	ackMsg += fmt.Sprintf("From: %s\r\n", from)
	ackMsg += fmt.Sprintf("To: %s\r\n", to)
	ackMsg += fmt.Sprintf("Call-ID: %s\r\n", callID)
	ackMsg += fmt.Sprintf("CSeq: %s ACK\r\n", cseqParts[0])
	ackMsg += "Content-Length: 0\r\n\r\n"

	debug.Debug("gb28181", "UDP发送ACK: %s", requestURI)
	s.udpConn.WriteToUDP([]byte(ackMsg), remoteAddr)
}

// handleRegister 处理注册请求
func (s *Server) handleRegister(conn net.Conn, message *SIPMessage) {
	// 解析From头获取设备信息
	fromHeader := message.Headers["From"]
	if fromHeader == "" {
		debug.Warn("gb28181", "REGISTER消息缺少From头")
		response := BuildSIPResponse(message, 400, "Bad Request")
		conn.Write(response)
		return
	}

	// 从From头中提取设备ID
	// From: <sip:34020000001320000001@3402000000>;tag=123456
	deviceID := extractDeviceID(fromHeader)
	if deviceID == "" {
		debug.Warn("gb28181", "无法从From头提取设备ID")
		response := BuildSIPResponse(message, 400, "Bad Request")
		conn.Write(response)
		return
	}

	// 认证检查
	if !s.authenticateMessage(message) {
		// 发送401未授权响应
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
		debug.Warn("gb28181", "REGISTER消息缺少Contact头")
		response := BuildSIPResponse(message, 400, "Bad Request")
		conn.Write(response)
		return
	}

	ip, port := extractIPPortFromContact(contactHeader)
	if ip == "" {
		debug.Warn("gb28181", "无法从Contact头提取IP和端口")
		response := BuildSIPResponse(message, 400, "Bad Request")
		conn.Write(response)
		return
	}

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
	response := BuildSIPResponse(message, 200, "OK")
	conn.Write(response)
}

// authenticateMessage 认证SIP消息
func (s *Server) authenticateMessage(message *SIPMessage) bool {
	// 如果配置中没有设置密码，则跳过认证
	if s.config.Password == "" {
		return true
	}

	// 获取Authorization头
	authHeader := message.Headers["Authorization"]
	if authHeader == "" {
		return false
	}

	// 解析Authorization头
	// Authorization: Digest username="34020000001320000001", realm="3402000000", nonce="123456", uri="sip:...", response="..."
	params := parseAuthParams(authHeader)

	// GB28181 使用设备ID作为用户名
	username, ok := params["username"]
	if !ok {
		return false
	}

	realm, ok := params["realm"]
	if !ok {
		return false
	}

	nonce, ok := params["nonce"]
	if !ok {
		return false
	}

	// 验证 nonce 是否有效（是我们之前发送的）
	if !isValidNonce(nonce) {
		debug.Warn("gb28181", "nonce 无效或已过期: %s", nonce)
		return false
	}

	uri, ok := params["uri"]
	if !ok {
		return false
	}

	response, ok := params["response"]
	if !ok {
		return false
	}

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
		return true
	}

	debug.Warn("gb28181", "认证失败: 用户=%s, 响应值不匹配", username)
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
		debug.Warn("gb28181", "INVITE消息缺少有效设备ID")
		response := BuildSIPResponse(message, 400, "Bad Request")
		conn.Write(response)
		return
	}

	debug.Debug("gb28181", "INVITE: 设备 %s 请求媒体流", deviceID)

	// 这里需要处理媒体流协商（SDP）
	// 简化处理，直接返回200 OK
	response := BuildSIPResponse(message, 200, "OK")
	conn.Write(response)
}

// handleAck 处理ACK请求
func (s *Server) handleAck(conn net.Conn, message *SIPMessage) {
	// ACK不需要响应
}

// handleBye 处理BYE请求（结束会话）
func (s *Server) handleBye(conn net.Conn, message *SIPMessage) {
	// 发送200 OK响应
	response := BuildSIPResponse(message, 200, "OK")
	conn.Write(response)
}

// handleMessage 处理MESSAGE请求（GB28181中的设备信息查询等）
func (s *Server) handleMessage(conn net.Conn, message *SIPMessage) {
	// 简化处理，返回200 OK
	response := BuildSIPResponse(message, 200, "OK")
	conn.Write(response)
}

// handleOptions 处理OPTIONS请求
func (s *Server) handleOptions(conn net.Conn, message *SIPMessage) {
	// 发送200 OK响应
	response := BuildSIPResponse(message, 200, "OK")
	conn.Write(response)
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
		debug.Warn("gb28181", "UDP REGISTER消息缺少From头")
		response := BuildSIPResponse(message, 400, "Bad Request")
		s.udpConn.WriteToUDP(response, remoteAddr)
		return
	}

	// 从From头中提取设备ID
	deviceID := extractDeviceID(fromHeader)
	if deviceID == "" {
		debug.Warn("gb28181", "UDP 无法从From头提取设备ID")
		response := BuildSIPResponse(message, 400, "Bad Request")
		s.udpConn.WriteToUDP(response, remoteAddr)
		return
	}

	// 认证检查
	if !s.authenticateMessage(message) {
		// 发送401未授权响应
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
	response := BuildSIPResponse(message, 200, "OK")
	s.udpConn.WriteToUDP(response, remoteAddr)
}

// handleMessageUDP 处理 UDP MESSAGE 请求
func (s *Server) handleMessageUDP(remoteAddr *net.UDPAddr, message *SIPMessage) {
	// 从 From 头提取设备ID
	fromHeader := message.Headers["From"]
	deviceID := extractDeviceID(fromHeader)

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
			log.Printf("[GB28181] ✓ 设备自动注册: %s", deviceID)

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
	}

	if len(message.Body) > 0 {
		if strings.Contains(message.Body, "Keepalive") {
			// 心跳消息，只更新状态
			if deviceID != "" {
				s.UpdateKeepAlive(deviceID)
			}
		} else if strings.Contains(message.Body, "Catalog") && strings.Contains(message.Body, "Response") {
			s.parseCatalogResponse(deviceID, message.Body)
		} else if strings.Contains(message.Body, "DeviceInfo") && strings.Contains(message.Body, "Response") {
			s.parseDeviceInfoResponse(deviceID, message.Body)
		} else {
			// 其他消息类型
			debug.Debug("gb28181", "收到MESSAGE (设备 %s): %d字节", deviceID, len(message.Body))
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
		debug.Warn("gb28181", "解析目录响应失败: %v", err)
		return
	}

	debug.Info("gb28181", "目录响应: 设备=%s, 总数=%d", catalog.DeviceID, catalog.SumNum)

	// 使用响应中的设备ID（如果有）
	if catalog.DeviceID != "" {
		deviceID = catalog.DeviceID
	}

	// 解析通道信息
	for _, item := range catalog.DeviceList.Devices {
		// 跳过非通道设备（如NVR本身）
		// GB28181 通道ID 一般以 132 或 134 开头（摄像头或报警设备）
		channelID := item.DeviceID

		// 获取PTZType：优先使用Item.Info.PTZType（大华等设备），否则使用Item.PTZType
		ptzType := item.PTZType
		if ptzType == 0 && item.Info.PTZType > 0 {
			ptzType = item.Info.PTZType
		}

		debug.Info("gb28181", "解析通道: ID=%s, Name=%s, PTZType=%d (item=%d, info=%d), Status=%s",
			channelID, item.Name, ptzType, item.PTZType, item.Info.PTZType, item.Status)

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
			PTZType:      ptzType,
			Longitude:    item.Longitude,
			Latitude:     item.Latitude,
		}

		// 添加到设备
		s.AddChannel(deviceID, channel)
	}

	if len(catalog.DeviceList.Devices) > 0 {
		log.Printf("[GB28181] ✓ 解析 %d 个通道", len(catalog.DeviceList.Devices))
	}
}

// parseDeviceInfoResponse 解析设备信息响应
func (s *Server) parseDeviceInfoResponse(deviceID string, body string) {
	// 替换 GB2312 编码声明为 UTF-8，因为 Go 标准库不支持 GB2312
	body = strings.Replace(body, `encoding="GB2312"`, `encoding="UTF-8"`, 1)
	body = strings.Replace(body, `encoding='GB2312'`, `encoding='UTF-8'`, 1)

	var info DeviceInfoResponse
	if err := xml.Unmarshal([]byte(body), &info); err != nil {
		debug.Warn("gb28181", "解析设备信息响应失败: %v", err)
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

	debug.Debug("gb28181", "设备信息更新: ID=%s, 名称=%s, 厂商=%s", deviceID, info.DeviceName, info.Manufacturer)
}

// handleInviteUDP 处理 UDP INVITE 请求
func (s *Server) handleInviteUDP(remoteAddr *net.UDPAddr, message *SIPMessage) {
	fromHeader := message.Headers["From"]
	deviceID := extractDeviceID(fromHeader)
	if deviceID == "" {
		debug.Warn("gb28181", "UDP INVITE消息缺少有效设备ID")
		response := BuildSIPResponse(message, 400, "Bad Request")
		s.udpConn.WriteToUDP(response, remoteAddr)
		return
	}

	// 简化处理，直接返回200 OK
	response := BuildSIPResponse(message, 200, "OK")
	s.udpConn.WriteToUDP(response, remoteAddr)
}

// handleByeUDP 处理 UDP BYE 请求
func (s *Server) handleByeUDP(remoteAddr *net.UDPAddr, message *SIPMessage) {
	response := BuildSIPResponse(message, 200, "OK")
	s.udpConn.WriteToUDP(response, remoteAddr)
}

// handleOptionsUDP 处理 UDP OPTIONS 请求（心跳）
func (s *Server) handleOptionsUDP(remoteAddr *net.UDPAddr, message *SIPMessage) {
	// 发送200 OK响应
	response := BuildSIPResponse(message, 200, "OK")
	s.udpConn.WriteToUDP(response, remoteAddr)
}

// handleSIPResponseUDP 处理 UDP SIP 响应消息
func (s *Server) handleSIPResponseUDP(remoteAddr *net.UDPAddr, message *SIPMessage) {
	// 从 From 头提取设备ID
	fromHeader := message.Headers["From"]
	deviceID := extractDeviceID(fromHeader)

	// 解析消息体（可能是设备目录、状态等XML数据）
	if len(message.Body) > 0 {
		if strings.Contains(message.Body, "Catalog") && strings.Contains(message.Body, "Response") {
			s.parseCatalogResponse(deviceID, message.Body)
		} else if strings.Contains(message.Body, "DeviceInfo") && strings.Contains(message.Body, "Response") {
			s.parseDeviceInfoResponse(deviceID, message.Body)
		}
	}
}
