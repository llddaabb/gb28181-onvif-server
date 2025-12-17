// internal/onvif/helper.go
package onvif

import (
	"crypto/rand"
	"encoding/xml"
	"fmt"
	"log"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// normalizeXAddr 规范化 XADDR 地址格式（仅用于用户输入的简单地址）
func normalizeXAddr(xaddr string) string {
	xaddr = strings.TrimSpace(xaddr)

	// 如果已经是完整 URL，直接返回，不做修改
	if strings.HasPrefix(xaddr, "http://") || strings.HasPrefix(xaddr, "https://") {
		return xaddr
	}

	// 如果是 IP:Port 或仅 IP，构建完整 URL
	// 如果包含 :443，用 HTTPS；否则用 HTTP
	if strings.Contains(xaddr, ":443") {
		if !strings.HasPrefix(xaddr, "https://") {
			xaddr = "https://" + xaddr
		}
		if !strings.HasSuffix(xaddr, "/onvif/device_service") {
			xaddr = strings.TrimSuffix(xaddr, "/") + "/onvif/device_service"
		}
		return xaddr
	}

	if !strings.HasPrefix(xaddr, "http://") {
		xaddr = "http://" + xaddr
	}
	if !strings.HasSuffix(xaddr, "/onvif/device_service") {
		xaddr = strings.TrimSuffix(xaddr, "/") + "/onvif/device_service"
	}
	return xaddr
}

// ParseXAddr 解析XADDR地址，返回主机和端口（不修改原始 xaddr）
func ParseXAddr(xaddr string) (host string, port int, err error) {
	// 去掉协议前缀进行解析
	original := xaddr
	xaddr = strings.TrimPrefix(xaddr, "https://")
	xaddr = strings.TrimPrefix(xaddr, "http://")

	// 提取 host:port 部分（去掉路径）
	hostPort := xaddr
	if idx := strings.Index(xaddr, "/"); idx != -1 {
		hostPort = xaddr[:idx]
	}

	// 尝试使用 net.SplitHostPort 解析
	host, portStr, err := net.SplitHostPort(hostPort)
	if err == nil {
		// 成功解析出端口
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return "", 0, fmt.Errorf("无效的端口: %w", err)
		}
		return host, port, nil
	}

	// 如果没有端口，hostPort 本身就是 host
	host = hostPort

	// 根据协议类型设置默认端口
	if strings.HasPrefix(original, "https://") {
		port = 443 // HTTPS 默认端口
	} else {
		port = 80 // HTTP 默认端口
	}

	return host, port, nil
}

// PTZVector 封装 PTZ 移动的向量信息
type PTZVector struct {
	PanTilt *Vector2D
	Zoom    *Vector1D
}

// Vector2D 二维向量 (Pan/Tilt)
type Vector2D struct {
	X float64 // -1.0 to 1.0
	Y float64 // -1.0 to 1.0
}

// Vector1D 一维向量 (Zoom)
type Vector1D struct {
	X float64 // -1.0 to 1.0
}

// ParsePTZDirection 解析方向字符串到 PTZVector
func ParsePTZDirection(direction string, speed float64) *PTZVector {
	vector := &PTZVector{
		PanTilt: &Vector2D{},
		Zoom:    &Vector1D{},
	}

	switch strings.ToLower(direction) {
	case "up":
		vector.PanTilt.Y = speed
	case "down":
		vector.PanTilt.Y = -speed
	case "left":
		vector.PanTilt.X = -speed
	case "right":
		vector.PanTilt.X = speed
	case "up_left":
		vector.PanTilt.X = -speed
		vector.PanTilt.Y = speed
	case "up_right":
		vector.PanTilt.X = speed
		vector.PanTilt.Y = speed
	case "down_left":
		vector.PanTilt.X = -speed
		vector.PanTilt.Y = -speed
	case "down_right":
		vector.PanTilt.X = speed
		vector.PanTilt.Y = -speed
	case "zoom_in":
		vector.Zoom.X = speed
	case "zoom_out":
		vector.Zoom.X = -speed
	}

	return vector
}

// Regex to find service URLs
var serviceURLRegex = regexp.MustCompile(`(http|https)://[^/]+(/.*)`)

// ParseServices 解析ONVIF设备发现的 services
func ParseServices(services []string) map[string]string {
	result := make(map[string]string)
	for _, service := range services {
		matches := serviceURLRegex.FindStringSubmatch(service)
		if len(matches) == 3 {
			// matches[2] is the path
			path := matches[2]
			parts := strings.Split(path, "/")
			if len(parts) > 2 {
				// 通常路径是 /Service/SubService
				serviceName := parts[len(parts)-2]
				result[serviceName] = service
			}
		}
	}
	return result
}

// DeviceDiscoveryResult 封装 ONVIF 发现 Scope 中的信息
type DeviceDiscoveryResult struct {
	Name         string
	Location     string
	Hardware     string
	Manufacturer string
	Model        string
	SourceIP     string
	XAddr        string            // 设备XADDR地址
	Types        []string          // 设备服务类型列表
	Scopes       []string          // 原始Scopes列表
	Extras       map[string]string // 额外属性
}

func ParseDiscoveryScopes(scopes string) *DeviceDiscoveryResult {
	result := &DeviceDiscoveryResult{Extras: make(map[string]string)}
	scopeList := strings.Fields(scopes)
	for _, scope := range scopeList {
		scope = strings.TrimSpace(scope)
		if strings.HasPrefix(scope, "onvif://www.onvif.org/") {
			parts := strings.SplitN(strings.TrimPrefix(scope, "onvif://www.onvif.org/"), "/", 2)
			if len(parts) == 2 {
				key := strings.ToLower(parts[0])
				value := parts[1]
				switch key {
				case "name":
					result.Name = value
				case "location":
					result.Location = value
				case "hardware":
					result.Hardware = value
				case "manufacturer":
					result.Manufacturer = value
				case "model":
					result.Model = value
				default:
					result.Extras[key] = value
				}
			}
		}
	}
	return result
}

func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d秒", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d分%d秒", int(d.Minutes()), int(d.Seconds())%60)
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%d小时%d分", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%d天%d小时", int(d.Hours())/24, int(d.Hours())%24)
}

// DeviceParams ONVIF设备参数
type DeviceParams struct {
	Xaddr    string
	Username string
	Password string
	Timeout  time.Duration
}

// ONVIFDeviceClient 封装的ONVIF设备客户端
type ONVIFDeviceClient struct {
	client *ONVIFDevice
	xaddr  string
}

// NewDevice 创建设备实例
func NewDevice(params DeviceParams) (*ONVIFDeviceClient, error) {
	client := NewONVIFDevice(params.Username, params.Password)
	if err := client.Connect(params.Xaddr); err != nil {
		return nil, err
	}

	return &ONVIFDeviceClient{
		client: client.(*ONVIFDevice),
		xaddr:  params.Xaddr,
	}, nil
}

// TestConnection 测试设备连接
func (d *ONVIFDeviceClient) TestConnection() error {
	if d.client == nil {
		return fmt.Errorf("device client is nil")
	}
	_, err := d.client.GetSystemDateAndTime()
	return err
}

// GetServices 获取服务列表
func (d *ONVIFDeviceClient) GetServices() map[string]string {
	services := make(map[string]string)
	if d.client != nil && d.client.sdkDevice != nil {
		// 从 goonvif Device 获取真实的服务端点
		endpoints := d.client.sdkDevice.GetServices()
		if len(endpoints) > 0 {
			log.Printf("[ONVIF] 📡 从 goonvif 获取的服务端点:")
			for serviceName, serviceAddr := range endpoints {
				log.Printf("[ONVIF]   - %s: %s", serviceName, serviceAddr)
				services[serviceName] = serviceAddr
			}
		} else {
			log.Printf("[ONVIF] ⚠️  goonvif 未返回任何服务端点")
		}
	} else {
		log.Printf("[ONVIF] ⚠️  设备客户端未初始化")
	}
	return services
}

// GetDeviceInfo 获取设备信息
func (d *ONVIFDeviceClient) GetDeviceInfo() (map[string]string, error) {
	info := make(map[string]string)
	if d.client == nil {
		return info, fmt.Errorf("device client is nil")
	}

	devInfo, err := d.client.GetDeviceInformation()
	if err != nil {
		return info, err
	}

	info["Manufacturer"] = devInfo.Manufacturer
	info["Model"] = devInfo.Model
	info["FirmwareVersion"] = devInfo.FirmwareVersion
	info["SerialNumber"] = devInfo.SerialNumber
	info["HardwareId"] = devInfo.HardwareID
	return info, nil
}

// GetCapabilities 获取设备能力
func (d *ONVIFDeviceClient) GetCapabilities() *DeviceCapabilities {
	if d.client == nil {
		return nil
	}
	caps, err := d.client.GetCapabilities()
	if err != nil {
		return nil
	}

	// 检查PTZ支持
	hasPTZ := false
	if caps != nil && caps.PTZ != nil {
		hasPTZ = caps.PTZ != nil
	}

	return &DeviceCapabilities{
		HasPTZ: hasPTZ,
		Media:  caps.Media,
		PTZ:    caps.PTZ,
	}
}

// GetMediaProfiles 获取媒体配置文件
func (d *ONVIFDeviceClient) GetMediaProfiles() ([]MediaProfile, error) {
	if d.client == nil {
		return nil, fmt.Errorf("device client is nil")
	}
	return d.client.GetProfiles()
}

// GetStreamURI 获取流地址
func (d *ONVIFDeviceClient) GetStreamURI(profileToken string) (string, error) {
	if d.client == nil {
		return "", fmt.Errorf("device client is nil")
	}
	return d.client.GetStreamURI(profileToken)
}

// GetSnapshotURI 获取快照地址
func (d *ONVIFDeviceClient) GetSnapshotURI(profileToken string) (string, error) {
	if d.client == nil {
		return "", fmt.Errorf("device client is nil")
	}
	return d.client.GetSnapshotURI(profileToken)
}

// GetSnapshot 获取快照
func (d *ONVIFDeviceClient) GetSnapshot(profileToken string) ([]byte, string, error) {
	snapshotURL, err := d.GetSnapshotURI(profileToken)
	if err != nil {
		return nil, "", err
	}
	// 返回URL而非二进制数据
	return nil, snapshotURL, nil
}

// PTZContinuousMove PTZ连续移动
func (d *ONVIFDeviceClient) PTZContinuousMove(profileToken string, velocity *PTZVector, timeout float64) error {
	if d.client == nil {
		return fmt.Errorf("device client is nil")
	}
	return d.client.ContinuousMove(profileToken, velocity)
}

// PTZStop PTZ停止
func (d *ONVIFDeviceClient) PTZStop(profileToken string, panTilt, zoom bool) error {
	if d.client == nil {
		return fmt.Errorf("device client is nil")
	}
	return d.client.Stop(profileToken)
}

// GotoHomePosition 移动到主页位置
func (d *ONVIFDeviceClient) GotoHomePosition(profileToken string, speed *PTZVector) error {
	if d.client == nil {
		return fmt.Errorf("device client is nil")
	}
	// 使用预置位1作为主页
	return d.client.GotoPreset(profileToken, "1", speed)
}

// GotoPreset 移动到预置位
func (d *ONVIFDeviceClient) GotoPreset(profileToken, presetToken string, speed *PTZVector) error {
	if d.client == nil {
		return fmt.Errorf("device client is nil")
	}
	return d.client.GotoPreset(profileToken, presetToken, speed)
}

// GetPTZPresets 获取预置位列表
func (d *ONVIFDeviceClient) GetPTZPresets(profileToken string) ([]PTZPreset, error) {
	if d.client == nil {
		return nil, fmt.Errorf("device client is nil")
	}
	return d.client.GetPresets(profileToken)
}

// SetPreset 设置预置位
func (d *ONVIFDeviceClient) SetPreset(profileToken, presetName, presetToken string) (string, error) {
	if d.client == nil {
		return "", fmt.Errorf("device client is nil")
	}
	return d.client.SetPreset(profileToken, presetName, presetToken)
}

// GetSystemDateAndTime 获取系统时间
func (d *ONVIFDeviceClient) GetSystemDateAndTime() (interface{}, error) {
	if d.client == nil {
		return nil, fmt.Errorf("device client is nil")
	}
	return d.client.GetSystemDateAndTime()
}

// RemovePreset 删除预置位
func (d *ONVIFDeviceClient) RemovePreset(profileToken, presetToken string) error {
	if d.client == nil {
		return fmt.Errorf("device client is nil")
	}
	return d.client.RemovePreset(profileToken, presetToken)
}

// ValidateIPAddress 验证IP地址
func ValidateIPAddress(ip string) bool {
	return net.ParseIP(ip) != nil
}

// ValidatePort 验证端口号
func ValidatePort(port int) bool {
	return port > 0 && port < 65536
}

// GenerateUUID 生成UUID
func GenerateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// BuildWSDiscoveryProbe 构建WS-Discovery探测消息
func BuildWSDiscoveryProbe(messageID string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Envelope xmlns="http://www.w3.org/2003/05/soap-envelope" xmlns:a="http://schemas.xmlsoap.org/ws/2004/08/addressing" xmlns:d="http://schemas.xmlsoap.org/ws/2005/04/discovery" xmlns:dn="http://schemas.microsoft.com/ws/2005/04/discovery/wsaddressing">
  <Header>
    <a:Action>http://schemas.xmlsoap.org/ws/2005/04/discovery/Probe</a:Action>
    <a:MessageID>urn:uuid:%s</a:MessageID>
    <a:ReplyTo>
      <a:Address>http://schemas.xmlsoap.org/ws/2004/08/addressing/role/anonymous</a:Address>
    </a:ReplyTo>
    <a:To>urn:schemas-xmlsoap-org:ws:2005:04:discovery</a:To>
  </Header>
  <Body>
    <d:Probe>
      <d:Types>dn:NetworkVideoTransmitter</d:Types>
    </d:Probe>
  </Body>
</Envelope>`, messageID)
}

// DiscoveryResponse WS-Discovery响应结构
type DiscoveryResponse struct {
	XMLName xml.Name `xml:"http://www.w3.org/2003/05/soap-envelope Envelope"`
	Body    struct {
		ProbeMatches struct {
			ProbeMatch []struct {
				EndpointReference struct {
					Address string `xml:"Address"`
				} `xml:"http://schemas.xmlsoap.org/ws/2004/08/addressing EndpointReference"`
				Types           string `xml:"http://schemas.xmlsoap.org/ws/2005/04/discovery Types"`
				Scopes          string `xml:"http://schemas.xmlsoap.org/ws/2005/04/discovery Scopes"`
				XAddrs          string `xml:"http://schemas.xmlsoap.org/ws/2005/04/discovery XAddrs"`
				MetadataVersion int    `xml:"http://schemas.xmlsoap.org/ws/2005/04/discovery MetadataVersion"`
			} `xml:"http://schemas.xmlsoap.org/ws/2005/04/discovery ProbeMatch"`
		} `xml:"http://schemas.xmlsoap.org/ws/2005/04/discovery ProbeMatches"`
	} `xml:"http://www.w3.org/2003/05/soap-envelope Body"`
}
