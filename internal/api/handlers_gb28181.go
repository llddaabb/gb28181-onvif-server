package api

import (
	"encoding/json"
	"fmt"
	"gb28181-onvif-server/internal/debug"
	"net/http"
	"strings"
	"time"

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

	res, err := s.previewManager.StartRTSPProxy(channelID, testStreamURL, app, zlmHost, httpPort, rtmpPort, "", "")
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

// handleGB28181QueryRecordInfo 查询GB28181设备录像
func (s *Server) handleGB28181QueryRecordInfo(w http.ResponseWriter, r *http.Request) {
	channelID := r.URL.Query().Get("channelId")
	startTime := r.URL.Query().Get("startTime")
	endTime := r.URL.Query().Get("endTime")
	recordType := r.URL.Query().Get("type") // all/time/alarm/manual

	if channelID == "" {
		respondBadRequest(w, "通道ID不能为空")
		return
	}

	// 如果没有指定时间，默认查询当天
	if startTime == "" || endTime == "" {
		now := time.Now()
		if startTime == "" {
			startTime = now.Format("2006-01-02") + "T00:00:00"
		}
		if endTime == "" {
			endTime = now.Format("2006-01-02") + "T23:59:59"
		}
	}

	if recordType == "" {
		recordType = "all"
	}

	// 清空该通道的旧缓存，以获取最新数据
	s.gb28181Server.ClearRecordCache(channelID)

	// 发送录像查询请求到设备
	if err := s.gb28181Server.QueryRecordInfo(channelID, startTime, endTime, recordType); err != nil {
		respondInternalError(w, fmt.Sprintf("发送录像查询失败: %v", err))
		return
	}

	// GB28181录像查询是异步的，等待设备响应（最多等待3秒）
	// 设备会通过MESSAGE返回录像列表
	var records []interface{}
	maxWaitTime := 3000 // 毫秒
	pollInterval := 100 // 毫秒
	elapsed := 0

	for elapsed < maxWaitTime {
		recordList := s.gb28181Server.GetRecordList(channelID)
		if len(recordList) > 0 {
			// 有结果，立即返回
			for _, r := range recordList {
				records = append(records, r)
			}
			break
		}
		time.Sleep(time.Duration(pollInterval) * time.Millisecond)
		elapsed += pollInterval
	}

	// 返回查询结果（即使为空）
	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"channelId": channelID,
		"count":     len(records),
		"records":   records,
		"startTime": startTime,
		"endTime":   endTime,
		"type":      recordType,
	})
}

// handleGB28181GetRecordList 获取GB28181设备录像列表（从缓存获取）
func (s *Server) handleGB28181GetRecordList(w http.ResponseWriter, r *http.Request) {
	channelID := r.URL.Query().Get("channelId")
	if channelID == "" {
		respondBadRequest(w, "通道ID不能为空")
		return
	}

	// 从缓存获取录像列表
	records := s.gb28181Server.GetRecordList(channelID)

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"channelId": channelID,
		"count":     len(records),
		"records":   records,
	})
}

// handleGB28181ClearRecordCache 清除GB28181设备录像缓存
func (s *Server) handleGB28181ClearRecordCache(w http.ResponseWriter, r *http.Request) {
	channelID := r.URL.Query().Get("channelId")
	if channelID == "" {
		respondBadRequest(w, "通道ID不能为空")
		return
	}

	s.gb28181Server.ClearRecordCache(channelID)

	respondSuccessMsg(w, "录像缓存已清除")
}

// handleGB28181RecordPlayback 请求GB28181设备端录像回放
// 通过 INVITE 请求设备将录像以 RTP 流方式直接发送到 ZLM，然后通过 ZLM 转为 FLV 流
func (s *Server) handleGB28181RecordPlayback(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChannelID string `json:"channelId"`
		StartTime string `json:"startTime"` // 格式: 2025-12-23T00:00:00
		EndTime   string `json:"endTime"`   // 格式: 2025-12-23T01:00:00
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, "无效的请求参数")
		return
	}

	if req.ChannelID == "" {
		respondBadRequest(w, "通道ID不能为空")
		return
	}

	if req.StartTime == "" || req.EndTime == "" {
		respondBadRequest(w, "开始时间和结束时间不能为空")
		return
	}

	debug.Info("gb28181", "请求设备端录像回放: 通道=%s, 时间=%s ~ %s", req.ChannelID, req.StartTime, req.EndTime)

	// 生成流ID（与 StartRecordPlayback 保持一致的格式）
	streamID := fmt.Sprintf("%s_%d", req.ChannelID, time.Now().Unix())

	// 1. 先在 ZLM 打开 RTP 接收端口
	zlmClient := s.zlmServer.GetAPIClient()
	rtpInfo, err := zlmClient.OpenRtpServer(streamID, 0, 0)
	if err != nil {
		respondInternalError(w, fmt.Sprintf("打开ZLM RTP端口失败: %v", err))
		return
	}
	debug.Info("gb28181", "ZLM RTP端口已打开: stream=%s port=%d", streamID, rtpInfo.Port)

	// 2. 发起录像回放请求（通过 GB28181 INVITE）
	// 设备会直接将 RTP 流推送到 ZLM 的 RTP 接收端口
	playbackInfo, err := s.gb28181Server.StartRecordPlaybackWithPort(req.ChannelID, req.StartTime, req.EndTime, streamID, rtpInfo.Port)
	if err != nil {
		// 清理 ZLM 端口
		_ = zlmClient.CloseRtpServer(streamID)
		respondInternalError(w, fmt.Sprintf("请求录像回放失败: %v", err))
		return
	}

	// 设备现在直接推送到 ZLM，无需 FFmpeg 中转
	debug.Info("gb28181", "设备将 RTP 流推送到 ZLM 的接收端口: %d", rtpInfo.Port)

	// 获取 ZLM 配置信息用于构建播放地址
	zlmHost := s.getZLMHost(r)
	zlmHTTPPort, _, _ := s.getZLMPorts()

	// 构建播放地址 - 使用 ZLM HTTP FLV 地址
	// FLV 地址格式: http://host:port/rtp/stream_id.live.flv
	// 设备推送到 ZLM 后，ZLM 会自动转换为 HTTP FLV 可访问的地址
	directFlvURL := fmt.Sprintf("http://%s:%d/rtp/%s.live.flv", zlmHost, zlmHTTPPort, streamID)

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"message":   "录像回放已启动，请等待3-5秒后播放",
		"channelId": req.ChannelID,
		"startTime": req.StartTime,
		"endTime":   req.EndTime,
		"streamId":  streamID,
		"ssrc":      playbackInfo.SSRC,
		"flvUrl":    directFlvURL, // ZLM 直接提供的 FLV 地址
	})
}

// handleGB28181StopRecordPlayback 停止GB28181设备端录像回放
func (s *Server) handleGB28181StopRecordPlayback(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChannelID string `json:"channelId"`
		StreamID  string `json:"streamId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, "无效的请求参数")
		return
	}

	if req.ChannelID == "" && req.StreamID == "" {
		respondBadRequest(w, "通道ID或流ID不能为空")
		return
	}

	debug.Info("gb28181", "停止设备端录像回放: 通道=%s, 流=%s", req.ChannelID, req.StreamID)

	// 发送 BYE 停止回放
	if err := s.gb28181Server.StopRecordPlayback(req.ChannelID, req.StreamID); err != nil {
		respondInternalError(w, fmt.Sprintf("停止录像回放失败: %v", err))
		return
	}

	respondSuccessMsg(w, "录像回放已停止")
}

// handleStartGB28181Service 启动GB28181服务
func (s *Server) handleStartGB28181Service(w http.ResponseWriter, r *http.Request) {
	if s.gb28181Running {
		respondSuccessMsg(w, "GB28181服务已在运行中")
		return
	}

	if err := s.gb28181Server.Start(); err != nil {
		respondInternalError(w, fmt.Sprintf("启动GB28181服务失败: %v", err))
		return
	}

	s.gb28181Running = true
	debug.Info("gb28181", "GB28181服务已启动")
	respondSuccessMsg(w, "GB28181服务已启动")
}

// handleStopGB28181Service 停止GB28181服务
func (s *Server) handleStopGB28181Service(w http.ResponseWriter, r *http.Request) {
	if !s.gb28181Running {
		respondSuccessMsg(w, "GB28181服务已停止")
		return
	}

	if err := s.gb28181Server.Stop(); err != nil {
		respondInternalError(w, fmt.Sprintf("停止GB28181服务失败: %v", err))
		return
	}

	s.gb28181Running = false
	debug.Info("gb28181", "GB28181服务已停止")
	respondSuccessMsg(w, "GB28181服务已停止")
}

// handleDiagnoseRTPPlayback 诊断 GB28181 RTP 录像回放
func (s *Server) handleDiagnoseRTPPlayback(w http.ResponseWriter, r *http.Request) {
	debug.Info("gb28181", "开始诊断 RTP 录像回放")

	diagnosis := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"checks":    make([]map[string]interface{}, 0),
	}

	// 检查 ZLM 服务
	checks := diagnosis["checks"].([]map[string]interface{})

	zlmClient := s.zlmServer.GetAPIClient()
	if zlmClient == nil {
		checks = append(checks, map[string]interface{}{
			"name":   "ZLM API Client",
			"status": "FAIL",
			"reason": "ZLM API 客户端未初始化",
		})
	} else {
		checks = append(checks, map[string]interface{}{
			"name":   "ZLM API Client",
			"status": "OK",
			"info":   "ZLM API 客户端已连接",
		})
	}

	// 检查 RTP 服务器配置
	zlmHost := s.getZLMHost(r)
	zlmHTTPPort, zlmRTMPPort, _ := s.getZLMPorts()

	checks = append(checks, map[string]interface{}{
		"name":   "ZLM Network Configuration",
		"status": "OK",
		"info": map[string]interface{}{
			"http_port": zlmHTTPPort,
			"rtmp_port": zlmRTMPPort,
			"host":      zlmHost,
		},
	})

	// 测试打开和关闭 RTP 服务器
	testStreamID := fmt.Sprintf("diagnostic_%d", time.Now().Unix())
	rtpInfo, err := zlmClient.OpenRtpServer(testStreamID, 0, 0)
	if err != nil {
		checks = append(checks, map[string]interface{}{
			"name":   "RTP Server Open",
			"status": "FAIL",
			"reason": fmt.Sprintf("打开 RTP 服务器失败: %v", err),
		})
	} else {
		checks = append(checks, map[string]interface{}{
			"name":   "RTP Server Open",
			"status": "OK",
			"info": map[string]interface{}{
				"allocated_port": rtpInfo.Port,
				"stream_id":      rtpInfo.StreamID,
			},
		})

		// 尝试列出 RTP 服务器
		servers, err := zlmClient.ListRtpServer()
		if err != nil {
			checks = append(checks, map[string]interface{}{
				"name":   "List RTP Servers",
				"status": "WARN",
				"reason": fmt.Sprintf("列出 RTP 服务器失败: %v", err),
			})
		} else {
			checks = append(checks, map[string]interface{}{
				"name":   "List RTP Servers",
				"status": "OK",
				"info": map[string]interface{}{
					"server_count": len(servers),
					"servers":      servers,
				},
			})
		}

		// 关闭测试 RTP 服务器
		if err := zlmClient.CloseRtpServer(testStreamID); err != nil {
			checks = append(checks, map[string]interface{}{
				"name":   "RTP Server Close",
				"status": "WARN",
				"reason": fmt.Sprintf("关闭 RTP 服务器失败: %v", err),
			})
		} else {
			checks = append(checks, map[string]interface{}{
				"name":   "RTP Server Close",
				"status": "OK",
				"info":   "RTP 服务器已关闭",
			})
		}
	}

	// 诊断建议
	recommendations := []string{
		"✓ 确保 GB28181 设备已注册并在线",
		"✓ 查询设备录像列表，确认有可用的录像文件",
		"✓ 发送录像回放请求时，等待 3-5 秒让设备建立 RTP 连接",
		"✓ 检查防火墙是否阻止 RTP 端口（10000 及以上范围）",
		"✓ 验证设备网络设置，确保能连接到本服务器的 RTP 接收端口",
		"✓ 如果仍无法播放，查看 ZLM 日志了解 RTP 接收情况",
	}

	diagnosis["checks"] = checks
	diagnosis["recommendations"] = recommendations
	diagnosis["rtp_port_range"] = "10000-35000"

	respondRaw(w, http.StatusOK, diagnosis)
}
