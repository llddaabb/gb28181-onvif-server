// internal/onvif/onvif_client.go
package onvif

import (
	"context"
	"fmt"
	"net/url"

	goonvif "github.com/use-go/onvif"
	"github.com/use-go/onvif/device"
	"github.com/use-go/onvif/media"
	ptz "github.com/use-go/onvif/ptz"
	sdkdevice "github.com/use-go/onvif/sdk/device"
	sdkmedia "github.com/use-go/onvif/sdk/media"
	sdkptz "github.com/use-go/onvif/sdk/ptz"
	"github.com/use-go/onvif/xsd"
	onvifx "github.com/use-go/onvif/xsd/onvif"
)

// VideoEncoderConfig for manager compatibility
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

// MediaProfile 媒体配置
type MediaProfile struct {
	Token        string              `json:"token"`
	Name         string              `json:"name"`
	Encoding     string              `json:"encoding"`
	Resolution   string              `json:"resolution"`
	Width        int                 `json:"width"`
	Height       int                 `json:"height"`
	FPS          int                 `json:"fps"`
	Bitrate      int                 `json:"bitrate"`
	VideoEncoder *VideoEncoderConfig `json:"videoEncoder,omitempty"`
	PTZConfig    *PTZConfig          `json:"ptzConfig,omitempty"`
}

// PTZConfig PTZ配置
type PTZConfig struct {
	Token     string `json:"token"`
	Name      string `json:"name"`
	NodeToken string `json:"nodeToken"`
}

// PTZPreset 预置位
type PTZPreset struct {
	Token string `json:"token"`
	Name  string `json:"name"`
}

// DeviceInformation 设备信息
type DeviceInformation struct {
	Manufacturer    string
	Model           string
	FirmwareVersion string
	SerialNumber    string
	HardwareID      string
	Name            string
}

// DeviceCapabilities 设备能力
type DeviceCapabilities struct {
	HasPTZ bool
	Media  *onvifx.MediaCapabilities
	PTZ    *onvifx.PTZCapabilities
	// 其他能力...
}

// ONVIFClient 客户端抽象接口
type ONVIFClient interface {
	Connect(xaddr string) error
	Close()

	// Device Service
	GetDeviceInformation() (*DeviceInformation, error)
	GetCapabilities() (*DeviceCapabilities, error)

	// Media Service
	GetProfiles() ([]MediaProfile, error)
	GetStreamURI(profileToken string) (string, error)
	GetSnapshotURI(profileToken string) (string, error)
	GetVideoEncoderConfigurations(profileToken string) ([]VideoEncoderConfig, error)

	// PTZ Service
	ContinuousMove(profileToken string, vector *PTZVector) error
	Stop(profileToken string) error
	GetPresets(profileToken string) ([]PTZPreset, error)
	SetPreset(profileToken, presetName, presetToken string) (string, error)
	GotoPreset(profileToken, presetToken string, speed *PTZVector) error
	RemovePreset(profileToken, presetToken string) error
}

// ONVIFDevice 原始设备实现
type ONVIFDevice struct {
	Username  string
	Password  string
	sdkDevice *goonvif.Device
}

// NewONVIFDevice 创建一个 ONVIFDevice 实例
func NewONVIFDevice(username string, password string) ONVIFClient {
	return &ONVIFDevice{
		Username: username,
		Password: password,
	}
}

// Connect 尝试连接到 ONVIF 设备
func (d *ONVIFDevice) Connect(xaddr string) error {
	var err error
	params := goonvif.DeviceParams{
		Xaddr:    xaddr,
		Username: d.Username,
		Password: d.Password,
	}
	d.sdkDevice, err = goonvif.NewDevice(params)
	if err != nil {
		return fmt.Errorf("onvif client connect failed: %w", err)
	}

	// 增加一个额外的 check，确保设备可以响应 GetSystemDateAndTime
	_, err = d.GetSystemDateAndTime()
	if err != nil {
		return fmt.Errorf("onvif check failed (GetSystemDateAndTime): %w", err)
	}

	return nil
}

// Close 不进行操作，因为 goonvif 库没有明确的 Close 方法
func (d *ONVIFDevice) Close() {
	d.sdkDevice = nil
}

// GetSystemDateAndTime 内部辅助函数，用于连接检查
func (d *ONVIFDevice) GetSystemDateAndTime() (*device.GetSystemDateAndTimeResponse, error) {
	if d.sdkDevice == nil {
		return nil, fmt.Errorf("device not connected")
	}
	resp, err := sdkdevice.Call_GetSystemDateAndTime(context.Background(), d.sdkDevice, device.GetSystemDateAndTime{})
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetDeviceInformation 获取设备信息
func (d *ONVIFDevice) GetDeviceInformation() (*DeviceInformation, error) {
	if d.sdkDevice == nil {
		return nil, fmt.Errorf("device not connected")
	}
	resp, err := sdkdevice.Call_GetDeviceInformation(context.Background(), d.sdkDevice, device.GetDeviceInformation{})
	if err != nil {
		return nil, err
	}

	return &DeviceInformation{
		Manufacturer:    resp.Manufacturer,
		Model:           resp.Model,
		FirmwareVersion: resp.FirmwareVersion,
		SerialNumber:    resp.SerialNumber,
		HardwareID:      resp.HardwareId,
		Name:            resp.Manufacturer + " " + resp.Model, // 组合名称
	}, nil
}

// GetCapabilities 获取设备能力
func (d *ONVIFDevice) GetCapabilities() (*DeviceCapabilities, error) {
	if d.sdkDevice == nil {
		return nil, fmt.Errorf("device not connected")
	}
	resp, err := sdkdevice.Call_GetCapabilities(context.Background(), d.sdkDevice, device.GetCapabilities{})
	if err != nil {
		return nil, err
	}

	return &DeviceCapabilities{
		HasPTZ: true, // 简化处理，假设设备支持PTZ
		Media:  &resp.Capabilities.Media,
		PTZ:    &resp.Capabilities.PTZ,
	}, nil
}

// GetProfiles 获取媒体配置
func (d *ONVIFDevice) GetProfiles() ([]MediaProfile, error) {
	if d.sdkDevice == nil {
		return nil, fmt.Errorf("device not connected")
	}
	resp, err := sdkmedia.Call_GetProfiles(context.Background(), d.sdkDevice, media.GetProfiles{})
	if err != nil {
		return nil, err
	}

	profiles := make([]MediaProfile, len(resp.Profiles))
	for i, p := range resp.Profiles {
		profile := MediaProfile{
			Token: string(p.Token),
			Name:  string(p.Name),
		}

		// 提取视频编码器信息
		if p.VideoEncoderConfiguration.Token != "" {
			profile.Encoding = string(p.VideoEncoderConfiguration.Encoding)
			if p.VideoEncoderConfiguration.Resolution.Width > 0 {
				profile.Width = int(p.VideoEncoderConfiguration.Resolution.Width)
				profile.Height = int(p.VideoEncoderConfiguration.Resolution.Height)
				profile.Resolution = fmt.Sprintf("%dx%d", profile.Width, profile.Height)
			}
			if p.VideoEncoderConfiguration.RateControl.FrameRateLimit > 0 {
				profile.FPS = int(p.VideoEncoderConfiguration.RateControl.FrameRateLimit)
				profile.Bitrate = int(p.VideoEncoderConfiguration.RateControl.BitrateLimit)
			}
		}

		// 提取PTZ配置
		if p.PTZConfiguration.Token != "" {
			profile.PTZConfig = &PTZConfig{
				Token:     string(p.PTZConfiguration.Token),
				Name:      string(p.PTZConfiguration.Name),
				NodeToken: string(p.PTZConfiguration.NodeToken),
			}
		}

		profiles[i] = profile
	}
	return profiles, nil
}

// GetStreamURI 获取流地址
func (d *ONVIFDevice) GetStreamURI(profileToken string) (string, error) {
	if d.sdkDevice == nil {
		return "", fmt.Errorf("device not connected")
	}
	resp, err := sdkmedia.Call_GetStreamUri(context.Background(), d.sdkDevice, media.GetStreamUri{
		StreamSetup: onvifx.StreamSetup{
			Stream: "RTP-Unicast", // 默认 RTP/UDP 单播
			Transport: onvifx.Transport{
				Protocol: "RTSP", // 默认 RTSP
			},
		},
		ProfileToken: onvifx.ReferenceToken(profileToken),
	})
	if err != nil {
		return "", err
	}

	uri := resp.MediaUri.Uri

	// 如果 uri 中没有用户名和密码，则手动添加
	u, err := url.Parse(string(uri))
	if err == nil {
		if u.User == nil && d.Username != "" {
			u.User = url.UserPassword(d.Username, d.Password)
			uri = xsd.AnyURI(u.String())
		}
	}

	return string(uri), nil
}

// GetSnapshotURI 获取抓图地址
func (d *ONVIFDevice) GetSnapshotURI(profileToken string) (string, error) {
	if d.sdkDevice == nil {
		return "", fmt.Errorf("device not connected")
	}
	resp, err := sdkmedia.Call_GetSnapshotUri(context.Background(), d.sdkDevice, media.GetSnapshotUri{
		ProfileToken: onvifx.ReferenceToken(profileToken),
	})
	if err != nil {
		return "", err
	}

	uri := resp.MediaUri.Uri

	// 如果 uri 中没有用户名和密码，则手动添加
	u, err := url.Parse(string(uri))
	if err == nil {
		if u.User == nil && d.Username != "" {
			u.User = url.UserPassword(d.Username, d.Password)
			uri = xsd.AnyURI(u.String())
		}
	}

	return string(uri), nil
}

// GetVideoEncoderConfigurations 获取视频编码配置
func (d *ONVIFDevice) GetVideoEncoderConfigurations(profileToken string) ([]VideoEncoderConfig, error) {
	if d.sdkDevice == nil {
		return nil, fmt.Errorf("device not connected")
	}

	// 1. 获取 Profile
	profilesResp, err := sdkmedia.Call_GetProfile(context.Background(), d.sdkDevice, media.GetProfile{
		ProfileToken: onvifx.ReferenceToken(profileToken),
	})
	if err != nil {
		return nil, fmt.Errorf("get profile failed: %w", err)
	}

	if profilesResp.Profile.VideoEncoderConfiguration.Token == "" {
		return nil, nil // 没有编码配置
	}

	// 2. 获取实际配置
	req := media.GetVideoEncoderConfiguration{
		ConfigurationToken: profilesResp.Profile.VideoEncoderConfiguration.Token,
	}

	resp, err := sdkmedia.Call_GetVideoEncoderConfiguration(context.Background(), d.sdkDevice, req)
	if err != nil {
		return nil, fmt.Errorf("get video encoder config failed: %w", err)
	}

	c := resp.Configuration

	config := VideoEncoderConfig{
		Token:        string(c.Token),
		Name:         string(c.Name),
		Encoding:     string(c.Encoding),
		Width:        int(c.Resolution.Width),
		Height:       int(c.Resolution.Height),
		Quality:      int(c.Quality),
		FrameRate:    int(c.RateControl.FrameRateLimit),
		BitrateLimit: int(c.RateControl.BitrateLimit),
		GovLength:    int(c.H264.GovLength),
		H264Profile:  string(c.H264.H264Profile),
	}

	return []VideoEncoderConfig{config}, nil
}

// ContinuousMove 持续移动
func (d *ONVIFDevice) ContinuousMove(profileToken string, vector *PTZVector) error {
	if d.sdkDevice == nil {
		return fmt.Errorf("ptz not available")
	}

	req := ptz.ContinuousMove{ProfileToken: onvifx.ReferenceToken(profileToken)}

	var pt onvifx.Vector2D
	var z onvifx.Vector1D
	var speed onvifx.PTZSpeed

	if vector.PanTilt != nil {
		pt.X, pt.Y = vector.PanTilt.X, vector.PanTilt.Y
		speed.PanTilt = pt
	}
	if vector.Zoom != nil {
		z.X = vector.Zoom.X
		speed.Zoom = z
	}

	req.Velocity = speed

	_, err := sdkptz.Call_ContinuousMove(context.Background(), d.sdkDevice, req)
	return err
}

// Stop 停止移动
func (d *ONVIFDevice) Stop(profileToken string) error {
	if d.sdkDevice == nil {
		return fmt.Errorf("ptz not available")
	}
	req := ptz.Stop{ProfileToken: onvifx.ReferenceToken(profileToken)}
	// 停止所有轴
	req.PanTilt = xsd.Boolean(true)
	req.Zoom = xsd.Boolean(true)

	_, err := sdkptz.Call_Stop(context.Background(), d.sdkDevice, req)
	return err
}

// BoolPtr returns a pointer to the given bool value.
func BoolPtr(b bool) *bool {
	return &b
}

// GetPresets 获取预置位
func (d *ONVIFDevice) GetPresets(profileToken string) ([]PTZPreset, error) {
	if d.sdkDevice == nil {
		return nil, fmt.Errorf("ptz not available")
	}
	resp, err := sdkptz.Call_GetPresets(context.Background(), d.sdkDevice, ptz.GetPresets{
		ProfileToken: onvifx.ReferenceToken(profileToken),
	})
	if err != nil {
		return nil, err
	}

	presets := make([]PTZPreset, 0)
	// resp.Preset is a struct, not a slice, so just append it directly
	p := resp.Preset
	presets = append(presets, PTZPreset{Token: string(p.Token), Name: string(p.Name)})
	return presets, nil
}

// GotoPreset 跳转预置位
func (d *ONVIFDevice) GotoPreset(profileToken, presetToken string, speed *PTZVector) error {
	if d.sdkDevice == nil {
		return fmt.Errorf("ptz not available")
	}
	req := ptz.GotoPreset{ProfileToken: onvifx.ReferenceToken(profileToken), PresetToken: onvifx.ReferenceToken(presetToken)}
	if speed != nil {
		var pt onvifx.Vector2D
		var z onvifx.Vector1D
		if speed.PanTilt != nil {
			pt.X, pt.Y = speed.PanTilt.X, speed.PanTilt.Y
		}
		if speed.Zoom != nil {
			z.X = speed.Zoom.X
		}
		req.Speed = onvifx.PTZSpeed{PanTilt: pt, Zoom: z}
	}
	_, err := sdkptz.Call_GotoPreset(context.Background(), d.sdkDevice, req)
	return err
}

// SetPreset 设置预置位
func (d *ONVIFDevice) SetPreset(profileToken, presetName, presetToken string) (string, error) {
	if d.sdkDevice == nil {
		return "", fmt.Errorf("ptz not available")
	}
	req := ptz.SetPreset{ProfileToken: onvifx.ReferenceToken(profileToken), PresetName: xsd.String(presetName)}
	if presetToken != "" {
		req.PresetToken = onvifx.ReferenceToken(presetToken)
	}
	resp, err := sdkptz.Call_SetPreset(context.Background(), d.sdkDevice, req)
	if err != nil {
		return "", err
	}
	return string(resp.PresetToken), nil
}

// RemovePreset 删除预置位
func (d *ONVIFDevice) RemovePreset(profileToken, presetToken string) error {
	if d.sdkDevice == nil {
		return fmt.Errorf("ptz not available")
	}
	req := ptz.RemovePreset{ProfileToken: onvifx.ReferenceToken(profileToken), PresetToken: onvifx.ReferenceToken(presetToken)}
	_, err := sdkptz.Call_RemovePreset(context.Background(), d.sdkDevice, req)
	return err
}
