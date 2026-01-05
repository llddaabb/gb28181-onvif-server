package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// 配置管理总览：
// ===============================================================================
// 本系统采用单一配置源 (Single Source of Truth) 设计：
//
// 1. 唯一配置源：config.yaml
//    - 包含所有系统配置：GB28181、ONVIF、API、调试、ZLM、AI、认证等
//    - 由运维人员维护和编辑
//    - 应被纳入版本控制
//
// 2. 自动生成：zlm_config.ini
//    - 由程序在启动时从 config.yaml 中的 ZLM 配置动态生成
//    - 生成方式：Config.ZLM.GenerateConfigINI() 方法
//    - 不应被手动编辑，不应纳入版本控制
//    - 作用：供 ZLMediaKit 进程读取
//
// 3. 启动流程：
//    a) 加载 config.yaml
//    b) 解析为 Config 结构体
//    c) 调用 cfg.ZLM.GenerateConfigINI() 生成 INI 格式内容
//    d) 通过 ProcessManager.SetConfigContent() 传递给 ZLM 进程
//    e) ZLM 进程启动前，将配置内容写入临时配置文件
//    f) ZLM 进程读取此配置文件启动
//
// 4. 配置同步原理：
//    - 每次启动时重新生成 zlm_config.ini，保证与 config.yaml 同步
//    - 如需修改 ZLM 配置，仅需编辑 config.yaml 然后重启服务
//    - 不存在两个配置文件版本不一致的问题
//
// 5. 关键配置项对应关系：
//    config.yaml ZLM.API.Secret  <-> zlm_config.ini [api] secret=
//    config.yaml ZLM.HTTP.Port   <-> zlm_config.ini [http] port=
//    config.yaml ZLM.RTMP.Port   <-> zlm_config.ini [rtmp] port=
//    ... 以此类推
// ===============================================================================

// GB28181Config GB28181配置结构体
type GB28181Config struct {
	SipIP             string `yaml:"SipIP"`
	SipPort           int    `yaml:"SipPort"`
	Realm             string `yaml:"Realm"`
	ServerID          string `yaml:"ServerID"`
	Password          string `yaml:"Password"`
	HeartbeatInterval int    `yaml:"HeartbeatInterval"`
	RegisterExpires   int    `yaml:"RegisterExpires"`
}

// ONVIFConfig ONVIF配置结构体
type ONVIFConfig struct {
	MediaPortRange    string `yaml:"MediaPortRange"`
	EnableCheck       bool   `yaml:"EnableCheck"`
	CheckInterval     int    `yaml:"CheckInterval"`
	DiscoveryInterval int    `yaml:"DiscoveryInterval"`
	MaxFailureCount   int    `yaml:"MaxFailureCount"`
}

// APIConfig API配置结构体
type APIConfig struct {
	Host             string   `yaml:"Host"`
	Port             int      `yaml:"Port"`
	CorsAllowOrigins []string `yaml:"CorsAllowOrigins"`
	StaticDir        string   `yaml:"StaticDir"` // 静态文件目录，默认为 www
}

// DebugConfig 调试配置结构体
type DebugConfig struct {
	Enabled    bool     `yaml:"Enabled"`
	LogLevel   string   `yaml:"LogLevel"`
	LogFile    string   `yaml:"LogFile"`
	Services   []string `yaml:"Services"`
	Timestamp  bool     `yaml:"Timestamp"`
	CallerInfo bool     `yaml:"CallerInfo"`
}

// ZLMConfig ZLMediaKit 配置结构体
type ZLMConfig struct {
	// 进程管理配置
	UseEmbedded bool `yaml:"UseEmbedded"`
	AutoRestart bool `yaml:"AutoRestart"`
	MaxRestarts int  `yaml:"MaxRestarts"`

	// 子配置
	API      *ZLMAPIConfig      `yaml:"API"`
	FFmpeg   *ZLMFFmpegConfig   `yaml:"FFmpeg"`
	Protocol *ZLMProtocolConfig `yaml:"Protocol"`
	General  *ZLMGeneralConfig  `yaml:"General"`
	HLS      *ZLMHLSConfig      `yaml:"HLS"`
	Hook     *ZLMHookConfig     `yaml:"Hook"`
	HTTP     *ZLMHTTPConfig     `yaml:"HTTP"`
	RTMP     *ZLMRTMPConfig     `yaml:"RTMP"`
	RTSP     *ZLMRTSPConfig     `yaml:"RTSP"`
	RTP      *ZLMRTPConfig      `yaml:"RTP"`
	RTPProxy *ZLMRTPProxyConfig `yaml:"RTPProxy"`
	Record   *ZLMRecordConfig   `yaml:"Record"`
	RTC      *ZLMRTCConfig      `yaml:"RTC"`
	SRT      *ZLMSRTConfig      `yaml:"SRT"`
	Shell    *ZLMShellConfig    `yaml:"Shell"`
}

// ZLMAPIConfig ZLM API 配置
type ZLMAPIConfig struct {
	Debug       bool   `yaml:"Debug"`
	Secret      string `yaml:"Secret"`
	SnapRoot    string `yaml:"SnapRoot"`
	DefaultSnap string `yaml:"DefaultSnap"`
}

// ZLMFFmpegConfig ZLM FFmpeg 配置
type ZLMFFmpegConfig struct {
	Bin        string `yaml:"Bin"`
	Log        string `yaml:"Log"`
	RestartSec int    `yaml:"RestartSec"`
}

// ZLMProtocolConfig ZLM 协议配置
type ZLMProtocolConfig struct {
	ModifyStamp    int  `yaml:"ModifyStamp"`
	EnableAudio    bool `yaml:"EnableAudio"`
	AddMuteAudio   bool `yaml:"AddMuteAudio"`
	AutoClose      bool `yaml:"AutoClose"`
	ContinuePushMS int  `yaml:"ContinuePushMS"`
	EnableHLS      bool `yaml:"EnableHLS"`
	EnableHLSFmp4  bool `yaml:"EnableHLSFmp4"`
	EnableMP4      bool `yaml:"EnableMP4"`
	EnableRTSP     bool `yaml:"EnableRTSP"`
	EnableRTMP     bool `yaml:"EnableRTMP"`
	EnableTS       bool `yaml:"EnableTS"`
	EnableFMP4     bool `yaml:"EnableFMP4"`
	HLSDemand      bool `yaml:"HLSDemand"`
	RTSPDemand     bool `yaml:"RTSPDemand"`
	RTMPDemand     bool `yaml:"RTMPDemand"`
	TSDemand       bool `yaml:"TSDemand"`
	FMP4Demand     bool `yaml:"FMP4Demand"`
}

// ZLMGeneralConfig ZLM 通用配置
type ZLMGeneralConfig struct {
	EnableVhost             bool   `yaml:"EnableVhost"`
	FlowThreshold           int    `yaml:"FlowThreshold"`
	MaxStreamWaitMS         int    `yaml:"MaxStreamWaitMS"`
	StreamNoneReaderDelayMS int    `yaml:"StreamNoneReaderDelayMS"`
	MergeWriteMS            int    `yaml:"MergeWriteMS"`
	MediaServerId           string `yaml:"MediaServerId"`
	ListenIP                string `yaml:"ListenIP"`
}

// ZLMHLSConfig ZLM HLS 配置
type ZLMHLSConfig struct {
	FileBufSize    int  `yaml:"FileBufSize"`
	SegDur         int  `yaml:"SegDur"`
	SegNum         int  `yaml:"SegNum"`
	SegDelay       int  `yaml:"SegDelay"`
	SegRetain      int  `yaml:"SegRetain"`
	DeleteDelaySec int  `yaml:"DeleteDelaySec"`
	SegKeep        bool `yaml:"SegKeep"`
}

// ZLMHookConfig ZLM Hook 配置
type ZLMHookConfig struct {
	Enable               bool    `yaml:"Enable"`
	TimeoutSec           int     `yaml:"TimeoutSec"`
	AliveInterval        float64 `yaml:"AliveInterval"`
	Retry                int     `yaml:"Retry"`
	RetryDelay           float64 `yaml:"RetryDelay"`
	OnFlowReport         string  `yaml:"OnFlowReport"`
	OnHttpAccess         string  `yaml:"OnHttpAccess"`
	OnPlay               string  `yaml:"OnPlay"`
	OnPublish            string  `yaml:"OnPublish"`
	OnRecordMP4          string  `yaml:"OnRecordMP4"`
	OnRecordTS           string  `yaml:"OnRecordTS"`
	OnStreamChanged      string  `yaml:"OnStreamChanged"`
	OnStreamNoneReader   string  `yaml:"OnStreamNoneReader"`
	OnStreamNotFound     string  `yaml:"OnStreamNotFound"`
	OnServerStarted      string  `yaml:"OnServerStarted"`
	OnServerExited       string  `yaml:"OnServerExited"`
	OnServerKeepalive    string  `yaml:"OnServerKeepalive"`
	OnSendRTPStopped     string  `yaml:"OnSendRTPStopped"`
	OnRTPServerTimeout   string  `yaml:"OnRTPServerTimeout"`
	StreamChangedSchemas string  `yaml:"StreamChangedSchemas"`
}

// ZLMHTTPConfig ZLM HTTP 配置
type ZLMHTTPConfig struct {
	Port              int    `yaml:"Port"`
	SSLPort           int    `yaml:"SSLPort"`
	CharSet           string `yaml:"CharSet"`
	KeepAliveSecond   int    `yaml:"KeepAliveSecond"`
	MaxReqSize        int    `yaml:"MaxReqSize"`
	RootPath          string `yaml:"RootPath"`
	SendBufSize       int    `yaml:"SendBufSize"`
	DirMenu           bool   `yaml:"DirMenu"`
	AllowCrossDomains bool   `yaml:"AllowCrossDomains"`
	AllowIPRange      string `yaml:"AllowIPRange"`
}

// ZLMRTMPConfig ZLM RTMP 配置
type ZLMRTMPConfig struct {
	Port            int  `yaml:"Port"`
	SSLPort         int  `yaml:"SSLPort"`
	HandshakeSecond int  `yaml:"HandshakeSecond"`
	KeepAliveSecond int  `yaml:"KeepAliveSecond"`
	DirectProxy     bool `yaml:"DirectProxy"`
	Enhanced        bool `yaml:"Enhanced"`
}

// ZLMRTSPConfig ZLM RTSP 配置
type ZLMRTSPConfig struct {
	Port             int  `yaml:"Port"`
	SSLPort          int  `yaml:"SSLPort"`
	AuthBasic        bool `yaml:"AuthBasic"`
	DirectProxy      bool `yaml:"DirectProxy"`
	HandshakeSecond  int  `yaml:"HandshakeSecond"`
	KeepAliveSecond  int  `yaml:"KeepAliveSecond"`
	LowLatency       bool `yaml:"LowLatency"`
	RTPTransportType int  `yaml:"RTPTransportType"`
}

// ZLMRTPConfig ZLM RTP 配置
type ZLMRTPConfig struct {
	AudioMtuSize int  `yaml:"AudioMtuSize"`
	VideoMtuSize int  `yaml:"VideoMtuSize"`
	RTPMaxSize   int  `yaml:"RTPMaxSize"`
	LowLatency   bool `yaml:"LowLatency"`
	H264StapA    bool `yaml:"H264StapA"`
}

// ZLMRTPProxyConfig ZLM RTP 代理配置
type ZLMRTPProxyConfig struct {
	Port                int    `yaml:"Port"`
	TimeoutSec          int    `yaml:"TimeoutSec"`
	PortRange           string `yaml:"PortRange"`
	H264PT              int    `yaml:"H264PT"`
	H265PT              int    `yaml:"H265PT"`
	PSPT                int    `yaml:"PSPT"`
	OpusPT              int    `yaml:"OpusPT"`
	GOPCache            int    `yaml:"GOPCache"`
	RTPG711DurMS        int    `yaml:"RTPG711DurMS"`
	UDPRecvSocketBuffer int    `yaml:"UDPRecvSocketBuffer"`
	MergeFrame          bool   `yaml:"MergeFrame"`
}

// ZLMRecordConfig ZLM 录制配置
type ZLMRecordConfig struct {
	AppName     string `yaml:"AppName"`
	RecordPath  string `yaml:"RecordPath"` // 录像存储路径（独立于ZLM工作目录）
	FileBufSize int    `yaml:"FileBufSize"`
	SampleMS    int    `yaml:"SampleMS"`
	FastStart   bool   `yaml:"FastStart"`
	FileRepeat  bool   `yaml:"FileRepeat"`
	EnableFmp4  bool   `yaml:"EnableFmp4"`

	// 录像分割配置
	FileSecond int `yaml:"FileSecond"` // 录像切片时长(秒), 0表示不切片
	FileSizeMB int `yaml:"FileSizeMB"` // 录像文件大小限制(MB), 0表示不限制

	// 视频编码配置
	EnableVideoCodec bool   `yaml:"EnableVideoCodec"` // 是否启用视频编码压缩
	VideoCodec       string `yaml:"VideoCodec"`       // 视频编码格式: h264, h265
	VideoBitrate     int    `yaml:"VideoBitrate"`     // 视频比特率(kbps), 0为自动
}

// ZLMRTCConfig ZLM WebRTC 配置
type ZLMRTCConfig struct {
	Port              int     `yaml:"Port"`
	TCPPort           int     `yaml:"TCPPort"`
	TimeoutSec        int     `yaml:"TimeoutSec"`
	ExternIP          string  `yaml:"ExternIP"`
	Interfaces        string  `yaml:"Interfaces"`
	RembBitRate       int     `yaml:"RembBitRate"`
	PreferredCodecA   string  `yaml:"PreferredCodecA"`
	PreferredCodecV   string  `yaml:"PreferredCodecV"`
	MaxRtpCacheMS     int     `yaml:"MaxRtpCacheMS"`
	MaxRtpCacheSize   int     `yaml:"MaxRtpCacheSize"`
	NackMaxSize       int     `yaml:"NackMaxSize"`
	NackMaxMS         int     `yaml:"NackMaxMS"`
	NackMaxCount      int     `yaml:"NackMaxCount"`
	NackIntervalRatio float64 `yaml:"NackIntervalRatio"`
	NackRtpSize       int     `yaml:"NackRtpSize"`
	BFilter           bool    `yaml:"BFilter"`
}

// ZLMSRTConfig ZLM SRT 配置
type ZLMSRTConfig struct {
	Port       int    `yaml:"Port"`
	TimeoutSec int    `yaml:"TimeoutSec"`
	LatencyMul int    `yaml:"LatencyMul"`
	PktBufSize int    `yaml:"PktBufSize"`
	PassPhrase string `yaml:"PassPhrase"`
}

// ZLMShellConfig ZLM Shell 配置
type ZLMShellConfig struct {
	Port       int `yaml:"Port"`
	MaxReqSize int `yaml:"MaxReqSize"`
}

// Config 总配置结构体
// AIConfig AI功能配置
type AIConfig struct {
	Enable         bool     `yaml:"Enable"`         // 是否启用AI功能
	AutoStart      bool     `yaml:"AutoStart"`      // 启动时自动开始AI检测
	DetectorType   string   `yaml:"DetectorType"`   // 检测器类型: auto, http, embedded, onnx
	APIEndpoint    string   `yaml:"APIEndpoint"`    // AI检测API地址（HTTP类型）
	ModelPath      string   `yaml:"ModelPath"`      // 本地模型路径（ONNX类型）
	Confidence     float32  `yaml:"Confidence"`     // 置信度阈值
	IoUThreshold   float32  `yaml:"IoUThreshold"`   // NMS IoU阈值
	InputSize      int      `yaml:"InputSize"`      // 输入图像尺寸
	DetectInterval int      `yaml:"DetectInterval"` // 检测间隔(秒)
	RecordDelay    int      `yaml:"RecordDelay"`    // 录像延迟(秒)
	MinRecordTime  int      `yaml:"MinRecordTime"`  // 最小录像时长(秒)
	NumThreads     int      `yaml:"NumThreads"`     // CPU线程数（0=自动）
	AutoChannels   []string `yaml:"AutoChannels"`   // 自动启动AI的通道ID列表（空=全部通道）
}

// AuthConfig 认证配置
type AuthConfig struct {
	Enable          bool   `yaml:"Enable"`          // 是否启用认证
	JWTSecret       string `yaml:"JWTSecret"`       // JWT密钥
	TokenExpiry     int    `yaml:"TokenExpiry"`     // 令牌过期时间(小时)
	UsersFile       string `yaml:"UsersFile"`       // 用户数据文件
	DefaultAdmin    string `yaml:"DefaultAdmin"`    // 默认管理员账户
	DefaultPassword string `yaml:"DefaultPassword"` // 默认管理员密码
}

type Config struct {
	GB28181 *GB28181Config `yaml:"GB28181"`
	ONVIF   *ONVIFConfig   `yaml:"ONVIF"`
	API     *APIConfig     `yaml:"API"`
	Debug   *DebugConfig   `yaml:"Debug"`
	ZLM     *ZLMConfig     `yaml:"ZLM"`
	AI      *AIConfig      `yaml:"AI"`
	Auth    *AuthConfig    `yaml:"Auth"`
}

// Load 从文件加载配置
func Load(filePath string) (*Config, error) {
	// 读取配置文件
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	// 解析YAML配置
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// 确保所有配置都有默认值
	if config.GB28181 == nil {
		config.GB28181 = &GB28181Config{
			SipIP:             "0.0.0.0",
			SipPort:           5060,
			Realm:             "3402000000",
			ServerID:          "34020000002000000001",
			Password:          "",
			HeartbeatInterval: 30,
			RegisterExpires:   3600,
		}
	}

	if config.ONVIF == nil {
		config.ONVIF = &ONVIFConfig{
			EnableCheck:     false,
			CheckInterval:   60,
			MaxFailureCount: 3,
			MediaPortRange:  "8000-9000",
		}
	}

	if config.API == nil {
		config.API = &APIConfig{
			Host:             "0.0.0.0",
			Port:             8080,
			CorsAllowOrigins: []string{"*"},
			StaticDir:        "www",
		}
	} else if config.API.StaticDir == "" {
		config.API.StaticDir = "www"
	}

	if config.ZLM == nil {
		config.ZLM = DefaultZLMConfig()
	} else {
		// 填充子配置默认值
		config.ZLM.FillDefaults()
	}

	if config.AI == nil {
		config.AI = &AIConfig{
			Enable:         false, // 默认关闭
			APIEndpoint:    "http://localhost:8000/detect",
			ModelPath:      "./models/yolov8n.onnx",
			Confidence:     0.5,
			DetectInterval: 2,
			RecordDelay:    10,
			MinRecordTime:  5,
		}
	}

	if config.Auth == nil {
		config.Auth = &AuthConfig{
			Enable:          true,
			JWTSecret:       "", // 为空时自动生成
			TokenExpiry:     24, // 24小时
			UsersFile:       "configs/users.json",
			DefaultAdmin:    "admin",
			DefaultPassword: "admin123",
		}
	}

	log.Printf("配置加载成功: %s", filePath)
	return &config, nil
}

// DefaultZLMConfig 返回默认的 ZLM 配置
func DefaultZLMConfig() *ZLMConfig {
	return &ZLMConfig{
		UseEmbedded: true,
		AutoRestart: true,
		MaxRestarts: 10,
		API: &ZLMAPIConfig{
			Debug:       false,
			Secret:      "035c73f7-bb6b-4889-a715-d9eb2d1925cc",
			SnapRoot:    "./www/snap/",
			DefaultSnap: "./www/logo.png",
		},
		FFmpeg: &ZLMFFmpegConfig{
			Bin:        "/usr/bin/ffmpeg",
			Log:        "./ffmpeg/ffmpeg.log",
			RestartSec: 0,
		},
		Protocol: &ZLMProtocolConfig{
			ModifyStamp:    2,
			EnableAudio:    true,
			AddMuteAudio:   true,
			AutoClose:      false,
			ContinuePushMS: 15000,
			EnableHLS:      true,
			EnableHLSFmp4:  false,
			EnableMP4:      false,
			EnableRTSP:     true,
			EnableRTMP:     true,
			EnableTS:       true,
			EnableFMP4:     true,
			HLSDemand:      false,
			RTSPDemand:     false,
			RTMPDemand:     false,
			TSDemand:       false,
			FMP4Demand:     false,
		},
		General: &ZLMGeneralConfig{
			EnableVhost:             false,
			FlowThreshold:           1024,
			MaxStreamWaitMS:         15000,
			StreamNoneReaderDelayMS: 20000,
			MergeWriteMS:            0,
			MediaServerId:           "zpip-server",
			ListenIP:                "::",
		},
		HLS: &ZLMHLSConfig{
			FileBufSize:    65536,
			SegDur:         2,
			SegNum:         3,
			SegDelay:       0,
			SegRetain:      5,
			DeleteDelaySec: 10,
			SegKeep:        false,
		},
		Hook: &ZLMHookConfig{
			Enable:               false,
			TimeoutSec:           10,
			AliveInterval:        10.0,
			Retry:                1,
			RetryDelay:           3.0,
			StreamChangedSchemas: "rtsp/rtmp/fmp4/ts/hls/hls.fmp4",
		},
		HTTP: &ZLMHTTPConfig{
			Port:              80,
			SSLPort:           443,
			CharSet:           "utf-8",
			KeepAliveSecond:   30,
			MaxReqSize:        40960,
			RootPath:          "./www",
			SendBufSize:       65536,
			DirMenu:           true,
			AllowCrossDomains: true,
			AllowIPRange:      "::1,127.0.0.1,172.16.0.0-172.31.255.255,192.168.0.0-192.168.255.255,10.0.0.0-10.255.255.255",
		},
		RTMP: &ZLMRTMPConfig{
			Port:            1935,
			SSLPort:         0,
			HandshakeSecond: 15,
			KeepAliveSecond: 15,
			DirectProxy:     true,
			Enhanced:        true,
		},
		RTSP: &ZLMRTSPConfig{
			Port:             554,
			SSLPort:          0,
			AuthBasic:        false,
			DirectProxy:      true,
			HandshakeSecond:  15,
			KeepAliveSecond:  15,
			LowLatency:       false,
			RTPTransportType: -1,
		},
		RTP: &ZLMRTPConfig{
			AudioMtuSize: 600,
			VideoMtuSize: 1400,
			RTPMaxSize:   10,
			LowLatency:   false,
			H264StapA:    true,
		},
		RTPProxy: &ZLMRTPProxyConfig{
			Port:                10000,
			TimeoutSec:          15,
			PortRange:           "30000-35000",
			H264PT:              98,
			H265PT:              99,
			PSPT:                96,
			OpusPT:              100,
			GOPCache:            1,
			RTPG711DurMS:        100,
			UDPRecvSocketBuffer: 4194304,
			MergeFrame:          true,
		},
		Record: &ZLMRecordConfig{
			AppName:          "record",
			RecordPath:       "./recordings", // 默认录像目录（项目根目录下）
			FileBufSize:      65536,
			SampleMS:         500,
			FastStart:        false,
			FileRepeat:       false,
			EnableFmp4:       false,
			FileSecond:       3600,   // 默认1小时切割一次
			FileSizeMB:       0,      // 0表示仅按时间切割
			EnableVideoCodec: false,  // 默认不启用转码（直接存储原始流）
			VideoCodec:       "h264", // 默认H264编码
			VideoBitrate:     2000,   // 默认2Mbps
		},
		RTC: &ZLMRTCConfig{
			Port:              8000,
			TCPPort:           8000,
			TimeoutSec:        15,
			ExternIP:          "",
			Interfaces:        "",
			RembBitRate:       0,
			PreferredCodecA:   "PCMA,PCMU,opus,mpeg4-generic",
			PreferredCodecV:   "H264,H265,AV1,VP9,VP8",
			MaxRtpCacheMS:     5000,
			MaxRtpCacheSize:   2048,
			NackMaxSize:       2048,
			NackMaxMS:         3000,
			NackMaxCount:      15,
			NackIntervalRatio: 1.0,
			NackRtpSize:       8,
			BFilter:           false,
		},
		SRT: &ZLMSRTConfig{
			Port:       9000,
			TimeoutSec: 5,
			LatencyMul: 4,
			PktBufSize: 8192,
			PassPhrase: "",
		},
		Shell: &ZLMShellConfig{
			Port:       0,
			MaxReqSize: 1024,
		},
	}
}

// FillDefaults 填充 ZLM 配置的默认值
func (z *ZLMConfig) FillDefaults() {
	defaults := DefaultZLMConfig()
	if z.API == nil {
		z.API = defaults.API
	}
	if z.FFmpeg == nil {
		z.FFmpeg = defaults.FFmpeg
	}
	if z.Protocol == nil {
		z.Protocol = defaults.Protocol
	}
	if z.General == nil {
		z.General = defaults.General
	}
	if z.HLS == nil {
		z.HLS = defaults.HLS
	}
	if z.Hook == nil {
		z.Hook = defaults.Hook
	}
	if z.HTTP == nil {
		z.HTTP = defaults.HTTP
	}
	if z.RTMP == nil {
		z.RTMP = defaults.RTMP
	}
	if z.RTSP == nil {
		z.RTSP = defaults.RTSP
	}
	if z.RTP == nil {
		z.RTP = defaults.RTP
	}
	if z.RTPProxy == nil {
		z.RTPProxy = defaults.RTPProxy
	}
	if z.Record == nil {
		z.Record = defaults.Record
	}
	if z.RTC == nil {
		z.RTC = defaults.RTC
	}
	if z.SRT == nil {
		z.SRT = defaults.SRT
	}
	if z.Shell == nil {
		z.Shell = defaults.Shell
	}
}

// GenerateConfigINI 生成 ZLMediaKit config.ini 内容
//
// 此方法将 config.yaml 中的 ZLM 配置转换为 INI 格式文本，供 ZLMediaKit 进程使用。
// 转换后的内容包括以下 14 个配置段：
//
//	[api]、[ffmpeg]、[protocol]、[general]、[hls]、[hook]、[cluster]、
//	[http]、[multicast]、[record]、[rtmp]、[rtp]、[rtp_proxy]、[rtc]、[srt]、[rtsp]、[shell]
//
// 说明：
// - 此方法在服务启动时调用，由 ProcessManager 使用
// - 生成的内容不应被手动编辑
// - 每次启动都重新生成，确保与 config.yaml 完全同步
func (z *ZLMConfig) GenerateConfigINI() string {
	var sb strings.Builder

	boolToInt := func(b bool) int {
		if b {
			return 1
		}
		return 0
	}

	// [api]
	sb.WriteString("[api]\n")
	sb.WriteString(fmt.Sprintf("apiDebug=%d\n", boolToInt(z.API.Debug)))
	sb.WriteString(fmt.Sprintf("secret=%s\n", z.API.Secret))
	sb.WriteString(fmt.Sprintf("snapRoot=%s\n", z.API.SnapRoot))
	sb.WriteString(fmt.Sprintf("defaultSnap=%s\n", z.API.DefaultSnap))
	sb.WriteString("downloadRoot=./www\n\n")

	// [ffmpeg]
	sb.WriteString("[ffmpeg]\n")
	sb.WriteString(fmt.Sprintf("bin=%s\n", z.FFmpeg.Bin))
	sb.WriteString("cmd=%s -re -i %s -c:a aac -strict -2 -ar 44100 -ab 48k -c:v libx264 -f flv %s\n")
	sb.WriteString("snap=%s -i %s -y -f mjpeg -frames:v 1 -an %s\n")
	sb.WriteString(fmt.Sprintf("log=%s\n", z.FFmpeg.Log))
	sb.WriteString(fmt.Sprintf("restart_sec=%d\n\n", z.FFmpeg.RestartSec))

	// [protocol]
	sb.WriteString("[protocol]\n")
	sb.WriteString(fmt.Sprintf("modify_stamp=%d\n", z.Protocol.ModifyStamp))
	sb.WriteString(fmt.Sprintf("enable_audio=%d\n", boolToInt(z.Protocol.EnableAudio)))
	sb.WriteString(fmt.Sprintf("add_mute_audio=%d\n", boolToInt(z.Protocol.AddMuteAudio)))
	sb.WriteString(fmt.Sprintf("auto_close=%d\n", boolToInt(z.Protocol.AutoClose)))
	sb.WriteString(fmt.Sprintf("continue_push_ms=%d\n", z.Protocol.ContinuePushMS))
	sb.WriteString("paced_sender_ms=0\n")
	sb.WriteString(fmt.Sprintf("enable_hls=%d\n", boolToInt(z.Protocol.EnableHLS)))
	sb.WriteString(fmt.Sprintf("enable_hls_fmp4=%d\n", boolToInt(z.Protocol.EnableHLSFmp4)))
	sb.WriteString(fmt.Sprintf("enable_mp4=%d\n", boolToInt(z.Protocol.EnableMP4)))
	sb.WriteString(fmt.Sprintf("enable_rtsp=%d\n", boolToInt(z.Protocol.EnableRTSP)))
	sb.WriteString(fmt.Sprintf("enable_rtmp=%d\n", boolToInt(z.Protocol.EnableRTMP)))
	sb.WriteString(fmt.Sprintf("enable_ts=%d\n", boolToInt(z.Protocol.EnableTS)))
	sb.WriteString(fmt.Sprintf("enable_fmp4=%d\n", boolToInt(z.Protocol.EnableFMP4)))
	sb.WriteString("mp4_as_player=0\n")
	sb.WriteString("mp4_max_second=3600\n")
	sb.WriteString("mp4_save_path=./www\n")
	sb.WriteString("hls_save_path=./www\n")
	sb.WriteString(fmt.Sprintf("hls_demand=%d\n", boolToInt(z.Protocol.HLSDemand)))
	sb.WriteString(fmt.Sprintf("rtsp_demand=%d\n", boolToInt(z.Protocol.RTSPDemand)))
	sb.WriteString(fmt.Sprintf("rtmp_demand=%d\n", boolToInt(z.Protocol.RTMPDemand)))
	sb.WriteString(fmt.Sprintf("ts_demand=%d\n", boolToInt(z.Protocol.TSDemand)))
	sb.WriteString(fmt.Sprintf("fmp4_demand=%d\n\n", boolToInt(z.Protocol.FMP4Demand)))

	// [general]
	sb.WriteString("[general]\n")
	sb.WriteString(fmt.Sprintf("enableVhost=%d\n", boolToInt(z.General.EnableVhost)))
	sb.WriteString(fmt.Sprintf("flowThreshold=%d\n", z.General.FlowThreshold))
	sb.WriteString(fmt.Sprintf("maxStreamWaitMS=%d\n", z.General.MaxStreamWaitMS))
	sb.WriteString(fmt.Sprintf("streamNoneReaderDelayMS=%d\n", z.General.StreamNoneReaderDelayMS))
	sb.WriteString("resetWhenRePlay=1\n")
	sb.WriteString(fmt.Sprintf("mergeWriteMS=%d\n", z.General.MergeWriteMS))
	sb.WriteString(fmt.Sprintf("mediaServerId=%s\n", z.General.MediaServerId))
	sb.WriteString("wait_track_ready_ms=10000\n")
	sb.WriteString("wait_audio_track_data_ms=1000\n")
	sb.WriteString("wait_add_track_ms=3000\n")
	sb.WriteString("unready_frame_cache=100\n")
	sb.WriteString("broadcast_player_count_changed=0\n")
	sb.WriteString(fmt.Sprintf("listen_ip=%s\n\n", z.General.ListenIP))

	// [hls]
	sb.WriteString("[hls]\n")
	sb.WriteString(fmt.Sprintf("fileBufSize=%d\n", z.HLS.FileBufSize))
	sb.WriteString(fmt.Sprintf("segDur=%d\n", z.HLS.SegDur))
	sb.WriteString(fmt.Sprintf("segNum=%d\n", z.HLS.SegNum))
	sb.WriteString(fmt.Sprintf("segDelay=%d\n", z.HLS.SegDelay))
	sb.WriteString(fmt.Sprintf("segRetain=%d\n", z.HLS.SegRetain))
	sb.WriteString("broadcastRecordTs=0\n")
	sb.WriteString(fmt.Sprintf("deleteDelaySec=%d\n", z.HLS.DeleteDelaySec))
	sb.WriteString(fmt.Sprintf("segKeep=%d\n", boolToInt(z.HLS.SegKeep)))
	sb.WriteString("fastRegister=0\n\n")

	// [hook]
	sb.WriteString("[hook]\n")
	sb.WriteString(fmt.Sprintf("enable=%d\n", boolToInt(z.Hook.Enable)))
	sb.WriteString(fmt.Sprintf("on_flow_report=%s\n", z.Hook.OnFlowReport))
	sb.WriteString(fmt.Sprintf("on_http_access=%s\n", z.Hook.OnHttpAccess))
	sb.WriteString(fmt.Sprintf("on_play=%s\n", z.Hook.OnPlay))
	sb.WriteString(fmt.Sprintf("on_publish=%s\n", z.Hook.OnPublish))
	sb.WriteString(fmt.Sprintf("on_record_mp4=%s\n", z.Hook.OnRecordMP4))
	sb.WriteString(fmt.Sprintf("on_record_ts=%s\n", z.Hook.OnRecordTS))
	sb.WriteString(fmt.Sprintf("on_stream_changed=%s\n", z.Hook.OnStreamChanged))
	sb.WriteString(fmt.Sprintf("stream_changed_schemas=%s\n", z.Hook.StreamChangedSchemas))
	sb.WriteString(fmt.Sprintf("on_stream_none_reader=%s\n", z.Hook.OnStreamNoneReader))
	sb.WriteString(fmt.Sprintf("on_stream_not_found=%s\n", z.Hook.OnStreamNotFound))
	sb.WriteString(fmt.Sprintf("on_server_started=%s\n", z.Hook.OnServerStarted))
	sb.WriteString(fmt.Sprintf("on_server_exited=%s\n", z.Hook.OnServerExited))
	sb.WriteString(fmt.Sprintf("on_server_keepalive=%s\n", z.Hook.OnServerKeepalive))
	sb.WriteString(fmt.Sprintf("on_send_rtp_stopped=%s\n", z.Hook.OnSendRTPStopped))
	sb.WriteString(fmt.Sprintf("on_rtp_server_timeout=%s\n", z.Hook.OnRTPServerTimeout))
	sb.WriteString(fmt.Sprintf("timeoutSec=%d\n", z.Hook.TimeoutSec))
	sb.WriteString(fmt.Sprintf("alive_interval=%.1f\n", z.Hook.AliveInterval))
	sb.WriteString(fmt.Sprintf("retry=%d\n", z.Hook.Retry))
	sb.WriteString(fmt.Sprintf("retry_delay=%.1f\n\n", z.Hook.RetryDelay))

	// [cluster]
	sb.WriteString("[cluster]\n")
	sb.WriteString("origin_url=\n")
	sb.WriteString("timeout_sec=15\n")
	sb.WriteString("retry_count=3\n\n")

	// [http]
	sb.WriteString("[http]\n")
	sb.WriteString(fmt.Sprintf("charSet=%s\n", z.HTTP.CharSet))
	sb.WriteString(fmt.Sprintf("keepAliveSecond=%d\n", z.HTTP.KeepAliveSecond))
	sb.WriteString(fmt.Sprintf("maxReqSize=%d\n", z.HTTP.MaxReqSize))
	sb.WriteString(fmt.Sprintf("port=%d\n", z.HTTP.Port))
	sb.WriteString(fmt.Sprintf("rootPath=%s\n", z.HTTP.RootPath))
	sb.WriteString(fmt.Sprintf("sendBufSize=%d\n", z.HTTP.SendBufSize))
	sb.WriteString(fmt.Sprintf("sslport=%d\n", z.HTTP.SSLPort))
	sb.WriteString(fmt.Sprintf("dirMenu=%d\n", boolToInt(z.HTTP.DirMenu)))
	sb.WriteString("virtualPath=\n")
	sb.WriteString("forbidCacheSuffix=\n")
	sb.WriteString("forwarded_ip_header=\n")
	sb.WriteString(fmt.Sprintf("allow_cross_domains=%d\n", boolToInt(z.HTTP.AllowCrossDomains)))
	sb.WriteString(fmt.Sprintf("allow_ip_range=%s\n\n", z.HTTP.AllowIPRange))

	// [multicast]
	sb.WriteString("[multicast]\n")
	sb.WriteString("addrMax=239.255.255.255\n")
	sb.WriteString("addrMin=239.0.0.0\n")
	sb.WriteString("udpTTL=64\n\n")

	// [record]
	sb.WriteString("[record]\n")
	sb.WriteString(fmt.Sprintf("appName=%s\n", z.Record.AppName))

	// 设置录像文件路径（如果配置了独立录像目录）
	// 必须使用绝对路径，否则会相对于ZLM工作目录
	if z.Record.RecordPath != "" {
		recordPath := z.Record.RecordPath
		// 将相对路径转换为绝对路径
		if !strings.HasPrefix(recordPath, "/") {
			if absPath, err := filepath.Abs(recordPath); err == nil {
				recordPath = absPath
			}
		}
		sb.WriteString(fmt.Sprintf("filePath=%s\n", recordPath))
	}

	sb.WriteString(fmt.Sprintf("fileBufSize=%d\n", z.Record.FileBufSize))
	sb.WriteString(fmt.Sprintf("sampleMS=%d\n", z.Record.SampleMS))
	sb.WriteString(fmt.Sprintf("fastStart=%d\n", boolToInt(z.Record.FastStart)))
	sb.WriteString(fmt.Sprintf("fileRepeat=%d\n", boolToInt(z.Record.FileRepeat)))
	sb.WriteString(fmt.Sprintf("enableFmp4=%d\n", boolToInt(z.Record.EnableFmp4)))

	// 录像分割配置
	if z.Record.FileSecond > 0 {
		sb.WriteString(fmt.Sprintf("fileSecond=%d\n", z.Record.FileSecond))
	}
	if z.Record.FileSizeMB > 0 {
		// ZLM使用字节为单位
		sb.WriteString(fmt.Sprintf("fileSizeMB=%d\n", z.Record.FileSizeMB))
	}

	// 视频编码转码配置
	if z.Record.EnableVideoCodec {
		sb.WriteString(fmt.Sprintf("enable_video_codec=%d\n", 1))
		sb.WriteString(fmt.Sprintf("video_codec=%s\n", z.Record.VideoCodec))
		if z.Record.VideoBitrate > 0 {
			sb.WriteString(fmt.Sprintf("video_bitrate=%d\n", z.Record.VideoBitrate))
		}
	}
	sb.WriteString("\n")

	// [rtmp]
	sb.WriteString("[rtmp]\n")
	sb.WriteString(fmt.Sprintf("handshakeSecond=%d\n", z.RTMP.HandshakeSecond))
	sb.WriteString(fmt.Sprintf("keepAliveSecond=%d\n", z.RTMP.KeepAliveSecond))
	sb.WriteString(fmt.Sprintf("port=%d\n", z.RTMP.Port))
	sb.WriteString(fmt.Sprintf("sslport=%d\n", z.RTMP.SSLPort))
	sb.WriteString(fmt.Sprintf("directProxy=%d\n", boolToInt(z.RTMP.DirectProxy)))
	sb.WriteString(fmt.Sprintf("enhanced=%d\n\n", boolToInt(z.RTMP.Enhanced)))

	// [rtp]
	sb.WriteString("[rtp]\n")
	sb.WriteString(fmt.Sprintf("audioMtuSize=%d\n", z.RTP.AudioMtuSize))
	sb.WriteString(fmt.Sprintf("videoMtuSize=%d\n", z.RTP.VideoMtuSize))
	sb.WriteString(fmt.Sprintf("rtpMaxSize=%d\n", z.RTP.RTPMaxSize))
	sb.WriteString(fmt.Sprintf("lowLatency=%d\n", boolToInt(z.RTP.LowLatency)))
	sb.WriteString(fmt.Sprintf("h264_stap_a=%d\n\n", boolToInt(z.RTP.H264StapA)))

	// [rtp_proxy]
	sb.WriteString("[rtp_proxy]\n")
	sb.WriteString("dumpDir=\n")
	sb.WriteString(fmt.Sprintf("port=%d\n", z.RTPProxy.Port))
	sb.WriteString(fmt.Sprintf("timeoutSec=%d\n", z.RTPProxy.TimeoutSec))
	sb.WriteString(fmt.Sprintf("port_range=%s\n", z.RTPProxy.PortRange))
	sb.WriteString(fmt.Sprintf("h264_pt=%d\n", z.RTPProxy.H264PT))
	sb.WriteString(fmt.Sprintf("h265_pt=%d\n", z.RTPProxy.H265PT))
	sb.WriteString(fmt.Sprintf("ps_pt=%d\n", z.RTPProxy.PSPT))
	sb.WriteString(fmt.Sprintf("opus_pt=%d\n", z.RTPProxy.OpusPT))
	sb.WriteString(fmt.Sprintf("gop_cache=%d\n", z.RTPProxy.GOPCache))
	sb.WriteString(fmt.Sprintf("rtp_g711_dur_ms=%d\n", z.RTPProxy.RTPG711DurMS))
	sb.WriteString(fmt.Sprintf("udp_recv_socket_buffer=%d\n", z.RTPProxy.UDPRecvSocketBuffer))
	sb.WriteString(fmt.Sprintf("merge_frame=%d\n\n", boolToInt(z.RTPProxy.MergeFrame)))

	// [rtc]
	sb.WriteString("[rtc]\n")
	sb.WriteString(fmt.Sprintf("port=%d\n", z.RTC.Port))
	sb.WriteString(fmt.Sprintf("tcpPort=%d\n", z.RTC.TCPPort))
	sb.WriteString(fmt.Sprintf("timeoutSec=%d\n", z.RTC.TimeoutSec))
	sb.WriteString(fmt.Sprintf("externIP=%s\n", z.RTC.ExternIP))
	sb.WriteString(fmt.Sprintf("interfaces=%s\n", z.RTC.Interfaces))
	sb.WriteString(fmt.Sprintf("rembBitRate=%d\n", z.RTC.RembBitRate))
	sb.WriteString(fmt.Sprintf("preferredCodecA=%s\n", z.RTC.PreferredCodecA))
	sb.WriteString(fmt.Sprintf("preferredCodecV=%s\n", z.RTC.PreferredCodecV))
	sb.WriteString("start_bitrate=0\n")
	sb.WriteString("max_bitrate=0\n")
	sb.WriteString("min_bitrate=0\n")
	sb.WriteString(fmt.Sprintf("maxRtpCacheMS=%d\n", z.RTC.MaxRtpCacheMS))
	sb.WriteString(fmt.Sprintf("maxRtpCacheSize=%d\n", z.RTC.MaxRtpCacheSize))
	sb.WriteString(fmt.Sprintf("nackMaxSize=%d\n", z.RTC.NackMaxSize))
	sb.WriteString(fmt.Sprintf("nackMaxMS=%d\n", z.RTC.NackMaxMS))
	sb.WriteString(fmt.Sprintf("nackMaxCount=%d\n", z.RTC.NackMaxCount))
	sb.WriteString(fmt.Sprintf("nackIntervalRatio=%.1f\n", z.RTC.NackIntervalRatio))
	sb.WriteString(fmt.Sprintf("nackRtpSize=%d\n", z.RTC.NackRtpSize))
	sb.WriteString(fmt.Sprintf("bfilter=%d\n\n", boolToInt(z.RTC.BFilter)))

	// [srt]
	sb.WriteString("[srt]\n")
	sb.WriteString(fmt.Sprintf("timeoutSec=%d\n", z.SRT.TimeoutSec))
	sb.WriteString(fmt.Sprintf("port=%d\n", z.SRT.Port))
	sb.WriteString(fmt.Sprintf("latencyMul=%d\n", z.SRT.LatencyMul))
	sb.WriteString(fmt.Sprintf("pktBufSize=%d\n", z.SRT.PktBufSize))
	sb.WriteString(fmt.Sprintf("passPhrase=%s\n\n", z.SRT.PassPhrase))

	// [rtsp]
	sb.WriteString("[rtsp]\n")
	sb.WriteString(fmt.Sprintf("authBasic=%d\n", boolToInt(z.RTSP.AuthBasic)))
	sb.WriteString(fmt.Sprintf("directProxy=%d\n", boolToInt(z.RTSP.DirectProxy)))
	sb.WriteString(fmt.Sprintf("handshakeSecond=%d\n", z.RTSP.HandshakeSecond))
	sb.WriteString(fmt.Sprintf("keepAliveSecond=%d\n", z.RTSP.KeepAliveSecond))
	sb.WriteString(fmt.Sprintf("port=%d\n", z.RTSP.Port))
	sb.WriteString(fmt.Sprintf("sslport=%d\n", z.RTSP.SSLPort))
	sb.WriteString(fmt.Sprintf("lowLatency=%d\n", boolToInt(z.RTSP.LowLatency)))
	sb.WriteString(fmt.Sprintf("rtpTransportType=%d\n\n", z.RTSP.RTPTransportType))

	// [shell]
	sb.WriteString("[shell]\n")
	sb.WriteString(fmt.Sprintf("maxReqSize=%d\n", z.Shell.MaxReqSize))
	sb.WriteString(fmt.Sprintf("port=%d\n\n", z.Shell.Port))

	return sb.String()
}

// GetHTTPPort 获取 HTTP 端口
func (z *ZLMConfig) GetHTTPPort() int {
	if z.HTTP != nil {
		return z.HTTP.Port
	}
	return 80
}

// GetRTMPPort 获取 RTMP 端口
func (z *ZLMConfig) GetRTMPPort() int {
	if z.RTMP != nil {
		return z.RTMP.Port
	}
	return 1935
}

// GetRTSPPort 获取 RTSP 端口
func (z *ZLMConfig) GetRTSPPort() int {
	if z.RTSP != nil {
		return z.RTSP.Port
	}
	return 554
}

// GetSecret 获取 API 密钥
func (z *ZLMConfig) GetSecret() string {
	if z.API != nil {
		return z.API.Secret
	}
	return ""
}

// Save 将配置保存到文件
func (c *Config) Save(filePath string) error {
	// 将配置转换为YAML格式
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	// 写入配置文件
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return err
	}

	log.Printf("配置保存成功: %s", filePath)
	return nil
}

// Validate 验证配置的正确性
// 返回警告信息列表和错误信息列表
// 错误表示配置无法启动，警告表示可能的问题但不影响启动
func (c *Config) Validate() (warnings []string, errors []string) {
	if c.ZLM != nil {
		// 检查 API 密钥
		if c.ZLM.API == nil || c.ZLM.API.Secret == "" {
			warnings = append(warnings, "ZLM API Secret 未配置，安全性降低")
		}

		// 检查关键端口是否有效
		if c.ZLM.HTTP != nil && c.ZLM.HTTP.Port <= 0 || c.ZLM.HTTP.Port > 65535 {
			errors = append(errors, fmt.Sprintf("ZLM HTTP 端口无效: %d", c.ZLM.HTTP.Port))
		}
		if c.ZLM.RTMP != nil && (c.ZLM.RTMP.Port <= 0 || c.ZLM.RTMP.Port > 65535) {
			errors = append(errors, fmt.Sprintf("ZLM RTMP 端口无效: %d", c.ZLM.RTMP.Port))
		}
		if c.ZLM.RTSP != nil && (c.ZLM.RTSP.Port <= 0 || c.ZLM.RTSP.Port > 65535) {
			errors = append(errors, fmt.Sprintf("ZLM RTSP 端口无效: %d", c.ZLM.RTSP.Port))
		}
		if c.ZLM.RTPProxy != nil && (c.ZLM.RTPProxy.Port <= 0 || c.ZLM.RTPProxy.Port > 65535) {
			errors = append(errors, fmt.Sprintf("ZLM RTP Proxy 端口无效: %d", c.ZLM.RTPProxy.Port))
		}

		// 检查 FFmpeg 路径是否存在
		if c.ZLM.FFmpeg != nil && c.ZLM.FFmpeg.Bin != "" {
			if _, err := os.Stat(c.ZLM.FFmpeg.Bin); err != nil {
				warnings = append(warnings, fmt.Sprintf("FFmpeg 不在指定路径: %s", c.ZLM.FFmpeg.Bin))
			}
		}

		// 检查录制路径是否有效
		if c.ZLM.Record != nil && c.ZLM.Record.RecordPath != "" {
			if _, err := os.Stat(c.ZLM.Record.RecordPath); os.IsNotExist(err) {
				// 路径不存在，尝试创建
				if err := os.MkdirAll(c.ZLM.Record.RecordPath, 0755); err != nil {
					warnings = append(warnings, fmt.Sprintf("无法创建录制目录: %s", c.ZLM.Record.RecordPath))
				}
			}
		}
	}

	// 检查 GB28181 配置
	if c.GB28181 != nil {
		if c.GB28181.SipPort <= 0 || c.GB28181.SipPort > 65535 {
			errors = append(errors, fmt.Sprintf("GB28181 SIP 端口无效: %d", c.GB28181.SipPort))
		}
		if c.GB28181.Realm == "" {
			errors = append(errors, "GB28181 Realm 未配置")
		}
		if c.GB28181.ServerID == "" {
			errors = append(errors, "GB28181 ServerID 未配置")
		}
	}

	// 检查 API 配置
	if c.API != nil {
		if c.API.Port <= 0 || c.API.Port > 65535 {
			errors = append(errors, fmt.Sprintf("API 端口无效: %d", c.API.Port))
		}
		if c.API.StaticDir == "" {
			c.API.StaticDir = "www" // 自动修复
		}
	}

	// 检查日志目录
	if c.Debug != nil && c.Debug.LogFile != "" {
		logDir := filepath.Dir(c.Debug.LogFile)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			warnings = append(warnings, fmt.Sprintf("无法创建日志目录: %s", logDir))
		}
	}

	return warnings, errors
}
