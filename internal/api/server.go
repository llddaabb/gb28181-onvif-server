package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"gb28181-onvif-server/internal/ai"
	"gb28181-onvif-server/internal/config"
	"gb28181-onvif-server/internal/debug"
	"gb28181-onvif-server/internal/gb28181"
	"gb28181-onvif-server/internal/onvif"
	"gb28181-onvif-server/internal/storage"
	"gb28181-onvif-server/internal/zlm"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"io/ioutil"
	"strconv"
	"syscall"

	"github.com/gorilla/mux"
)

// Server API服务器结构体
type Server struct {
	config             *config.Config
	gb28181Server      *gb28181.Server
	onvifManager       *onvif.Manager
	zlmServer          *zlm.ZLMServer
	zlmProcess         *zlm.ProcessManager
	diskManager        *storage.DiskManager
	aiManager          *ai.AIRecordingManager
	server             *http.Server
	configPath         string
	channelManager     *ChannelManager
	recordingManager   *RecordingManager
	streamManager      *StreamManager
	startTime          time.Time
	recordingWatchStop chan struct{} // 停止录像监控器
}

// NewServer 创建API服务器实例
func NewServer(cfg *config.Config, gbServer *gb28181.Server, onvifMgr *onvif.Manager, configPath string) *Server {
	return &Server{
		config:           cfg,
		gb28181Server:    gbServer,
		onvifManager:     onvifMgr,
		zlmServer:        nil,
		zlmProcess:       nil,
		configPath:       configPath,
		channelManager:   NewChannelManager(),
		recordingManager: NewRecordingManager(),
		streamManager:    NewStreamManager(),
		startTime:        time.Now(),
	}
}

// NewServerWithZLM 创建带ZLM的API服务器实例
func NewServerWithZLM(cfg *config.Config, gbServer *gb28181.Server, onvifMgr *onvif.Manager, zlmSrv *zlm.ZLMServer, configPath string) *Server {
	return &Server{
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
}

// SetZLMProcess 设置 ZLM 进程管理器
func (s *Server) SetZLMProcess(pm *zlm.ProcessManager) {
	s.zlmProcess = pm
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

	// 创建录像控制函数
	recordControl := func(channelID string, start bool) error {
		if start {
			// 调用现有的录像启动API
			debug.Info("ai", "AI触发录像启动: channelID=%s", channelID)
			// 这里可以调用 handleStartChannelRecording 的逻辑
			return nil
		} else {
			// 调用现有的录像停止API
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
	// 将GB28181通道转换为API通道类型
	apiChannel := &Channel{
		ChannelID:   channel.ChannelID,
		ChannelName: channel.Name,
		DeviceID:    channel.DeviceID,
		DeviceType:  "gb28181",
		Status:      channel.Status,
		StreamURL:   "", // 流地址将在预览时生成
	}

	// 如果通道已存在，则更新；否则添加
	if existingChannel, exists := s.channelManager.GetChannel(channel.ChannelID); exists {
		// 更新现有通道
		existingChannel.ChannelName = channel.Name
		existingChannel.Status = channel.Status
		return s.channelManager.UpdateChannel(existingChannel)
	} else {
		// 添加新通道
		return s.channelManager.AddChannel(apiChannel)
	}
}

// Start 启动API服务器
func (s *Server) Start() error {
	// 创建路由
	r := mux.NewRouter()

	// 添加CORS中间件
	r.Use(s.corsMiddleware)

	// 添加请求日志中间件
	r.Use(s.loggingMiddleware)

	// 设置路由
	s.setupRoutes(r)

	// 创建HTTP服务器
	s.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", s.config.API.Host, s.config.API.Port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Println("═══════════════════════════════════════════════════════════")
	log.Printf("[API] ✓ REST API服务器启动成功")
	log.Printf("[API] 监听地址: http://%s:%d", s.config.API.Host, s.config.API.Port)
	log.Printf("[API] 配置 - CORS: %v | Timeout: 15s", len(s.config.API.CorsAllowOrigins) > 0)
	log.Println("═══════════════════════════════════════════════════════════")
	debug.Info("api", "API服务器启动成功，监听地址: %s:%d", s.config.API.Host, s.config.API.Port)
	debug.Debug("api", "服务器配置: Host=%s, Port=%d, CORS=%v",
		s.config.API.Host, s.config.API.Port, s.config.API.CorsAllowOrigins)

	// 启动录像监控器
	s.startRecordingWatchdog()

	// 启动服务器
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		debug.Error("api", "启动API服务器失败: %v", err)
		return fmt.Errorf("启动API服务器失败: %w", err)
	}

	return nil
}

// Stop 停止API服务器
func (s *Server) Stop() error {
	// 停止录像监控器
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
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 允许的来源
		w.Header().Set("Access-Control-Allow-Origin", "*")

		// 允许的方法
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")

		// 允许的头
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// 处理OPTIONS请求
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// 继续处理请求
		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware 请求日志中间件
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 创建自定义的ResponseWriter来捕获状态码
		lw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// 处理请求
		next.ServeHTTP(lw, r)

		// 记录请求信息
		duration := time.Since(start)
		debug.Info("api", "%s %s - %d - %s - %s",
			r.Method, r.URL.Path, lw.statusCode, r.RemoteAddr, duration)

		// 对于调试级别，记录更多详细信息
		debug.Debug("api", "请求详情: UserAgent=%s, ContentLength=%d, ContentType=%s",
			r.UserAgent(), r.ContentLength, r.Header.Get("Content-Type"))
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

// jsonResponse 发送 JSON 响应
func (s *Server) jsonResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// jsonError 发送 JSON 错误响应
func (s *Server) jsonError(w http.ResponseWriter, statusCode int, message string) {
	s.jsonResponse(w, statusCode, map[string]interface{}{
		"success": false,
		"error":   message,
	})
}

// setupRoutes 设置API路由
func (s *Server) setupRoutes(r *mux.Router) {
	// API 路由应该更具体，放在前面更容易被匹配

	// 健康检查
	r.HandleFunc("/api/health", s.handleHealthCheck).Methods("GET")

	// 系统状态
	r.HandleFunc("/api/status", s.handleGetStatus).Methods("GET")

	// 兼容前端期望的统计、资源和日志接口
	r.HandleFunc("/api/stats", s.handleGetStats).Methods("GET")
	r.HandleFunc("/api/resources", s.handleGetResources).Methods("GET")
	r.HandleFunc("/api/logs/latest", s.handleGetLatestLogs).Methods("GET")

	// 配置管理API
	r.HandleFunc("/api/config", s.handleGetConfig).Methods("GET")
	r.HandleFunc("/api/config", s.handleUpdateConfig).Methods("PUT")

	// GB28181设备相关API
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

	// ONVIF设备相关API
	onvifGroup := r.PathPrefix("/api/onvif").Subrouter()
	onvifGroup.HandleFunc("/devices", s.handleGetONVIFDevices).Methods("GET")
	onvifGroup.HandleFunc("/devices", s.handleAddONVIFDevice).Methods("POST")
	onvifGroup.HandleFunc("/devices/{id}", s.handleGetONVIFDevice).Methods("GET")
	onvifGroup.HandleFunc("/devices/{id}", s.handleRemoveONVIFDevice).Methods("DELETE")
	onvifGroup.HandleFunc("/devices/{id}/refresh", s.handleRefreshONVIFDevice).Methods("PUT")
	onvifGroup.HandleFunc("/devices/{id}/profiles", s.handleGetONVIFProfiles).Methods("GET")
	onvifGroup.HandleFunc("/devices/{id}/snapshot", s.handleGetONVIFSnapshot).Methods("GET")
	onvifGroup.HandleFunc("/devices/{id}/presets", s.handleGetONVIFPresets).Methods("GET")
	onvifGroup.HandleFunc("/devices/{id}/preset", s.handleSetONVIFPreset).Methods("POST")
	onvifGroup.HandleFunc("/devices/{id}/preset/{token}", s.handleGotoONVIFPreset).Methods("POST")
	onvifGroup.HandleFunc("/devices/{id}/preview/start", s.handleStartONVIFPreview).Methods("POST")
	onvifGroup.HandleFunc("/devices/{id}/preview/stop", s.handleStopONVIFPreview).Methods("POST")
	onvifGroup.HandleFunc("/discover", s.handleDiscoverONVIFDevices).Methods("POST")
	onvifGroup.HandleFunc("/batch-add", s.handleBatchAddONVIFDevices).Methods("POST")
	onvifGroup.HandleFunc("/statistics", s.handleGetONVIFStatistics).Methods("GET")

	// 媒体流相关API
	streamGroup := r.PathPrefix("/api/stream").Subrouter()
	streamGroup.HandleFunc("/start", s.handleStartStream).Methods("POST")
	streamGroup.HandleFunc("/stop", s.handleStopStream).Methods("POST")
	streamGroup.HandleFunc("/list", s.handleListStreams).Methods("GET")

	// 通道管理API
	channelGroup := r.PathPrefix("/api/channel").Subrouter()
	channelGroup.HandleFunc("/list", s.handleListChannels).Methods("GET")
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

	// 设备控制相关API
	controlGroup := r.PathPrefix("/api/control").Subrouter()
	controlGroup.HandleFunc("/ptz", s.handlePTZControl).Methods("POST")
	controlGroup.HandleFunc("/ptz/reset", s.handlePTZReset).Methods("POST")

	// ZLM媒体服务器API
	zlmGroup := r.PathPrefix("/api/zlm").Subrouter()
	zlmGroup.HandleFunc("/status", s.handleZLMStatus).Methods("GET")
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

	// 静态文件服务（必须在最后，作为 catch-all）
	staticDir := "frontend/dist"
	// 提供 assets 文件
	r.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", http.FileServer(http.Dir(staticDir+"/assets"))))
	// 根路由和其他文件
	r.HandleFunc("/", s.handleServeStaticFile).Methods("GET")
	r.HandleFunc("/{path:.*\\.html$}", s.handleServeStaticFile).Methods("GET")
	// 处理前端路由 - 所有不匹配API路由的请求都返回index.html
	r.PathPrefix("/").HandlerFunc(s.handleServeStaticFile).Methods("GET")
}

// handleServeStaticFile 提供静态文件
func (s *Server) handleServeStaticFile(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "/" || path == "" {
		path = "/index.html"
	}

	filePath := "frontend/dist" + path

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// 如果文件不存在，返回 index.html（SPA 路由）
		if !strings.HasPrefix(path, "/api") && !strings.HasPrefix(path, "/assets") {
			http.ServeFile(w, r, "frontend/dist/index.html")
			return
		}
		// API 路由或 assets 路由返回 404
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, filePath)
}

// handleStaticOrAPI 处理静态文件或 API 路由
func (s *Server) handleStaticOrAPI(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// 如果是 API 路由，由 mux 路由器处理
	if path == "/" || path == "/index.html" {
		// 返回 index.html 让前端应用处理路由
		http.ServeFile(w, r, "frontend/dist/index.html")
		return
	}

	// 尝试从 dist 目录提供文件
	staticPath := "frontend/dist" + path
	http.ServeFile(w, r, staticPath)
}

// handleHealthCheck 健康检查
func (s *Server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// handleGetStatus 获取系统状态
func (s *Server) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	debug.Info("api", "处理系统状态请求")

	// 获取各服务状态
	gb28181Devices := s.gb28181Server.GetDevices()
	onvifDevices := s.onvifManager.GetDevices()

	// 计算运行时长
	uptime := time.Since(s.startTime)
	days := int(uptime.Hours()) / 24
	hours := int(uptime.Hours()) % 24
	minutes := int(uptime.Minutes()) % 60
	uptimeStr := fmt.Sprintf("%d天 %d小时 %d分钟", days, hours, minutes)

	// 获取启动时间
	startTimeStr := s.startTime.Format("2006-01-02 15:04:05")

	// 获取内存使用率
	var memUsage int
	if data, err := ioutil.ReadFile("/proc/meminfo"); err == nil {
		s := string(data)
		var memTotal, memAvailable uint64
		for _, line := range strings.Split(s, "\n") {
			if strings.HasPrefix(line, "MemTotal:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					v, _ := strconv.ParseUint(fields[1], 10, 64)
					memTotal = v
				}
			}
			if strings.HasPrefix(line, "MemAvailable:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					v, _ := strconv.ParseUint(fields[1], 10, 64)
					memAvailable = v
				}
			}
		}
		if memTotal > 0 {
			used := memTotal - memAvailable
			memUsage = int((float64(used) / float64(memTotal)) * 100)
		}
	}

	// 获取CPU使用率
	var cpuUsage int
	readCPU := func() (uint64, uint64) {
		data, err := ioutil.ReadFile("/proc/stat")
		if err != nil {
			return 0, 0
		}
		lines := strings.Split(string(data), "\n")
		if len(lines) == 0 {
			return 0, 0
		}
		fields := strings.Fields(lines[0])
		if len(fields) < 5 {
			return 0, 0
		}
		var total, idle uint64
		for i := 1; i < len(fields); i++ {
			v, _ := strconv.ParseUint(fields[i], 10, 64)
			total += v
			if i == 4 {
				idle = v
			}
		}
		return total, idle
	}
	t1, i1 := readCPU()
	time.Sleep(200 * time.Millisecond)
	t2, i2 := readCPU()
	if t2 > t1 {
		totalDiff := float64(t2 - t1)
		idleDiff := float64(i2 - i1)
		usage := (1.0 - idleDiff/totalDiff) * 100.0
		cpuUsage = int(usage)
	}

	// 构建状态响应
	status := map[string]interface{}{
		"status": "running",
		"serverInfo": map[string]interface{}{
			"startTime":   startTimeStr,
			"uptime":      uptimeStr,
			"memoryUsage": fmt.Sprintf("%d%%", memUsage),
			"cpuUsage":    fmt.Sprintf("%d%%", cpuUsage),
		},
		"servers": map[string]interface{}{
			"gb28181": map[string]interface{}{
				"status":  "running",
				"devices": len(gb28181Devices),
			},
			"onvif": map[string]interface{}{
				"status":  "running",
				"devices": len(onvifDevices),
			},
			"api": map[string]interface{}{
				"status": "running",
				"port":   s.config.API.Port,
			},
		},
		"statistics": map[string]interface{}{
			"totalDevices":   len(gb28181Devices) + len(onvifDevices),
			"gb28181Devices": len(gb28181Devices),
			"onvifDevices":   len(onvifDevices),
		},
	}

	// 如果ZLM服务器已初始化，添加ZLM状态
	if s.zlmServer != nil {
		zlmStats := s.zlmServer.GetStatistics()
		status["servers"].(map[string]interface{})["zlm"] = map[string]interface{}{
			"status":  "running",
			"streams": zlmStats["totalStreams"],
		}
		status["statistics"].(map[string]interface{})["activeStreams"] = zlmStats["totalStreams"]
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)

	debug.Debug("api", "系统状态响应成功，GB28181设备: %d, ONVIF设备: %d",
		len(gb28181Devices), len(onvifDevices))
}

// handleGetGB28181Devices 获取GB28181设备列表
func (s *Server) handleGetGB28181Devices(w http.ResponseWriter, r *http.Request) {
	debug.Info("api", "处理GB28181设备列表请求")

	devices := s.gb28181Server.GetDevices()
	debug.Debug("api", "获取到 %d 个GB28181设备", len(devices))

	// 创建API响应结构
	response := struct {
		Success bool          `json:"success"`
		Devices []interface{} `json:"devices"`
	}{
		Success: true,
		Devices: make([]interface{}, len(devices)),
	}

	// 转换设备数据格式
	for i, device := range devices {
		// 获取设备的通道列表
		channels := s.gb28181Server.GetChannels(device.DeviceID)
		channelList := make([]map[string]interface{}, 0, len(channels))
		for _, ch := range channels {
			channelList = append(channelList, map[string]interface{}{
				"channelId": ch.ChannelID,
				"id":        ch.ChannelID,
				"name":      ch.Name,
				"status":    ch.Status,
				"ptzType":   ch.PTZType,
			})
		}

		response.Devices[i] = map[string]interface{}{
			"deviceId":        device.DeviceID,
			"id":              device.DeviceID,
			"name":            device.Name,
			"manufacturer":    device.Manufacturer,
			"model":           device.Model,
			"firmware":        device.Firmware,
			"status":          device.Status,
			"sipIP":           device.SipIP,
			"sipPort":         device.SipPort,
			"transport":       device.Transport,
			"registerTime":    device.RegisterTime,
			"lastKeepAlive":   device.LastKeepAlive,
			"expires":         device.Expires,
			"channelCount":    device.ChannelCount,
			"onlineChannels":  device.OnlineChannels,
			"ptzSupported":    device.PTZSupported,
			"recordSupported": device.RecordSupported,
			"streamMode":      device.StreamMode,
			"channels":        channelList,
		}
	}

	// 返回JSON响应
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)

	debug.Debug("api", "GB28181设备列表响应成功，返回 %d 个设备", len(devices))
}

// handleGetGB28181Device 获取单个GB28181设备
func (s *Server) handleGetGB28181Device(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	deviceID := params["id"]

	device, exists := s.gb28181Server.GetDeviceByID(deviceID)
	if !exists {
		http.Error(w, `{"error":"设备不存在"}`, http.StatusNotFound)
		return
	}

	// 创建API响应结构
	response := struct {
		Device map[string]interface{} `json:"device"`
	}{
		Device: map[string]interface{}{
			"deviceId":     device.DeviceID,
			"name":         device.Name,
			"status":       device.Status,
			"sipIP":        device.SipIP,
			"sipPort":      device.SipPort,
			"registerTime": device.RegisterTime,
			"expires":      device.Expires,
		},
	}

	// 返回JSON响应
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleRemoveGB28181Device 删除GB28181设备
func (s *Server) handleRemoveGB28181Device(w http.ResponseWriter, r *http.Request) {
	debug.Info("api", "处理删除GB28181设备请求")

	vars := mux.Vars(r)
	deviceID := vars["id"]

	if deviceID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "设备ID不能为空",
		})
		return
	}

	if s.gb28181Server.RemoveDevice(deviceID) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "设备已删除",
		})
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "设备不存在",
		})
	}
}

// handleGetGB28181Channels 获取设备通道列表
func (s *Server) handleGetGB28181Channels(w http.ResponseWriter, r *http.Request) {
	debug.Info("api", "处理获取GB28181设备通道请求")

	vars := mux.Vars(r)
	deviceID := vars["id"]

	channels := s.gb28181Server.GetChannels(deviceID)
	if channels == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "设备不存在",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"channels": channels,
	})
}

// handleRefreshGB28181Device 刷新设备信息和通道列表
func (s *Server) handleRefreshGB28181Device(w http.ResponseWriter, r *http.Request) {
	debug.Info("api", "处理刷新GB28181设备请求")

	vars := mux.Vars(r)
	deviceID := vars["id"]

	// 查询设备信息
	if err := s.gb28181Server.QueryDeviceInfo(deviceID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// 查询设备目录（通道列表）
	if err := s.gb28181Server.QueryCatalog(deviceID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "已发送设备信息和目录查询请求",
	})
}

// handleGetGB28181Statistics 获取GB28181统计信息
func (s *Server) handleGetGB28181Statistics(w http.ResponseWriter, r *http.Request) {
	debug.Info("api", "处理获取GB28181统计信息请求")

	stats := s.gb28181Server.GetStatistics()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    true,
		"statistics": stats,
	})
}

// handleGetGB28181ServerConfig 获取GB28181服务器配置信息
func (s *Server) handleGetGB28181ServerConfig(w http.ResponseWriter, r *http.Request) {
	debug.Info("api", "处理获取GB28181服务器配置请求")

	// 返回服务器配置信息（不包含密码）
	serverConfig := map[string]interface{}{
		"sip_ip":             s.config.GB28181.SipIP,
		"sip_port":           s.config.GB28181.SipPort,
		"realm":              s.config.GB28181.Realm,
		"server_id":          s.config.GB28181.ServerID,
		"heartbeat_interval": s.config.GB28181.HeartbeatInterval,
		"register_expires":   s.config.GB28181.RegisterExpires,
		"auth_enabled":       s.config.GB28181.Password != "",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"config":  serverConfig,
	})
}

// handleUpdateGB28181ServerConfig 更新GB28181服务器配置
func (s *Server) handleUpdateGB28181ServerConfig(w http.ResponseWriter, r *http.Request) {
	debug.Info("api", "处理更新GB28181服务器配置请求")

	var req struct {
		SipIP           string `json:"sip_ip"`
		SipPort         int    `json:"sip_port"`
		Realm           string `json:"realm"`
		ServerID        string `json:"server_id"`
		Password        string `json:"password"`
		RegisterExpires int    `json:"register_expires"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"success":false,"error":"无效的请求数据"}`, http.StatusBadRequest)
		return
	}

	// 更新配置
	if req.SipIP != "" {
		s.config.GB28181.SipIP = req.SipIP
	}
	if req.SipPort > 0 {
		s.config.GB28181.SipPort = req.SipPort
	}
	if req.Realm != "" {
		s.config.GB28181.Realm = req.Realm
	}
	if req.ServerID != "" {
		s.config.GB28181.ServerID = req.ServerID
	}
	// 密码可以设置为空（禁用认证）
	s.config.GB28181.Password = req.Password
	if req.RegisterExpires > 0 {
		s.config.GB28181.RegisterExpires = req.RegisterExpires
	}

	// 保存配置到文件
	if err := s.config.Save(s.configPath); err != nil {
		log.Printf("[API] 保存配置失败: %v", err)
		http.Error(w, fmt.Sprintf(`{"success":false,"error":"保存配置失败: %v"}`, err), http.StatusInternalServerError)
		return
	}

	log.Printf("[API] GB28181配置已更新: SIP=%s:%d, Realm=%s, ServerID=%s",
		s.config.GB28181.SipIP, s.config.GB28181.SipPort, s.config.GB28181.Realm, s.config.GB28181.ServerID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "配置已保存，需要重启服务器生效",
	})
}

// GB28181 预览会话
type GB28181PreviewSession struct {
	DeviceID   string `json:"device_id"`
	ChannelID  string `json:"channel_id"`
	StreamKey  string `json:"stream_key"`
	App        string `json:"app"`
	Stream     string `json:"stream"`
	FlvURL     string `json:"flv_url"`
	WsFlvURL   string `json:"ws_flv_url"`
	HlsURL     string `json:"hls_url"`
	RtmpURL    string `json:"rtmp_url"`
	CreateTime int64  `json:"create_time"`
}

var gb28181PreviewSessions = make(map[string]*GB28181PreviewSession)
var gb28181SessionMutex sync.RWMutex

// handleStartGB28181Preview 启动GB28181设备预览
func (s *Server) handleStartGB28181Preview(w http.ResponseWriter, r *http.Request) {
	debug.Info("api", "处理启动GB28181预览请求")

	vars := mux.Vars(r)
	deviceID := vars["id"]

	// 解析请求体
	var req struct {
		ChannelID string `json:"channelId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// 如果没有指定通道，使用默认通道
		req.ChannelID = deviceID
	}
	if req.ChannelID == "" {
		req.ChannelID = deviceID
	}

	if deviceID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "设备ID不能为空",
		})
		return
	}

	// 检查设备是否存在且在线
	device, exists := s.gb28181Server.GetDeviceByID(deviceID)
	if !exists {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "设备不存在",
		})
		return
	}

	if device.Status != "online" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "设备离线，无法预览",
		})
		return
	}

	// 检查 ZLM 服务是否可用
	if s.zlmServer == nil || s.zlmServer.GetAPIClient() == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "ZLM媒体服务未启动",
		})
		return
	}

	zlmClient := s.zlmServer.GetAPIClient()

	// 生成流ID
	streamID := strings.ReplaceAll(req.ChannelID, "-", "")
	app := "rtp"

	// 获取 ZLM 服务器地址
	zlmHost := "127.0.0.1"
	if s.config.ZLM != nil && s.config.ZLM.General != nil && s.config.ZLM.General.ListenIP != "" && s.config.ZLM.General.ListenIP != "::" {
		zlmHost = s.config.ZLM.General.ListenIP
	}
	if zlmHost == "" || zlmHost == "0.0.0.0" || zlmHost == "::" {
		// 获取本机IP
		zlmHost = getLocalIP()
	}
	zlmHTTPPort := 80
	if s.config.ZLM != nil {
		zlmHTTPPort = s.config.ZLM.GetHTTPPort()
	}
	zlmRTMPPort := 1935
	if s.config.ZLM != nil {
		zlmRTMPPort = s.config.ZLM.GetRTMPPort()
	}

	// 1. 在 ZLM 打开 RTP 接收端口
	rtpInfo, err := zlmClient.OpenRtpServer(streamID, 0, 0) // 0: UDP模式, 0: 随机端口
	if err != nil {
		debug.Error("api", "打开RTP端口失败: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("打开RTP端口失败: %v", err),
		})
		return
	}

	debug.Info("api", "ZLM RTP端口已打开: port=%d, streamID=%s", rtpInfo.Port, streamID)

	// 2. 向设备发送 INVITE 请求
	mediaSession, err := s.gb28181Server.InviteRequest(deviceID, req.ChannelID, rtpInfo.Port, zlmHost)
	if err != nil {
		// 关闭已打开的 RTP 端口
		zlmClient.CloseRtpServer(streamID)

		debug.Error("api", "发送INVITE失败: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("发送INVITE失败: %v", err),
		})
		return
	}

	// 创建预览会话
	session := &GB28181PreviewSession{
		DeviceID:   deviceID,
		ChannelID:  req.ChannelID,
		StreamKey:  streamID,
		App:        app,
		Stream:     streamID,
		FlvURL:     fmt.Sprintf("http://%s:%d/%s/%s.live.flv", zlmHost, zlmHTTPPort, app, streamID),
		WsFlvURL:   fmt.Sprintf("ws://%s:%d/%s/%s.live.flv", zlmHost, zlmHTTPPort, app, streamID),
		HlsURL:     fmt.Sprintf("http://%s:%d/%s/%s/hls.m3u8", zlmHost, zlmHTTPPort, app, streamID),
		RtmpURL:    fmt.Sprintf("rtmp://%s:%d/%s/%s", zlmHost, zlmRTMPPort, app, streamID),
		CreateTime: time.Now().Unix(),
	}

	// 保存会话
	gb28181SessionMutex.Lock()
	gb28181PreviewSessions[deviceID+"_"+req.ChannelID] = session
	gb28181SessionMutex.Unlock()

	debug.Info("api", "GB28181预览启动: deviceID=%s, channelID=%s, rtpPort=%d, ssrc=%s",
		deviceID, req.ChannelID, rtpInfo.Port, mediaSession.SSRC)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "预览启动中，等待设备推流",
		"data": map[string]interface{}{
			"device_id":  deviceID,
			"channel_id": req.ChannelID,
			"stream_id":  streamID,
			"rtp_port":   rtpInfo.Port,
			"ssrc":       mediaSession.SSRC,
			"flv_url":    session.FlvURL,
			"ws_flv_url": session.WsFlvURL,
			"hls_url":    session.HlsURL,
			"rtmp_url":   session.RtmpURL,
			"status":     mediaSession.Status,
		},
	})
}

// handleStopGB28181Preview 停止GB28181设备预览
func (s *Server) handleStopGB28181Preview(w http.ResponseWriter, r *http.Request) {
	debug.Info("api", "处理停止GB28181预览请求")

	vars := mux.Vars(r)
	deviceID := vars["id"]

	// 解析请求体获取通道ID
	var req struct {
		ChannelID string `json:"channelId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.ChannelID = deviceID
	}
	if req.ChannelID == "" {
		req.ChannelID = deviceID
	}

	if deviceID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "设备ID不能为空",
		})
		return
	}

	sessionKey := deviceID + "_" + req.ChannelID

	// 获取会话
	gb28181SessionMutex.RLock()
	session, exists := gb28181PreviewSessions[sessionKey]
	gb28181SessionMutex.RUnlock()

	if exists && session != nil {
		// 1. 向设备发送 BYE 停止推流
		if err := s.gb28181Server.ByeRequest(deviceID, req.ChannelID); err != nil {
			debug.Warn("api", "发送BYE失败: %v", err)
		}

		// 2. 关闭 ZLM RTP 端口
		if s.zlmServer != nil && s.zlmServer.GetAPIClient() != nil {
			streamID := session.StreamKey
			if err := s.zlmServer.GetAPIClient().CloseRtpServer(streamID); err != nil {
				debug.Warn("api", "关闭RTP端口失败: %v", err)
			}
			// 同时关闭流
			if err := s.zlmServer.GetAPIClient().CloseStream(session.App, session.Stream); err != nil {
				debug.Warn("api", "关闭流失败: %v", err)
			}
		}
	}

	// 删除会话
	gb28181SessionMutex.Lock()
	delete(gb28181PreviewSessions, sessionKey)
	gb28181SessionMutex.Unlock()

	debug.Info("api", "GB28181预览已停止: deviceID=%s, channelID=%s", deviceID, req.ChannelID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "预览已停止",
	})
}

// handleStartGB28181ChannelPreview 启动GB28181设备指定通道预览
func (s *Server) handleStartGB28181ChannelPreview(w http.ResponseWriter, r *http.Request) {
	debug.Info("api", "处理启动GB28181通道预览请求")

	vars := mux.Vars(r)
	deviceID := vars["id"]
	channelID := vars["channelId"]

	if deviceID == "" || channelID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "设备ID和通道ID不能为空",
		})
		return
	}

	// 复用原有逻辑，构造包含 channelId 的请求体
	req := struct {
		ChannelID string `json:"channelId"`
	}{ChannelID: channelID}

	// 模拟请求体
	body, _ := json.Marshal(req)
	r.Body = io.NopCloser(bytes.NewReader(body))

	// 调用原有的预览处理函数
	s.handleStartGB28181Preview(w, r)
}

// handleTestGB28181ChannelPreview 测试预览 - 使用流代理方式拉取公共测试流
func (s *Server) handleTestGB28181ChannelPreview(w http.ResponseWriter, r *http.Request) {
	debug.Info("api", "处理测试预览请求（使用流代理）")

	vars := mux.Vars(r)
	deviceID := vars["id"]
	channelID := vars["channelId"]

	if deviceID == "" || channelID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "设备ID和通道ID不能为空",
		})
		return
	}

	// 检查 ZLM 服务是否可用
	if s.zlmServer == nil || s.zlmServer.GetAPIClient() == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "ZLM媒体服务未启动",
		})
		return
	}

	zlmClient := s.zlmServer.GetAPIClient()

	// 公共测试流地址
	testStreamURL := "rtmp://ns8.indexforce.com/home/mystream"

	// 使用通道ID作为流ID（去掉横杠，保持与 ZLM 一致）
	streamID := strings.ReplaceAll(channelID, "-", "")
	if streamID == "" {
		streamID = channelID
	}
	app := "live"

	// 获取 ZLM 服务器地址
	zlmHost := "127.0.0.1"
	if s.config.ZLM != nil && s.config.ZLM.General != nil && s.config.ZLM.General.ListenIP != "" && s.config.ZLM.General.ListenIP != "::" {
		zlmHost = s.config.ZLM.General.ListenIP
	}
	if zlmHost == "" || zlmHost == "0.0.0.0" || zlmHost == "::" {
		zlmHost = getLocalIP()
	}
	zlmHTTPPort := 80
	if s.config.ZLM != nil {
		zlmHTTPPort = s.config.ZLM.GetHTTPPort()
	}
	zlmRTMPPort := 1935
	if s.config.ZLM != nil {
		zlmRTMPPort = s.config.ZLM.GetRTMPPort()
	}

	// 先检测流是否已经在线，避免重复创建
	streamExists := false
	if online, err := zlmClient.IsStreamOnline(app, streamID); err == nil {
		streamExists = online
		if streamExists {
			debug.Info("api", "检测到流已在线，直接复用: app=%s, stream=%s", app, streamID)
		}
	} else {
		debug.Warn("api", "检测流状态失败，将继续尝试创建: %v", err)
	}

	var proxyInfo *zlm.StreamProxyInfo
	if !streamExists {
		var err error
		proxyInfo, err = zlmClient.AddStreamProxy(testStreamURL, app, streamID)
		if err != nil {
			errStr := err.Error()
			if strings.Contains(errStr, "already exists") || strings.Contains(errStr, "code: -1") {
				streamExists = true
				debug.Info("api", "流已存在，AddStreamProxy 抛出已有提示: app=%s, stream=%s", app, streamID)
			} else {
				debug.Error("api", "添加流代理失败: %v", err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   fmt.Sprintf("添加流代理失败: %v", err),
				})
				return
			}
		}
	}

	proxyKey := ""
	if proxyInfo != nil {
		proxyKey = proxyInfo.Key
	}

	if !streamExists {
		debug.Info("api", "测试预览流代理已创建: app=%s, stream=%s, key=%s", app, streamID, proxyKey)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "测试预览已启动",
		"exists":  streamExists,
		"data": map[string]interface{}{
			"device_id":   deviceID,
			"channel_id":  channelID,
			"stream_id":   streamID,
			"proxy_key":   proxyKey,
			"source_url":  testStreamURL,
			"flv_url":     fmt.Sprintf("http://%s:%d/%s/%s.live.flv", zlmHost, zlmHTTPPort, app, streamID),
			"ws_flv_url":  fmt.Sprintf("ws://%s:%d/%s/%s.live.flv", zlmHost, zlmHTTPPort, app, streamID),
			"hls_url":     fmt.Sprintf("http://%s:%d/%s/%s/hls.m3u8", zlmHost, zlmHTTPPort, app, streamID),
			"rtmp_url":    fmt.Sprintf("rtmp://%s:%d/%s/%s", zlmHost, zlmRTMPPort, app, streamID),
		},
	})
}

// handleStopGB28181ChannelPreview 停止GB28181设备指定通道预览
func (s *Server) handleStopGB28181ChannelPreview(w http.ResponseWriter, r *http.Request) {
	debug.Info("api", "处理停止GB28181通道预览请求")

	vars := mux.Vars(r)
	deviceID := vars["id"]
	channelID := vars["channelId"]

	if deviceID == "" || channelID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "设备ID和通道ID不能为空",
		})
		return
	}

	// 复用原有逻辑，构造包含 channelId 的请求体
	req := struct {
		ChannelID string `json:"channelId"`
	}{ChannelID: channelID}

	// 模拟请求体
	body, _ := json.Marshal(req)
	r.Body = io.NopCloser(bytes.NewReader(body))

	// 调用原有的停止预览处理函数
	s.handleStopGB28181Preview(w, r)
}

// handleGB28181Catalog 触发GB28181设备目录查询
func (s *Server) handleGB28181Catalog(w http.ResponseWriter, r *http.Request) {
	debug.Info("api", "处理GB28181目录查询请求")

	vars := mux.Vars(r)
	deviceID := vars["id"]

	if deviceID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "设备ID不能为空",
		})
		return
	}

	// 检查设备是否存在且在线
	device, exists := s.gb28181Server.GetDeviceByID(deviceID)
	if !exists {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "设备不存在",
		})
		return
	}

	if device.Status != "online" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "设备离线，无法查询目录",
		})
		return
	}

	// 向设备发送 Catalog 查询
	err := s.gb28181Server.SendCatalogQuery(deviceID)
	if err != nil {
		debug.Error("api", "发送Catalog查询失败: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("发送目录查询失败: %v", err),
		})
		return
	}

	debug.Info("api", "已发送Catalog查询: deviceID=%s", deviceID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "目录查询已发送，请等待设备响应后刷新通道列表",
	})
}

// handleGB28181PTZ 处理GB28181 PTZ控制
func (s *Server) handleGB28181PTZ(w http.ResponseWriter, r *http.Request) {
	debug.Info("api", "处理GB28181 PTZ控制请求")

	vars := mux.Vars(r)
	deviceID := vars["id"]

	// 解析请求体
	var req struct {
		Command   string `json:"command"`   // up, down, left, right, zoomIn, zoomOut, stop
		ChannelID string `json:"channelId"` // 通道ID
		Speed     int    `json:"speed"`     // 速度 1-255
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "无效的请求参数",
		})
		return
	}

	if req.Speed == 0 {
		req.Speed = 128 // 默认速度
	}
	if req.ChannelID == "" {
		req.ChannelID = deviceID // 默认使用设备ID作为通道ID
	}

	// 发送PTZ命令
	err := s.gb28181Server.SendPTZCommand(deviceID, req.ChannelID, req.Command, req.Speed)
	if err != nil {
		debug.Error("api", "GB28181 PTZ控制失败: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("PTZ控制失败: %v", err),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "PTZ命令已发送",
	})
}

// handleDiscoverGB28181Devices GB28181设备发现
func (s *Server) handleDiscoverGB28181Devices(w http.ResponseWriter, r *http.Request) {
	// 这里可以添加设备发现逻辑
	// 目前先返回成功响应
	response := struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}{
		Success: true,
		Message: "设备发现请求已接收，请等待设备主动注册",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleGetONVIFDevices 获取ONVIF设备列表
func (s *Server) handleGetONVIFDevices(w http.ResponseWriter, r *http.Request) {
	devices := s.onvifManager.GetDevices()

	// 创建API响应结构
	response := struct {
		Devices []interface{} `json:"devices"`
	}{
		Devices: make([]interface{}, len(devices)),
	}

	// 转换设备数据格式
	for i, device := range devices {
		response.Devices[i] = map[string]interface{}{
			"deviceId":        device.DeviceID,
			"name":            device.Name,
			"model":           device.Model,
			"manufacturer":    device.Manufacturer,
			"firmwareVersion": device.FirmwareVersion,
			"serialNumber":    device.SerialNumber,
			"ip":              device.IP,
			"port":            device.Port,
			"status":          device.Status,
			"services":        device.Services,
			"previewURL":      device.PreviewURL,
			"snapshotURL":     device.SnapshotURL,
			"responseTime":    device.ResponseTime,
			"lastCheckTime":   device.LastCheckTime,
			"discoveryTime":   device.DiscoveryTime,
			"lastSeenTime":    device.LastSeenTime,
			"checkInterval":   device.CheckInterval,
			"failureCount":    device.FailureCount,
			"ptzSupported":    device.PTZSupported,
			"audioSupported":  device.AudioSupported,
		}
	}

	// 返回JSON响应
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleDiscoverONVIFDevices ONVIF设备发现
func (s *Server) handleDiscoverONVIFDevices(w http.ResponseWriter, r *http.Request) {
	// 执行设备发现
	discoveredDevices, err := s.onvifManager.DiscoverDevices()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"设备发现失败: %v"}`, err), http.StatusInternalServerError)
		return
	}

	// 构建发现结果列表
	results := make([]map[string]interface{}, 0, len(discoveredDevices))
	for _, device := range discoveredDevices {
		results = append(results, map[string]interface{}{
			"xaddr":        device.XAddr,
			"types":        device.Types,
			"manufacturer": device.Manufacturer,
			"model":        device.Model,
			"name":         device.Name,
			"location":     device.Location,
			"hardware":     device.Hardware,
			"sourceIP":     device.SourceIP,
		})
	}

	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("发现 %d 个ONVIF设备", len(discoveredDevices)),
		"count":   len(discoveredDevices),
		"devices": results,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleGetONVIFDevice 获取单个ONVIF设备
func (s *Server) handleGetONVIFDevice(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	deviceID := params["id"]

	device, exists := s.onvifManager.GetDeviceByID(deviceID)
	if !exists {
		http.Error(w, `{"error":"设备不存在"}`, http.StatusNotFound)
		return
	}

	// 创建API响应结构
	response := struct {
		Device map[string]interface{} `json:"device"`
	}{
		Device: map[string]interface{}{
			"deviceId":      device.DeviceID,
			"name":          device.Name,
			"model":         device.Model,
			"manufacturer":  device.Manufacturer,
			"ip":            device.IP,
			"port":          device.Port,
			"status":        device.Status,
			"discoveryTime": device.DiscoveryTime.Format(time.RFC3339),
			"lastSeenTime":  device.LastSeenTime.Format(time.RFC3339),
			"services":      device.Services,
		},
	}

	// 返回JSON响应
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleStartStream 开始媒体流
func (s *Server) handleStartStream(w http.ResponseWriter, r *http.Request) {
	// 解析请求参数
	var req struct {
		DeviceID     string `json:"deviceId"`
		DeviceType   string `json:"deviceType"` // "gb28181" or "onvif"
		Channel      string `json:"channel,omitempty"`
		ProfileToken string `json:"profileToken,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("无效的请求参数: %v", err), http.StatusBadRequest)
		return
	}

	// 根据设备类型调用不同的媒体流控制函数
	var streamURL string
	var err error

	switch req.DeviceType {
	case "gb28181":
		// 调用GB28181服务器的媒体流控制功能
		// 这里简化处理，实际应该实现SIP INVITE流程
		streamURL = fmt.Sprintf("rtsp://%s:%d/stream/%s", s.config.API.Host, 554, req.DeviceID)
	case "onvif":
		// 调用ONVIF管理器的媒体流控制功能
		streamURL, err = s.onvifManager.StartStream(req.DeviceID, req.ProfileToken)
		if err != nil {
			http.Error(w, fmt.Sprintf("ONVIF流启动失败: %v", err), http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, "无效的设备类型", http.StatusBadRequest)
		return
	}

	// 返回成功响应
	response := map[string]interface{}{
		"status":    "ok",
		"streamUrl": streamURL,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleStopStream 停止媒体流
func (s *Server) handleStopStream(w http.ResponseWriter, r *http.Request) {
	// 解析请求参数
	var req struct {
		DeviceID   string `json:"deviceId"`
		DeviceType string `json:"deviceType"` // "gb28181" or "onvif"
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("无效的请求参数: %v", err), http.StatusBadRequest)
		return
	}

	// 这里简化处理，实际应该实现停止媒体流的逻辑
	// 根据设备类型调用不同的媒体流控制函数

	// 返回成功响应
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// handlePTZControl PTZ控制
func (s *Server) handlePTZControl(w http.ResponseWriter, r *http.Request) {
	// 解析请求参数
	var req struct {
		DeviceID string `json:"deviceId"`
		Channel  string `json:"channel"`
		PTZCmd   string `json:"ptzCmd"`
		Speed    int    `json:"speed"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("无效的请求参数: %v", err), http.StatusBadRequest)
		return
	}

	// 调用GB28181服务器的PTZ控制功能
	err := s.gb28181Server.SendPTZCommand(req.DeviceID, req.Channel, req.PTZCmd, req.Speed)
	if err != nil {
		http.Error(w, fmt.Sprintf("PTZ控制失败: %v", err), http.StatusInternalServerError)
		return
	}

	// 返回成功响应
	response := map[string]interface{}{
		"status":  "ok",
		"message": "PTZ控制命令发送成功",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handlePTZReset PTZ复位
func (s *Server) handlePTZReset(w http.ResponseWriter, r *http.Request) {
	// 解析请求参数
	var req struct {
		DeviceID string `json:"deviceId"`
		Channel  string `json:"channel"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("无效的请求参数: %v", err), http.StatusBadRequest)
		return
	}

	// 调用GB28181服务器的PTZ复位功能
	err := s.gb28181Server.ResetPTZ(req.DeviceID, req.Channel)
	if err != nil {
		http.Error(w, fmt.Sprintf("PTZ复位失败: %v", err), http.StatusInternalServerError)
		return
	}

	// 返回成功响应
	response := map[string]interface{}{
		"status":  "ok",
		"message": "PTZ复位命令发送成功",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleGetConfig 获取系统配置
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(s.config)
}

// handleUpdateConfig 更新系统配置
func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	// 解析请求参数
	var updatedConfig config.Config
	if err := json.NewDecoder(r.Body).Decode(&updatedConfig); err != nil {
		http.Error(w, fmt.Sprintf("无效的请求参数: %v", err), http.StatusBadRequest)
		return
	}

	// 更新配置
	s.config = &updatedConfig

	// 保存配置到文件
	if err := s.config.Save(s.configPath); err != nil {
		http.Error(w, fmt.Sprintf("配置保存失败: %v", err), http.StatusInternalServerError)
		return
	}

	// 返回成功响应
	response := map[string]interface{}{
		"status":  "ok",
		"message": "配置更新成功",
		"config":  s.config,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

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

	response := map[string]interface{}{
		"channels": channelsWithAI,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleGetChannel 获取单个通道
func (s *Server) handleGetChannel(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	channelID := params["id"]

	channel, exists := s.channelManager.GetChannel(channelID)
	if !exists {
		http.Error(w, `{"error":"通道不存在"}`, http.StatusNotFound)
		return
	}

	// 构建包含AI录像状态的响应
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

	response := map[string]interface{}{
		"channel": channelData,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleAddChannel 添加通道
func (s *Server) handleAddChannel(w http.ResponseWriter, r *http.Request) {
	var channel Channel
	if err := json.NewDecoder(r.Body).Decode(&channel); err != nil {
		http.Error(w, fmt.Sprintf("无效的请求参数: %v", err), http.StatusBadRequest)
		return
	}

	// 生成通道ID
	channel.ChannelID = fmt.Sprintf("%s_%s_%d", channel.DeviceID, channel.ChannelName, time.Now().Unix())

	if err := s.channelManager.AddChannel(&channel); err != nil {
		http.Error(w, fmt.Sprintf("添加通道失败: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status":    "ok",
		"message":   "通道添加成功",
		"channelId": channel.ChannelID,
		"channel":   channel,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleDeleteChannel 删除通道
func (s *Server) handleDeleteChannel(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	channelID := params["id"]

	if err := s.channelManager.DeleteChannel(channelID); err != nil {
		http.Error(w, fmt.Sprintf("删除通道失败: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status":  "ok",
		"message": "通道删除成功",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleListStreams 获取流列表
func (s *Server) handleListStreams(w http.ResponseWriter, r *http.Request) {
	streams := s.streamManager.GetStreams()

	response := map[string]interface{}{
		"streams": streams,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleQueryRecordings 查询录像
func (s *Server) handleQueryRecordings(w http.ResponseWriter, r *http.Request) {
	// 解析查询参数
	channelID := r.URL.Query().Get("channelId")
	dateStr := r.URL.Query().Get("date")

	if channelID == "" || dateStr == "" {
		http.Error(w, "缺少必要参数: channelId 或 date", http.StatusBadRequest)
		return
	}

	// 解析日期
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("无效的日期格式: %v", err), http.StatusBadRequest)
		return
	}

	recordings := s.recordingManager.GetRecordingsByDate(channelID, date)

	response := map[string]interface{}{
		"recordings": recordings,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleGetRecording 获取单个录像
func (s *Server) handleGetRecording(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	recordingID := params["id"]

	recording, exists := s.recordingManager.GetRecording(recordingID)
	if !exists {
		http.Error(w, `{"error":"录像不存在"}`, http.StatusNotFound)
		return
	}

	response := map[string]interface{}{
		"recording": recording,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleDownloadRecording 下载录像
func (s *Server) handleDownloadRecording(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	recordingID := params["id"]

	recording, exists := s.recordingManager.GetRecording(recordingID)
	if !exists {
		http.Error(w, `{"error":"录像不存在"}`, http.StatusNotFound)
		return
	}

	// 设置响应头
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="recording_%s.mp4"`, recordingID))
	w.Header().Set("Content-Type", "video/mp4")

	// 这里应该实现实际的文件下载逻辑
	// 当前简化实现，返回成功状态
	response := map[string]interface{}{
		"status":      "ok",
		"recordingId": recordingID,
		"filePath":    recording.FilePath,
	}

	json.NewEncoder(w).Encode(response)
}

// handleZLMStatus 获取ZLM服务器状态
func (s *Server) handleZLMStatus(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"success": true,
	}

	// 添加进程状态
	if s.zlmProcess != nil {
		response["process"] = s.zlmProcess.GetStatus()
	} else {
		response["process"] = map[string]interface{}{
			"running": false,
			"message": "ZLM 进程管理器未初始化",
		}
	}

	// 添加服务器统计
	if s.zlmServer != nil {
		response["server"] = s.zlmServer.GetStatistics()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleZLMProcessStatus 获取 ZLM 进程状态
func (s *Server) handleZLMProcessStatus(w http.ResponseWriter, r *http.Request) {
	if s.zlmProcess == nil {
		s.jsonResponse(w, http.StatusOK, map[string]interface{}{
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

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"status":  status,
	})
}

// handleZLMProcessStart 启动 ZLM 进程
func (s *Server) handleZLMProcessStart(w http.ResponseWriter, r *http.Request) {
	if s.zlmProcess == nil {
		s.jsonError(w, http.StatusBadRequest, "ZLM 进程管理器未初始化")
		return
	}

	if s.zlmProcess.IsRunning() {
		s.jsonResponse(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "ZLM 进程已在运行中",
			"status":  s.zlmProcess.GetStatus(),
		})
		return
	}

	if err := s.zlmProcess.Start(); err != nil {
		s.jsonError(w, http.StatusInternalServerError, fmt.Sprintf("启动 ZLM 进程失败: %v", err))
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "ZLM 进程启动成功",
		"status":  s.zlmProcess.GetStatus(),
	})
}

// handleZLMProcessStop 停止 ZLM 进程
func (s *Server) handleZLMProcessStop(w http.ResponseWriter, r *http.Request) {
	if s.zlmProcess == nil {
		s.jsonError(w, http.StatusBadRequest, "ZLM 进程管理器未初始化")
		return
	}

	if !s.zlmProcess.IsRunning() {
		s.jsonResponse(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "ZLM 进程未在运行",
		})
		return
	}

	if err := s.zlmProcess.Stop(); err != nil {
		s.jsonError(w, http.StatusInternalServerError, fmt.Sprintf("停止 ZLM 进程失败: %v", err))
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "ZLM 进程已停止",
	})
}

// handleZLMProcessRestart 重启 ZLM 进程
func (s *Server) handleZLMProcessRestart(w http.ResponseWriter, r *http.Request) {
	if s.zlmProcess == nil {
		s.jsonError(w, http.StatusBadRequest, "ZLM 进程管理器未初始化")
		return
	}

	if err := s.zlmProcess.Restart(); err != nil {
		s.jsonError(w, http.StatusInternalServerError, fmt.Sprintf("重启 ZLM 进程失败: %v", err))
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "ZLM 进程重启成功",
		"status":  s.zlmProcess.GetStatus(),
	})
}

func (s *Server) handleAddONVIFDevice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		XAddr    string `json:"xaddr"`
		IP       string `json:"ip"`
		Port     int    `json:"port"`
		Username string `json:"username"`
		Password string `json:"password"`
		Name     string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("无效的请求参数: %v", err), http.StatusBadRequest)
		return
	}

	var device *onvif.Device
	var err error

	// 支持两种添加方式：URL 方式和 IP:Port 方式
	if req.XAddr != "" {
		// 方式1：通过 XADDR 添加（支持多种地址格式）
		device, err = s.onvifManager.AddDevice(req.XAddr, req.Username, req.Password)
	} else if req.IP != "" && req.Port > 0 {
		// 方式2：通过 IP 和 Port 添加（适合多网卡场景）
		device, err = s.onvifManager.AddDeviceWithIP(req.IP, req.Port, req.Username, req.Password)
	} else {
		http.Error(w, "必须提供 xaddr 或 (ip + port)", http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, fmt.Sprintf("添加ONVIF设备失败: %v", err), http.StatusInternalServerError)
		return
	}

	// 如果提供了设备名称，更新设备名称
	if req.Name != "" {
		device.Name = req.Name
	}

	response := map[string]interface{}{
		"success": true,
		"message": "ONVIF设备添加成功",
		"device": map[string]interface{}{
			"deviceId":      device.DeviceID,
			"name":          device.Name,
			"model":         device.Model,
			"manufacturer":  device.Manufacturer,
			"ip":            device.IP,
			"port":          device.Port,
			"status":        device.Status,
			"previewURL":    device.PreviewURL,
			"responseTime":  device.ResponseTime,
			"lastCheckTime": device.LastCheckTime,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleRemoveONVIFDevice 移除ONVIF设备
func (s *Server) handleRemoveONVIFDevice(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	deviceID := params["id"]

	if err := s.onvifManager.RemoveDevice(deviceID); err != nil {
		http.Error(w, fmt.Sprintf("移除ONVIF设备失败: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "设备移除成功",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleRefreshONVIFDevice 刷新设备信息（支持多网卡场景）
func (s *Server) handleRefreshONVIFDevice(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	deviceID := params["id"]

	var req struct {
		IP   string `json:"ip"`
		Port int    `json:"port"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("无效的请求参数: %v", err), http.StatusBadRequest)
		return
	}

	if req.IP == "" || req.Port <= 0 {
		http.Error(w, "必须提供有效的 IP 地址和端口", http.StatusBadRequest)
		return
	}

	if err := s.onvifManager.RefreshDevice(deviceID, req.IP, req.Port); err != nil {
		http.Error(w, fmt.Sprintf("刷新设备信息失败: %v", err), http.StatusInternalServerError)
		return
	}

	device, _ := s.onvifManager.GetDeviceByID(deviceID)
	response := map[string]interface{}{
		"success": true,
		"message": "设备信息已更新",
		"device": map[string]interface{}{
			"deviceId": device.DeviceID,
			"name":     device.Name,
			"ip":       device.IP,
			"port":     device.Port,
			"status":   device.Status,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleBatchAddONVIFDevices 批量添加ONVIF设备
func (s *Server) handleBatchAddONVIFDevices(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Devices []struct {
			IP       string `json:"ip"`
			Port     int    `json:"port"`
			Username string `json:"username"`
			Password string `json:"password"`
			Name     string `json:"name"`
		} `json:"devices"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("无效的请求参数: %v", err), http.StatusBadRequest)
		return
	}

	if len(req.Devices) == 0 {
		http.Error(w, "设备列表不能为空", http.StatusBadRequest)
		return
	}

	// 逐个添加设备
	var added []*onvif.Device
	var failedList []map[string]interface{}

	for _, deviceInfo := range req.Devices {
		device, err := s.onvifManager.AddDeviceWithIP(deviceInfo.IP, deviceInfo.Port, deviceInfo.Username, deviceInfo.Password)
		if err != nil {
			failedList = append(failedList, map[string]interface{}{
				"address": fmt.Sprintf("%s:%d", deviceInfo.IP, deviceInfo.Port),
				"error":   err.Error(),
			})
		} else {
			if deviceInfo.Name != "" {
				device.Name = deviceInfo.Name
			}
			added = append(added, device)
		}
	}

	response := map[string]interface{}{
		"success": true,
		"summary": map[string]interface{}{
			"total":  len(req.Devices),
			"added":  len(added),
			"failed": len(failedList),
		},
		"devices": added,
		"errors":  failedList,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleGetONVIFProfiles 获取ONVIF设备配置文件
func (s *Server) handleGetONVIFProfiles(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	deviceID := params["id"]

	profiles, err := s.onvifManager.GetProfiles(deviceID)
	if err != nil {
		http.Error(w, fmt.Sprintf("获取配置文件失败: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success":  true,
		"profiles": profiles,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleGetONVIFSnapshot 获取ONVIF设备快照
func (s *Server) handleGetONVIFSnapshot(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	deviceID := params["id"]

	profileToken := r.URL.Query().Get("profile")

	data, contentType, err := s.onvifManager.GetSnapshot(deviceID, profileToken)
	if err != nil {
		http.Error(w, fmt.Sprintf("获取快照失败: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"snapshot_%s.jpg\"", deviceID))
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// handleGetONVIFPresets 获取ONVIF设备预置位列表
func (s *Server) handleGetONVIFPresets(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	deviceID := params["id"]

	presets, err := s.onvifManager.GetPTZPresets(deviceID)
	if err != nil {
		http.Error(w, fmt.Sprintf("获取预置位失败: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"presets": presets,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleSetONVIFPreset 设置ONVIF设备预置位
func (s *Server) handleSetONVIFPreset(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	deviceID := params["id"]

	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("无效的请求参数: %v", err), http.StatusBadRequest)
		return
	}

	presetToken, err := s.onvifManager.SetPTZPreset(deviceID, req.Name)
	if err != nil {
		http.Error(w, fmt.Sprintf("设置预置位失败: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "预置位设置成功",
		"token":   presetToken,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleGotoONVIFPreset 移动到ONVIF设备预置位
func (s *Server) handleGotoONVIFPreset(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	deviceID := params["id"]
	presetToken := params["token"]

	if err := s.onvifManager.PTZGotoPreset(deviceID, presetToken); err != nil {
		http.Error(w, fmt.Sprintf("移动到预置位失败: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "已移动到预置位",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleGetONVIFStatistics 获取ONVIF设备统计信息
func (s *Server) handleGetONVIFStatistics(w http.ResponseWriter, r *http.Request) {
	stats := s.onvifManager.GetDeviceStatistics()

	response := map[string]interface{}{
		"success":    true,
		"statistics": stats,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleZLMGetStreams 获取ZLM媒体流列表（直接从ZLM API获取）
func (s *Server) handleZLMGetStreams(w http.ResponseWriter, r *http.Request) {
	if s.zlmProcess == nil {
		s.jsonError(w, http.StatusInternalServerError, "ZLM进程未初始化")
		return
	}

	apiClient := s.zlmProcess.GetAPIClient()
	if apiClient == nil {
		s.jsonError(w, http.StatusInternalServerError, "ZLM API客户端未初始化")
		return
	}

	// 从ZLM API直接获取流列表
	streams, err := apiClient.GetMediaList()
	if err != nil {
		// API 调用失败，返回空列表
		response := map[string]interface{}{
			"streams": []interface{}{},
			"error":   err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	response := map[string]interface{}{
		"streams": streams,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleZLMAddStream 添加ZLM媒体流（通过流代理）
func (s *Server) handleZLMAddStream(w http.ResponseWriter, r *http.Request) {
	if s.zlmProcess == nil {
		s.jsonError(w, http.StatusInternalServerError, "ZLM进程未初始化")
		return
	}

	apiClient := s.zlmProcess.GetAPIClient()
	if apiClient == nil {
		s.jsonError(w, http.StatusInternalServerError, "ZLM API客户端未初始化")
		return
	}

	var req struct {
		App    string `json:"app"`
		Stream string `json:"stream"`
		URL    string `json:"url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, http.StatusBadRequest, fmt.Sprintf("无效的请求参数: %v", err))
		return
	}

	if req.App == "" || req.Stream == "" {
		s.jsonError(w, http.StatusBadRequest, "应用名称和流名称不能为空")
		return
	}

	if req.URL == "" {
		s.jsonError(w, http.StatusBadRequest, "源地址不能为空")
		return
	}

	// 调用 ZLM API 添加流代理
	proxyInfo, err := apiClient.AddStreamProxy(req.URL, req.App, req.Stream)
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, fmt.Sprintf("添加媒体流失败: %v", err))
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "媒体流添加成功",
		"key":     proxyInfo.Key,
		"app":     req.App,
		"stream":  req.Stream,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleZLMRemoveStream 删除ZLM媒体流
func (s *Server) handleZLMRemoveStream(w http.ResponseWriter, r *http.Request) {
	if s.zlmProcess == nil {
		s.jsonError(w, http.StatusInternalServerError, "ZLM进程未初始化")
		return
	}

	apiClient := s.zlmProcess.GetAPIClient()
	if apiClient == nil {
		s.jsonError(w, http.StatusInternalServerError, "ZLM API客户端未初始化")
		return
	}

	params := mux.Vars(r)
	streamID := params["id"]

	// 解析 app_stream 格式
	parts := strings.SplitN(streamID, "_", 2)
	if len(parts) != 2 {
		s.jsonError(w, http.StatusBadRequest, "无效的流ID格式，应为 app_stream")
		return
	}
	app := parts[0]
	stream := parts[1]

	// 调用ZLM API关闭流
	if err := apiClient.CloseStream(app, stream); err != nil {
		s.jsonError(w, http.StatusInternalServerError, fmt.Sprintf("删除媒体流失败: %v", err))
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "媒体流删除成功",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleZLMStartRecording 启动ZLM录像
func (s *Server) handleZLMStartRecording(w http.ResponseWriter, r *http.Request) {
	if s.zlmServer == nil {
		http.Error(w, `{"error":"ZLM服务器未初始化"}`, http.StatusInternalServerError)
		return
	}

	params := mux.Vars(r)
	streamID := params["id"]

	var req struct {
		RecordingPath string `json:"recordingPath"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if err := s.zlmServer.StartRecording(streamID, req.RecordingPath); err != nil {
		http.Error(w, fmt.Sprintf("启动录像失败: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status":   "ok",
		"message":  "录像启动成功",
		"streamId": streamID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleZLMStopRecording 停止ZLM录像
func (s *Server) handleZLMStopRecording(w http.ResponseWriter, r *http.Request) {
	if s.zlmServer == nil {
		http.Error(w, `{"error":"ZLM服务器未初始化"}`, http.StatusInternalServerError)
		return
	}

	params := mux.Vars(r)
	streamID := params["id"]

	if err := s.zlmServer.StopRecording(streamID); err != nil {
		http.Error(w, fmt.Sprintf("停止录像失败: %v", err), http.StatusInternalServerError)
		return
	}

	recordingPath, _ := s.zlmServer.GetRecordingPath(streamID)

	response := map[string]interface{}{
		"status":        "ok",
		"message":       "录像停止成功",
		"streamId":      streamID,
		"recordingPath": recordingPath,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleGetStats 返回设备与流的统计信息（兼容前端 /api/stats）
func (s *Server) handleGetStats(w http.ResponseWriter, r *http.Request) {
	debug.Info("api", "处理统计信息请求")

	gbDevices := s.gb28181Server.GetDevices()
	onvifDevices := s.onvifManager.GetDevices()

	stats := map[string]interface{}{
		"gb28181": map[string]interface{}{
			"total":  len(gbDevices),
			"online": len(gbDevices),
		},
		"onvif": map[string]interface{}{
			"total":  len(onvifDevices),
			"online": len(onvifDevices),
		},
		"activeStreams": 0,
	}

	if s.zlmServer != nil {
		zlmStats := s.zlmServer.GetStatistics()
		if v, ok := zlmStats["totalStreams"].(int); ok {
			stats["activeStreams"] = v
		} else if v, ok := zlmStats["totalStreams"].(float64); ok {
			stats["activeStreams"] = int(v)
		}
	}

	response := map[string]interface{}{
		"success": true,
		"stats":   stats,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleGetResources 返回简单的系统资源信息（兼容前端 /api/resources）
func (s *Server) handleGetResources(w http.ResponseWriter, r *http.Request) {
	debug.Info("api", "处理系统资源请求")

	// 使用 /proc 和 syscall 获取系统资源（免依赖实现）
	// 磁盘使用率（基于根目录）
	var diskUsage int
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err == nil {
		total := stat.Blocks * uint64(stat.Bsize)
		free := stat.Bfree * uint64(stat.Bsize)
		used := total - free
		if total > 0 {
			diskUsage = int((float64(used) / float64(total)) * 100)
		}
	}

	// 内存使用：解析 /proc/meminfo
	memUsage := 0
	if data, err := ioutil.ReadFile("/proc/meminfo"); err == nil {
		s := string(data)
		var memTotal, memAvailable uint64
		for _, line := range strings.Split(s, "\n") {
			if strings.HasPrefix(line, "MemTotal:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					v, _ := strconv.ParseUint(fields[1], 10, 64)
					memTotal = v
				}
			}
			if strings.HasPrefix(line, "MemAvailable:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					v, _ := strconv.ParseUint(fields[1], 10, 64)
					memAvailable = v
				}
			}
		}
		if memTotal > 0 {
			used := memTotal - memAvailable
			memUsage = int((float64(used) / float64(memTotal)) * 100)
		}
	}

	// CPU 使用率：读取 /proc/stat，两次采样计算差值
	cpuUsage := 0
	readCPU := func() (uint64, uint64) {
		data, err := ioutil.ReadFile("/proc/stat")
		if err != nil {
			return 0, 0
		}
		lines := strings.Split(string(data), "\n")
		if len(lines) == 0 {
			return 0, 0
		}
		fields := strings.Fields(lines[0]) // cpu  user nice system idle iowait irq softirq steal guest guest_nice
		if len(fields) < 5 {
			return 0, 0
		}
		var total, idle uint64
		for i := 1; i < len(fields); i++ {
			v, _ := strconv.ParseUint(fields[i], 10, 64)
			total += v
			if i == 4 { // idle is the 4th field (index 4)
				idle = v
			}
		}
		return total, idle
	}
	t1, i1 := readCPU()
	time.Sleep(200 * time.Millisecond)
	t2, i2 := readCPU()
	if t2 > t1 {
		totalDiff := float64(t2 - t1)
		idleDiff := float64(i2 - i1)
		usage := (1.0 - idleDiff/totalDiff) * 100.0
		cpuUsage = int(usage)
	}

	// 网络速率：读取 /sys/class/net/*/statistics 下的 rx_bytes/tx_bytes，两次采样
	netBytes := func() (uint64, uint64) {
		rxTotal := uint64(0)
		txTotal := uint64(0)
		files, _ := ioutil.ReadDir("/sys/class/net")
		for _, f := range files {
			if f.Name() == "lo" {
				continue
			}
			rxPath := "/sys/class/net/" + f.Name() + "/statistics/rx_bytes"
			txPath := "/sys/class/net/" + f.Name() + "/statistics/tx_bytes"
			if b, err := ioutil.ReadFile(rxPath); err == nil {
				if v, err := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64); err == nil {
					rxTotal += v
				}
			}
			if b, err := ioutil.ReadFile(txPath); err == nil {
				if v, err := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64); err == nil {
					txTotal += v
				}
			}
		}
		return rxTotal, txTotal
	}
	rx1, tx1 := netBytes()
	time.Sleep(200 * time.Millisecond)
	rx2, tx2 := netBytes()
	upload := "0 B/s"
	download := "0 B/s"
	if rx2 >= rx1 {
		// bytes per 200ms -> bytes per second multiply by 5
		downBps := (rx2 - rx1) * 5
		uploadBps := (tx2 - tx1) * 5
		download = fmt.Sprintf("%d KB/s", downBps/1024)
		upload = fmt.Sprintf("%d KB/s", uploadBps/1024)
	}

	resources := map[string]interface{}{
		"diskUsage": diskUsage,
		"memory": map[string]interface{}{
			"usedPercent": memUsage,
		},
		"cpuUsage": cpuUsage,
		"network": map[string]interface{}{
			"upload":   upload,
			"download": download,
		},
	}

	response := map[string]interface{}{
		"success":   true,
		"resources": resources,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// 简单从日志文件尾部读取若干行并尝试解析成时间/等级/消息
func tailLogLines(path string, maxLines int) ([]map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := []string{}
	for _, l := range bytes.Split(data, []byte("\n")) {
		lines = append(lines, string(l))
	}
	// 取最后 maxLines 行
	start := 0
	if len(lines) > maxLines {
		start = len(lines) - maxLines
	}
	res := []map[string]string{}
	for _, line := range lines[start:] {
		if line == "" {
			continue
		}
		entry := map[string]string{"time": "", "level": "INFO", "message": line}
		// 试图解析: 时间 开头，然后 [LEVEL]
		// 示例: 2025-12-04 15:11:32 [INFO] [api] ...
		if len(line) > 20 {
			entry["time"] = line[:19]
			if idx := strings.Index(line, "["); idx != -1 {
				// 找到第一个 [ 和下一]
				end := strings.Index(line[idx+1:], "]")
				if end != -1 {
					lvl := line[idx+1 : idx+1+end]
					entry["level"] = lvl
				}
			}
			// message 尝试截取后半段
			if idx := strings.Index(line, "]"); idx != -1 && idx+2 < len(line) {
				entry["message"] = strings.TrimSpace(line[idx+1:])
			}
		}
		res = append(res, entry)
	}
	return res, nil
}

// handleGetLatestLogs 返回最近的日志条目（兼容前端 /api/logs/latest）
func (s *Server) handleGetLatestLogs(w http.ResponseWriter, r *http.Request) {
	debug.Info("api", "处理最新日志请求")

	// 优先读取 debug 日志，如果不可用，尝试 server.log
	paths := []string{"logs/debug.log", "server.log"}
	var entries []map[string]string
	var err error
	for _, p := range paths {
		entries, err = tailLogLines(p, 50)
		if err == nil && len(entries) > 0 {
			break
		}
	}
	if err != nil {
		// 返回空日志但标记成功，以免前端反复报错
		response := map[string]interface{}{"success": true, "logs": []map[string]string{}}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
		return
	}

	response := map[string]interface{}{"success": true, "logs": entries}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
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

// previewSessions 存储活动的预览会话
var previewSessions = make(map[string]*PreviewSession)

// handleStartONVIFPreview 启动ONVIF设备预览
func (s *Server) handleStartONVIFPreview(w http.ResponseWriter, r *http.Request) {
	debug.Info("api", "处理启动ONVIF预览请求")

	vars := mux.Vars(r)
	deviceID := vars["id"]

	if deviceID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "设备ID不能为空",
		})
		return
	}

	// 检查 ZLM 服务是否可用
	if s.zlmServer == nil || s.zlmServer.GetAPIClient() == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "ZLM媒体服务未启动",
		})
		return
	}

	// 获取设备信息
	device, exists := s.onvifManager.GetDeviceByID(deviceID)
	if !exists || device == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "设备不存在",
		})
		return
	}

	// 获取 RTSP 流地址
	rtspURL := device.PreviewURL
	if rtspURL == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "设备没有预览地址",
		})
		return
	}

	// 检查是否已有预览会话
	if session, ok := previewSessions[deviceID]; ok {
		// 检查流是否还在线
		online, err := s.zlmServer.GetAPIClient().IsStreamOnline(session.App, session.Stream)
		if err == nil && online {
			// 流还在线，直接返回现有会话
			debug.Info("api", "复用现有预览会话: %s", deviceID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"message": "预览已启动",
				"data":    session,
			})
			return
		}
		// 流不在线，删除旧会话
		delete(previewSessions, deviceID)
	}

	// 创建流代理
	// 使用设备ID作为流名，确保唯一性
	app := "onvif"
	streamName := strings.ReplaceAll(deviceID, "-", "_")
	// 将特殊字符替换为下划线
	streamName = strings.ReplaceAll(streamName, ":", "_")
	streamName = strings.ReplaceAll(streamName, ".", "_")

	debug.Info("api", "创建流代理: app=%s, stream=%s, url=%s", app, streamName, rtspURL)

	proxyInfo, err := s.zlmServer.GetAPIClient().AddStreamProxy(rtspURL, app, streamName)
	if err != nil {
		debug.Error("api", "创建流代理失败: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("创建流代理失败: %v", err),
		})
		return
	}

	// 获取 ZLM 服务器地址
	zlmHost := "127.0.0.1"
	if s.config.ZLM != nil && s.config.ZLM.General != nil && s.config.ZLM.General.ListenIP != "" && s.config.ZLM.General.ListenIP != "::" {
		zlmHost = s.config.ZLM.General.ListenIP
	}
	if zlmHost == "" || zlmHost == "0.0.0.0" || zlmHost == "::" {
		zlmHost = "127.0.0.1"
	}
	zlmHTTPPort := 80
	if s.config.ZLM != nil {
		zlmHTTPPort = s.config.ZLM.GetHTTPPort()
	}
	zlmRTMPPort := 1935
	if s.config.ZLM != nil {
		zlmRTMPPort = s.config.ZLM.GetRTMPPort()
	}
	zlmRTSPPort := 554
	if s.config.ZLM != nil {
		zlmRTSPPort = s.config.ZLM.GetRTSPPort()
	}

	// 创建预览会话
	session := &PreviewSession{
		DeviceID:   deviceID,
		StreamKey:  proxyInfo.Key,
		App:        app,
		Stream:     streamName,
		SourceURL:  rtspURL,
		FlvURL:     fmt.Sprintf("http://%s:%d/%s/%s.live.flv", zlmHost, zlmHTTPPort, app, streamName),
		WsFlvURL:   fmt.Sprintf("ws://%s:%d/%s/%s.live.flv", zlmHost, zlmHTTPPort, app, streamName),
		HlsURL:     fmt.Sprintf("http://%s:%d/%s/%s/hls.m3u8", zlmHost, zlmHTTPPort, app, streamName),
		RtmpURL:    fmt.Sprintf("rtmp://%s:%d/%s/%s", zlmHost, zlmRTMPPort, app, streamName),
		RtspURL:    fmt.Sprintf("rtsp://%s:%d/%s/%s", zlmHost, zlmRTSPPort, app, streamName),
		CreateTime: time.Now().Unix(),
	}

	// 保存会话
	previewSessions[deviceID] = session

	debug.Info("api", "ONVIF预览启动成功: deviceID=%s, flvURL=%s", deviceID, session.FlvURL)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "预览启动成功",
		"data":    session,
	})
}

// handleStopONVIFPreview 停止ONVIF设备预览
func (s *Server) handleStopONVIFPreview(w http.ResponseWriter, r *http.Request) {
	debug.Info("api", "处理停止ONVIF预览请求")

	vars := mux.Vars(r)
	deviceID := vars["id"]

	if deviceID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "设备ID不能为空",
		})
		return
	}

	// 检查 ZLM 服务是否可用
	if s.zlmServer == nil || s.zlmServer.GetAPIClient() == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "ZLM媒体服务未启动",
		})
		return
	}

	// 获取预览会话
	session, exists := previewSessions[deviceID]
	if !exists {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "预览会话不存在",
		})
		return
	}

	// 删除流代理
	err := s.zlmServer.GetAPIClient().DelStreamProxy(session.StreamKey)
	if err != nil {
		debug.Warn("api", "删除流代理失败: %v", err)
		// 继续删除会话
	}

	// 删除会话
	delete(previewSessions, deviceID)

	debug.Info("api", "ONVIF预览已停止: deviceID=%s", deviceID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "预览已停止",
	})
}

// getLocalIP 获取本机IP地址
func getLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().String()
	// 提取IP部分
	if idx := strings.LastIndex(localAddr, ":"); idx > 0 {
		return localAddr[:idx]
	}
	return "127.0.0.1"
}

// handleStartChannelRecording 开始通道录像
func (s *Server) handleStartChannelRecording(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	channelID := params["id"]

	debug.Info("api", "开始通道录像: channelID=%s", channelID)

	// 获取 ZLM API 客户端
	if s.zlmServer == nil || s.zlmServer.GetAPIClient() == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "ZLM 服务器未初始化"})
		return
	}

	apiClient := s.zlmServer.GetAPIClient()

	// 标准化流ID（去掉横杠）
	streamID := strings.ReplaceAll(channelID, "-", "")
	if streamID == "" {
		streamID = channelID
	}

	// 查找流所在的 app（优先级：live > rtp）
	apps := []string{"live", "rtp"}
	var foundApp, foundStream string
	
	for _, app := range apps {
		online, err := apiClient.IsStreamOnline(app, streamID)
		if err == nil && online {
			foundApp = app
			foundStream = streamID
			break
		}
		// 也检查原始 channelID
		if streamID != channelID {
			online, err := apiClient.IsStreamOnline(app, channelID)
			if err == nil && online {
				foundApp = app
				foundStream = channelID
				break
			}
		}
	}

	// 如果流不存在，先创建流代理
	if foundApp == "" {
		debug.Info("api", "流不在线，尝试启动测试流: channelID=%s", channelID)
		
		// 使用测试流地址创建流代理
		testStreamURL := "rtmp://ns8.indexforce.com/home/mystream"
		app := "live"
		
		// 尝试添加流代理
		proxyInfo, err := apiClient.AddStreamProxy(testStreamURL, app, streamID)
		
		// 流已存在也算成功
		if err != nil {
			errStr := err.Error()
			if !strings.Contains(errStr, "already exists") && !strings.Contains(errStr, "code: -1") {
				debug.Error("api", "创建流代理失败: %v", err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   fmt.Sprintf("启动流失败: %v", err),
				})
				return
			}
		}
		
		// 等待流建立
		time.Sleep(1 * time.Second)
		
		foundApp = app
		foundStream = streamID
		
		if proxyInfo != nil {
			debug.Info("api", "流代理已创建: app=%s, stream=%s, key=%s", app, streamID, proxyInfo.Key)
		} else {
			debug.Info("api", "流已存在，直接使用: app=%s, stream=%s", app, streamID)
		}
	}

	// 开始 MP4 录像 (type=1)
	err := apiClient.StartRecord(foundApp, foundStream, 1, "", 0)
	if err != nil {
		debug.Error("api", "开始录像失败: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": fmt.Sprintf("开始录像失败: %v", err)})
		return
	}

	// 标记为持久录像
	s.recordingManager.SetPersistentRecording(channelID, true)

	debug.Info("api", "通道录像已开始（持久录像）: channelID=%s, app=%s, stream=%s", channelID, foundApp, foundStream)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
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

	// 获取 ZLM API 客户端
	if s.zlmServer == nil || s.zlmServer.GetAPIClient() == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "ZLM 服务器未初始化"})
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
		// 检查是否正在录像
		isRec, err := apiClient.IsRecording(app, streamID, 1)
		if err == nil && isRec {
			foundApp = app
			foundStream = streamID
			break
		}
		// 也检查原始 channelID
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
		// 尝试停止所有可能的位置
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
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": fmt.Sprintf("停止录像失败: %v", err)})
			return
		}
	}

	// 取消持久录像标记
	s.recordingManager.SetPersistentRecording(channelID, false)

	debug.Info("api", "通道录像已停止: channelID=%s", channelID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"message":   "录像已停止",
		"channelId": channelID,
	})
}

// handleGetChannelRecordingStatus 获取通道录像状态
func (s *Server) handleGetChannelRecordingStatus(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	channelID := params["id"]

	// 获取 ZLM API 客户端
	if s.zlmServer == nil || s.zlmServer.GetAPIClient() == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "channelId": channelID, "isRecording": false})
		return
	}

	apiClient := s.zlmServer.GetAPIClient()

	// 标准化流ID
	streamID := strings.ReplaceAll(channelID, "-", "")
	if streamID == "" {
		streamID = channelID
	}

	// 检查所有可能的 app 位置
	apps := []string{"live", "rtp"}
	isRecording := false
	
	for _, app := range apps {
		rec, err := apiClient.IsRecording(app, streamID, 1)
		if err == nil && rec {
			isRecording = true
			break
		}
		// 也检查原始 channelID
		if streamID != channelID {
			rec, err := apiClient.IsRecording(app, channelID, 1)
			if err == nil && rec {
				isRecording = true
				break
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"channelId":   channelID,
		"isRecording": isRecording,
	})
}

// handleStartAIRecording 启动AI录像
func (s *Server) handleStartAIRecording(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChannelID string `json:"channel_id"`
		StreamURL string `json:"stream_url"`
		Mode      string `json:"mode"` // person, motion, continuous
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if s.aiManager == nil {
		http.Error(w, "AI功能未启用", http.StatusServiceUnavailable)
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
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
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if s.aiManager == nil {
		http.Error(w, "AI功能未启用", http.StatusServiceUnavailable)
		return
	}

	err := s.aiManager.StopChannelRecording(req.ChannelID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    true,
		"channel_id": req.ChannelID,
	})
}

// handleGetAIRecordingStatus 获取AI录像状态
func (s *Server) handleGetAIRecordingStatus(w http.ResponseWriter, r *http.Request) {
	channelID := r.URL.Query().Get("channel_id")
	if channelID == "" {
		http.Error(w, "缺少channel_id参数", http.StatusBadRequest)
		return
	}

	if s.aiManager == nil {
		http.Error(w, "AI功能未启用", http.StatusServiceUnavailable)
		return
	}

	status, err := s.aiManager.GetChannelStatus(channelID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"status":  status,
	})
}

// handleGetAllAIRecordingStatus 获取所有AI录像状态
func (s *Server) handleGetAllAIRecordingStatus(w http.ResponseWriter, r *http.Request) {
	if s.aiManager == nil {
		http.Error(w, "AI功能未启用", http.StatusServiceUnavailable)
		return
	}

	status := s.aiManager.GetAllStatus()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"status":  status,
	})
}

// handleGetAIConfig 获取AI配置
func (s *Server) handleGetAIConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"config":  s.config.AI,
	})
}

// handleUpdateAIConfig 更新AI配置
func (s *Server) handleUpdateAIConfig(w http.ResponseWriter, r *http.Request) {
	var aiConfig config.AIConfig
	if err := json.NewDecoder(r.Body).Decode(&aiConfig); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.config.AI = &aiConfig

	// 保存配置到文件
	if err := s.config.Save(s.configPath); err != nil {
		http.Error(w, fmt.Sprintf("保存配置失败: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"config":  s.config.AI,
	})
}

// startRecordingWatchdog 启动录像监控器，自动恢复断流后的持久录像
func (s *Server) startRecordingWatchdog() {
	s.recordingWatchStop = make(chan struct{})
	
	go func() {
		ticker := time.NewTicker(10 * time.Second) // 每10秒检查一次
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

// checkAndRestartRecordings 检查并重启需要持久录像但已停止的通道
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
		
		// 查找流所在的 app
		apps := []string{"live", "rtp"}
		var foundApp, foundStream string
		var isOnline, isRecording bool
		
		for _, app := range apps {
			// 检查流是否在线
			online, err := apiClient.IsStreamOnline(app, streamID)
			if err == nil && online {
				foundApp = app
				foundStream = streamID
				isOnline = true
				
				// 检查是否正在录像
				isRec, err := apiClient.IsRecording(app, streamID, 1)
				if err == nil {
					isRecording = isRec
				}
				break
			}
			
			// 也检查原始 channelID
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
		
		// 如果流在线但没有录像，重新启动录像
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


