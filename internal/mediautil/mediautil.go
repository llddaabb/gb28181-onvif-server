package mediautil

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"gb28181-onvif-server/internal/debug"
)

// StartFFmpegTranscode 启动 ffmpeg 将 input 拉取并转码为 h264 推送到 rtmpTarget。
// StartFFmpegTranscode 启动 ffmpeg 将 input 拉取并转码为 h264 推送到 rtmpTarget。
// localHost/localPort 可选，用于替换 ZLM 返回的占位 origin（如 rtp://__defaultVhost__/...）为可访问的本地地址。
func StartFFmpegTranscode(stream string, app string, inputURL string, rtmpTarget string, localHost string, localPort int) error {
	// 简单的内存映射用在调用端维护；为了避免重复管理，这里使用进程启动且不尝试追踪
	ffmpegBin := "/usr/bin/ffmpeg"
	// 如果 origin 是 rtp://__defaultVhost__/... 并且提供了 localPort，则重写为本地地址（使用 udp://<host>:<port>）
	if strings.HasPrefix(strings.ToLower(inputURL), "rtp://__defaultvhost__/") && localPort > 0 {
		host := localHost
		if host == "" || host == "0.0.0.0" {
			host = "127.0.0.1"
		}
		// 只使用本地端口作为 UDP 监听端点，丢弃路径部分
		inputURL = fmt.Sprintf("udp://%s:%d", host, localPort)
	}

	// 根据输入协议选择合适的 ffmpeg 参数
	var args []string
	lower := strings.ToLower(inputURL)
	if strings.HasPrefix(lower, "rtsp://") {
		args = []string{"-rtsp_transport", "tcp", "-re", "-i", inputURL, "-loglevel", "warning", "-c:a", "aac", "-b:a", "64k", "-c:v", "libx264", "-preset", "veryfast", "-tune", "zerolatency", "-x264-params", "keyint=50", "-f", "flv", rtmpTarget}
	} else {
		// UDP/other: 不使用 -rtsp_transport
		args = []string{"-re", "-i", inputURL, "-loglevel", "warning", "-c:a", "aac", "-b:a", "64k", "-c:v", "libx264", "-preset", "veryfast", "-tune", "zerolatency", "-x264-params", "keyint=50", "-f", "flv", rtmpTarget}
	}

	cmd := exec.Command(ffmpegBin, args...)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	debug.Info("ffmpeg", "starting ffmpeg for %s -> %s, cmd: %s %v", stream, rtmpTarget, ffmpegBin, args)
	if err := cmd.Start(); err != nil {
		debug.Error("ffmpeg", "start ffmpeg failed for %s: %v", stream, err)
		return fmt.Errorf("start ffmpeg failed: %w", err)
	}

	// 限制日志输出大小的辅助函数
	truncateOutput := func(output string, maxLen int) string {
		if len(output) <= maxLen {
			return output
		}
		return output[:maxLen] + "... [输出过长，已截断]"
	}

	go func(s string, c *exec.Cmd, outBuf, errBuf *bytes.Buffer) {
		err := c.Wait()
		if err != nil {
			// 只记录错误信息和截断的stderr
			debug.Error("ffmpeg", "ffmpeg process for %s exited with error: %v, stderr=%s", s, err, truncateOutput(errBuf.String(), 200))
			// 如果是因为 udp 端口被占用，尝试从本地 ZLM 的 RTMP 源拉取并重试一次
			low := strings.ToLower(errBuf.String())
			if strings.Contains(low, "address already in use") || strings.Contains(low, "bind failed") {
				debug.Info("ffmpeg", "尝试使用 RTMP 作为回退输入拉取: %s", s)
				// 构造 rtmp 输入，默认使用本地 1935 端口
				rtmpIn := fmt.Sprintf("rtmp://127.0.0.1:1935/%s/%s", app, s)
				args2 := []string{"-re", "-i", rtmpIn, "-loglevel", "warning", "-c:a", "aac", "-b:a", "64k", "-c:v", "libx264", "-preset", "veryfast", "-tune", "zerolatency", "-x264-params", "keyint=50", "-f", "flv", rtmpTarget}
				cmd2 := exec.Command(ffmpegBin, args2...)
				var out2 bytes.Buffer
				var err2 bytes.Buffer
				cmd2.Stdout = &out2
				cmd2.Stderr = &err2
				debug.Info("ffmpeg", "starting fallback ffmpeg for %s -> %s, cmd: %s %v", s, rtmpTarget, ffmpegBin, args2)
				if err := cmd2.Start(); err != nil {
					debug.Error("ffmpeg", "start fallback ffmpeg failed for %s: %v", s, err)
					return
				}
				if err := cmd2.Wait(); err != nil {
					debug.Error("ffmpeg", "fallback ffmpeg process for %s exited with error: %v, stderr=%s", s, err, truncateOutput(err2.String(), 200))
				} else {
					debug.Info("ffmpeg", "fallback ffmpeg process for %s exited", s)
				}
			}
		} else {
			debug.Info("ffmpeg", "ffmpeg process for %s exited normally", s)
		}
	}(stream, cmd, &stdout, &stderr)

	return nil
}

// DetectVideoCodec 使用 ffprobe 对输入流做一次轻量探测，返回视频流的 codec_name（如 h264, hevc）
func DetectVideoCodec(inputURL string, timeout time.Duration) (string, error) {
	ffprobePath, _ := exec.LookPath("ffprobe")
	var cmd *exec.Cmd
	if ffprobePath != "" {
		cmd = exec.Command(ffprobePath, "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=codec_name", "-of", "default=noprint_wrappers=1:nokey=1", inputURL)
	} else {
		// fallback: use ffmpeg and parse stderr
		ffmpegPath, err := exec.LookPath("ffmpeg")
		if err != nil {
			return "", fmt.Errorf("ffprobe and ffmpeg not found in PATH")
		}
		cmd = exec.Command(ffmpegPath, "-hide_banner", "-loglevel", "error", "-i", inputURL, "-t", "0.5", "-c", "copy", "-f", "null", "-")
	}

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if timeout > 0 {
		if err := cmd.Start(); err != nil {
			return "", fmt.Errorf("start probe failed: %w", err)
		}
		done := make(chan error)
		go func() { done <- cmd.Wait() }()
		select {
		case err := <-done:
			if err != nil {
				debug.Warn("probe", "probe error: %v stderr=%s", err, stderr.String())
			}
		case <-time.After(timeout):
			_ = cmd.Process.Kill()
			return "", fmt.Errorf("probe timeout")
		}
	} else {
		if err := cmd.Run(); err != nil {
			debug.Warn("probe", "probe run error: %v stderr=%s", err, stderr.String())
		}
	}

	outStr := strings.TrimSpace(out.String())
	if outStr == "" {
		low := strings.ToLower(stderr.String())
		if strings.Contains(low, "hevc") || strings.Contains(low, "h265") {
			return "hevc", nil
		}
		if strings.Contains(low, "h264") || strings.Contains(low, "avc1") {
			return "h264", nil
		}
		return "", fmt.Errorf("no codec info from probe")
	}

	codec := strings.ToLower(outStr)
	return codec, nil
}
