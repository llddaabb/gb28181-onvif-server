package api

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// getZLMHost 获取 ZLM 服务器地址（用于前端访问）
func (s *Server) getZLMHost(r *http.Request) string {
	zlmHost := ""

	// 1. 优先从配置获取
	if s.config.ZLM != nil && s.config.ZLM.General != nil && s.config.ZLM.General.ListenIP != "" {
		if s.config.ZLM.General.ListenIP != "0.0.0.0" && s.config.ZLM.General.ListenIP != "::" {
			zlmHost = s.config.ZLM.General.ListenIP
		}
	}

	// 2. 如果配置为空或 0.0.0.0，尝试从请求获取
	if zlmHost == "" || zlmHost == "0.0.0.0" || zlmHost == "::" {
		if r.Header.Get("X-Forwarded-Host") != "" {
			zlmHost = r.Header.Get("X-Forwarded-Host")
		} else if r.Host != "" {
			if idx := strings.Index(r.Host, ":"); idx > 0 {
				zlmHost = r.Host[:idx]
			} else {
				zlmHost = r.Host
			}
		}
	}

	// 3. 如果仍未获取，使用本机IP
	if zlmHost == "" || zlmHost == "0.0.0.0" || zlmHost == "::" || zlmHost == "localhost" {
		zlmHost = getLocalIP()
	}

	// 如果 getLocalIP() 返回特殊 IP，改用 localhost
	if zlmHost == "127.0.1.1" || zlmHost == "127.0.0.2" {
		zlmHost = "localhost"
	}

	return zlmHost
}

// getZLMPorts 获取 ZLM 各端口配置
func (s *Server) getZLMPorts() (httpPort, rtmpPort, rtspPort int) {
	httpPort = 8081
	rtmpPort = 1935
	rtspPort = 8554

	if s.config.ZLM != nil {
		httpPort = s.config.ZLM.GetHTTPPort()
		rtmpPort = s.config.ZLM.GetRTMPPort()
		rtspPort = s.config.ZLM.GetRTSPPort()
	}
	return
}

// StreamURLs 流URL结构
type StreamURLs struct {
	FlvURL   string `json:"flv_url"`
	WsFlvURL string `json:"ws_flv_url"`
	HlsURL   string `json:"hls_url"`
	RtmpURL  string `json:"rtmp_url"`
}

// buildStreamURLs 构建流访问URL
func (s *Server) buildStreamURLs(r *http.Request, app, streamID string) StreamURLs {
	zlmHost := s.getZLMHost(r)
	_, rtmpPort, _ := s.getZLMPorts()

	// 使用相对路径，让前端使用自己的 origin，避免跨域问题
	// /zlm/ 路径会被代理到 ZLM 服务器
	return StreamURLs{
		FlvURL:   fmt.Sprintf("/zlm/%s/%s.live.flv", app, streamID),
		WsFlvURL: fmt.Sprintf("/zlm/%s/%s.live.flv", app, streamID),
		HlsURL:   fmt.Sprintf("/zlm/%s/%s/hls.m3u8", app, streamID),
		RtmpURL:  fmt.Sprintf("rtmp://%s:%d/%s/%s", zlmHost, rtmpPort, app, streamID),
	}
}

// checkZLMAvailable 检查 ZLM 服务是否可用
func (s *Server) checkZLMAvailable(w http.ResponseWriter) bool {
	if s.zlmServer == nil || s.zlmServer.GetAPIClient() == nil {
		respondServiceUnavailable(w, "ZLM媒体服务未启动")
		return false
	}
	return true
}

// checkZLMProcessAvailable 检查 ZLM 进程管理器是否可用
func (s *Server) checkZLMProcessAvailable(w http.ResponseWriter) bool {
	if s.zlmProcess == nil {
		respondBadRequest(w, "ZLM 进程管理器未初始化")
		return false
	}
	return true
}

// getLocalIP 获取本机IP地址
func getLocalIP() string {
	// 通过连接到外部地址来获取本机IP
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()
		localAddr := conn.LocalAddr().String()
		if idx := strings.LastIndex(localAddr, ":"); idx > 0 {
			ip := localAddr[:idx]
			if ip != "" && ip != "127.0.0.1" && ip != "::1" {
				return ip
			}
		}
	}

	// 尝试获取所有网络接口中的第一个非回环IP
	interfaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range interfaces {
			if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
				continue
			}
			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}
			for _, addr := range addrs {
				var ip net.IP
				switch v := addr.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				default:
					continue
				}
				if ip.To4() != nil {
					return ip.String()
				}
			}
		}
	}

	return "localhost"
}

// parseDate 解析日期字符串
func parseDate(dateStr string) (time.Time, error) {
	return time.Parse("2006-01-02", dateStr)
}
