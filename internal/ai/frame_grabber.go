package ai

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"os/exec"
	"time"

	"gb28181-onvif-server/internal/debug"
)

// FrameGrabber 帧捕获器（使用ffmpeg）
type FrameGrabber struct {
	streamURL string
	ffmpegBin string
}

// NewFrameGrabber 创建帧捕获器
func NewFrameGrabber(streamURL string, ffmpegBin string) *FrameGrabber {
	if ffmpegBin == "" {
		ffmpegBin = "ffmpeg"
	}

	return &FrameGrabber{
		streamURL: streamURL,
		ffmpegBin: ffmpegBin,
	}
}

// CaptureFrame 捕获单帧
func (g *FrameGrabber) CaptureFrame(ctx context.Context) (image.Image, error) {
	// 使用ffmpeg捕获单帧
	// ffmpeg -i <stream_url> -vframes 1 -f image2pipe -vcodec mjpeg -

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, g.ffmpegBin,
		"-i", g.streamURL,
		"-vframes", "1",
		"-f", "image2pipe",
		"-vcodec", "mjpeg",
		"-",
	)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		debug.Error("ai", "ffmpeg错误: %s", stderr.String())
		return nil, fmt.Errorf("捕获帧失败: %w", err)
	}

	// 解码JPEG
	img, err := jpeg.Decode(&stdout)
	if err != nil {
		return nil, fmt.Errorf("解码JPEG失败: %w", err)
	}

	return img, nil
}

// CaptureFrameScaled 捕获并缩放帧
func (g *FrameGrabber) CaptureFrameScaled(ctx context.Context, width, height int) (image.Image, error) {
	// ffmpeg -i <stream_url> -vframes 1 -vf scale=320:320 -f image2pipe -vcodec mjpeg -

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	scaleFilter := fmt.Sprintf("scale=%d:%d", width, height)

	cmd := exec.CommandContext(ctx, g.ffmpegBin,
		"-i", g.streamURL,
		"-vframes", "1",
		"-vf", scaleFilter,
		"-f", "image2pipe",
		"-vcodec", "mjpeg",
		"-",
	)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		debug.Error("ai", "ffmpeg错误: %s", stderr.String())
		return nil, fmt.Errorf("捕获帧失败: %w", err)
	}

	// 解码JPEG
	img, err := jpeg.Decode(&stdout)
	if err != nil {
		return nil, fmt.Errorf("解码JPEG失败: %w", err)
	}

	return img, nil
}
