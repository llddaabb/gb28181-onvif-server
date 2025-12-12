package api

import (
	"encoding/json"
	"fmt"
	"gb28181-onvif-server/internal/debug"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

// handleGetGB28181Devices 获取GB28181设备列表
func (s *Server) handleGetGB28181Devices(w http.ResponseWriter, r *http.Request) {
	devices := s.gb28181Server.GetDevices()
	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"devices": devices,
	})
}

// handleGetGB28181Device 获取单个GB28181设备信息
func (s *Server) handleGetGB28181Device(w http.ResponseWriter, r *http.Request) {
	deviceID := mux.Vars(r)["id"]
	device, exists := s.gb28181Server.GetDeviceByID(deviceID)
	if !exists {
		respondNotFound(w, "设备不存在")
		return
	}
	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"device":  device,
	})
}

// handleRemoveGB28181Device 移除GB28181设备
func (s *Server) handleRemoveGB28181Device(w http.ResponseWriter, r *http.Request) {
	deviceID := mux.Vars(r)["id"]
	if ok := s.gb28181Server.RemoveDevice(deviceID); !ok {
		respondInternalError(w, "删除设备失败")
		return
	}
	respondSuccessMsg(w, "设备已删除")
}

// handleGetGB28181Channels 获取设备通道列表
func (s *Server) handleGetGB28181Channels(w http.ResponseWriter, r *http.Request) {
	deviceID := mux.Vars(r)["id"]
	channels := s.gb28181Server.GetChannels(deviceID)
	if channels == nil {
		respondNotFound(w, "设备不存在")
		return
	}
	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"channels": channels,
	})
}

// handleRefreshGB28181Device 刷新设备信息和通道列表
func (s *Server) handleRefreshGB28181Device(w http.ResponseWriter, r *http.Request) {
	deviceID := mux.Vars(r)["id"]

	if err := s.gb28181Server.QueryDeviceInfo(deviceID); err != nil {
		respondNotFound(w, err.Error())
		return
	}

	if err := s.gb28181Server.QueryCatalog(deviceID); err != nil {
		respondInternalError(w, err.Error())
		return
	}

	respondSuccessMsg(w, "已发送设备信息和目录查询请求")
}

// handleGetGB28181Statistics 获取GB28181统计信息
func (s *Server) handleGetGB28181Statistics(w http.ResponseWriter, r *http.Request) {
	stats := s.gb28181Server.GetStatistics()
	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"statistics": stats,
	})
}

// handleGetGB28181ServerConfig 获取GB28181服务器配置
func (s *Server) handleGetGB28181ServerConfig(w http.ResponseWriter, r *http.Request) {
	serverConfig := map[string]interface{}{
		"sip_ip":             s.config.GB28181.SipIP,
		"sip_port":           s.config.GB28181.SipPort,
		"realm":              s.config.GB28181.Realm,
		"server_id":          s.config.GB28181.ServerID,
		"heartbeat_interval": s.config.GB28181.HeartbeatInterval,
		"register_expires":   s.config.GB28181.RegisterExpires,
		"auth_enabled":       s.config.GB28181.Password != "",
	}
	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"config":  serverConfig,
	})
}

// handleUpdateGB28181ServerConfig 更新GB28181服务器配置
func (s *Server) handleUpdateGB28181ServerConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SipIP           string `json:"sip_ip"`
		SipPort         int    `json:"sip_port"`
		Realm           string `json:"realm"`
		ServerID        string `json:"server_id"`
		Password        string `json:"password"`
		RegisterExpires int    `json:"register_expires"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, "无效的请求数据")
		return
	}

	// 更新配置
	if req.SipIP != "" {
		s.config.GB28181.SipIP = req.SipIP
	}
	if req.SipPort > 0 {
		s.config.GB28181.SipPort = req.SipPort
	}
	if req.Realm != "" {
		s.config.GB28181.Realm = req.Realm
	}
	if req.ServerID != "" {
		s.config.GB28181.ServerID = req.ServerID
	}
	s.config.GB28181.Password = req.Password
	if req.RegisterExpires > 0 {
		s.config.GB28181.RegisterExpires = req.RegisterExpires
	}

	if err := s.config.Save(s.configPath); err != nil {
		respondInternalError(w, fmt.Sprintf("保存配置失败: %v", err))
		return
	}

	respondSuccessMsg(w, "配置已保存，需要重启服务器生效")
}

// handleGB28181Catalog 触发GB28181设备目录查询
func (s *Server) handleGB28181Catalog(w http.ResponseWriter, r *http.Request) {
	deviceID := mux.Vars(r)["id"]
	if deviceID == "" {
		respondBadRequest(w, "设备ID不能为空")
		return
	}

	device, exists := s.gb28181Server.GetDeviceByID(deviceID)
	if !exists {
		respondNotFound(w, "设备不存在")
		return
	}

	if device.Status != "online" {
		respondBadRequest(w, "设备离线，无法查询目录")
		return
	}

	if err := s.gb28181Server.SendCatalogQuery(deviceID); err != nil {
		respondInternalError(w, fmt.Sprintf("发送目录查询失败: %v", err))
		return
	}

	respondSuccessMsg(w, "目录查询已发送，请等待设备响应后刷新通道列表")
}

// handleGB28181PTZ 处理GB28181 PTZ控制
func (s *Server) handleGB28181PTZ(w http.ResponseWriter, r *http.Request) {
	deviceID := mux.Vars(r)["id"]

	var req struct {
		Command   string `json:"command"`
		ChannelID string `json:"channelId"`
		DeviceID  string `json:"deviceId"`
		Speed     int    `json:"speed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, "无效的请求参数")
		return
	}

	if req.Speed == 0 {
		req.Speed = 128
	}
	if req.ChannelID == "" {
		req.ChannelID = deviceID
	}

	if err := s.gb28181Server.SendPTZCommand(req.DeviceID, req.ChannelID, req.Command, req.Speed); err != nil {
		respondInternalError(w, fmt.Sprintf("PTZ控制失败: %v", err))
		return
	}

	respondSuccessMsg(w, "PTZ命令已发送")
}

// handleDiscoverGB28181Devices GB28181设备发现
func (s *Server) handleDiscoverGB28181Devices(w http.ResponseWriter, r *http.Request) {
	respondSuccessMsg(w, "设备发现请求已接收，请等待设备主动注册")
}

// handleStartGB28181Preview 启动GB28181设备预览
func (s *Server) handleStartGB28181Preview(w http.ResponseWriter, r *http.Request) {
	debug.Info("api", "处理启动GB28181预览请求")

	deviceID := mux.Vars(r)["id"]

	var req struct {
		ChannelID string `json:"channelId"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.ChannelID == "" {
		req.ChannelID = deviceID
	}

	if deviceID == "" {
		respondBadRequest(w, "设备ID不能为空")
		return
	}

	device, exists := s.gb28181Server.GetDeviceByID(deviceID)
	if !exists {
		respondNotFound(w, "设备不存在")
		return
	}

	if device.Status != "online" {
		respondBadRequest(w, "设备离线，无法预览")
		return
	}

	if !s.checkZLMAvailable(w) {
		return
	}

	app := "rtp"
	zlmHost := s.getZLMHost(r)
	httpPort, rtmpPort, _ := s.getZLMPorts()

	// 使用 PreviewManager 统一处理预览逻辑
	if s.previewManager == nil {
		respondInternalError(w, "preview manager 未初始化")
		return
	}

	res, err := s.previewManager.StartChannelPreview(deviceID, req.ChannelID, app, zlmHost, httpPort, rtmpPort)
	if err != nil {
		respondInternalError(w, fmt.Sprintf("启动预览失败: %v", err))
		return
	}

	urls := s.buildStreamURLs(r, app, res.StreamID)

	debug.Info("api", "GB28181预览已启动: device=%s, channel=%s, stream=%s, port=%d, ssrc=%s", deviceID, req.ChannelID, res.StreamID, res.RTPPort, res.SSRC)
	debug.Info("api", "流地址: FLV=%s, HLS=%s", urls.FlvURL, urls.HlsURL)

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "预览启动成功，等待设备推流（约3-5秒）",
		"data": map[string]interface{}{
			"device_id":  deviceID,
			"channel_id": req.ChannelID,
			"stream_id":  res.StreamID,
			"rtp_port":   res.RTPPort,
			"ssrc":       res.SSRC,
			"status":     "inviting",
			"flv_url":    urls.FlvURL,
			"ws_flv_url": urls.WsFlvURL,
			"hls_url":    urls.HlsURL,
			"rtmp_url":   res.RtmpURL,
			"tip":        "请等待3-5秒后再播放，设备需要时间建立RTP连接",
		},
	})
}

// handleStopGB28181Preview 停止GB28181设备预览
func (s *Server) handleStopGB28181Preview(w http.ResponseWriter, r *http.Request) {
	debug.Info("api", "处理停止GB28181预览请求")

	deviceID := mux.Vars(r)["id"]

	var req struct {
		ChannelID string `json:"channelId"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.ChannelID == "" {
		req.ChannelID = deviceID
	}

	streamID := strings.ReplaceAll(req.ChannelID, "-", "")

	// 优先使用 PreviewManager 停止并清理
	if s.previewManager != nil {
		if err := s.previewManager.StopChannelPreview(deviceID, req.ChannelID); err != nil {
			debug.Warn("api", "停止预览失败: %v", err)
			// 如果 manager 停止失败，继续尝试底层回退
			if err := s.gb28181Server.ByeRequest(deviceID, req.ChannelID); err != nil {
				debug.Warn("api", "发送BYE失败: %v", err)
			}
			if s.zlmServer != nil && s.zlmServer.GetAPIClient() != nil {
				s.zlmServer.GetAPIClient().CloseRtpServer(streamID)
				s.zlmServer.GetAPIClient().CloseStream("rtp", streamID)
			}
		}
	} else {
		// 兜底逻辑
		if err := s.gb28181Server.ByeRequest(deviceID, req.ChannelID); err != nil {
			debug.Warn("api", "发送BYE失败: %v", err)
		}
		if s.zlmServer != nil && s.zlmServer.GetAPIClient() != nil {
			s.zlmServer.GetAPIClient().CloseRtpServer(streamID)
			s.zlmServer.GetAPIClient().CloseStream("rtp", streamID)
		}
	}

	respondSuccessMsg(w, "预览已停止")
}

// handleStartGB28181ChannelPreview 启动GB28181设备指定通道预览
func (s *Server) handleStartGB28181ChannelPreview(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	deviceID := vars["id"]
	channelID := vars["channelId"]

	debug.Info("api", "处理启动GB28181通道预览请求: device=%s, channel=%s", deviceID, channelID)

	if deviceID == "" || channelID == "" {
		respondBadRequest(w, "设备ID和通道ID不能为空")
		return
	}

	device, exists := s.gb28181Server.GetDeviceByID(deviceID)
	if !exists {
		respondNotFound(w, "设备不存在")
		return
	}

	if device.Status != "online" {
		respondBadRequest(w, "设备离线，无法预览")
		return
	}

	if !s.checkZLMAvailable(w) {
		return
	}

	// 使用 preview.Manager 统一处理 channel 级别的 GB28181 预览
	app := "rtp"
	res, err := s.startPreview(r, deviceID, channelID, "", app)
	if err != nil {
		errMsg := fmt.Sprintf("启动预览失败: %v", err)
		// 特别处理会话已存在的错误，提供更友好的提示
		if strings.Contains(err.Error(), "会话已存在") {
			errMsg = "预览会话已存在或正在启动中，请稍后再试"
		}
		respondInternalError(w, errMsg)
		return
	}

	respondSuccessData(w, res, "预览启动中，等待设备推流")
}

// handleStopGB28181ChannelPreview 停止GB28181设备指定通道预览
func (s *Server) handleStopGB28181ChannelPreview(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	deviceID := vars["id"]
	channelID := vars["channelId"]

	if deviceID == "" || channelID == "" {
		respondBadRequest(w, "设备ID和通道ID不能为空")
		return
	}

	streamID := strings.ReplaceAll(channelID, "-", "")

	// 优先使用 preview.Manager 停止通道预览
	if s.previewManager != nil {
		if err := s.previewManager.StopChannelPreview(deviceID, channelID); err != nil {
			debug.Warn("api", "停止预览失败: %v", err)
			// 回退清理
			if err := s.gb28181Server.ByeRequest(deviceID, channelID); err != nil {
				debug.Warn("api", "发送BYE失败: %v", err)
			}
			if s.zlmServer != nil && s.zlmServer.GetAPIClient() != nil {
				s.zlmServer.GetAPIClient().CloseRtpServer(streamID)
				s.zlmServer.GetAPIClient().CloseStream("rtp", streamID)
			}
		}
	} else {
		if err := s.gb28181Server.ByeRequest(deviceID, channelID); err != nil {
			debug.Warn("api", "发送BYE失败: %v", err)
		}
		if s.zlmServer != nil && s.zlmServer.GetAPIClient() != nil {
			s.zlmServer.GetAPIClient().CloseRtpServer(streamID)
			s.zlmServer.GetAPIClient().CloseStream("rtp", streamID)
		}
	}

	respondSuccessMsg(w, "预览已停止")
}

// handleTestGB28181ChannelPreview 测试GB28181通道预览（使用公共测试流）
func (s *Server) handleTestGB28181ChannelPreview(w http.ResponseWriter, r *http.Request) {
	debug.Info("api", "处理测试预览请求（使用流代理）")

	vars := mux.Vars(r)
	deviceID := vars["id"]
	channelID := vars["channelId"]

	if deviceID == "" || channelID == "" {
		respondBadRequest(w, "设备ID和通道ID不能为空")
		return
	}

	if !s.checkZLMAvailable(w) {
		return
	}

	// 使用 preview.Manager 启动测试流代理
	app := "live"
	zlmHost := s.getZLMHost(r)
	httpPort, rtmpPort, _ := s.getZLMPorts()

	if s.previewManager == nil {
		respondInternalError(w, "preview manager 未初始化")
		return
	}

	testStreamURL := "rtmp://ns8.indexforce.com/home/mystream"

	res, err := s.previewManager.StartRTSPProxy(channelID, testStreamURL, app, zlmHost, httpPort, rtmpPort)
	if err != nil {
		debug.Error("api", "添加流代理失败: %v", err)
		respondInternalError(w, fmt.Sprintf("添加流代理失败: %v", err))
		return
	}

	// 检测流是否在线
	streamExists := false
	if s.zlmServer != nil && s.zlmServer.GetAPIClient() != nil {
		if online, err := s.zlmServer.GetAPIClient().IsStreamOnline(app, res.StreamID); err == nil && online {
			streamExists = true
		}
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "测试预览已启动",
		"exists":  streamExists,
		"data": map[string]interface{}{
			"device_id":  deviceID,
			"channel_id": channelID,
			"stream_id":  res.StreamID,
			"proxy_key":  "",
			"source_url": testStreamURL,
			"flv_url":    fmt.Sprintf("/zlm/%s/%s.live.flv", app, res.StreamID),
			"ws_flv_url": fmt.Sprintf("/zlm/%s/%s.live.flv", app, res.StreamID),
			"hls_url":    fmt.Sprintf("/zlm/%s/%s/hls.m3u8", app, res.StreamID),
			"rtmp_url":   res.RtmpURL,
		},
	})
}
