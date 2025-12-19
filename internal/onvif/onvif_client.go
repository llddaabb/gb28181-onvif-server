// internal/onvif/onvif_client.go
// 纯SOAP实现的ONVIF客户端（已移除goonvif依赖）
package onvif

// VideoEncoderConfig 视频编码配置
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

// DeviceCapabilities 设备能力结构
type DeviceCapabilities struct {
	HasPTZ bool
	Media  interface{}
	PTZ    interface{}
}
