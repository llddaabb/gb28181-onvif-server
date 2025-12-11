package preview

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"gb28181-onvif-server/internal/debug"
	"gb28181-onvif-server/internal/gb28181"
	"gb28181-onvif-server/internal/mediautil"
	"gb28181-onvif-server/internal/zlm"
)

// PreviewResult 返回给调用者的预览信息
type PreviewResult struct {
	DeviceID  string
	ChannelID string
	StreamID  string
	RTPPort   int
	SSRC      string
	FlvURL    string
	WsFlvURL  string
	HlsURL    string
	RtmpURL   string
}

// Manager 负责封装预览开始/停止的逻辑
type Manager struct {
	gbServer *gb28181.Server
	zlm      *zlm.ZLMServer
}

// NewManager 创建新的预览管理器
func NewManager(gb *gb28181.Server, z *zlm.ZLMServer) *Manager {
	return &Manager{gbServer: gb, zlm: z}
}

// StartChannelPreview 为指定通道启动预览
// zlmHost/httpPort/rtmpPort 用于生成外部可访问的流地址
func (m *Manager) StartChannelPreview(deviceID, channelID, app, zlmHost string, httpPort, rtmpPort int) (*PreviewResult, error) {
	if m.zlm == nil || m.zlm.GetAPIClient() == nil {
		return nil, fmt.Errorf("zlm 未配置")
	}

	device, exists := m.gbServer.GetDeviceByID(deviceID)
	if !exists {
		return nil, fmt.Errorf("设备不存在")
	}

	zlmClient := m.zlm.GetAPIClient()
	streamID := strings.ReplaceAll(channelID, "-", "")

	// 判断 RTP 服务是否已存在
	rtpOnline, rtpPort, ssrc, err := zlmClient.IsRtpServerOnline(streamID)
	if err != nil {
		return nil, fmt.Errorf("检查RTP服务失败: %w", err)
	}
	if rtpOnline {
		debug.Info("preview", "RTP服务已存在: device=%s channel=%s stream=%s rtp=%d ssrc=%s", deviceID, channelID, streamID, rtpPort, ssrc)
		res := &PreviewResult{
			DeviceID:  deviceID,
			ChannelID: channelID,
			StreamID:  streamID,
			RTPPort:   rtpPort,
			SSRC:      ssrc,
			FlvURL:    fmt.Sprintf("/zlm/%s/%s.live.flv", app, streamID),
			WsFlvURL:  fmt.Sprintf("/zlm/%s/%s.live.flv", app, streamID),
			HlsURL:    fmt.Sprintf("/zlm/%s/%s/hls.m3u8", app, streamID),
			RtmpURL:   fmt.Sprintf("rtmp://%s:%d/%s/%s", zlmHost, rtmpPort, app, streamID),
		}
		return res, nil
	}

	// 打开RTP端口
	rtpInfo, err := zlmClient.OpenRtpServer(streamID, 0, 0)
	if err != nil {
		// 如果 ZLM 报 stream already exists，尝试查询现有 RTP 服务信息并返回，避免重复创建
		if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "This stream already exists") {
			debug.Warn("preview", "OpenRtpServer: stream already exists, try to query existing rtp server: %s", streamID)
			// 查询 RTP 列表，寻找匹配的 streamID
			if list, lerr := zlmClient.ListRtpServer(); lerr == nil {
				for _, item := range list {
					if sid, ok := item["stream_id"].(string); ok && sid == streamID {
						if portF, ok := item["port"].(float64); ok {
							port := int(portF)
							// ssrc may be present
							ssrc := ""
							if s, ok2 := item["ssrc"].(string); ok2 {
								ssrc = s
							}
							res := &PreviewResult{
								DeviceID:  deviceID,
								ChannelID: channelID,
								StreamID:  streamID,
								RTPPort:   port,
								SSRC:      ssrc,
								FlvURL:    fmt.Sprintf("/zlm/%s/%s.live.flv", app, streamID),
								WsFlvURL:  fmt.Sprintf("/zlm/%s/%s.live.flv", app, streamID),
								HlsURL:    fmt.Sprintf("/zlm/%s/%s/hls.m3u8", app, streamID),
								RtmpURL:   fmt.Sprintf("rtmp://%s:%d/%s/%s", zlmHost, rtmpPort, app, streamID),
							}
							return res, nil
						}
					}
				}
			} else {
				debug.Warn("preview", "ListRtpServer failed: %v", lerr)
			}
		}
		return nil, fmt.Errorf("打开RTP端口失败: %w", err)
	}

	// 选择媒体IP：优先通过UDP探测本地出口地址
	mediaIP := device.SipIP
	conn, err := net.Dial("udp", device.SipIP+":"+strconv.Itoa(device.SipPort))
	if err == nil {
		defer conn.Close()
		localAddr := conn.LocalAddr().String()
		if idx := strings.LastIndex(localAddr, ":"); idx > 0 {
			candidateIP := localAddr[:idx]
			if candidateIP != "" && candidateIP != "127.0.0.1" && candidateIP != "::1" {
				mediaIP = candidateIP
			}
		}
	}

	mediaSession, err := m.gbServer.InviteRequest(deviceID, channelID, rtpInfo.Port, mediaIP)
	if err != nil {
		// 清理ZLM端口
		_ = zlmClient.CloseRtpServer(streamID)
		return nil, fmt.Errorf("发送INVITE失败: %w", err)
	}

	debug.Info("preview", "预览启动: device=%s channel=%s stream=%s rtp=%d ssrc=%s", deviceID, channelID, streamID, rtpInfo.Port, mediaSession.SSRC)

	res := &PreviewResult{
		DeviceID:  deviceID,
		ChannelID: channelID,
		StreamID:  streamID,
		RTPPort:   rtpInfo.Port,
		SSRC:      mediaSession.SSRC,
		FlvURL:    fmt.Sprintf("/zlm/%s/%s.live.flv", app, streamID),
		WsFlvURL:  fmt.Sprintf("/zlm/%s/%s.live.flv", app, streamID),
		HlsURL:    fmt.Sprintf("/zlm/%s/%s/hls.m3u8", app, streamID),
		RtmpURL:   fmt.Sprintf("rtmp://%s:%d/%s/%s", zlmHost, rtmpPort, app, streamID),
	}

	// 若需要生成绝对http/ws地址，调用方可基于 httpPort/zlmHost 处理

	return res, nil
}

// StopChannelPreview 停止指定通道的预览（发送BYE并关闭ZLM资源）
func (m *Manager) StopChannelPreview(deviceID, channelID string) error {
	if m.zlm == nil || m.zlm.GetAPIClient() == nil {
		return fmt.Errorf("zlm 未配置")
	}

	streamID := strings.ReplaceAll(channelID, "-", "")

	if err := m.gbServer.ByeRequest(deviceID, channelID); err != nil {
		debug.Warn("preview", "发送BYE失败: %v", err)
	}

	client := m.zlm.GetAPIClient()
	if client != nil {
		if err := client.CloseRtpServer(streamID); err != nil {
			debug.Warn("preview", "关闭RTP服务失败: %v", err)
		}
		if err := client.CloseStream("rtp", streamID); err != nil {
			debug.Warn("preview", "关闭流失败: %v", err)
		}
	}

	return nil
}

// StartRTSPProxy 为指定设备启动 RTSP -> ZLM 流代理（用于 ONVIF 等场景）
func (m *Manager) StartRTSPProxy(deviceID, rtspURL, app, zlmHost string, httpPort, rtmpPort int) (*PreviewResult, error) {
	if m.zlm == nil || m.zlm.GetAPIClient() == nil {
		return nil, fmt.Errorf("zlm 未配置")
	}

	zlmClient := m.zlm.GetAPIClient()

	// 生成 stream id，尽量避免特殊字符
	streamID := strings.ReplaceAll(deviceID, "-", "_")
	streamID = strings.ReplaceAll(streamID, ":", "_")
	streamID = strings.ReplaceAll(streamID, ".", "_")

	// 如果流已存在，直接返回
	online, _ := zlmClient.IsStreamOnline(app, streamID)
	if online {
		res := &PreviewResult{
			DeviceID: deviceID,
			StreamID: streamID,
			FlvURL:   fmt.Sprintf("/zlm/%s/%s.live.flv", app, streamID),
			WsFlvURL: fmt.Sprintf("/zlm/%s/%s.live.flv", app, streamID),
			HlsURL:   fmt.Sprintf("/zlm/%s/%s/hls.m3u8", app, streamID),
			RtmpURL:  fmt.Sprintf("rtmp://%s:%d/%s/%s", zlmHost, rtmpPort, app, streamID),
		}
		return res, nil
	}

	// 首先尝试默认（tcp rtp_type）方式添加代理
	proxyInfo, err := zlmClient.AddStreamProxy(rtspURL, app, streamID)
	if err != nil {
		debug.Warn("preview", "AddStreamProxy default failed: %v, will try UDP fallback", err)
		// 如果报 already exists，直接返回现有信息
		if strings.Contains(err.Error(), "already exists") {
			res := &PreviewResult{
				DeviceID: deviceID,
				StreamID: streamID,
				FlvURL:   fmt.Sprintf("/zlm/%s/%s.live.flv", app, streamID),
				WsFlvURL: fmt.Sprintf("/zlm/%s/%s.live.flv", app, streamID),
				HlsURL:   fmt.Sprintf("/zlm/%s/%s/hls.m3u8", app, streamID),
				RtmpURL:  fmt.Sprintf("rtmp://%s:%d/%s/%s", zlmHost, rtmpPort, app, streamID),
			}
			go func(inURL, sid string) {
				if strings.HasPrefix(strings.ToLower(inURL), "rtsp://") || strings.HasPrefix(strings.ToLower(inURL), "rtmp://") || strings.HasPrefix(strings.ToLower(inURL), "http://") {
					codec, err := mediautil.DetectVideoCodec(inURL, 1500*time.Millisecond)
					if err != nil {
						debug.Info("preview", "probe codec failed for %s: %v", inURL, err)
						return
					}
					if codec == "hevc" || strings.Contains(strings.ToLower(codec), "h265") {
						rtmpTarget := fmt.Sprintf("rtmp://127.0.0.1:%d/%s/%s", rtmpPort, app, sid)
						if err := mediautil.StartFFmpegTranscode(sid, app, inURL, rtmpTarget, "", 0); err != nil {
							debug.Error("transcode", "启动转码失败: %v", err)
						} else {
							debug.Info("transcode", "已为代理流 %s 启动转码", sid)
						}
					}
				}
			}(rtspURL, streamID)
			return res, nil
		}

		// 回退：尝试使用 UDP rtp_type 并增加 retry_count
		opts := map[string]interface{}{"rtp_type": 1, "retry_count": 5, "timeout_sec": 30}
		proxyInfo, err = zlmClient.AddStreamProxyWithOptions(rtspURL, app, streamID, opts)
		if err != nil {
			return nil, fmt.Errorf("添加流代理失败: %w", err)
		}
	}

	res := &PreviewResult{
		DeviceID: deviceID,
		StreamID: streamID,
		FlvURL:   fmt.Sprintf("/zlm/%s/%s.live.flv", app, streamID),
		WsFlvURL: fmt.Sprintf("/zlm/%s/%s.live.flv", app, streamID),
		HlsURL:   fmt.Sprintf("/zlm/%s/%s/hls.m3u8", app, streamID),
		RtmpURL:  fmt.Sprintf("rtmp://%s:%d/%s/%s", zlmHost, rtmpPort, app, streamID),
	}

	// 可以在需要时把 proxyInfo.Key 返回给调用者
	_ = proxyInfo

	return res, nil
}

// StopRTSPProxy 停止 RTSP -> ZLM 的流代理（关闭 ZLM 流）
func (m *Manager) StopRTSPProxy(deviceID, app string) error {
	if m.zlm == nil || m.zlm.GetAPIClient() == nil {
		return fmt.Errorf("zlm 未配置")
	}

	streamID := strings.ReplaceAll(deviceID, "-", "_")
	streamID = strings.ReplaceAll(streamID, ":", "_")
	streamID = strings.ReplaceAll(streamID, ".", "_")

	client := m.zlm.GetAPIClient()
	if client != nil {
		if err := client.CloseStream(app, streamID); err != nil {
			debug.Warn("preview", "关闭流失败: %v", err)
		}
	}

	return nil
}
