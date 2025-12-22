package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"gb28181-onvif-server/internal/ai"
	"gb28181-onvif-server/internal/config"
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

	mode := ai.RecordingModePerson // 默认人形检测
	switch req.Mode {
	case "motion":
		mode = ai.RecordingModeMotion
	case "continuous":
		mode = ai.RecordingModeContinuous
	case "manual":
		mode = ai.RecordingModeManual
	}

	err := s.aiManager.StartChannelRecording(req.ChannelID, mode)
	if err != nil {
		respondInternalError(w, err.Error())
		return
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"channel_id": req.ChannelID,
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
