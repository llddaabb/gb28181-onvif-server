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
		ProfileToken string `json:"profileToken,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, fmt.Sprintf("无效的请求参数: %v", err))
		return
	}

	var streamURL string
	var err error

	switch req.DeviceType {
	case "gb28181":
		streamURL = fmt.Sprintf("rtsp://%s:%d/stream/%s", s.config.API.Host, 554, req.DeviceID)
	case "onvif":
		streamURL, err = s.onvifManager.StartStream(req.DeviceID, req.ProfileToken)
		if err != nil {
			respondInternalError(w, fmt.Sprintf("ONVIF流启动失败: %v", err))
			return
		}
	default:
		respondBadRequest(w, "无效的设备类型")
		return
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"streamUrl": streamURL,
	})
}

// handleStopStream 停止媒体流
func (s *Server) handleStopStream(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID   string `json:"deviceId"`
		DeviceType string `json:"deviceType"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, fmt.Sprintf("无效的请求参数: %v", err))
		return
	}

	// 简化处理
	respondRaw(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
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
