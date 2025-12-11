package api

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"gb28181-onvif-server/internal/debug"
)

// DetectVideoCodec 使用 ffprobe 对输入流做一次轻量探测，返回视频流的 codec_name（如 h264, hevc）
// 如果无法探测或 ffprobe 不存在则返回错误。
func DetectVideoCodec(inputURL string, timeout time.Duration) (string, error) {
	ffprobePath, err := exec.LookPath("ffprobe")
	if err != nil {
		// fallback to ffmpeg - we can try ffmpeg -hide_banner -loglevel error -select_streams v:0 -show_entries stream=codec_name -of default=noprint_wrappers=1:nokey=1
		ffprobePath = ""
	}

	var cmd *exec.Cmd
	if ffprobePath != "" {
		cmd = exec.Command(ffprobePath, "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=codec_name", "-of", "default=noprint_wrappers=1:nokey=1", inputURL)
	} else {
		// try ffmpeg as fallback
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
		// start with timeout: use goroutine and kill after duration
		if err := cmd.Start(); err != nil {
			return "", fmt.Errorf("start probe failed: %w", err)
		}
		done := make(chan error)
		go func() { done <- cmd.Wait() }()
		select {
		case err := <-done:
			if err != nil {
				debug.Warn("probe", "ffprobe/ffmpeg probe error: %v stderr=%s", err, stderr.String())
			}
		case <-time.After(timeout):
			_ = cmd.Process.Kill()
			return "", fmt.Errorf("probe timeout")
		}
	} else {
		if err := cmd.Run(); err != nil {
			debug.Warn("probe", "ffprobe/ffmpeg probe run error: %v stderr=%s", err, stderr.String())
		}
	}

	outStr := strings.TrimSpace(out.String())
	if outStr == "" {
		// try parse stderr for codec (ffmpeg fallback)
		low := strings.ToLower(stderr.String())
		if strings.Contains(low, "hevc") || strings.Contains(low, "h265") {
			return "hevc", nil
		}
		if strings.Contains(low, "h264") || strings.Contains(low, "avc1") {
			return "h264", nil
		}
		return "", fmt.Errorf("no codec info from probe")
	}

	// ffprobe returns codec like: hevc or h264
	codec := strings.ToLower(outStr)
	return codec, nil
}
