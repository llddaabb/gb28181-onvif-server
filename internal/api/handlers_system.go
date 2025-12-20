package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"gb28181-onvif-server/internal/debug"
)

// handleHealthCheck 健康检查
func (s *Server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	respondRaw(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleGetStatus 获取系统状态
func (s *Server) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	debug.Info("api", "处理系统状态请求")

	gb28181Devices := s.gb28181Server.GetDevices()
	onvifDevices := s.onvifManager.GetDevices()

	// 计算运行时长
	uptime := time.Since(s.startTime)
	days := int(uptime.Hours()) / 24
	hours := int(uptime.Hours()) % 24
	minutes := int(uptime.Minutes()) % 60
	uptimeStr := fmt.Sprintf("%d天 %d小时 %d分钟", days, hours, minutes)

	// 获取系统资源使用情况
	memUsage := getMemoryUsage()
	cpuUsage := getCPUUsage()

	// ZLM 状态
	zlmStatus := "stopped"
	zlmStreams := 0
	if s.zlmServer != nil {
		zlmStatus = "running"
		stats := s.zlmServer.GetStatistics()
		if activeStreams, ok := stats["activeStreams"].(int); ok {
			zlmStreams = activeStreams
		}
	}

	// GB28181 和 ONVIF 服务状态
	gb28181Status := "stopped"
	if s.gb28181Running {
		gb28181Status = "running"
	}
	onvifStatus := "stopped"
	if s.onvifRunning {
		onvifStatus = "running"
	}

	response := map[string]interface{}{
		"status": "running",
		"servers": map[string]interface{}{
			"gb28181": map[string]interface{}{
				"status":  gb28181Status,
				"enabled": s.gb28181Running,
				"devices": len(gb28181Devices),
			},
			"onvif": map[string]interface{}{
				"status":  onvifStatus,
				"enabled": s.onvifRunning,
				"devices": len(onvifDevices),
			},
			"zlm": map[string]interface{}{
				"status":  zlmStatus,
				"streams": zlmStreams,
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
			"activeStreams":  zlmStreams,
		},
		"serverInfo": map[string]interface{}{
			"startTime":   s.startTime.Format("2006-01-02 15:04:05"),
			"uptime":      uptimeStr,
			"memoryUsage": fmt.Sprintf("%d%%", memUsage),
			"cpuUsage":    fmt.Sprintf("%d%%", cpuUsage),
		},
	}

	respondRaw(w, http.StatusOK, response)
}

// handleGetStats 获取统计信息（兼容前端）
func (s *Server) handleGetStats(w http.ResponseWriter, r *http.Request) {
	gb28181Devices := s.gb28181Server.GetDevices()
	onvifDevices := s.onvifManager.GetDevices()

	activeStreams := 0
	if s.zlmServer != nil {
		stats := s.zlmServer.GetStatistics()
		if val, ok := stats["activeStreams"].(int); ok {
			activeStreams = val
		}
	}

	response := map[string]interface{}{
		"totalDevices":   len(gb28181Devices) + len(onvifDevices),
		"gb28181Devices": len(gb28181Devices),
		"onvifDevices":   len(onvifDevices),
		"activeStreams":  activeStreams,
		"recordingCount": 0,
		"storageUsed":    0,
		"storageTotal":   0,
		"storagePercent": 0,
	}

	// 从系统获取磁盘统计
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err == nil {
		total := stat.Blocks * uint64(stat.Bsize)
		free := stat.Bfree * uint64(stat.Bsize)
		used := total - free
		response["storageUsed"] = used
		response["storageTotal"] = total
		if total > 0 {
			response["storagePercent"] = float64(used) / float64(total) * 100
		}
	}

	respondRaw(w, http.StatusOK, response)
}

// handleGetResources 获取资源信息
func (s *Server) handleGetResources(w http.ResponseWriter, r *http.Request) {
	// 获取磁盘使用率
	diskUsage := 0
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err == nil {
		total := stat.Blocks * uint64(stat.Bsize)
		free := stat.Bfree * uint64(stat.Bsize)
		used := total - free
		if total > 0 {
			diskUsage = int((float64(used) / float64(total)) * 100)
		}
	}

	response := map[string]interface{}{
		"cpu": map[string]interface{}{
			"usage": getCPUUsage(),
		},
		"memory": map[string]interface{}{
			"usage": getMemoryUsage(),
		},
		"disk": map[string]interface{}{
			"usage": diskUsage,
		},
	}

	respondRaw(w, http.StatusOK, response)
}

// handleGetLatestLogs 获取最新日志
func (s *Server) handleGetLatestLogs(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if val, err := strconv.Atoi(limitStr); err == nil && val > 0 {
			limit = val
		}
	}

	// 读取日志文件
	logFile := "logs/debug.log"
	logs := make([]map[string]interface{}, 0)

	if data, err := ioutil.ReadFile(logFile); err == nil {
		lines := strings.Split(string(data), "\n")
		start := len(lines) - limit
		if start < 0 {
			start = 0
		}
		for i := start; i < len(lines); i++ {
			if lines[i] != "" {
				logs = append(logs, map[string]interface{}{
					"message": lines[i],
					"index":   i,
				})
			}
		}
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"logs":  logs,
		"count": len(logs),
	})
}

// handleGetConfig 获取配置
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	// 返回完整的配置信息
	config := map[string]interface{}{
		"GB28181": map[string]interface{}{
			"SipIP":             s.config.GB28181.SipIP,
			"SipPort":           s.config.GB28181.SipPort,
			"Realm":             s.config.GB28181.Realm,
			"ServerID":          s.config.GB28181.ServerID,
			"Password":          "", // 不返回密码
			"HeartbeatInterval": s.config.GB28181.HeartbeatInterval,
			"RegisterExpires":   s.config.GB28181.RegisterExpires,
		},
		"API": map[string]interface{}{
			"Host":             s.config.API.Host,
			"Port":             s.config.API.Port,
			"CorsAllowOrigins": s.config.API.CorsAllowOrigins,
		},
	}

	if s.config.ONVIF != nil {
		config["ONVIF"] = map[string]interface{}{
			"CheckInterval":     s.config.ONVIF.CheckInterval,
			"DiscoveryInterval": s.config.ONVIF.DiscoveryInterval,
			"MediaPortRange":    s.config.ONVIF.MediaPortRange,
			"EnableCheck":       s.config.ONVIF.EnableCheck,
		}
	}

	if s.config.ZLM != nil {
		zlmConfig := map[string]interface{}{
			"UseEmbedded": s.config.ZLM.UseEmbedded,
			"AutoRestart": s.config.ZLM.AutoRestart,
			"MaxRestarts": s.config.ZLM.MaxRestarts,
		}
		if s.config.ZLM.API != nil {
			zlmConfig["API"] = map[string]interface{}{
				"Debug":       s.config.ZLM.API.Debug,
				"Secret":      s.config.ZLM.API.Secret,
				"SnapRoot":    s.config.ZLM.API.SnapRoot,
				"DefaultSnap": s.config.ZLM.API.DefaultSnap,
			}
		}
		if s.config.ZLM.HTTP != nil {
			zlmConfig["HTTP"] = map[string]interface{}{
				"Port":              s.config.ZLM.HTTP.Port,
				"SSLPort":           s.config.ZLM.HTTP.SSLPort,
				"RootPath":          s.config.ZLM.HTTP.RootPath,
				"DirMenu":           s.config.ZLM.HTTP.DirMenu,
				"AllowCrossDomains": s.config.ZLM.HTTP.AllowCrossDomains,
			}
		}
		if s.config.ZLM.RTSP != nil {
			zlmConfig["RTSP"] = map[string]interface{}{
				"Port":        s.config.ZLM.RTSP.Port,
				"SSLPort":     s.config.ZLM.RTSP.SSLPort,
				"DirectProxy": s.config.ZLM.RTSP.DirectProxy,
				"LowLatency":  s.config.ZLM.RTSP.LowLatency,
				"AuthBasic":   s.config.ZLM.RTSP.AuthBasic,
			}
		}
		if s.config.ZLM.RTMP != nil {
			zlmConfig["RTMP"] = map[string]interface{}{
				"Port":        s.config.ZLM.RTMP.Port,
				"SSLPort":     s.config.ZLM.RTMP.SSLPort,
				"DirectProxy": s.config.ZLM.RTMP.DirectProxy,
			}
		}
		if s.config.ZLM.RTPProxy != nil {
			zlmConfig["RTPProxy"] = map[string]interface{}{
				"Port":       s.config.ZLM.RTPProxy.Port,
				"TimeoutSec": s.config.ZLM.RTPProxy.TimeoutSec,
				"PortRange":  s.config.ZLM.RTPProxy.PortRange,
			}
		}
		if s.config.ZLM.Protocol != nil {
			zlmConfig["Protocol"] = map[string]interface{}{
				"EnableAudio":  s.config.ZLM.Protocol.EnableAudio,
				"AddMuteAudio": s.config.ZLM.Protocol.AddMuteAudio,
				"EnableHLS":    s.config.ZLM.Protocol.EnableHLS,
				"EnableMP4":    s.config.ZLM.Protocol.EnableMP4,
				"EnableRTSP":   s.config.ZLM.Protocol.EnableRTSP,
				"EnableRTMP":   s.config.ZLM.Protocol.EnableRTMP,
				"EnableTS":     s.config.ZLM.Protocol.EnableTS,
				"EnableFMP4":   s.config.ZLM.Protocol.EnableFMP4,
			}
		}
		if s.config.ZLM.Record != nil {
			zlmConfig["Record"] = map[string]interface{}{
				"RecordPath": s.config.ZLM.Record.RecordPath,
				"AppName":    s.config.ZLM.Record.AppName,
				"SampleMS":   s.config.ZLM.Record.SampleMS,
				"FastStart":  s.config.ZLM.Record.FastStart,
				"EnableFmp4": s.config.ZLM.Record.EnableFmp4,
				"FileSecond": s.config.ZLM.Record.FileSecond,
				"FileSizeMB": s.config.ZLM.Record.FileSizeMB,
			}
		}
		if s.config.ZLM.RTC != nil {
			zlmConfig["RTC"] = map[string]interface{}{
				"Port":       s.config.ZLM.RTC.Port,
				"TCPPort":    s.config.ZLM.RTC.TCPPort,
				"TimeoutSec": s.config.ZLM.RTC.TimeoutSec,
				"ExternIP":   s.config.ZLM.RTC.ExternIP,
			}
		}
		config["ZLM"] = zlmConfig
	}

	if s.config.AI != nil {
		config["AI"] = map[string]interface{}{
			"Enable":         s.config.AI.Enable,
			"APIEndpoint":    s.config.AI.APIEndpoint,
			"ModelPath":      s.config.AI.ModelPath,
			"Confidence":     s.config.AI.Confidence,
			"DetectInterval": s.config.AI.DetectInterval,
			"RecordDelay":    s.config.AI.RecordDelay,
			"MinRecordTime":  s.config.AI.MinRecordTime,
		}
	}

	respondRaw(w, http.StatusOK, config)
}

// handleUpdateConfig 更新配置
func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, "无效的请求参数")
		return
	}

	// 这里可以添加配置更新逻辑
	// 目前只保存配置
	if err := s.config.Save(s.configPath); err != nil {
		respondInternalError(w, fmt.Sprintf("保存配置失败: %v", err))
		return
	}

	respondSuccessMsg(w, "配置已更新")
}

// handleServeStaticFile 提供静态文件
func (s *Server) handleServeStaticFile(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "/" || path == "" {
		path = "/index.html"
	}

	filePath := "frontend/dist" + path

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		if !strings.HasPrefix(path, "/api") && !strings.HasPrefix(path, "/assets") {
			http.ServeFile(w, r, "frontend/dist/index.html")
			return
		}
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, filePath)
}

// 辅助函数：获取内存使用率
func getMemoryUsage() int {
	data, err := ioutil.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}

	var memTotal, memAvailable uint64
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				memTotal, _ = strconv.ParseUint(fields[1], 10, 64)
			}
		}
		if strings.HasPrefix(line, "MemAvailable:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				memAvailable, _ = strconv.ParseUint(fields[1], 10, 64)
			}
		}
	}

	if memTotal > 0 {
		used := memTotal - memAvailable
		return int((float64(used) / float64(memTotal)) * 100)
	}
	return 0
}

// 辅助函数：获取CPU使用率
func getCPUUsage() int {
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
		if len(fields) < 8 || fields[0] != "cpu" {
			return 0, 0
		}

		var total, idle uint64
		for i := 1; i < len(fields); i++ {
			val, _ := strconv.ParseUint(fields[i], 10, 64)
			total += val
			if i == 4 {
				idle = val
			}
		}
		return total, idle
	}

	total1, idle1 := readCPU()
	time.Sleep(100 * time.Millisecond)
	total2, idle2 := readCPU()

	if total2 <= total1 {
		return 0
	}

	totalDiff := float64(total2 - total1)
	idleDiff := float64(idle2 - idle1)
	return int((1 - idleDiff/totalDiff) * 100)
}

// decodeJSON 解析JSON请求体
func decodeJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// handleGetServiceStatus 获取服务状态
func (s *Server) handleGetServiceStatus(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"gb28181": map[string]interface{}{
			"enabled": s.gb28181Running,
			"status":  map[bool]string{true: "running", false: "stopped"}[s.gb28181Running],
		},
		"onvif": map[string]interface{}{
			"enabled": s.onvifRunning,
			"status":  map[bool]string{true: "running", false: "stopped"}[s.onvifRunning],
		},
	}
	respondRaw(w, http.StatusOK, response)
}

// handleControlGB28181Service 控制GB28181服务
func (s *Server) handleControlGB28181Service(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Action string `json:"action"` // start 或 stop
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, "无效的请求参数")
		return
	}

	switch req.Action {
	case "start":
		if s.gb28181Running {
			respondSuccessMsg(w, "GB28181服务已在运行中")
			return
		}
		if err := s.gb28181Server.Start(); err != nil {
			respondInternalError(w, fmt.Sprintf("启动GB28181服务失败: %v", err))
			return
		}
		s.gb28181Running = true
		debug.Info("api", "GB28181服务已启动")
		respondSuccessMsg(w, "GB28181服务已启动")

	case "stop":
		if !s.gb28181Running {
			respondSuccessMsg(w, "GB28181服务已停止")
			return
		}
		if err := s.gb28181Server.Stop(); err != nil {
			respondInternalError(w, fmt.Sprintf("停止GB28181服务失败: %v", err))
			return
		}
		s.gb28181Running = false
		debug.Info("api", "GB28181服务已停止")
		respondSuccessMsg(w, "GB28181服务已停止")

	default:
		respondBadRequest(w, "无效的操作，请使用 start 或 stop")
	}
}

// handleControlONVIFService 控制ONVIF服务
func (s *Server) handleControlONVIFService(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Action string `json:"action"` // start 或 stop
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, "无效的请求参数")
		return
	}

	switch req.Action {
	case "start":
		if s.onvifRunning {
			respondSuccessMsg(w, "ONVIF服务已在运行中")
			return
		}
		if err := s.onvifManager.Start(); err != nil {
			respondInternalError(w, fmt.Sprintf("启动ONVIF服务失败: %v", err))
			return
		}
		s.onvifRunning = true
		debug.Info("api", "ONVIF服务已启动")
		respondSuccessMsg(w, "ONVIF服务已启动")

	case "stop":
		if !s.onvifRunning {
			respondSuccessMsg(w, "ONVIF服务已停止")
			return
		}
		if err := s.onvifManager.Stop(); err != nil {
			respondInternalError(w, fmt.Sprintf("停止ONVIF服务失败: %v", err))
			return
		}
		s.onvifRunning = false
		debug.Info("api", "ONVIF服务已停止")
		respondSuccessMsg(w, "ONVIF服务已停止")

	default:
		respondBadRequest(w, "无效的操作，请使用 start 或 stop")
	}
}
