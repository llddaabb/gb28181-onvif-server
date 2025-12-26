//go:build !embed_frontend
// +build !embed_frontend

package frontend

import (
	"errors"
	"io/fs"
)

// EmbedEnabled 是否启用嵌入式前端
var EmbedEnabled = false

// GetFS 获取前端文件系统（未嵌入时返回错误）
func GetFS() (fs.FS, error) {
	return nil, errors.New("前端文件未嵌入，请使用本地文件系统")
}
