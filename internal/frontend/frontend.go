// Package frontend 提供前端静态文件的嵌入和服务功能
package frontend

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// StaticFileServer 静态文件服务器
type StaticFileServer struct {
	fs       http.FileSystem
	embedded bool
	localDir string
}

// NewStaticFileServer 创建静态文件服务器
// localDirs 可选的本地目录列表，用于回退
func NewStaticFileServer(localDirs ...string) *StaticFileServer {
	server := &StaticFileServer{}

	// 优先使用嵌入的文件系统
	embeddedFS, err := GetFS()
	if err == nil && EmbedEnabled {
		server.fs = http.FS(embeddedFS)
		server.embedded = true
		return server
	}

	// 回退到本地文件系统
	candidates := append(localDirs,
		"www",
		"frontend/dist",
		"./www",
		"./frontend/dist",
	)

	for _, dir := range candidates {
		if dir == "" {
			continue
		}
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			server.fs = http.Dir(dir)
			server.localDir = dir
			server.embedded = false
			return server
		}
	}

	// 默认目录
	server.fs = http.Dir("www")
	server.localDir = "www"
	server.embedded = false
	return server
}

// IsEmbedded 是否使用嵌入的文件系统
func (s *StaticFileServer) IsEmbedded() bool {
	return s.embedded
}

// LocalDir 返回本地目录路径（仅当使用本地文件系统时有效）
func (s *StaticFileServer) LocalDir() string {
	return s.localDir
}

// GetLocalDir 返回本地目录路径（兼容旧接口）
func (s *StaticFileServer) GetLocalDir() string {
	return s.localDir
}

// ServeHTTP 实现 http.Handler 接口
func (s *StaticFileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// 尝试打开请求的文件
	file, err := s.fs.Open(path)
	if err != nil {
		// 文件不存在，返回 index.html（SPA 路由支持）
		s.serveIndex(w, r)
		return
	}
	defer file.Close()

	// 检查是否是目录
	stat, err := file.Stat()
	if err != nil {
		s.serveIndex(w, r)
		return
	}

	if stat.IsDir() {
		// 尝试提供 index.html
		indexPath := filepath.Join(path, "index.html")
		if indexFile, err := s.fs.Open(indexPath); err == nil {
			indexFile.Close()
			path = indexPath
		} else {
			s.serveIndex(w, r)
			return
		}
	}

	// 设置正确的 Content-Type
	s.setContentType(w, path)

	// 使用 http.FileServer 提供文件
	http.FileServer(s.fs).ServeHTTP(w, r)
}

// serveIndex 提供 index.html
func (s *StaticFileServer) serveIndex(w http.ResponseWriter, r *http.Request) {
	file, err := s.fs.Open("index.html")
	if err != nil {
		http.Error(w, "index.html not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		http.Error(w, "Failed to stat index.html", http.StatusInternalServerError)
		return
	}

	// 读取文件内容
	content := make([]byte, stat.Size())
	if _, err := file.Read(content); err != nil {
		http.Error(w, "Failed to read index.html", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(content)
}

// setContentType 根据文件扩展名设置 Content-Type
func (s *StaticFileServer) setContentType(w http.ResponseWriter, path string) {
	ext := strings.ToLower(filepath.Ext(path))
	contentTypes := map[string]string{
		".html":  "text/html; charset=utf-8",
		".css":   "text/css; charset=utf-8",
		".js":    "application/javascript; charset=utf-8",
		".json":  "application/json; charset=utf-8",
		".wasm":  "application/wasm",
		".png":   "image/png",
		".jpg":   "image/jpeg",
		".jpeg":  "image/jpeg",
		".gif":   "image/gif",
		".svg":   "image/svg+xml",
		".ico":   "image/x-icon",
		".woff":  "font/woff",
		".woff2": "font/woff2",
		".ttf":   "font/ttf",
		".eot":   "application/vnd.ms-fontobject",
	}

	if ct, ok := contentTypes[ext]; ok {
		w.Header().Set("Content-Type", ct)
	}
}

// SubDirHandler 返回子目录的 http.Handler
func (s *StaticFileServer) SubDirHandler(subDir string) http.Handler {
	if s.embedded {
		// 嵌入式文件系统
		embeddedFS, err := GetFS()
		if err != nil {
			return http.NotFoundHandler()
		}
		subFS, err := fs.Sub(embeddedFS, subDir)
		if err != nil {
			return http.NotFoundHandler()
		}
		return http.FileServer(http.FS(subFS))
	}
	// 本地文件系统
	return http.FileServer(http.Dir(filepath.Join(s.localDir, subDir)))
}

// FileSystem 返回底层文件系统
func (s *StaticFileServer) FileSystem() http.FileSystem {
	return s.fs
}

// ListFiles 列出所有文件（用于调试）
func (s *StaticFileServer) ListFiles() ([]string, error) {
	var files []string

	if s.embedded {
		embeddedFS, err := GetFS()
		if err != nil {
			return nil, err
		}
		err = fs.WalkDir(embeddedFS, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				files = append(files, path)
			}
			return nil
		})
		return files, err
	}

	// 本地文件系统
	err := filepath.Walk(s.localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relPath, _ := filepath.Rel(s.localDir, path)
			files = append(files, relPath)
		}
		return nil
	})
	return files, err
}

// String 返回描述信息
func (s *StaticFileServer) String() string {
	if s.embedded {
		return "[前端] ✓ 使用嵌入式前端文件"
	}
	return fmt.Sprintf("[前端] 使用本地前端文件: %s", s.localDir)
}
