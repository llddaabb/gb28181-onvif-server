package gb28181

import (
	"encoding/xml"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"
)

// PTZCommand PTZ控制命令结构体
type PTZCommand struct {
	XMLName      xml.Name `xml:"Control"`
	CmdType      string   `xml:"CmdType"`
	SN           string   `xml:"SN"`
	DeviceID     string   `xml:"DeviceID"`
	PTZCmd       string   `xml:"PTZCmd"`
	Speed        int      `xml:"Speed"`
	Channel      string   `xml:"Channel"`
	PresetID     string   `xml:"PresetID,omitempty"`
	TrackID      string   `xml:"TrackID,omitempty"`
	AbsoluteZoom int      `xml:"AbsoluteZoom,omitempty"`
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

// SendPTZCommand 发送PTZ控制命令
func (s *Server) SendPTZCommand(deviceID, channel, ptzCmd string, speed int) error {
	device, exists := s.GetDeviceByID(deviceID)
	if !exists {
		log.Printf("[PTZ] [ERROR] 设备不存在: %s", deviceID)
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	log.Printf("[PTZ] 发送命令到设备 %s: 命令=%s, 速度=%d", deviceID, ptzCmd, speed)

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
	}

	// 生成PTZ控制XML内容
	ptzCmdXML := &PTZCommand{
		CmdType:  "DeviceControl",
		SN:       strconv.FormatInt(time.Now().UnixNano()/1000000, 10),
		DeviceID: deviceID,
		PTZCmd:   ptzCmd,
		Speed:    speed,
		Channel:  channel,
	}

	xmlBytes, err := xml.Marshal(ptzCmdXML)
	if err != nil {
		return fmt.Errorf("生成PTZ命令XML失败: %v", err)
	}

	// 设置消息体
	msg.Body = string(xmlBytes)

	// 优先尝试使用 UDP（如果设备配置为 UDP 或本地 UDP 连接已初始化）
	msgStr := buildSIPMessageString(msg)
	if device.Transport == "UDP" || s.udpConn != nil {
		if err := s.sendSIPMessageUDP(device, msgStr); err == nil {
			return nil
		} else {
			log.Printf("[PTZ] [WARN] UDP 发送失败，回退到 TCP: %v", err)
		}
	}

	// 发送消息（TCP）
	return s.sendSIPMessage(device, msg)
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
