package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"gb28181-onvif-server/internal/debug"

	"github.com/gorilla/mux"
)

// handleGetPreviewSessions 获取所有预览会话
func (s *Server) handleGetPreviewSessions(w http.ResponseWriter, r *http.Request) {
	sessions := s.previewSessions.GetAll()

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"sessions": sessions,
		"count":    len(sessions),
	})
}

// handleGetPreviewSession 获取单个预览会话
func (s *Server) handleGetPreviewSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["key"]

	session, exists := s.previewSessions.Get(key)
	if !exists {
		respondNotFound(w, fmt.Sprintf("预览会话不存在: %s", key))
		return
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"session": session,
	})
}

// handleStopPreviewSession 停止预览会话
func (s *Server) handleStopPreviewSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["key"]

	session, exists := s.previewSessions.Get(key)
	if !exists {
		respondNotFound(w, fmt.Sprintf("预览会话不存在: %s", key))
		return
	}

	// 停止预览
	if s.previewManager != nil {
		var err error
		if session.DeviceType == "onvif" {
			err = s.previewManager.StopRTSPProxy(session.DeviceID, session.App)
		} else {
			err = s.previewManager.StopChannelPreview(session.DeviceID, session.ChannelID)
		}
		if err != nil {
			debug.Error("preview", "停止预览失败: deviceID=%s, app=%s, error=%v",
				session.DeviceID, session.App, err)
			// 即使停止失败也移除会话记录
		}
	}

	// 移除会话
	s.previewSessions.Remove(key)

	debug.Info("preview", "预览会话已停止: key=%s, deviceID=%s", key, session.DeviceID)

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "预览会话已停止",
	})
}

// handleStartPreview 统一的预览启动接口
func (s *Server) handleStartPreview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID     string `json:"device_id"`
		ChannelID    string `json:"channel_id,omitempty"`
		DeviceType   string `json:"device_type"` // "gb28181" or "onvif"
		RtspURL      string `json:"rtsp_url,omitempty"`
		App          string `json:"app,omitempty"` // 默认: gb28181=rtp, onvif=onvif
		ProfileToken string `json:"profile_token,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, fmt.Sprintf("无效的请求参数: %v", err))
		return
	}

	if req.DeviceID == "" {
		respondBadRequest(w, "缺少必要参数: device_id")
		return
	}

	// 确定app
	if req.App == "" {
		if req.DeviceType == "onvif" {
			req.App = "onvif"
		} else {
			req.App = "rtp"
		}
	}

	// 检查是否已存在会话
	key := fmt.Sprintf("%s:%s", req.DeviceID, req.ChannelID)
	if session, exists := s.previewSessions.Get(key); exists {
		// 会话已存在，直接返回
		respondRaw(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "预览会话已存在",
			"session": session,
		})
		return
	}

	// 根据设备类型处理
	var rtspURL string
	var err error

	if req.DeviceType == "onvif" {
		// ONVIF设备，需要先获取RTSP URL
		if req.RtspURL != "" {
			rtspURL = req.RtspURL
		} else if s.onvifManager != nil {
			// 从ONVIF管理器获取流URL
			rtspURL, err = s.onvifManager.GetStreamURI(req.DeviceID, req.ProfileToken)
			if err != nil {
				respondInternalError(w, fmt.Sprintf("获取ONVIF流地址失败: %v", err))
				return
			}
		} else {
			respondServiceUnavailable(w, "ONVIF管理器未初始化")
			return
		}
	} else if req.DeviceType == "gb28181" {
		// GB28181设备，ChannelID必需
		if req.ChannelID == "" {
			respondBadRequest(w, "GB28181设备需要提供channel_id")
			return
		}
	} else {
		respondBadRequest(w, "不支持的设备类型: "+req.DeviceType)
		return
	}

	// 启动预览
	result, err := s.startPreview(r, req.DeviceID, req.ChannelID, rtspURL, req.App)
	if err != nil {
		respondInternalError(w, fmt.Sprintf("启动预览失败: %v", err))
		return
	}

	// 获取保存的会话信息
	session, _ := s.previewSessions.Get(key)

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "预览已启动",
		"result":  result,
		"session": session,
	})
}

// handleStopPreview 统一的预览停止接口
func (s *Server) handleStopPreview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID  string `json:"device_id"`
		ChannelID string `json:"channel_id,omitempty"`
		App       string `json:"app,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, fmt.Sprintf("无效的请求参数: %v", err))
		return
	}

	if req.DeviceID == "" {
		respondBadRequest(w, "缺少必要参数: device_id")
		return
	}

	// 查找会话
	key := fmt.Sprintf("%s:%s", req.DeviceID, req.ChannelID)
	session, exists := s.previewSessions.Get(key)

	var app string
	if exists {
		app = session.App
	} else if req.App != "" {
		app = req.App
	} else {
		app = "rtp" // 默认
	}

	// 停止预览
	if s.previewManager != nil {
		var err error
		if exists && session.DeviceType == "onvif" {
			err = s.previewManager.StopRTSPProxy(req.DeviceID, app)
		} else {
			err = s.previewManager.StopChannelPreview(req.DeviceID, req.ChannelID)
		}
		if err != nil {
			debug.Warn("preview", "停止预览失败: deviceID=%s, app=%s, error=%v",
				req.DeviceID, app, err)
			// 继续执行，移除会话记录
		}
	}

	// 移除会话
	if exists {
		s.previewSessions.Remove(key)
		debug.Info("preview", "预览会话已停止: key=%s", key)
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "预览已停止",
	})
}
