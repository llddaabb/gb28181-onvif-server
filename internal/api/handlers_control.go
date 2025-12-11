package api

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// handlePTZControl PTZ控制
func (s *Server) handlePTZControl(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID string `json:"deviceId"`
		Channel  string `json:"channel"`
		PTZCmd   string `json:"ptzCmd"`
		Speed    int    `json:"speed"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, fmt.Sprintf("无效的请求参数: %v", err))
		return
	}

	err := s.gb28181Server.SendPTZCommand(req.DeviceID, req.Channel, req.PTZCmd, req.Speed)
	if err != nil {
		respondInternalError(w, fmt.Sprintf("PTZ控制失败: %v", err))
		return
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"message": "PTZ控制命令发送成功",
	})
}

// handlePTZReset PTZ复位
func (s *Server) handlePTZReset(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID string `json:"deviceId"`
		Channel  string `json:"channel"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, fmt.Sprintf("无效的请求参数: %v", err))
		return
	}

	err := s.gb28181Server.ResetPTZ(req.DeviceID, req.Channel)
	if err != nil {
		respondInternalError(w, fmt.Sprintf("PTZ复位失败: %v", err))
		return
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"message": "PTZ复位命令发送成功",
	})
}
