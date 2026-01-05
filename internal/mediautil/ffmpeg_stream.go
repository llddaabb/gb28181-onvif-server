package mediautil

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"
	"sync"
	"time"
)

// StreamSession 表示一个 ffmpeg 推流会话
type StreamSession struct {
	ID          string
	FilePath    string
	RTMPUrl     string
	FLVUrl      string
	cmd         *exec.Cmd
	cancel      context.CancelFunc
	StartTime   time.Time
	HWAccelType HWAccelType
	mu          sync.Mutex
	running     bool
	errorChan   chan error
}

// FFmpegStreamManager ffmpeg 推流管理器
type FFmpegStreamManager struct {
	mu           sync.RWMutex
	sessions     map[string]*StreamSession
	detector     *HWAccelDetector
	zlmRTMPHost  string
	zlmRTMPPort  int
	zlmHTTPHost  string
	zlmHTTPPort  int
	sessionIndex int
}

// NewFFmpegStreamManager 创建 ffmpeg 推流管理器
func NewFFmpegStreamManager(zlmRTMPHost string, zlmRTMPPort int, zlmHTTPHost string, zlmHTTPPort int) *FFmpegStreamManager {
	return &FFmpegStreamManager{
		sessions:    make(map[string]*StreamSession),
		detector:    GetGlobalDetector(),
		zlmRTMPHost: zlmRTMPHost,
		zlmRTMPPort: zlmRTMPPort,
		zlmHTTPHost: zlmHTTPHost,
		zlmHTTPPort: zlmHTTPPort,
	}
}

// generateStreamID 生成唯一的流 ID
func (m *FFmpegStreamManager) generateStreamID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessionIndex++
	return fmt.Sprintf("recording_%d_%d", time.Now().Unix(), m.sessionIndex)
}

// StartStream 开始推流
// filePath: 录像文件路径
// 返回: FLV 播放 URL 和错误
func (m *FFmpegStreamManager) StartStream(filePath string) (*StreamSession, error) {
	streamID := m.generateStreamID()

	// 构造 RTMP 推流地址
	// 注意：使用固定的app名称"live"与前端保持一致
	rtmpURL := fmt.Sprintf("rtmp://%s:%d/live/%s", m.zlmRTMPHost, m.zlmRTMPPort, streamID)

	// 构造 FLV 播放地址
	// ZLM 的 FLV 访问路径格式: http://host:port/{app}/{stream}.live.flv
	flvURL := fmt.Sprintf("http://%s:%d/live/%s.live.flv", m.zlmHTTPHost, m.zlmHTTPPort, streamID)

	log.Printf("[ffmpeg推流] 准备推流: %s -> %s", filePath, rtmpURL)

	// 获取硬件加速类型
	hwAccel := m.detector.GetBestAccel()
	log.Printf("[ffmpeg推流] 使用硬件加速: %s", hwAccel)

	// 构建 ffmpeg 命令
	ctx, cancel := context.WithCancel(context.Background())

	args := []string{}

	// 1. 添加硬件加速参数（在输入之前）
	hwAccelArgs := m.detector.GetFFmpegHWAccelArgs()
	args = append(args, hwAccelArgs...)

	// 2. 添加输入文件
	args = append(args, "-re") // 按照原始帧率读取
	args = append(args, "-i", filePath)

	// 3. 编码参数
	// FLV 格式不支持 HEVC，需要检查源编码并可能转码
	args = append(args,
		"-c:v", "libx264", // 使用 H.264 编码（FLV 兼容）
		"-preset", "veryfast", // 快速编码
		"-crf", "28", // 质量：0-51，28 是默认值
		"-c:a", "aac", // 音频转 AAC
		"-ar", "44100",
		"-b:a", "128k",
	)

	// 4. RTMP 输出参数
	args = append(args,
		"-loglevel", "warning",
		"-f", "flv",
		"-flvflags", "no_duration_filesize",
		rtmpURL,
	)

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)

	// 捕获 stderr 用于调试（但限制输出量）
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	// 调试：记录 ffmpeg 命令行
	log.Printf("[ffmpeg推流] 完整命令: ffmpeg %v", args)

	// 创建会话
	session := &StreamSession{
		ID:          streamID,
		FilePath:    filePath,
		RTMPUrl:     rtmpURL,
		FLVUrl:      flvURL,
		cmd:         cmd,
		cancel:      cancel,
		StartTime:   time.Now(),
		HWAccelType: hwAccel,
		running:     true,
		errorChan:   make(chan error, 1),
	}

	// 启动 ffmpeg
	if err := cmd.Start(); err != nil {
		cancel()
		log.Printf("[ffmpeg推流] 启动失败: %v", err)
		return nil, fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	log.Printf("[ffmpeg推流] 成功启动推流会话: %s, PID: %d", streamID, cmd.Process.Pid)

	// 保存会话
	m.mu.Lock()
	m.sessions[streamID] = session
	m.mu.Unlock()

	// 异步监控进程
	go m.monitorSession(session, &stderr)

	// 等待一小段时间确保推流已建立连接
	time.Sleep(500 * time.Millisecond)

	return session, nil
}

// monitorSession 监控推流会话
func (m *FFmpegStreamManager) monitorSession(session *StreamSession, stderr *bytes.Buffer) {
	err := session.cmd.Wait()

	session.mu.Lock()
	session.running = false
	session.mu.Unlock()

	// 限制日志输出大小的辅助函数
	truncateOutput := func(output string, maxLen int) string {
		if len(output) <= maxLen {
			return output
		}
		return output[:maxLen] + "... [输出过长，已截断]"
	}

	if err != nil {
		if err.Error() != "signal: killed" && err.Error() != "context canceled" {
			log.Printf("[ffmpeg推流] 会话 %s 异常退出: %v", session.ID, err)
			// 记录更多的stderr内容用于调试
			stderrOutput := stderr.String()
			log.Printf("[ffmpeg推流] stderr长度: %d 字节", len(stderrOutput))
			log.Printf("[ffmpeg推流] stderr内容: %s", truncateOutput(stderrOutput, 1000))
			session.errorChan <- err
		} else {
			log.Printf("[ffmpeg推流] 会话 %s 正常停止", session.ID)
		}
	} else {
		log.Printf("[ffmpeg推流] 会话 %s 完成", session.ID)
	}

	// 从管理器中移除
	m.mu.Lock()
	delete(m.sessions, session.ID)
	m.mu.Unlock()
}

// StopStream 停止推流
func (m *FFmpegStreamManager) StopStream(streamID string) error {
	m.mu.RLock()
	session, exists := m.sessions[streamID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("stream not found: %s", streamID)
	}

	session.mu.Lock()
	if !session.running {
		session.mu.Unlock()
		return fmt.Errorf("stream already stopped: %s", streamID)
	}
	session.mu.Unlock()

	log.Printf("[ffmpeg推流] 停止推流会话: %s", streamID)

	// 取消上下文，优雅停止
	session.cancel()

	// 等待进程退出（最多等待 5 秒）
	done := make(chan struct{})
	go func() {
		session.cmd.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Printf("[ffmpeg推流] 会话 %s 已停止", streamID)
	case <-time.After(5 * time.Second):
		// 强制杀死进程
		if session.cmd.Process != nil {
			session.cmd.Process.Kill()
			log.Printf("[ffmpeg推流] 会话 %s 强制停止", streamID)
		}
	}

	// 从管理器中移除
	m.mu.Lock()
	delete(m.sessions, streamID)
	m.mu.Unlock()

	return nil
}

// GetSession 获取会话信息
func (m *FFmpegStreamManager) GetSession(streamID string) (*StreamSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	session, exists := m.sessions[streamID]
	return session, exists
}

// ListSessions 列出所有会话
func (m *FFmpegStreamManager) ListSessions() []*StreamSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*StreamSession, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

// StopAll 停止所有推流
func (m *FFmpegStreamManager) StopAll() {
	m.mu.RLock()
	sessionIDs := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		sessionIDs = append(sessionIDs, id)
	}
	m.mu.RUnlock()

	for _, id := range sessionIDs {
		if err := m.StopStream(id); err != nil {
			log.Printf("[ffmpeg推流] 停止会话 %s 失败: %v", id, err)
		}
	}
}

// CleanupExpiredSessions 清理过期的会话
// 如果会话运行时间超过 maxDuration，则自动停止
func (m *FFmpegStreamManager) CleanupExpiredSessions(maxDuration time.Duration) {
	m.mu.RLock()
	expiredIDs := []string{}
	now := time.Now()

	for id, session := range m.sessions {
		if now.Sub(session.StartTime) > maxDuration {
			expiredIDs = append(expiredIDs, id)
		}
	}
	m.mu.RUnlock()

	for _, id := range expiredIDs {
		log.Printf("[ffmpeg推流] 清理过期会话: %s", id)
		m.StopStream(id)
	}
}

// IsRunning 检查会话是否在运行
func (s *StreamSession) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// GetDuration 获取会话运行时长
func (s *StreamSession) GetDuration() time.Duration {
	return time.Since(s.StartTime)
}
