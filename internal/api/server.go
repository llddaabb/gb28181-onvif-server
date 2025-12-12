package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"gb28181-onvif-server/internal/ai"
	"gb28181-onvif-server/internal/config"
	"gb28181-onvif-server/internal/debug"
	"gb28181-onvif-server/internal/gb28181"
	"gb28181-onvif-server/internal/onvif"
	"gb28181-onvif-server/internal/preview"
	"gb28181-onvif-server/internal/storage"
	"gb28181-onvif-server/internal/zlm"

	"github.com/gorilla/mux"
)

// Server API服务器结构体
type Server struct {
	config             *config.Config
	gb28181Server      *gb28181.Server
	onvifManager       *onvif.Manager
	zlmServer          *zlm.ZLMServer
	previewManager     *preview.Manager
	zlmProcess         *zlm.ProcessManager
	diskManager        *storage.DiskManager
	aiManager          *ai.AIRecordingManager
	server             *http.Server
	configPath         string
	channelManager     *ChannelManager
	recordingManager   *RecordingManager
	streamManager      *StreamManager
	startTime          time.Time
	recordingWatchStop chan struct{}
}

// NewServer 创建一个新的API服务器实例。
// 如果提供了zlmSrv，它还会初始化预览管理器。
func NewServer(cfg *config.Config, gbServer *gb28181.Server, onvifMgr *onvif.Manager, zlmSrv *zlm.ZLMServer, configPath string) *Server {
	s := &Server{
		config:           cfg,
		gb28181Server:    gbServer,
		onvifManager:     onvifMgr,
		zlmServer:        zlmSrv,
		zlmProcess:       nil,
		configPath:       configPath,
		channelManager:   NewChannelManager(),
		recordingManager: NewRecordingManager(),
		streamManager:    NewStreamManager(),
		startTime:        time.Now(),
	}
	if zlmSrv != nil {
		s.previewManager = preview.NewManager(gbServer, zlmSrv)
	}
	return s
}

// SetZLMProcess 设置 ZLM 进程管理器
func (s *Server) SetZLMProcess(pm *zlm.ProcessManager) {
	s.zlmProcess = pm
}

// PreviewManager 返回预览管理器实例
func (s *Server) PreviewManager() *preview.Manager {
	return s.previewManager
}

// SetDiskManager 设置磁盘管理器
func (s *Server) SetDiskManager(dm *storage.DiskManager) {
	s.diskManager = dm
}

// InitAIManager 初始化AI录像管理器
func (s *Server) InitAIManager() error {
	if s.config.AI == nil || !s.config.AI.Enable {
		debug.Info("api", "AI功能未启用")
		return nil
	}

	recordControl := func(channelID string, start bool) error {
		if start {
			debug.Info("ai", "AI触发录像启动: channelID=%s", channelID)
			return nil
		} else {
			debug.Info("ai", "AI触发录像停止: channelID=%s", channelID)
			return nil
		}
	}

	s.aiManager = ai.NewAIRecordingManager(recordControl)
	debug.Info("api", "AI录像管理器已初始化")
	return nil
}

// SyncGB28181Channel 同步GB28181通道到API通道管理器
func (s *Server) SyncGB28181Channel(channel *gb28181.Channel) error {
	apiChannel := &Channel{
		ChannelID:   channel.ChannelID,
		ChannelName: channel.Name,
		DeviceID:    channel.DeviceID,
		DeviceType:  "gb28181",
		Status:      channel.Status,
		StreamURL:   "",
	}

	if existingChannel, exists := s.channelManager.GetChannel(channel.ChannelID); exists {
		existingChannel.ChannelName = channel.Name
		existingChannel.Status = channel.Status
		return s.channelManager.UpdateChannel(existingChannel)
	} else {
		return s.channelManager.AddChannel(apiChannel)
	}
}

// Start 启动API服务器
func (s *Server) Start() error {
	r := mux.NewRouter()

	r.Use(s.corsMiddleware)
	r.Use(s.loggingMiddleware)

	s.setupRoutes(r)

	s.server = &http.Server{
		Addr:        fmt.Sprintf("%s:%d", s.config.API.Host, s.config.API.Port),
		Handler:     r,
		ReadTimeout: 15 * time.Second,
		// Disable WriteTimeout to allow long-lived streaming proxied requests
		// (reverse proxy may stream data for an extended period). A non-zero
		// WriteTimeout can cause the server to cancel the request context
		// and produce "context canceled" errors observed in logs.
		WriteTimeout: 0,
		IdleTimeout:  60 * time.Second,
	}

	log.Println("═══════════════════════════════════════════════════════════")
	log.Printf("[API] ✓ REST API服务器启动成功")
	log.Printf("[API] 监听地址: http://%s:%d", s.config.API.Host, s.config.API.Port)
	log.Printf("[API] 配置 - CORS: %v | Timeout: 15s", len(s.config.API.CorsAllowOrigins) > 0)
	log.Println("═══════════════════════════════════════════════════════════")
	debug.Info("api", "API服务器启动成功，监听地址: %s:%d", s.config.API.Host, s.config.API.Port)

	s.startRecordingWatchdog()

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		debug.Error("api", "启动API服务器失败: %v", err)
		return fmt.Errorf("启动API服务器失败: %w", err)
	}

	return nil
}

// Stop 停止API服务器
func (s *Server) Stop() error {
	s.stopRecordingWatchdog()

	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.server.Shutdown(ctx)
	}
	return nil
}

// corsMiddleware CORS中间件
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	isAllowedOrigin := func(origin string, allowedOrigins []string) bool {
		for _, o := range allowedOrigins {
			if o == origin || o == "*" {
				return true
			}
		}
		return false
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowedOrigins := s.config.API.CorsAllowOrigins
		origin := r.Header.Get("Origin")

		allowOrigin := ""
		if len(allowedOrigins) == 0 || isAllowedOrigin(origin, allowedOrigins) {
			allowOrigin = origin
		}
		// 如果没有匹配的源，则不设置 `Access-Control-Allow-Origin` 头，让浏览器执行其默认策略
		w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware 请求日志中间件
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(lw, r)
		duration := time.Since(start)
		debug.Info("api", "%s %s - %d - %s - %s",
			r.Method, r.URL.Path, lw.statusCode, r.RemoteAddr, duration)
	})
}

// loggingResponseWriter 自定义ResponseWriter用于捕获状态码
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lw *loggingResponseWriter) WriteHeader(code int) {
	lw.statusCode = code
	lw.ResponseWriter.WriteHeader(code)
}

// Hijack proxies to the underlying ResponseWriter if it supports http.Hijacker.
// This is required so reverse proxies can perform WebSocket upgrades when
// middleware wraps the original ResponseWriter.
func (lw *loggingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := lw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("underlying ResponseWriter does not support Hijacker")
	}
	return hj.Hijack()
}

// Flush proxies to the underlying ResponseWriter if it supports http.Flusher.
func (lw *loggingResponseWriter) Flush() {
	if f, ok := lw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// ReadFrom proxies to the underlying ResponseWriter if it supports io.ReaderFrom.
// This helps optimize copying large streaming responses.
func (lw *loggingResponseWriter) ReadFrom(r io.Reader) (int64, error) {
	if rf, ok := lw.ResponseWriter.(io.ReaderFrom); ok {
		return rf.ReadFrom(r)
	}
	return 0, fmt.Errorf("underlying ResponseWriter does not support ReadFrom")
}

// jsonResponse 发送 JSON 响应（保留用于向后兼容）
func (s *Server) jsonResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// jsonError 发送 JSON 错误响应（保留用于向后兼容）
func (s *Server) jsonError(w http.ResponseWriter, statusCode int, message string) {
	s.jsonResponse(w, statusCode, map[string]interface{}{
		"success": false,
		"error":   message,
	})
}

// setupRoutes 设置API路由
func (s *Server) setupRoutes(r *mux.Router) {
	// 健康检查和系统状态
	r.HandleFunc("/api/health", s.handleHealthCheck).Methods("GET")
	r.HandleFunc("/api/status", s.handleGetStatus).Methods("GET")
	r.HandleFunc("/api/stats", s.handleGetStats).Methods("GET")
	r.HandleFunc("/api/resources", s.handleGetResources).Methods("GET")
	r.HandleFunc("/api/logs/latest", s.handleGetLatestLogs).Methods("GET")

	// 配置管理
	r.HandleFunc("/api/config", s.handleGetConfig).Methods("GET")
	r.HandleFunc("/api/config", s.handleUpdateConfig).Methods("PUT")

	// GB28181设备API
	gb28181Group := r.PathPrefix("/api/gb28181").Subrouter()
	gb28181Group.HandleFunc("/devices", s.handleGetGB28181Devices).Methods("GET")
	gb28181Group.HandleFunc("/devices/{id}", s.handleGetGB28181Device).Methods("GET")
	gb28181Group.HandleFunc("/devices/{id}", s.handleRemoveGB28181Device).Methods("DELETE")
	gb28181Group.HandleFunc("/devices/{id}/channels", s.handleGetGB28181Channels).Methods("GET")
	gb28181Group.HandleFunc("/devices/{id}/catalog", s.handleGB28181Catalog).Methods("POST")
	gb28181Group.HandleFunc("/devices/{id}/refresh", s.handleRefreshGB28181Device).Methods("POST")
	gb28181Group.HandleFunc("/devices/{id}/preview/start", s.handleStartGB28181Preview).Methods("POST")
	gb28181Group.HandleFunc("/devices/{id}/preview/stop", s.handleStopGB28181Preview).Methods("POST")
	gb28181Group.HandleFunc("/devices/{id}/channels/{channelId}/preview/start", s.handleStartGB28181ChannelPreview).Methods("POST")
	gb28181Group.HandleFunc("/devices/{id}/channels/{channelId}/preview/stop", s.handleStopGB28181ChannelPreview).Methods("POST")
	gb28181Group.HandleFunc("/devices/{id}/channels/{channelId}/preview/test", s.handleTestGB28181ChannelPreview).Methods("POST")
	gb28181Group.HandleFunc("/devices/{id}/ptz", s.handleGB28181PTZ).Methods("POST")
	gb28181Group.HandleFunc("/discover", s.handleDiscoverGB28181Devices).Methods("POST")
	gb28181Group.HandleFunc("/statistics", s.handleGetGB28181Statistics).Methods("GET")
	gb28181Group.HandleFunc("/server-config", s.handleGetGB28181ServerConfig).Methods("GET")
	gb28181Group.HandleFunc("/server-config", s.handleUpdateGB28181ServerConfig).Methods("PUT")

	// ONVIF设备API
	onvifGroup := r.PathPrefix("/api/onvif").Subrouter()
	onvifGroup.HandleFunc("/devices", s.handleGetONVIFDevices).Methods("GET")
	onvifGroup.HandleFunc("/devices", s.handleAddONVIFDevice).Methods("POST")
	onvifGroup.HandleFunc("/devices/{id}", s.handleGetONVIFDevice).Methods("GET")
	onvifGroup.HandleFunc("/devices/{id}", s.handleRemoveONVIFDevice).Methods("DELETE")
	onvifGroup.HandleFunc("/devices/{id}/refresh", s.handleRefreshONVIFDevice).Methods("PUT")
	onvifGroup.HandleFunc("/devices/{id}/profiles", s.handleGetONVIFProfiles).Methods("GET")
	onvifGroup.HandleFunc("/devices/{id}/snapshot", s.handleGetONVIFSnapshot).Methods("GET")
	onvifGroup.HandleFunc("/devices/{id}/presets", s.handleGetONVIFPresets).Methods("GET")
	onvifGroup.HandleFunc("/devices/{id}/auth/check", s.handleCheckONVIFAuth).Methods("POST")
	onvifGroup.HandleFunc("/devices/{id}/channels/sync", s.handleSyncONVIFChannels).Methods("POST")
	onvifGroup.HandleFunc("/devices/{id}/preset", s.handleSetONVIFPreset).Methods("POST")
	onvifGroup.HandleFunc("/devices/{id}/preset/{token}", s.handleGotoONVIFPreset).Methods("POST")
	onvifGroup.HandleFunc("/devices/{id}/preview/start", s.handleStartONVIFPreview).Methods("POST")
	onvifGroup.HandleFunc("/devices/{id}/preview/stop", s.handleStopONVIFPreview).Methods("POST")
	onvifGroup.HandleFunc("/discover", s.handleDiscoverONVIFDevices).Methods("POST")
	onvifGroup.HandleFunc("/batch-add", s.handleBatchAddONVIFDevices).Methods("POST")
	onvifGroup.HandleFunc("/statistics", s.handleGetONVIFStatistics).Methods("GET")

	// 媒体流API
	streamGroup := r.PathPrefix("/api/stream").Subrouter()
	streamGroup.HandleFunc("/start", s.handleStartStream).Methods("POST")
	streamGroup.HandleFunc("/stop", s.handleStopStream).Methods("POST")
	streamGroup.HandleFunc("/list", s.handleListStreams).Methods("GET")

	// 通道管理API
	channelGroup := r.PathPrefix("/api/channel").Subrouter()
	channelGroup.HandleFunc("/list", s.handleListChannels).Methods("GET")
	channelGroup.HandleFunc("/import", s.handleImportChannels).Methods("POST")
	channelGroup.HandleFunc("/add", s.handleAddChannel).Methods("POST")
	channelGroup.HandleFunc("/{id}", s.handleDeleteChannel).Methods("DELETE")
	channelGroup.HandleFunc("/{id}", s.handleGetChannel).Methods("GET")
	channelGroup.HandleFunc("/{id}/recording/start", s.handleStartChannelRecording).Methods("POST")
	channelGroup.HandleFunc("/{id}/recording/stop", s.handleStopChannelRecording).Methods("POST")
	channelGroup.HandleFunc("/{id}/recording/status", s.handleGetChannelRecordingStatus).Methods("GET")

	// 录像管理API
	recordingGroup := r.PathPrefix("/api/recording").Subrouter()
	recordingGroup.HandleFunc("/zlm/list", s.handleListZLMRecordings).Methods("GET")
	recordingGroup.HandleFunc("/zlm/play/{app}/{stream}/{file}", s.handlePlayZLMRecording).Methods("GET")
	recordingGroup.HandleFunc("/query", s.handleQueryRecordings).Methods("GET")
	recordingGroup.HandleFunc("/{id}", s.handleGetRecording).Methods("GET")
	recordingGroup.HandleFunc("/{id}/download", s.handleDownloadRecording).Methods("GET")

	// 存储管理API
	storageGroup := r.PathPrefix("/api/storage").Subrouter()
	storageGroup.HandleFunc("/disks", s.handleGetDisks).Methods("GET")
	storageGroup.HandleFunc("/disks", s.handleAddDisk).Methods("POST")
	storageGroup.HandleFunc("/disks/{id}", s.handleUpdateDisk).Methods("PUT")
	storageGroup.HandleFunc("/disks/{id}", s.handleRemoveDisk).Methods("DELETE")
	storageGroup.HandleFunc("/stats", s.handleGetDiskStats).Methods("GET")
	storageGroup.HandleFunc("/recycle-policy", s.handleGetRecyclePolicy).Methods("GET")
	storageGroup.HandleFunc("/recycle-policy", s.handleSetRecyclePolicy).Methods("PUT")

	// AI录像管理API
	aiGroup := r.PathPrefix("/api/ai").Subrouter()
	aiGroup.HandleFunc("/recording/start", s.handleStartAIRecording).Methods("POST")
	aiGroup.HandleFunc("/recording/stop", s.handleStopAIRecording).Methods("POST")
	aiGroup.HandleFunc("/recording/status", s.handleGetAIRecordingStatus).Methods("GET")
	aiGroup.HandleFunc("/recording/status/all", s.handleGetAllAIRecordingStatus).Methods("GET")
	aiGroup.HandleFunc("/config", s.handleGetAIConfig).Methods("GET")
	aiGroup.HandleFunc("/config", s.handleUpdateAIConfig).Methods("PUT")

	// 设备控制API
	controlGroup := r.PathPrefix("/api/control").Subrouter()
	controlGroup.HandleFunc("/ptz", s.handlePTZControl).Methods("POST")
	controlGroup.HandleFunc("/ptz/reset", s.handlePTZReset).Methods("POST")

	// ZLM媒体服务器API
	zlmGroup := r.PathPrefix("/api/zlm").Subrouter()
	zlmGroup.HandleFunc("/status", s.handleZLMStatus).Methods("GET")
	zlmGroup.HandleFunc("/media-list", s.handleGetZLMMediaList).Methods("GET")
	zlmGroup.HandleFunc("/process/status", s.handleZLMProcessStatus).Methods("GET")
	zlmGroup.HandleFunc("/process/start", s.handleZLMProcessStart).Methods("POST")
	zlmGroup.HandleFunc("/process/stop", s.handleZLMProcessStop).Methods("POST")
	zlmGroup.HandleFunc("/process/restart", s.handleZLMProcessRestart).Methods("POST")
	if s.zlmServer != nil {
		zlmGroup.HandleFunc("/streams", s.handleZLMGetStreams).Methods("GET")
		zlmGroup.HandleFunc("/streams/add", s.handleZLMAddStream).Methods("POST")
		zlmGroup.HandleFunc("/streams/{id}/remove", s.handleZLMRemoveStream).Methods("DELETE")
		zlmGroup.HandleFunc("/recording/{id}/start", s.handleZLMStartRecording).Methods("POST")
		zlmGroup.HandleFunc("/recording/{id}/stop", s.handleZLMStopRecording).Methods("POST")
	}

	// ZLM流代理 - 解决跨域问题
	r.PathPrefix("/zlm/").HandlerFunc(s.handleZLMProxy)

	// 静态文件服务（必须在最后）

	staticDir := "frontend/dist"
	r.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", http.FileServer(http.Dir(staticDir+"/assets"))))
	r.PathPrefix("/jessibuca/").Handler(http.StripPrefix("/jessibuca/", http.FileServer(http.Dir(staticDir+"/jessibuca"))))
	r.HandleFunc("/", s.handleServeStaticFile).Methods("GET")
	r.HandleFunc("/{path:.*\\.html$}", s.handleServeStaticFile).Methods("GET")
	r.PathPrefix("/").HandlerFunc(s.handleServeStaticFile).Methods("GET")
}

// startRecordingWatchdog 启动录像监控器
func (s *Server) startRecordingWatchdog() {
	s.recordingWatchStop = make(chan struct{})

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		debug.Info("api", "[录像监控器] 已启动，检查间隔: 10秒")

		for {
			select {
			case <-ticker.C:
				s.checkAndRestartRecordings()
			case <-s.recordingWatchStop:
				debug.Info("api", "[录像监控器] 已停止")
				return
			}
		}
	}()
}

// stopRecordingWatchdog 停止录像监控器
func (s *Server) stopRecordingWatchdog() {
	if s.recordingWatchStop != nil {
		close(s.recordingWatchStop)
		s.recordingWatchStop = nil
	}
}

// checkAndRestartRecordings 检查并重启需要持久录像的通道
func (s *Server) checkAndRestartRecordings() {
	if s.zlmServer == nil || s.zlmServer.GetAPIClient() == nil {
		return
	}

	apiClient := s.zlmServer.GetAPIClient()
	persistentChannels := s.recordingManager.GetPersistentRecordings()

	if len(persistentChannels) == 0 {
		return
	}

	debug.Info("api", "[录像监控器] 检查 %d 个持久录像通道", len(persistentChannels))

	for _, channelID := range persistentChannels {
		streamID := strings.ReplaceAll(channelID, "-", "")
		if streamID == "" {
			streamID = channelID
		}

		apps := []string{"live", "rtp"}
		var foundApp, foundStream string
		var isOnline, isRecording bool

		for _, app := range apps {
			online, err := apiClient.IsStreamOnline(app, streamID)
			if err == nil && online {
				foundApp = app
				foundStream = streamID
				isOnline = true

				isRec, err := apiClient.IsRecording(app, streamID, 1)
				if err == nil {
					isRecording = isRec
				}
				break
			}

			if streamID != channelID {
				online, err := apiClient.IsStreamOnline(app, channelID)
				if err == nil && online {
					foundApp = app
					foundStream = channelID
					isOnline = true

					isRec, err := apiClient.IsRecording(app, channelID, 1)
					if err == nil {
						isRecording = isRec
					}
					break
				}
			}
		}

		if isOnline && !isRecording {
			debug.Info("api", "[录像监控器] 重启录像: channelID=%s, app=%s, stream=%s", channelID, foundApp, foundStream)
			err := apiClient.StartRecord(foundApp, foundStream, 1, "", 0)
			if err != nil {
				debug.Error("api", "[录像监控器] 重启录像失败: channelID=%s, error=%v", channelID, err)
			} else {
				debug.Info("api", "[录像监控器] 录像已重启: channelID=%s", channelID)
			}
		} else if isRecording {
			debug.Info("api", "[录像监控器] 录像正常: channelID=%s", channelID)
		} else {
			debug.Info("api", "[录像监控器] 流离线，等待重连: channelID=%s", channelID)
		}
	}
}

// PreviewSession 预览会话信息
type PreviewSession struct {
	DeviceID   string `json:"device_id"`
	StreamKey  string `json:"stream_key"`
	App        string `json:"app"`
	Stream     string `json:"stream"`
	SourceURL  string `json:"source_url"`
	FlvURL     string `json:"flv_url"`
	WsFlvURL   string `json:"ws_flv_url"`
	HlsURL     string `json:"hls_url"`
	RtmpURL    string `json:"rtmp_url"`
	RtspURL    string `json:"rtsp_url"`
	CreateTime int64  `json:"create_time"`
}

// handleZLMProxy ZLM流媒体代理，解决跨域问题
// 将 /zlm/{app}/{stream}.live.flv 代理到 ZLM 的 HTTP-FLV 服务
func (s *Server) handleZLMProxy(w http.ResponseWriter, r *http.Request) {
	// 获取 ZLM HTTP 端口
	zlmPort := s.config.ZLM.HTTP.Port
	if zlmPort == 0 {
		zlmPort = 8081
	}

	// 构建 ZLM 目标 URL - 使用 r.URL.Path 并去掉 /zlm 前缀，保留起始斜杠
	// 如果存在查询字符串，则附加 RawQuery
	rawPath := strings.TrimPrefix(r.URL.Path, "/zlm")
	if rawPath == "" {
		rawPath = "/"
	}
	if !strings.HasPrefix(rawPath, "/") {
		rawPath = "/" + rawPath
	}
	targetURL := fmt.Sprintf("http://127.0.0.1:%d%s", zlmPort, rawPath)
	if r.URL.RawQuery != "" {
		targetURL = targetURL + "?" + r.URL.RawQuery
	}
	target, err := url.Parse(targetURL)
	if err != nil {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	// 创建反向代理
	proxy := httputil.NewSingleHostReverseProxy(target)

	// 设置 Director：将请求的 scheme/host/path/query 指向 ZLM 目标的对应字段。
	// 使用精确赋值可避免因路径拼接导致的错误路由（例如重复前缀），
	// 但仍保留原始请求的 header（例如 Origin、Authorization 等）。
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path = target.Path
		req.URL.RawQuery = target.RawQuery
		// 保持原始 Host 头为目标主机，方便 ZLM 根据 Host 处理请求
		req.Host = target.Host
	}

	// 修改响应，添加 CORS 头（清除可能重复的头）
	proxy.ModifyResponse = func(resp *http.Response) error {
		// 清除可能重复的 CORS 头
		resp.Header.Del("Access-Control-Allow-Origin")
		resp.Header.Del("Access-Control-Allow-Methods")
		resp.Header.Del("Access-Control-Allow-Headers")
		resp.Header.Del("Access-Control-Expose-Headers")
		// 重新设置正确的 CORS 头
		resp.Header.Set("Access-Control-Allow-Origin", "*")
		resp.Header.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		resp.Header.Set("Access-Control-Allow-Headers", "Content-Type, Range")
		resp.Header.Set("Access-Control-Expose-Headers", "Content-Length, Content-Range")
		return nil
	}

	// 处理 OPTIONS 预检请求
	if r.Method == "OPTIONS" {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Range")
		w.WriteHeader(http.StatusOK)
		return
	}

	proxy.ServeHTTP(w, r)
}

// startPreview 是一个统一的预览启动函数
// 对于 GB28181，rtspURL 为空；对于 ONVIF，rtspURL 不为空
func (s *Server) startPreview(r *http.Request, deviceID, channelID, rtspURL, app string) (*preview.PreviewResult, error) {
	if s.previewManager == nil {
		return nil, fmt.Errorf("preview manager 未初始化")
	}

	zlmHost := s.getZLMHost(r)
	httpPort, rtmpPort, _ := s.getZLMPorts()

	var res *preview.PreviewResult
	var err error

	if rtspURL != "" {
		// ONVIF 或其他 RTSP 流
		res, err = s.previewManager.StartRTSPProxy(deviceID, rtspURL, app, zlmHost, httpPort, rtmpPort)
	} else {
		// GB28181 流
		res, err = s.previewManager.StartChannelPreview(deviceID, channelID, app, zlmHost, httpPort, rtmpPort)
	}

	if err != nil {
		return nil, err
	}

	// 构建可访问的 URL
	urls := s.buildStreamURLs(r, app, res.StreamID)
	res.FlvURL = urls.FlvURL
	res.WsFlvURL = urls.WsFlvURL
	res.HlsURL = urls.HlsURL
	// RtmpURL 已经在 preview.Manager 中生成

	return res, nil
}
