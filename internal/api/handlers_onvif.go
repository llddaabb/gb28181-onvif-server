package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

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
		"sipPort":         device.SipPort,
		"username":        device.Username,
		"status":          device.Status,
		"previewURL":      device.PreviewURL,
		"snapshotURL":     device.SnapshotURL,
		"ptzSupported":    device.PTZSupported,
		"audioSupported":  device.AudioSupported,
		"discoveryTime":   device.DiscoveryTime.Format(time.RFC3339),
		"lastSeenTime":    device.LastSeenTime.Format(time.RFC3339),
		"lastCheckTime":   device.LastCheckTime.Format(time.RFC3339),
		"checkInterval":   device.CheckInterval,
		"responseTime":    device.ResponseTime, // int64
		"failureCount":    device.FailureCount,
		"services":        device.Services,
		"metadata":        device.Metadata,
	}
}

// handleGetONVIFDevice 获取单个ONVIF设备详情
func (s *Server) handleGetONVIFDevice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	deviceID := vars["deviceId"]

	device, ok := s.onvifManager.GetDeviceByID(deviceID)
	if !ok {
		respondNotFound(w, "设备未找到")
		return
	}

	respondRaw(w, http.StatusOK, convertONVIFDevice(device))
}

// handleSearchONVIFDevices 搜索ONVIF设备
func (s *Server) handleSearchONVIFDevices(w http.ResponseWriter, r *http.Request) {
	// 启动发现，等待 3 秒
	s.onvifManager.StartDiscovery(3 * time.Second)

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "ONVIF 设备发现已启动，请稍后查询设备列表",
	})
}

// handleAddONVIFDevice 手动添加ONVIF设备
func (s *Server) handleAddONVIFDevice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IP       string `json:"ip"`
		Port     int    `json:"port"`
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, "无效的请求参数")
		return
	}

	if req.IP == "" || req.Username == "" {
		respondBadRequest(w, "IP 和用户名不能为空")
		return
	}

	port := req.Port
	if port == 0 {
		port = 80
	}

	device, err := s.onvifManager.AddDeviceWithIP(req.IP, port, req.Username, req.Password)
	if err != nil {
		respondInternalError(w, fmt.Sprintf("添加设备失败: %s", err.Error()))
		return
	}

	respondRaw(w, http.StatusCreated, map[string]interface{}{
		"success":  true,
		"message":  "设备添加成功",
		"deviceId": device.DeviceID,
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
		"success": true,
		"message": fmt.Sprintf("成功添加 %d/%d 个设备", successCount, len(req.Devices)),
		"summary": map[string]interface{}{
			"added":  successCount,
			"failed": len(req.Devices) - successCount,
		},
		"results": results,
	})
}

// handleRefreshONVIFDevice 刷新ONVIF设备信息
func (s *Server) handleRefreshONVIFDevice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	deviceID := vars["id"]

	device, ok := s.onvifManager.GetDeviceByID(deviceID)
	if !ok {
		respondNotFound(w, fmt.Sprintf("设备不存在: %s", deviceID))
		return
	}

	// 刷新操作：重新获取设备详细信息
	// 这里可以添加参数以更新IP地址（可选）
	var req struct {
		NewIP   string `json:"new_ip"`
		NewPort int    `json:"new_port"`
	}

	// 尝试解析请求体，但即使失败也可以继续
	json.NewDecoder(r.Body).Decode(&req)

	// 如果提供了新的 IP 或端口，更新设备信息
	if req.NewIP != "" || req.NewPort > 0 {
		if err := s.onvifManager.UpdateDeviceIP(deviceID, req.NewIP, req.NewPort); err != nil {
			respondInternalError(w, fmt.Sprintf("更新设备IP失败: %s", err.Error()))
			return
		}
		// 重新获取更新后的设备信息（使用新的ID）
		newDeviceID := deviceID
		if req.NewIP != "" {
			port := req.NewPort
			if port == 0 {
				port = device.Port
			}
			newDeviceID = fmt.Sprintf("%s:%d", req.NewIP, port)
		} else if req.NewPort > 0 {
			newDeviceID = fmt.Sprintf("%s:%d", device.IP, req.NewPort)
		}
		device, _ = s.onvifManager.GetDeviceByID(newDeviceID)
	}

	// 确保 device 不为 nil
	if device == nil {
		respondInternalError(w, "设备更新后无法获取设备信息")
		return
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "设备已刷新",
		"device": map[string]interface{}{
			"deviceId": device.DeviceID,
			"ip":       device.IP,
			"port":     device.Port,
		},
	})
}

// handleDeleteONVIFDevice 删除ONVIF设备
func (s *Server) handleDeleteONVIFDevice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	deviceID := vars["deviceId"]

	if err := s.onvifManager.RemoveDevice(deviceID); err != nil {
		respondNotFound(w, fmt.Sprintf("删除设备失败: %s", err.Error()))
		return
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("设备 %s 已删除", deviceID),
	})
}

// handleStartONVIFPreview 启动ONVIF设备预览流
func (s *Server) handleStartONVIFPreview(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	deviceID := vars["id"]

	if deviceID == "" {
		respondBadRequest(w, "deviceId 不能为空")
		return
	}

	device, ok := s.onvifManager.GetDeviceByID(deviceID)
	if !ok {
		respondNotFound(w, "设备未找到: "+deviceID)
		return
	}

	// 支持从 query 参数或 body 中获取 profileToken 和凭据
	profileToken := r.URL.Query().Get("profileToken")
	var reqUsername, reqPassword string

	// 尝试从 body 中获取参数
	var reqBody struct {
		ProfileToken string `json:"profileToken"`
		Username     string `json:"username"`
		Password     string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err == nil {
		if profileToken == "" {
			profileToken = reqBody.ProfileToken
		}
		reqUsername = reqBody.Username
		reqPassword = reqBody.Password
	}

	// 使用请求中的凭据，或者回退到设备存储的凭据
	username := reqUsername
	password := reqPassword
	if username == "" {
		username = device.Username
	}
	if password == "" {
		password = device.Password
	}
	if username == "" {
		username = "admin"
	}

	// 如果还是没有 profileToken，使用默认的第一个 profile
	if profileToken == "" {
		profiles, err := s.onvifManager.GetProfiles(deviceID)
		if err == nil && len(profiles) > 0 {
			if token, ok := profiles[0]["token"].(string); ok {
				profileToken = token
				log.Printf("[ONVIF] 使用默认 profile: %s", profileToken)
			}
		}
	}

	var rtspURL string
	var err error

	// 如果有 profileToken，尝试通过 ONVIF 获取流地址
	if profileToken != "" {
		rtspURL, err = s.onvifManager.GetStreamURI(deviceID, profileToken)
		if err != nil {
			log.Printf("[ONVIF] GetStreamURI 失败: %s, 尝试使用设备预设 URL", err.Error())
		}
	}

	// 如果 ONVIF 获取失败，使用设备已有的 previewURL
	if rtspURL == "" && device.PreviewURL != "" {
		rtspURL = device.PreviewURL
		log.Printf("[ONVIF] 使用设备预设 previewURL: %s", rtspURL)
	}

	// 如果还是没有 URL，尝试生成默认的 RTSP URL（适用于海康等设备）
	if rtspURL == "" {
		// 使用前面已确定的 username/password
		if password == "" {
			password = "a123456789" // 默认密码
		}
		// 海康标准 RTSP URL 格式
		rtspURL = fmt.Sprintf("rtsp://%s:%s@%s:554/Streaming/Channels/101", username, password, device.IP)
		log.Printf("[ONVIF] 使用默认 RTSP URL 模板: %s", rtspURL)
	}

	// 如果还是没有 URL，返回错误
	if rtspURL == "" {
		respondInternalError(w, "无法获取流地址，请检查设备凭据或配置")
		return
	}

	res, err := s.startPreview(r, deviceID, "", rtspURL, "onvif")
	if err != nil {
		// 记录详细的错误信息
		log.Printf("[ONVIF] ⚠️ 启动预览失败: %s (RTSP URL: %s)", err.Error(), rtspURL)

		// 返回更有帮助的错误消息
		var errMsg string
		if strings.Contains(err.Error(), "404") {
			errMsg = fmt.Sprintf("RTSP 地址不存在。请检查：\n1. 设备 RTSP 服务是否启用\n2. RTSP 端口是否正确（当前用 554）\n3. 流路径是否正确\n\n生成的 URL: %s", rtspURL)
		} else if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "Unauthorized") {
			errMsg = fmt.Sprintf("RTSP 认证失败。请检查用户名和密码是否正确。\n设备凭证: %s / ****", device.Username)
		} else if strings.Contains(err.Error(), "Connection refused") || strings.Contains(err.Error(), "dial tcp") {
			errMsg = fmt.Sprintf("无法连接到设备的 RTSP 服务。请检查：\n1. 设备是否在线\n2. RTSP 端口 554 是否可访问\n\n生成的 URL: %s", rtspURL)
		} else {
			errMsg = fmt.Sprintf("启动预览失败: %s\n\n生成的 URL: %s", err.Error(), rtspURL)
		}
		respondInternalError(w, errMsg)
		return
	}

	// 更新设备预览URL
	s.onvifManager.UpdateDevicePreview(deviceID, res.FlvURL, "")

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    res,
	})
}

// handleStopONVIFPreview 停止ONVIF设备预览流
func (s *Server) handleStopONVIFPreview(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	deviceID := vars["id"]

	if deviceID == "" {
		respondBadRequest(w, "deviceId 不能为空")
		return
	}

	res, err := s.stopPreview(deviceID, "", "onvif")
	if err != nil {
		respondInternalError(w, fmt.Sprintf("停止预览失败: %s", err.Error()))
		return
	}

	// 清除设备预览URL
	s.onvifManager.UpdateDevicePreview(deviceID, "", "")

	respondRaw(w, http.StatusOK, res)
}

// stopPreview 停止预览流
func (s *Server) stopPreview(deviceID string, _ string, app string) (any, error) {
	if s.previewManager == nil {
		return nil, fmt.Errorf("preview manager 未初始化")
	}

	var err error
	if app == "onvif" {
		// ONVIF 预览使用 RTSP 代理
		err = s.previewManager.StopRTSPProxy(deviceID, app)
	} else {
		// GB28181 预览使用通道预览
		// 注意：此处需要 channelID，但在 ONVIF 上下文中 channelID 为空
		// 我们从设备 ID 推导出 channelID（假设设备 ID 包含 channelID 信息）
		// 或者使用空 channelID 并让管理器自动处理
		err = s.previewManager.StopChannelPreview(deviceID, "")
	}

	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"success": true,
		"message": "预览已停止",
	}, nil
}

// handleONVIFPTZControl ONVIF PTZ 控制
func (s *Server) handleONVIFPTZControl(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	deviceID := vars["id"] // 路由参数为 id

	var req struct {
		ProfileToken string `json:"profileToken"`
		Command      string `json:"command"`     // move, stop, preset
		Direction    string `json:"direction"`   // up, down, left, right, up_left, etc.
		PresetToken  string `json:"presetToken"` // for preset command
		Speed        int    `json:"speed"`       // 1-100
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, "无效的请求参数")
		return
	}

	if deviceID == "" || req.ProfileToken == "" || req.Command == "" {
		respondBadRequest(w, "deviceId, profileToken 和 command 不能为空")
		return
	}

	// PTZ 速度标准化 0.0 - 1.0
	speed := float64(req.Speed) / 100.0
	if speed <= 0 {
		speed = 0.5
	} else if speed > 1.0 {
		speed = 1.0
	}

	var err error
	switch req.Command {
	case "move":
		if req.Direction == "" {
			respondBadRequest(w, "移动命令 direction 不能为空")
			return
		}
		err = s.onvifManager.ContinuousMove(deviceID, req.ProfileToken, req.Direction, speed)
	case "stop":
		err = s.onvifManager.StopPTZ(deviceID, req.ProfileToken)
	case "home":
		// Home 命令：停止当前动作并回到预设的首页位置
		// 先停止当前运动
		_ = s.onvifManager.StopPTZ(deviceID, req.ProfileToken)
		// 如果有预设位置叫 "home" 或首页，可以跳转到该位置
		// 暂时只实现停止功能
		err = nil
	case "setPreset":
		if req.PresetToken == "" {
			respondBadRequest(w, "设置预置位 presetToken 不能为空")
			return
		}
		// 预置位名称
		presetName := r.URL.Query().Get("presetName")
		token, setErr := s.onvifManager.SetPreset(deviceID, req.ProfileToken, presetName, req.PresetToken)
		if setErr == nil {
			respondRaw(w, http.StatusOK, map[string]interface{}{"success": true, "token": token})
			return
		}
		err = setErr
	case "gotoPreset":
		if req.PresetToken == "" {
			respondBadRequest(w, "跳转预置位 presetToken 不能为空")
			return
		}
		err = s.onvifManager.GotoPreset(deviceID, req.ProfileToken, req.PresetToken, speed)
	case "removePreset":
		if req.PresetToken == "" {
			respondBadRequest(w, "删除预置位 presetToken 不能为空")
			return
		}
		err = s.onvifManager.RemovePreset(deviceID, req.ProfileToken, req.PresetToken)
	default:
		respondBadRequest(w, "不支持的 PTZ 命令")
		return
	}

	if err != nil {
		respondInternalError(w, fmt.Sprintf("PTZ 控制失败: %s", err.Error()))
		return
	}

	respondOK(w, "PTZ 控制成功")
}

// handleGetONVIFProfiles 获取ONVIF设备媒体配置
func (s *Server) handleGetONVIFProfiles(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	deviceID := vars["id"]

	// 支持从 query 参数或 body 中获取凭据
	username := r.URL.Query().Get("username")
	password := r.URL.Query().Get("password")

	// 尝试从 body 中获取凭据
	var reqBody struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err == nil {
		if username == "" {
			username = reqBody.Username
		}
		if password == "" {
			password = reqBody.Password
		}
	}

	profiles, err := s.onvifManager.GetProfilesWithCredentials(deviceID, username, password)
	if err != nil {
		respondInternalError(w, fmt.Sprintf("获取 Profiles 失败: %s", err.Error()))
		return
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"profiles": profiles,
	})
}

// handleGetONVIFPresets 获取ONVIF设备预置位
func (s *Server) handleGetONVIFPresets(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	deviceID := vars["id"]
	profileToken := r.URL.Query().Get("profileToken")

	if profileToken == "" {
		respondBadRequest(w, "profileToken 不能为空")
		return
	}

	presets, err := s.onvifManager.GetPresets(deviceID, profileToken)
	if err != nil {
		respondInternalError(w, fmt.Sprintf("获取预置位失败: %s", err.Error()))
		return
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"presets": presets,
	})
}

// handleGetONVIFSnapshotURI 获取抓图地址
func (s *Server) handleGetONVIFSnapshotURI(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	deviceID := vars["id"]
	profileToken := r.URL.Query().Get("profileToken")

	if profileToken == "" {
		respondBadRequest(w, "profileToken 不能为空")
		return
	}

	uri, err := s.onvifManager.GetSnapshotURI(deviceID, profileToken)
	if err != nil {
		respondInternalError(w, fmt.Sprintf("获取抓图地址失败: %s", err.Error()))
		return
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"uri": uri,
	})
}

// handleONVIFUpdateConfig 更新设备配置 (如 PTZ 配置)
func (s *Server) handleONVIFUpdateConfig(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	deviceID := vars["id"]

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		// Other config fields to update
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, "无效的请求参数")
		return
	}

	if deviceID == "" {
		respondBadRequest(w, "deviceId 不能为空")
		return
	}

	// 仅支持更新用户名和密码，其他配置待扩展
	if req.Username != "" || req.Password != "" {
		err := s.onvifManager.UpdateDeviceCredentials(deviceID, req.Username, req.Password)
		if err != nil {
			respondInternalError(w, fmt.Sprintf("更新设备凭据失败: %s", err.Error()))
			return
		}
	} else {
		respondBadRequest(w, "无可更新的配置项")
		return
	}

	respondOK(w, "设备配置更新成功")
}

// respondOK 简化的 200 成功响应
func respondOK(w http.ResponseWriter, message string) {
	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": message,
	})
}
