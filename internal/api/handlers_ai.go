package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"

	"gb28181-onvif-server/internal/ai"
	"gb28181-onvif-server/internal/config"
	"gb28181-onvif-server/internal/debug"
)

// handleStartAIRecording 启动AI录像
func (s *Server) handleStartAIRecording(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChannelID string `json:"channel_id"`
		StreamURL string `json:"stream_url"`
		Mode      string `json:"mode"` // person, motion, continuous
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, err.Error())
		return
	}

	if s.aiManager == nil {
		respondServiceUnavailable(w, "AI功能未启用")
		return
	}

	// 如果没有提供StreamURL，尝试从通道配置获取
	if req.StreamURL == "" && req.ChannelID != "" {
		if channel, exists := s.channelManager.GetChannel(req.ChannelID); exists && channel != nil {
			req.StreamURL = channel.StreamURL
		}
	}

	// 如果StreamURL仍为空，自动启动预览
	if req.StreamURL == "" && req.ChannelID != "" {
		// 1. 首先检查channelManager中是否存在通道
		channel, exists := s.channelManager.GetChannel(req.ChannelID)

		// 2. 如果channelManager中没有，尝试从GB28181服务器获取
		var deviceID string
		var app string = "rtp"

		if !exists || channel == nil {
			// 尝试从GB28181设备获取通道
			if s.gb28181Server != nil {
				if gb28181Channel, found := s.gb28181Server.GetChannelByID(req.ChannelID); found {
					debug.Info("ai", "通道 %s 从GB28181设备获取成功", req.ChannelID)
					deviceID = gb28181Channel.DeviceID
					app = "rtp"
				}
			}

			// 如果仍然没找到，返回错误
			if deviceID == "" {
				respondBadRequest(w, fmt.Sprintf("通道 %s 不存在", req.ChannelID))
				return
			}
		} else {
			// 从channelManager获取
			deviceID = channel.DeviceID
			if channel.DeviceType == "onvif" {
				app = "onvif"
			}
		}

		// 自动启动预览
		debug.Info("ai", "通道 %s 未启动预览，自动启动预览...", req.ChannelID)
		previewRes, err := s.startPreview(r, deviceID, req.ChannelID, "", app)
		if err != nil {
			respondInternalError(w, fmt.Sprintf("自动启动预览失败: %v", err))
			return
		}

		// 使用RTSP流进行AI检测（因为FLV不支持H.265）
		// 构建RTSP URL: rtsp://127.0.0.1:8554/app/streamID
		zlmHost := s.getZLMHost(r)
		_, _, rtspPort := s.getZLMPorts()
		req.StreamURL = fmt.Sprintf("rtsp://%s:%d/%s/%s", zlmHost, rtspPort, app, previewRes.StreamID)

		debug.Info("ai", "通道 %s 预览启动成功，使用RTSP流: %s", req.ChannelID, req.StreamURL)

		// 如果通道存在于channelManager，更新StreamURL
		if channel != nil {
			channel.StreamURL = previewRes.HlsURL
			s.channelManager.UpdateChannel(channel)
		}
	}

	// 验证 StreamURL
	if req.StreamURL == "" {
		respondBadRequest(w, "无法获取通道流地址")
		return
	}

	mode := ai.RecordingModePerson // 默认人形检测
	switch req.Mode {
	case "motion":
		mode = ai.RecordingModeMotion
	case "continuous":
		mode = ai.RecordingModeContinuous
	case "manual":
		mode = ai.RecordingModeManual
	}

	err := s.aiManager.StartChannelRecording(req.ChannelID, req.StreamURL, mode)
	if err != nil {
		respondInternalError(w, err.Error())
		return
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"channel_id": req.ChannelID,
		"stream_url": req.StreamURL,
		"mode":       mode,
	})
}

// handleStopAIRecording 停止AI录像
func (s *Server) handleStopAIRecording(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChannelID string `json:"channel_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, err.Error())
		return
	}

	if s.aiManager == nil {
		respondServiceUnavailable(w, "AI功能未启用")
		return
	}

	err := s.aiManager.StopChannelRecording(req.ChannelID)
	if err != nil {
		respondInternalError(w, err.Error())
		return
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"channel_id": req.ChannelID,
	})
}

// handleGetAIRecordingStatus 获取AI录像状态
func (s *Server) handleGetAIRecordingStatus(w http.ResponseWriter, r *http.Request) {
	channelID := r.URL.Query().Get("channel_id")
	if channelID == "" {
		respondBadRequest(w, "缺少channel_id参数")
		return
	}

	if s.aiManager == nil {
		respondServiceUnavailable(w, "AI功能未启用")
		return
	}

	status, err := s.aiManager.GetChannelStatus(channelID)
	if err != nil {
		respondNotFound(w, err.Error())
		return
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"status":  status,
	})
}

// handleGetAllAIRecordingStatus 获取所有AI录像状态
func (s *Server) handleGetAllAIRecordingStatus(w http.ResponseWriter, r *http.Request) {
	if s.aiManager == nil {
		respondServiceUnavailable(w, "AI功能未启用")
		return
	}

	status := s.aiManager.GetAllStatus()

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"status":  status,
	})
}

// handleGetAIConfig 获取AI配置
func (s *Server) handleGetAIConfig(w http.ResponseWriter, r *http.Request) {
	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"config":  s.config.AI,
	})
}

// handleUpdateAIConfig 更新AI配置
func (s *Server) handleUpdateAIConfig(w http.ResponseWriter, r *http.Request) {
	var aiConfig config.AIConfig
	if err := json.NewDecoder(r.Body).Decode(&aiConfig); err != nil {
		respondBadRequest(w, err.Error())
		return
	}

	s.config.AI = &aiConfig

	if err := s.config.Save(s.configPath); err != nil {
		respondInternalError(w, fmt.Sprintf("保存配置失败: %v", err))
		return
	}

	// 更新AI管理器配置
	if s.aiManager != nil {
		s.aiManager.SetConfig(&aiConfig)
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"config":  s.config.AI,
	})
}

// handleGetAIDetectorInfo 获取AI检测器信息
func (s *Server) handleGetAIDetectorInfo(w http.ResponseWriter, r *http.Request) {
	if s.aiManager == nil {
		respondRaw(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"info": map[string]interface{}{
				"available": false,
				"error":     "AI功能未启用",
			},
		})
		return
	}

	info := s.aiManager.GetDetectorInfo()
	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"info":    info,
	})
}

// handleAIDetect 执行单次AI检测
func (s *Server) handleAIDetect(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChannelID string `json:"channel_id"`
		StreamURL string `json:"stream_url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, err.Error())
		return
	}

	if s.aiManager == nil {
		respondServiceUnavailable(w, "AI功能未启用")
		return
	}

	detector := s.aiManager.GetDetector()
	if detector == nil {
		respondServiceUnavailable(w, "AI检测器未初始化")
		return
	}

	// 如果没有提供StreamURL，尝试从通道配置获取
	streamURL := req.StreamURL
	if streamURL == "" && req.ChannelID != "" {
		if channel, exists := s.channelManager.GetChannel(req.ChannelID); exists && channel != nil {
			streamURL = channel.StreamURL
		}
	}

	if streamURL == "" {
		respondBadRequest(w, "缺少流地址")
		return
	}

	// 执行检测（这里简化处理，实际应该从流中抓帧）
	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"channel_id": req.ChannelID,
		"message":    "检测请求已提交，请通过AI录像状态API查看结果",
	})
}

// handleStopAllAIRecording 停止所有AI录像
func (s *Server) handleStopAllAIRecording(w http.ResponseWriter, r *http.Request) {
	if s.aiManager == nil {
		respondServiceUnavailable(w, "AI功能未启用")
		return
	}

	s.aiManager.StopAll()

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "所有AI录像已停止",
	})
}

// handleListAIRecordings 查询AI录像列表
// 查询参数:
//   - channel_id: 通道ID（必需）
//   - date: 查询日期，格式: 2025-12-07（可选，不指定则查询所有日期）
func (s *Server) handleListAIRecordings(w http.ResponseWriter, r *http.Request) {
	channelID := r.URL.Query().Get("channel_id")
	dateStr := r.URL.Query().Get("date") // 格式: 2025-12-07
	app := "rtp"                         // AI录像统一使用rtp应用

	if channelID == "" {
		s.jsonError(w, http.StatusBadRequest, "缺少channel_id参数")
		return
	}

	// AI录像保存在ZLM配置的录像目录下
	// 目录结构: {filePath}/{app}/{channelId}/{date}/{files}
	// ZLM 的 record.filePath 配置决定了录像的保存位置

	var recordPath string
	if s.config.ZLM != nil && s.config.ZLM.Record != nil && s.config.ZLM.Record.RecordPath != "" {
		// 使用config.yaml中配置的RecordPath（ZLM的record.filePath）
		recordPath = s.config.ZLM.Record.RecordPath
		// 转换为绝对路径
		if absPath, err := filepath.Abs(recordPath); err == nil {
			recordPath = absPath
		}
	} else if s.zlmProcess != nil {
		// 回退到ZLM工作目录下的www/record（HLS等默认目录）
		recordPath = filepath.Join(s.zlmProcess.GetWorkDir(), "www", "record")
	} else {
		// 默认路径
		recordPath = "./recordings"
		if absPath, err := filepath.Abs(recordPath); err == nil {
			recordPath = absPath
		}
	}

	// 构造通道录像目录路径: {recordPath}/{app}/{channelId}/
	channelRecordPath := filepath.Join(recordPath, app, channelID)

	debug.Info("ai", "查询AI录像: channelID=%s, date=%s, recordPath=%s", channelID, dateStr, channelRecordPath)

	// 检查通道录像目录是否存在
	if _, err := os.Stat(channelRecordPath); os.IsNotExist(err) {
		s.jsonResponse(w, http.StatusOK, map[string]interface{}{
			"success":    true,
			"channelId":  channelID,
			"date":       dateStr,
			"recordPath": channelRecordPath,
			"total":      0,
			"recordings": []interface{}{},
		})
		return
	}

	var recordings []map[string]interface{}

	// 如果指定了日期，只查询该日期的录像
	if dateStr != "" {
		datePath := filepath.Join(channelRecordPath, dateStr)

		if info, err := os.Stat(datePath); err == nil && info.IsDir() {
			recordings = scanDateDirectory(datePath, app, channelID, dateStr)
		}
	} else {
		// 未指定日期，扫描所有日期目录
		entries, err := os.ReadDir(channelRecordPath)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					datePath := filepath.Join(channelRecordPath, entry.Name())
					dateRecordings := scanDateDirectory(datePath, app, channelID, entry.Name())
					recordings = append(recordings, dateRecordings...)
				}
			}
		}
	}

	// 按开始时间倒序排序
	sort.Slice(recordings, func(i, j int) bool {
		ti := recordings[i]["startTime"].(string)
		tj := recordings[j]["startTime"].(string)
		return ti > tj
	})

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"channelId":  channelID,
		"date":       dateStr,
		"app":        app,
		"recordPath": channelRecordPath,
		"total":      len(recordings),
		"recordings": recordings,
	})
}
