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

	response := map[string]interface{}{
		"status": "running",
		"servers": map[string]interface{}{
			"gb28181": map[string]interface{}{
				"status":  "running",
				"devices": len(gb28181Devices),
			},
			"onvif": map[string]interface{}{
				"status":  "running",
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
	// 返回安全的配置信息（不含敏感数据）
	config := map[string]interface{}{
		"gb28181": map[string]interface{}{
			"sipIP":             s.config.GB28181.SipIP,
			"sipPort":           s.config.GB28181.SipPort,
			"realm":             s.config.GB28181.Realm,
			"serverID":          s.config.GB28181.ServerID,
			"heartbeatInterval": s.config.GB28181.HeartbeatInterval,
			"registerExpires":   s.config.GB28181.RegisterExpires,
		},
		"api": map[string]interface{}{
			"host": s.config.API.Host,
			"port": s.config.API.Port,
		},
	}

	if s.config.ONVIF != nil {
		config["onvif"] = map[string]interface{}{
			"checkInterval": s.config.ONVIF.CheckInterval,
		}
	}

	respondRaw(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"config":  config,
	})
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
