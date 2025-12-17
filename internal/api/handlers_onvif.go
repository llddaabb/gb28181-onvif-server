package api

import (
	"encoding/json"
	"fmt"
	"net/http"
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

	// 如果提供了新的 IP，更新设备信息
	if req.NewIP != "" {
		device.IP = req.NewIP
		if req.NewPort > 0 {
			device.Port = req.NewPort
		}
	}

	// 将更新后的设备保存回管理器（需要在管理器中实现该方法）
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
	deviceID := vars["deviceId"]
	profileToken := r.URL.Query().Get("profileToken")

	if deviceID == "" || profileToken == "" {
		respondBadRequest(w, "deviceId 和 profileToken 不能为空")
		return
	}

	device, ok := s.onvifManager.GetDeviceByID(deviceID)
	if !ok {
		respondNotFound(w, "设备未找到"+device.DeviceID)
		return
	}

	rtspURL, err := s.onvifManager.GetStreamURI(deviceID, profileToken)
	if err != nil {
		respondInternalError(w, fmt.Sprintf("获取流地址失败: %s", err.Error()))
		return
	}

	res, err := s.startPreview(r, deviceID, "", rtspURL, "onvif")
	if err != nil {
		respondInternalError(w, fmt.Sprintf("启动预览失败: %s", err.Error()))
		return
	}

	// 更新设备预览URL
	s.onvifManager.UpdateDevicePreview(deviceID, res.FlvURL, "")

	respondRaw(w, http.StatusOK, res)
}

// handleStopONVIFPreview 停止ONVIF设备预览流
func (s *Server) handleStopONVIFPreview(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	deviceID := vars["deviceId"]

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

func (s *Server) stopPreview(deviceID string, param2 string, param3 string) (any, error) {
	panic("unimplemented")
}

// handleONVIFPTZControl ONVIF PTZ 控制
func (s *Server) handleONVIFPTZControl(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	deviceID := vars["deviceId"]

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
	deviceID := vars["deviceId"]

	profiles, err := s.onvifManager.GetProfiles(deviceID)
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
	deviceID := vars["deviceId"]
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
	deviceID := vars["deviceId"]
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

// handleONVIFGetStatus 获取设备状态
func (s *Server) handleONVIFGetStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	deviceID := vars["deviceId"]

	status, err := s.onvifManager.GetDeviceStatus(deviceID)
	if err != nil {
		respondInternalError(w, fmt.Sprintf("获取设备状态失败: %s", err.Error()))
		return
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"status": status,
	})
}

// handleGetONVIFStats 获取ONVIF模块统计信息
func (s *Server) handleGetONVIFStats(w http.ResponseWriter, r *http.Request) {
	stats := s.onvifManager.GetStats()
	respondRaw(w, http.StatusOK, stats)
}

// handleONVIFExport 导出ONVIF设备列表
func (s *Server) handleONVIFExport(w http.ResponseWriter, r *http.Request) {
	deviceList := s.onvifManager.ExportDevices()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=onvif_devices.json")
	json.NewEncoder(w).Encode(deviceList)
}

// handleONVIFImport 导入ONVIF设备列表
func (s *Server) handleONVIFImport(w http.ResponseWriter, r *http.Request) {
	var deviceList []map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&deviceList); err != nil {
		respondBadRequest(w, "无效的请求格式")
		return
	}

	added, failed, errors := s.onvifManager.ImportDevices(deviceList)

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success":     true,
		"message":     fmt.Sprintf("导入完成：成功 %d 个，失败 %d 个", added, failed),
		"addedCount":  added,
		"failedCount": failed,
		"errors":      errors,
	})
}

// handleGetVideoEncoderConfigs 获取ONVIF设备视频编码配置
func (s *Server) handleGetVideoEncoderConfigs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	deviceID := vars["deviceId"]
	profileToken := r.URL.Query().Get("profileToken")

	if profileToken == "" {
		respondBadRequest(w, "profileToken 不能为空")
		return
	}

	configs, err := s.onvifManager.GetVideoEncoderConfigurations(deviceID, profileToken)
	if err != nil {
		respondInternalError(w, fmt.Sprintf("获取视频编码配置失败: %s", err.Error()))
		return
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"configs": configs,
	})
}

// respondOK 简化的 200 成功响应
func respondOK(w http.ResponseWriter, message string) {
	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": message,
	})
}
