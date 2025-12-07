//go:build !linux
// +build !linux

package embedded

import (
	"embed"
	"fmt"
	"sync"
)

// 非 Linux 平台的占位实现

var (
	EmbedEnabled      = false
	MediaServerBinary []byte
	ConfigTemplate    []byte
	WWWFiles          embed.FS
	Version           = "not available on this platform"
)

// EmbeddedZLM 嵌入式 ZLM 管理器 (非 Linux 占位)
type EmbeddedZLM struct {
	extractDir string
	mutex      sync.Mutex
}

// NewEmbeddedZLM 创建嵌入式 ZLM 管理器
func NewEmbeddedZLM(extractDir string) *EmbeddedZLM {
	return &EmbeddedZLM{extractDir: extractDir}
}

// IsAvailable 检查嵌入式 ZLM 是否可用
func (e *EmbeddedZLM) IsAvailable() bool {
	return false
}

// Extract 释放嵌入的文件
func (e *EmbeddedZLM) Extract() error {
	return fmt.Errorf("嵌入式 ZLM 只支持 Linux 平台")
}

// GetBinPath 获取可执行文件路径
func (e *EmbeddedZLM) GetBinPath() string {
	return ""
}

// GetConfigPath 获取配置文件路径
func (e *EmbeddedZLM) GetConfigPath() string {
	return ""
}

// GetWorkDir 获取工作目录
func (e *EmbeddedZLM) GetWorkDir() string {
	return ""
}

// GetWWWPath 获取 www 目录路径
func (e *EmbeddedZLM) GetWWWPath() string {
	return ""
}

// Cleanup 清理释放的文件
func (e *EmbeddedZLM) Cleanup() error {
	return nil
}

// GetVersion 获取版本信息
func (e *EmbeddedZLM) GetVersion() string {
	return Version
}
