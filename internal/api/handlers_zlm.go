package api

import (
	"fmt"
	"net/http"
	"strings"

	"gb28181-onvif-server/internal/preview"

	"github.com/gorilla/mux"
)

// handleZLMStatus 获取ZLM服务器状态
func (s *Server) handleZLMStatus(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"success": true,
	}

	if s.zlmProcess != nil {
		response["process"] = s.zlmProcess.GetStatus()
	} else {
		response["process"] = map[string]interface{}{
			"running": false,
			"message": "ZLM 进程管理器未初始化",
		}
	}

	if s.zlmServer != nil {
		response["server"] = s.zlmServer.GetStatistics()
	}

	respondRaw(w, http.StatusOK, response)
}

// handleZLMProcessStatus 获取 ZLM 进程状态
func (s *Server) handleZLMProcessStatus(w http.ResponseWriter, r *http.Request) {
	if s.zlmProcess == nil {
		respondRaw(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"status": map[string]interface{}{
				"running":   false,
				"available": false,
				"message":   "ZLM 进程管理器未初始化",
			},
		})
		return
	}

	status := s.zlmProcess.GetStatus()
	status["available"] = true

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"status":  status,
	})
}

// handleZLMConfig 获取 ZLM 配置信息
func (s *Server) handleZLMConfig(w http.ResponseWriter, r *http.Request) {
	config := map[string]interface{}{
		"http": map[string]interface{}{
			"port": 8081,
		},
		"rtsp": map[string]interface{}{
			"port": 8554,
		},
		"rtmp": map[string]interface{}{
			"port": 1935,
		},
	}

	// 如果配置文件中有端口信息，使用配置文件中的值
	if s.config.ZLM != nil {
		if s.config.ZLM.HTTP != nil && s.config.ZLM.HTTP.Port > 0 {
			config["http"] = map[string]interface{}{
				"port": s.config.ZLM.HTTP.Port,
			}
		}
		if s.config.ZLM.RTSP != nil && s.config.ZLM.RTSP.Port > 0 {
			config["rtsp"] = map[string]interface{}{
				"port": s.config.ZLM.RTSP.Port,
			}
		}
		if s.config.ZLM.RTMP != nil && s.config.ZLM.RTMP.Port > 0 {
			config["rtmp"] = map[string]interface{}{
				"port": s.config.ZLM.RTMP.Port,
			}
		}
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"config":  config,
	})
}

// handleZLMProcessStart 启动 ZLM 进程
func (s *Server) handleZLMProcessStart(w http.ResponseWriter, r *http.Request) {
	if !s.checkZLMProcessAvailable(w) {
		return
	}

	if s.zlmProcess.IsRunning() {
		respondRaw(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "ZLM 进程已在运行中",
			"status":  s.zlmProcess.GetStatus(),
		})
		return
	}

	if err := s.zlmProcess.Start(); err != nil {
		respondInternalError(w, fmt.Sprintf("启动 ZLM 进程失败: %v", err))
		return
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "ZLM 进程启动成功",
		"status":  s.zlmProcess.GetStatus(),
	})
}

// handleZLMProcessStop 停止 ZLM 进程
func (s *Server) handleZLMProcessStop(w http.ResponseWriter, r *http.Request) {
	if !s.checkZLMProcessAvailable(w) {
		return
	}

	if !s.zlmProcess.IsRunning() {
		respondSuccessMsg(w, "ZLM 进程未在运行")
		return
	}

	if err := s.zlmProcess.Stop(); err != nil {
		respondInternalError(w, fmt.Sprintf("停止 ZLM 进程失败: %v", err))
		return
	}

	respondSuccessMsg(w, "ZLM 进程已停止")
}

// handleZLMProcessRestart 重启 ZLM 进程
func (s *Server) handleZLMProcessRestart(w http.ResponseWriter, r *http.Request) {
	if !s.checkZLMProcessAvailable(w) {
		return
	}

	if err := s.zlmProcess.Restart(); err != nil {
		respondInternalError(w, fmt.Sprintf("重启 ZLM 进程失败: %v", err))
		return
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "ZLM 进程重启成功",
		"status":  s.zlmProcess.GetStatus(),
	})
}

// handleZLMGetStreams 获取ZLM流列表
func (s *Server) handleZLMGetStreams(w http.ResponseWriter, r *http.Request) {
	if !s.checkZLMAvailable(w) {
		return
	}

	streams, err := s.zlmServer.GetAPIClient().GetMediaList()
	if err != nil {
		respondInternalError(w, fmt.Sprintf("获取流列表失败: %v", err))
		return
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"streams": streams,
	})
}

// handleZLMAddStream 添加ZLM流代理
func (s *Server) handleZLMAddStream(w http.ResponseWriter, r *http.Request) {
	if !s.checkZLMAvailable(w) {
		return
	}

	var req struct {
		URL      string `json:"url"`
		App      string `json:"app"`
		StreamID string `json:"stream_id"`
	}

	if err := decodeJSON(r, &req); err != nil {
		respondBadRequest(w, "无效的请求参数")
		return
	}

	if req.App == "" {
		req.App = "live"
	}

	// 简单校验 URL 合法性
	if !strings.HasPrefix(req.URL, "rtsp://") && !strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "rtsps://") {
		respondBadRequest(w, "流地址必须以 rtsp://, rtsps:// 或 http:// 开头")
		return
	}

	zlmHost := s.getZLMHost(r)
	httpPort, rtmpPort, _ := s.getZLMPorts()

	var res *preview.PreviewResult
	var err error
	if s.previewManager != nil {
		res, err = s.previewManager.StartRTSPProxy(req.StreamID, req.URL, req.App, zlmHost, httpPort, rtmpPort, "", "")
	} else {
		// 添加流代理时启用TCP模式以支持长连接
		_, err = s.zlmServer.GetAPIClient().AddStreamProxy(req.URL, req.App, req.StreamID)
		if err == nil {
			// 构造 PreviewResult 仅用于流地址返回
			res = &preview.PreviewResult{
				DeviceID: req.StreamID,
				StreamID: req.StreamID,
				FlvURL:   fmt.Sprintf("http://%s:%d/zlm/%s/%s.live.flv", zlmHost, httpPort, req.App, req.StreamID),
				WsFlvURL: fmt.Sprintf("http://%s:%d/zlm/%s/%s.live.flv", zlmHost, httpPort, req.App, req.StreamID),
				HlsURL:   fmt.Sprintf("http://%s:%d/zlm/%s/%s/hls.m3u8", zlmHost, httpPort, req.App, req.StreamID),
				RtmpURL:  fmt.Sprintf("rtmp://%s:%d/%s/%s", zlmHost, rtmpPort, req.App, req.StreamID),
			}
		}
	}

	if err != nil {
		respondInternalError(w, fmt.Sprintf("添加流代理失败: %v", err))
		return
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"message":   "流代理添加成功",
		"proxy_key": "",
		"urls": map[string]string{
			"flv":    res.FlvURL,
			"ws_flv": res.WsFlvURL,
			"hls":    res.HlsURL,
			"rtmp":   res.RtmpURL,
		},
	})
}

// handleZLMRemoveStream 删除ZLM流
func (s *Server) handleZLMRemoveStream(w http.ResponseWriter, r *http.Request) {
	if !s.checkZLMAvailable(w) {
		return
	}

	streamID := mux.Vars(r)["id"]
	app := r.URL.Query().Get("app")
	if app == "" {
		app = "live"
	}

	var err error
	if s.previewManager != nil {
		// RTSP代理流优先用 StopRTSPProxy
		err = s.previewManager.StopRTSPProxy(streamID, app)
		if err != nil {
			// GB28181通道流用 StopChannelPreview
			err = s.previewManager.StopChannelPreview(streamID, streamID)
		}
	}

	if err != nil && s.zlmServer != nil && s.zlmServer.GetAPIClient() != nil {
		_ = s.zlmServer.GetAPIClient().CloseRtpServer(streamID)
		err = s.zlmServer.GetAPIClient().CloseStream(app, streamID)
	}

	if err != nil {
		respondInternalError(w, fmt.Sprintf("删除流失败: %v", err))
		return
	}

	respondSuccessMsg(w, "流已删除")
}

// handleZLMStartRecording 启动ZLM录像
func (s *Server) handleZLMStartRecording(w http.ResponseWriter, r *http.Request) {
	if !s.checkZLMAvailable(w) {
		return
	}

	streamID := mux.Vars(r)["id"]

	var req struct {
		RecordingPath string `json:"recording_path"`
	}
	decodeJSON(r, &req)

	if err := s.zlmServer.StartRecording(streamID, req.RecordingPath); err != nil {
		respondInternalError(w, fmt.Sprintf("启动录像失败: %v", err))
		return
	}

	respondSuccessMsg(w, "录像已启动")
}

// handleZLMStopRecording 停止ZLM录像
func (s *Server) handleZLMStopRecording(w http.ResponseWriter, r *http.Request) {
	if !s.checkZLMAvailable(w) {
		return
	}

	streamID := mux.Vars(r)["id"]

	if err := s.zlmServer.StopRecording(streamID); err != nil {
		respondInternalError(w, fmt.Sprintf("停止录像失败: %v", err))
		return
	}

	respondSuccessMsg(w, "录像已停止")
}
