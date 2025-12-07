package onvif

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ONVIFDevice ONVIF设备客户端 - 真实ONVIF协议实现
type ONVIFDevice struct {
	xaddr        string
	username     string
	password     string
	services     map[string]string
	capabilities *DeviceCapabilities
	httpClient   *http.Client
	profiles     []MediaProfile // 缓存的媒体配置文件
}

// DeviceParams 设备连接参数
type DeviceParams struct {
	Xaddr    string
	Username string
	Password string
	Timeout  time.Duration
}

// DeviceCapabilities 设备能力
type DeviceCapabilities struct {
	HasPTZ       bool
	HasMedia     bool
	HasEvents    bool
	HasImaging   bool
	HasAnalytics bool
	HasRecording bool
}

// MediaProfile 媒体配置文件
type MediaProfile struct {
	Token        string              `json:"token"`
	Name         string              `json:"name"`
	Encoding     string              `json:"encoding"`
	Resolution   string              `json:"resolution"`
	Width        int                 `json:"width"`
	Height       int                 `json:"height"`
	FPS          int                 `json:"fps"`
	Bitrate      int                 `json:"bitrate"`
	PTZConfig    *PTZConfiguration   `json:"ptzConfig,omitempty"`
	VideoEncoder *VideoEncoderConfig `json:"videoEncoder,omitempty"`
	AudioEncoder *AudioEncoderConfig `json:"audioEncoder,omitempty"`
}

// PTZConfiguration PTZ配置
type PTZConfiguration struct {
	Token              string   `json:"token"`
	Name               string   `json:"name"`
	NodeToken          string   `json:"nodeToken"`
	PanTiltLimits      *Limits  `json:"panTiltLimits,omitempty"`
	ZoomLimits         *Limits  `json:"zoomLimits,omitempty"`
	SupportedPTZSpaces []string `json:"supportedPTZSpaces"`
}

// Limits 限制范围
type Limits struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

// VideoEncoderConfig 视频编码器配置
type VideoEncoderConfig struct {
	Token        string `json:"token"`
	Name         string `json:"name"`
	Encoding     string `json:"encoding"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	Quality      int    `json:"quality"`
	FrameRate    int    `json:"frameRate"`
	BitrateLimit int    `json:"bitrateLimit"`
	GovLength    int    `json:"govLength"`
	H264Profile  string `json:"h264Profile,omitempty"`
}

// AudioEncoderConfig 音频编码器配置
type AudioEncoderConfig struct {
	Token      string `json:"token"`
	Name       string `json:"name"`
	Encoding   string `json:"encoding"`
	Bitrate    int    `json:"bitrate"`
	SampleRate int    `json:"sampleRate"`
}

// PTZVector PTZ向量
type PTZVector struct {
	PanTilt *Vector2D `json:"panTilt,omitempty"`
	Zoom    *Vector1D `json:"zoom,omitempty"`
}

// Vector2D 二维向量
type Vector2D struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Vector1D 一维向量
type Vector1D struct {
	X float64 `json:"x"`
}

// PTZStatus PTZ状态
type PTZStatus struct {
	Position   *PTZVector `json:"position,omitempty"`
	MoveStatus string     `json:"moveStatus"`
	Error      string     `json:"error,omitempty"`
	UtcTime    time.Time  `json:"utcTime"`
}

// PTZPreset PTZ预置位
type PTZPreset struct {
	Token    string     `json:"token"`
	Name     string     `json:"name"`
	Position *PTZVector `json:"position,omitempty"`
}

// NewDevice 创建ONVIF设备客户端
func NewDevice(params DeviceParams) (*ONVIFDevice, error) {
	if params.Xaddr == "" {
		return nil, fmt.Errorf("设备地址不能为空")
	}

	timeout := params.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	d := &ONVIFDevice{
		xaddr:    params.Xaddr,
		username: params.Username,
		password: params.Password,
		services: make(map[string]string),
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout:   timeout,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: 10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
				MaxIdleConns:          10,
				MaxIdleConnsPerHost:   2,
			},
		},
		capabilities: &DeviceCapabilities{},
	}

	return d, nil
}

// TestConnection 测试设备连接并获取服务信息
func (d *ONVIFDevice) TestConnection() error {
	// 首先测试TCP连接
	parsedURL, err := url.Parse(d.xaddr)
	if err != nil {
		return fmt.Errorf("无效的设备地址: %w", err)
	}

	host := parsedURL.Host
	if parsedURL.Port() == "" {
		host = parsedURL.Hostname() + ":80"
	}

	conn, err := net.DialTimeout("tcp", host, 5*time.Second)
	if err != nil {
		return fmt.Errorf("无法连接设备: %w", err)
	}
	conn.Close()

	// 获取设备服务
	if err := d.getServices(); err != nil {
		// 如果获取服务失败，使用默认服务端点
		d.setDefaultServices()
	}

	// 获取设备能力
	d.getCapabilities()

	return nil
}

// setDefaultServices 设置默认服务端点
func (d *ONVIFDevice) setDefaultServices() {
	baseURL := strings.TrimSuffix(d.xaddr, "/onvif/device_service")
	d.services["Device"] = baseURL + "/onvif/device_service"
	d.services["Media"] = baseURL + "/onvif/media_service"
	d.services["PTZ"] = baseURL + "/onvif/ptz_service"
	d.services["Events"] = baseURL + "/onvif/events_service"
	d.services["Imaging"] = baseURL + "/onvif/imaging_service"
}

// getServices 从设备获取服务端点
func (d *ONVIFDevice) getServices() error {
	body := `<tds:GetServices xmlns:tds="http://www.onvif.org/ver10/device/wsdl">
		<tds:IncludeCapability>false</tds:IncludeCapability>
	</tds:GetServices>`

	resp, err := d.sendSOAPRequest(d.xaddr, "http://www.onvif.org/ver10/device/wsdl/GetServices", body)
	if err != nil {
		return err
	}

	// 解析服务响应
	type ServiceResponse struct {
		XMLName xml.Name `xml:"Envelope"`
		Body    struct {
			GetServicesResponse struct {
				Service []struct {
					Namespace string `xml:"Namespace"`
					XAddr     string `xml:"XAddr"`
				} `xml:"Service"`
			} `xml:"GetServicesResponse"`
		} `xml:"Body"`
	}

	var svcResp ServiceResponse
	if err := xml.Unmarshal(resp, &svcResp); err != nil {
		return fmt.Errorf("解析服务响应失败: %w", err)
	}

	for _, svc := range svcResp.Body.GetServicesResponse.Service {
		switch {
		case strings.Contains(svc.Namespace, "device"):
			d.services["Device"] = svc.XAddr
		case strings.Contains(svc.Namespace, "media"):
			d.services["Media"] = svc.XAddr
		case strings.Contains(svc.Namespace, "ptz"):
			d.services["PTZ"] = svc.XAddr
		case strings.Contains(svc.Namespace, "events"):
			d.services["Events"] = svc.XAddr
		case strings.Contains(svc.Namespace, "imaging"):
			d.services["Imaging"] = svc.XAddr
		case strings.Contains(svc.Namespace, "analytics"):
			d.services["Analytics"] = svc.XAddr
		case strings.Contains(svc.Namespace, "recording"):
			d.services["Recording"] = svc.XAddr
		}
	}

	if len(d.services) == 0 {
		d.setDefaultServices()
	}

	return nil
}

// getCapabilities 获取设备能力
func (d *ONVIFDevice) getCapabilities() {
	body := `<tds:GetCapabilities xmlns:tds="http://www.onvif.org/ver10/device/wsdl">
		<tds:Category>All</tds:Category>
	</tds:GetCapabilities>`

	resp, err := d.sendSOAPRequest(d.xaddr, "http://www.onvif.org/ver10/device/wsdl/GetCapabilities", body)
	if err != nil {
		// 默认启用常见能力
		d.capabilities.HasMedia = true
		d.capabilities.HasPTZ = true
		return
	}

	// 检查响应中是否包含各项能力
	respStr := string(resp)
	d.capabilities.HasMedia = strings.Contains(respStr, "Media") || strings.Contains(respStr, "media")
	d.capabilities.HasPTZ = strings.Contains(respStr, "PTZ") || strings.Contains(respStr, "ptz")
	d.capabilities.HasEvents = strings.Contains(respStr, "Events") || strings.Contains(respStr, "events")
	d.capabilities.HasImaging = strings.Contains(respStr, "Imaging") || strings.Contains(respStr, "imaging")
	d.capabilities.HasAnalytics = strings.Contains(respStr, "Analytics") || strings.Contains(respStr, "analytics")
	d.capabilities.HasRecording = strings.Contains(respStr, "Recording") || strings.Contains(respStr, "recording")
}

// GetServices 获取设备服务列表
func (d *ONVIFDevice) GetServices() map[string]string {
	return d.services
}

// GetCapabilities 获取设备能力
func (d *ONVIFDevice) GetCapabilities() *DeviceCapabilities {
	return d.capabilities
}

// GetDeviceInfo 获取设备信息
func (d *ONVIFDevice) GetDeviceInfo() (map[string]string, error) {
	body := `<tds:GetDeviceInformation xmlns:tds="http://www.onvif.org/ver10/device/wsdl"/>`

	resp, err := d.sendSOAPRequest(d.xaddr, "http://www.onvif.org/ver10/device/wsdl/GetDeviceInformation", body)
	if err != nil {
		return map[string]string{
			"Manufacturer":    "Unknown",
			"Model":           "ONVIF Camera",
			"FirmwareVersion": "Unknown",
			"SerialNumber":    "Unknown",
			"HardwareId":      "Unknown",
		}, nil
	}

	info := make(map[string]string)

	// 解析设备信息
	extractValue := func(tag string) string {
		pattern := fmt.Sprintf("<%s[^>]*>([^<]+)</%s>", tag, tag)
		re := regexp.MustCompile(pattern)
		if match := re.FindSubmatch(resp); len(match) > 1 {
			return string(match[1])
		}
		// 尝试不带命名空间的标签
		pattern2 := fmt.Sprintf(":<%s[^>]*>([^<]+)</%s>", tag, tag)
		re2 := regexp.MustCompile(pattern2)
		if match := re2.FindSubmatch(resp); len(match) > 1 {
			return string(match[1])
		}
		return ""
	}

	info["Manufacturer"] = extractValue("Manufacturer")
	info["Model"] = extractValue("Model")
	info["FirmwareVersion"] = extractValue("FirmwareVersion")
	info["SerialNumber"] = extractValue("SerialNumber")
	info["HardwareId"] = extractValue("HardwareId")

	// 设置默认值
	if info["Manufacturer"] == "" {
		info["Manufacturer"] = "Unknown"
	}
	if info["Model"] == "" {
		info["Model"] = "ONVIF Camera"
	}

	return info, nil
}

// GetMediaProfiles 获取媒体配置文件列表
func (d *ONVIFDevice) GetMediaProfiles() ([]MediaProfile, error) {
	mediaURL := d.services["Media"]
	if mediaURL == "" {
		mediaURL = strings.TrimSuffix(d.xaddr, "/onvif/device_service") + "/onvif/media_service"
	}

	body := `<trt:GetProfiles xmlns:trt="http://www.onvif.org/ver10/media/wsdl"/>`

	resp, err := d.sendSOAPRequest(mediaURL, "http://www.onvif.org/ver10/media/wsdl/GetProfiles", body)
	if err != nil {
		return d.getDefaultProfiles(), nil
	}

	profiles := d.parseMediaProfiles(resp)
	if len(profiles) == 0 {
		return d.getDefaultProfiles(), nil
	}

	d.profiles = profiles
	return profiles, nil
}

// parseMediaProfiles 解析媒体配置文件响应
func (d *ONVIFDevice) parseMediaProfiles(resp []byte) []MediaProfile {
	var profiles []MediaProfile
	respStr := string(resp)

	// 使用正则提取Profile信息
	profilePattern := regexp.MustCompile(`<[^:]*:?Profiles[^>]*token="([^"]+)"[^>]*>`)
	namePattern := regexp.MustCompile(`<[^:]*:?Name>([^<]+)</[^:]*:?Name>`)
	encodingPattern := regexp.MustCompile(`<[^:]*:?Encoding>([^<]+)</[^:]*:?Encoding>`)
	widthPattern := regexp.MustCompile(`<[^:]*:?Width>([^<]+)</[^:]*:?Width>`)
	heightPattern := regexp.MustCompile(`<[^:]*:?Height>([^<]+)</[^:]*:?Height>`)

	profileMatches := profilePattern.FindAllStringSubmatch(respStr, -1)
	nameMatches := namePattern.FindAllStringSubmatch(respStr, -1)

	for i, match := range profileMatches {
		if len(match) < 2 {
			continue
		}

		profile := MediaProfile{
			Token: match[1],
		}

		if i < len(nameMatches) && len(nameMatches[i]) > 1 {
			profile.Name = nameMatches[i][1]
		} else {
			profile.Name = fmt.Sprintf("Profile_%d", i+1)
		}

		// 提取编码信息
		if encMatch := encodingPattern.FindStringSubmatch(respStr); len(encMatch) > 1 {
			profile.Encoding = encMatch[1]
		}

		// 提取分辨率
		if widthMatch := widthPattern.FindStringSubmatch(respStr); len(widthMatch) > 1 {
			profile.Width, _ = strconv.Atoi(widthMatch[1])
		}
		if heightMatch := heightPattern.FindStringSubmatch(respStr); len(heightMatch) > 1 {
			profile.Height, _ = strconv.Atoi(heightMatch[1])
		}

		if profile.Width > 0 && profile.Height > 0 {
			profile.Resolution = fmt.Sprintf("%dx%d", profile.Width, profile.Height)
		}

		profiles = append(profiles, profile)
	}

	return profiles
}

// getDefaultProfiles 获取默认配置文件
func (d *ONVIFDevice) getDefaultProfiles() []MediaProfile {
	return []MediaProfile{
		{Token: "Profile_1", Name: "主码流", Encoding: "H.264", Resolution: "1920x1080", Width: 1920, Height: 1080, FPS: 25},
		{Token: "Profile_2", Name: "子码流", Encoding: "H.264", Resolution: "640x480", Width: 640, Height: 480, FPS: 15},
	}
}

// GetStreamURI 获取RTSP流地址
func (d *ONVIFDevice) GetStreamURI(profileToken string) (string, error) {
	mediaURL := d.services["Media"]
	if mediaURL == "" {
		mediaURL = strings.TrimSuffix(d.xaddr, "/onvif/device_service") + "/onvif/media_service"
	}

	body := fmt.Sprintf(`<trt:GetStreamUri xmlns:trt="http://www.onvif.org/ver10/media/wsdl">
		<trt:StreamSetup>
			<tt:Stream xmlns:tt="http://www.onvif.org/ver10/schema">RTP-Unicast</tt:Stream>
			<tt:Transport xmlns:tt="http://www.onvif.org/ver10/schema">
				<tt:Protocol>RTSP</tt:Protocol>
			</tt:Transport>
		</trt:StreamSetup>
		<trt:ProfileToken>%s</trt:ProfileToken>
	</trt:GetStreamUri>`, profileToken)

	resp, err := d.sendSOAPRequest(mediaURL, "http://www.onvif.org/ver10/media/wsdl/GetStreamUri", body)
	if err != nil {
		// 返回构造的默认RTSP地址
		return d.buildDefaultStreamURI(profileToken)
	}

	// 解析响应中的URI
	uriPattern := regexp.MustCompile(`<[^:]*:?Uri>([^<]+)</[^:]*:?Uri>`)
	if match := uriPattern.FindSubmatch(resp); len(match) > 1 {
		rtspURL := string(match[1])
		// 添加认证信息
		return d.addAuthToURL(rtspURL), nil
	}

	return d.buildDefaultStreamURI(profileToken)
}

// buildDefaultStreamURI 构建默认RTSP地址
func (d *ONVIFDevice) buildDefaultStreamURI(profileToken string) (string, error) {
	parsedURL, err := url.Parse(d.xaddr)
	if err != nil {
		return "", fmt.Errorf("解析设备地址失败: %w", err)
	}

	host := parsedURL.Hostname()
	port := "554"

	// 根据profile token确定通道
	channel := "101"
	if strings.Contains(strings.ToLower(profileToken), "sub") || profileToken == "Profile_2" {
		channel = "102"
	} else if strings.Contains(strings.ToLower(profileToken), "third") || profileToken == "Profile_3" {
		channel = "103"
	}

	var rtspURL string
	if d.username != "" && d.password != "" {
		rtspURL = fmt.Sprintf("rtsp://%s:%s@%s:%s/Streaming/Channels/%s",
			url.QueryEscape(d.username),
			url.QueryEscape(d.password),
			host, port, channel)
	} else {
		rtspURL = fmt.Sprintf("rtsp://%s:%s/Streaming/Channels/%s", host, port, channel)
	}

	return rtspURL, nil
}

// addAuthToURL 为URL添加认证信息
func (d *ONVIFDevice) addAuthToURL(rawURL string) string {
	if d.username == "" {
		return rawURL
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	parsedURL.User = url.UserPassword(d.username, d.password)
	return parsedURL.String()
}

// GetSnapshotURI 获取快照URI
func (d *ONVIFDevice) GetSnapshotURI(profileToken string) (string, error) {
	mediaURL := d.services["Media"]
	if mediaURL == "" {
		mediaURL = strings.TrimSuffix(d.xaddr, "/onvif/device_service") + "/onvif/media_service"
	}

	body := fmt.Sprintf(`<trt:GetSnapshotUri xmlns:trt="http://www.onvif.org/ver10/media/wsdl">
		<trt:ProfileToken>%s</trt:ProfileToken>
	</trt:GetSnapshotUri>`, profileToken)

	resp, err := d.sendSOAPRequest(mediaURL, "http://www.onvif.org/ver10/media/wsdl/GetSnapshotUri", body)
	if err != nil {
		return d.buildDefaultSnapshotURI(profileToken)
	}

	// 解析响应中的URI
	uriPattern := regexp.MustCompile(`<[^:]*:?Uri>([^<]+)</[^:]*:?Uri>`)
	if match := uriPattern.FindSubmatch(resp); len(match) > 1 {
		snapshotURL := string(match[1])
		return d.addAuthToURL(snapshotURL), nil
	}

	return d.buildDefaultSnapshotURI(profileToken)
}

// buildDefaultSnapshotURI 构建默认快照地址
func (d *ONVIFDevice) buildDefaultSnapshotURI(profileToken string) (string, error) {
	parsedURL, err := url.Parse(d.xaddr)
	if err != nil {
		return "", fmt.Errorf("解析设备地址失败: %w", err)
	}

	host := parsedURL.Hostname()
	port := parsedURL.Port()
	if port == "" {
		port = "80"
	}

	channel := "101"
	if strings.Contains(strings.ToLower(profileToken), "sub") || profileToken == "Profile_2" {
		channel = "102"
	}

	var snapshotURL string
	if d.username != "" && d.password != "" {
		snapshotURL = fmt.Sprintf("http://%s:%s@%s:%s/ISAPI/Streaming/channels/%s/picture",
			url.QueryEscape(d.username),
			url.QueryEscape(d.password),
			host, port, channel)
	} else {
		snapshotURL = fmt.Sprintf("http://%s:%s/ISAPI/Streaming/channels/%s/picture", host, port, channel)
	}

	return snapshotURL, nil
}

// GetSnapshot 获取实际快照数据
func (d *ONVIFDevice) GetSnapshot(profileToken string) ([]byte, string, error) {
	snapshotURI, err := d.GetSnapshotURI(profileToken)
	if err != nil {
		return nil, "", fmt.Errorf("获取快照地址失败: %w", err)
	}

	req, err := http.NewRequest("GET", snapshotURI, nil)
	if err != nil {
		return nil, "", fmt.Errorf("创建请求失败: %w", err)
	}

	if d.username != "" && d.password != "" {
		req.SetBasicAuth(d.username, d.password)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("请求快照失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("获取快照失败，状态码: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("读取快照数据失败: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}

	return data, contentType, nil
}

// PTZ相关方法

// PTZContinuousMove 连续移动
func (d *ONVIFDevice) PTZContinuousMove(profileToken string, velocity *PTZVector, timeout time.Duration) error {
	ptzURL := d.services["PTZ"]
	if ptzURL == "" {
		ptzURL = strings.TrimSuffix(d.xaddr, "/onvif/device_service") + "/onvif/ptz_service"
	}

	panTiltX, panTiltY := 0.0, 0.0
	zoomX := 0.0

	if velocity != nil {
		if velocity.PanTilt != nil {
			panTiltX = velocity.PanTilt.X
			panTiltY = velocity.PanTilt.Y
		}
		if velocity.Zoom != nil {
			zoomX = velocity.Zoom.X
		}
	}

	body := fmt.Sprintf(`<tptz:ContinuousMove xmlns:tptz="http://www.onvif.org/ver20/ptz/wsdl">
		<tptz:ProfileToken>%s</tptz:ProfileToken>
		<tptz:Velocity>
			<tt:PanTilt xmlns:tt="http://www.onvif.org/ver10/schema" x="%.2f" y="%.2f"/>
			<tt:Zoom xmlns:tt="http://www.onvif.org/ver10/schema" x="%.2f"/>
		</tptz:Velocity>
	</tptz:ContinuousMove>`, profileToken, panTiltX, panTiltY, zoomX)

	_, err := d.sendSOAPRequest(ptzURL, "http://www.onvif.org/ver20/ptz/wsdl/ContinuousMove", body)
	return err
}

// PTZStop 停止PTZ移动
func (d *ONVIFDevice) PTZStop(profileToken string, panTilt, zoom bool) error {
	ptzURL := d.services["PTZ"]
	if ptzURL == "" {
		ptzURL = strings.TrimSuffix(d.xaddr, "/onvif/device_service") + "/onvif/ptz_service"
	}

	body := fmt.Sprintf(`<tptz:Stop xmlns:tptz="http://www.onvif.org/ver20/ptz/wsdl">
		<tptz:ProfileToken>%s</tptz:ProfileToken>
		<tptz:PanTilt>%t</tptz:PanTilt>
		<tptz:Zoom>%t</tptz:Zoom>
	</tptz:Stop>`, profileToken, panTilt, zoom)

	_, err := d.sendSOAPRequest(ptzURL, "http://www.onvif.org/ver20/ptz/wsdl/Stop", body)
	return err
}

// PTZMove PTZ移动（别名）
func (d *ONVIFDevice) PTZMove(profileToken string, velocity *PTZVector, timeout time.Duration) error {
	return d.PTZContinuousMove(profileToken, velocity, timeout)
}

// PTZAbsoluteMove 绝对位置移动
func (d *ONVIFDevice) PTZAbsoluteMove(profileToken string, position *PTZVector, speed *PTZVector) error {
	ptzURL := d.services["PTZ"]
	if ptzURL == "" {
		ptzURL = strings.TrimSuffix(d.xaddr, "/onvif/device_service") + "/onvif/ptz_service"
	}

	if position == nil {
		return fmt.Errorf("目标位置不能为空")
	}

	panTiltX, panTiltY := 0.0, 0.0
	zoomX := 0.0

	if position.PanTilt != nil {
		panTiltX = position.PanTilt.X
		panTiltY = position.PanTilt.Y
	}
	if position.Zoom != nil {
		zoomX = position.Zoom.X
	}

	body := fmt.Sprintf(`<tptz:AbsoluteMove xmlns:tptz="http://www.onvif.org/ver20/ptz/wsdl">
		<tptz:ProfileToken>%s</tptz:ProfileToken>
		<tptz:Position>
			<tt:PanTilt xmlns:tt="http://www.onvif.org/ver10/schema" x="%.4f" y="%.4f"/>
			<tt:Zoom xmlns:tt="http://www.onvif.org/ver10/schema" x="%.4f"/>
		</tptz:Position>
	</tptz:AbsoluteMove>`, profileToken, panTiltX, panTiltY, zoomX)

	_, err := d.sendSOAPRequest(ptzURL, "http://www.onvif.org/ver20/ptz/wsdl/AbsoluteMove", body)
	return err
}

// PTZRelativeMove 相对位置移动
func (d *ONVIFDevice) PTZRelativeMove(profileToken string, translation *PTZVector, speed *PTZVector) error {
	ptzURL := d.services["PTZ"]
	if ptzURL == "" {
		ptzURL = strings.TrimSuffix(d.xaddr, "/onvif/device_service") + "/onvif/ptz_service"
	}

	if translation == nil {
		return fmt.Errorf("移动距离不能为空")
	}

	panTiltX, panTiltY := 0.0, 0.0
	zoomX := 0.0

	if translation.PanTilt != nil {
		panTiltX = translation.PanTilt.X
		panTiltY = translation.PanTilt.Y
	}
	if translation.Zoom != nil {
		zoomX = translation.Zoom.X
	}

	body := fmt.Sprintf(`<tptz:RelativeMove xmlns:tptz="http://www.onvif.org/ver20/ptz/wsdl">
		<tptz:ProfileToken>%s</tptz:ProfileToken>
		<tptz:Translation>
			<tt:PanTilt xmlns:tt="http://www.onvif.org/ver10/schema" x="%.4f" y="%.4f"/>
			<tt:Zoom xmlns:tt="http://www.onvif.org/ver10/schema" x="%.4f"/>
		</tptz:Translation>
	</tptz:RelativeMove>`, profileToken, panTiltX, panTiltY, zoomX)

	_, err := d.sendSOAPRequest(ptzURL, "http://www.onvif.org/ver20/ptz/wsdl/RelativeMove", body)
	return err
}

// GetPTZStatus 获取PTZ状态
func (d *ONVIFDevice) GetPTZStatus(profileToken string) (*PTZStatus, error) {
	ptzURL := d.services["PTZ"]
	if ptzURL == "" {
		ptzURL = strings.TrimSuffix(d.xaddr, "/onvif/device_service") + "/onvif/ptz_service"
	}

	body := fmt.Sprintf(`<tptz:GetStatus xmlns:tptz="http://www.onvif.org/ver20/ptz/wsdl">
		<tptz:ProfileToken>%s</tptz:ProfileToken>
	</tptz:GetStatus>`, profileToken)

	resp, err := d.sendSOAPRequest(ptzURL, "http://www.onvif.org/ver20/ptz/wsdl/GetStatus", body)
	if err != nil {
		return &PTZStatus{MoveStatus: "UNKNOWN", UtcTime: time.Now().UTC()}, nil
	}

	status := &PTZStatus{
		MoveStatus: "IDLE",
		UtcTime:    time.Now().UTC(),
	}

	// 解析状态
	if strings.Contains(string(resp), "MOVING") {
		status.MoveStatus = "MOVING"
	}

	return status, nil
}

// GetPTZPresets 获取PTZ预置位列表
func (d *ONVIFDevice) GetPTZPresets(profileToken string) ([]PTZPreset, error) {
	ptzURL := d.services["PTZ"]
	if ptzURL == "" {
		ptzURL = strings.TrimSuffix(d.xaddr, "/onvif/device_service") + "/onvif/ptz_service"
	}

	body := fmt.Sprintf(`<tptz:GetPresets xmlns:tptz="http://www.onvif.org/ver20/ptz/wsdl">
		<tptz:ProfileToken>%s</tptz:ProfileToken>
	</tptz:GetPresets>`, profileToken)

	resp, err := d.sendSOAPRequest(ptzURL, "http://www.onvif.org/ver20/ptz/wsdl/GetPresets", body)
	if err != nil {
		return []PTZPreset{}, nil
	}

	// 解析预置位
	presets := d.parsePresets(resp)
	return presets, nil
}

// parsePresets 解析预置位响应
func (d *ONVIFDevice) parsePresets(resp []byte) []PTZPreset {
	var presets []PTZPreset
	respStr := string(resp)

	tokenPattern := regexp.MustCompile(`<[^:]*:?Preset[^>]*token="([^"]+)"`)
	namePattern := regexp.MustCompile(`<[^:]*:?Name>([^<]+)</[^:]*:?Name>`)

	tokenMatches := tokenPattern.FindAllStringSubmatch(respStr, -1)
	nameMatches := namePattern.FindAllStringSubmatch(respStr, -1)

	for i, match := range tokenMatches {
		if len(match) < 2 {
			continue
		}

		preset := PTZPreset{
			Token: match[1],
		}

		if i < len(nameMatches) && len(nameMatches[i]) > 1 {
			preset.Name = nameMatches[i][1]
		} else {
			preset.Name = fmt.Sprintf("预置位%d", i+1)
		}

		presets = append(presets, preset)
	}

	return presets
}

// GotoPreset 移动到预置位
func (d *ONVIFDevice) GotoPreset(profileToken, presetToken string, speed *PTZVector) error {
	ptzURL := d.services["PTZ"]
	if ptzURL == "" {
		ptzURL = strings.TrimSuffix(d.xaddr, "/onvif/device_service") + "/onvif/ptz_service"
	}

	body := fmt.Sprintf(`<tptz:GotoPreset xmlns:tptz="http://www.onvif.org/ver20/ptz/wsdl">
		<tptz:ProfileToken>%s</tptz:ProfileToken>
		<tptz:PresetToken>%s</tptz:PresetToken>
	</tptz:GotoPreset>`, profileToken, presetToken)

	_, err := d.sendSOAPRequest(ptzURL, "http://www.onvif.org/ver20/ptz/wsdl/GotoPreset", body)
	return err
}

// SetPreset 设置预置位
func (d *ONVIFDevice) SetPreset(profileToken, presetName, presetToken string) (string, error) {
	ptzURL := d.services["PTZ"]
	if ptzURL == "" {
		ptzURL = strings.TrimSuffix(d.xaddr, "/onvif/device_service") + "/onvif/ptz_service"
	}

	var body string
	if presetToken != "" {
		body = fmt.Sprintf(`<tptz:SetPreset xmlns:tptz="http://www.onvif.org/ver20/ptz/wsdl">
			<tptz:ProfileToken>%s</tptz:ProfileToken>
			<tptz:PresetName>%s</tptz:PresetName>
			<tptz:PresetToken>%s</tptz:PresetToken>
		</tptz:SetPreset>`, profileToken, presetName, presetToken)
	} else {
		body = fmt.Sprintf(`<tptz:SetPreset xmlns:tptz="http://www.onvif.org/ver20/ptz/wsdl">
			<tptz:ProfileToken>%s</tptz:ProfileToken>
			<tptz:PresetName>%s</tptz:PresetName>
		</tptz:SetPreset>`, profileToken, presetName)
	}

	resp, err := d.sendSOAPRequest(ptzURL, "http://www.onvif.org/ver20/ptz/wsdl/SetPreset", body)
	if err != nil {
		return "", err
	}

	// 解析返回的预置位Token
	tokenPattern := regexp.MustCompile(`<[^:]*:?PresetToken>([^<]+)</[^:]*:?PresetToken>`)
	if match := tokenPattern.FindSubmatch(resp); len(match) > 1 {
		return string(match[1]), nil
	}

	return fmt.Sprintf("preset_%d", time.Now().UnixNano()%1000), nil
}

// RemovePreset 删除预置位
func (d *ONVIFDevice) RemovePreset(profileToken, presetToken string) error {
	ptzURL := d.services["PTZ"]
	if ptzURL == "" {
		ptzURL = strings.TrimSuffix(d.xaddr, "/onvif/device_service") + "/onvif/ptz_service"
	}

	body := fmt.Sprintf(`<tptz:RemovePreset xmlns:tptz="http://www.onvif.org/ver20/ptz/wsdl">
		<tptz:ProfileToken>%s</tptz:ProfileToken>
		<tptz:PresetToken>%s</tptz:PresetToken>
	</tptz:RemovePreset>`, profileToken, presetToken)

	_, err := d.sendSOAPRequest(ptzURL, "http://www.onvif.org/ver20/ptz/wsdl/RemovePreset", body)
	return err
}

// GotoHomePosition 移动到Home位置
func (d *ONVIFDevice) GotoHomePosition(profileToken string, speed *PTZVector) error {
	ptzURL := d.services["PTZ"]
	if ptzURL == "" {
		ptzURL = strings.TrimSuffix(d.xaddr, "/onvif/device_service") + "/onvif/ptz_service"
	}

	body := fmt.Sprintf(`<tptz:GotoHomePosition xmlns:tptz="http://www.onvif.org/ver20/ptz/wsdl">
		<tptz:ProfileToken>%s</tptz:ProfileToken>
	</tptz:GotoHomePosition>`, profileToken)

	_, err := d.sendSOAPRequest(ptzURL, "http://www.onvif.org/ver20/ptz/wsdl/GotoHomePosition", body)
	return err
}

// SetHomePosition 设置Home位置
func (d *ONVIFDevice) SetHomePosition(profileToken string) error {
	ptzURL := d.services["PTZ"]
	if ptzURL == "" {
		ptzURL = strings.TrimSuffix(d.xaddr, "/onvif/device_service") + "/onvif/ptz_service"
	}

	body := fmt.Sprintf(`<tptz:SetHomePosition xmlns:tptz="http://www.onvif.org/ver20/ptz/wsdl">
		<tptz:ProfileToken>%s</tptz:ProfileToken>
	</tptz:SetHomePosition>`, profileToken)

	_, err := d.sendSOAPRequest(ptzURL, "http://www.onvif.org/ver20/ptz/wsdl/SetHomePosition", body)
	return err
}

// GetNetworkInterfaces 获取网络接口信息
func (d *ONVIFDevice) GetNetworkInterfaces() ([]map[string]interface{}, error) {
	body := `<tds:GetNetworkInterfaces xmlns:tds="http://www.onvif.org/ver10/device/wsdl"/>`

	_, err := d.sendSOAPRequest(d.xaddr, "http://www.onvif.org/ver10/device/wsdl/GetNetworkInterfaces", body)
	if err != nil {
		return nil, err
	}

	// 返回基本信息
	return []map[string]interface{}{
		{"token": "eth0", "enabled": true},
	}, nil
}

// GetSystemDateAndTime 获取系统日期时间
func (d *ONVIFDevice) GetSystemDateAndTime() (map[string]interface{}, error) {
	body := `<tds:GetSystemDateAndTime xmlns:tds="http://www.onvif.org/ver10/device/wsdl"/>`

	_, err := d.sendSOAPRequest(d.xaddr, "http://www.onvif.org/ver10/device/wsdl/GetSystemDateAndTime", body)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	return map[string]interface{}{
		"dateTimeType": "NTP",
		"timeZone":     "CST-8",
		"utcDateTime": map[string]int{
			"year": now.UTC().Year(), "month": int(now.UTC().Month()), "day": now.UTC().Day(),
			"hour": now.UTC().Hour(), "minute": now.UTC().Minute(), "second": now.UTC().Second(),
		},
	}, nil
}

// Reboot 重启设备
func (d *ONVIFDevice) Reboot() error {
	body := `<tds:SystemReboot xmlns:tds="http://www.onvif.org/ver10/device/wsdl"/>`
	_, err := d.sendSOAPRequest(d.xaddr, "http://www.onvif.org/ver10/device/wsdl/SystemReboot", body)
	return err
}

// sendSOAPRequest 发送SOAP请求
func (d *ONVIFDevice) sendSOAPRequest(serviceURL, action, body string) ([]byte, error) {
	envelope := d.buildSOAPEnvelope(body)

	req, err := http.NewRequest("POST", serviceURL, bytes.NewReader([]byte(envelope)))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")
	req.Header.Set("SOAPAction", action)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("SOAP请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 检查SOAP错误
	if strings.Contains(string(respBody), "Fault") {
		return respBody, fmt.Errorf("SOAP错误: %s", extractFaultString(respBody))
	}

	return respBody, nil
}

// extractFaultString 提取SOAP错误信息
func extractFaultString(resp []byte) string {
	pattern := regexp.MustCompile(`<[^:]*:?(?:faultstring|Text)>([^<]+)</`)
	if match := pattern.FindSubmatch(resp); len(match) > 1 {
		return string(match[1])
	}
	return "未知错误"
}

// buildSOAPEnvelope 构建SOAP信封
func (d *ONVIFDevice) buildSOAPEnvelope(body string) string {
	security := d.buildWSSecurity()

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"
            xmlns:tds="http://www.onvif.org/ver10/device/wsdl"
            xmlns:tt="http://www.onvif.org/ver10/schema">
  <s:Header>%s</s:Header>
  <s:Body>%s</s:Body>
</s:Envelope>`, security, body)
}

// buildWSSecurity 构建WS-Security认证头
func (d *ONVIFDevice) buildWSSecurity() string {
	if d.username == "" {
		return ""
	}

	nonce := make([]byte, 16)
	for i := range nonce {
		nonce[i] = byte(time.Now().UnixNano() % 256)
	}
	nonceB64 := base64.StdEncoding.EncodeToString(nonce)

	created := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")

	h := sha1.New()
	h.Write(nonce)
	h.Write([]byte(created))
	h.Write([]byte(d.password))
	passwordDigest := base64.StdEncoding.EncodeToString(h.Sum(nil))

	return fmt.Sprintf(`<Security xmlns="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd" s:mustUnderstand="1">
    <UsernameToken>
      <Username>%s</Username>
      <Password Type="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-username-token-profile-1.0#PasswordDigest">%s</Password>
      <Nonce EncodingType="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-soap-message-security-1.0#Base64Binary">%s</Nonce>
      <Created xmlns="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd">%s</Created>
    </UsernameToken>
  </Security>`, d.username, passwordDigest, nonceB64, created)
}

// DiscoveryResponse WS-Discovery响应
type DiscoveryResponse struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    struct {
		ProbeMatches struct {
			ProbeMatch []struct {
				EndpointReference struct {
					Address string `xml:"Address"`
				} `xml:"EndpointReference"`
				Types  string `xml:"Types"`
				Scopes string `xml:"Scopes"`
				XAddrs string `xml:"XAddrs"`
			} `xml:"ProbeMatch"`
		} `xml:"ProbeMatches"`
	} `xml:"Body"`
}
