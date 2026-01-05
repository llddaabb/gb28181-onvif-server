package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
)

// handleListStreams 获取流列表
func (s *Server) handleListStreams(w http.ResponseWriter, r *http.Request) {
	streams := s.streamManager.GetStreams()

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"streams": streams,
	})
}

// handleStartStream 开始媒体流
func (s *Server) handleStartStream(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID     string `json:"deviceId"`
		DeviceType   string `json:"deviceType"` // "gb28181" or "onvif"
		Channel      string `json:"channel,omitempty"`
		ChannelID    string `json:"channelId,omitempty"` // 兼容字段
		ProfileToken string `json:"profileToken,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, fmt.Sprintf("无效的请求参数: %v", err))
		return
	}

	// 兼容处理
	if req.Channel == "" && req.ChannelID != "" {
		req.Channel = req.ChannelID
	}

	var streamURL string
	var result interface{}

	switch req.DeviceType {
	case "gb28181":
		if req.Channel == "" {
			respondBadRequest(w, "GB28181设备需要提供channel或channelId")
			return
		}

		// 使用统一的预览启动函数
		app := "rtp"
		previewResult, previewErr := s.startPreview(r, req.DeviceID, req.Channel, "", app)
		if previewErr != nil {
			respondInternalError(w, fmt.Sprintf("GB28181流启动失败: %v", previewErr))
			return
		}

		streamURL = previewResult.FlvURL
		result = previewResult

	case "onvif":
		if s.onvifManager == nil {
			respondServiceUnavailable(w, "ONVIF服务未启动")
			return
		}

		// 获取RTSP URL
		rtspURL, err := s.onvifManager.GetStreamURI(req.DeviceID, req.ProfileToken)
		if err != nil {
			respondInternalError(w, fmt.Sprintf("获取ONVIF流地址失败: %v", err))
			return
		}

		// 使用统一的预览启动函数
		app := "onvif"
		previewResult, err := s.startPreview(r, req.DeviceID, req.Channel, rtspURL, app)
		if err != nil {
			respondInternalError(w, fmt.Sprintf("ONVIF流启动失败: %v", err))
			return
		}

		streamURL = previewResult.FlvURL
		result = previewResult

	default:
		respondBadRequest(w, "无效的设备类型，支持: gb28181, onvif")
		return
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"status":    "ok",
		"streamUrl": streamURL,
		"result":    result,
	})
}

// handleStopStream 停止媒体流
func (s *Server) handleStopStream(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID   string `json:"deviceId"`
		ChannelID  string `json:"channelId,omitempty"`
		DeviceType string `json:"deviceType,omitempty"`
		App        string `json:"app,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, fmt.Sprintf("无效的请求参数: %v", err))
		return
	}

	if req.DeviceID == "" {
		respondBadRequest(w, "缺少必要参数: deviceId")
		return
	}

	// 查找并停止预览会话
	key := fmt.Sprintf("%s:%s", req.DeviceID, req.ChannelID)
	session, exists := s.previewSessions.Get(key)

	var app string
	if exists {
		app = session.App
	} else if req.App != "" {
		app = req.App
	} else {
		// 根据设备类型推断app
		if req.DeviceType == "onvif" {
			app = "onvif"
		} else {
			app = "rtp"
		}
	}

	// 停止预览
	if s.previewManager != nil {
		var stopErr error
		if exists && session.DeviceType == "onvif" {
			stopErr = s.previewManager.StopRTSPProxy(req.DeviceID, app)
		} else {
			stopErr = s.previewManager.StopChannelPreview(req.DeviceID, req.ChannelID)
		}
		if stopErr != nil {
			respondInternalError(w, fmt.Sprintf("停止流失败: %v", stopErr))
			return
		}
	}

	// 移除会话记录
	if exists {
		s.previewSessions.Remove(key)
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"status":  "ok",
		"message": "流已停止",
	})
}

// handleQueryRecordings 查询录像
func (s *Server) handleQueryRecordings(w http.ResponseWriter, r *http.Request) {
	channelID := r.URL.Query().Get("channelId")
	dateStr := r.URL.Query().Get("date")

	if channelID == "" || dateStr == "" {
		respondBadRequest(w, "缺少必要参数: channelId 或 date")
		return
	}

	// 解析日期
	date, err := parseDate(dateStr)
	if err != nil {
		respondBadRequest(w, fmt.Sprintf("无效的日期格式: %v", err))
		return
	}

	recordings := s.recordingManager.GetRecordingsByDate(channelID, date)

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"recordings": recordings,
	})
}

// handleGetRecording 获取单个录像
func (s *Server) handleGetRecording(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	recordingID := params["id"]

	recording, exists := s.recordingManager.GetRecording(recordingID)
	if !exists {
		respondNotFound(w, "录像不存在")
		return
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"recording": recording,
	})
}

// handleDownloadRecording 下载录像
func (s *Server) handleDownloadRecording(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	recordingID := params["id"]

	recording, exists := s.recordingManager.GetRecording(recordingID)
	if !exists {
		respondNotFound(w, "录像不存在")
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="recording_%s.mp4"`, recordingID))
	w.Header().Set("Content-Type", "video/mp4")

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"status":      "ok",
		"recordingId": recordingID,
		"filePath":    recording.FilePath,
	})
}
