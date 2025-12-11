package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"gb28181-onvif-server/internal/debug"
	"gb28181-onvif-server/internal/onvif"

	"github.com/gorilla/mux"
)

// previewSessions holds active preview sessions keyed by device ID.
var previewSessions = make(map[string]*PreviewSession)

// handleGetONVIFDevices 获取ONVIF设备列表
func (s *Server) handleGetONVIFDevices(w http.ResponseWriter, r *http.Request) {
	devices := s.onvifManager.GetDevices()

	// 转换设备数据格式
	deviceList := make([]map[string]interface{}, len(devices))
	for i, device := range devices {
		deviceList[i] = convertONVIFDevice(device)
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"devices": deviceList,
	})
}

// convertONVIFDevice 转换ONVIF设备到API响应格式
func convertONVIFDevice(device *onvif.Device) map[string]interface{} {
	return map[string]interface{}{
		"deviceId":        device.DeviceID,
		"name":            device.Name,
		"model":           device.Model,
		"manufacturer":    device.Manufacturer,
		"firmwareVersion": device.FirmwareVersion,
		"serialNumber":    device.SerialNumber,
		"ip":              device.IP,
		"port":            device.Port,
		"username":        device.Username,
		"password":        device.Password,
		"hasCredentials":  device.Username != "" || device.Password != "",
		"status":          device.Status,
		"services":        device.Services,
		"previewURL":      device.PreviewURL,
		"snapshotURL":     device.SnapshotURL,
		"responseTime":    device.ResponseTime,
		"lastCheckTime":   device.LastCheckTime,
		"discoveryTime":   device.DiscoveryTime,
		"lastSeenTime":    device.LastSeenTime,
		"checkInterval":   device.CheckInterval,
		"failureCount":    device.FailureCount,
		"ptzSupported":    device.PTZSupported,
		"audioSupported":  device.AudioSupported,
	}
}

// handleGetONVIFDevice 获取单个ONVIF设备信息
func (s *Server) handleGetONVIFDevice(w http.ResponseWriter, r *http.Request) {
	deviceID := mux.Vars(r)["id"]
	device, exists := s.onvifManager.GetDeviceByID(deviceID)
	if !exists {
		respondNotFound(w, "设备不存在")
		return
	}
	respondRaw(w, http.StatusOK, map[string]interface{}{
		"device": convertONVIFDevice(device),
	})
}

// handleAddONVIFDevice 添加ONVIF设备
func (s *Server) handleAddONVIFDevice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		XAddr    string `json:"xaddr"`
		IP       string `json:"ip"`
		Port     int    `json:"port"`
		Username string `json:"username"`
		Password string `json:"password"`
		Name     string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, fmt.Sprintf("无效的请求参数: %v", err))
		return
	}

	var device *onvif.Device
	var err error

	if req.XAddr != "" {
		device, err = s.onvifManager.AddDevice(req.XAddr, req.Username, req.Password)
	} else if req.IP != "" {
		port := req.Port
		if port == 0 {
			port = 80
		}
		device, err = s.onvifManager.AddDeviceWithIP(req.IP, port, req.Username, req.Password)
	} else {
		respondBadRequest(w, "需要提供 xaddr 或 ip 地址")
		return
	}

	if err != nil {
		respondInternalError(w, fmt.Sprintf("添加设备失败: %v", err))
		return
	}

	if req.Name != "" {
		device.Name = req.Name
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"message": "设备添加成功",
		"device":  convertONVIFDevice(device),
	})
}

// handleCheckONVIFAuth 验证ONVIF设备凭据
func (s *Server) handleCheckONVIFAuth(w http.ResponseWriter, r *http.Request) {
	deviceID := mux.Vars(r)["id"]

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, fmt.Sprintf("无效的请求参数: %v", err))
		return
	}

	device, exists := s.onvifManager.GetDeviceByID(deviceID)
	if !exists {
		respondNotFound(w, "设备不存在")
		return
	}

	// 使用 manager 中的 VerifyDeviceCredentials 函数
	err := s.onvifManager.VerifyDeviceCredentials(device.IP, device.Port, req.Username, req.Password)
	if err != nil {
		respondError(w, http.StatusUnauthorized, fmt.Sprintf("凭据验证失败: %v", err))
		return
	}

	respondSuccessMsg(w, "凭据验证成功")
}

// handleRemoveONVIFDevice 删除ONVIF设备
func (s *Server) handleRemoveONVIFDevice(w http.ResponseWriter, r *http.Request) {
	deviceID := mux.Vars(r)["id"]
	if err := s.onvifManager.RemoveDevice(deviceID); err != nil {
		respondInternalError(w, fmt.Sprintf("删除设备失败: %v", err))
		return
	}
	respondSuccessMsg(w, "设备已删除")
}

// handleRefreshONVIFDevice 刷新ONVIF设备
func (s *Server) handleRefreshONVIFDevice(w http.ResponseWriter, r *http.Request) {
	deviceID := mux.Vars(r)["id"]

	var req struct {
		IP   string `json:"ip"`
		Port int    `json:"port"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, fmt.Sprintf("无效的请求参数: %v", err))
		return
	}

	if req.IP == "" || req.Port <= 0 {
		respondBadRequest(w, "必须提供有效的 IP 地址和端口")
		return
	}

	if err := s.onvifManager.RefreshDevice(deviceID, req.IP, req.Port); err != nil {
		respondInternalError(w, fmt.Sprintf("刷新设备失败: %v", err))
		return
	}

	device, _ := s.onvifManager.GetDeviceByID(deviceID)

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "设备信息已刷新",
		"device":  convertONVIFDevice(device),
	})
}

// handleGetONVIFProfiles 获取ONVIF设备配置文件列表
func (s *Server) handleGetONVIFProfiles(w http.ResponseWriter, r *http.Request) {
	deviceID := mux.Vars(r)["id"]

	profiles, err := s.onvifManager.GetProfiles(deviceID)
	if err != nil {
		respondInternalError(w, fmt.Sprintf("获取配置文件失败: %v", err))
		return
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"profiles": profiles,
	})
}

// handleGetONVIFSnapshot 获取ONVIF设备快照
func (s *Server) handleGetONVIFSnapshot(w http.ResponseWriter, r *http.Request) {
	deviceID := mux.Vars(r)["id"]
	profileToken := r.URL.Query().Get("profile")

	data, contentType, err := s.onvifManager.GetSnapshot(deviceID, profileToken)
	if err != nil {
		respondInternalError(w, fmt.Sprintf("获取快照失败: %v", err))
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"snapshot_%s.jpg\"", deviceID))
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// handleGetONVIFPresets 获取ONVIF设备预置位
func (s *Server) handleGetONVIFPresets(w http.ResponseWriter, r *http.Request) {
	deviceID := mux.Vars(r)["id"]

	presets, err := s.onvifManager.GetPTZPresets(deviceID)
	if err != nil {
		respondInternalError(w, fmt.Sprintf("获取预置位失败: %v", err))
		return
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"presets": presets,
	})
}

// handleSetONVIFPreset 设置ONVIF预置位
func (s *Server) handleSetONVIFPreset(w http.ResponseWriter, r *http.Request) {
	deviceID := mux.Vars(r)["id"]

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, "无效的请求参数")
		return
	}

	presetToken, err := s.onvifManager.SetPTZPreset(deviceID, req.Name)
	if err != nil {
		respondInternalError(w, fmt.Sprintf("设置预置位失败: %v", err))
		return
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success":     true,
		"message":     "预置位设置成功",
		"presetToken": presetToken,
	})
}

// handleGotoONVIFPreset 调用ONVIF预置位
func (s *Server) handleGotoONVIFPreset(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	deviceID := vars["id"]
	presetToken := vars["token"]

	if err := s.onvifManager.PTZGotoPreset(deviceID, presetToken); err != nil {
		respondInternalError(w, fmt.Sprintf("调用预置位失败: %v", err))
		return
	}

	respondSuccessMsg(w, "预置位调用成功")
}

// handleSyncONVIFChannels 同步ONVIF设备的媒体配置文件为通道
func (s *Server) handleSyncONVIFChannels(w http.ResponseWriter, r *http.Request) {
	deviceID := mux.Vars(r)["id"]

	device, exists := s.onvifManager.GetDeviceByID(deviceID)
	if !exists {
		respondNotFound(w, "ONVIF设备不存在")
		return
	}

	// 获取最新的媒体配置文件
	profiles, err := s.onvifManager.GetProfiles(deviceID)
	if err != nil {
		respondInternalError(w, fmt.Sprintf("获取设备配置文件失败: %v", err))
		return
	}

	if len(profiles) == 0 {
		respondSuccessMsg(w, "设备没有可同步的媒体配置文件")
		return
	}

	addedCount := 0
	updatedCount := 0

	for _, profile := range profiles {
		// 使用 profile token 作为通道ID的一部分，确保唯一性
		channelID := fmt.Sprintf("%s_%s", device.DeviceID, profile["token"])
		channelName := fmt.Sprintf("%s - %s", device.Name, profile["name"])

		apiChannel := &Channel{
			ChannelID:    channelID,
			ChannelName:  channelName,
			DeviceID:     device.DeviceID,
			DeviceType:   "onvif",
			Status:       device.Status,
			SipPort:      device.SipPort, // 关键：传递SipPort
			StreamURL:    "",             // 预览时动态获取
			ProfileToken: profile["token"].(string),
		}

		if _, exists := s.channelManager.GetChannel(channelID); exists {
			s.channelManager.UpdateChannel(apiChannel)
			updatedCount++
		} else {
			s.channelManager.AddChannel(apiChannel)
			addedCount++
		}
	}

	message := fmt.Sprintf("同步完成: 新增 %d 个通道, 更新 %d 个通道", addedCount, updatedCount)
	respondSuccessMsg(w, message)
}

// handleStartONVIFPreview 启动ONVIF设备预览
func (s *Server) handleStartONVIFPreview(w http.ResponseWriter, r *http.Request) {
	deviceID := mux.Vars(r)["id"]

	if !s.checkZLMAvailable(w) {
		return
	}

	var req struct {
		ProfileToken string `json:"profileToken"`
	}
	json.NewDecoder(r.Body).Decode(&req) // 忽略解析错误，允许空请求体

	// 从 onvif.Manager 获取流地址，它会处理认证和URL构建
	rtspURL, err := s.onvifManager.GetStreamURL(deviceID, req.ProfileToken)
	if err != nil {
		respondInternalError(w, fmt.Sprintf("获取流地址失败: %v", err))
		return
	}

	if rtspURL == "" {
		respondBadRequest(w, "无法获取设备的预览地址")
		return
	}

	// 调用统一的预览启动函数
	app := "onvif" // ONVIF流使用 'onvif' app
	res, err := s.startPreview(r, deviceID, deviceID, rtspURL, app)
	if err != nil {
		debug.Error("api", "添加流代理失败: %v", err)
		respondInternalError(w, fmt.Sprintf("添加流代理失败: %v", err))
		return
	}

	session := &PreviewSession{
		DeviceID:   deviceID,
		StreamKey:  "",
		App:        app,
		Stream:     res.StreamID,
		SourceURL:  rtspURL,
		FlvURL:     res.FlvURL,
		WsFlvURL:   res.WsFlvURL,
		HlsURL:     res.HlsURL,
		RtmpURL:    res.RtmpURL,
		CreateTime: time.Now().Unix(),
	}
	previewSessions[deviceID] = session

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "预览启动成功",
		"data":    session,
	})
}

// handleStopONVIFPreview 停止ONVIF设备预览
func (s *Server) handleStopONVIFPreview(w http.ResponseWriter, r *http.Request) {
	deviceID := mux.Vars(r)["id"]

	// 使用 preview.Manager 停止 RTSP 代理
	if s.previewManager != nil {
		if err := s.previewManager.StopRTSPProxy(deviceID, "onvif"); err != nil {
			debug.Warn("api", "停止 RTSP 代理失败: %v", err)
		}
	} else if s.zlmServer != nil && s.zlmServer.GetAPIClient() != nil {
		s.zlmServer.GetAPIClient().CloseStream("live", deviceID)
	}

	respondSuccessMsg(w, "预览已停止")
}

// handleDiscoverONVIFDevices ONVIF设备发现
func (s *Server) handleDiscoverONVIFDevices(w http.ResponseWriter, r *http.Request) {
	discoveredDevices, err := s.onvifManager.DiscoverDevices()
	if err != nil {
		respondInternalError(w, fmt.Sprintf("设备发现失败: %v", err))
		return
	}

	results := make([]map[string]interface{}, 0, len(discoveredDevices))
	for _, device := range discoveredDevices {
		results = append(results, map[string]interface{}{
			"xaddr":        device.XAddr,
			"types":        device.Types,
			"manufacturer": device.Manufacturer,
			"model":        device.Model,
			"name":         device.Name,
			"location":     device.Location,
			"hardware":     device.Hardware,
			"sourceIP":     device.SourceIP,
		})
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("发现 %d 个ONVIF设备", len(discoveredDevices)),
		"count":   len(discoveredDevices),
		"devices": results,
	})
}

// handleBatchAddONVIFDevices 批量添加ONVIF设备
func (s *Server) handleBatchAddONVIFDevices(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Devices []struct {
			IP       string `json:"ip"`
			Port     int    `json:"port"`
			Username string `json:"username"`
			Password string `json:"password"`
		} `json:"devices"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, "无效的请求参数")
		return
	}

	results := make([]map[string]interface{}, 0)
	successCount := 0

	for _, d := range req.Devices {
		port := d.Port
		if port == 0 {
			port = 80
		}

		device, err := s.onvifManager.AddDeviceWithIP(d.IP, port, d.Username, d.Password)
		if err != nil {
			results = append(results, map[string]interface{}{
				"ip":      d.IP,
				"success": false,
				"error":   err.Error(),
			})
		} else {
			successCount++
			results = append(results, map[string]interface{}{
				"ip":       d.IP,
				"success":  true,
				"deviceId": device.DeviceID,
			})
		}
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success":      true,
		"message":      fmt.Sprintf("成功添加 %d/%d 个设备", successCount, len(req.Devices)),
		"successCount": successCount,
		"totalCount":   len(req.Devices),
		"results":      results,
	})
}

// handleGetONVIFStatistics 获取ONVIF统计信息
func (s *Server) handleGetONVIFStatistics(w http.ResponseWriter, r *http.Request) {
	stats := s.onvifManager.GetDeviceStatistics()
	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"statistics": stats,
	})
}
