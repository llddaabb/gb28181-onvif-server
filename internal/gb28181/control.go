package gb28181

import (
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"
)

// PTZCommand PTZ控制命令结构体
type PTZCommand struct {
	XMLName  xml.Name `xml:"Control"`
	CmdType  string   `xml:"CmdType"`
	SN       string   `xml:"SN"`
	DeviceID string   `xml:"DeviceID"`
	PTZCmd   string   `xml:"PTZCmd"`
}

// PTZResponse PTZ控制响应结构体
type PTZResponse struct {
	XMLName    xml.Name `xml:"Response"`
	CmdType    string   `xml:"CmdType"`
	SN         string   `xml:"SN"`
	DeviceID   string   `xml:"DeviceID"`
	Result     string   `xml:"Result"`
	ResultDesp string   `xml:"ResultDesp,omitempty"`
}

// generatePTZCmdBytes 生成 GB28181 PTZ 控制字节码
// GB28181 PTZ 命令格式: A50F01[指令字][水平速度][垂直速度][缩放速度][校验码]
// 指令字:
//   - 上: 0x08, 下: 0x04, 左: 0x02, 右: 0x01
//   - 左上: 0x0A, 左下: 0x06, 右上: 0x09, 右下: 0x05
//   - 放大: 0x10, 缩小: 0x20
//   - 停止: 0x00
func generatePTZCmdBytes(command string, speed int) string {
	// 标准前缀 A50F01
	prefix := []byte{0xA5, 0x0F, 0x01}

	// 根据命令确定指令字和速度分配
	var cmdByte byte = 0x00
	var panSpeed byte = 0x00  // 水平速度
	var tiltSpeed byte = 0x00 // 垂直速度
	var zoomSpeed byte = 0x00 // 缩放速度

	// 将速度转换为 0-255 范围
	if speed <= 0 {
		speed = 128
	}
	if speed > 255 {
		speed = 255
	}
	speedByte := byte(speed)

	switch strings.ToLower(command) {
	case "up":
		cmdByte = 0x08
		tiltSpeed = speedByte
	case "down":
		cmdByte = 0x04
		tiltSpeed = speedByte
	case "left":
		cmdByte = 0x02
		panSpeed = speedByte
	case "right":
		cmdByte = 0x01
		panSpeed = speedByte
	case "leftup", "upleft":
		cmdByte = 0x0A
		panSpeed = speedByte
		tiltSpeed = speedByte
	case "leftdown", "downleft":
		cmdByte = 0x06
		panSpeed = speedByte
		tiltSpeed = speedByte
	case "rightup", "upright":
		cmdByte = 0x09
		panSpeed = speedByte
		tiltSpeed = speedByte
	case "rightdown", "downright":
		cmdByte = 0x05
		panSpeed = speedByte
		tiltSpeed = speedByte
	case "zoomin", "zoom_in":
		cmdByte = 0x10
		zoomSpeed = speedByte & 0x0F // 缩放速度只取低4位
	case "zoomout", "zoom_out":
		cmdByte = 0x20
		zoomSpeed = speedByte & 0x0F // 缩放速度只取低4位
	case "stop":
		cmdByte = 0x00
		panSpeed = 0
		tiltSpeed = 0
		zoomSpeed = 0
	default:
		// 默认停止
		cmdByte = 0x00
	}

	// 组装命令字节
	cmdBytes := append(prefix, cmdByte, panSpeed, tiltSpeed, zoomSpeed)

	// 计算校验码 (所有字节异或)
	checksum := byte(0)
	for _, b := range cmdBytes {
		checksum ^= b
	}
	cmdBytes = append(cmdBytes, checksum)

	// 转换为大写十六进制字符串
	return strings.ToUpper(hex.EncodeToString(cmdBytes))
}

// SendPTZCommand 发送PTZ控制命令
func (s *Server) SendPTZCommand(deviceID, channel, ptzCmd string, speed int) error {
	device, exists := s.GetDeviceByID(deviceID)
	if !exists {
		log.Printf("[PTZ] [ERROR] 设备不存在: %s", deviceID)
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	// 生成 GB28181 PTZ 字节码
	ptzCmdHex := generatePTZCmdBytes(ptzCmd, speed)
	log.Printf("[PTZ] 发送命令到设备 %s: 命令=%s, 速度=%d, 字节码=%s", deviceID, ptzCmd, speed, ptzCmdHex)

	// 发送的目标设备ID应该是通道ID
	targetDeviceID := channel
	if targetDeviceID == "" {
		targetDeviceID = deviceID
	}

	// 生成PTZ控制XML内容
	ptzCmdXML := &PTZCommand{
		CmdType:  "DeviceControl",
		SN:       strconv.FormatInt(time.Now().UnixNano()/1000000, 10),
		DeviceID: targetDeviceID,
		PTZCmd:   ptzCmdHex,
	}

	xmlBytes, err := xml.MarshalIndent(ptzCmdXML, "", "  ")
	if err != nil {
		return fmt.Errorf("生成PTZ命令XML失败: %v", err)
	}
	xmlContent := `<?xml version="1.0" encoding="GB2312"?>` + "\r\n" + string(xmlBytes)

	// 优先尝试使用 UDP 发送
	if device.Transport == "UDP" || s.udpConn != nil {
		if err := s.sendPTZCommandUDP(device, targetDeviceID, xmlContent); err == nil {
			return nil
		} else {
			log.Printf("[PTZ] [WARN] UDP 发送失败，回退到 TCP: %v", err)
		}
	}

	// 生成SIP消息（TCP）
	msg := &SIPMessage{
		Type: "MESSAGE",
		Headers: map[string]string{
			"To":           fmt.Sprintf("<sip:%s@%s:%d>", deviceID, device.SipIP, device.SipPort),
			"From":         fmt.Sprintf("<sip:%s@%s:%d>;tag=%s", s.config.ServerID, s.config.SipIP, s.config.SipPort, generateTag()),
			"Call-ID":      generateCallID(),
			"CSeq":         "1 MESSAGE",
			"Content-Type": "Application/MANSCDP+xml",
		},
		Body: xmlContent,
	}

	// 发送消息（TCP）
	return s.sendSIPMessage(device, msg)
}

// sendPTZCommandUDP 通过UDP发送PTZ控制命令
func (s *Server) sendPTZCommandUDP(device *Device, targetDeviceID, xmlContent string) error {
	if s.udpConn == nil {
		return fmt.Errorf("UDP连接未初始化")
	}

	// 解析设备地址
	remoteAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", device.SipIP, device.SipPort))
	if err != nil {
		return fmt.Errorf("解析设备地址失败: %v", err)
	}

	// 构建 SIP MESSAGE 请求
	callID := generateCallID()
	fromTag := generateTag()
	branch := fmt.Sprintf("z9hG4bK%d", time.Now().UnixNano())
	cseq := "1"

	message := fmt.Sprintf("MESSAGE sip:%s@%s:%d SIP/2.0\r\n"+
		"Via: SIP/2.0/UDP %s:%d;rport;branch=%s\r\n"+
		"From: <sip:%s@%s:%d>;tag=%s\r\n"+
		"To: <sip:%s@%s:%d>\r\n"+
		"Call-ID: %s\r\n"+
		"CSeq: %s MESSAGE\r\n"+
		"Content-Type: Application/MANSCDP+xml\r\n"+
		"Max-Forwards: 70\r\n"+
		"Content-Length: %d\r\n"+
		"\r\n%s",
		device.DeviceID, device.SipIP, device.SipPort,
		s.config.SipIP, s.config.SipPort, branch,
		s.config.ServerID, s.config.SipIP, s.config.SipPort, fromTag,
		device.DeviceID, device.SipIP, device.SipPort,
		callID,
		cseq,
		len(xmlContent),
		xmlContent,
	)

	_, err = s.udpConn.WriteToUDP([]byte(message), remoteAddr)
	if err != nil {
		log.Printf("[PTZ] [ERROR] UDP发送PTZ命令失败: %v", err)
		return err
	}

	log.Printf("[PTZ] ✓ 已发送PTZ命令到设备 %s (%s)", device.DeviceID, remoteAddr.String())
	return nil
}

// ResetPTZ 复位PTZ
func (s *Server) ResetPTZ(deviceID, channel string) error {
	return s.SendPTZCommand(deviceID, channel, "Reset", 1)
}

// CatalogQuery 目录查询请求结构体
type CatalogQuery struct {
	XMLName  xml.Name `xml:"Query"`
	CmdType  string   `xml:"CmdType"`
	SN       string   `xml:"SN"`
	DeviceID string   `xml:"DeviceID"`
}

// SendCatalogQuery 向设备发送目录查询
func (s *Server) SendCatalogQuery(deviceID string) error {
	device, exists := s.GetDeviceByID(deviceID)
	if !exists {
		log.Printf("[Catalog] [ERROR] 设备不存在: %s", deviceID)
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	log.Printf("[Catalog] 发送目录查询到设备 %s", deviceID)

	// 生成目录查询 XML
	query := &CatalogQuery{
		CmdType:  "Catalog",
		SN:       strconv.FormatInt(time.Now().UnixNano()/1000000, 10),
		DeviceID: deviceID,
	}

	xmlBytes, err := xml.MarshalIndent(query, "", "  ")
	if err != nil {
		return fmt.Errorf("生成Catalog查询XML失败: %v", err)
	}
	xmlContent := `<?xml version="1.0" encoding="GB2312"?>` + "\r\n" + string(xmlBytes)

	// 生成SIP消息
	msg := &SIPMessage{
		Type: "MESSAGE",
		Headers: map[string]string{
			"To":           fmt.Sprintf("<sip:%s@%s:%d>", deviceID, device.SipIP, device.SipPort),
			"From":         fmt.Sprintf("<sip:%s@%s:%d>;tag=%s", s.config.ServerID, s.config.SipIP, s.config.SipPort, generateTag()),
			"Call-ID":      generateCallID(),
			"CSeq":         "1 MESSAGE",
			"Content-Type": "Application/MANSCDP+xml",
		},
		Body: xmlContent,
	}

	// 尝试使用 UDP 发送（优先）
	if device.Transport == "UDP" || s.udpConn != nil {
		return s.sendCatalogQueryUDP(device, xmlContent)
	}

	// 否则使用 TCP 发送
	return s.sendSIPMessage(device, msg)
}

// sendCatalogQueryUDP 通过UDP发送目录查询
func (s *Server) sendCatalogQueryUDP(device *Device, xmlContent string) error {
	if s.udpConn == nil {
		return fmt.Errorf("UDP连接未初始化")
	}

	// 解析设备地址
	remoteAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", device.SipIP, device.SipPort))
	if err != nil {
		return fmt.Errorf("解析设备地址失败: %v", err)
	}

	// 构建 SIP MESSAGE 请求
	callID := generateCallID()
	fromTag := generateTag()
	branch := fmt.Sprintf("z9hG4bK%d", time.Now().UnixNano())
	cseq := "1"

	message := fmt.Sprintf("MESSAGE sip:%s@%s:%d SIP/2.0\r\n"+
		"Via: SIP/2.0/UDP %s:%d;rport;branch=%s\r\n"+
		"From: <sip:%s@%s:%d>;tag=%s\r\n"+
		"To: <sip:%s@%s:%d>\r\n"+
		"Call-ID: %s\r\n"+
		"CSeq: %s MESSAGE\r\n"+
		"Content-Type: Application/MANSCDP+xml\r\n"+
		"Max-Forwards: 70\r\n"+
		"Content-Length: %d\r\n"+
		"\r\n%s",
		device.DeviceID, device.SipIP, device.SipPort,
		s.config.SipIP, s.config.SipPort, branch,
		s.config.ServerID, s.config.SipIP, s.config.SipPort, fromTag,
		device.DeviceID, device.SipIP, device.SipPort,
		callID,
		cseq,
		len(xmlContent),
		xmlContent,
	)

	_, err = s.udpConn.WriteToUDP([]byte(message), remoteAddr)
	if err != nil {
		log.Printf("[Catalog] [ERROR] 发送目录查询失败: %v", err)
		return err
	}

	log.Printf("[Catalog] ✓ 已发送目录查询到设备 %s (%s)", device.DeviceID, remoteAddr.String())
	return nil
}

// sendSIPMessage 发送SIP消息到设备
func (s *Server) sendSIPMessage(device *Device, message *SIPMessage) error {
	// 建立TCP连接
	addr := net.JoinHostPort(device.SipIP, strconv.Itoa(device.SipPort))
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		log.Printf("[PTZ] [ERROR] 连接设备失败 %s: %v", addr, err)
		return fmt.Errorf("连接设备失败 %s: %v", addr, err)
	}
	defer conn.Close()

	// 构建完整的SIP消息
	msgStr := buildSIPMessageString(message)

	// 发送消息
	_, err = conn.Write([]byte(msgStr))
	if err != nil {
		log.Printf("[PTZ] [ERROR] 发送SIP消息失败: %v", err)
		return fmt.Errorf("发送SIP消息失败: %v", err)
	}

	log.Printf("[PTZ] ✓ 已发送SIP消息到设备 %s", device.DeviceID)
	return nil
}

// buildSIPMessageString 构建完整的SIP消息字符串
func buildSIPMessageString(msg *SIPMessage) string {
	// 构建请求行
	requestLine := msg.Type + " " + msg.Headers["To"] + " SIP/2.0\r\n"

	// 构建头部
	headers := ""
	for key, value := range msg.Headers {
		headers += key + ": " + value + "\r\n"
	}

	// 构建Content-Length
	contentLength := 0
	if msg.Body != "" {
		contentLength = len(msg.Body)
	}
	headers += "Content-Length: " + strconv.Itoa(contentLength) + "\r\n"

	// 组合完整消息
	message := requestLine + headers + "\r\n"
	if msg.Body != "" {
		message += msg.Body
	}

	return message
}

// generateTag 生成SIP消息的Tag
func generateTag() string {
	return fmt.Sprintf("tag_%d", time.Now().UnixNano())
}

// generateCallID 生成SIP消息的Call-ID
func generateCallID() string {
	return fmt.Sprintf("callid_%d", time.Now().UnixNano())
}
