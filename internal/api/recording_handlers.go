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
	zlmHTTPPort := 8080
	if s.config.ZLM != nil {
		zlmHTTPPort = s.config.ZLM.GetHTTPPort()
	}

	// 构造多种播放 URL
	// 1. HTTP直接访问MP4文件（可能浏览器不支持某些编码）
	mp4URL := fmt.Sprintf("http://%s:%d/%s", zlmHost, zlmHTTPPort, strings.ReplaceAll(relPath, "\\", "/"))

	// 2. HTTP-FLV流（需要通过addStreamProxy创建）
	// 生成唯一的回放流ID
	playbackStreamID := fmt.Sprintf("playback_%s_%d", stream, time.Now().Unix())

	// 使用ZLM API创建流代理（将MP4文件作为输入源）
	flvURL := fmt.Sprintf("http://%s:%d/%s/%s.live.flv", zlmHost, zlmHTTPPort, app, playbackStreamID)

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success":        true,
		"mp4Url":         mp4URL,
		"flvUrl":         flvURL,
		"playUrl":        mp4URL, // 默认返回MP4直接访问
		"playbackStream": playbackStreamID,
		"filePath":       filePath,
		"relativePath":   relPath,
		"app":            app,
		"stream":         stream,
		"fileName":       fileName,
		"note":           "如浏览器无法播放MP4，请使用VLC等播放器",
	})
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
