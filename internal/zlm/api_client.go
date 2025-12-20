package zlm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ZLMAPIClient ZLM REST API 客户端
// ZLM Pro 通过 HTTP API 提供丰富的接口
// API 文档: http://docs.zlmediakit.com/
type ZLMAPIClient struct {
	baseURL    string
	httpClient *http.Client
	secret     string // API 密钥 (如配置)
	timeout    time.Duration
}

// APIResponse ZLM API 响应结构
type APIResponse struct {
	Code    int         `json:"code"` // 0=success, 非0=error
	Msg     string      `json:"msg"`  // 错误信息
	Data    interface{} `json:"data"` // 响应数据
	RawBody string      `json:"-"`    // 原始响应 (调试用)
}

// VersionInfo 版本信息
type VersionInfo struct {
	Version   string `json:"version"`
	BuildTime string `json:"buildTime"`
	GitHash   string `json:"gitHash"`
}

// StreamInfo 流信息
type StreamInfo struct {
	App         string `json:"app"`
	Stream      string `json:"stream"`
	Schema      string `json:"schema"`      // rtsp, rtmp, hls, flv
	Online      int    `json:"online"`      // 0=offline, 1=online
	ReaderCount int    `json:"readerCount"` // 观众数
	BytesSpeed  int64  `json:"bytesSpeed"`  // 实时码率 (字节/秒)
	CreateTime  int64  `json:"createTime"`  // 创建时间
	AliveSecond int    `json:"aliveSecond"` // 存活时间 (秒)
	OriginURL   string `json:"originUrl"`   // 源流地址
	OriginSock  *struct {
		Identifier string `json:"identifier"`
		LocalIP    string `json:"local_ip"`
		LocalPort  int    `json:"local_port"`
		PeerIP     string `json:"peer_ip"`
		PeerPort   int    `json:"peer_port"`
	} `json:"originSock,omitempty"`
	Tracks []TrackInfo `json:"tracks,omitempty"`
}

// TrackInfo 流中单个 track 的信息（用于判断编码）
type TrackInfo struct {
	CodecID   int    `json:"codec_id"`
	CodecName string `json:"codec_id_name"`
	CodecType int    `json:"codec_type"`
}

// RTPInfo RTP 推流信息
type RTPInfo struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"` // rtp, rtcp
}

// StreamProxyInfo 流代理信息
type StreamProxyInfo struct {
	Key        string `json:"key"`      // 流唯一标识
	App        string `json:"app"`      // 应用名
	Stream     string `json:"stream"`   // 流名
	URL        string `json:"url"`      // 源流地址
	Protocol   string `json:"protocol"` // 协议类型 (rtsp, rtmp, hls, flv)
	EnableRTSP bool   `json:"enable_rtsp"`
	EnableRTMP bool   `json:"enable_rtmp"`
	EnableHLS  bool   `json:"enable_hls"`
	EnableFLV  bool   `json:"enable_flv"`
}

// NewZLMAPIClient 创建 API 客户端
func NewZLMAPIClient(baseURL string, opts ...ClientOption) *ZLMAPIClient {
	client := &ZLMAPIClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		timeout: 120 * time.Second,
	}

	// 应用选项
	for _, opt := range opts {
		opt(client)
	}

	return client
}

// ClientOption 客户端选项函数
type ClientOption func(*ZLMAPIClient)

// WithSecret 设置 API 密钥
func WithSecret(secret string) ClientOption {
	return func(c *ZLMAPIClient) {
		c.secret = secret
	}
}

// WithTimeout 设置超时时间
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *ZLMAPIClient) {
		c.timeout = timeout
		c.httpClient.Timeout = timeout
	}
}

// GetVersion 获取版本信息
// API: GET /api/version
func (c *ZLMAPIClient) GetVersion() (*VersionInfo, error) {
	var resp struct {
		Code    int                    `json:"code"`
		Version string                 `json:"version"`
		Data    map[string]interface{} `json:"data"`
	}

	err := c.doRequest("GET", "/api/version", nil, &resp)
	if err != nil {
		return nil, err
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("API error: code=%d", resp.Code)
	}

	// 提取版本信息
	version := &VersionInfo{
		Version: resp.Version,
	}

	if data, ok := resp.Data["version"].(string); ok {
		version.Version = data
	}

	return version, nil
}

// GetMediaList 获取所有媒体流列表
// API: GET /api/getMediaList
func (c *ZLMAPIClient) GetMediaList() ([]*StreamInfo, error) {
	var resp struct {
		Code int                      `json:"code"`
		Msg  string                   `json:"msg"`
		Data []map[string]interface{} `json:"data"`
	}

	err := c.doRequest("GET", "/index/api/getMediaList", nil, &resp)
	if err != nil {
		return nil, err
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("get media list failed: %s", resp.Msg)
	}

	// 解析响应数组
	streams := make([]*StreamInfo, 0)
	seen := make(map[string]bool) // 去重

	for _, item := range resp.Data {
		app, _ := item["app"].(string)
		stream, _ := item["stream"].(string)
		schema, _ := item["schema"].(string)

		// 使用 app_stream 作为唯一键，只保留第一个（通常是 rtsp）
		key := app + "_" + stream
		if seen[key] {
			continue
		}
		seen[key] = true

		streamInfo := &StreamInfo{
			App:    app,
			Stream: stream,
			Online: 1, // 在列表中就表示在线
		}

		if originUrl, ok := item["originUrl"].(string); ok {
			streamInfo.OriginURL = originUrl
		}
		if readerCount, ok := item["readerCount"].(float64); ok {
			streamInfo.ReaderCount = int(readerCount)
		}
		if bytesSpeed, ok := item["bytesSpeed"].(float64); ok {
			streamInfo.BytesSpeed = int64(bytesSpeed)
		}
		if aliveSecond, ok := item["aliveSecond"].(float64); ok {
			streamInfo.AliveSecond = int(aliveSecond)
		}
		if createStamp, ok := item["createStamp"].(float64); ok {
			streamInfo.CreateTime = int64(createStamp)
		}
		streamInfo.Schema = schema

		// 解析 tracks 字段以获取 codec 信息
		if tarr, ok := item["tracks"].([]interface{}); ok {
			for _, ti := range tarr {
				if m, ok2 := ti.(map[string]interface{}); ok2 {
					var tr TrackInfo
					if cid, ok3 := m["codec_id"].(float64); ok3 {
						tr.CodecID = int(cid)
					}
					if cname, ok3 := m["codec_id_name"].(string); ok3 {
						tr.CodecName = cname
					}
					if ctype, ok3 := m["codec_type"].(float64); ok3 {
						tr.CodecType = int(ctype)
					}
					streamInfo.Tracks = append(streamInfo.Tracks, tr)
				}
			}
		}

		// 解析 originSock（如果有）以便上层能获取本地端口信息
		if osock, ok := item["originSock"].(map[string]interface{}); ok {
			var sock struct {
				Identifier string `json:"identifier"`
				LocalIP    string `json:"local_ip"`
				LocalPort  int    `json:"local_port"`
				PeerIP     string `json:"peer_ip"`
				PeerPort   int    `json:"peer_port"`
			}
			if id, ok2 := osock["identifier"].(string); ok2 {
				sock.Identifier = id
			}
			if lip, ok2 := osock["local_ip"].(string); ok2 {
				sock.LocalIP = lip
			}
			if lp, ok2 := osock["local_port"].(float64); ok2 {
				sock.LocalPort = int(lp)
			}
			if pip, ok2 := osock["peer_ip"].(string); ok2 {
				sock.PeerIP = pip
			}
			if pp, ok2 := osock["peer_port"].(float64); ok2 {
				sock.PeerPort = int(pp)
			}
			streamInfo.OriginSock = &sock
		}

		streams = append(streams, streamInfo)
	}

	return streams, nil
}

// OpenRTP 打开 RTP 推流端口
// API: POST /api/openRtp
func (c *ZLMAPIClient) OpenRTP(app, stream string) (*RTPInfo, error) {
	params := map[string]interface{}{
		"app":    app,
		"stream": stream,
	}

	var resp struct {
		Code int                    `json:"code"`
		Msg  string                 `json:"msg"`
		Data map[string]interface{} `json:"data"`
	}

	err := c.doRequest("POST", "/api/openRtp", params, &resp)
	if err != nil {
		return nil, err
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("open RTP failed: %s", resp.Msg)
	}

	rtpInfo := &RTPInfo{}
	if data := resp.Data; data != nil {
		if port, ok := data["port"].(float64); ok {
			rtpInfo.Port = int(port)
		}
	}

	return rtpInfo, nil
}

// CloseStream 关闭媒体流
// API: GET /index/api/close_streams
func (c *ZLMAPIClient) CloseStream(app, stream string) error {
	params := map[string]interface{}{
		"app":    app,
		"stream": stream,
		"vhost":  "__defaultVhost__",
		"force":  1,
	}

	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}

	err := c.doRequest("GET", "/index/api/close_streams", params, &resp)
	if err != nil {
		return err
	}

	if resp.Code != 0 {
		return fmt.Errorf("close stream failed: %s", resp.Msg)
	}

	return nil
}

// AddStreamProxy 添加 RTSP/RTMP 流代理
// API: POST /api/addStreamProxy
// 将 RTSP/RTMP 流转为支持 Web 播放的 HTTP-FLV/WS-FLV/HLS 流
func (c *ZLMAPIClient) AddStreamProxy(sourceURL, app, stream string) (*StreamProxyInfo, error) {
	return c.AddStreamProxyWithOptions(sourceURL, app, stream, nil)
}

// AddStreamProxyWithOptions 支持自定义参数（如 rtp_type/timeout_sec/retry_count）
func (c *ZLMAPIClient) AddStreamProxyWithOptions(sourceURL, app, stream string, opts map[string]interface{}) (*StreamProxyInfo, error) {
	// 默认参数
	params := map[string]interface{}{
		"vhost":          "__defaultVhost__",
		"app":            app,
		"stream":         stream,
		"url":            sourceURL,
		"enable_rtsp":    true,
		"enable_rtmp":    true,
		"enable_hls":     true,
		"enable_mp4":     false,
		"enable_audio":   true,
		"add_mute_audio": true,
		"rtp_type":       0, // 0: tcp, 1: udp, 2: multicast
		"timeout_sec":    15,
		"retry_count":    3,
	}

	// 合并 opts
	if opts != nil {
		for k, v := range opts {
			params[k] = v
		}
	}

	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Key string `json:"key"`
		} `json:"data"`
	}

	err := c.doRequest("GET", "/index/api/addStreamProxy", params, &resp)
	if err != nil {
		return nil, fmt.Errorf("add stream proxy request failed: %w", err)
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("add stream proxy failed: %s (code: %d)", resp.Msg, resp.Code)
	}

	proxyInfo := &StreamProxyInfo{
		Key:        resp.Data.Key,
		App:        app,
		Stream:     stream,
		URL:        sourceURL,
		Protocol:   "rtsp",
		EnableRTSP: true,
		EnableRTMP: true,
		EnableHLS:  true,
		EnableFLV:  true,
	}

	return proxyInfo, nil
}

// DelStreamProxy 删除流代理
// API: POST /api/delStreamProxy
func (c *ZLMAPIClient) DelStreamProxy(key string) error {
	params := map[string]interface{}{
		"key": key,
	}

	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}

	err := c.doRequest("POST", "/api/delStreamProxy", params, &resp)
	if err != nil {
		return fmt.Errorf("del stream proxy request failed: %w", err)
	}

	if resp.Code != 0 {
		return fmt.Errorf("del stream proxy failed: %s (code: %d)", resp.Msg, resp.Code)
	}

	return nil
}

// GetStreamProxyList 获取流代理列表 (仅 Pro 版)
// API: GET /api/getStreamProxyList
func (c *ZLMAPIClient) GetStreamProxyList() ([]*StreamProxyInfo, error) {
	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			Key    string `json:"key"`
			App    string `json:"app"`
			Stream string `json:"stream"`
			URL    string `json:"url"`
		} `json:"data"`
	}

	err := c.doRequest("GET", "/api/getStreamProxyList", nil, &resp)
	if err != nil {
		return nil, fmt.Errorf("get stream proxy list failed: %w", err)
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("get stream proxy list failed: %s", resp.Msg)
	}

	proxies := make([]*StreamProxyInfo, 0, len(resp.Data))
	for _, item := range resp.Data {
		proxies = append(proxies, &StreamProxyInfo{
			Key:    item.Key,
			App:    item.App,
			Stream: item.Stream,
			URL:    item.URL,
		})
	}

	return proxies, nil
}

// IsStreamOnline 检查流是否在线
// API: GET /api/isMediaOnline
func (c *ZLMAPIClient) IsStreamOnline(app, stream string) (bool, error) {
	var resp struct {
		Code   int  `json:"code"`
		Online bool `json:"online"`
	}

	params := map[string]interface{}{
		"app":    app,
		"stream": stream,
	}

	err := c.doRequest("GET", "/api/isMediaOnline", params, &resp)
	if err != nil {
		return false, err
	}

	return resp.Online, nil
}

// GetServerConfig 获取服务器配置
// API: GET /api/getServerConfig
func (c *ZLMAPIClient) GetServerConfig() (map[string]interface{}, error) {
	var resp struct {
		Code int                    `json:"code"`
		Msg  string                 `json:"msg"`
		Data map[string]interface{} `json:"data"`
	}

	err := c.doRequest("GET", "/api/getServerConfig", nil, &resp)
	if err != nil {
		return nil, err
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("get server config failed: %s", resp.Msg)
	}

	return resp.Data, nil
}

// SetServerConfig 设置服务器配置
// API: POST /api/setServerConfig
func (c *ZLMAPIClient) SetServerConfig(config map[string]interface{}) error {
	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}

	err := c.doRequest("POST", "/api/setServerConfig", config, &resp)
	if err != nil {
		return err
	}

	if resp.Code != 0 {
		return fmt.Errorf("set server config failed: %s", resp.Msg)
	}

	return nil
}

// GetStatistic 获取统计信息
// API: GET /api/getStatistic
func (c *ZLMAPIClient) GetStatistic() (map[string]interface{}, error) {
	var resp struct {
		Code int                    `json:"code"`
		Msg  string                 `json:"msg"`
		Data map[string]interface{} `json:"data"`
	}

	err := c.doRequest("GET", "/api/getStatistic", nil, &resp)
	if err != nil {
		return nil, err
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("get statistic failed: %s", resp.Msg)
	}

	return resp.Data, nil
}

// GB28181RtpInfo GB28181 RTP 服务器信息
type GB28181RtpInfo struct {
	Port     int    `json:"port"`      // RTP 接收端口
	StreamID string `json:"stream_id"` // 流ID
}

// OpenRtpServer 打开 GB28181 RTP 接收端口
// API: POST /index/api/openRtpServer
// 用于接收 GB28181 设备推送的 PS 流
func (c *ZLMAPIClient) OpenRtpServer(streamID string, tcpMode int, port int) (*GB28181RtpInfo, error) {
	params := map[string]interface{}{
		"port":      port,     // 0 表示随机端口
		"tcp_mode":  tcpMode,  // 0: 不启用tcp, 1: tcp主动, 2: tcp被动
		"stream_id": streamID, // 流ID
	}

	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Port int    `json:"port"`
	}

	err := c.doRequest("GET", "/index/api/openRtpServer", params, &resp)
	if err != nil {
		return nil, fmt.Errorf("open rtp server failed: %w", err)
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("open rtp server failed: %s (code: %d)", resp.Msg, resp.Code)
	}

	return &GB28181RtpInfo{
		Port:     resp.Port,
		StreamID: streamID,
	}, nil
}

// CloseRtpServer 关闭 GB28181 RTP 接收端口
// API: POST /index/api/closeRtpServer
func (c *ZLMAPIClient) CloseRtpServer(streamID string) error {
	params := map[string]interface{}{
		"stream_id": streamID,
	}

	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}

	err := c.doRequest("GET", "/index/api/closeRtpServer", params, &resp)
	if err != nil {
		return fmt.Errorf("close rtp server failed: %w", err)
	}

	if resp.Code != 0 && resp.Code != -500 { // -500 表示流不存在，忽略
		return fmt.Errorf("close rtp server failed: %s (code: %d)", resp.Msg, resp.Code)
	}

	return nil
}

// ListRtpServer 列出所有 GB28181 RTP 接收端口
// API: GET /index/api/listRtpServer
func (c *ZLMAPIClient) ListRtpServer() ([]map[string]interface{}, error) {
	var resp struct {
		Code int                      `json:"code"`
		Msg  string                   `json:"msg"`
		Data []map[string]interface{} `json:"data"`
	}

	err := c.doRequest("GET", "/index/api/listRtpServer", nil, &resp)
	if err != nil {
		return nil, fmt.Errorf("list rtp server failed: %w", err)
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("list rtp server failed: %s", resp.Msg)
	}

	return resp.Data, nil
}

// doRequest 发送 HTTP 请求
func (c *ZLMAPIClient) doRequest(method, path string, body interface{}, result interface{}) error {
	// 构建 URL
	requestURL := c.baseURL + path

	// 构建请求
	var req *http.Request
	var err error

	if method == "POST" && body != nil {
		// 序列化请求体
		bodyData, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body failed: %w", err)
		}

		// 创建请求
		req, err = http.NewRequest(method, requestURL, io.NopCloser(io.Reader(bytes.NewBuffer(bodyData))))
		if err != nil {
			return err
		}

		// 设置 Content-Type
		req.Header.Set("Content-Type", "application/json")
	} else if method == "GET" && body != nil {
		// GET 请求带查询参数
		req, err = http.NewRequest(method, requestURL, nil)
		if err != nil {
			return err
		}

		// 添加查询参数
		if params, ok := body.(map[string]interface{}); ok {
			q := req.URL.Query()
			for k, v := range params {
				q.Add(k, fmt.Sprintf("%v", v))
			}
			req.URL.RawQuery = q.Encode()
		}
	} else {
		// 无参数的请求
		req, err = http.NewRequest(method, requestURL, nil)
		if err != nil {
			return err
		}
	}

	// 如果设置了 API 密钥，添加到请求中
	if c.secret != "" {
		q := req.URL.Query()
		q.Add("secret", c.secret)
		req.URL.RawQuery = q.Encode()
	}

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response failed: %w", err)
	}

	// 解析 JSON
	if err := json.Unmarshal(respBody, result); err != nil {
		return fmt.Errorf("unmarshal response failed: %w, body: %s", err, string(respBody))
	}

	return nil
}

// Health 健康检查
func (c *ZLMAPIClient) Health() bool {
	_, err := c.GetVersion()
	return err == nil
}

// StartRecord 开始录像
// API: POST /index/api/startRecord
// 参数:
//   - type: 0-hls, 1-mp4
//   - vhost: 虚拟主机
//   - app: 应用名
//   - stream: 流名
//   - customized_path: 自定义录像路径（可选）
//   - max_second: 最大录像时长（秒），0表示无限制
func (c *ZLMAPIClient) StartRecord(app, stream string, recordType int, customPath string, maxSecond int) error {
	var resp struct {
		Code   int    `json:"code"`
		Msg    string `json:"msg"`
		Result bool   `json:"result"`
	}

	params := map[string]interface{}{
		"type":   recordType,
		"vhost":  "__defaultVhost__",
		"app":    app,
		"stream": stream,
	}

	if customPath != "" {
		params["customized_path"] = customPath
	}

	if maxSecond > 0 {
		params["max_second"] = maxSecond
	}

	err := c.doRequest("GET", "/index/api/startRecord", params, &resp)
	if err != nil {
		return err
	}

	if resp.Code != 0 {
		return fmt.Errorf("start record failed: %s", resp.Msg)
	}

	if !resp.Result {
		return fmt.Errorf("start record failed: result=false")
	}

	return nil
}

// StopRecord 停止录像
// API: POST /index/api/stopRecord
func (c *ZLMAPIClient) StopRecord(app, stream string, recordType int) error {
	var resp struct {
		Code   int    `json:"code"`
		Msg    string `json:"msg"`
		Result bool   `json:"result"`
	}

	params := map[string]interface{}{
		"type":   recordType,
		"vhost":  "__defaultVhost__",
		"app":    app,
		"stream": stream,
	}

	err := c.doRequest("GET", "/index/api/stopRecord", params, &resp)
	if err != nil {
		return err
	}

	if resp.Code != 0 {
		return fmt.Errorf("stop record failed: %s", resp.Msg)
	}

	return nil
}

// IsRecording 检查是否正在录像
// API: GET /index/api/isRecording
func (c *ZLMAPIClient) IsRecording(app, stream string, recordType int) (bool, error) {
	var resp struct {
		Code   int  `json:"code"`
		Status bool `json:"status"`
	}

	params := map[string]interface{}{
		"type":   recordType,
		"vhost":  "__defaultVhost__",
		"app":    app,
		"stream": stream,
	}

	err := c.doRequest("GET", "/index/api/isRecording", params, &resp)
	if err != nil {
		return false, err
	}

	if resp.Code != 0 {
		return false, nil
	}

	return resp.Status, nil
}

// GetRecordStatus 获取录像状态（批量查询所有流的录像状态）
func (c *ZLMAPIClient) GetRecordStatus(streams []struct{ App, Stream string }) (map[string]bool, error) {
	result := make(map[string]bool)

	for _, s := range streams {
		key := fmt.Sprintf("%s/%s", s.App, s.Stream)
		recording, err := c.IsRecording(s.App, s.Stream, 1) // 1=mp4
		if err != nil {
			result[key] = false
		} else {
			result[key] = recording
		}
	}

	return result, nil
}

// IsRtpServerOnline 检查 RTP 服务是否已存在，返回端口和 SSRC
func (c *ZLMAPIClient) IsRtpServerOnline(streamID string) (bool, int, string, error) {
	var resp struct {
		Code  int    `json:"code"`
		Msg   string `json:"msg"`
		Exist bool   `json:"exist"`
		Port  int    `json:"port"`
		SSRC  string `json:"ssrc"`
	}

	params := map[string]interface{}{
		"stream_id": streamID,
	}

	err := c.doRequest("GET", "/index/api/getRtpInfo", params, &resp)
	if err != nil {
		return false, 0, "", err
	}

	if resp.Code != 0 || !resp.Exist {
		return false, 0, "", nil
	}

	return true, resp.Port, resp.SSRC, nil
}

// PushStreamInfo 推流信息
type PushStreamInfo struct {
	Key          string `json:"key"`            // 推流唯一标识
	SrcURL       string `json:"src_url"`        // 源流地址
	DstURL       string `json:"dst_url"`        // 目标推流地址
	TimeoutMS    int    `json:"timeout_ms"`     // 超时时间(毫秒)
	EnableHLS    bool   `json:"enable_hls"`     // 是否转HLS
	EnableMP4    bool   `json:"enable_mp4"`     // 是否录制MP4
	EnableRTSP   bool   `json:"enable_rtsp"`    // 是否转RTSP
	EnableRTMP   bool   `json:"enable_rtmp"`    // 是否转RTMP
	AddMuteAudio bool   `json:"add_mute_audio"` // 是否添加静音音轨
}

// PushStreamResult 推流结果
type PushStreamResult struct {
	Key  string `json:"key"`  // 推流唯一标识
	Code int    `json:"code"` // 0=成功
	Msg  string `json:"msg"`  // 消息
}

// AddFFmpegSource 添加 FFmpeg 拉流代理（用于推流）
// 使用 FFmpeg 拉取源流并推送到目标地址
func (c *ZLMAPIClient) AddFFmpegSource(srcURL, dstURL string, timeoutMS int, enableAudio bool) (*PushStreamResult, error) {
	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Key  string `json:"key"`
	}

	params := map[string]interface{}{
		"src_url":        srcURL,
		"dst_url":        dstURL,
		"timeout_ms":     timeoutMS,
		"enable_hls":     false,
		"enable_mp4":     false,
		"add_mute_audio": enableAudio,
	}

	err := c.doRequest("GET", "/index/api/addFFmpegSource", params, &resp)
	if err != nil {
		return nil, err
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("add ffmpeg source failed: %s", resp.Msg)
	}

	return &PushStreamResult{
		Key:  resp.Key,
		Code: resp.Code,
		Msg:  resp.Msg,
	}, nil
}

// DelFFmpegSource 删除 FFmpeg 推流
func (c *ZLMAPIClient) DelFFmpegSource(key string) error {
	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}

	params := map[string]interface{}{
		"key": key,
	}

	err := c.doRequest("GET", "/index/api/delFFmpegSource", params, &resp)
	if err != nil {
		return err
	}

	if resp.Code != 0 {
		return fmt.Errorf("delete ffmpeg source failed: %s", resp.Msg)
	}

	return nil
}

// ListFFmpegSource 列出所有 FFmpeg 推流任务
func (c *ZLMAPIClient) ListFFmpegSource() ([]map[string]interface{}, error) {
	var resp struct {
		Code int                      `json:"code"`
		Msg  string                   `json:"msg"`
		Data []map[string]interface{} `json:"data"`
	}

	err := c.doRequest("GET", "/index/api/listFFmpegSource", nil, &resp)
	if err != nil {
		return nil, err
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("list ffmpeg source failed: %s", resp.Msg)
	}

	return resp.Data, nil
}

// StartSendRtp 开始 RTP 推流（GB28181 方式）
// app: 应用名, stream: 流名, ssrc: RTP的SSRC, dstURL: 目标地址, dstPort: 目标端口
// isUDP: 是否使用UDP, srcPort: 本地端口(可选)
func (c *ZLMAPIClient) StartSendRtp(app, stream, ssrc, dstURL string, dstPort int, isUDP bool, srcPort int) error {
	var resp struct {
		Code      int    `json:"code"`
		Msg       string `json:"msg"`
		LocalPort int    `json:"local_port"`
	}

	params := map[string]interface{}{
		"vhost":    "__defaultVhost__",
		"app":      app,
		"stream":   stream,
		"ssrc":     ssrc,
		"dst_url":  dstURL,
		"dst_port": dstPort,
		"is_udp":   isUDP,
	}

	if srcPort > 0 {
		params["src_port"] = srcPort
	}

	err := c.doRequest("GET", "/index/api/startSendRtp", params, &resp)
	if err != nil {
		return err
	}

	if resp.Code != 0 {
		return fmt.Errorf("start send rtp failed: %s", resp.Msg)
	}

	return nil
}

// StopSendRtp 停止 RTP 推流
func (c *ZLMAPIClient) StopSendRtp(app, stream, ssrc string) error {
	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}

	params := map[string]interface{}{
		"vhost":  "__defaultVhost__",
		"app":    app,
		"stream": stream,
	}

	if ssrc != "" {
		params["ssrc"] = ssrc
	}

	err := c.doRequest("GET", "/index/api/stopSendRtp", params, &resp)
	if err != nil {
		return err
	}

	if resp.Code != 0 {
		return fmt.Errorf("stop send rtp failed: %s", resp.Msg)
	}

	return nil
}
