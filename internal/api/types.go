package api

import "time"

// Channel 通道信息
type Channel struct {
	ChannelID    string `json:"channelId"`
	ChannelName  string `json:"channelName"`
	DeviceID     string `json:"deviceId"`
	DeviceType   string `json:"deviceType"` // "gb28181" or "onvif"
	Status       string `json:"status"`     // "online" or "offline"
	StreamURL    string `json:"streamUrl"`
	Channel      string `json:"channel,omitempty"`      // GB28181通道号
	ProfileToken string `json:"profileToken,omitempty"` // ONVIF Profile Token
}

// Recording 录像信息
type Recording struct {
	RecordingID string    `json:"recordingId"`
	ChannelID   string    `json:"channelId"`
	ChannelName string    `json:"channelName"`
	DeviceID    string    `json:"deviceId"`
	StartTime   time.Time `json:"startTime"`
	EndTime     time.Time `json:"endTime"`
	Duration    string    `json:"duration"`
	FileSize    string    `json:"fileSize"`
	Status      string    `json:"status"` // "complete" or "recording"
	PlaybackURL string    `json:"playbackUrl"`
	FilePath    string    `json:"filePath,omitempty"`
	FrameRate   string    `json:"frameRate,omitempty"`
}

// StreamInfo 流信息
type StreamInfo struct {
	StreamID    string    `json:"streamId"`
	ChannelID   string    `json:"channelId"`
	DeviceID    string    `json:"deviceId"`
	StreamURL   string    `json:"streamUrl"`
	StreamType  string    `json:"streamType"`
	Status      string    `json:"status"`
	StartTime   time.Time `json:"startTime"`
	EndTime     time.Time `json:"endTime,omitempty"`
	Duration    string    `json:"duration,omitempty"`
}
