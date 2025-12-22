package gb28181

import (
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"log"
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

	// 使用统一方法构建 SIP MESSAGE
	sipMessage := s.BuildSIPMessageString(device, targetDeviceID, "Application/MANSCDP+xml", xmlContent)

	// 使用统一方法发送（根据设备 Transport 自动选择 TCP/UDP）
	err = s.SendSIPMessageToDevice(device, sipMessage)
	if err != nil {
		log.Printf("[PTZ] [ERROR] 发送PTZ命令失败: %v", err)
		return err
	}

	log.Printf("[PTZ] ✓ 已发送PTZ命令到设备 %s [%s]", device.DeviceID, device.Transport)
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

	log.Printf("[Catalog] 发送目录查询到设备 %s [%s]", deviceID, device.Transport)

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

	// 使用统一方法构建 SIP MESSAGE
	sipMessage := s.BuildSIPMessageString(device, deviceID, "Application/MANSCDP+xml", xmlContent)

	// 使用统一方法发送（根据设备 Transport 自动选择 TCP/UDP）
	err = s.SendSIPMessageToDevice(device, sipMessage)
	if err != nil {
		log.Printf("[Catalog] [ERROR] 发送目录查询失败: %v", err)
		return err
	}

	log.Printf("[Catalog] ✓ 已发送目录查询到设备 %s [%s]", deviceID, device.Transport)
	return nil
}

// generateTag 生成SIP消息的Tag
func generateTag() string {
	return fmt.Sprintf("tag_%d", time.Now().UnixNano())
}

// generateCallID 生成SIP消息的Call-ID
func generateCallID() string {
	return fmt.Sprintf("callid_%d", time.Now().UnixNano())
}
