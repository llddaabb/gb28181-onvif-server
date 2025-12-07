package onvif

import (
	"encoding/xml"
	"fmt"
	"log"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// normalizeXAddr 规范化 XADDR 地址格式
// 支持多种格式：
// - 192.168.1.100:8080
// - http://192.168.1.100:8080
// - http://192.168.1.100:8080/onvif/device_service
func normalizeXAddr(xaddr string) string {
	if xaddr == "" {
		return ""
	}

	// 移除首尾空格
	xaddr = strings.TrimSpace(xaddr)

	// 如果已经是完整的 http URL
	if strings.HasPrefix(xaddr, "http://") || strings.HasPrefix(xaddr, "https://") {
		// 先移除可能存在的重复后缀
		for strings.HasSuffix(xaddr, "/onvif/device_service/onvif/device_service") {
			xaddr = strings.TrimSuffix(xaddr, "/onvif/device_service")
		}

		// 确保包含且只有一个 /onvif/device_service 后缀
		if !strings.HasSuffix(xaddr, "/onvif/device_service") {
			// 移除末尾斜杠
			xaddr = strings.TrimSuffix(xaddr, "/")
			xaddr += "/onvif/device_service"
		}
		return xaddr
	}

	// 如果是 IP:Port 格式，补充 http:// 和路径
	// 先移除可能存在的路径
	if idx := strings.Index(xaddr, "/"); idx != -1 {
		xaddr = xaddr[:idx]
	}
	return fmt.Sprintf("http://%s/onvif/device_service", xaddr)
}

// ParseXAddr 解析XADDR地址，返回主机和端口
func ParseXAddr(xaddr string) (host string, port int, err error) {
	xaddr = normalizeXAddr(xaddr)

	// 移除 http:// 或 https://
	xaddr = strings.TrimPrefix(xaddr, "http://")
	xaddr = strings.TrimPrefix(xaddr, "https://")

	// 提取主机:端口部分
	hostPort := xaddr
	if idx := strings.Index(xaddr, "/"); idx != -1 {
		hostPort = xaddr[:idx]
	}

	// 解析主机和端口
	if idx := strings.LastIndex(hostPort, ":"); idx != -1 {
		host = hostPort[:idx]
		port, err = strconv.Atoi(hostPort[idx+1:])
		if err != nil {
			return "", 0, fmt.Errorf("无效的端口号: %v", err)
		}
	} else {
		host = hostPort
		port = 80 // 默认HTTP端口
	}

	return host, port, nil
}

// ValidateIPAddress 验证IP地址格式
func ValidateIPAddress(ip string) bool {
	// IPv4正则
	ipv4Pattern := `^(\d{1,3}\.){3}\d{1,3}$`
	matched, _ := regexp.MatchString(ipv4Pattern, ip)
	if !matched {
		return false
	}

	// 检查每个部分是否在0-255范围内
	parts := strings.Split(ip, ".")
	for _, part := range parts {
		num, err := strconv.Atoi(part)
		if err != nil || num < 0 || num > 255 {
			return false
		}
	}

	return true
}

// ValidatePort 验证端口号
func ValidatePort(port int) bool {
	return port > 0 && port <= 65535
}

// GetDevicesByNetwork 获取特定网络接口的所有设备
// 用于支持多网卡场景下的设备管理
func (m *Manager) GetDevicesByNetwork(ipPrefix string) []*Device {
	m.devicesMux.RLock()
	defer m.devicesMux.RUnlock()

	var devices []*Device
	for _, device := range m.devices {
		// 检查设备 IP 是否匹配网络前缀
		if strings.HasPrefix(device.IP, ipPrefix) {
			devices = append(devices, device)
		}
	}

	return devices
}

// RefreshDevice 刷新设备信息（更新为不同网卡的 IP）
// 用于多网卡设备迁移场景
func (m *Manager) RefreshDevice(deviceID string, newIP string, newPort int) error {
	m.devicesMux.Lock()
	defer m.devicesMux.Unlock()

	device, exists := m.devices[deviceID]
	if !exists {
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	// 验证新IP和端口
	if !ValidateIPAddress(newIP) {
		return fmt.Errorf("无效的IP地址: %s", newIP)
	}
	if !ValidatePort(newPort) {
		return fmt.Errorf("无效的端口号: %d", newPort)
	}

	oldIP := device.IP
	oldPort := device.Port

	// 更新设备信息
	device.IP = newIP
	device.Port = newPort
	device.LastSeenTime = time.Now()
	device.Status = "unknown" // 标记为未知状态，等待下次检查

	log.Printf("[ONVIF] 🔄 设备信息更新: ID=%s | 旧地址=%s:%d | 新地址=%s:%d",
		deviceID, oldIP, oldPort, newIP, newPort)

	return nil
}

// GetLocalIPAddresses 获取本机所有网络接口的IP地址
func GetLocalIPAddresses() ([]string, error) {
	var ips []string

	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("获取网络接口失败: %w", err)
	}

	for _, iface := range interfaces {
		// 跳过回环接口和未启用的接口
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// 只要IPv4地址
			if ip == nil || ip.IsLoopback() || ip.To4() == nil {
				continue
			}

			ips = append(ips, ip.String())
		}
	}

	return ips, nil
}

// GetNetworkInterfaces 获取所有网络接口信息
func GetNetworkInterfaces() ([]NetworkInterface, error) {
	var result []NetworkInterface

	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("获取网络接口失败: %w", err)
	}

	for _, iface := range interfaces {
		// 跳过未启用的接口
		if iface.Flags&net.FlagUp == 0 {
			continue
		}

		ni := NetworkInterface{
			Name:       iface.Name,
			MacAddress: iface.HardwareAddr.String(),
			IsUp:       iface.Flags&net.FlagUp != 0,
			IsLoopback: iface.Flags&net.FlagLoopback != 0,
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				if v.IP.To4() != nil {
					ni.IPv4Addresses = append(ni.IPv4Addresses, v.IP.String())
					ni.SubnetMask = net.IP(v.Mask).String()
				} else {
					ni.IPv6Addresses = append(ni.IPv6Addresses, v.IP.String())
				}
			}
		}

		if len(ni.IPv4Addresses) > 0 || len(ni.IPv6Addresses) > 0 {
			result = append(result, ni)
		}
	}

	return result, nil
}

// NetworkInterface 网络接口信息
type NetworkInterface struct {
	Name          string   `json:"name"`
	MacAddress    string   `json:"macAddress"`
	IPv4Addresses []string `json:"ipv4Addresses"`
	IPv6Addresses []string `json:"ipv6Addresses"`
	SubnetMask    string   `json:"subnetMask"`
	IsUp          bool     `json:"isUp"`
	IsLoopback    bool     `json:"isLoopback"`
}

// CalculateSubnet 计算子网地址
func CalculateSubnet(ip string, mask string) (string, error) {
	ipAddr := net.ParseIP(ip)
	if ipAddr == nil {
		return "", fmt.Errorf("无效的IP地址: %s", ip)
	}

	maskAddr := net.ParseIP(mask)
	if maskAddr == nil {
		return "", fmt.Errorf("无效的子网掩码: %s", mask)
	}

	ipv4 := ipAddr.To4()
	maskv4 := maskAddr.To4()
	if ipv4 == nil || maskv4 == nil {
		return "", fmt.Errorf("只支持IPv4地址")
	}

	subnet := make(net.IP, 4)
	for i := 0; i < 4; i++ {
		subnet[i] = ipv4[i] & maskv4[i]
	}

	return subnet.String(), nil
}

// GenerateIPRange 生成IP地址范围
func GenerateIPRange(startIP, endIP string) ([]string, error) {
	start := net.ParseIP(startIP).To4()
	end := net.ParseIP(endIP).To4()

	if start == nil || end == nil {
		return nil, fmt.Errorf("无效的IP地址范围")
	}

	var ips []string
	for i := ipToInt(start); i <= ipToInt(end); i++ {
		ips = append(ips, intToIP(i).String())
	}

	return ips, nil
}

// ipToInt 将IP地址转换为整数
func ipToInt(ip net.IP) uint32 {
	ip = ip.To4()
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

// intToIP 将整数转换为IP地址
func intToIP(n uint32) net.IP {
	return net.IPv4(byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
}

// ScanIPRange 扫描IP地址范围内的ONVIF设备
func ScanIPRange(startIP, endIP string, port int, timeout time.Duration) ([]string, error) {
	ips, err := GenerateIPRange(startIP, endIP)
	if err != nil {
		return nil, err
	}

	var discovered []string
	results := make(chan string, len(ips))

	// 并发扫描
	for _, ip := range ips {
		go func(ip string) {
			// 使用JoinHostPort正确处理IPv4/IPv6
			addr := net.JoinHostPort(ip, strconv.Itoa(port))
			conn, err := net.DialTimeout("tcp", addr, timeout)
			if err == nil {
				conn.Close()
				results <- ip
			} else {
				results <- ""
			}
		}(ip)
	}

	// 收集结果
	for range ips {
		if ip := <-results; ip != "" {
			discovered = append(discovered, ip)
		}
	}

	return discovered, nil
}

// DeviceDiscoveryResult 设备发现结果
type DeviceDiscoveryResult struct {
	XAddr        string            `json:"xaddr"`
	Types        []string          `json:"types"`
	Scopes       []string          `json:"scopes"`
	Manufacturer string            `json:"manufacturer"`
	Model        string            `json:"model"`
	Name         string            `json:"name"`
	Location     string            `json:"location"`
	Hardware     string            `json:"hardware"`
	SourceIP     string            `json:"sourceIP"` // 响应来源IP
	Extras       map[string]string `json:"extras"`
}

// ParseDiscoveryScopes 解析设备发现的Scopes字段
func ParseDiscoveryScopes(scopes string) *DeviceDiscoveryResult {
	result := &DeviceDiscoveryResult{
		Extras: make(map[string]string),
	}

	scopeList := strings.Fields(scopes)
	for _, scope := range scopeList {
		scope = strings.TrimSpace(scope)

		// 解析ONVIF标准scope格式
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

// FormatDuration 格式化时间间隔
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d秒", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%d分%d秒", int(d.Minutes()), int(d.Seconds())%60)
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%d小时%d分", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%d天%d小时", int(d.Hours())/24, int(d.Hours())%24)
}

// WSDiscoveryProbe WS-Discovery探测消息
type WSDiscoveryProbe struct {
	XMLName   xml.Name `xml:"Envelope"`
	Namespace string   `xml:"xmlns:s,attr"`
	Header    struct {
		Action    string `xml:"Action"`
		MessageID string `xml:"MessageID"`
		To        string `xml:"To"`
	} `xml:"Header"`
	Body struct {
		Probe struct {
			Types  string `xml:"Types,omitempty"`
			Scopes string `xml:"Scopes,omitempty"`
		} `xml:"Probe"`
	} `xml:"Body"`
}

// BuildWSDiscoveryProbe 构建WS-Discovery探测消息
func BuildWSDiscoveryProbe(messageID string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope" 
            xmlns:a="http://schemas.xmlsoap.org/ws/2004/08/addressing"
            xmlns:d="http://schemas.xmlsoap.org/ws/2005/04/discovery"
            xmlns:dn="http://www.onvif.org/ver10/network/wsdl">
  <s:Header>
    <a:Action>http://schemas.xmlsoap.org/ws/2005/04/discovery/Probe</a:Action>
    <a:MessageID>uuid:%s</a:MessageID>
    <a:To>urn:schemas-xmlsoap-org:ws:2005:04:discovery</a:To>
  </s:Header>
  <s:Body>
    <d:Probe>
      <d:Types>dn:NetworkVideoTransmitter</d:Types>
    </d:Probe>
  </s:Body>
</s:Envelope>`, messageID)
}

// GenerateUUID 生成简单的UUID
func GenerateUUID() string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		time.Now().UnixNano()&0xffffffff,
		time.Now().UnixNano()>>32&0xffff,
		0x4000|time.Now().UnixNano()>>48&0x0fff,
		0x8000|time.Now().UnixNano()>>60&0x3fff,
		time.Now().UnixNano())
}
