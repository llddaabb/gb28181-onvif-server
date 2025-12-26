// internal/onvif/soap_client.go
package onvif

import (
	"bytes"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// SOAPClient 纯SOAP实现的ONVIF客户端
type SOAPClient struct {
	username   string
	password   string
	endpoint   string
	httpClient *http.Client
	mediaAddr  string // 媒体服务地址
	ptzAddr    string // PTZ服务地址
}

// NewSOAPClient 创建新的SOAP客户端
func NewSOAPClient(endpoint, username, password string) *SOAPClient {
	// 创建 HTTP Transport
	tr := &http.Transport{
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     90 * time.Second,
	}

	client := &SOAPClient{
		username: username,
		password: password,
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout:   10 * time.Second, // PTZ控制需要更快响应
			Transport: tr,
		},
	}

	return client
}

// GetPTZAddr 获取PTZ服务地址
func (c *SOAPClient) GetPTZAddr() string {
	return c.ptzAddr
}

// GetMediaAddr 获取媒体服务地址
func (c *SOAPClient) GetMediaAddr() string {
	return c.mediaAddr
}

// ============================================================================
// WSSE 认证相关
// ============================================================================

// truncateString 截断字符串以避免日志过长
func truncateString(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + fmt.Sprintf("...(总长: %d字符)", len(s))
	}
	return s
}

// generateNonce 生成随机nonce（未编码的字节，返回时base64编码）
func generateNonce() (string, []byte) {
	b := make([]byte, 16)
	for i := range b {
		b[i] = byte(rand.Intn(256))
	}
	encoded := base64.StdEncoding.EncodeToString(b)
	return encoded, b
}

// generateWSSEHeader 生成WS-Security认证头
func (c *SOAPClient) generateWSSEHeader() string {
	if c.username == "" || c.password == "" {
		return ""
	}

	nonce, nonceBytes := generateNonce()
	created := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")

	// 计算密码摘要: SHA1(nonce_bytes + created + password)
	h := sha1.New()
	h.Write(nonceBytes)
	h.Write([]byte(created))
	h.Write([]byte(c.password))
	hash := h.Sum(nil)
	passwordDigest := base64.StdEncoding.EncodeToString(hash)

	// 构建WSSE头（手动添加缩进以匹配脚本格式）
	// 第一行前的缩进由外层 fmt.Sprintf 的 "    %s" 提供（4个空格）
	// 后续行需要手动添加完整缩进
	return fmt.Sprintf(`<Security s:mustUnderstand="1" xmlns="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd">
      <UsernameToken>
        <Username>%s</Username>
        <Password Type="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-username-token-profile-1.0#PasswordDigest">%s</Password>
        <Nonce EncodingType="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-soap-message-security-1.0#Base64Binary">%s</Nonce>
        <Created xmlns="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd">%s</Created>
      </UsernameToken>
    </Security>`, c.username, passwordDigest, nonce, created)
}

// callSOAP 调用SOAP方法（支持WSSE认证）
func (c *SOAPClient) callSOAP(action, body string) (string, error) {
	return c.callSOAPOnEndpoint(c.endpoint, action, body)
}

// callSOAPOnEndpoint 在指定端点调用SOAP方法
func (c *SOAPClient) callSOAPOnEndpoint(endpoint, action, body string) (string, error) {
	if endpoint == "" {
		endpoint = c.endpoint
	}

	// 构建SOAP信封（使用脚本兼容的格式）
	securityHeader := c.generateWSSEHeader()

	// 如果有安全头，添加到信封中；否则不包含 Header 部分或添加空 Header
	var soapEnvelope string
	if securityHeader != "" {
		soapEnvelope = fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope">
  <s:Header>
    %s
  </s:Header>
  <s:Body>
    %s
  </s:Body>
</s:Envelope>`, securityHeader, body)
	} else {
		// 如果没有安全头，仍然包含 Header 但留空
		soapEnvelope = fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope">
  <s:Header/>
  <s:Body>
    %s
  </s:Body>
</s:Envelope>`, body)
	}

	// 发送SOAP请求
	req, err := http.NewRequest("POST", endpoint, bytes.NewBufferString(soapEnvelope))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")
	req.Header.Set("SOAPAction", action)

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	respStr := string(respBody)

	// 检查HTTP状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// 尝试提取错误信息
		type SoapFault struct {
			Code   string
			Reason string
		}
		var fault SoapFault

		// 尝试解析 SOAP Fault
		decoder := xml.NewDecoder(strings.NewReader(respStr))
		for {
			token, err := decoder.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				break
			}

			if se, ok := token.(xml.StartElement); ok {
				if se.Name.Local == "Code" {
					var code string
					decoder.DecodeElement(&code, &se)
					fault.Code = code
				} else if se.Name.Local == "Reason" {
					var reason string
					decoder.DecodeElement(&reason, &se)
					fault.Reason = reason
				}
			}
		}

		// 构建错误消息
		errMsg := fmt.Sprintf("HTTP %d", resp.StatusCode)
		if fault.Code != "" {
			errMsg += fmt.Sprintf(" | Code: %s", fault.Code)
		}
		if fault.Reason != "" {
			errMsg += fmt.Sprintf(" | Reason: %s", fault.Reason)
		}

		// 对于 4xx 错误，保存请求和响应到临时文件（用于调试）
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			os.WriteFile("/tmp/go_soap_request.xml", []byte(soapEnvelope), 0644)
			os.WriteFile("/tmp/go_soap_response.xml", respBody, 0644)
		}

		return respStr, fmt.Errorf("%s", errMsg)
	}

	return respStr, nil
}

// ============================================================================
// 设备信息方法
// ============================================================================

// GetDeviceInformation 获取设备信息
func (c *SOAPClient) GetDeviceInformation() (map[string]string, error) {
	body := `<GetDeviceInformation xmlns="http://www.onvif.org/ver10/device/wsdl"/>`

	resp, err := c.callSOAP("http://www.onvif.org/ver10/device/wsdl/GetDeviceInformation", body)
	if err != nil {
		return nil, err
	}

	info := make(map[string]string)

	// 解析XML响应
	decoder := xml.NewDecoder(strings.NewReader(resp))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		if se, ok := token.(xml.StartElement); ok {
			var value string
			decoder.DecodeElement(&value, &se)

			localName := se.Name.Local
			switch localName {
			case "Manufacturer":
				info["Manufacturer"] = value
			case "Model":
				info["Model"] = value
			case "FirmwareVersion":
				info["FirmwareVersion"] = value
			case "SerialNumber":
				info["SerialNumber"] = value
			case "HardwareId":
				info["HardwareId"] = value
			}
		}
	}

	return info, nil
}

// GetSystemDateAndTime 获取系统时间（用于测试连接）
func (c *SOAPClient) GetSystemDateAndTime() (time.Time, error) {
	body := `<GetSystemDateAndTime xmlns="http://www.onvif.org/ver10/device/wsdl"/>`

	resp, err := c.callSOAP("http://www.onvif.org/ver10/device/wsdl/GetSystemDateAndTime", body)
	if err != nil {
		return time.Time{}, err
	}

	// 解析时间值
	var dateTime struct {
		Year   int `xml:"Year"`
		Month  int `xml:"Month"`
		Day    int `xml:"Day"`
		Hour   int `xml:"Hour"`
		Minute int `xml:"Minute"`
		Second int `xml:"Second"`
	}

	decoder := xml.NewDecoder(strings.NewReader(resp))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		if se, ok := token.(xml.StartElement); ok {
			if se.Name.Local == "UTCDateTime" {
				decoder.DecodeElement(&dateTime, &se)
				break
			}
		}
	}

	t := time.Date(dateTime.Year, time.Month(dateTime.Month), dateTime.Day,
		dateTime.Hour, dateTime.Minute, dateTime.Second, 0, time.UTC)
	return t, nil
}

// GetCapabilities 获取设备能力
func (c *SOAPClient) GetCapabilities() (map[string]interface{}, error) {
	body := `<GetCapabilities xmlns="http://www.onvif.org/ver10/device/wsdl">
    <Category>All</Category>
  </GetCapabilities>`

	resp, err := c.callSOAP("http://www.onvif.org/ver10/device/wsdl/GetCapabilities", body)
	if err != nil {
		return nil, err
	}

	caps := make(map[string]interface{})

	// 解析媒体/PTZ服务地址，准确读取嵌套的 XAddr 字段
	decoder := xml.NewDecoder(strings.NewReader(resp))
	currentSection := ""
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "Media":
				currentSection = "Media"
			case "PTZ":
				currentSection = "PTZ"
			case "XAddr":
				var xaddr string
				if err := decoder.DecodeElement(&xaddr, &t); err == nil {
					if xaddr != "" {
						if currentSection == "Media" {
							c.mediaAddr = xaddr
							caps["MediaAddr"] = xaddr
						} else if currentSection == "PTZ" {
							c.ptzAddr = xaddr
							caps["PTZAddr"] = xaddr
						}
					}
				}
			}
		case xml.EndElement:
			if t.Name.Local == "Media" || t.Name.Local == "PTZ" {
				currentSection = ""
			}
		}
	}

	return caps, nil
}

// ============================================================================
// 媒体配置方法
// ============================================================================

// GetMediaProfiles 获取媒体配置文件（带重试机制）
func (c *SOAPClient) GetMediaProfiles() ([]MediaProfile, error) {
	const maxRetries = 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		profiles, err := c.getMediaProfilesAttempt(attempt)
		if err == nil {
			return profiles, nil
		}
		lastErr = err

		// 如果不是最后一次尝试，等待后重试
		if attempt < maxRetries {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}

	return nil, lastErr
}

// getMediaProfilesAttempt 单次尝试获取媒体配置文件
func (c *SOAPClient) getMediaProfilesAttempt(attempt int) ([]MediaProfile, error) {
	// 首先尝试从媒体服务地址获取
	if c.mediaAddr == "" {
		c.GetCapabilities()
	}

	endpoint := c.mediaAddr
	if endpoint == "" {
		endpoint = c.endpoint
	}

	// 使用带命名空间前缀的 GetProfiles 请求格式（标准 ONVIF 格式）
	body := `<trt:GetProfiles xmlns:trt="http://www.onvif.org/ver10/media/wsdl"/>`

	// 先尝试带 WSSE 认证的请求
	resp, err := c.callSOAPOnEndpoint(endpoint, "http://www.onvif.org/ver10/media/wsdl/GetProfiles", body)

	// 如果返回 HTTP 400 并且有 WSSE 认证，尝试不带认证的请求
	if err != nil && strings.Contains(err.Error(), "HTTP 400") && c.username != "" {
		// 临时清空凭据，重新发送
		originalUsername := c.username
		originalPassword := c.password
		c.username = ""
		c.password = ""

		resp, err = c.callSOAPOnEndpoint(endpoint, "http://www.onvif.org/ver10/media/wsdl/GetProfiles", body)

		// 恢复凭据
		c.username = originalUsername
		c.password = originalPassword
	}

	if err != nil {
		return nil, err
	}

	// 保存响应用于调试
	os.WriteFile("/tmp/getprofiles_response.xml", []byte(resp), 0644)

	var profiles []MediaProfile

	// 使用正则表达式解析 Profiles（更稳定）
	// 匹配单个 Profile 块
	profileRegex := regexp.MustCompile(`(?s)<(?:trt:)?Profiles[^>]*token="([^"]*)"[^>]*>.*?<(?:tt:)?Name>([^<]*)</(?:tt:)?Name>.*?</(?:trt:)?Profiles>`)
	matches := profileRegex.FindAllStringSubmatch(resp, -1)

	for _, match := range matches {
		if len(match) >= 3 {
			profile := MediaProfile{
				Token: match[1],
				Name:  match[2],
			}

			// 从当前 Profile 块中提取更多信息
			profileBlock := match[0]

			// 提取编码格式
			encodingRegex := regexp.MustCompile(`<(?:tt:)?Encoding>([^<]*)</(?:tt:)?Encoding>`)
			if em := encodingRegex.FindStringSubmatch(profileBlock); len(em) >= 2 {
				profile.Encoding = em[1]
			}

			// 提取分辨率
			widthRegex := regexp.MustCompile(`<(?:tt:)?Width>(\d+)</(?:tt:)?Width>`)
			heightRegex := regexp.MustCompile(`<(?:tt:)?Height>(\d+)</(?:tt:)?Height>`)
			if wm := widthRegex.FindStringSubmatch(profileBlock); len(wm) >= 2 {
				if w, err := strconv.Atoi(wm[1]); err == nil {
					profile.Width = w
				}
			}
			if hm := heightRegex.FindStringSubmatch(profileBlock); len(hm) >= 2 {
				if h, err := strconv.Atoi(hm[1]); err == nil {
					profile.Height = h
				}
			}
			if profile.Width > 0 && profile.Height > 0 {
				profile.Resolution = fmt.Sprintf("%dx%d", profile.Width, profile.Height)
			}

			// 提取帧率
			fpsRegex := regexp.MustCompile(`<(?:tt:)?FrameRateLimit>(\d+)</(?:tt:)?FrameRateLimit>`)
			if fm := fpsRegex.FindStringSubmatch(profileBlock); len(fm) >= 2 {
				if fps, err := strconv.Atoi(fm[1]); err == nil {
					profile.FPS = fps
				}
			}

			// 提取码率
			bitrateRegex := regexp.MustCompile(`<(?:tt:)?BitrateLimit>(\d+)</(?:tt:)?BitrateLimit>`)
			if bm := bitrateRegex.FindStringSubmatch(profileBlock); len(bm) >= 2 {
				if br, err := strconv.Atoi(bm[1]); err == nil {
					profile.Bitrate = br
				}
			}

			profiles = append(profiles, profile)
		}
	}

	if len(profiles) == 0 {
		// 尝试备用正则（不同命名空间格式）
		altRegex := regexp.MustCompile(`(?s)Profiles\s+token="([^"]*)"[^>]*>.*?<Name>([^<]*)</Name>`)
		altMatches := altRegex.FindAllStringSubmatch(resp, -1)
		for _, match := range altMatches {
			if len(match) >= 3 {
				profile := MediaProfile{
					Token: match[1],
					Name:  match[2],
				}
				profiles = append(profiles, profile)
			}
		}
	}

	if len(profiles) == 0 {
		return nil, fmt.Errorf("no profiles found")
	}

	return profiles, nil
}

// GetStreamURI 获取流地址
func (c *SOAPClient) GetStreamURI(profileToken string) (string, error) {
	if c.mediaAddr == "" {
		c.GetCapabilities()
	}

	endpoint := c.mediaAddr
	if endpoint == "" {
		endpoint = c.endpoint
	}

	// 使用标准 ONVIF GetStreamUri 格式
	body := fmt.Sprintf(`<trt:GetStreamUri xmlns:trt="http://www.onvif.org/ver10/media/wsdl" xmlns:tt="http://www.onvif.org/ver10/schema">
    <trt:StreamSetup>
      <tt:Stream>RTP-Unicast</tt:Stream>
      <tt:Transport>
        <tt:Protocol>RTSP</tt:Protocol>
      </tt:Transport>
    </trt:StreamSetup>
    <trt:ProfileToken>%s</trt:ProfileToken>
  </trt:GetStreamUri>`, profileToken)

	resp, err := c.callSOAPOnEndpoint(endpoint, "http://www.onvif.org/ver10/media/wsdl/GetStreamUri", body)
	if err != nil {
		return "", err
	}

	// 保存响应用于调试
	os.WriteFile("/tmp/getstreamuri_response.xml", []byte(resp), 0644)

	// 解析流地址
	decoder := xml.NewDecoder(strings.NewReader(resp))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		if se, ok := token.(xml.StartElement); ok {
			if se.Name.Local == "Uri" {
				var uri string
				decoder.DecodeElement(&uri, &se)
				if uri != "" {
					return uri, nil
				}
			}
		}
	}

	return "", fmt.Errorf("no stream URI found in response")
}

// GetSnapshotURI 获取快照地址
func (c *SOAPClient) GetSnapshotURI(profileToken string) (string, error) {
	if c.mediaAddr == "" {
		c.GetCapabilities()
	}

	endpoint := c.mediaAddr
	if endpoint == "" {
		endpoint = c.endpoint
	}

	body := fmt.Sprintf(`<trt:GetSnapshotUri xmlns:trt="http://www.onvif.org/ver10/media/wsdl">
    <trt:ProfileToken>%s</trt:ProfileToken>
  </trt:GetSnapshotUri>`, profileToken)

	resp, err := c.callSOAPOnEndpoint(endpoint, "http://www.onvif.org/ver10/media/wsdl/GetSnapshotUri", body)
	if err != nil {
		return "", err
	}

	// 解析快照地址
	decoder := xml.NewDecoder(strings.NewReader(resp))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		if se, ok := token.(xml.StartElement); ok {
			if se.Name.Local == "Uri" {
				var uri string
				decoder.DecodeElement(&uri, &se)
				if uri != "" {
					return uri, nil
				}
			}
		}
	}

	return "", fmt.Errorf("no snapshot URI found")
}

// ============================================================================
// PTZ控制方法
// ============================================================================

// ContinuousMove PTZ连续移动
func (c *SOAPClient) ContinuousMove(profileToken string, x, y, z float64, timeout float64) error {
	// 优化：不再每次都检查 ptzAddr，直接使用可用的端点
	endpoint := c.ptzAddr
	if endpoint == "" {
		endpoint = c.endpoint
	}

	body := fmt.Sprintf(`<ContinuousMove xmlns="http://www.onvif.org/ver20/ptz/wsdl">
    <ProfileToken>%s</ProfileToken>
    <Velocity>
      <PanTilt x="%.2f" y="%.2f" xmlns="http://www.onvif.org/ver10/schema"/>
      <Zoom x="%.2f" xmlns="http://www.onvif.org/ver10/schema"/>
    </Velocity>
    <Timeout>PT%.1fS</Timeout>
  </ContinuousMove>`, profileToken, x, y, z, timeout)

	_, err := c.callSOAPOnEndpoint(endpoint, "http://www.onvif.org/ver20/ptz/wsdl/ContinuousMove", body)
	return err
}

// StopPTZ 停止PTZ
func (c *SOAPClient) StopPTZ(profileToken string) error {
	// 优化：不再每次都检查 ptzAddr，直接使用可用的端点
	endpoint := c.ptzAddr
	if endpoint == "" {
		endpoint = c.endpoint
	}

	body := fmt.Sprintf(`<Stop xmlns="http://www.onvif.org/ver20/ptz/wsdl">
    <ProfileToken>%s</ProfileToken>
    <PanTilt>true</PanTilt>
    <Zoom>true</Zoom>
  </Stop>`, profileToken)

	_, err := c.callSOAPOnEndpoint(endpoint, "http://www.onvif.org/ver20/ptz/wsdl/Stop", body)
	return err
}

// GotoPreset 移动到预置位
func (c *SOAPClient) GotoPreset(profileToken, presetToken string) error {
	// 优化：不再每次都检查 ptzAddr，直接使用可用的端点
	endpoint := c.ptzAddr
	if endpoint == "" {
		endpoint = c.endpoint
	}

	body := fmt.Sprintf(`<tptz:GotoPreset xmlns:tptz="http://www.onvif.org/ver20/ptz/wsdl">
    <tptz:ProfileToken>%s</tptz:ProfileToken>
    <tptz:PresetToken>%s</tptz:PresetToken>
  </tptz:GotoPreset>`, profileToken, presetToken)

	_, err := c.callSOAPOnEndpoint(endpoint, "http://www.onvif.org/ver20/ptz/wsdl/GotoPreset", body)
	return err
}

// SetPreset 设置预置位
func (c *SOAPClient) SetPreset(profileToken, presetName, presetToken string) (string, error) {
	// 优化：不再每次都检查 ptzAddr，直接使用可用的端点
	endpoint := c.ptzAddr
	if endpoint == "" {
		endpoint = c.endpoint
	}

	body := fmt.Sprintf(`<tptz:SetPreset xmlns:tptz="http://www.onvif.org/ver20/ptz/wsdl">
    <tptz:ProfileToken>%s</tptz:ProfileToken>
    <tptz:PresetName>%s</tptz:PresetName>
    <tptz:PresetToken>%s</tptz:PresetToken>
  </tptz:SetPreset>`, profileToken, presetName, presetToken)

	resp, err := c.callSOAPOnEndpoint(endpoint, "http://www.onvif.org/ver20/ptz/wsdl/SetPreset", body)
	if err != nil {
		return "", err
	}

	// 解析返回的preset token
	decoder := xml.NewDecoder(strings.NewReader(resp))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		if se, ok := token.(xml.StartElement); ok {
			if se.Name.Local == "PresetToken" {
				var token string
				decoder.DecodeElement(&token, &se)
				return token, nil
			}
		}
	}

	return "", nil
}

// RemovePreset 删除预置位
func (c *SOAPClient) RemovePreset(profileToken, presetToken string) error {
	// 优化：不再每次都检查 ptzAddr，直接使用可用的端点
	endpoint := c.ptzAddr
	if endpoint == "" {
		endpoint = c.endpoint
	}

	body := fmt.Sprintf(`<tptz:RemovePreset xmlns:tptz="http://www.onvif.org/ver20/ptz/wsdl">
    <tptz:ProfileToken>%s</tptz:ProfileToken>
    <tptz:PresetToken>%s</tptz:PresetToken>
  </tptz:RemovePreset>`, profileToken, presetToken)

	_, err := c.callSOAPOnEndpoint(endpoint, "http://www.onvif.org/ver20/ptz/wsdl/RemovePreset", body)
	return err
}

// GetPresets 获取预置位列表
func (c *SOAPClient) GetPresets(profileToken string) ([]PTZPreset, error) {
	// 优化：不再每次都检查 ptzAddr，直接使用可用的端点
	endpoint := c.ptzAddr
	if endpoint == "" {
		endpoint = c.endpoint
	}

	body := fmt.Sprintf(`<tptz:GetPresets xmlns:tptz="http://www.onvif.org/ver20/ptz/wsdl">
    <tptz:ProfileToken>%s</tptz:ProfileToken>
  </tptz:GetPresets>`, profileToken)

	resp, err := c.callSOAPOnEndpoint(endpoint, "http://www.onvif.org/ver20/ptz/wsdl/GetPresets", body)
	if err != nil {
		return nil, err
	}

	var presets []PTZPreset

	// 解析预置位列表
	decoder := xml.NewDecoder(strings.NewReader(resp))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		if se, ok := token.(xml.StartElement); ok {
			if se.Name.Local == "Preset" {
				var preset struct {
					Token string `xml:"token,attr"`
					Name  string `xml:"Name"`
				}
				decoder.DecodeElement(&preset, &se)
				if preset.Token != "" {
					presets = append(presets, PTZPreset{
						Token: preset.Token,
						Name:  preset.Name,
					})
				}
			}
		}
	}

	return presets, nil
}

// TestConnection 测试连接是否正常
func (c *SOAPClient) TestConnection() error {
	_, err := c.GetSystemDateAndTime()
	return err
}

// ============================================================================
// Recording Search 录像搜索相关方法
// ============================================================================

// RecordingSearchItem ONVIF 录像搜索结果项
type RecordingSearchItem struct {
	RecordingToken string    `json:"recordingToken"`
	Name           string    `json:"name"`
	Source         string    `json:"source"`
	Content        string    `json:"content"`
	Track          string    `json:"track"`
	StartTime      time.Time `json:"startTime"`
	EndTime        time.Time `json:"endTime"`
}

// GetRecordingSearchResults 获取录像搜索结果
type GetRecordingSearchResults struct {
	SearchToken string                `json:"searchToken"`
	ResultList  []RecordingSearchItem `json:"resultList"`
	SearchState string                `json:"searchState"` // Searching, Completed, Unknown
}

// FindRecordings 查找设备上的录像
// 参数: startTime, endTime 为 ISO 8601 格式
func (c *SOAPClient) FindRecordings(startTime, endTime string) (*GetRecordingSearchResults, error) {
	// 首先获取 Recording Search 服务地址
	searchAddr, err := c.getRecordingSearchServiceAddress()
	if err != nil {
		return nil, fmt.Errorf("获取录像搜索服务地址失败: %w", err)
	}

	// 构建 FindRecordings 请求
	body := `<tse:FindRecordings xmlns:tse="http://www.onvif.org/ver10/search/wsdl" xmlns:tt="http://www.onvif.org/ver10/schema">
    <tse:Scope>
      <tt:IncludedSources/>
      <tt:IncludedRecordings/>
      <tt:RecordingInformationFilter/>
    </tse:Scope>
    <tse:MaxMatches>100</tse:MaxMatches>
    <tse:KeepAliveTime>PT60S</tse:KeepAliveTime>
  </tse:FindRecordings>`

	resp, err := c.callSOAPOnEndpoint(searchAddr, "http://www.onvif.org/ver10/search/wsdl/FindRecordings", body)
	if err != nil {
		return nil, fmt.Errorf("FindRecordings 请求失败: %w", err)
	}

	// 解析 SearchToken
	searchToken := extractXMLValue(resp, "SearchToken")
	if searchToken == "" {
		return nil, fmt.Errorf("未获取到 SearchToken")
	}

	// 获取搜索结果
	return c.getRecordingSearchResults(searchAddr, searchToken)
}

// getRecordingSearchServiceAddress 获取录像搜索服务地址
func (c *SOAPClient) getRecordingSearchServiceAddress() (string, error) {
	// 先尝试从 capabilities 获取
	body := `<GetCapabilities xmlns="http://www.onvif.org/ver10/device/wsdl">
    <Category>All</Category>
  </GetCapabilities>`

	resp, err := c.callSOAP("http://www.onvif.org/ver10/device/wsdl/GetCapabilities", body)
	if err != nil {
		return "", err
	}

	// 解析 Search 服务地址
	decoder := xml.NewDecoder(strings.NewReader(resp))
	inSearch := false
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "Search" {
				inSearch = true
			} else if t.Name.Local == "XAddr" && inSearch {
				var xaddr string
				if err := decoder.DecodeElement(&xaddr, &t); err == nil && xaddr != "" {
					return xaddr, nil
				}
			}
		case xml.EndElement:
			if t.Name.Local == "Search" {
				inSearch = false
			}
		}
	}

	// 如果没有找到，尝试使用默认的 search 服务路径
	u, err := url.Parse(c.endpoint)
	if err != nil {
		return "", fmt.Errorf("解析端点地址失败: %w", err)
	}
	u.Path = "/onvif/search_service"
	return u.String(), nil
}

// getRecordingSearchResults 获取录像搜索结果
func (c *SOAPClient) getRecordingSearchResults(searchAddr, searchToken string) (*GetRecordingSearchResults, error) {
	body := fmt.Sprintf(`<tse:GetRecordingSearchResults xmlns:tse="http://www.onvif.org/ver10/search/wsdl">
    <tse:SearchToken>%s</tse:SearchToken>
    <tse:MinResults>1</tse:MinResults>
    <tse:MaxResults>100</tse:MaxResults>
    <tse:WaitTime>PT5S</tse:WaitTime>
  </tse:GetRecordingSearchResults>`, searchToken)

	resp, err := c.callSOAPOnEndpoint(searchAddr, "http://www.onvif.org/ver10/search/wsdl/GetRecordingSearchResults", body)
	if err != nil {
		return nil, fmt.Errorf("GetRecordingSearchResults 请求失败: %w", err)
	}

	result := &GetRecordingSearchResults{
		SearchToken: searchToken,
		ResultList:  make([]RecordingSearchItem, 0),
	}

	// 解析搜索状态
	result.SearchState = extractXMLValue(resp, "SearchState")

	// 解析录像列表
	decoder := xml.NewDecoder(strings.NewReader(resp))
	var currentItem *RecordingSearchItem
	inRecordingInformation := false

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "RecordingInformation":
				inRecordingInformation = true
				currentItem = &RecordingSearchItem{}
				// 获取 RecordingToken 属性
				for _, attr := range t.Attr {
					if attr.Name.Local == "token" {
						currentItem.RecordingToken = attr.Value
					}
				}
			case "RecordingToken":
				if inRecordingInformation && currentItem != nil {
					var token string
					decoder.DecodeElement(&token, &t)
					currentItem.RecordingToken = token
				}
			case "Name":
				if inRecordingInformation && currentItem != nil {
					var name string
					decoder.DecodeElement(&name, &t)
					currentItem.Name = name
				}
			case "Source":
				if inRecordingInformation && currentItem != nil {
					var source string
					decoder.DecodeElement(&source, &t)
					currentItem.Source = source
				}
			case "Content":
				if inRecordingInformation && currentItem != nil {
					var content string
					decoder.DecodeElement(&content, &t)
					currentItem.Content = content
				}
			case "EarliestRecording", "StartTime":
				if inRecordingInformation && currentItem != nil {
					var timeStr string
					decoder.DecodeElement(&timeStr, &t)
					if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
						currentItem.StartTime = t
					}
				}
			case "LatestRecording", "EndTime":
				if inRecordingInformation && currentItem != nil {
					var timeStr string
					decoder.DecodeElement(&timeStr, &t)
					if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
						currentItem.EndTime = t
					}
				}
			}
		case xml.EndElement:
			if t.Name.Local == "RecordingInformation" {
				inRecordingInformation = false
				if currentItem != nil && currentItem.RecordingToken != "" {
					result.ResultList = append(result.ResultList, *currentItem)
				}
				currentItem = nil
			}
		}
	}

	return result, nil
}

// GetRecordings 获取设备上的所有录像 (Recording Control 服务)
func (c *SOAPClient) GetRecordings() ([]RecordingSearchItem, error) {
	// 获取 Recording Control 服务地址
	recordingAddr, err := c.getRecordingControlServiceAddress()
	if err != nil {
		// 如果获取失败，尝试使用 Search 服务
		return c.getRecordingsViaSearch()
	}

	body := `<trc:GetRecordings xmlns:trc="http://www.onvif.org/ver10/recording/wsdl"/>`

	resp, err := c.callSOAPOnEndpoint(recordingAddr, "http://www.onvif.org/ver10/recording/wsdl/GetRecordings", body)
	if err != nil {
		// 如果失败，尝试使用 Search 服务
		return c.getRecordingsViaSearch()
	}

	var recordings []RecordingSearchItem

	// 解析录像列表
	decoder := xml.NewDecoder(strings.NewReader(resp))
	var currentItem *RecordingSearchItem
	inRecordingItem := false

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "RecordingItem":
				inRecordingItem = true
				currentItem = &RecordingSearchItem{}
			case "RecordingToken":
				if inRecordingItem && currentItem != nil {
					var token string
					decoder.DecodeElement(&token, &t)
					currentItem.RecordingToken = token
				}
			case "Name":
				if inRecordingItem && currentItem != nil {
					var name string
					decoder.DecodeElement(&name, &t)
					currentItem.Name = name
				}
			case "Content":
				if inRecordingItem && currentItem != nil {
					var content string
					decoder.DecodeElement(&content, &t)
					currentItem.Content = content
				}
			}
		case xml.EndElement:
			if t.Name.Local == "RecordingItem" {
				inRecordingItem = false
				if currentItem != nil && currentItem.RecordingToken != "" {
					recordings = append(recordings, *currentItem)
				}
				currentItem = nil
			}
		}
	}

	return recordings, nil
}

// getRecordingsViaSearch 通过 Search 服务获取录像
func (c *SOAPClient) getRecordingsViaSearch() ([]RecordingSearchItem, error) {
	result, err := c.FindRecordings("", "")
	if err != nil {
		return nil, err
	}
	return result.ResultList, nil
}

// getRecordingControlServiceAddress 获取录像控制服务地址
func (c *SOAPClient) getRecordingControlServiceAddress() (string, error) {
	body := `<GetCapabilities xmlns="http://www.onvif.org/ver10/device/wsdl">
    <Category>All</Category>
  </GetCapabilities>`

	resp, err := c.callSOAP("http://www.onvif.org/ver10/device/wsdl/GetCapabilities", body)
	if err != nil {
		return "", err
	}

	// 解析 Recording 服务地址
	decoder := xml.NewDecoder(strings.NewReader(resp))
	inRecording := false
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "Recording" {
				inRecording = true
			} else if t.Name.Local == "XAddr" && inRecording {
				var xaddr string
				if err := decoder.DecodeElement(&xaddr, &t); err == nil && xaddr != "" {
					return xaddr, nil
				}
			}
		case xml.EndElement:
			if t.Name.Local == "Recording" {
				inRecording = false
			}
		}
	}

	return "", fmt.Errorf("设备不支持 Recording 服务")
}

// FindRecordingsByTime 按时间范围查找录像
func (c *SOAPClient) FindRecordingsByTime(startTime, endTime time.Time) ([]RecordingSearchItem, error) {
	searchAddr, err := c.getRecordingSearchServiceAddress()
	if err != nil {
		return nil, fmt.Errorf("获取录像搜索服务地址失败: %w", err)
	}

	// 格式化时间为 ISO 8601
	startTimeStr := startTime.UTC().Format(time.RFC3339)
	endTimeStr := endTime.UTC().Format(time.RFC3339)

	// FindEvents 可能更适合按时间查询
	body := fmt.Sprintf(`<tse:FindEvents xmlns:tse="http://www.onvif.org/ver10/search/wsdl" xmlns:tt="http://www.onvif.org/ver10/schema">
    <tse:StartPoint>%s</tse:StartPoint>
    <tse:EndPoint>%s</tse:EndPoint>
    <tse:Scope>
      <tt:IncludedSources/>
      <tt:IncludedRecordings/>
    </tse:Scope>
    <tse:SearchFilter>
      <tt:TopicExpression Dialect="http://www.onvif.org/ver10/tev/topicExpression/ConcreteSet">
        tns1:RecordingHistory/Track/State
      </tt:TopicExpression>
    </tse:SearchFilter>
    <tse:IncludeStartState>true</tse:IncludeStartState>
    <tse:MaxMatches>100</tse:MaxMatches>
    <tse:KeepAliveTime>PT60S</tse:KeepAliveTime>
  </tse:FindEvents>`, startTimeStr, endTimeStr)

	resp, err := c.callSOAPOnEndpoint(searchAddr, "http://www.onvif.org/ver10/search/wsdl/FindEvents", body)
	if err != nil {
		// FindEvents 失败时尝试 FindRecordings
		result, err := c.FindRecordings(startTimeStr, endTimeStr)
		if err != nil {
			return nil, err
		}
		return result.ResultList, nil
	}

	// 解析 SearchToken
	searchToken := extractXMLValue(resp, "SearchToken")
	if searchToken == "" {
		// 尝试使用 FindRecordings
		result, err := c.FindRecordings(startTimeStr, endTimeStr)
		if err != nil {
			return nil, err
		}
		return result.ResultList, nil
	}

	// 获取事件搜索结果
	result, err := c.getRecordingSearchResults(searchAddr, searchToken)
	if err != nil {
		return nil, err
	}

	// 过滤时间范围内的录像
	var filtered []RecordingSearchItem
	for _, item := range result.ResultList {
		// 如果录像时间范围与查询时间有重叠
		if !item.EndTime.Before(startTime) && !item.StartTime.After(endTime) {
			filtered = append(filtered, item)
		}
	}

	return filtered, nil
}

// GetReplayUri 获取录像回放地址
func (c *SOAPClient) GetReplayUri(recordingToken, streamType string) (string, error) {
	// 获取 Replay 服务地址
	replayAddr, err := c.getReplayServiceAddress()
	if err != nil {
		return "", fmt.Errorf("获取回放服务地址失败: %w", err)
	}

	if streamType == "" {
		streamType = "RTP-Unicast"
	}

	body := fmt.Sprintf(`<trp:GetReplayUri xmlns:trp="http://www.onvif.org/ver10/replay/wsdl" xmlns:tt="http://www.onvif.org/ver10/schema">
    <trp:StreamSetup>
      <tt:Stream>%s</tt:Stream>
      <tt:Transport>
        <tt:Protocol>RTSP</tt:Protocol>
      </tt:Transport>
    </trp:StreamSetup>
    <trp:RecordingToken>%s</trp:RecordingToken>
  </trp:GetReplayUri>`, streamType, recordingToken)

	resp, err := c.callSOAPOnEndpoint(replayAddr, "http://www.onvif.org/ver10/replay/wsdl/GetReplayUri", body)
	if err != nil {
		return "", fmt.Errorf("GetReplayUri 请求失败: %w", err)
	}

	// 解析 Uri
	uri := extractXMLValue(resp, "Uri")
	if uri == "" {
		return "", fmt.Errorf("未获取到回放地址")
	}

	return uri, nil
}

// getReplayServiceAddress 获取回放服务地址
func (c *SOAPClient) getReplayServiceAddress() (string, error) {
	body := `<GetCapabilities xmlns="http://www.onvif.org/ver10/device/wsdl">
    <Category>All</Category>
  </GetCapabilities>`

	resp, err := c.callSOAP("http://www.onvif.org/ver10/device/wsdl/GetCapabilities", body)
	if err != nil {
		return "", err
	}

	// 解析 Replay 服务地址
	decoder := xml.NewDecoder(strings.NewReader(resp))
	inReplay := false
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "Replay" {
				inReplay = true
			} else if t.Name.Local == "XAddr" && inReplay {
				var xaddr string
				if err := decoder.DecodeElement(&xaddr, &t); err == nil && xaddr != "" {
					return xaddr, nil
				}
			}
		case xml.EndElement:
			if t.Name.Local == "Replay" {
				inReplay = false
			}
		}
	}

	// 尝试默认路径
	u, err := url.Parse(c.endpoint)
	if err != nil {
		return "", fmt.Errorf("设备不支持 Replay 服务")
	}
	u.Path = "/onvif/replay_service"
	return u.String(), nil
}

// extractXMLValue 从 XML 中提取指定标签的值
func extractXMLValue(xmlStr, tagName string) string {
	decoder := xml.NewDecoder(strings.NewReader(xmlStr))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		if se, ok := token.(xml.StartElement); ok {
			if se.Name.Local == tagName {
				var value string
				decoder.DecodeElement(&value, &se)
				return value
			}
		}
	}
	return ""
}
