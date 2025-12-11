package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gb28181-onvif-server/internal/debug"

	"github.com/gorilla/mux"
)

// handleListChannels 获取通道列表
func (s *Server) handleListChannels(w http.ResponseWriter, r *http.Request) {
	channels := s.channelManager.GetChannels()

	// 为每个通道添加AI录像状态
	channelsWithAI := make([]map[string]interface{}, 0, len(channels))
	for _, channel := range channels {
		channelData := map[string]interface{}{
			"channelId":    channel.ChannelID,
			"channelName":  channel.ChannelName,
			"deviceId":     channel.DeviceID,
			"deviceType":   channel.DeviceType,
			"status":       channel.Status,
			"streamUrl":    channel.StreamURL,
			"channel":      channel.Channel,
			"profileToken": channel.ProfileToken,
		}

		// 获取AI录像状态
		if s.aiManager != nil {
			if aiStatus, err := s.aiManager.GetChannelStatus(channel.ChannelID); err == nil {
				channelData["aiRecording"] = aiStatus
			}
		}

		channelsWithAI = append(channelsWithAI, channelData)
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"channels": channelsWithAI,
	})
}

func (s *Server) handleGetChannel(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	channelID := params["id"]

	channel, exists := s.channelManager.GetChannel(channelID)
	if !exists {
		respondNotFound(w, "通道不存在")
		return
	}

	channelData := map[string]interface{}{
		"channelId":    channel.ChannelID,
		"channelName":  channel.ChannelName,
		"deviceId":     channel.DeviceID,
		"deviceType":   channel.DeviceType,
		"status":       channel.Status,
		"streamUrl":    channel.StreamURL,
		"channel":      channel.Channel,
		"profileToken": channel.ProfileToken,
	}

	// 获取AI录像状态
	if s.aiManager != nil {
		if aiStatus, err := s.aiManager.GetChannelStatus(channelID); err == nil {
			channelData["aiRecording"] = aiStatus
		}
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"channel": channelData,
	})
}

// handleAddChannel 添加通道
func (s *Server) handleAddChannel(w http.ResponseWriter, r *http.Request) {
	var channel Channel
	if err := json.NewDecoder(r.Body).Decode(&channel); err != nil {
		respondBadRequest(w, fmt.Sprintf("无效的请求参数: %v", err))
		return
	}

	// 如果前端提交了通道标识（可能在 channelId 或 channel 字段），则先验证该设备是否包含此通道
	requestedID := ""
	if channel.ChannelID != "" {
		requestedID = channel.ChannelID
	} else if channel.Channel != "" {
		requestedID = channel.Channel
	}

	if requestedID != "" {
		// 仅在 GB28181 设备上验证 channel existence
		if channel.DeviceType == "gb28181" && s.gb28181Server != nil {
			if ch, ok := s.gb28181Server.GetChannelByID(requestedID); ok {
				// 使用设备侧的 channel ID 作为 API 通道 ID
				channel.ChannelID = ch.ChannelID
			} else {
				respondBadRequest(w, "指定的通道在设备上不存在")
				return
			}
		} else {
			// 对于非 GB28181 设备，目前不支持前端提交已有通道ID的验证
			respondBadRequest(w, "当前设备类型不支持使用外部通道ID添加")
			return
		}
	} else {
		// 未提供通道标识，由后端生成唯一通道ID
		channel.ChannelID = fmt.Sprintf("ch_%s_%d", channel.DeviceID, time.Now().Unix())
	}

	if err := s.channelManager.AddChannel(&channel); err != nil {
		respondInternalError(w, fmt.Sprintf("添加通道失败: %v", err))
		return
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"message":   "通道添加成功",
		"channelId": channel.ChannelID,
		"channel":   channel,
	})
}

// handleImportChannels 批量导入通道
func (s *Server) handleImportChannels(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Channels []*Channel `json:"channels"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, fmt.Sprintf("无效的请求参数: %v", err))
		return
	}

	if len(req.Channels) == 0 {
		respondBadRequest(w, "导入的通道列表不能为空")
		return
	}

	addedCount := 0
	failedCount := 0
	var errors []string

	for _, channel := range req.Channels {
		if channel.ChannelID == "" {
			channel.ChannelID = fmt.Sprintf("ch_%s_%d", channel.DeviceID, time.Now().UnixNano())
		}
		if err := s.channelManager.AddChannel(channel); err != nil {
			failedCount++
			errors = append(errors, fmt.Sprintf("通道 %s: %v", channel.ChannelID, err))
		} else {
			addedCount++
		}
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success":     true,
		"message":     fmt.Sprintf("导入完成: 成功 %d, 失败 %d", addedCount, failedCount),
		"addedCount":  addedCount,
		"failedCount": failedCount,
		"errors":      errors,
	})
}

// handleDeleteChannel 删除通道
func (s *Server) handleDeleteChannel(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	channelID := params["id"]

	if err := s.channelManager.DeleteChannel(channelID); err != nil {
		respondInternalError(w, fmt.Sprintf("删除通道失败: %v", err))
		return
	}

	respondSuccessMsg(w, "通道删除成功")
}

// handleStartChannelRecording 开始通道录像
func (s *Server) handleStartChannelRecording(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	channelID := params["id"]

	debug.Info("api", "开始通道录像: channelID=%s", channelID)

	if !s.checkZLMAvailable(w) {
		return
	}

	apiClient := s.zlmServer.GetAPIClient()

	// 标准化流ID
	streamID := strings.ReplaceAll(channelID, "-", "")
	if streamID == "" {
		streamID = channelID
	}

	// 查找流所在的 app
	apps := []string{"live", "rtp"}
	var foundApp, foundStream string

	for _, app := range apps {
		online, err := apiClient.IsStreamOnline(app, streamID)
		if err == nil && online {
			foundApp = app
			foundStream = streamID
			break
		}
		if streamID != channelID {
			online, err := apiClient.IsStreamOnline(app, channelID)
			if err == nil && online {
				foundApp = app
				foundStream = channelID
				break
			}
		}
	}

	// 开始 MP4 录像 (type=1)
	err := apiClient.StartRecord(foundApp, foundStream, 1, "", 0)
	if err != nil {
		debug.Error("api", "开始录像失败: %v", err)
		respondInternalError(w, fmt.Sprintf("开始录像失败: %v", err))
		return
	}

	// 标记为持久录像
	s.recordingManager.SetPersistentRecording(channelID, true)

	debug.Info("api", "通道录像已开始（持久录像）: channelID=%s, app=%s, stream=%s", channelID, foundApp, foundStream)

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"message":   "录像已开始",
		"channelId": channelID,
		"app":       foundApp,
		"stream":    foundStream,
	})
}

// handleStopChannelRecording 停止通道录像
func (s *Server) handleStopChannelRecording(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	channelID := params["id"]

	debug.Info("api", "停止通道录像: channelID=%s", channelID)

	if !s.checkZLMAvailable(w) {
		return
	}

	apiClient := s.zlmServer.GetAPIClient()

	// 标准化流ID
	streamID := strings.ReplaceAll(channelID, "-", "")
	if streamID == "" {
		streamID = channelID
	}

	// 查找流所在的 app
	apps := []string{"live", "rtp"}
	var foundApp, foundStream string

	for _, app := range apps {
		isRec, err := apiClient.IsRecording(app, streamID, 1)
		if err == nil && isRec {
			foundApp = app
			foundStream = streamID
			break
		}
		if streamID != channelID {
			isRec, err := apiClient.IsRecording(app, channelID, 1)
			if err == nil && isRec {
				foundApp = app
				foundStream = channelID
				break
			}
		}
	}

	if foundApp == "" {
		debug.Warn("api", "未找到录像流，尝试停止所有可能位置")
		for _, app := range apps {
			apiClient.StopRecord(app, streamID, 1)
			if streamID != channelID {
				apiClient.StopRecord(app, channelID, 1)
			}
		}
	} else {
		err := apiClient.StopRecord(foundApp, foundStream, 1)
		if err != nil {
			debug.Error("api", "停止录像失败: %v", err)
			respondInternalError(w, fmt.Sprintf("停止录像失败: %v", err))
			return
		}
	}

	// 取消持久录像标记
	s.recordingManager.SetPersistentRecording(channelID, false)

	debug.Info("api", "通道录像已停止: channelID=%s", channelID)

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"message":   "录像已停止",
		"channelId": channelID,
	})
}

// handleGetChannelRecordingStatus 获取通道录像状态
func (s *Server) handleGetChannelRecordingStatus(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	channelID := params["id"]

	if s.zlmServer == nil || s.zlmServer.GetAPIClient() == nil {
		respondRaw(w, http.StatusOK, map[string]interface{}{
			"success":     true,
			"channelId":   channelID,
			"isRecording": false,
		})
		return
	}

	apiClient := s.zlmServer.GetAPIClient()

	streamID := strings.ReplaceAll(channelID, "-", "")
	if streamID == "" {
		streamID = channelID
	}

	apps := []string{"live", "rtp"}
	isRecording := false

	for _, app := range apps {
		rec, err := apiClient.IsRecording(app, streamID, 1)
		if err == nil && rec {
			isRecording = true
			break
		}
		if streamID != channelID {
			rec, err := apiClient.IsRecording(app, channelID, 1)
			if err == nil && rec {
				isRecording = true
				break
			}
		}
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success":     true,
		"channelId":   channelID,
		"isRecording": isRecording,
	})
}
