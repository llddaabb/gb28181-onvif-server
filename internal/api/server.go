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
	"gb28181-onvif-server/internal/auth"
	"gb28181-onvif-server/internal/config"
	"gb28181-onvif-server/internal/debug"
	"gb28181-onvif-server/internal/frontend"
	"gb28181-onvif-server/internal/gb28181"
	"gb28181-onvif-server/internal/onvif"
	"gb28181-onvif-server/internal/preview"
	"gb28181-onvif-server/internal/push"
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
	pushManager        *push.Manager
	authManager        *auth.AuthManager
	authMiddleware     *auth.Middleware
	authHandler        *auth.AuthHandler
	server             *http.Server
	configPath         string
	channelManager     *ChannelManager
	recordingManager   *RecordingManager
	streamManager      *StreamManager
	startTime          time.Time
	recordingWatchStop chan struct{}
	gb28181Running     bool                       // GB28181 服务运行状态
	onvifRunning       bool                       // ONVIF 服务运行状态
	staticServer       *frontend.StaticFileServer // 静态文件服务器
}

// NewServer 创建一个新的API服务器实例。
// 如果提供了zlmSrv，它还会初始化预览管理器。
func NewServer(cfg *config.Config, gbServer *gb28181.Server, onvifMgr *onvif.Manager, zlmSrv *zlm.ZLMServer, configPath string) *Server {
	// 初始化静态文件服务器（支持嵌入式和本地文件系统）
	var staticDirs []string
	if cfg != nil && cfg.API != nil && cfg.API.StaticDir != "" {
		staticDirs = append(staticDirs, cfg.API.StaticDir)
	}
	staticServer := frontend.NewStaticFileServer(staticDirs...)
	if staticServer.IsEmbedded() {
		log.Println("[前端] ✓ 使用嵌入式前端文件")
	} else {
		log.Printf("[前端] 使用本地前端文件: %s", staticServer.GetLocalDir())
	}

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
		gb28181Running:   true, // 默认启动时为运行状态
		onvifRunning:     true, // 默认启动时为运行状态
		staticServer:     staticServer,
	}
	if zlmSrv != nil {
		s.previewManager = preview.NewManager(gbServer, zlmSrv)
		// 初始化推流管理器
		s.pushManager = push.NewManager(zlmSrv.GetAPIClient(), "configs/push_targets.json", cfg.ZLM.HTTP.Port)
	}

	// 初始化认证模块
	s.initAuth()

	return s
}

// initAuth 初始化认证模块
func (s *Server) initAuth() {
	if s.config.Auth == nil {
		return
	}

	// 转换配置
	authConfig := &auth.AuthConfig{
		Enable:          s.config.Auth.Enable,
		JWTSecret:       s.config.Auth.JWTSecret,
		TokenExpiry:     time.Duration(s.config.Auth.TokenExpiry) * time.Hour,
		UsersFile:       s.config.Auth.UsersFile,
		DefaultAdmin:    s.config.Auth.DefaultAdmin,
		DefaultPassword: s.config.Auth.DefaultPassword,
	}

	s.authManager = auth.NewAuthManager(authConfig)
	s.authMiddleware = auth.NewMiddleware(s.authManager)
	s.authHandler = auth.NewAuthHandler(s.authManager)

	debug.Info("api", "认证模块初始化完成，启用状态: %v", s.config.Auth.Enable)
}

// GetZLMAPIClient 提供给其他模块获取 ZLM API 客户端
func (s *Server) GetZLMAPIClient() *zlm.ZLMAPIClient {
	if s.zlmServer == nil {
		return nil
	}
	return s.zlmServer.GetAPIClient()
}

// SetZLMProcess 设置 ZLM 进程管理器
func (s *Server) SetZLMProcess(pm *zlm.ProcessManager) {
	s.zlmProcess = pm
}

// SetServiceStatus 设置服务运行状态
func (s *Server) SetServiceStatus(gb28181Running, onvifRunning bool) {
	s.gb28181Running = gb28181Running
	s.onvifRunning = onvifRunning
}

// PreviewManager 返回预览管理器实例
func (s *Server) PreviewManager() *preview.Manager {
	return s.previewManager
}

// SetupAutoStreamProxy 设置 ONVIF 设备发现后自动添加流代理
func (s *Server) SetupAutoStreamProxy() {
	if s.onvifManager == nil || s.previewManager == nil || s.zlmServer == nil {
		debug.Warn("api", "自动流代理设置失败: onvifManager=%v, previewManager=%v, zlmServer=%v",
			s.onvifManager != nil, s.previewManager != nil, s.zlmServer != nil)
		return
	}

	// 获取 ZLM 端口配置
	httpPort, rtmpPort, _ := s.getZLMPorts()

	s.onvifManager.SetStreamProxyCallback(func(deviceID, rtspURL, username, password string) error {
		debug.Info("api", "自动添加流代理: deviceID=%s, rtspURL=%s", deviceID, rtspURL)
		_, err := s.previewManager.StartRTSPProxy(deviceID, rtspURL, "onvif", "127.0.0.1", httpPort, rtmpPort, username, password)
		return err
	})
	debug.Info("api", "✅ 自动流代理已设置")
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

	// 创建检测器配置
	detectorConfig := ai.DetectorConfig{
		ModelPath:    s.config.AI.ModelPath,
		Backend:      "auto",
		InputSize:    s.config.AI.InputSize,
		Confidence:   s.config.AI.Confidence,
		IoUThreshold: s.config.AI.IoUThreshold,
		NumThreads:   s.config.AI.NumThreads,
	}

	// 设置默认值
	if detectorConfig.InputSize == 0 {
		detectorConfig.InputSize = 640
	}
	if detectorConfig.Confidence == 0 {
		detectorConfig.Confidence = 0.5
	}
	if detectorConfig.IoUThreshold == 0 {
		detectorConfig.IoUThreshold = 0.45
	}

	// 创建检测器
	factoryConfig := ai.DetectorFactoryConfig{
		Type:        ai.DetectorType(s.config.AI.DetectorType),
		Config:      detectorConfig,
		APIEndpoint: s.config.AI.APIEndpoint,
	}

	// 如果类型为空，使用 auto
	if factoryConfig.Type == "" {
		factoryConfig.Type = ai.DetectorTypeAuto
	}

	detector, err := ai.CreateDetector(factoryConfig)
	if err != nil {
		return fmt.Errorf("创建AI检测器失败: %w", err)
	}

	// 记录检测器信息
	info := detector.GetModelInfo()
	log.Printf("[AI] ✓ AI检测器已创建: name=%s, backend=%s", info.Name, info.Backend)

	// 录像控制回调
	recordControl := func(channelID string, start bool) error {
		if start {
			debug.Info("ai", "AI触发录像启动: channelID=%s", channelID)

			// 调用实际的录像启动接口
			if s.zlmServer != nil && s.zlmServer.GetAPIClient() != nil {
				apiClient := s.zlmServer.GetAPIClient()
				// 使用rtp应用，recordType=1表示MP4格式
				// 不使用自定义路径，让 ZLM 使用默认的录像目录结构: {record_path}/{app}/{stream}/{date}/
				if err := apiClient.StartRecord("rtp", channelID, 1, "", 0); err != nil {
					debug.Error("ai", "启动ZLM录像失败: %v", err)
					return err
				}
				debug.Info("ai", "ZLM录像已启动: app=rtp, stream=%s", channelID)
			}
			return nil
		} else {
			debug.Info("ai", "AI触发录像停止: channelID=%s", channelID)

			// 调用实际的录像停止接口
			if s.zlmServer != nil && s.zlmServer.GetAPIClient() != nil {
				apiClient := s.zlmServer.GetAPIClient()
				// recordType=1表示MP4格式
				if err := apiClient.StopRecord("rtp", channelID, 1); err != nil {
					debug.Error("ai", "停止ZLM录像失败: %v", err)
					return err
				}
				debug.Info("ai", "ZLM录像已停止: app=rtp, stream=%s", channelID)
			}
			return nil
		}
	}

	s.aiManager = ai.NewAIRecordingManager(recordControl)
	s.aiManager.SetDetector(detector)
	s.aiManager.SetConfig(s.config.AI)

	debug.Info("api", "AI录像管理器已初始化")

	// 如果配置了自动启动，则启动AI检测
	if s.config.AI.AutoStart {
		go s.autoStartAIDetection()
	}

	return nil
}

// autoStartAIDetection 自动启动AI检测
func (s *Server) autoStartAIDetection() {
	// 等待一段时间让其他服务启动
	time.Sleep(3 * time.Second)

	log.Println("[AI] 正在自动启动AI检测...")

	// 获取要启动的通道列表
	var channelsToStart []struct {
		ID        string
		StreamURL string
	}

	if len(s.config.AI.AutoChannels) > 0 {
		// 使用配置的通道列表
		for _, chID := range s.config.AI.AutoChannels {
			if ch, exists := s.channelManager.GetChannel(chID); exists && ch != nil && ch.StreamURL != "" {
				channelsToStart = append(channelsToStart, struct {
					ID        string
					StreamURL string
				}{ID: chID, StreamURL: ch.StreamURL})
			}
		}
	} else {
		// 获取所有已配置的通道
		allChannels := s.channelManager.GetChannels()
		for _, ch := range allChannels {
			if ch.StreamURL != "" {
				channelsToStart = append(channelsToStart, struct {
					ID        string
					StreamURL string
				}{ID: ch.ChannelID, StreamURL: ch.StreamURL})
			}
		}
	}

	if len(channelsToStart) == 0 {
		log.Println("[AI] 没有可用的通道，等待通道配置后再启动AI检测")
		log.Println("[AI] 提示：GB28181通道需要先启动预览才能进行AI检测")
		return
	}

	// 启动各通道的AI检测
	startedCount := 0
	for _, ch := range channelsToStart {
		if err := s.aiManager.StartChannelRecording(ch.ID, ch.StreamURL, ai.RecordingModePerson); err != nil {
			log.Printf("[AI] 启动通道 %s 的AI检测失败: %v", ch.ID, err)
		} else {
			startedCount++
			log.Printf("[AI] ✓ 通道 %s 的AI检测已启动", ch.ID)
		}
	}

	log.Printf("[AI] ✓ 自动启动完成，共启动 %d 个通道的AI检测", startedCount)
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

	// 添加认证中间件
	if s.authMiddleware != nil {
		r.Use(s.authMiddleware.Handler)
	}

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

		allowOrigin := "*"
		if origin != "" && len(allowedOrigins) > 0 && isAllowedOrigin(origin, allowedOrigins) {
			allowOrigin = origin
		}

		w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Max-Age", "86400")

		// OPTIONS 预检请求直接返回
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

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

	// 认证API路由 - 直接使用完整路径以继承中间件
	if s.authHandler != nil {
		r.HandleFunc("/api/auth/login", s.authHandler.HandleLogin).Methods("POST", "OPTIONS")
		r.HandleFunc("/api/auth/logout", s.authHandler.HandleLogout).Methods("POST", "OPTIONS")
		r.HandleFunc("/api/auth/refresh", s.authHandler.HandleRefreshToken).Methods("POST", "OPTIONS")
		r.HandleFunc("/api/auth/user", s.authHandler.HandleGetCurrentUser).Methods("GET", "OPTIONS")
		r.HandleFunc("/api/auth/users", s.authHandler.HandleListUsers).Methods("GET", "OPTIONS")
		r.HandleFunc("/api/auth/users", s.authHandler.HandleCreateUser).Methods("POST", "OPTIONS")
		r.HandleFunc("/api/auth/users/update", s.authHandler.HandleUpdateUser).Methods("PUT", "OPTIONS")
		r.HandleFunc("/api/auth/users/delete", s.authHandler.HandleDeleteUser).Methods("DELETE", "OPTIONS")
		r.HandleFunc("/api/auth/password", s.authHandler.HandleChangePassword).Methods("PUT", "OPTIONS")
		r.HandleFunc("/api/auth/validate", s.authHandler.HandleValidateToken).Methods("GET", "OPTIONS")
	}

	// 服务控制API
	r.HandleFunc("/api/services/status", s.handleGetServiceStatus).Methods("GET")
	r.HandleFunc("/api/services/gb28181/control", s.handleControlGB28181Service).Methods("POST")
	r.HandleFunc("/api/services/onvif/control", s.handleControlONVIFService).Methods("POST")

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
	gb28181Group.HandleFunc("/devices/{id}/ptz", s.handleGB28181PTZ).Methods("POST")
	gb28181Group.HandleFunc("/discover", s.handleDiscoverGB28181Devices).Methods("POST")
	gb28181Group.HandleFunc("/statistics", s.handleGetGB28181Statistics).Methods("GET")
	gb28181Group.HandleFunc("/server-config", s.handleGetGB28181ServerConfig).Methods("GET")
	gb28181Group.HandleFunc("/server-config", s.handleUpdateGB28181ServerConfig).Methods("PUT")
	gb28181Group.HandleFunc("/record/query", s.handleGB28181QueryRecordInfo).Methods("GET")             // 设备录像查询
	gb28181Group.HandleFunc("/record/list", s.handleGB28181GetRecordList).Methods("GET")                // 获取录像列表
	gb28181Group.HandleFunc("/record/clear", s.handleGB28181ClearRecordCache).Methods("DELETE")         // 清除录像缓存
	gb28181Group.HandleFunc("/record/playback", s.handleGB28181RecordPlayback).Methods("POST")          // 设备端录像回放
	gb28181Group.HandleFunc("/record/playback/stop", s.handleGB28181StopRecordPlayback).Methods("POST") // 停止录像回放
	gb28181Group.HandleFunc("/start", s.handleStartGB28181Service).Methods("POST")                      // 启动GB28181服务
	gb28181Group.HandleFunc("/stop", s.handleStopGB28181Service).Methods("POST")                        // 停止GB28181服务

	// ONVIF设备API
	onvifGroup := r.PathPrefix("/api/onvif").Subrouter()
	onvifGroup.HandleFunc("/devices", s.handleGetONVIFDevices).Methods("GET")
	onvifGroup.HandleFunc("/devices", s.handleAddONVIFDevice).Methods("POST")
	onvifGroup.HandleFunc("/devices/{id:[^/]+}", s.handleGetONVIFDevice).Methods("GET")
	onvifGroup.HandleFunc("/devices/{id:[^/]+}", s.handleDeleteONVIFDevice).Methods("DELETE")
	onvifGroup.HandleFunc("/devices/{id:[^/]+}/refresh", s.handleRefreshONVIFDevice).Methods("POST")
	onvifGroup.HandleFunc("/batch-add", s.handleBatchAddONVIFDevices).Methods("POST")

	onvifGroup.HandleFunc("/devices/{id:[^/]+}/profiles", s.handleGetONVIFProfiles).Methods("GET")
	onvifGroup.HandleFunc("/devices/{id:[^/]+}/snapshot", s.handleGetONVIFSnapshotURI).Methods("GET")
	onvifGroup.HandleFunc("/devices/{id:[^/]+}/presets", s.handleGetONVIFPresets).Methods("GET")

	// ONVIF PTZ 控制路由
	onvifGroup.HandleFunc("/devices/{id:[^/]+}/ptz-control", s.handleONVIFPTZControl).Methods("POST")
	onvifGroup.HandleFunc("/devices/{id:[^/]+}/update-credentials", s.handleONVIFUpdateConfig).Methods("POST")
	onvifGroup.HandleFunc("/devices/{id:[^/]+}/credentials", s.handleONVIFUpdateConfig).Methods("PUT")

	onvifGroup.HandleFunc("/devices/{id:[^/]+}/preview/start", s.handleStartONVIFPreview).Methods("POST")
	onvifGroup.HandleFunc("/devices/{id:[^/]+}/preview/stop", s.handleStopONVIFPreview).Methods("POST")
	onvifGroup.HandleFunc("/discover", s.handleGetONVIFDevices).Methods("POST")

	// ONVIF 录像查询路由
	onvifGroup.HandleFunc("/devices/{id:[^/]+}/recordings", s.handleONVIFQueryRecordings).Methods("GET")
	onvifGroup.HandleFunc("/devices/{id:[^/]+}/replay-uri", s.handleONVIFGetReplayUri).Methods("GET")

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
	recordingGroup.HandleFunc("/zlm/dates", s.handleGetRecordingDates).Methods("GET") // 获取有录像的日期列表
	recordingGroup.HandleFunc("/zlm/stop", s.handleStopPlayback).Methods("POST")      // 停止回放
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
	aiGroup.HandleFunc("/recording/stop/all", s.handleStopAllAIRecording).Methods("POST")
	aiGroup.HandleFunc("/recording/status", s.handleGetAIRecordingStatus).Methods("GET")
	aiGroup.HandleFunc("/recording/status/all", s.handleGetAllAIRecordingStatus).Methods("GET")
	aiGroup.HandleFunc("/recording/list", s.handleListAIRecordings).Methods("GET")
	aiGroup.HandleFunc("/config", s.handleGetAIConfig).Methods("GET")
	aiGroup.HandleFunc("/config", s.handleUpdateAIConfig).Methods("PUT")
	aiGroup.HandleFunc("/detector/info", s.handleGetAIDetectorInfo).Methods("GET")
	aiGroup.HandleFunc("/detect", s.handleAIDetect).Methods("POST")

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

	// 推流管理API
	pushGroup := r.PathPrefix("/api/push").Subrouter()
	pushGroup.HandleFunc("/platforms", s.handleGetPushPlatforms).Methods("GET")
	pushGroup.HandleFunc("/targets", s.handleGetPushTargets).Methods("GET")
	pushGroup.HandleFunc("/targets", s.handleAddPushTarget).Methods("POST")
	pushGroup.HandleFunc("/targets/{id}", s.handleGetPushTarget).Methods("GET")
	pushGroup.HandleFunc("/targets/{id}", s.handleUpdatePushTarget).Methods("PUT")
	pushGroup.HandleFunc("/targets/{id}", s.handleDeletePushTarget).Methods("DELETE")
	pushGroup.HandleFunc("/targets/{id}/start", s.handleStartPush).Methods("POST")
	pushGroup.HandleFunc("/targets/{id}/stop", s.handleStopPush).Methods("POST")
	pushGroup.HandleFunc("/channel/{channelId}", s.handleGetChannelPushTargets).Methods("GET")

	// ZLM流代理 - 解决跨域问题
	r.PathPrefix("/zlm/").HandlerFunc(s.handleZLMProxy)

	// 静态文件服务（必须在最后）- 支持嵌入式和本地文件系统
	r.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", s.staticServer.SubDirHandler("assets")))
	r.PathPrefix("/jessibuca/").Handler(http.StripPrefix("/jessibuca/", s.staticServer.SubDirHandler("jessibuca")))
	r.PathPrefix("/h265webjs/").Handler(http.StripPrefix("/h265webjs/", s.staticServer.SubDirHandler("h265webjs")))
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
	zlmPort, _, _ := s.getZLMPorts()

	// 去掉 /zlm 前缀，获取实际的 ZLM 路径
	zlmPath := strings.TrimPrefix(r.URL.Path, "/zlm")
	if zlmPath == "" {
		zlmPath = "/"
	}

	// 构建 ZLM 目标 URL（去掉 /zlm 前缀）
	targetURL := fmt.Sprintf("http://127.0.0.1:%d%s", zlmPort, zlmPath)
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	target, err := url.Parse(targetURL)
	if err != nil {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	// 创建反向代理
	proxy := httputil.NewSingleHostReverseProxy(target)

	// Director 负责修改请求信息
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		// 保留原始 Host，只修改 URL
		req.Host = target.Host
		req.Header.Del("Connection")
		req.Header.Del("Upgrade")
	}

	// 修改响应，添加 CORS 头
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

	log.Printf("[ZLM代理] 转发请求: %s -> %s", r.URL.Path, targetURL)
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
		rtspUser := ""
		rtspPassword := ""
		// 尝试从 onvifManager 获取设备凭据（仅在 app 为 onvif 时）
		if app == "onvif" && s.onvifManager != nil {
			if dev, ok := s.onvifManager.GetDeviceByID(deviceID); ok {
				rtspUser = dev.Username
				rtspPassword = dev.Password
			}
		}
		debug.Info("preview", "添加RTSP流代理: deviceID=%s, app=%s, rtspURL=%s", deviceID, app, rtspURL)
		res, err = s.previewManager.StartRTSPProxy(deviceID, rtspURL, app, zlmHost, httpPort, rtmpPort, rtspUser, rtspPassword)
		if err == nil {
			debug.Info("preview", "RTSP流代理添加成功: streamID=%s, flvURL=%s", res.StreamID, res.FlvURL)
		}
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

// ==================== 推流管理 API ====================

// handleGetPushPlatforms 获取支持的直播平台列表
func (s *Server) handleGetPushPlatforms(w http.ResponseWriter, r *http.Request) {
	if s.pushManager == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "Push manager not initialized")
		return
	}

	platforms := s.pushManager.GetPlatforms()
	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"platforms": platforms,
	})
}

// handleGetPushTargets 获取所有推流目标
func (s *Server) handleGetPushTargets(w http.ResponseWriter, r *http.Request) {
	if s.pushManager == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "Push manager not initialized")
		return
	}

	targets := s.pushManager.GetTargets()
	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"targets": targets,
	})
}

// handleGetPushTarget 获取单个推流目标
func (s *Server) handleGetPushTarget(w http.ResponseWriter, r *http.Request) {
	if s.pushManager == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "Push manager not initialized")
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	target, err := s.pushManager.GetTarget(id)
	if err != nil {
		s.jsonError(w, http.StatusNotFound, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"target":  target,
	})
}

// handleAddPushTarget 添加推流目标
func (s *Server) handleAddPushTarget(w http.ResponseWriter, r *http.Request) {
	if s.pushManager == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "Push manager not initialized")
		return
	}

	var req struct {
		Name        string `json:"name"`
		Platform    string `json:"platform"`
		PushURL     string `json:"push_url"`
		StreamKey   string `json:"stream_key"`
		ChannelID   string `json:"channel_id"`
		ChannelName string `json:"channel_name"`
		SourceURL   string `json:"source_url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" || req.Platform == "" {
		s.jsonError(w, http.StatusBadRequest, "Name and platform are required")
		return
	}

	if req.SourceURL == "" && req.ChannelID == "" {
		s.jsonError(w, http.StatusBadRequest, "Either source_url or channel_id is required")
		return
	}

	target := &push.PushTarget{
		Name:        req.Name,
		Platform:    req.Platform,
		PushURL:     req.PushURL,
		StreamKey:   req.StreamKey,
		ChannelID:   req.ChannelID,
		ChannelName: req.ChannelName,
		SourceURL:   req.SourceURL,
	}

	if err := s.pushManager.AddTarget(target); err != nil {
		s.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"target":  target,
		"message": "Push target added successfully",
	})
}

// handleUpdatePushTarget 更新推流目标
func (s *Server) handleUpdatePushTarget(w http.ResponseWriter, r *http.Request) {
	if s.pushManager == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "Push manager not initialized")
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		s.jsonError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := s.pushManager.UpdateTarget(id, updates); err != nil {
		s.jsonError(w, http.StatusNotFound, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Push target updated successfully",
	})
}

// handleDeletePushTarget 删除推流目标
func (s *Server) handleDeletePushTarget(w http.ResponseWriter, r *http.Request) {
	if s.pushManager == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "Push manager not initialized")
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	if err := s.pushManager.DeleteTarget(id); err != nil {
		s.jsonError(w, http.StatusNotFound, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Push target deleted successfully",
	})
}

// handleStartPush 开始推流
func (s *Server) handleStartPush(w http.ResponseWriter, r *http.Request) {
	if s.pushManager == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "Push manager not initialized")
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	if err := s.pushManager.StartPush(id); err != nil {
		s.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	target, _ := s.pushManager.GetTarget(id)
	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"target":  target,
		"message": "Push started successfully",
	})
}

// handleStopPush 停止推流
func (s *Server) handleStopPush(w http.ResponseWriter, r *http.Request) {
	if s.pushManager == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "Push manager not initialized")
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	if err := s.pushManager.StopPush(id); err != nil {
		s.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	target, _ := s.pushManager.GetTarget(id)
	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"target":  target,
		"message": "Push stopped successfully",
	})
}

// handleGetChannelPushTargets 获取通道的推流任务
func (s *Server) handleGetChannelPushTargets(w http.ResponseWriter, r *http.Request) {
	if s.pushManager == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "Push manager not initialized")
		return
	}

	vars := mux.Vars(r)
	channelID := vars["channelId"]

	targets := s.pushManager.GetTargetsByChannel(channelID)
	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"targets": targets,
	})
}
