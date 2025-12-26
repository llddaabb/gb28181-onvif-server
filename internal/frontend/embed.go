//go:build embed_frontend

package frontend

import (
	"embed"
	"io/fs"
)

//go:embed dist/*
var distFS embed.FS

// EmbedEnabled 是否启用嵌入式前端
var EmbedEnabled = true

// GetFS 获取前端文件系统
func GetFS() (fs.FS, error) {
	// 去掉 dist 子目录前缀
	return fs.Sub(distFS, "dist")
}
