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

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"config":  s.config.AI,
	})
}
