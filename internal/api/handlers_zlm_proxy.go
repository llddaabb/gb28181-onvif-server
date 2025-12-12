package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"gb28181-onvif-server/internal/debug"
)

// handleGetZLMMediaList 获取 ZLM 媒体列表（代理请求，自动添加 secret）
func (s *Server) handleGetZLMMediaList(w http.ResponseWriter, r *http.Request) {
	debug.Info("api", "获取 ZLM 媒体列表")

	// 获取 ZLM API 配置
	zlmHost := s.getZLMHost(r)
	httpPort, _, _ := s.getZLMPorts()

	// 从配置中获取 secret（如果有的话）
	secret := ""
	if s.zlmServer != nil && s.zlmServer.GetAPIClient() != nil {
		// TODO: 从 ZLM API 客户端获取 secret
		// 暂时从环境变量或配置文件读取
	}

	// 构建 ZLM API URL
	zlmURL := fmt.Sprintf("http://%s:%d/index/api/getMediaList", zlmHost, httpPort)
	if secret != "" {
		zlmURL += "?secret=" + secret
	}

	debug.Info("api", "代理请求 ZLM API: %s", zlmURL)

	// 发送请求到 ZLM
	resp, err := http.Get(zlmURL)
	if err != nil {
		respondInternalError(w, fmt.Sprintf("请求 ZLM API 失败: %v", err))
		return
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		respondInternalError(w, fmt.Sprintf("读取 ZLM 响应失败: %v", err))
		return
	}

	// 解析 JSON 响应
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		respondInternalError(w, fmt.Sprintf("解析 ZLM 响应失败: %v", err))
		return
	}

	// 返回结果
	respondRaw(w, http.StatusOK, result)
}
