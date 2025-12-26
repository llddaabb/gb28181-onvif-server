package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

// handleListZLMRecordings 按通道和日期查询ZLM录像文件列表 (NVR风格)
func (s *Server) handleListZLMRecordings(w http.ResponseWriter, r *http.Request) {
	if s.zlmProcess == nil {
		s.jsonError(w, http.StatusInternalServerError, "ZLM进程未初始化")
		return
	}

	// 获取查询参数
	channelId := r.URL.Query().Get("channelId")
	dateStr := r.URL.Query().Get("date") // 格式: 2025-12-07
	app := r.URL.Query().Get("app")      // 默认为 live

	if channelId == "" {
		s.jsonError(w, http.StatusBadRequest, "缺少channelId参数")
		return
	}

	if app == "" {
		app = "live"
	}

	// 获取ZLM实际工作目录
	workDir := s.zlmProcess.GetWorkDir()
	recordPath := filepath.Join(workDir, "www", "record")

	// 构造通道录像目录路径: record/{app}/{channelId}/
	channelRecordPath := filepath.Join(recordPath, app, channelId)

	// 检查通道录像目录是否存在
	if _, err := os.Stat(channelRecordPath); os.IsNotExist(err) {
		s.jsonResponse(w, http.StatusOK, map[string]interface{}{
			"success":    true,
			"channelId":  channelId,
			"date":       dateStr,
			"recordPath": channelRecordPath,
			"total":      0,
			"recordings": []interface{}{},
		})
		return
	}

	var recordings []map[string]interface{}
	var targetDate string

	// 如果指定了日期，只查询该日期的录像
	if dateStr != "" {
		targetDate = dateStr
		datePath := filepath.Join(channelRecordPath, targetDate)

		if info, err := os.Stat(datePath); err == nil && info.IsDir() {
			recordings = scanDateDirectory(datePath, app, channelId, targetDate)
		}
	} else {
		// 未指定日期，扫描所有日期目录
		entries, err := os.ReadDir(channelRecordPath)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					datePath := filepath.Join(channelRecordPath, entry.Name())
					dateRecordings := scanDateDirectory(datePath, app, channelId, entry.Name())
					recordings = append(recordings, dateRecordings...)
				}
			}
		}
	}

	// 按开始时间倒序排序
	sort.Slice(recordings, func(i, j int) bool {
		ti := recordings[i]["startTime"].(string)
		tj := recordings[j]["startTime"].(string)
		return ti > tj
	})

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"channelId":  channelId,
		"date":       dateStr,
		"app":        app,
		"recordPath": channelRecordPath,
		"total":      len(recordings),
		"recordings": recordings,
	})
}

// scanDateDirectory 扫描指定日期目录下的录像文件
func scanDateDirectory(datePath, app, channelId, date string) []map[string]interface{} {
	var recordings []map[string]interface{}

	entries, err := os.ReadDir(datePath)
	if err != nil {
		return recordings
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fileName := entry.Name()
		// 处理 mp4 文件（包括正在录制的临时文件，以.开头）
		if !strings.HasSuffix(strings.ToLower(fileName), ".mp4") {
			continue
		}

		// 判断是否为正在录制的文件
		isRecording := strings.HasPrefix(fileName, ".")
		if isRecording {
			// 移除前导点用于显示
			fileName = strings.TrimPrefix(fileName, ".")
		}

		filePath := filepath.Join(datePath, entry.Name()) // 使用原始文件名（包含点）
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// 解析文件名获取时间信息
		// 格式: 2025-12-07-10-25-35-0.mp4
		startTime, endTime, duration := parseRecordingFileName(fileName, date)

		status := "complete"
		if isRecording {
			status = "recording"
			endTime = "录制中"
			duration = "录制中"
		}

		recording := map[string]interface{}{
			"recordingId": fmt.Sprintf("%s_%s_%s", channelId, date, fileName),
			"channelId":   channelId,
			"fileName":    fileName,
			"filePath":    filePath,
			"app":         app,
			"stream":      channelId,
			"date":        date,
			"startTime":   startTime,
			"endTime":     endTime,
			"duration":    duration,
			"size":        info.Size(),
			"fileSize":    formatFileSize(info.Size()),
			"modTime":     info.ModTime().Format("2006-01-02 15:04:05"),
			"timestamp":   info.ModTime().Unix(),
			"status":      status,
		}

		recordings = append(recordings, recording)
	}

	return recordings
}

// parseRecordingFileName 解析录像文件名获取时间信息
// 文件名格式: 2025-12-07-10-25-35-0.mp4 或 .2025-12-07-10-25-35-0.mp4
func parseRecordingFileName(fileName, date string) (startTime, endTime, duration string) {
	// 移除扩展名和前导点
	name := strings.TrimPrefix(fileName, ".")
	name = strings.TrimSuffix(name, ".mp4")

	// 尝试解析时间部分
	// 格式: YYYY-MM-DD-HH-MM-SS-index
	parts := strings.Split(name, "-")
	if len(parts) >= 6 {
		timeStr := fmt.Sprintf("%s-%s-%s %s:%s:%s",
			parts[0], parts[1], parts[2], // 日期
			parts[3], parts[4], parts[5]) // 时间

		startTime = timeStr

		// 尝试解析为时间对象计算结束时间（假设每个文件最多1小时）
		if t, err := time.Parse("2006-01-02 15:04:05", timeStr); err == nil {
			// 简单假设录像时长，实际应该从文件元数据获取
			// 这里暂时设置为文件修改时间作为结束时间的参考
			endTime = t.Add(1 * time.Hour).Format("2006-01-02 15:04:05")
			duration = "未知" // 需要读取视频文件元数据才能准确获取
		} else {
			endTime = startTime
			duration = "未知"
		}
	} else {
		// 无法解析，使用日期作为开始时间
		startTime = date + " 00:00:00"
		endTime = date + " 23:59:59"
		duration = "未知"
	}

	return
}

// handlePlayZLMRecording 播放ZLM录像
// 通过创建临时流代理来播放MP4录像文件
func (s *Server) handlePlayZLMRecording(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	app := params["app"]
	stream := params["stream"]
	fileName := params["file"]

	if s.zlmProcess == nil {
		s.jsonError(w, http.StatusInternalServerError, "ZLM进程未初始化")
		return
	}

	// 获取ZLM实际工作目录
	workDir := s.zlmProcess.GetWorkDir()
	recordPath := filepath.Join(workDir, "www", "record")

	// 文件名可能包含日期目录，需要递归查找
	var filePath string
	if strings.Contains(fileName, "/") {
		// 文件名包含路径，直接拼接
		filePath = filepath.Join(recordPath, app, stream, fileName)
	} else {
		// 只有文件名，需要在通道目录下查找
		filePath = findRecordingFile(recordPath, app, stream, fileName)
	}

	// 检查文件是否存在
	if filePath == "" || !fileExists(filePath) {
		s.jsonError(w, http.StatusNotFound, "录像文件不存在")
		return
	}

	// 获取相对于www目录的路径用于ZLM访问
	// ZLM可以通过 /record/{app}/{stream}/{date}/{file} 访问录像文件
	relPath, _ := filepath.Rel(filepath.Join(workDir, "www"), filePath)

	// 获取 ZLM HTTP 端口
	zlmHost := "127.0.0.1"
	zlmHTTPPort, _, _ := s.getZLMPorts()

	// 构造多种播放 URL
	// 1. HTTP直接访问MP4文件（可能浏览器不支持某些编码）
	mp4URL := fmt.Sprintf("http://%s:%d/%s", zlmHost, zlmHTTPPort, strings.ReplaceAll(relPath, "\\", "/"))

	// 2. 生成唯一的回放流ID
	playbackStreamID := fmt.Sprintf("playback_%s_%d", stream, time.Now().Unix())

	// 3. 尝试通过 ZLM API 创建流代理（将 MP4 文件转为 FLV 流）
	var flvURL string
	var wsFlvURL string
	var streamKey string
	var proxyCreated bool

	if s.zlmServer != nil && s.zlmServer.GetAPIClient() != nil {
		apiClient := s.zlmServer.GetAPIClient()
		// 使用 file:// 协议让 ZLM 读取本地 MP4 文件
		fileSourceURL := "file://" + filePath

		// 添加流代理选项
		opts := map[string]interface{}{
			"enable_mp4":   false,
			"enable_hls":   false,
			"enable_rtsp":  false,
			"enable_rtmp":  false,
			"timeout_sec":  300, // 5分钟超时
			"retry_count":  0,   // 不重试（文件播放完即结束）
			"enable_audio": true,
		}

		proxyInfo, err := apiClient.AddStreamProxyWithOptions(fileSourceURL, "playback", playbackStreamID, opts)
		if err == nil && proxyInfo != nil {
			streamKey = proxyInfo.Key
			flvURL = fmt.Sprintf("http://%s:%d/playback/%s.live.flv", zlmHost, zlmHTTPPort, playbackStreamID)
			wsFlvURL = fmt.Sprintf("ws://%s:%d/playback/%s.live.flv", zlmHost, zlmHTTPPort, playbackStreamID)
			proxyCreated = true
		}
	}

	// 获取文件信息
	fileInfo, _ := os.Stat(filePath)
	var fileSize string
	if fileInfo != nil {
		fileSize = formatFileSize(fileInfo.Size())
	}

	response := map[string]interface{}{
		"success":      true,
		"mp4Url":       mp4URL,
		"playUrl":      mp4URL, // 默认返回MP4
		"downloadUrl":  mp4URL,
		"filePath":     filePath,
		"relativePath": relPath,
		"app":          app,
		"stream":       stream,
		"fileName":     fileName,
		"fileSize":     fileSize,
	}

	if proxyCreated {
		response["flvUrl"] = flvURL
		response["wsFlvUrl"] = wsFlvURL
		response["streamKey"] = streamKey
		response["playUrl"] = flvURL // 优先使用 FLV 流
		response["note"] = "使用 FLV 流播放，支持 H.264/H.265"
	} else {
		response["note"] = "直接播放 MP4 文件，如浏览器不支持请下载后使用播放器"
	}

	s.jsonResponse(w, http.StatusOK, response)
}

// findRecordingFile 在通道目录下递归查找录像文件
func findRecordingFile(recordPath, app, stream, fileName string) string {
	channelPath := filepath.Join(recordPath, app, stream)
	var foundPath string

	// 需要同时查找带点和不带点的文件名
	targetNames := []string{fileName, "." + fileName}

	filepath.Walk(channelPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		for _, name := range targetNames {
			if info.Name() == name {
				foundPath = path
				return filepath.SkipAll
			}
		}
		return nil
	})

	return foundPath
}

// fileExists 检查文件是否存在
func fileExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

// formatFileSize 格式化文件大小
func formatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// handleGetRecordingDates 获取通道有录像的日期列表
// 用于日历标记功能，返回指定年月内有录像的日期
func (s *Server) handleGetRecordingDates(w http.ResponseWriter, r *http.Request) {
	if s.zlmProcess == nil {
		s.jsonError(w, http.StatusInternalServerError, "ZLM进程未初始化")
		return
	}

	// 获取查询参数
	channelId := r.URL.Query().Get("channelId")
	yearStr := r.URL.Query().Get("year")
	monthStr := r.URL.Query().Get("month")
	app := r.URL.Query().Get("app")

	if channelId == "" {
		s.jsonError(w, http.StatusBadRequest, "缺少channelId参数")
		return
	}

	if app == "" {
		app = "live"
	}

	// 解析年月，默认为当前月
	now := time.Now()
	year := now.Year()
	month := int(now.Month())

	if yearStr != "" {
		if y, err := time.Parse("2006", yearStr); err == nil {
			year = y.Year()
		}
	}
	if monthStr != "" {
		if m, err := time.Parse("01", monthStr); err == nil {
			month = int(m.Month())
		} else if m, err := time.Parse("1", monthStr); err == nil {
			month = int(m.Month())
		}
	}

	// 获取ZLM实际工作目录
	workDir := s.zlmProcess.GetWorkDir()
	recordPath := filepath.Join(workDir, "www", "record")

	// 构造通道录像目录路径
	channelRecordPath := filepath.Join(recordPath, app, channelId)

	// 存储有录像的日期
	var recordingDates []string
	dateSet := make(map[string]bool)

	// 检查通道录像目录是否存在
	if _, err := os.Stat(channelRecordPath); os.IsNotExist(err) {
		s.jsonResponse(w, http.StatusOK, map[string]interface{}{
			"success":   true,
			"channelId": channelId,
			"year":      year,
			"month":     month,
			"dates":     []string{},
			"count":     0,
		})
		return
	}

	// 扫描日期目录
	entries, err := os.ReadDir(channelRecordPath)
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, "读取录像目录失败")
		return
	}

	// 目标年月前缀
	targetPrefix := fmt.Sprintf("%d-%02d", year, month)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dateName := entry.Name() // 格式: 2025-12-23

		// 检查是否是目标年月
		if !strings.HasPrefix(dateName, targetPrefix) {
			continue
		}

		// 检查目录内是否有录像文件
		datePath := filepath.Join(channelRecordPath, dateName)
		hasRecordings := checkDirectoryHasRecordings(datePath)

		if hasRecordings && !dateSet[dateName] {
			dateSet[dateName] = true
			recordingDates = append(recordingDates, dateName)
		}
	}

	// 排序日期
	sort.Strings(recordingDates)

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"channelId": channelId,
		"year":      year,
		"month":     month,
		"dates":     recordingDates,
		"count":     len(recordingDates),
	})
}

// checkDirectoryHasRecordings 检查目录内是否有录像文件
func checkDirectoryHasRecordings(dirPath string) bool {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		fileName := entry.Name()
		// 检查是否为 mp4 文件（包括正在录制的）
		if strings.HasSuffix(strings.ToLower(fileName), ".mp4") {
			return true
		}
	}
	return false
}

// handleStopPlayback 停止录像回放（释放流代理资源）
func (s *Server) handleStopPlayback(w http.ResponseWriter, r *http.Request) {
	streamKey := r.URL.Query().Get("key")
	if streamKey == "" {
		s.jsonError(w, http.StatusBadRequest, "缺少 key 参数")
		return
	}

	if s.zlmServer == nil || s.zlmServer.GetAPIClient() == nil {
		s.jsonError(w, http.StatusInternalServerError, "ZLM服务不可用")
		return
	}

	apiClient := s.zlmServer.GetAPIClient()
	err := apiClient.DelStreamProxy(streamKey)
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, fmt.Sprintf("停止回放失败: %v", err))
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "回放已停止",
	})
}
